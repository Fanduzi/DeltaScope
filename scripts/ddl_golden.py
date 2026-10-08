#!/usr/bin/env python3
# input: task manifest testdata/ddl-golden/<TASK>.json, docker/ddl-golden-compose.yaml, freshly built deltascope CLI
# output: inspectable golden artifact (artifact.json + generated policy files + per-case raw evidence) validated against the manifest, including synchronous execute verification and the frozen T05-A3, T05-A4, T05-A5, T05-A6, and T06-A1–A9 oracles
# pos: DDL golden-path runner and artifact validator behind `make ddl-golden TASK=Txx ARTIFACT_DIR=...`
# note: if this file changes, update this header and module README.md.
"""DeltaScope DDL golden-path runner (milestone T02/#81, T04/#83).

`run` mode builds the CLI from the current checkout, starts the pinned
per-anchor database services, executes the manifest's database and CLI cases,
records every raw return code / metadata query result / JSON payload, tears the
stack down deterministically, and then validates the emitted artifact.

`validate` mode re-checks an existing artifact without touching Docker: it
rejects zero cases, missing expected fields, unexecuted required cases,
unreachable or version-mismatched databases, fabricated PASS records, and
stale or missing binaries.
"""

import argparse
import datetime
import hashlib
import json
import os
import pathlib
import re
import subprocess
import sys
import time

ROOT_DIR = pathlib.Path(__file__).resolve().parent.parent
COMPOSE_FILE = ROOT_DIR / "docker" / "ddl-golden-compose.yaml"
COMPOSE_PROJECT = "deltascope-ddl-golden"
MANIFEST_DIR = ROOT_DIR / "testdata" / "ddl-golden"
HEALTH_TIMEOUT_SECONDS = 300
HEALTH_POLL_SECONDS = 5


def fail_run(message):
    print(f"[ddl-golden][FAIL] {message}", file=sys.stderr)
    raise SystemExit(1)


def run_cmd(args, cwd=None, timeout=120, stdin_text=None, env=None):
    proc = subprocess.run(
        args,
        cwd=str(cwd) if cwd else None,
        input=stdin_text,
        text=True,
        capture_output=True,
        timeout=timeout,
        env=env,
    )
    return proc.returncode, proc.stdout, proc.stderr


def utc_now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def load_manifest(task):
    path = MANIFEST_DIR / f"{task}.json"
    if not path.is_file():
        fail_run(f"missing task manifest: {path}")
    with path.open("r", encoding="utf-8") as handle:
        manifest = json.load(handle)
    if manifest.get("task_id") != task:
        fail_run(f"manifest task_id mismatch: expected {task}, got {manifest.get('task_id')}")
    return manifest


BASELINE_FILE = MANIFEST_DIR / "anchors-baseline.json"


def load_baseline():
    if not BASELINE_FILE.is_file():
        fail_run(f"missing locked anchor baseline: {BASELINE_FILE}")
    with BASELINE_FILE.open("r", encoding="utf-8") as handle:
        return json.load(handle)


def baseline_manifest_failures(manifest, baseline):
    """Anchors/dialects a manifest may not drop. The baseline file is checked in
    independently of any task manifest, so shrinking a manifest cannot shrink
    the milestone denominator."""
    failures = []
    anchors = manifest.get("anchors") or {}
    for key, spec in (baseline.get("required_anchors") or {}).items():
        anchor = anchors.get(key)
        if anchor is None:
            failures.append(f"baseline anchor {key} missing from manifest")
            continue
        for field in ("product", "version_contains", "image"):
            if anchor.get(field) != spec.get(field):
                failures.append(f"baseline anchor {key}: manifest {field}={anchor.get(field)!r} != baseline {spec.get(field)!r}")
    return failures


def baseline_executed_failures(artifact, executed, cases, baseline):
    """The baseline also fixes what must be executed: every required anchor
    needs its db_ddl + syntax_negative cases, and every required CLI dialect a
    cli_audit case — regardless of what the manifest's required_case_ids say."""
    task = artifact.get("task_id") or ""
    failures = []
    for key in (baseline.get("required_anchors") or {}):
        for suffix in ("ddl", "syntax_negative"):
            cid = f"{task}.db.{key}.{suffix}"
            if cid not in executed:
                failures.append(f"baseline case {cid} not executed")
    dialects_executed = {c.get("dialect") for c in cases if c.get("kind") == "cli_audit"}
    for dialect in baseline.get("required_cli_dialects") or []:
        if dialect not in dialects_executed:
            failures.append(f"baseline cli dialect {dialect} not executed")
    return failures


def compose(*args, timeout=180):
    rc, out, err = run_cmd(
        ["docker", "compose", "-p", COMPOSE_PROJECT, "-f", str(COMPOSE_FILE), *args],
        cwd=ROOT_DIR,
        timeout=timeout,
    )
    return rc, out, err


def container_health(container):
    rc, out, _ = run_cmd(
        ["docker", "inspect", "--format", "{{if .Config.Healthcheck}}{{ .State.Health.Status }}{{else}}{{.State.Status}}{{end}}", container],
        timeout=30,
    )
    return out.strip() if rc == 0 else "missing"


def wait_for_healthy(containers):
    deadline = time.monotonic() + HEALTH_TIMEOUT_SECONDS
    pending = {c: "pending" for c in containers}
    while time.monotonic() < deadline:
        for container in pending:
            status = container_health(container)
            pending[container] = status
        if all(s == "healthy" or s == "running" for s in pending.values()):
            return pending
        time.sleep(HEALTH_POLL_SECONDS)
    return pending


def image_digest(image):
    rc, out, _ = run_cmd(["docker", "image", "inspect", image, "--format", "{{json .RepoDigests}}"], timeout=30)
    if rc != 0:
        return ""
    digests = json.loads(out.strip() or "[]")
    return digests[0] if digests else ""


def mysql_exec(anchor, sql, database=None, silent=True):
    """Execute fixed fixture SQL through the anchor's dedicated mysql client."""
    client = list(anchor["exec_client"])
    if database:
        client.append(database)
    flag = "-Nse" if silent else "-e"
    client += [flag, sql]
    target = anchor.get("client_container") or anchor["container"]
    return run_cmd(["docker", "exec", target, *client], timeout=120)


def build_cli(work_dir):
    work_dir.mkdir(parents=True, exist_ok=True)
    binary = work_dir / "deltascope"
    rc, out, err = run_cmd(["go", "env", "GOVERSION"], cwd=ROOT_DIR)
    go_version = out.strip() if rc == 0 else "unknown"
    build_env = {**os.environ, "CGO_ENABLED": "0"}
    rc, out, err = run_cmd(
        ["go", "build", "-o", str(binary), "./cmd/deltascope"],
        cwd=ROOT_DIR,
        timeout=600,
        env=build_env,
    )
    if rc != 0:
        fail_run(f"CLI build failed: {err.strip()}")
    digest = hashlib.sha256(binary.read_bytes()).hexdigest()
    return {
        "path": str(binary),
        "sha256": digest,
        "build": {
            "command": "CGO_ENABLED=0 go build -o <artifact>/deltascope ./cmd/deltascope",
            "cgo_enabled": "0",
            "go_version": go_version,
            "built_at": utc_now(),
        },
    }


def sha256_file(path):
    return hashlib.sha256(pathlib.Path(path).read_bytes()).hexdigest()


def make_policies(binary, work_dir, manifest):
    """Build every policy file a manifest may select.

    Without a `policy` block the manifest behaves exactly as before: a single
    generated all-rules-disabled profile (T02/T03 shape). With
    `policy.enable` the manifest fixes an isolated profile — every listed rule
    is enabled with its declared level/params and every other cataloged rule
    is disabled — and an all-rules-disabled file is generated alongside it so
    individual cases can still pin the rule-disabled path (issue #83 T04-A).
    `policy.profiles` names additional isolated profiles a case may select via
    `spec.policy` (issue #83 T04-B). Each enabled rule ID must exist in the
    live `rules list` catalog; the catalog size is never hardcoded.
    """
    rc, out, err = run_cmd([binary, "rules", "list", "--format", "json"], timeout=60)
    if rc != 0:
        fail_run(f"rules list failed: {err.strip()}")
    catalog = json.loads(out)
    ids = sorted({r["rule_id"] for r in catalog.get("rules", [])})
    if not ids:
        fail_run("rules catalog is empty; cannot build isolation profile")

    profile = manifest.get("policy_profile", "all-rules-disabled")
    policy_spec = manifest.get("policy") or {}
    enable = policy_spec.get("enable") or {}
    extra_profiles = {
        name: (spec.get("enable") or {})
        for name, spec in (policy_spec.get("profiles") or {}).items()
    }
    for name in extra_profiles:
        if name == "all-rules-disabled":
            fail_run("policy.profiles may not redefine all-rules-disabled")
        if name == profile:
            fail_run(f"policy.profiles may not redefine the default policy_profile {profile!r}")
    for name, enable_map in [(profile, enable)] + list(extra_profiles.items()):
        for rid in enable_map:
            if rid not in ids:
                fail_run(f"manifest policy profile {name!r} enables unknown rule: {rid}")

    def render(enabled):
        lines = ["rules:"]
        for rid in ids:
            cfg = enabled.get(rid)
            if cfg is None:
                lines.append(f"  {json.dumps(rid)}:\n    enabled: false")
                continue
            lines.append(f"  {json.dumps(rid)}:")
            lines.append(f"    enabled: {str(bool(cfg.get('enabled', True))).lower()}")
            if cfg.get("level"):
                lines.append(f"    level: {cfg['level']}")
            params = cfg.get("params") or {}
            if params:
                lines.append("    params:")
                for key in sorted(params):
                    lines.append(f"      {key}: {json.dumps(params[key])}")
        return "\n".join(lines) + "\n"

    def record(name, path, enabled):
        return {
            "profile": name,
            "path": str(path),
            "sha256": sha256_file(path),
            "catalog_rules": len(ids),
            "enabled_rules": enabled,
            "disabled_rules": len(ids) - len(enabled),
        }

    policies = {}
    if enable:
        policy_path = work_dir / "golden-policy.yaml"
        policy_path.write_text(render(enable), encoding="utf-8")
        policies[profile] = record(profile, policy_path, enable)
    for name, enable_map in extra_profiles.items():
        policy_path = work_dir / f"golden-policy-{name}.yaml"
        policy_path.write_text(render(enable_map), encoding="utf-8")
        policies[name] = record(name, policy_path, enable_map)
    alloff_path = work_dir / ("golden-policy-all-off.yaml" if enable or extra_profiles else "golden-policy.yaml")
    alloff_path.write_text(render({}), encoding="utf-8")
    policies["all-rules-disabled"] = record("all-rules-disabled", alloff_path, {})
    return policies


def policy_for_case(manifest, policies, spec):
    """Resolve which generated policy a case pins. Defaults to the manifest
    policy profile; `spec.policy` may select another generated profile such as
    the always-available all-rules-disabled file."""
    name = spec.get("policy") or manifest.get("policy_profile", "all-rules-disabled")
    policy = policies.get(name)
    if policy is None:
        fail_run(f"case {spec.get('id')!r} selects unknown policy profile {name!r}")
    return name, policy


def execute_ddl_case(manifest, anchor_key):
    anchor = manifest["anchors"][anchor_key]
    case_id = f"{manifest['task_id']}.db.{anchor_key}.ddl"
    case = {
        "case_id": case_id,
        "kind": "db_ddl",
        "anchor": anchor_key,
        "input_sql": [s["sql"] for s in manifest["ddl_steps"]],
        "expected": {"steps": [{"name": s["name"], "rc": s["expect_rc"], "verify": s["verify"]} for s in manifest["ddl_steps"]]},
        "actual": {"steps": [], "database": {}},
        "assertions": [],
        "status": "pass",
    }
    database = {
        "product": anchor["product"],
        "image": anchor["image"],
        "image_digest": image_digest(anchor["image"]),
        "container": anchor["container"],
        "reachable": False,
        "version": "",
    }
    rc, out, err = mysql_exec(anchor, "SELECT VERSION()")
    database["reachable"] = rc == 0
    database["version"] = out.strip()
    case["actual"]["database"] = database
    ok = rc == 0 and anchor["version_contains"] in database["version"]
    case["assertions"].append({
        "name": "database reachable with expected version",
        "ok": ok,
        "detail": f"SELECT VERSION() rc={rc} version={database['version']!r} expected_contains={anchor['version_contains']!r}",
    })
    if not database["reachable"]:
        case["status"] = "fail"
        return case

    if anchor.get("needs_database_create"):
        rc, out, err = mysql_exec(anchor, f"CREATE DATABASE IF NOT EXISTS {anchor['database']}")
        case["assertions"].append({
            "name": "fixture database created",
            "ok": rc == 0,
            "detail": f"CREATE DATABASE rc={rc} stderr={err.strip()!r}",
        })

    for step in manifest["ddl_steps"]:
        step_actual = {"name": step["name"], "sql": step["sql"], "verify": []}
        rc, out, err = mysql_exec(anchor, step["sql"], database=anchor["database"], silent=False)
        step_actual["rc"] = rc
        step_actual["stdout"] = out.strip()
        step_actual["stderr"] = err.strip()
        case["assertions"].append({
            "name": f"{step['name']} return code",
            "ok": rc == step["expect_rc"],
            "detail": f"rc={rc} expected={step['expect_rc']} stderr={err.strip()!r}",
        })
        for verify in step["verify"]:
            vrc, vout, verr = mysql_exec(anchor, verify["sql"], silent=True)
            step_actual["verify"].append({"assert": verify["assert"], "sql": verify["sql"], "rc": vrc, "output": vout.strip(), "stderr": verr.strip()})
            case["assertions"].append({
                "name": f"{step['name']}: {verify['assert']}",
                "ok": vrc == 0 and vout.strip() == verify["expect"],
                "detail": f"rc={vrc} output={vout.strip()!r} expected={verify['expect']!r}",
            })
        case["actual"]["steps"].append(step_actual)

    if not all(a["ok"] for a in case["assertions"]):
        case["status"] = "fail"
    return case


def execute_negative_case(manifest, anchor_key):
    anchor = manifest["anchors"][anchor_key]
    negative = manifest["syntax_negative"]
    case_id = f"{manifest['task_id']}.db.{anchor_key}.syntax_negative"
    case = {
        "case_id": case_id,
        "kind": "db_syntax_negative",
        "anchor": anchor_key,
        "input_sql": negative["sql"],
        "expected": negative["expect"],
        "actual": {},
        "assertions": [],
        "status": "pass",
    }
    rc, out, err = mysql_exec(anchor, negative["sql"], database=anchor["database"], silent=False)
    combined = (out + err).strip()
    match = re.search(r"ERROR\s+(\d+)", combined)
    error_class = match.group(1) if match else ""
    case["actual"] = {"rc": rc, "stdout": out.strip(), "stderr": err.strip(), "error_class": error_class}
    case["assertions"].append({
        "name": "nonzero syntax failure",
        "ok": rc != 0,
        "detail": f"rc={rc}",
    })
    case["assertions"].append({
        "name": "expected error class",
        "ok": error_class == negative["expect"]["error_class"],
        "detail": f"error_class={error_class!r} expected={negative['expect']['error_class']!r}",
    })
    leaked = [m for m in negative["expect"]["forbidden_markers"] if m in combined]
    case["assertions"].append({
        "name": "no permission/connection/object-conflict markers",
        "ok": not leaked,
        "detail": f"forbidden markers found: {leaked!r}",
    })
    if not all(a["ok"] for a in case["assertions"]):
        case["status"] = "fail"
    return case


def cli_case_specs(manifest):
    """Normalize the CLI audit cases a manifest declares. `cli_audit` keeps the
    original one-spec-many-dialects shape; `cli_cases` declares named cases with
    per-case dialect, SQL, extra CLI args, and expectations (issue #82)."""
    if "cli_cases" in manifest:
        return manifest["cli_cases"]
    spec = manifest["cli_audit"]
    return [
        {"id": dialect, "dialect": dialect, "sql": spec["sql"], "expect": spec["expect"]}
        for dialect in spec["dialects"]
    ]


def cli_case_expect_checks(parsed, rc, expect):
    """Assertion rows for one executed CLI case. Required keys are always
    checked; optional keys are checked only when the manifest declares them."""
    statements = (parsed or {}).get("statements") or []
    findings = sum(len(s.get("findings", [])) for s in statements) + len((parsed or {}).get("global_findings") or [])
    diagnostics = (parsed or {}).get("diagnostics") or []
    unsupported = (parsed or {}).get("unsupported") or []
    # evidence_gaps live per statement; the flat list pairs each gap with its
    # owning statement index so entry assertions can pin attribution.
    all_gaps = [
        (i, g)
        for i, s in enumerate(statements)
        for g in (s.get("evidence_gaps") or [])
    ]
    all_findings = [
        f for s in statements for f in (s.get("findings") or [])
    ] + list((parsed or {}).get("global_findings") or [])
    checks = [
        ("exit code", parsed is not None and rc == expect["exit"], f"rc={rc} expected={expect['exit']}"),
        ("verdict", parsed is not None and parsed.get("verdict") == expect["verdict"], f"verdict={parsed.get('verdict') if parsed else None!r}"),
        ("top-level statements", parsed is not None and len(statements) == expect["statements"], f"statements={len(statements)} expected={expect['statements']}"),
        ("zero findings", parsed is not None and findings == expect["findings"], f"findings={findings}"),
        ("no diagnostics", parsed is not None and len(diagnostics) == expect["diagnostics"], f"diagnostics={diagnostics!r}"),
        ("no unsupported", parsed is not None and len(unsupported) == expect["unsupported"], f"unsupported={unsupported!r}"),
    ]
    if "rule_summary_loaded" in expect:
        got = ((parsed or {}).get("rule_summary") or {}).get("loaded")
        checks.append(("loaded rule count", got == expect["rule_summary_loaded"], f"loaded={got!r}"))
    if "statement_indices" in expect:
        got = [s.get("index", 0) for s in statements]
        checks.append(("statement indices", got == expect["statement_indices"], f"indices={got!r}"))
        owned = not ((parsed or {}).get("global_findings") or []) and all(
            f.get("statement_index", 0) == i
            for i, statement in enumerate(statements) for f in statement.get("findings", []))
        checks.append(("finding statement ownership", owned, "findings must belong to their enclosing statement"))
    if "coverage" in expect:
        got = (parsed or {}).get("coverage", {}).get("status")
        checks.append(("aggregate coverage", parsed is not None and got == expect["coverage"], f"coverage={got!r}"))
    if "statement_coverage" in expect:
        got = [s.get("coverage", {}).get("status") for s in statements]
        checks.append(("per-statement coverage", parsed is not None and got == expect["statement_coverage"], f"statement_coverage={got!r}"))
    if "unsupported_features" in expect:
        got = sorted(u.get("feature") for u in unsupported)
        want = sorted(expect["unsupported_features"])
        checks.append(("unsupported features", parsed is not None and got == want, f"features={got!r} expected={want!r}"))
    if "unsupported_entries" in expect:
        # index carries omitempty, so a zero statement index is absent in JSON.
        got = sorted((u.get("index", 0), u.get("feature"), u.get("reason")) for u in unsupported)
        want = sorted((e.get("index", 0), e.get("feature"), e.get("reason")) for e in expect["unsupported_entries"])
        checks.append(("unsupported entries", parsed is not None and got == want, f"entries={got!r} expected={want!r}"))
        # Entries may additionally pin "metadata": each pinned expectation
        # consumes one actual entry sharing its (index, feature, reason) tuple
        # and carrying exactly that bounded metadata map. Repeated tuples are
        # legal — two indexes on one table can emit two expr entries — so
        # matching is one-to-one rather than uniqueness-based.
        consumed = set()
        for e in expect["unsupported_entries"]:
            if "metadata" not in e:
                continue
            key = (e.get("index", 0), e.get("feature"), e.get("reason"))
            found = False
            for i, u in enumerate(unsupported):
                if i in consumed:
                    continue
                if (u.get("index", 0), u.get("feature"), u.get("reason")) == key and u.get("metadata") == e["metadata"]:
                    consumed.add(i)
                    found = True
                    break
            checks.append((
                f"unsupported metadata {key}",
                parsed is not None and found,
                f"entries={[u for u in unsupported if (u.get('index', 0), u.get('feature'), u.get('reason')) == key]!r} expected_metadata={e['metadata']!r}",
            ))
    if "statement_sql" in expect:
        got = [s.get("raw_sql") for s in statements]
        checks.append(("statement raw SQL identity", parsed is not None and got == expect["statement_sql"], f"raw_sql={got!r} expected={expect['statement_sql']!r}"))
    # Every unsupported entry carries the original statement SQL under the
    # current public contract; when either side records it, the two must
    # agree at the entry's statement index. The index itself must reference a
    # retained statement — out-of-range indices are invalid even when both
    # text fields are omitted. Synthetic artifacts that omit both fields on a
    # valid index skip this check.
    for u in unsupported:
        idx = u.get("index", 0)
        in_range = isinstance(idx, int) and not isinstance(idx, bool) and 0 <= idx < len(statements)
        if not in_range:
            checks.append((
                f"unsupported sql identity index={idx!r}",
                False,
                f"index out of range for {len(statements)} statements",
            ))
            continue
        bound = statements[idx].get("raw_sql")
        if u.get("sql") is None and bound is None:
            continue
        checks.append((
            f"unsupported sql identity index={idx}",
            parsed is not None and u.get("sql") == bound,
            f"sql={u.get('sql')!r} expected={bound!r}",
        ))
    if "evidence_gaps" in expect:
        checks.append(("evidence gap count", len(all_gaps) == expect["evidence_gaps"], f"gaps={len(all_gaps)}"))
    if "evidence_gap_entries" in expect:
        got = sorted((i, g.get("rule_id"), g.get("reason_code")) for i, g in all_gaps)
        want = sorted((e.get("index", 0), e.get("rule_id"), e.get("reason_code")) for e in expect["evidence_gap_entries"])
        checks.append(("evidence gap entries", got == want, f"gaps={got!r} expected={want!r}"))
        # required_facts pins consume actual entries one-to-one, like the
        # unsupported metadata pins: order and content must match exactly.
        consumed = set()
        for e in expect["evidence_gap_entries"]:
            if "required_facts" not in e:
                continue
            key = (e.get("index", 0), e.get("rule_id"), e.get("reason_code"))
            found = False
            for pos, (i, g) in enumerate(all_gaps):
                if pos in consumed:
                    continue
                if (i, g.get("rule_id"), g.get("reason_code")) == key and (g.get("required_facts") or []) == e["required_facts"]:
                    consumed.add(pos)
                    found = True
                    break
            checks.append((
                f"evidence gap required_facts {key}",
                parsed is not None and found,
                f"gaps={[g for _, g in all_gaps]!r} expected_facts={e['required_facts']!r}",
            ))
        # A gap smuggled into findings is a contract violation, not a variant.
        gap_ids = {e.get("rule_id") for e in expect["evidence_gap_entries"]}
        if gap_ids:
            leaked = [f for f in all_findings if f.get("rule_id") in gap_ids and f.get("reason_code")]
            checks.append(("no gap encoded as finding", not leaked, f"findings carrying gap shape={leaked!r}"))
    if "finding_entries" in expect:
        got = sorted(
            (f.get("statement_index", 0), f.get("rule_id"), f.get("level"))
            for f in all_findings
        )
        want = sorted(
            (e.get("index", 0), e.get("rule_id"), e.get("level"))
            for e in expect["finding_entries"]
        )
        checks.append(("finding entries", parsed is not None and got == want, f"findings={got!r} expected={want!r}"))
    if "finding_metadata" in expect:
        consumed = set()
        for e in expect["finding_metadata"]:
            key = (e.get("index", 0), e.get("rule_id"))
            found = False
            for pos, f in enumerate(all_findings):
                if pos in consumed:
                    continue
                meta = f.get("metadata")
                meta = meta if isinstance(meta, dict) else {}
                if (f.get("statement_index", 0), f.get("rule_id")) == key and all(meta.get(k) == v for k, v in e["metadata"].items()):
                    consumed.add(pos)
                    found = True
                    break
            checks.append((
                f"finding metadata {key}",
                parsed is not None and found,
                f"findings={[f.get('metadata') for f in all_findings]!r} expected_subset={e['metadata']!r}",
            ))
    if "finding_locations" in expect:
        for e in expect["finding_locations"]:
            idx = e.get("index", 0)
            found = any(
                f.get("statement_index", 0) == idx
                and isinstance(f.get("location"), dict)
                and f["location"].get("line") == e.get("line")
                and f["location"].get("column") == e.get("column")
                for f in all_findings
            )
            checks.append((
                f"finding location index={idx}",
                parsed is not None and found,
                f"locations={[f.get('location') for f in all_findings]!r} expected={e!r}",
            ))
    if "fail_on_triggered" in expect:
        checks.append(("fail_on_triggered", parsed is not None and parsed.get("fail_on_triggered") == expect["fail_on_triggered"], f"fail_on_triggered={parsed.get('fail_on_triggered') if parsed else None!r}"))
    if "version" in expect:
        got = (parsed or {}).get("version")
        want = expect["version"]
        checks.append((
            "version identity",
            parsed is not None and isinstance(got, dict) and all(got.get(k) == v for k, v in want.items()),
            f"version={got!r} expected={want!r}",
        ))
    return checks


def spec_target_version_raw(spec):
    """The raw --target-version token a case's extra args carry, if any."""
    args = spec.get("args") or []
    for i, token in enumerate(args):
        if token == "--target-version" and i + 1 < len(args):
            return args[i + 1]
    return None


def canonical_target_version(raw):
    """Mirror of spec.ParseTargetVersion: [v]MAJOR.MINOR.PATCH → canonical, or
    None when the input is malformed (an input error, never a silent default)."""
    if raw is None:
        return None
    match = re.match(r"^[vV]?(\d+)\.(\d+)\.(\d+)$", raw.strip())
    if not match:
        return None
    return f"{int(match[1])}.{int(match[2])}.{int(match[3])}"


def canonical_observed_version(product, banner):
    """Mirror of spec.ParseObservedVersion: a raw server banner →
    {product, version} canonical dict, honoring the TiDB compatibility marker
    (`8.0.11-TiDB-v8.5.0` → tidb/8.5.0, never the MySQL prefix)."""
    raw = (banner or "").strip()
    if not raw:
        return None
    tidb = re.search(r"-TiDB-v(\d+)\.(\d+)\.(\d+)", raw, re.IGNORECASE)
    if tidb:
        return {"product": "tidb", "version": f"{int(tidb[1])}.{int(tidb[2])}.{int(tidb[3])}"}
    match = re.match(r"^(\d+)\.(\d+)\.(\d+)", raw)
    if not match or not (product or "").strip():
        return None
    return {"product": product.strip(), "version": f"{int(match[1])}.{int(match[2])}.{int(match[3])}"}


def version_evidence_record(spec, parsed, observed_banner=None, observed_product=None):
    """The recorded version evidence for one executed case: raw target input
    plus its canonical form (or None when malformed), the raw observed banner
    plus its canonical form for online cases, and the identity the audit
    result actually emitted. Raw banners stay inside this record and are never
    projected into expectations."""
    target_raw = spec_target_version_raw(spec)
    observed_canonical = canonical_observed_version(observed_product, observed_banner)
    return {
        "target_raw": target_raw,
        "target_canonical": canonical_target_version(target_raw),
        "observed_raw": (observed_banner or "").strip() or None,
        "observed_canonical": observed_canonical,
        "resolved": (parsed or {}).get("version") if isinstance(parsed, dict) else None,
    }


def execute_cli_case(manifest, binary, policies, spec):
    case_id = f"{manifest['task_id']}.cli.{spec['id']}"
    profile_name, policy = policy_for_case(manifest, policies, spec)
    args = [
        binary, "audit",
        "--dialect", spec["dialect"],
        "--sql", spec["sql"],
        "--config", policy["path"],
        "--format", "json",
    ]
    args.extend(spec.get("args") or [])
    rc, out, err = run_cmd(args, cwd=ROOT_DIR, timeout=120, env=case_env(spec))
    case = {
        "case_id": case_id,
        "kind": "cli_audit",
        "cli_case": spec["id"],
        "dialect": spec["dialect"],
        "input_sql": spec["sql"],
        "policy_profile": profile_name,
        "policy_path": policy["path"],
        "command": args,
        "expected": spec["expect"],
        "actual": {"exit": rc, "stdout": out, "stderr": err},
        "assertions": [],
        "status": "pass",
    }
    parsed = {}
    try:
        parsed = json.loads(out)
    except json.JSONDecodeError:
        parsed = None
    case["actual"]["parsed"] = parsed
    case["actual"]["version_evidence"] = version_evidence_record(spec, parsed)
    for name, ok, detail in cli_case_expect_checks(parsed, rc, spec["expect"]):
        case["assertions"].append({"name": name, "ok": bool(ok), "detail": detail})
    if not all(a["ok"] for a in case["assertions"]):
        case["status"] = "fail"
    return case


def case_env(spec):
    """Environment for a CLI invocation. `env` entries inject fixed fixture
    values (for example a password env var name → the compose fixture's root
    password); the manifest declares them so no secret ever enters the case."""
    env = dict(os.environ)
    for key, value in (spec.get("env") or {}).items():
        env[key] = str(value)
    return env


def metadata_case_specs(manifest):
    return manifest.get("metadata_cases") or []


# Instance facts a metadata case may pin (issue #83 T04-B): each name maps to
# the live read path the runner executes against the case's anchor, so the
# artifact records real observed values — never fabricated defaults.
INSTANCE_FACT_QUERIES = {
    "innodb_page_size": "show variables like 'innodb_page_size'",
    "innodb_large_prefix": "show variables like 'innodb_large_prefix'",
    "innodb_default_row_format": "show variables like 'innodb_default_row_format'",
    "tidb_max_index_length": "show config where Type = 'tidb' and Name = 'max-index-length'",
    # MySQL 8.0/8.4 GIPK knobs (issue #85 T06): recorded so a server-generated
    # invisible primary key can never masquerade as a declared PRIMARY KEY.
    "sql_require_primary_key": "show variables like 'sql_require_primary_key'",
    "sql_generate_invisible_primary_key": "show variables like 'sql_generate_invisible_primary_key'",
    "show_gipk_in_create_table_and_information_schema": "show variables like 'show_gipk_in_create_table_and_information_schema'",
}


def read_instance_fact(anchor, fact):
    """Read one live instance fact through the anchor's fixture client.
    Numeric facts return ints, on/off variables return the raw token; a failed
    or empty read returns None so an unreadable fact can never masquerade as
    an observed value."""
    query = INSTANCE_FACT_QUERIES.get(fact)
    if query is None:
        return None
    rc, out, _ = mysql_exec(anchor, query)
    if rc != 0:
        return None
    lines = [line for line in out.strip().splitlines() if line.strip()]
    if not lines:
        return None
    raw = lines[0].split("\t")[-1].strip()
    try:
        return int(raw)
    except ValueError:
        return raw or None


def metadata_case_spec_for(manifest, case):
    case_name = case.get("cli_case") or case.get("metadata_case")
    for spec in metadata_case_specs(manifest):
        if spec.get("id") == case_name:
            return spec
    return None


def execute_metadata_case(manifest, anchor_key, binary, policies, spec):
    """Run the freshly built CLI audit against a live anchor database: fixture
    setup and teardown stay test-driven, the audit only reads metadata, and
    post_verify steps prove the audited object was not mutated."""
    anchor = manifest["anchors"][anchor_key]
    case_id = f"{manifest['task_id']}.meta.{spec['id']}"
    profile_name, policy = policy_for_case(manifest, policies, spec)
    conn = spec["connect"]
    case = {
        "case_id": case_id,
        "kind": "cli_metadata",
        "cli_case": spec["id"],
        "anchor": anchor_key,
        "dialect": spec["dialect"],
        "input_sql": spec["sql"],
        "policy_profile": profile_name,
        "policy_path": policy["path"],
        "connect": {k: v for k, v in conn.items() if k != "password"},
        "expected": spec["expect"],
        "actual": {"database": {}, "setup": [], "post_verify": [], "execute": [], "structure": [], "teardown": []},
        "assertions": [],
        "status": "pass",
    }

    database = {
        "product": anchor["product"],
        "image": anchor["image"],
        "image_digest": image_digest(anchor["image"]),
        "container": anchor["container"],
        "reachable": False,
        "version": "",
    }
    rc, out, _ = mysql_exec(anchor, "SELECT VERSION()")
    database["reachable"] = rc == 0
    database["version"] = out.strip()
    case["actual"]["database"] = database
    case["assertions"].append({
        "name": "database reachable with expected version",
        "ok": rc == 0 and anchor["version_contains"] in database["version"],
        "detail": f"SELECT VERSION() rc={rc} version={database['version']!r} expected_contains={anchor['version_contains']!r}",
    })
    if not database["reachable"]:
        case["status"] = "fail"
        return case

    for step in spec.get("setup") or []:
        src, sout, serr = mysql_exec(anchor, step["sql"], database=anchor["database"], silent=False)
        record = {"name": step["name"], "sql": step["sql"], "rc": src, "stdout": sout.strip(), "stderr": serr.strip()}
        case["actual"]["setup"].append(record)
        case["assertions"].append({
            "name": f"setup {step['name']} return code",
            "ok": src == step.get("expect_rc", 0),
            "detail": f"rc={src} expected={step.get('expect_rc', 0)} stderr={serr.strip()!r}",
        })
        for verify in step.get("verify") or []:
            vrc, vout, verr = mysql_exec(anchor, verify["sql"], database=anchor["database"] if verify.get("use_database") else None, silent=True)
            record.setdefault("verify", []).append({"assert": verify["assert"], "sql": verify["sql"], "rc": vrc, "output": vout.strip(), "stderr": verr.strip()})
            case["assertions"].append({
                "name": f"setup {step['name']}: {verify['assert']}",
                "ok": vrc == 0 and vout.strip() == verify["expect"],
                "detail": f"rc={vrc} output={vout.strip()!r} expected={verify['expect']!r}",
            })

    args = [
        binary, "audit",
        "--dialect", spec["dialect"],
        "--sql", spec["sql"],
        "--config", policy["path"],
        "--format", "json",
        "--host", conn["host"],
        "--port", str(conn["port"]),
        "--user", conn["user"],
    ]
    if conn.get("password_env"):
        args += ["--password-env", conn["password_env"]]
    if conn.get("password_file"):
        args += ["--password-file", conn["password_file"]]
    if conn.get("schema"):
        args += ["--schema", conn["schema"]]
    args.extend(spec.get("args") or [])
    env = case_env(spec)
    if conn.get("password_env") and conn.get("password") is not None:
        env[conn["password_env"]] = str(conn["password"])
    rc, out, err = run_cmd(args, cwd=ROOT_DIR, timeout=180, env=env)
    case["command"] = args
    case["actual"].update({"exit": rc, "stdout": out, "stderr": err})
    parsed = {}
    try:
        parsed = json.loads(out)
    except json.JSONDecodeError:
        parsed = None
    case["actual"]["parsed"] = parsed
    case["actual"]["version_evidence"] = version_evidence_record(
        spec, parsed, observed_banner=database.get("version"), observed_product=anchor["product"],
    )
    declared_facts = (spec.get("expect") or {}).get("instance_facts")
    if declared_facts is not None:
        observed_facts = {}
        for fact_name, want in declared_facts.items():
            observed_facts[fact_name] = read_instance_fact(anchor, fact_name)
            case["assertions"].append({
                "name": f"instance fact {fact_name}",
                "ok": observed_facts[fact_name] == want,
                "detail": f"{fact_name}={observed_facts[fact_name]!r} expected={want!r}",
            })
        case["actual"]["instance_facts"] = observed_facts
    for name, ok, detail in cli_case_expect_checks(parsed, rc, spec["expect"]):
        case["assertions"].append({"name": name, "ok": bool(ok), "detail": detail})

    for check in spec.get("post_verify") or []:
        vrc, vout, verr = mysql_exec(anchor, check["sql"], database=anchor["database"] if check.get("use_database") else None, silent=True)
        case["actual"]["post_verify"].append({"assert": check["assert"], "sql": check["sql"], "rc": vrc, "output": vout.strip(), "stderr": verr.strip()})
        case["assertions"].append({
            "name": f"post-verify {check['assert']}",
            "ok": vrc == 0 and vout.strip() == check["expect"],
            "detail": f"rc={vrc} output={vout.strip()!r} expected={check['expect']!r}",
        })

    # The test driver applies the audited statements itself, statement by
    # statement — the product only ever reads; the fixture must prove the
    # audited SQL actually works on this anchor (issue #84 T05 oracle).
    for step in spec.get("execute") or []:
        erc, eout, eerr = mysql_exec(anchor, step["sql"], database=anchor["database"], silent=False)
        record = {"name": step["name"], "sql": step["sql"], "rc": erc, "stdout": eout.strip(), "stderr": eerr.strip()}
        case["actual"]["execute"].append(record)
        case["assertions"].append({
            "name": f"execute {step['name']} return code",
            "ok": erc == step.get("expect_rc", 0),
            "detail": f"rc={erc} expected={step.get('expect_rc', 0)} stderr={eerr.strip()!r}",
        })
        for marker in step.get("stderr_contains") or []:
            case["assertions"].append({"name": f"execute {step['name']} stderr marker",
                                       "ok": marker in eerr, "detail": marker})
        for marker in step.get("stdout_contains") or []:
            case["assertions"].append({"name": f"execute {step['name']} stdout marker",
                                       "ok": marker in eout, "detail": marker})
        for verify in step.get("verify") or []:
            vrc, vout, verr = mysql_exec(anchor, verify["sql"], database=anchor["database"] if verify.get("use_database") else None, silent=True)
            record.setdefault("verify", []).append({"assert": verify["assert"], "sql": verify["sql"], "rc": vrc, "output": vout.strip(), "stderr": verr.strip()})
            case["assertions"].append({
                "name": f"execute {step['name']}: {verify['assert']}",
                "ok": vrc == 0 and vout.strip() == verify["expect"],
                "detail": f"rc={vrc} output={vout.strip()!r} expected={verify['expect']!r}",
            })

    # Structure oracle: after the driver applies the audited statements the
    # resulting schema must match the frozen expectation exactly — column
    # type, index membership/order, primary key membership/order.
    for check in spec.get("structure") or []:
        vrc, vout, verr = mysql_exec(anchor, check["sql"], database=anchor["database"] if check.get("use_database") else None, silent=True)
        case["actual"]["structure"].append({"assert": check["assert"], "sql": check["sql"], "rc": vrc, "output": vout.strip(), "stderr": verr.strip()})
        case["assertions"].append({
            "name": f"structure {check['assert']}",
            "ok": vrc == 0 and vout.strip() == check["expect"],
            "detail": f"rc={vrc} output={vout.strip()!r} expected={check['expect']!r}",
        })

    for step in spec.get("teardown") or []:
        trc, tout, terr = mysql_exec(anchor, step["sql"], database=anchor["database"], silent=False)
        record = {"name": step["name"], "sql": step["sql"], "rc": trc, "stderr": terr.strip()}
        case["actual"]["teardown"].append(record)
        case["assertions"].append({
            "name": f"teardown {step['name']} return code",
            "ok": trc == step.get("expect_rc", 0),
            "detail": f"rc={trc} expected={step.get('expect_rc', 0)} stderr={terr.strip()!r}",
        })
        for verify in step.get("verify") or []:
            vrc, vout, verr = mysql_exec(anchor, verify["sql"], database=anchor["database"] if verify.get("use_database") else None, silent=True)
            record.setdefault("verify", []).append({"assert": verify["assert"], "sql": verify["sql"], "rc": vrc, "output": vout.strip(), "stderr": verr.strip()})
            case["assertions"].append({
                "name": f"teardown {step['name']}: {verify['assert']}",
                "ok": vrc == 0 and vout.strip() == verify["expect"],
                "detail": f"rc={vrc} output={vout.strip()!r} expected={verify['expect']!r}",
            })

    if not all(a["ok"] for a in case["assertions"]):
        case["status"] = "fail"
    return case


def error_case_specs(manifest):
    return manifest.get("error_cases") or []


def error_case_spec_for(manifest, case):
    for spec in error_case_specs(manifest):
        if spec.get("id") == (case.get("cli_case") or case.get("error_case")):
            return spec
    return None


def execute_error_case(manifest, binary, policies, spec):
    """Run a CLI invocation expected to fail before producing an audit result —
    for example a real connection refusal — asserting only the exit code and
    bounded stderr markers, never parsing stdout as a result."""
    case_id = f"{manifest['task_id']}.clierr.{spec['id']}"
    profile_name, policy = policy_for_case(manifest, policies, spec)
    args = [
        binary, "audit",
        "--dialect", spec["dialect"],
        "--sql", spec["sql"],
        "--config", policy["path"],
        "--format", "json",
    ]
    args.extend(spec.get("args") or [])
    rc, out, err = run_cmd(args, cwd=ROOT_DIR, timeout=120, env=case_env(spec))
    # Anchored error cases record the observed server banner they conflicted
    # with, so the validator can re-derive the canonical identity behind a
    # version/product mismatch instead of trusting stderr text alone.
    anchor = (manifest.get("anchors") or {}).get(spec.get("anchor") or "")
    observed_banner = None
    observed_product = None
    if anchor is not None:
        brc, bout, _ = mysql_exec(anchor, "SELECT VERSION()")
        if brc == 0:
            observed_banner = bout.strip()
            observed_product = anchor["product"]
    case = {
        "case_id": case_id,
        "kind": "cli_error",
        "cli_case": spec["id"],
        "dialect": spec["dialect"],
        "input_sql": spec["sql"],
        "policy_profile": profile_name,
        "policy_path": policy["path"],
        "command": args,
        "expected": spec["expect"],
        "actual": {"exit": rc, "stdout": out, "stderr": err,
                   "version_evidence": version_evidence_record(
                       spec, None, observed_banner=observed_banner, observed_product=observed_product)},
        "assertions": [],
        "status": "pass",
    }
    if anchor is not None:
        case["anchor"] = spec["anchor"]
        case["actual"]["observed_banner"] = observed_banner
        case["assertions"].append({
            "name": "observed banner evidence",
            "ok": observed_banner is not None,
            "detail": f"SELECT VERSION() on {spec['anchor']} returned {observed_banner!r}",
        })
    expect = spec["expect"]
    case["assertions"].append({
        "name": "exit code",
        "ok": rc == expect["exit"],
        "detail": f"rc={rc} expected={expect['exit']}",
    })
    for marker in expect.get("stderr_contains") or []:
        case["assertions"].append({
            "name": f"stderr contains {marker!r}",
            "ok": marker in err,
            "detail": f"stderr={err.strip()!r} marker={marker!r}",
        })
    if not all(a["ok"] for a in case["assertions"]):
        case["status"] = "fail"
    return case


def parse_generated_policy(text):
    """Explicit state machine for the restricted, deterministic policy YAML
    shape make_policies renders — stdlib-only, no third-party dependency.
    Grammar: exactly one `rules:` header as the first non-empty line; rule
    blocks only after it; per rule at most one `enabled`, one `level`, and
    one `params:` block (field order fixed: enabled/level before params);
    each params key at most once. Unknown lines, entries outside a rule
    block, duplicate headers/fields/param keys/rule IDs, and non-JSON param
    values all raise ValueError, so a hand-tampered file with different
    structure is rejected rather than silently mis-parsed."""
    rules = {}
    saw_header = False
    current = None
    in_params = False
    seen_enabled = seen_level = seen_params = False
    param_keys = set()
    for lineno, raw in enumerate(text.splitlines(), 1):
        if not raw.strip():
            continue
        if not saw_header:
            if raw == "rules:":
                saw_header = True
                continue
            raise ValueError(f"line {lineno}: first non-empty line must be 'rules:', got {raw!r}")
        m = re.match(r'^  ("(?:[^"\\]|\\.)*"):\s*$', raw)
        if m:
            current = json.loads(m.group(1))
            if current in rules:
                raise ValueError(f"line {lineno}: duplicate rule id {current!r}")
            rules[current] = {}
            in_params = False
            seen_enabled = seen_level = seen_params = False
            param_keys = set()
            continue
        if current is None:
            raise ValueError(f"line {lineno}: entry outside a rule block: {raw!r}")
        m = re.match(r'^    enabled: (true|false)\s*$', raw)
        if m:
            if in_params or seen_enabled:
                raise ValueError(f"line {lineno}: duplicate or misplaced enabled in {current!r}")
            rules[current]["enabled"] = m.group(1) == "true"
            seen_enabled = True
            continue
        m = re.match(r'^    level: (\S+)\s*$', raw)
        if m:
            if in_params or seen_level:
                raise ValueError(f"line {lineno}: duplicate or misplaced level in {current!r}")
            rules[current]["level"] = m.group(1)
            seen_level = True
            continue
        if raw == "    params:":
            if seen_params:
                raise ValueError(f"line {lineno}: duplicate params block in {current!r}")
            rules[current]["params"] = {}
            seen_params = True
            in_params = True
            continue
        m = re.match(r'^      ([^\s:]+): (.*)$', raw)
        if m and in_params:
            key = m.group(1)
            if key in param_keys:
                raise ValueError(f"line {lineno}: duplicate param key {key!r} in {current!r}")
            param_keys.add(key)
            rules[current]["params"][key] = json.loads(m.group(2))
            continue
        raise ValueError(f"line {lineno}: unrecognized policy line: {raw!r}")
    if not saw_header:
        raise ValueError("policy file has no top-level rules: map")
    return rules


def policy_semantics_failures(profile, record, path, expected_enable, catalog_ids):
    """Re-derive a generated policy's semantics from the YAML on disk and the
    live rules catalog — never from artifact fields alone. expected_enable is
    the manifest-declared enabled map for the isolated profile and {} for
    every other profile, so an emptied or re-leveled file fails even when its
    sha256 was regenerated. The file is parsed with the stdlib-only restricted
    parser matching the runner's own generated grammar."""
    failures = []
    try:
        rules = parse_generated_policy(path.read_text(encoding="utf-8"))
    except (OSError, ValueError) as exc:
        failures.append(f"policy {profile}: unreadable/invalid policy YAML: {exc}")
        return failures
    if catalog_ids is not None:
        if sorted(rules.keys()) != catalog_ids:
            failures.append(f"policy {profile}: YAML rule set differs from live rules catalog")
        if record.get("catalog_rules") != len(catalog_ids):
            failures.append(f"policy {profile}: catalog_rules {record.get('catalog_rules')!r} != live catalog {len(catalog_ids)}")
    elif record.get("catalog_rules") is not None and len(rules) != record["catalog_rules"]:
        failures.append(f"policy {profile}: YAML rule count {len(rules)} != recorded catalog_rules {record['catalog_rules']}")
    enabled_actual = {}
    for rid, cfg in rules.items():
        if not isinstance(cfg, dict):
            failures.append(f"policy {profile}: rule {rid!r} has non-map config")
            continue
        if cfg.get("enabled") is True:
            enabled_actual[rid] = cfg
        elif cfg.get("enabled") is not False:
            failures.append(f"policy {profile}: rule {rid!r} enabled flag is not a boolean")
    if sorted(enabled_actual) != sorted(expected_enable):
        failures.append(f"policy {profile}: enabled rule set {sorted(enabled_actual)!r} != manifest-derived {sorted(expected_enable)!r}")
    for rid, cfg in enabled_actual.items():
        want = expected_enable.get(rid) or {}
        if cfg.get("level") != want.get("level"):
            failures.append(f"policy {profile}: rule {rid!r} level {cfg.get('level')!r} != expected {want.get('level')!r}")
        if (cfg.get("params") or {}) != (want.get("params") or {}):
            failures.append(f"policy {profile}: rule {rid!r} params {cfg.get('params')!r} != expected {want.get('params')!r}")
    if record.get("enabled_rules") != expected_enable:
        failures.append(f"policy {profile}: recorded enabled_rules differ from manifest-derived expectation")
    return failures


def expected_cli_argv(cli_path, spec, policy_path, connect=None):
    """Rebuild the exact argv a case must record, derived from the manifest
    spec, the declared policy file path, and the declared connect target."""
    argv = [cli_path, "audit",
            "--dialect", spec["dialect"],
            "--sql", spec["sql"],
            "--config", policy_path,
            "--format", "json"]
    if connect:
        argv += ["--host", connect["host"], "--port", str(connect["port"]), "--user", connect["user"]]
        if connect.get("password_env"):
            argv += ["--password-env", connect["password_env"]]
        if connect.get("password_file"):
            argv += ["--password-file", connect["password_file"]]
        if connect.get("schema"):
            argv += ["--schema", connect["schema"]]
    argv += spec.get("args") or []
    return argv


def command_binding_failures(case, command, spec, policy_by_name, cli_path, connect=None):
    """Bind the recorded command to the manifest-derived argv: binary path,
    --dialect/--sql/--config/--format values, declared connect target, and any
    spec args (including --fail-on) must match verbatim."""
    failures = []
    case_id = case.get("case_id")
    if not command:
        failures.append(f"case {case_id}: no recorded command")
        return failures
    profile_name = case.get("policy_profile")
    record = policy_by_name.get(profile_name)
    if record is None or not record.get("path"):
        failures.append(f"case {case_id}: cannot bind --config, policy record for {profile_name!r} has no path")
        return failures
    expected = expected_cli_argv(cli_path, spec, record["path"], connect)
    if command != expected:
        failures.append(f"case {case_id}: recorded command differs from manifest-derived argv\n  expected: {expected!r}\n  recorded: {command!r}")
    return failures


def compose_cleanup():
    rc, out, err = compose("down", "-v", "--remove-orphans", timeout=180)
    rc2, remaining, _ = run_cmd(
        ["docker", "ps", "-aq", "--filter", f"name={COMPOSE_PROJECT}"],
        timeout=30,
    )
    residual = [line for line in remaining.splitlines() if line.strip()]
    return {
        "compose_down_rc": rc,
        "compose_down_stderr": err.strip(),
        "residual_containers": residual,
    }


def manifest_expected(manifest, case):
    """Rebuild a case's expected record from the manifest, never the artifact."""
    kind = case.get("kind")
    if kind == "db_ddl":
        return {"steps": [{"name": s["name"], "rc": s["expect_rc"], "verify": s["verify"]} for s in manifest["ddl_steps"]]}
    if kind == "db_syntax_negative":
        return manifest["syntax_negative"]["expect"]
    if kind == "cli_audit":
        spec = cli_case_spec_for(manifest, case)
        if spec is None:
            return None
        return spec["expect"]
    if kind == "cli_metadata":
        spec = metadata_case_spec_for(manifest, case)
        if spec is None:
            return None
        return spec["expect"]
    if kind == "cli_error":
        spec = error_case_spec_for(manifest, case)
        if spec is None:
            return None
        return spec["expect"]
    return None


def expected_policy_profile(manifest, case, spec):
    """The profile a case must record, derived from the manifest alone."""
    if spec is not None and spec.get("policy"):
        return spec["policy"]
    return manifest.get("policy_profile", "all-rules-disabled")


def manifest_declared_policy_enables(manifest):
    """{profile: enable_map} for every manifest-declared isolated profile —
    the default `policy.enable` profile plus every `policy.profiles` entry."""
    policy_spec = manifest.get("policy") or {}
    declared = {}
    if (policy_spec.get("enable") or {}):
        declared[manifest.get("policy_profile", "all-rules-disabled")] = policy_spec["enable"]
    for name, spec in (policy_spec.get("profiles") or {}).items():
        declared[name] = spec.get("enable") or {}
    return declared


def version_evidence_failures(case, spec, reparsed, anchor=None):
    """Re-derive version evidence from the manifest spec and raw stdout.

    The recorded version_evidence block is never trusted: target raw input and
    its canonical form must match the declared --target-version arg; observed
    raw/canonical identity must match the live anchor banner; and the emitted
    result version must equal the canonical observed identity online (never a
    caller override, never the TiDB compatibility prefix)."""
    failures = []
    case_id = case.get("case_id")
    actual = case.get("actual") or {}
    evidence = actual.get("version_evidence")
    target_raw = spec_target_version_raw(spec) if spec else None
    target_canonical = canonical_target_version(target_raw)

    if evidence is None:
        if target_raw is not None or reparsed and reparsed.get("version") is not None:
            failures.append(f"case {case_id}: missing version_evidence record")
        return failures

    if evidence.get("target_raw") != target_raw:
        failures.append(f"case {case_id}: recorded target_raw {evidence.get('target_raw')!r} != manifest --target-version {target_raw!r}")
    if evidence.get("target_canonical") != target_canonical:
        failures.append(f"case {case_id}: target_canonical {evidence.get('target_canonical')!r} inconsistent with raw input {target_raw!r}")

    observed_canonical = None
    if anchor is not None:
        # cli_metadata cases record the live banner under actual.database;
        # anchored cli_error cases record it under actual.observed_banner.
        banner = ((actual.get("database") or {}).get("version") or (actual.get("observed_banner") or "")).strip()
        observed_canonical = canonical_observed_version(anchor["product"], banner)
        if not banner:
            failures.append(f"case {case_id}: anchored case is missing the observed server banner")
        if evidence.get("observed_raw") != (banner or None):
            failures.append(f"case {case_id}: observed_raw {evidence.get('observed_raw')!r} != recorded database banner {banner!r}")
        if evidence.get("observed_canonical") != observed_canonical:
            failures.append(f"case {case_id}: observed_canonical {evidence.get('observed_canonical')!r} != canonical banner {observed_canonical!r}")

    reparsed_version = reparsed.get("version") if isinstance(reparsed, dict) else None
    if evidence.get("resolved") != reparsed_version:
        failures.append(f"case {case_id}: recorded resolved version differs from raw stdout version")

    if isinstance(reparsed_version, dict):
        # Public numeric components are always serialized — a zero minor or
        # patch (8.0.46, 8.5.0) must not vanish through omitempty, and the trio
        # must rebuild the canonical version string.
        for component in ("major", "minor", "patch"):
            if component not in reparsed_version:
                failures.append(f"case {case_id}: emitted version missing numeric component {component!r}")
        canonical = reparsed_version.get("version")
        parts = canonical.split(".") if isinstance(canonical, str) else []
        if len(parts) == 3 and all(p.isdigit() for p in parts):
            actual_components = (
                reparsed_version.get("major"),
                reparsed_version.get("minor"),
                reparsed_version.get("patch"),
            )
            if actual_components != (int(parts[0]), int(parts[1]), int(parts[2])):
                failures.append(
                    f"case {case_id}: emitted version components {actual_components!r} "
                    f"disagree with canonical version {canonical!r}")
        product = reparsed_version.get("product")
        version = reparsed_version.get("version")
        source = reparsed_version.get("source")
        if anchor is not None and observed_canonical is not None:
            if (product, version, source) != (observed_canonical["product"], observed_canonical["version"], "observed"):
                failures.append(
                    f"case {case_id}: emitted version {reparsed_version!r} does not match canonical observed identity {observed_canonical!r}")
            if target_canonical is not None and version != target_canonical:
                failures.append(
                    f"case {case_id}: target {target_canonical!r} conflicts with observed {version!r} yet recorded as a successful audit")
        elif anchor is None and target_canonical is not None:
            want_product = {"mysql": "mysql", "tidb": "tidb"}.get((spec or {}).get("dialect"))
            if version != target_canonical or source != "target" or (want_product and product != want_product):
                failures.append(f"case {case_id}: emitted version {reparsed_version!r} inconsistent with canonical target {target_canonical!r}")
        if reparsed_version.get("validated_range") is False:
            coverage = (reparsed.get("coverage") or {}).get("status") if isinstance(reparsed, dict) else None
            if coverage == "complete":
                failures.append(f"case {case_id}: out-of-range version recorded as complete coverage")
    return failures


def cli_stdout_expect_failures(case, spec, failures, anchor=None):
    """Re-parse the recorded raw stdout and re-run the manifest expectation —
    the artifact's stored `parsed` blob is never trusted over raw bytes."""
    raw = case.get("actual", {}).get("stdout")
    try:
        reparsed = json.loads(raw) if isinstance(raw, str) else None
    except json.JSONDecodeError:
        reparsed = None
    if reparsed is None:
        failures.append(f"case {case.get('case_id')}: CLI stdout is not JSON")
        return
    recorded_parsed = case.get("actual", {}).get("parsed")
    if recorded_parsed is not None and recorded_parsed != reparsed:
        failures.append(f"case {case.get('case_id')}: recorded parsed disagrees with raw stdout")
    for name, ok, detail in cli_case_expect_checks(reparsed, case.get("actual", {}).get("exit"), spec["expect"]):
        if not ok:
            failures.append(f"case {case.get('case_id')}: {name}: {detail}")
    failures.extend(version_evidence_failures(case, spec, reparsed, anchor=anchor))


def cli_case_spec_for(manifest, case):
    """Find the manifest CLI spec a recorded case claims to satisfy."""
    case_name = case.get("cli_case") or case.get("dialect")
    for spec in cli_case_specs(manifest):
        if spec.get("id") == case_name:
            return spec
    return None


def t05_a3_contract():
    profile = "t05-drop-recreate-isolated"
    drop_rule = "ddl.table.drop.exists.require"
    index_rule = "ddl.create_index.columns.exists.require"
    enable = {rid: {"enabled": True, "level": "blocker", "params": {}}
              for rid in ("ddl.table.exists.create.forbid", drop_rule,
                          "ddl.table.exists.alter.require", "ddl.alter.add_column.exists.forbid", index_rule)}
    enable[index_rule]["params"] = {"required": True}
    table_query = "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'"

    def verify(label, sql, expect):
        return {"assert": label, "sql": sql, "expect": expect}

    def expectation(sqls, verdict="pass", finding=None, gap=False):
        result = {"exit": 0 if verdict == "pass" else 1, "verdict": verdict,
                  "statements": len(sqls), "findings": int(finding is not None),
                  "diagnostics": 0, "unsupported": 0,
                  "coverage": "unverified" if gap else "complete",
                  "statement_coverage": ["unverified" if gap else "complete"] * len(sqls),
                  "statement_sql": [sql + ";" for sql in sqls],
                  "evidence_gaps": int(gap), "fail_on_triggered": verdict != "pass",
                  "rule_summary_loaded": 5, "statement_indices": list(range(len(sqls)))}
        if finding is not None:
            idx, rid, metadata = finding
            result["finding_entries"] = [{"index": idx, "rule_id": rid, "level": "blocker"}]
            result["finding_metadata"] = [{"index": idx, "rule_id": rid, "metadata": metadata}]
        if gap:
            result["evidence_gap_entries"] = [{"index": 0, "rule_id": drop_rule,
                                               "reason_code": "unknown_table_state",
                                               "required_facts": ["target_table.existence"]}]
        return result

    metadata_cases, cli_cases = [], []
    for anchor, port in (("mysql57", 23357), ("mysql80", 23380), ("mysql84", 23384), ("tidb85", 24000)):
        dialect = "tidb" if anchor == "tidb85" else "mysql"
        modes = ["drop-recreate", "drop-if-exists-recreate"]
        if anchor in ("mysql84", "tidb85"):
            modes += ["drop-absent", "drop-if-exists-absent", "drop-recreate-old-column"]
        for mode in modes:
            drop = "DROP TABLE IF EXISTS t" if "if-exists" in mode else "DROP TABLE t"
            absent = mode.endswith("absent")
            old_column = mode.endswith("old-column")
            sqls = [drop] if absent else ["CREATE TABLE t (old_c INT PRIMARY KEY)", drop,
                      "CREATE TABLE t (id INT PRIMARY KEY)", "ALTER TABLE t ADD COLUMN c INT",
                      "CREATE INDEX idx_c ON t(c)"]
            if old_column:
                sqls.append("CREATE INDEX ix_old ON t(old_c)")
            finding = None
            if absent:
                finding = (0, drop_rule, {"table": "t", "operation": "drop_table", "exists": False})
            elif old_column:
                finding = (5, index_rule, {"schema": "golden", "table": "t", "index": "ix_old", "column": "old_c", "exists": False})
            connect = {"host": "127.0.0.1", "port": port, "user": "root", "schema": "golden"}
            if dialect == "mysql":
                connect.update(password_env="DS_T05_GOLDEN_PW", password="root")
            case = {"id": f"t05-a3-{anchor}-{mode}", "anchor": anchor, "dialect": dialect,
                    "sql": " ".join(sql + ";" for sql in sqls), "policy": profile,
                    "args": ["--fail-on", "warning"], "connect": connect,
                    "setup": [{"name": "ensure t absent", "sql": "DROP TABLE IF EXISTS t", "expect_rc": 0,
                               "verify": [verify("t absent before audit", table_query, "0")]}],
                    "expect": expectation(sqls, "reject" if finding else "pass", finding),
                    "post_verify": [verify("audit did not create t", table_query, "0")],
                    "execute": [{"name": f"driver statement {i}", "sql": sql, "expect_rc": 0}
                                for i, sql in enumerate(sqls)],
                    "structure": [],
                    "teardown": [{"name": "drop fixture", "sql": "DROP TABLE IF EXISTS t", "expect_rc": 0,
                                  "verify": [verify("no residual t", table_query, "0")]}]}
            if absent:
                step = case["execute"][0]
                if "if-exists" in mode:
                    step["sql"] += "; SHOW WARNINGS;"
                    step["stdout_contains"] = ["Note", "1051", "golden.t"]
                else:
                    step["expect_rc"] = 1
                    step["stderr_contains"] = ["ERROR 1051", "golden.t"]
                step["verify"] = [verify("t remains absent", table_query, "0")]
            else:
                case["execute"][1]["verify"] = [verify("t absent between DROP and CREATE", table_query, "0")]
                case["structure"] = [
                    verify("new t exists", table_query, "1"),
                    verify("new columns exactly id and c", "SELECT GROUP_CONCAT(CONCAT(COLUMN_NAME,':',DATA_TYPE) ORDER BY ORDINAL_POSITION SEPARATOR ',') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'", "id:int,c:int"),
                    verify("old_c is absent", "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND COLUMN_NAME='old_c'", "0"),
                    verify("new primary key exactly id", "SELECT GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND INDEX_NAME='PRIMARY'", "id"),
                    verify("new idx_c exactly c", "SELECT GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND INDEX_NAME='idx_c'", "c"),
                    verify("only the two new indexes", "SELECT COUNT(DISTINCT INDEX_NAME) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'", "2")]
                if old_column:
                    case["execute"][-1].update(expect_rc=1, stderr_contains=["ERROR 1072", "old_c"])
            metadata_cases.append(case)
    for dialect in ("mysql", "tidb"):
        for conditional in (False, True):
            sql = "DROP TABLE IF EXISTS t" if conditional else "DROP TABLE t"
            mode = "drop-if-exists-unknown" if conditional else "drop-unknown"
            cli_cases.append({"id": f"t05-a3-{dialect}-{mode}", "dialect": dialect,
                              "sql": sql + ";", "policy": profile, "args": ["--fail-on", "warning"],
                              "expect": expectation([sql], "review", gap=True)})
    return profile, {"enable": enable}, metadata_cases, cli_cases


def t05_a4_contract():
    """Frozen T05-A4 MODIFY oracle. The 19 cases and their expects are
    independent of the manifest, so shrinking the manifest and artifact together
    still fails. Eighteen are the original ordinary replacement paths. The
    tidb85 primary-key signedness case records the native refusal."""
    profile = "t05-a4-modify-isolated"
    compat = "ddl.alter.modify_column.compatibility.require"
    create_rule = "ddl.table.exists.create.forbid"
    enable = {rid: {"enabled": True, "level": "blocker", "params": {}}
              for rid in (create_rule, "ddl.table.exists.alter.require",
                          "ddl.alter.modify_column.exists.require", compat)}
    enable[compat]["params"] = {"required": True}
    table_query = "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'"
    columns_query = ("SELECT GROUP_CONCAT(CONCAT(COLUMN_NAME,':',DATA_TYPE) ORDER BY ORDINAL_POSITION SEPARATOR ',') "
                     "FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'")
    pk_query = ("SELECT GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') FROM information_schema.STATISTICS "
                "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND INDEX_NAME='PRIMARY'")
    varchar_query = ("SELECT CONCAT(DATA_TYPE,':',IFNULL(CHARACTER_MAXIMUM_LENGTH,''),':',IFNULL(CHARACTER_SET_NAME,''),':',"
                     "IFNULL(COLLATION_NAME,''),':',IS_NULLABLE,':',IFNULL(COLUMN_DEFAULT,'<nil>'),':',IFNULL(COLUMN_COMMENT,'')) "
                     "FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND COLUMN_NAME='c'")
    int_query = ("SELECT CONCAT(DATA_TYPE,':',IF(COLUMN_TYPE LIKE '%unsigned%','unsigned','signed'),':',IS_NULLABLE,':',"
                 "IFNULL(COLUMN_DEFAULT,'<nil>'),':',IFNULL(COLUMN_COMMENT,'')) "
                 "FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND COLUMN_NAME='c'")
    id_query = ("SELECT CONCAT(DATA_TYPE,':',IS_NULLABLE) FROM information_schema.COLUMNS "
                "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND COLUMN_NAME='id'")
    id_signed_query = ("SELECT CONCAT(DATA_TYPE,':',IF(COLUMN_TYPE LIKE '%unsigned%','unsigned','signed'),':',IS_NULLABLE) "
                       "FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND COLUMN_NAME='id'")
    plain_query = ("SELECT CONCAT(DATA_TYPE,':',IS_NULLABLE) FROM information_schema.COLUMNS "
                   "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND COLUMN_NAME='c'")

    def verify(label, sql, expect):
        return {"assert": label, "sql": sql, "expect": expect}

    def varchar_text(length):
        return f"varchar:{length}:utf8mb4:utf8mb4_bin:NO:<nil>:"

    def varchar_sqls(lengths):
        sqls = [f"CREATE TABLE t (id INT PRIMARY KEY, c VARCHAR({lengths[0]}) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL)"]
        sqls.extend(
            f"ALTER TABLE t MODIFY COLUMN c VARCHAR({length}) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL"
            for length in lengths[1:])
        return sqls

    def expectation(sqls, verdict, coverage, statement_coverage, findings=0, gap_entries=None,
                    finding_entries=None, finding_metadata=None):
        result = {
            "exit": 0 if verdict != "reject" else 1,
            "verdict": verdict,
            "statements": len(sqls),
            "findings": findings,
            "diagnostics": 0,
            "unsupported": 0,
            "coverage": coverage,
            "statement_coverage": statement_coverage,
            "statement_sql": [sql + ";" for sql in sqls],
            "evidence_gaps": len(gap_entries or []),
            "fail_on_triggered": verdict == "reject",
            "rule_summary_loaded": 4,
            "statement_indices": list(range(len(sqls))),
        }
        if finding_entries:
            result["finding_entries"] = finding_entries
        if finding_metadata:
            result["finding_metadata"] = finding_metadata
        if gap_entries:
            result["evidence_gap_entries"] = gap_entries
        return result

    def shrink(index, source, target):
        metadata = {"action": "modify_column", "table": "t", "name": "c", "column_name": "c",
                    "source_length": source, "target_length": target}
        return ([{"index": index, "rule_id": compat, "level": "blocker"}],
                [{"index": index, "rule_id": compat, "metadata": metadata}])

    def attribute(index, metadata):
        payload = {"action": "modify_column", "table": "t", "name": "c", "column_name": "c"}
        payload.update(metadata)
        return ({"index": index, "rule_id": compat, "level": "blocker"},
                {"index": index, "rule_id": compat, "metadata": payload})

    def lifecycle(sqls, lengths, query, values, verdict, finding=None):
        complete = ["complete"] * len(sqls)
        kwargs = {}
        if finding:
            kwargs["finding_entries"], kwargs["finding_metadata"] = finding
            kwargs["findings"] = len(kwargs["finding_entries"])
        return expectation(sqls, verdict, "complete", complete, **kwargs), [
            {"name": f"driver statement {i}", "sql": sql, "expect_rc": 0,
             "verify": [verify(f"definition after statement {i}", query, values[i])]}
            for i, sql in enumerate(sqls)]

    def metadata_case(case_id, anchor, port, dialect, sqls, expect, execute, structure):
        connect = {"host": "127.0.0.1", "port": port, "user": "root", "schema": "golden"}
        if dialect == "mysql":
            connect.update(password_env="DS_T05_GOLDEN_PW", password="root")
        return {"id": case_id, "anchor": anchor, "dialect": dialect,
                "sql": " ".join(sql + ";" for sql in sqls), "policy": profile,
                "args": ["--fail-on", "blocker"], "connect": connect,
                "setup": [{"name": "ensure t absent", "sql": "DROP TABLE IF EXISTS t", "expect_rc": 0,
                           "verify": [verify("t absent before audit", table_query, "0")]}],
                "expect": expect,
                "post_verify": [verify("audit did not create t", table_query, "0")],
                "execute": execute, "structure": structure,
                "teardown": [{"name": "drop fixture", "sql": "DROP TABLE IF EXISTS t", "expect_rc": 0,
                              "verify": [verify("no residual t", table_query, "0")]}]}

    def varchar_case(anchor, port, dialect, suffix, lengths, verdict, finding, columns):
        sqls = varchar_sqls(lengths)
        expect, execute = lifecycle(sqls, lengths, varchar_query, [varchar_text(n) for n in lengths], verdict, finding)
        structure = [
            verify("t exists", table_query, "1"),
            verify("columns stay id then c", columns_query, columns),
            verify("primary key stays id", pk_query, "id"),
            verify("final c definition", varchar_query, varchar_text(lengths[-1]))]
        return metadata_case(f"t05-a4-{anchor}-{suffix}", anchor, port, dialect, sqls, expect, execute, structure)

    metadata_cases = []
    for anchor, port in (("mysql57", 23357), ("mysql80", 23380), ("mysql84", 23384), ("tidb85", 24000)):
        dialect = "tidb" if anchor == "tidb85" else "mysql"
        metadata_cases.append(varchar_case(
            anchor, port, dialect, "narrow", [10, 20, 15], "reject", shrink(2, 20, 15), "id:int,c:varchar"))
        metadata_cases.append(varchar_case(
            anchor, port, dialect, "wide", [10, 20, 30], "pass", None, "id:int,c:varchar"))
        if anchor in ("mysql84", "tidb85"):
            metadata_cases.append(varchar_case(
                anchor, port, dialect, "narrow-then-18", [10, 20, 15, 18], "reject", shrink(2, 20, 15), "id:int,c:varchar"))
            metadata_cases.append(varchar_case(
                anchor, port, dialect, "wide-then-25", [10, 20, 30, 25], "reject", shrink(3, 30, 25), "id:int,c:varchar"))
            omit_sqls = ["CREATE TABLE t (id INT PRIMARY KEY, c INT UNSIGNED NOT NULL DEFAULT 1 COMMENT 'old')",
                         "ALTER TABLE t MODIFY COLUMN c INT",
                         "ALTER TABLE t MODIFY COLUMN c INT NOT NULL"]
            signed, signed_meta = attribute(1, {"source_unsigned": True, "target_unsigned": False})
            nullability, null_meta = attribute(2, {"source_not_null": False, "target_not_null": True})
            omit_expect, omit_execute = lifecycle(
                omit_sqls, None, int_query,
                ["int:unsigned:NO:1:old", "int:signed:YES:<nil>:", "int:signed:NO:<nil>:"],
                "reject", ([signed, nullability], [signed_meta, null_meta]))
            metadata_cases.append(metadata_case(
                f"t05-a4-{anchor}-omit-integer", anchor, port, dialect, omit_sqls, omit_expect, omit_execute, [
                    verify("t exists", table_query, "1"),
                    verify("columns stay id then c", columns_query, "id:int,c:int"),
                    verify("primary key stays id", pk_query, "id"),
                    verify("c lost unsigned default and comment", int_query, "int:signed:NO:<nil>:")]))
            pk_sqls = ["CREATE TABLE t (id INT PRIMARY KEY, c INT)",
                       "ALTER TABLE t MODIFY COLUMN id BIGINT",
                       "ALTER TABLE t MODIFY COLUMN id BIGINT NOT NULL"]
            pk_expect = expectation(pk_sqls, "pass", "complete", ["complete"] * 3)
            pk_execute = [
                {"name": "driver statement 0", "sql": pk_sqls[0], "expect_rc": 0,
                 "verify": [verify("id starts not null", id_query, "int:NO")]},
                {"name": "driver statement 1", "sql": pk_sqls[1], "expect_rc": 0,
                 "verify": [verify("widened primary key stays not null", id_query, "bigint:NO")]},
                {"name": "driver statement 2", "sql": pk_sqls[2], "expect_rc": 0,
                 "verify": [verify("explicit not null primary key stays not null", id_query, "bigint:NO"),
                            verify("unmodified c stays nullable int", plain_query, "int:YES")]}]
            metadata_cases.append(metadata_case(
                f"t05-a4-{anchor}-pk-not-null", anchor, port, dialect, pk_sqls, pk_expect, pk_execute, [
                    verify("t exists", table_query, "1"),
                    verify("columns stay id then c", columns_query, "id:bigint,c:int"),
                    verify("primary key stays id", pk_query, "id"),
                    verify("id remains not null bigint", id_query, "bigint:NO")]))
        if anchor == "tidb85":
            # The third statement is a product-audit successor only. The driver
            # stops after the documented ERROR 8200 and does not send it.
            neg_sqls = ["CREATE TABLE t (id INT PRIMARY KEY)",
                        "ALTER TABLE t MODIFY COLUMN id INT UNSIGNED",
                        "ALTER TABLE t MODIFY COLUMN id BIGINT UNSIGNED"]
            neg_meta = {"action": "modify_column", "table": "t", "name": "id", "column_name": "id",
                        "source_unsigned": False, "target_unsigned": True}
            neg_expect = expectation(
                neg_sqls, "reject", "unverified", ["complete", "complete", "unverified"], findings=1,
                gap_entries=[
                    {"index": 2, "rule_id": compat, "reason_code": "missing_source_column",
                     "required_facts": ["source_column.definition"]},
                    {"index": 2, "rule_id": "ddl.table.exists.alter.require",
                     "reason_code": "unknown_table_state", "required_facts": ["target_table.existence"]},
                    {"index": 2, "rule_id": "ddl.alter.modify_column.exists.require",
                     "reason_code": "unknown_table_state",
                     "required_facts": ["target_table.columns", "target_table.existence"]},
                ],
                finding_entries=[{"index": 1, "rule_id": compat, "level": "blocker"}],
                finding_metadata=[{"index": 1, "rule_id": compat, "metadata": neg_meta}])
            neg_execute = [
                {"name": "driver statement 0", "sql": neg_sqls[0], "expect_rc": 0,
                 "verify": [verify("id starts signed not null", id_signed_query, "int:signed:NO")]},
                {"name": "driver statement 1", "sql": neg_sqls[1], "expect_rc": 1,
                 "stderr_contains": ["ERROR 8200", "column has primary key flag"],
                 "verify": [verify("rejected signedness change leaves signed int", id_signed_query, "int:signed:NO"),
                            verify("primary key stays id after rejection", pk_query, "id")]}]
            metadata_cases.append(metadata_case(
                "t05-a4-tidb85-pk-unsigned", anchor, port, dialect, neg_sqls, neg_expect, neg_execute, [
                    verify("t exists", table_query, "1"),
                    verify("column stays id", columns_query, "id:int"),
                    verify("primary key stays id", pk_query, "id"),
                    verify("id stays signed not null", id_signed_query, "int:signed:NO")]))
    cli_cases = []
    gap = [{"index": 0, "rule_id": create_rule, "reason_code": "unknown_table_state",
            "required_facts": ["target_table.existence"]}]
    narrow = varchar_sqls([10, 20, 15])
    wide = varchar_sqls([10, 20, 30])
    narrow_entries, narrow_meta = shrink(2, 20, 15)
    cli_cases.append({"id": "t05-a4-mysql-narrow-offline", "dialect": "mysql",
                      "sql": " ".join(sql + ";" for sql in narrow), "policy": profile,
                      "args": ["--fail-on", "blocker"],
                      "expect": expectation(narrow, "reject", "unverified",
                                           ["unverified", "complete", "complete"], findings=1,
                                           gap_entries=gap, finding_entries=narrow_entries,
                                           finding_metadata=narrow_meta)})
    cli_cases.append({"id": "t05-a4-tidb-wide-offline", "dialect": "tidb",
                      "sql": " ".join(sql + ";" for sql in wide), "policy": profile,
                      "args": ["--fail-on", "blocker"],
                      "expect": expectation(wide, "review", "unverified",
                                           ["unverified", "complete", "complete"], gap_entries=gap)})
    return profile, {"enable": enable}, metadata_cases, cli_cases


def t05_a3_manifest_failures(manifest):
    if manifest.get("task_id") != "T05":
        return []
    profile, policy, metadata_cases, cli_cases = t05_a3_contract()
    failures = []
    if ((manifest.get("policy") or {}).get("profiles") or {}).get(profile) != policy:
        failures.append("T05-A3 frozen five-rule profile missing or changed")
    required = manifest.get("required_case_ids") or []
    for field, specs, kind in (("metadata_cases", metadata_cases, "meta"), ("cli_cases", cli_cases, "cli")):
        for wanted in specs:
            matches = [s for s in manifest.get(field, []) if s.get("id") == wanted["id"]]
            if len(matches) != 1 or matches[0] != wanted:
                failures.append(f"T05-A3 frozen oracle changed or missing: {wanted['id']}")
            if f"T05.{kind}.{wanted['id']}" not in required:
                failures.append(f"T05-A3 required case missing: {wanted['id']}")
    return failures


def t05_a4_manifest_failures(manifest):
    if manifest.get("task_id") != "T05":
        return []
    profile, policy, metadata_cases, cli_cases = t05_a4_contract()
    failures = []
    if ((manifest.get("policy") or {}).get("profiles") or {}).get(profile) != policy:
        failures.append("T05-A4 frozen four-rule profile missing or changed")
    required = manifest.get("required_case_ids") or []
    for field, specs, kind in (("metadata_cases", metadata_cases, "meta"), ("cli_cases", cli_cases, "cli")):
        for wanted in specs:
            matches = [s for s in manifest.get(field, []) if s.get("id") == wanted["id"]]
            if len(matches) != 1 or matches[0] != wanted:
                failures.append(f"T05-A4 frozen oracle changed or missing: {wanted['id']}")
            if f"T05.{kind}.{wanted['id']}" not in required:
                failures.append(f"T05-A4 required case missing: {wanted['id']}")
    return failures


def t05_a5_contract():
    """Frozen T05-A5 column-identity oracle. The 27 cases and their expects are
    independent of the manifest. The profile is the original eight blockers
    plus the two destination-name checks and the RENAME COLUMN version check."""
    profile = "t05-a5-column-identity-isolated"
    create_rule = "ddl.table.exists.create.forbid"
    alter_rule = "ddl.table.exists.alter.require"
    change_exists = "ddl.alter.change_column.exists.require"
    change_compat = "ddl.alter.change_column.compatibility.require"
    rename_exists = "ddl.alter.rename_column.exists.require"
    modify_exists = "ddl.alter.modify_column.exists.require"
    modify_compat = "ddl.alter.modify_column.compatibility.require"
    index_rule = "ddl.create_index.columns.exists.require"
    change_target = "ddl.alter.change_column.target.exists.forbid"
    rename_target = "ddl.alter.rename_column.target.exists.forbid"
    rename_version = "ddl.alter.rename_column.version.require"
    enable = {rid: {"enabled": True, "level": "blocker", "params": {}} for rid in (
        create_rule, alter_rule, change_exists, change_compat, rename_exists,
        modify_exists, modify_compat, index_rule, change_target, rename_target, rename_version)}
    for rid in (change_compat, modify_compat, index_rule, rename_version):
        enable[rid]["params"] = {"required": True}

    table_query = "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'"
    columns_query = ("SELECT GROUP_CONCAT(COLUMN_NAME ORDER BY ORDINAL_POSITION SEPARATOR ',') "
                     "FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'")

    def column_count(name):
        return ("SELECT COUNT(*) FROM information_schema.COLUMNS "
                f"WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND COLUMN_NAME='{name}'")

    def varchar_query(name):
        return ("SELECT CONCAT(DATA_TYPE,':',IFNULL(CHARACTER_MAXIMUM_LENGTH,''),':',IFNULL(CHARACTER_SET_NAME,''),':',"
                "IFNULL(COLLATION_NAME,''),':',IS_NULLABLE,':',IFNULL(COLUMN_DEFAULT,'<nil>'),':',IFNULL(COLUMN_COMMENT,'')) "
                f"FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND COLUMN_NAME='{name}'")

    def int_query(name):
        return ("SELECT CONCAT(DATA_TYPE,':',IS_NULLABLE) FROM information_schema.COLUMNS "
                f"WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND COLUMN_NAME='{name}'")

    def index_query(name):
        return ("SELECT GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') FROM information_schema.STATISTICS "
                f"WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND INDEX_NAME='{name}'")

    def index_count(name):
        return ("SELECT COUNT(*) FROM information_schema.STATISTICS "
                f"WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND INDEX_NAME='{name}'")

    def verify(label, sql, expect):
        return {"assert": label, "sql": sql, "expect": expect}

    def varchar_text(length):
        return f"varchar:{length}:utf8mb4:utf8mb4_bin:NO:<nil>:"

    def expectation(sqls, verdict, coverage, statement_coverage, findings=0, gap_entries=None,
                    finding_entries=None, finding_metadata=None):
        result = {
            "exit": 0 if verdict != "reject" else 1,
            "verdict": verdict,
            "statements": len(sqls),
            "findings": findings,
            "diagnostics": 0,
            "unsupported": 0,
            "coverage": coverage,
            "statement_coverage": statement_coverage,
            "statement_sql": [sql + ";" for sql in sqls],
            "evidence_gaps": len(gap_entries or []),
            "fail_on_triggered": verdict == "reject",
            "rule_summary_loaded": 11,
            "statement_indices": list(range(len(sqls))),
        }
        if finding_entries:
            result["finding_entries"] = finding_entries
        if finding_metadata:
            result["finding_metadata"] = finding_metadata
        if gap_entries:
            result["evidence_gap_entries"] = gap_entries
        return result

    def absent_setup():
        return [{"name": "ensure t absent", "sql": "DROP TABLE IF EXISTS t", "expect_rc": 0,
                 "verify": [verify("t absent before audit", table_query, "0")]}]

    def metadata_case(case_id, anchor, port, dialect, sqls, expect, setup, execute, structure, post_verify):
        connect = {"host": "127.0.0.1", "port": port, "user": "root", "schema": "golden"}
        if dialect == "mysql":
            connect.update(password_env="DS_T05_GOLDEN_PW", password="root")
        return {"id": case_id, "anchor": anchor, "dialect": dialect,
                "sql": " ".join(sql + ";" for sql in sqls), "policy": profile,
                "args": ["--fail-on", "blocker"], "connect": connect,
                "setup": setup, "expect": expect, "post_verify": post_verify,
                "execute": execute, "structure": structure,
                "teardown": [{"name": "drop fixture", "sql": "DROP TABLE IF EXISTS t", "expect_rc": 0,
                              "verify": [verify("no residual t", table_query, "0")]}]}

    create = "CREATE TABLE t (id INT PRIMARY KEY, c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL)"
    change = "ALTER TABLE t CHANGE COLUMN c c2 VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL"
    rename = "ALTER TABLE t RENAME COLUMN c TO c2"
    modify = "ALTER TABLE t MODIFY COLUMN c2 VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL"
    idx_new = "CREATE INDEX idx_c2 ON t(c2)"
    idx_old = "CREATE INDEX ix_old ON t(c)"

    def core(second, index_sql):
        return [create, second, modify, index_sql]

    def varchar_steps(sqls, failing_index=False):
        c_def = varchar_query("c")
        c2_def = varchar_query("c2")
        steps = [
            {"name": "driver statement 0", "sql": sqls[0], "expect_rc": 0, "verify": [
                verify("columns are id then c", columns_query, "id,c"),
                verify("c starts at length 10", c_def, varchar_text(10)),
                verify("c2 absent before the rename", column_count("c2"), "0")]},
            {"name": "driver statement 1", "sql": sqls[1], "expect_rc": 0, "verify": [
                verify("columns are id then c2", columns_query, "id,c2"),
                verify("c2 length is 10", c2_def, varchar_text(10)),
                verify("c absent after the rename", column_count("c"), "0")]},
            {"name": "driver statement 2", "sql": sqls[2], "expect_rc": 0, "verify": [
                verify("c2 length is 20", c2_def, varchar_text(20)),
                verify("c stays absent after modify", column_count("c"), "0")]},
        ]
        if failing_index:
            steps.append({"name": "driver statement 3", "sql": sqls[3], "expect_rc": 1,
                          "stderr_contains": ["ERROR 1072"], "verify": [
                              verify("rejected index leaves c absent", column_count("c"), "0"),
                              verify("rejected index leaves c2 at length 20", c2_def, varchar_text(20))]})
        else:
            steps.append({"name": "driver statement 3", "sql": sqls[3], "expect_rc": 0, "verify": [
                verify("idx_c2 references c2", index_query("idx_c2"), "c2")]})
        return steps

    def varchar_structure(index_name):
        checks = [
            verify("t exists", table_query, "1"),
            verify("columns are id then c2", columns_query, "id,c2"),
            verify("c absent", column_count("c"), "0"),
            verify("c2 length is 20", varchar_query("c2"), varchar_text(20)),
            verify("primary key stays id", index_query("PRIMARY"), "id"),
        ]
        if index_name == "idx_c2":
            checks.append(verify("idx_c2 references c2", index_query("idx_c2"), "c2"))
        else:
            checks.append(verify("ix_old was not created", index_count("ix_old"), "0"))
        return checks

    def old_index_expect(sqls):
        metadata = {"table": "t", "index": "ix_old", "column": "c", "exists": False}
        return expectation(
            sqls, "reject", "complete", ["complete"] * 4, findings=1,
            finding_entries=[{"index": 3, "rule_id": index_rule, "level": "blocker"}],
            finding_metadata=[{"index": 3, "rule_id": index_rule, "metadata": metadata}])

    def rebind_steps(sqls):
        return [
            {"name": "driver statement 0", "sql": sqls[0], "expect_rc": 0, "verify": [
                verify("columns are c then k", columns_query, "c,k"),
                verify("primary key is c", index_query("PRIMARY"), "c"),
                verify("idx_c references c", index_query("idx_c"), "c"),
                verify("uk_c_k references c then k", index_query("uk_c_k"), "c,k"),
                verify("c is not null", int_query("c"), "int:NO"),
                verify("k stays nullable", int_query("k"), "int:YES")]},
            {"name": "driver statement 1", "sql": sqls[1], "expect_rc": 0, "verify": [
                verify("columns are c2 then k", columns_query, "c2,k"),
                verify("primary key moved to c2", index_query("PRIMARY"), "c2"),
                verify("idx_c name stays and references c2", index_query("idx_c"), "c2"),
                verify("uk_c_k keeps c2 then k", index_query("uk_c_k"), "c2,k"),
                verify("c2 stays not null", int_query("c2"), "int:NO"),
                verify("old c is gone", column_count("c"), "0"),
                verify("k is unchanged", int_query("k"), "int:YES")]},
            {"name": "driver statement 2", "sql": sqls[2], "expect_rc": 0, "verify": [
                verify("c2 widened and stays not null", int_query("c2"), "bigint:NO")]},
            {"name": "driver statement 3", "sql": sqls[3], "expect_rc": 0, "verify": [
                verify("ix_new references c2", index_query("ix_new"), "c2")]},
        ]

    def rebind_structure():
        return [
            verify("t exists", table_query, "1"),
            verify("columns are c2 then k", columns_query, "c2,k"),
            verify("primary key is c2", index_query("PRIMARY"), "c2"),
            verify("idx_c still named idx_c and references c2", index_query("idx_c"), "c2"),
            verify("uk_c_k keeps order c2,k", index_query("uk_c_k"), "c2,k"),
            verify("c2 is not null bigint", int_query("c2"), "bigint:NO"),
            verify("k stays nullable int", int_query("k"), "int:YES"),
            verify("ix_new references c2", index_query("ix_new"), "c2"),
            verify("old c is absent", column_count("c"), "0"),
        ]

    def conflict_case(case_id, anchor, port, dialect, sql, rule_id, action):
        sqls = [sql]
        metadata = {"table": "t", "action": action, "source_column": "c", "target_column": "c2", "exists": True}
        expect = expectation(
            sqls, "reject", "complete", ["complete"], findings=1,
            finding_entries=[{"index": 0, "rule_id": rule_id, "level": "blocker"}],
            finding_metadata=[{"index": 0, "rule_id": rule_id, "metadata": metadata}])
        setup = [
            {"name": "drop t", "sql": "DROP TABLE IF EXISTS t", "expect_rc": 0},
            {"name": "create both columns", "sql": "CREATE TABLE t (c INT, c2 INT)", "expect_rc": 0,
             "verify": [verify("both columns exist before audit", columns_query, "c,c2")]},
        ]
        execute = [{"name": "driver statement 0", "sql": sql, "expect_rc": 1,
                    "stderr_contains": ["ERROR 1060", "Duplicate column"],
                    "verify": [verify("rejected rename leaves both columns", columns_query, "c,c2")]}]
        structure = [
            verify("t still exists", table_query, "1"),
            verify("columns stay c then c2", columns_query, "c,c2"),
            verify("c remains", column_count("c"), "1"),
            verify("c2 remains", column_count("c2"), "1"),
        ]
        return metadata_case(case_id, anchor, port, dialect, sqls, expect, setup, execute, structure,
                             [verify("audit did not rename a column", columns_query, "c,c2")])

    metadata_cases = []
    for anchor, port in (("mysql57", 23357), ("mysql80", 23380), ("mysql84", 23384), ("tidb85", 24000)):
        dialect = "tidb" if anchor == "tidb85" else "mysql"
        changed = core(change, idx_new)
        metadata_cases.append(metadata_case(
            f"t05-a5-{anchor}-change", anchor, port, dialect, changed,
            expectation(changed, "pass", "complete", ["complete"] * 4),
            absent_setup(), varchar_steps(changed), varchar_structure("idx_c2"),
            [verify("audit did not create t", table_query, "0")]))
        old = core(change, idx_old)
        metadata_cases.append(metadata_case(
            f"t05-a5-{anchor}-change-old-index", anchor, port, dialect, old, old_index_expect(old),
            absent_setup(), varchar_steps(old, failing_index=True), varchar_structure("ix_old"),
            [verify("audit did not create t", table_query, "0")]))
    for anchor, port in (("mysql80", 23380), ("mysql84", 23384), ("tidb85", 24000)):
        dialect = "tidb" if anchor == "tidb85" else "mysql"
        renamed = core(rename, idx_new)
        metadata_cases.append(metadata_case(
            f"t05-a5-{anchor}-rename", anchor, port, dialect, renamed,
            expectation(renamed, "pass", "complete", ["complete"] * 4),
            absent_setup(), varchar_steps(renamed), varchar_structure("idx_c2"),
            [verify("audit did not create t", table_query, "0")]))
        old = core(rename, idx_old)
        metadata_cases.append(metadata_case(
            f"t05-a5-{anchor}-rename-old-index", anchor, port, dialect, old, old_index_expect(old),
            absent_setup(), varchar_steps(old, failing_index=True), varchar_structure("ix_old"),
            [verify("audit did not create t", table_query, "0")]))
    for anchor, port in (("mysql84", 23384), ("tidb85", 24000)):
        dialect = "tidb" if anchor == "tidb85" else "mysql"
        for action, second in (("change", "ALTER TABLE t CHANGE COLUMN c c2 INT"),
                               ("rename", "ALTER TABLE t RENAME COLUMN c TO c2")):
            sqls = [
                "CREATE TABLE t (c INT PRIMARY KEY, k INT, KEY idx_c(c), UNIQUE KEY uk_c_k(c, k))",
                second,
                "ALTER TABLE t MODIFY COLUMN c2 BIGINT NOT NULL",
                "CREATE INDEX ix_new ON t(c2)",
            ]
            metadata_cases.append(metadata_case(
                f"t05-a5-{anchor}-{action}-rebind", anchor, port, dialect, sqls,
                expectation(sqls, "pass", "complete", ["complete"] * 4),
                absent_setup(), rebind_steps(sqls), rebind_structure(),
                [verify("audit did not create t", table_query, "0")]))
        metadata_cases.append(conflict_case(
            f"t05-a5-{anchor}-change-conflict", anchor, port, dialect,
            "ALTER TABLE t CHANGE COLUMN c c2 INT", change_target, "change_column"))
        metadata_cases.append(conflict_case(
            f"t05-a5-{anchor}-rename-conflict", anchor, port, dialect,
            "ALTER TABLE t RENAME COLUMN c TO c2", rename_target, "rename_column"))

    version_sqls = ["ALTER TABLE t RENAME COLUMN c TO c2"]
    version_meta = {"action": "rename_column", "product": "mysql", "target_version": "5.7.44",
                    "minimum_supported_version": "8.0.3"}
    version_expect = expectation(
        version_sqls, "reject", "complete", ["complete"], findings=1,
        finding_entries=[{"index": 0, "rule_id": rename_version, "level": "blocker"}],
        finding_metadata=[{"index": 0, "rule_id": rename_version, "metadata": version_meta}])
    version_setup = [
        {"name": "drop t", "sql": "DROP TABLE IF EXISTS t", "expect_rc": 0},
        {"name": "create source column", "sql": "CREATE TABLE t (c INT)", "expect_rc": 0,
         "verify": [verify("c exists before audit", int_query("c"), "int:YES")]},
    ]
    version_execute = [{"name": "driver statement 0", "sql": version_sqls[0], "expect_rc": 1,
                        "stderr_contains": ["ERROR 1064"], "verify": [
                            verify("native rejection leaves c", int_query("c"), "int:YES"),
                            verify("native rejection does not create c2", column_count("c2"), "0")]}]
    metadata_cases.append(metadata_case(
        "t05-a5-mysql57-rename-version", "mysql57", 23357, "mysql", version_sqls, version_expect,
        version_setup, version_execute, [
            verify("t still exists", table_query, "1"),
            verify("column stays c", columns_query, "c"),
            verify("c2 was not created", column_count("c2"), "0"),
        ], [verify("audit did not rename c", columns_query, "c")]))

    create_gap = {"index": 0, "rule_id": create_rule, "reason_code": "unknown_table_state",
                  "required_facts": ["target_table.existence"]}

    def offline_rename(reason, facts):
        return [
            create_gap,
            {"index": 1, "rule_id": rename_version, "reason_code": reason, "required_facts": [facts]},
            {"index": 2, "rule_id": modify_compat, "reason_code": "missing_source_column",
             "required_facts": ["source_column.definition"]},
            {"index": 2, "rule_id": alter_rule, "reason_code": "unknown_table_state",
             "required_facts": ["target_table.existence"]},
            {"index": 2, "rule_id": modify_exists, "reason_code": "unknown_table_state",
             "required_facts": ["target_table.columns", "target_table.existence"]},
            {"index": 3, "rule_id": index_rule, "reason_code": "unknown_table_state",
             "required_facts": ["target_table.columns", "target_table.existence"]},
        ]

    renamed = core(rename, idx_new)
    changed = core(change, idx_new)
    cli_cases = [
        {"id": "t05-a5-mysql-rename-offline", "dialect": "mysql",
         "sql": " ".join(sql + ";" for sql in renamed), "policy": profile,
         "args": ["--fail-on", "blocker"],
         "expect": expectation(renamed, "review", "unverified", ["unverified"] * 4,
                               gap_entries=offline_rename("missing_target_version", "target.version"))},
        {"id": "t05-a5-mysql-rename-offline-900", "dialect": "mysql",
         "sql": " ".join(sql + ";" for sql in renamed), "policy": profile,
         "args": ["--fail-on", "blocker", "--target-version", "9.0.0"],
         "expect": expectation(renamed, "review", "unverified", ["unverified"] * 4,
                               gap_entries=offline_rename("target_version_out_of_validated_range",
                                                          "target.version.validated_range"))},
        {"id": "t05-a5-mysql-change-offline", "dialect": "mysql",
         "sql": " ".join(sql + ";" for sql in changed), "policy": profile,
         "args": ["--fail-on", "blocker"],
         "expect": expectation(changed, "review", "unverified",
                               ["unverified", "complete", "complete", "complete"], gap_entries=[create_gap])},
        {"id": "t05-a5-tidb-change-offline", "dialect": "tidb",
         "sql": " ".join(sql + ";" for sql in changed), "policy": profile,
         "args": ["--fail-on", "blocker"],
         "expect": expectation(changed, "review", "unverified",
                               ["unverified", "complete", "complete", "complete"], gap_entries=[create_gap])},
    ]
    return profile, {"enable": enable}, metadata_cases, cli_cases


def t05_a5_manifest_failures(manifest):
    if manifest.get("task_id") != "T05":
        return []
    profile, policy, metadata_cases, cli_cases = t05_a5_contract()
    failures = []
    if ((manifest.get("policy") or {}).get("profiles") or {}).get(profile) != policy:
        failures.append("T05-A5 frozen eleven-rule profile missing or changed")
    required = manifest.get("required_case_ids") or []
    for field, specs, kind in (("metadata_cases", metadata_cases, "meta"), ("cli_cases", cli_cases, "cli")):
        for wanted in specs:
            matches = [s for s in manifest.get(field, []) if s.get("id") == wanted["id"]]
            if len(matches) != 1 or matches[0] != wanted:
                failures.append(f"T05-A5 frozen oracle changed or missing: {wanted['id']}")
            if f"T05.{kind}.{wanted['id']}" not in required:
                failures.append(f"T05-A5 required case missing: {wanted['id']}")
    return failures


def t05_a6_contract():
    """Frozen T05-A6 dependency-free DROP COLUMN oracle. The 16 cases and their
    expects are independent of the manifest. The profile is six existing blockers."""
    profile = "t05-a6-drop-column-isolated"
    create_rule = "ddl.table.exists.create.forbid"
    alter_rule = "ddl.table.exists.alter.require"
    drop_exists = "ddl.alter.drop_column.exists.require"
    modify_exists = "ddl.alter.modify_column.exists.require"
    modify_compat = "ddl.alter.modify_column.compatibility.require"
    index_rule = "ddl.create_index.columns.exists.require"
    enable = {rid: {"enabled": True, "level": "blocker", "params": {}} for rid in (
        create_rule, alter_rule, drop_exists, modify_exists, modify_compat, index_rule)}
    for rid in (modify_compat, index_rule):
        enable[rid]["params"] = {"required": True}

    table_query = "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'"
    columns_query = ("SELECT GROUP_CONCAT(COLUMN_NAME ORDER BY ORDINAL_POSITION SEPARATOR ',') "
                     "FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'")

    def column_count(name):
        return ("SELECT COUNT(*) FROM information_schema.COLUMNS "
                f"WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND COLUMN_NAME='{name}'")

    def varchar_query(name):
        return ("SELECT CONCAT(DATA_TYPE,':',IFNULL(CHARACTER_MAXIMUM_LENGTH,''),':',IFNULL(CHARACTER_SET_NAME,''),':',"
                "IFNULL(COLLATION_NAME,''),':',IS_NULLABLE,':',IFNULL(COLUMN_DEFAULT,'<nil>'),':',IFNULL(COLUMN_COMMENT,'')) "
                f"FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND COLUMN_NAME='{name}'")

    def definition_query(name):
        return ("SELECT CONCAT(DATA_TYPE,':',IF(COLUMN_TYPE LIKE '%unsigned%',1,0),':',"
                "IFNULL(COLUMN_DEFAULT,'<nil>'),':',IFNULL(COLUMN_COMMENT,'')) "
                f"FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND COLUMN_NAME='{name}'")

    def index_query(name):
        return ("SELECT GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') FROM information_schema.STATISTICS "
                f"WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND INDEX_NAME='{name}'")

    def index_count(name):
        return ("SELECT COUNT(*) FROM information_schema.STATISTICS "
                f"WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND INDEX_NAME='{name}'")

    def verify(label, sql, expect):
        return {"assert": label, "sql": sql, "expect": expect}

    def varchar_text(length):
        return f"varchar:{length}:utf8mb4:utf8mb4_bin:NO:<nil>:"

    def expectation(sqls, verdict, coverage, statement_coverage, findings=0, gap_entries=None,
                    finding_entries=None, finding_metadata=None):
        result = {
            "exit": 0 if verdict != "reject" else 1,
            "verdict": verdict,
            "statements": len(sqls),
            "findings": findings,
            "diagnostics": 0,
            "unsupported": 0,
            "coverage": coverage,
            "statement_coverage": statement_coverage,
            "statement_sql": [sql + ";" for sql in sqls],
            "evidence_gaps": len(gap_entries or []),
            "fail_on_triggered": verdict == "reject",
            "rule_summary_loaded": 6,
            "statement_indices": list(range(len(sqls))),
        }
        if finding_entries:
            result["finding_entries"] = finding_entries
        if finding_metadata:
            result["finding_metadata"] = finding_metadata
        if gap_entries:
            result["evidence_gap_entries"] = gap_entries
        return result

    def absent_setup():
        return [{"name": "ensure t absent", "sql": "DROP TABLE IF EXISTS t", "expect_rc": 0,
                 "verify": [verify("t absent before audit", table_query, "0")]}]

    def metadata_case(case_id, anchor, port, dialect, sqls, expect, execute, structure):
        connect = {"host": "127.0.0.1", "port": port, "user": "root", "schema": "golden"}
        if dialect == "mysql":
            connect.update(password_env="DS_T05_GOLDEN_PW", password="root")
        return {"id": case_id, "anchor": anchor, "dialect": dialect,
                "sql": " ".join(sql + ";" for sql in sqls), "policy": profile,
                "args": ["--fail-on", "blocker"], "connect": connect,
                "setup": absent_setup(), "expect": expect,
                "post_verify": [verify("audit did not create t", table_query, "0")],
                "execute": execute, "structure": structure,
                "teardown": [{"name": "drop fixture", "sql": "DROP TABLE IF EXISTS t", "expect_rc": 0,
                              "verify": [verify("no residual t", table_query, "0")]}]}

    keep10 = "keep_c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL"
    keep20 = "ALTER TABLE t MODIFY COLUMN keep_c VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL"
    drop_obsolete = "ALTER TABLE t DROP COLUMN obsolete"
    create_plain = f"CREATE TABLE t (id INT PRIMARY KEY, obsolete INT, {keep10})"
    create_indexed = (f"CREATE TABLE t (id INT PRIMARY KEY, obsolete INT, {keep10}, "
                      "KEY idx_existing(keep_c), UNIQUE KEY uk_id_keep(id, keep_c))")
    create_readd = ("CREATE TABLE t (id INT PRIMARY KEY, obsolete INT UNSIGNED NOT NULL DEFAULT 7 "
                    f"COMMENT 'retired', {keep10})")
    idx_keep = "CREATE INDEX idx_keep ON t(keep_c)"
    idx_removed = "CREATE INDEX ix_removed ON t(obsolete)"
    add_new = "ALTER TABLE t ADD COLUMN obsolete VARCHAR(12) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL"
    modify_new = "ALTER TABLE t MODIFY COLUMN obsolete VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL"
    idx_readded = "CREATE INDEX idx_readded ON t(obsolete)"
    keep_def = varchar_query("keep_c")
    obsolete_def = definition_query("obsolete")
    obsolete_varchar = varchar_query("obsolete")

    def base_steps(sqls, indexed=False, failing_index=False):
        drop_verify = [
            verify("columns are id then keep_c", columns_query, "id,keep_c"),
            verify("obsolete absent after drop", column_count("obsolete"), "0"),
            verify("keep_c stays length 10", keep_def, varchar_text(10)),
            verify("primary key stays id", index_query("PRIMARY"), "id"),
        ]
        modify_verify = [
            verify("keep_c length is 20", keep_def, varchar_text(20)),
            verify("obsolete stays absent after modify", column_count("obsolete"), "0"),
            verify("columns stay id then keep_c", columns_query, "id,keep_c"),
            verify("primary key stays id after modify", index_query("PRIMARY"), "id"),
        ]
        if indexed:
            drop_verify.extend([
                verify("idx_existing stays keep_c", index_query("idx_existing"), "keep_c"),
                verify("uk_id_keep stays id then keep_c", index_query("uk_id_keep"), "id,keep_c"),
            ])
            modify_verify.extend([
                verify("idx_existing stays keep_c after modify", index_query("idx_existing"), "keep_c"),
                verify("uk_id_keep stays id then keep_c after modify", index_query("uk_id_keep"), "id,keep_c"),
            ])
        steps = [
            {"name": "driver statement 0", "sql": sqls[0], "expect_rc": 0, "verify": [
                verify("columns are id, obsolete, keep_c", columns_query, "id,obsolete,keep_c"),
                verify("keep_c starts at length 10", keep_def, varchar_text(10)),
                verify("obsolete is present", column_count("obsolete"), "1"),
                verify("primary key is id", index_query("PRIMARY"), "id")]},
            {"name": "driver statement 1", "sql": sqls[1], "expect_rc": 0, "verify": drop_verify},
            {"name": "driver statement 2", "sql": sqls[2], "expect_rc": 0, "verify": modify_verify},
        ]
        if failing_index:
            steps.append({"name": "driver statement 3", "sql": sqls[3], "expect_rc": 1,
                          "stderr_contains": ["ERROR 1072", "obsolete"], "verify": [
                              verify("rejected index leaves obsolete absent", column_count("obsolete"), "0"),
                              verify("rejected index leaves keep_c at length 20", keep_def, varchar_text(20)),
                              verify("ix_removed was not created", index_count("ix_removed"), "0"),
                              verify("columns stay id then keep_c after rejection", columns_query, "id,keep_c")]})
        else:
            steps.append({"name": "driver statement 3", "sql": sqls[3], "expect_rc": 0, "verify": [
                verify("idx_keep references keep_c", index_query("idx_keep"), "keep_c")]})
        return steps

    def base_structure(index_name, indexed=False):
        checks = [
            verify("t exists", table_query, "1"),
            verify("columns are id then keep_c", columns_query, "id,keep_c"),
            verify("obsolete absent", column_count("obsolete"), "0"),
            verify("keep_c length is 20", keep_def, varchar_text(20)),
            verify("primary key stays id", index_query("PRIMARY"), "id"),
        ]
        if indexed:
            checks.extend([
                verify("idx_existing stays keep_c", index_query("idx_existing"), "keep_c"),
                verify("uk_id_keep stays id then keep_c", index_query("uk_id_keep"), "id,keep_c"),
            ])
        if index_name == "idx_keep":
            checks.append(verify("idx_keep references keep_c", index_query("idx_keep"), "keep_c"))
        else:
            checks.append(verify("ix_removed was not created", index_count("ix_removed"), "0"))
        return checks

    def old_index_expect(sqls, schema):
        metadata = {"schema": schema, "table": "t", "index": "ix_removed", "column": "obsolete", "exists": False}
        return expectation(
            sqls, "reject", "complete", ["complete"] * 4, findings=1,
            finding_entries=[{"index": 3, "rule_id": index_rule, "level": "blocker"}],
            finding_metadata=[{"index": 3, "rule_id": index_rule, "metadata": metadata}])

    def readd_steps(sqls):
        return [
            {"name": "driver statement 0", "sql": sqls[0], "expect_rc": 0, "verify": [
                verify("columns are id, obsolete, keep_c", columns_query, "id,obsolete,keep_c"),
                verify("obsolete starts unsigned with default 7", obsolete_def, "int:1:7:retired"),
                verify("keep_c starts at length 10", keep_def, varchar_text(10)),
                verify("primary key is id", index_query("PRIMARY"), "id")]},
            {"name": "driver statement 1", "sql": sqls[1], "expect_rc": 0, "verify": [
                verify("columns are id then keep_c", columns_query, "id,keep_c"),
                verify("obsolete absent after drop", column_count("obsolete"), "0"),
                verify("keep_c stays length 10", keep_def, varchar_text(10)),
                verify("primary key stays id", index_query("PRIMARY"), "id")]},
            {"name": "driver statement 2", "sql": sqls[2], "expect_rc": 0, "verify": [
                verify("columns are id, keep_c, obsolete", columns_query, "id,keep_c,obsolete"),
                verify("readded obsolete drops the old definition", obsolete_def, "varchar:0:<nil>:"),
                verify("readded obsolete length is 12", obsolete_varchar, varchar_text(12)),
                verify("keep_c stays length 10 after readd", keep_def, varchar_text(10))]},
            {"name": "driver statement 3", "sql": sqls[3], "expect_rc": 0, "verify": [
                verify("readded obsolete length is 20", obsolete_varchar, varchar_text(20)),
                verify("readded obsolete keeps the new definition", obsolete_def, "varchar:0:<nil>:"),
                verify("columns stay id, keep_c, obsolete", columns_query, "id,keep_c,obsolete")]},
            {"name": "driver statement 4", "sql": sqls[4], "expect_rc": 0, "verify": [
                verify("idx_readded references the new obsolete", index_query("idx_readded"), "obsolete")]},
        ]

    def readd_structure():
        return [
            verify("t exists", table_query, "1"),
            verify("columns are id, keep_c, obsolete", columns_query, "id,keep_c,obsolete"),
            verify("obsolete length is 20", obsolete_varchar, varchar_text(20)),
            verify("obsolete keeps the new definition", obsolete_def, "varchar:0:<nil>:"),
            verify("primary key stays id", index_query("PRIMARY"), "id"),
            verify("idx_readded references obsolete", index_query("idx_readded"), "obsolete"),
            verify("keep_c stays length 10", keep_def, varchar_text(10)),
        ]

    metadata_cases = []
    for anchor, port in (("mysql57", 23357), ("mysql80", 23380), ("mysql84", 23384), ("tidb85", 24000)):
        dialect = "tidb" if anchor == "tidb85" else "mysql"
        positive = [create_plain, drop_obsolete, keep20, idx_keep]
        metadata_cases.append(metadata_case(
            f"t05-a6-{anchor}-drop", anchor, port, dialect, positive,
            expectation(positive, "pass", "complete", ["complete"] * 4),
            base_steps(positive), base_structure("idx_keep")))
        negative = [create_plain, drop_obsolete, keep20, idx_removed]
        metadata_cases.append(metadata_case(
            f"t05-a6-{anchor}-drop-old-index", anchor, port, dialect, negative,
            old_index_expect(negative, "golden"),
            base_steps(negative, failing_index=True), base_structure("ix_removed")))
    for anchor, port in (("mysql84", 23384), ("tidb85", 24000)):
        dialect = "tidb" if anchor == "tidb85" else "mysql"
        indexed = [create_indexed, drop_obsolete, keep20, idx_keep]
        metadata_cases.append(metadata_case(
            f"t05-a6-{anchor}-unrelated-index", anchor, port, dialect, indexed,
            expectation(indexed, "pass", "complete", ["complete"] * 4),
            base_steps(indexed, indexed=True), base_structure("idx_keep", indexed=True)))
        readded = [create_readd, drop_obsolete, add_new, modify_new, idx_readded]
        metadata_cases.append(metadata_case(
            f"t05-a6-{anchor}-readd", anchor, port, dialect, readded,
            expectation(readded, "pass", "complete", ["complete"] * 5),
            readd_steps(readded), readd_structure()))

    create_gap = {"index": 0, "rule_id": create_rule, "reason_code": "unknown_table_state",
                  "required_facts": ["target_table.existence"]}
    positive = [create_plain, drop_obsolete, keep20, idx_keep]
    negative = [create_plain, drop_obsolete, keep20, idx_removed]
    offline_meta = {"schema": "", "table": "t", "index": "ix_removed", "column": "obsolete", "exists": False}
    offline_negative = expectation(
        negative, "reject", "unverified", ["unverified", "complete", "complete", "complete"], findings=1,
        gap_entries=[create_gap],
        finding_entries=[{"index": 3, "rule_id": index_rule, "level": "blocker"}],
        finding_metadata=[{"index": 3, "rule_id": index_rule, "metadata": offline_meta}])

    def cli_case(case_id, dialect, sqls, expect):
        return {"id": case_id, "dialect": dialect, "sql": " ".join(sql + ";" for sql in sqls),
                "policy": profile, "args": ["--fail-on", "blocker"], "expect": expect}

    cli_cases = [
        cli_case("t05-a6-mysql-drop-offline", "mysql", positive,
                 expectation(positive, "review", "unverified",
                             ["unverified", "complete", "complete", "complete"], gap_entries=[create_gap])),
        cli_case("t05-a6-mysql-drop-old-index-offline", "mysql", negative, offline_negative),
        cli_case("t05-a6-tidb-drop-offline", "tidb", positive,
                 expectation(positive, "review", "unverified",
                             ["unverified", "complete", "complete", "complete"], gap_entries=[create_gap])),
        cli_case("t05-a6-tidb-drop-old-index-offline", "tidb", negative, offline_negative),
    ]
    return profile, {"enable": enable}, metadata_cases, cli_cases


def t05_a6_manifest_failures(manifest):
    if manifest.get("task_id") != "T05":
        return []
    profile, policy, metadata_cases, cli_cases = t05_a6_contract()
    failures = []
    if ((manifest.get("policy") or {}).get("profiles") or {}).get(profile) != policy:
        failures.append("T05-A6 frozen six-rule profile missing or changed")
    required = manifest.get("required_case_ids") or []
    for field, specs, kind in (("metadata_cases", metadata_cases, "meta"), ("cli_cases", cli_cases, "cli")):
        for wanted in specs:
            matches = [s for s in manifest.get(field, []) if s.get("id") == wanted["id"]]
            if len(matches) != 1 or matches[0] != wanted:
                failures.append(f"T05-A6 frozen oracle changed or missing: {wanted['id']}")
            if f"T05.{kind}.{wanted['id']}" not in required:
                failures.append(f"T05-A6 required case missing: {wanted['id']}")
    return failures


# ---------------------------------------------------------------------------
# T06-A1 frozen oracle (issue #85): CREATE TABLE primary-key presence.
# The 32 required cases, the isolated policy profiles, and the structure
# queries are hardcoded here — independent of the manifest — so deleting or
# rewriting a case, an input SQL, a profile, or a required structure check on
# the manifest and artifact sides at once still fails validation.
# ---------------------------------------------------------------------------

T06_PK_RULE = "ddl.table.primary_key.require"
T06_PK_NN_RULE = "ddl.table.primary_key.not_null.require"
T06_DEFAULT_RULE = "ddl.column.default.require"
T06_ISOLATED_PROFILE = "t06-pk-presence-isolated"
T06_REQUIRED_FALSE_PROFILE = "t06-pk-required-false"
T06_A2_PK_NULL_PROFILE = "t06-a2-pk-nullability-isolated"
T06_A2_DEFAULT_PROFILE = "t06-a2-default-presence-isolated"
T06_NO_PK_SQL = "CREATE TABLE t (id INT);"
T06_INLINE_PK_SQL = "CREATE TABLE t (id INT PRIMARY KEY);"
T06_TABLE_PK_SQL = "CREATE TABLE t (id INT, PRIMARY KEY (id));"
T06_A2_COMPOSITE_PK_SQL = "CREATE TABLE t (a INT, spare INT, b INT, PRIMARY KEY (b,a));"
T06_A2_NULL_TABLE_SQL = "CREATE TABLE t (id INT NULL, PRIMARY KEY (id));"
T06_A2_NULL_INLINE_SQL = "CREATE TABLE t (id INT NULL PRIMARY KEY);"
T06_A2_NO_DEFAULT_SQL = "CREATE TABLE t (c INT);"
T06_A2_DEFAULT_NULL_SQL = "CREATE TABLE t (c INT DEFAULT NULL);"
T06_BASELINE_SQL = (
    "CREATE TABLE golden_t (id INT PRIMARY KEY); "
    "ALTER TABLE golden_t ADD COLUMN c INT; DROP TABLE golden_t;"
)
T06_BASELINE_STATEMENTS = [
    "CREATE TABLE golden_t (id INT PRIMARY KEY);",
    "ALTER TABLE golden_t ADD COLUMN c INT;",
    "DROP TABLE golden_t;",
]

T06_TABLE_COUNT = (
    "SELECT COUNT(*) FROM information_schema.TABLES "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'"
)
T06_COLUMN_COUNT = (
    "SELECT COUNT(*) FROM information_schema.COLUMNS "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'"
)
T06_COLUMN_ROW = (
    "SELECT CONCAT_WS(':', COLUMN_NAME, DATA_TYPE, ORDINAL_POSITION, IS_NULLABLE) "
    "FROM information_schema.COLUMNS "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' ORDER BY ORDINAL_POSITION"
)
T06_PK_CONSTRAINT = (
    "SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND CONSTRAINT_TYPE='PRIMARY KEY'"
)
T06_PK_PARTS = (
    "SELECT COUNT(*) FROM information_schema.STATISTICS "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND INDEX_NAME='PRIMARY'"
)
T06_PK_MEMBER = (
    "SELECT CONCAT_WS(':', COLUMN_NAME, SEQ_IN_INDEX) FROM information_schema.STATISTICS "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND INDEX_NAME='PRIMARY' "
    "ORDER BY SEQ_IN_INDEX"
)
T06_ENGINE = (
    "SELECT ENGINE FROM information_schema.TABLES "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'"
)
# Ordered single-row shapes keep multi-column/multi-member assertions scalar.
T06_A2_COLUMN_ROWS = (
    "SELECT GROUP_CONCAT(CONCAT_WS(':', COLUMN_NAME, DATA_TYPE, ORDINAL_POSITION, IS_NULLABLE) "
    "ORDER BY ORDINAL_POSITION) FROM information_schema.COLUMNS "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'"
)
T06_A2_PK_MEMBERS = (
    "SELECT GROUP_CONCAT(CONCAT_WS(':', COLUMN_NAME, SEQ_IN_INDEX) "
    "ORDER BY SEQ_IN_INDEX) FROM information_schema.STATISTICS "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND INDEX_NAME='PRIMARY'"
)
# MySQL 8.0/8.4 only: the server could add an invisible PK silently. Recording
# these variables proves none was generated; a generated GIPK would also be
# caught by the structure oracle (id would not be the sole column member).
T06_GIPK_FACTS = {
    "sql_require_primary_key": "OFF",
    "sql_generate_invisible_primary_key": "OFF",
    "show_gipk_in_create_table_and_information_schema": "ON",
}

# T06-A3 (issue #85): SQL DEFAULT NULL typed-identity fix. The CLI matrix
# reuses the A2 default-presence profile; the three-statement drop path gets
# its own isolated four-rule profile so no other rule can mask the seam.
T06_A3_DROP_PROFILE = "t06-a3-null-drop-state-isolated"
T06_A3_NO_DEFAULT_SQL = "CREATE TABLE t (c VARCHAR(8));"
T06_A3_SQL_NULL_SQL = "CREATE TABLE t (c VARCHAR(8) DEFAULT NULL);"
T06_A3_TEXT_NULL_SQL = "CREATE TABLE t (c VARCHAR(8) DEFAULT 'NULL');"
T06_A3_TEXT_NIL_SQL = "CREATE TABLE t (c VARCHAR(8) DEFAULT '<nil>');"
T06_A3_MATRIX_SQL = (
    "CREATE TABLE t (a VARCHAR(8), b VARCHAR(8) DEFAULT NULL, "
    "c VARCHAR(8) DEFAULT 'NULL', d VARCHAR(8) DEFAULT '<nil>');"
)
T06_A3_DROP_STATEMENTS = [
    "CREATE TABLE t (id INT PRIMARY KEY, obsolete INT, keep_c VARCHAR(8) DEFAULT NULL);",
    "ALTER TABLE t DROP COLUMN obsolete;",
    "CREATE INDEX idx_keep ON t(keep_c);",
]
T06_A3_DROP_STATE_SQL = " ".join(T06_A3_DROP_STATEMENTS)
# One ordered row pins every column's NULL flag plus the raw bytes of any
# non-NULL default: `a:1,-`/`b:1,-` are SQL NULL, `c`/`d` carry HEX output so
# a string literal can never masquerade as the null datum.
T06_A3_DEFAULT_ROWS = (
    "SELECT GROUP_CONCAT(CONCAT_WS(':', COLUMN_NAME, COLUMN_DEFAULT IS NULL, "
    "IFNULL(HEX(COLUMN_DEFAULT),'-')) ORDER BY ORDINAL_POSITION) "
    "FROM information_schema.COLUMNS "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'"
)
T06_A3_KEEPC_COLUMN = (
    "SELECT CONCAT_WS(':', COLUMN_NAME, DATA_TYPE, CHARACTER_MAXIMUM_LENGTH, "
    "IS_NULLABLE, COLUMN_DEFAULT IS NULL) FROM information_schema.COLUMNS "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND COLUMN_NAME='keep_c'"
)
# BINARY ordering keeps PRIMARY ahead of idx_keep on every anchor: MySQL's
# information_schema columns carry a case-insensitive collation while TiDB
# compares bytes, so a plain ORDER BY INDEX_NAME forks by engine.
T06_A3_INDEX_ROWS = (
    "SELECT GROUP_CONCAT(CONCAT_WS(':', INDEX_NAME, COLUMN_NAME, SEQ_IN_INDEX) "
    "ORDER BY BINARY INDEX_NAME, SEQ_IN_INDEX) FROM information_schema.STATISTICS "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'"
)

# T06-A4 (issue #85): provider default-identity fix. The physical CREATE lives
# in setup — the audited SQL is only DROP COLUMN + CREATE INDEX — so the live
# information_schema snapshot (not the input AST) is what feeds the ordered
# drop-column state. That is what exercises the mysql/tidb provider fix:
# stored literal 'NULL' must not masquerade as SQL NULL.
T06_A4_PROFILE = "t06-a4-provider-defaults-isolated"
T06_A4_TABLE = "t06_a4_defaults"
T06_A4_SETUP_AB_SQL = (
    "CREATE TABLE t06_a4_defaults (id INT PRIMARY KEY, a VARCHAR(8), "
    "b VARCHAR(8) DEFAULT NULL, c VARCHAR(8) DEFAULT 'NULL', "
    "d VARCHAR(8) DEFAULT '<nil>');"
)
# The C control initial state only differs in column c's declaration:
# DEFAULT NULL instead of the 'NULL' string literal.
T06_A4_SETUP_C_SQL = (
    "CREATE TABLE t06_a4_defaults (id INT PRIMARY KEY, a VARCHAR(8), "
    "b VARCHAR(8) DEFAULT NULL, c VARCHAR(8) DEFAULT NULL, "
    "d VARCHAR(8) DEFAULT '<nil>');"
)
T06_A4_DROP_D_STATEMENTS = [
    "ALTER TABLE t06_a4_defaults DROP COLUMN d;",
    "CREATE INDEX idx_b ON t06_a4_defaults(b);",
]
T06_A4_DROP_C_STATEMENTS = [
    "ALTER TABLE t06_a4_defaults DROP COLUMN c;",
    "CREATE INDEX idx_b ON t06_a4_defaults(b);",
]
T06_A4_DROP_D_SQL = "\n".join(T06_A4_DROP_D_STATEMENTS)
T06_A4_DROP_C_SQL = "\n".join(T06_A4_DROP_C_STATEMENTS)
T06_A4_TABLE_COUNT = (
    "SELECT COUNT(*) FROM information_schema.TABLES "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t06_a4_defaults'"
)
T06_A4_COLUMN_ROWS = (
    "SELECT GROUP_CONCAT(CONCAT_WS(':', COLUMN_NAME, DATA_TYPE, ORDINAL_POSITION, IS_NULLABLE) "
    "ORDER BY ORDINAL_POSITION) FROM information_schema.COLUMNS "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t06_a4_defaults'"
)
# The frozen oracle covers the four varchar probes only; id carries no default
# in any variant and is pinned separately via the column-order and PRIMARY
# member queries. `1` = COLUMN_DEFAULT IS NULL, `-` is only this query's
# unambiguous display for the SQL NULL datum, not a stored value.
T06_A4_DEFAULT_ROWS = (
    "SELECT GROUP_CONCAT(CONCAT_WS(':', COLUMN_NAME, COLUMN_DEFAULT IS NULL, "
    "IFNULL(HEX(COLUMN_DEFAULT),'-')) ORDER BY ORDINAL_POSITION) "
    "FROM information_schema.COLUMNS "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t06_a4_defaults' "
    "AND COLUMN_NAME <> 'id'"
)
T06_A4_PK_MEMBER = (
    "SELECT CONCAT_WS(':', COLUMN_NAME, SEQ_IN_INDEX) FROM information_schema.STATISTICS "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t06_a4_defaults' AND INDEX_NAME='PRIMARY' "
    "ORDER BY SEQ_IN_INDEX"
)
T06_A4_INDEX_ROWS = (
    "SELECT GROUP_CONCAT(CONCAT_WS(':', INDEX_NAME, COLUMN_NAME, SEQ_IN_INDEX) "
    "ORDER BY BINARY INDEX_NAME, SEQ_IN_INDEX) FROM information_schema.STATISTICS "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t06_a4_defaults'"
)
T06_A4_IDX_B_COUNT = (
    "SELECT COUNT(*) FROM information_schema.STATISTICS "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t06_a4_defaults' "
    "AND INDEX_NAME='idx_b'"
)
# Frozen oracle rows (issue #85): A/B initial defaults store literal 'NULL' on
# c; the C control stores SQL NULL. Mid-drop rows drop one probe each.
T06_A4_SETUP_COLUMNS = "id:int:1:NO,a:varchar:2:YES,b:varchar:3:YES,c:varchar:4:YES,d:varchar:5:YES"
T06_A4_SETUP_DEFAULTS_AB = "a:1:-,b:1:-,c:0:4E554C4C,d:0:3C6E696C3E"
T06_A4_SETUP_DEFAULTS_C = "a:1:-,b:1:-,c:1:-,d:0:3C6E696C3E"
T06_A4_AFTER_DROP_D_COLUMNS = "id:int:1:NO,a:varchar:2:YES,b:varchar:3:YES,c:varchar:4:YES"
T06_A4_AFTER_DROP_C_COLUMNS = "id:int:1:NO,a:varchar:2:YES,b:varchar:3:YES,d:varchar:4:YES"
T06_A4_AFTER_DROP_D_DEFAULTS_AB = "a:1:-,b:1:-,c:0:4E554C4C"
T06_A4_AFTER_DROP_C_DEFAULTS_AB = "a:1:-,b:1:-,d:0:3C6E696C3E"
T06_A4_AFTER_DROP_D_DEFAULTS_C = "a:1:-,b:1:-,c:1:-"
T06_A4_FINAL_INDEXES = "PRIMARY:id:1,idx_b:b:1"

# T06-A5 (issue #85): declared CHAR/VARCHAR length vs policy threshold.
# The rules compare the declared character count — never bytes, stored data,
# or an index/row-size limit — so every case pins the policy boundary at
# N∈{7,8,9} under limit=8, and the anchored metadata cases pin the catalog's
# two independent length fields (CHARACTER_MAXIMUM_LENGTH=N characters,
# CHARACTER_OCTET_LENGTH=4*N bytes under utf8mb4).
T06_A5_CHAR_PROFILE = "t06-a5-char-length-isolated"
T06_A5_VARCHAR_PROFILE = "t06-a5-varchar-length-isolated"
T06_A5_CHAR_RULE = "ddl.column.char.max_length"
T06_A5_VARCHAR_RULE = "ddl.column.varchar.max_length"
T06_A5_CHAR_MESSAGE = 'char column "c" must not exceed 8 characters'
T06_A5_VARCHAR_MESSAGE = 'varchar column "c" must not exceed 8 characters'


def t06_a5_sql(keyword, n, anchored):
    if anchored:
        return (f"CREATE TABLE t (c {keyword}({n}) "
                "CHARACTER SET utf8mb4 COLLATE utf8mb4_bin);")
    return f"CREATE TABLE t (c {keyword}({n}));"


T06_A5_LENGTH_PAIR = (
    "SELECT CONCAT_WS(':', CHARACTER_MAXIMUM_LENGTH, CHARACTER_OCTET_LENGTH) "
    "FROM information_schema.COLUMNS "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND COLUMN_NAME='c'"
)
T06_A5_CHARSET = (
    "SELECT CONCAT_WS(':', CHARACTER_SET_NAME, COLLATION_NAME) "
    "FROM information_schema.COLUMNS "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND COLUMN_NAME='c'"
)

# T06-A7 (issue #85): column charset/collation isolation plus the new
# table-level COLLATE allowlist. The column rules stay untouched — the
# proof isolates each of the three on its own profile; the table rule
# reads the declared table option and is a team policy boundary, never a
# native collation catalog check.
T06_A7_CS_PROFILE = "t06-a7-column-charset-isolated"
T06_A7_CC_PROFILE = "t06-a7-column-collation-isolated"
T06_A7_CM_PROFILE = "t06-a7-column-match-isolated"
T06_A7_TC_PROFILE = "t06-a7-table-collation-optional"
T06_A7_TCR_PROFILE = "t06-a7-table-collation-required"
T06_A7_CS_RULE = "ddl.column.charset.allowlist"
T06_A7_CC_RULE = "ddl.column.collation.allowlist"
T06_A7_CM_RULE = "ddl.column.charset_collation.match.require"
T06_A7_TC_RULE = "ddl.table.collation.allowlist"
T06_A7_CS_MESSAGE = 'column "c" uses unsupported charset "latin1"'
T06_A7_CC_MESSAGE = 'column "c" uses unsupported collation "utf8mb4_general_ci"'
T06_A7_CM_TOGETHER_MESSAGE = (
    'column "c" must specify charset and collation together'
)
T06_A7_CM_MISMATCH_MESSAGE = (
    'column "c" collation "latin1_swedish_ci" must match charset "utf8mb4"'
)
T06_A7_CM_MISMATCH_ANCHOR_MESSAGE = (
    'column "c" collation "latin1_bin" must match charset "utf8mb4"'
)
T06_A7_TC_MESSAGE = "table collation must be one of [utf8mb4_bin]"
T06_A7_TABLE_COLLATION = (
    "SELECT TABLE_COLLATION FROM information_schema.TABLES "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'"
)


def t06_a7_column_sql(declared):
    if declared:
        return "CREATE TABLE t (c VARCHAR(16) " + declared + ");"
    return "CREATE TABLE t (c VARCHAR(16));"

# T06-A8 (issue #85): comment declaration fidelity. Column.Comment stores the
# parser-decoded string content (no SQL quote wrapping), the table comment
# max_length rule counts Unicode code points (utf8.RuneCountInString), and the
# derived CREATE shape carries Table.Comment. The anchored cases pin the raw
# stored comment plus its CHAR_LENGTH/OCTET_LENGTH/HEX triple so a missing row
# or a NULL can never pass as an empty string, and SET NAMES utf8mb4 makes the
# multibyte delivery encoding an explicit recorded fact.
T06_A8_TABLE_PROFILE = "t06-a8-table-comment-required"
T06_A8_COLUMN_PROFILE = "t06-a8-column-comment-required"
T06_A8_LENGTH_PROFILE = "t06-a8-table-comment-length"
T06_A8_TABLE_RULE = "ddl.table.comment.require"
T06_A8_COLUMN_RULE = "ddl.column.comment.require"
T06_A8_LENGTH_RULE = "ddl.table.comment.max_length"
T06_A8_TABLE_MESSAGE = "table comment is required"
T06_A8_COLUMN_MESSAGE = 'column "c" must include a comment'
T06_A8_LENGTH_MESSAGE = "table comment must not exceed 8 characters"
T06_A8_TABLE_COMMENT_STATS = (
    "SELECT CONCAT_WS(':', CHAR_LENGTH(TABLE_COMMENT), "
    "OCTET_LENGTH(TABLE_COMMENT), HEX(TABLE_COMMENT)) "
    "FROM information_schema.TABLES "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'"
)
T06_A8_COLUMN_COMMENT_STATS = (
    "SELECT CONCAT_WS(':', CHAR_LENGTH(COLUMN_COMMENT), "
    "OCTET_LENGTH(COLUMN_COMMENT), HEX(COLUMN_COMMENT)) "
    "FROM information_schema.COLUMNS "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND COLUMN_NAME='c'"
)
T06_A8_TABLE_COMMENT_RAW = (
    "SET NAMES utf8mb4; SELECT TABLE_COMMENT FROM information_schema.TABLES "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'"
)
T06_A8_COLUMN_COMMENT_RAW = (
    "SET NAMES utf8mb4; SELECT COLUMN_COMMENT FROM information_schema.COLUMNS "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND COLUMN_NAME='c'"
)
T06_A8_DELIVER_CHARSET = "SET NAMES utf8mb4; SELECT @@character_set_client; "

# T06-A9 (issue #85): explicit audit-time-column roles. The rule recognizes
# roles from typed facts — time-like type + DefaultIsCurrentTimestamp, plus
# OnUpdateCurrentTimestamp for the updated role — never from column names.
# The anchored oracle does not compare version-rendered catalog text for the
# temporal attributes: it asserts semantic booleans computed in SQL (a frozen
# lowercased CURRENT_TIMESTAMP whitelist for the default, a normalized EXTRA
# for the ON UPDATE marker) while a raw ordered column row is still recorded
# for human inspection. fsp stays 0 throughout; `current_timestamp(3)` or any
# unknown rendering fails the whitelist by construction.
T06_A9_PROFILE = "t06-a9-audit-columns-isolated"
T06_A9_RULE = "ddl.table.audit_columns.require"
T06_A9_CREATED_MESSAGE = (
    "table should include a created-time audit column with DEFAULT CURRENT_TIMESTAMP"
)
T06_A9_UPDATED_MESSAGE = (
    "table should include an updated-time audit column "
    "with DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP"
)
# Ordered column identity is deterministic across anchors: DATA_TYPE and
# IS_NULLABLE render identically on all four, and DATETIME_PRECISION is 0 for
# the declared DATETIME columns and NULL (pinned as '-') for INT.
T06_A9_COLUMN_LIST = (
    "SELECT GROUP_CONCAT(CONCAT_WS(':', COLUMN_NAME, DATA_TYPE, "
    "ORDINAL_POSITION, IS_NULLABLE, IFNULL(DATETIME_PRECISION,'-')) "
    "ORDER BY ORDINAL_POSITION) FROM information_schema.COLUMNS "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'"
)
# Raw rows are recorded verbatim (execute observation step) — never asserted
# — so per-version catalog rendering stays inspectable without relaxing the
# semantic oracle.
T06_A9_COLUMN_RAW_ROWS = (
    "SELECT GROUP_CONCAT(CONCAT_WS(':', COLUMN_NAME, DATA_TYPE, "
    "ORDINAL_POSITION, IS_NULLABLE, COALESCE(DATETIME_PRECISION,'<null>'), "
    "COALESCE(COLUMN_DEFAULT,'<null>'), EXTRA) ORDER BY ORDINAL_POSITION "
    "SEPARATOR '|') FROM information_schema.COLUMNS "
    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'"
)


def t06_a9_column_default_ok(column):
    """Semantic boolean: the stored default is a frozen zero-precision
    CURRENT_TIMESTAMP spelling (SQL NULL, literals, fsp>0 and unknown
    expressions all fail)."""
    return (
        "SELECT LOWER(COALESCE(COLUMN_DEFAULT,'<null>')) IN "
        "('current_timestamp','current_timestamp()','current_timestamp(0)') "
        "FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='golden' "
        f"AND TABLE_NAME='t' AND COLUMN_NAME='{column}'"
    )


T06_A9_EXTRA_CREATED_WHITELIST = ("", "default_generated")
T06_A9_EXTRA_UPDATED_WHITELIST = (
    "on update current_timestamp",
    "on update current_timestamp()",
    "on update current_timestamp(0)",
    "default_generated on update current_timestamp",
    "default_generated on update current_timestamp()",
    "default_generated on update current_timestamp(0)",
)
# Distinctive marker emitted for any EXTRA text outside the frozen
# whitelist — it cannot satisfy either audit-column role, so malformed or
# unknown catalog text can never be "repaired" into a passing shape.
T06_A9_EXTRA_SENTINEL = "<unrecognized-extra>"


def t06_a9_extra_case(expr):
    """CASE folding an EXTRA value into the frozen role marker: '' for a
    created column, 'on update current_timestamp' for an updated column,
    and the sentinel for everything else. The complete lower-cased text
    must match a whitelist entry exactly — no substring deletion, token
    removal, or whitespace merging ever rewrites unknown text."""
    created = "(" + ",".join(f"'{v}'" for v in T06_A9_EXTRA_CREATED_WHITELIST) + ")"
    updated = "(" + ",".join(f"'{v}'" for v in T06_A9_EXTRA_UPDATED_WHITELIST) + ")"
    return (
        f"CASE WHEN LOWER({expr}) IN {created} THEN '' "
        f"WHEN LOWER({expr}) IN {updated} THEN 'on update current_timestamp' "
        f"ELSE '{T06_A9_EXTRA_SENTINEL}' END"
    )


def t06_a9_extra_normalized(column):
    """Stored EXTRA of a DATETIME column folded through the strict
    whitelist CASE — the same expression the literal control SELECTs."""
    return (
        "SELECT " + t06_a9_extra_case("EXTRA") + " "
        "FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='golden' "
        f"AND TABLE_NAME='t' AND COLUMN_NAME='{column}'"
    )


# Read-only scalar control: the whitelist CASE over fixed literals —
# every legal spelling for both roles, then a battery of malformed texts
# that substring-replacement oracles used to accept. The same CASE
# expression as t06_a9_extra_normalized is applied to each literal.
T06_A9_EXTRA_LITERALS = (
    "", "default_generated", "DEFAULT_GENERATED",
    "on update current_timestamp",
    "ON UPDATE CURRENT_TIMESTAMP()",
    "on update current_timestamp(0)",
    "default_generated on update current_timestamp",
    "default_generated ON UPDATE current_timestamp()",
    "default_generated on update current_timestamp(0)",
    "DEFAULT_GENERATED()",
    "DEFAULT_GENERATEDDEFAULT_GENERATED",
    "()",
    "on update CURRENT_TIMESTAMP(0)()",
    "on update current_default_generatedtimestamp",
    "on update curr()ent_timestamp",
    "on update current_timestamp(3)",
    "stored generated",
)
T06_A9_EXTRA_LITERAL_SQL = (
    "SELECT CONCAT_WS('|',"
    + ",".join(
        t06_a9_extra_case("NULL" if lit is None else "'" + lit + "'")
        for lit in (*T06_A9_EXTRA_LITERALS, None))
    + ")"
)


def t06_a9_extra_literal_expect():
    """Frozen CONCAT_WS output for the literal control — computed from
    the same whitelist constants the CASE expression embeds."""
    parts = []
    for lit in T06_A9_EXTRA_LITERALS:
        lowered = lit.lower()
        if lowered in T06_A9_EXTRA_CREATED_WHITELIST:
            parts.append("")
        elif lowered in T06_A9_EXTRA_UPDATED_WHITELIST:
            parts.append("on update current_timestamp")
        else:
            parts.append(T06_A9_EXTRA_SENTINEL)
    parts.append(T06_A9_EXTRA_SENTINEL)  # SQL NULL literal
    return "|".join(parts)


T06_ANCHORS = {
    "mysql57": {
        "service": "mysql57",
        "container": "deltascope-ddl-golden-mysql57",
        "image": "mysql:5.7.44",
        "product": "mysql",
        "version_contains": "5.7.44",
        "database": "golden",
        "exec_client": ["mysql", "-uroot", "-proot"],
        "needs_database_create": False,
    },
    "mysql80": {
        "service": "mysql80",
        "container": "deltascope-ddl-golden-mysql80",
        "image": "mysql:8.0.46",
        "product": "mysql",
        "version_contains": "8.0.46",
        "database": "golden",
        "exec_client": ["mysql", "-uroot", "-proot"],
        "needs_database_create": False,
    },
    "mysql84": {
        "service": "mysql84",
        "container": "deltascope-ddl-golden-mysql84",
        "image": "mysql:8.4.10",
        "product": "mysql",
        "version_contains": "8.4.10",
        "database": "golden",
        "exec_client": ["mysql", "-uroot", "-proot"],
        "needs_database_create": False,
    },
    "tidb85": {
        "service": "tidb85",
        "container": "deltascope-ddl-golden-tidb85",
        "client_container": "deltascope-ddl-golden-tidb85-client",
        "client_service": "tidb85-client",
        "image": "pingcap/tidb:v8.5.0",
        "product": "tidb",
        "version_contains": "v8.5.0",
        "database": "golden",
        "exec_client": ["mysql", "--protocol=tcp", "-h", "tidb85", "-P", "4000", "-uroot"],
        "needs_database_create": True,
    },
}

# The baseline execution block is the T02 contract verbatim: same statements,
# same verify queries, same negative case. It proves connectivity and the
# audit/execute split on this anchor set — it is not T06 semantic evidence.
T06_DDL_STEPS = [
    {
        "name": "create",
        "sql": "CREATE TABLE golden_t (id INT PRIMARY KEY)",
        "expect_rc": 0,
        "verify": [
            {
                "assert": "table exists",
                "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='golden_t'",
                "expect": "1",
            },
            {
                "assert": "primary key exists",
                "sql": "SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='golden_t' AND CONSTRAINT_TYPE='PRIMARY KEY'",
                "expect": "1",
            },
        ],
    },
    {
        "name": "alter",
        "sql": "ALTER TABLE golden_t ADD COLUMN c INT",
        "expect_rc": 0,
        "verify": [
            {
                "assert": "column c exists",
                "sql": "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='golden_t' AND COLUMN_NAME='c'",
                "expect": "1",
            },
        ],
    },
    {
        "name": "drop",
        "sql": "DROP TABLE golden_t",
        "expect_rc": 0,
        "verify": [
            {
                "assert": "table absent",
                "sql": "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='golden_t'",
                "expect": "0",
            },
        ],
    },
]

T06_SYNTAX_NEGATIVE = {
    "sql": "CREATE TABLE golden_broken (",
    "expect": {
        "rc_nonzero": True,
        "error_class": "1064",
        "forbidden_markers": [
            "ERROR 1045",
            "ERROR 1044",
            "ERROR 1049",
            "ERROR 1142",
            "ERROR 1143",
            "ERROR 2002",
            "ERROR 2003",
            "ERROR 2005",
            "Access denied",
            "Unknown database",
            "Can't connect",
            "Connection refused",
            "already exists",
        ],
    },
}


def t06_a1_contract():
    """Frozen T06 oracle (issue #85): the accepted A1 32-case subset, the
    A2 24-case subset, the A3 16-case subset, the A4 12-case subset, the
    A5 40-case subset, the A7 64-case subset, the A8 40-case subset, and the
    A9 28-case subset — 256 cases total. The A8 subset pins comment
    declaration fidelity: a 24-case offline matrix isolates the two presence
    rules and the code-point length rule across both dialects, and 16
    anchored roles prove stored comments arrive losslessly (raw text plus
    CHAR_LENGTH/OCTET_LENGTH/HEX) even when the policy rejects the audit.

    The A9 subset pins explicit audit-time-column roles under a single
    isolated profile: a 12-case offline matrix covers the complete pair, each
    single-role miss, the two-finding missing-both shape (the frozen
    expectation is a complete finding-entry multiset), the NOW() synonym
    spellings, and the all-off control; 16 anchored roles prove the driver
    stores the declared columns — per-column default whitelist booleans,
    normalized EXTRA markers, PK membership, engine — regardless of the
    product policy verdict.

    A1 baseline cases reuse the T02 batch verbatim; the offline controls pin
    the isolated policy profiles; the anchored cases pin the
    information_schema structure oracle.

    The A2 subset pins primary-key member nullability normalization: legal
    table-level and composite members pass the isolated not-null rule, while
    explicit NULL members keep exactly one policy blocker and the driver
    rejects them with ERROR 1171. DEFAULT presence keeps its exact contract —
    an absent clause reports, explicit DEFAULT NULL satisfies.

    The A3 subset pins typed DEFAULT NULL identity: a four-spelling CLI
    matrix reuses the default-presence profile, a four-column representation
    case proves via COLUMN_DEFAULT NULL flags and HEX bytes that the server
    stored `b` as SQL NULL and `c`/`d` as strings, and a three-statement
    CREATE→DROP COLUMN→CREATE INDEX path proves a sibling DEFAULT NULL no
    longer poisons the drop-column state projection.

    The A4 subset pins provider default-identity: the physical fixture lives
    in setup, so the audited DROP+INDEX batch can only be answered from the
    live information_schema snapshot — a stored 'NULL'/'<nil>' literal keeps
    the conservative unknown_table_state gap while a stored SQL NULL stays
    precise.

    The A5 subset pins declared CHAR/VARCHAR length policy: a 16-cell offline
    matrix (2 types × 2 dialects × {below,at,above,off}) plus 24 anchored
    metadata cases prove the threshold compares declared characters, the
    product never executes the audited CREATE (post_verify), and the driver
    replay always succeeds — even for the rejected N=9 — with
    CHARACTER_MAXIMUM_LENGTH=N and CHARACTER_OCTET_LENGTH=4·N recorded as two
    independent facts.

    MySQL 8.0/8.4 cases additionally record the GIPK-related server variables
    so a server-generated invisible primary key can never masquerade as a
    declared PRIMARY KEY."""

    def verify(label, sql, expect):
        return {"assert": label, "sql": sql, "expect": expect}

    def audit_expect(sql, rejects, loaded=None, finding=None):
        expect = {
            "exit": 1 if rejects else 0,
            "verdict": "reject" if rejects else "pass",
            "statements": 1,
            "findings": 1 if rejects else 0,
            "diagnostics": 0,
            "unsupported": 0,
            "coverage": "complete",
            "statement_coverage": ["complete"],
            "statement_sql": [sql],
            "statement_indices": [0],
            "evidence_gaps": 0,
            "fail_on_triggered": bool(rejects),
        }
        if loaded is not None:
            expect["rule_summary_loaded"] = loaded
        if rejects:
            items = finding if isinstance(finding, list) else [finding or {}]
            expect["findings"] = len(items)
            expect["finding_entries"] = [
                {"index": 0,
                 "rule_id": item.get("rule_id") or T06_PK_RULE,
                 "level": "blocker"}
                for item in items
            ]
            expect["finding_metadata"] = [
                {"index": 0,
                 "rule_id": item.get("rule_id") or T06_PK_RULE,
                 "metadata": item.get("metadata") or {"table": "t"}}
                for item in items
            ]
            expect["finding_locations"] = [
                {"index": 0, "line": 1, "column": 1} for _ in items
            ]
            if len(items) == 1:
                expect["finding_message"] = items[0].get(
                    "message") or "primary key is required"
            else:
                # Multi-finding cases pin complete entries as a multiset —
                # statement index, rule_id, level, message, the whole
                # metadata map, and location per entry — so two findings on
                # one rule can never be merged or cross-swapped.
                expect["finding_entries_full"] = [
                    {"index": 0,
                     "rule_id": item.get("rule_id") or T06_PK_RULE,
                     "level": "blocker",
                     "message": item.get("message") or "primary key is required",
                     "metadata": item.get("metadata") or {"table": "t"},
                     "line": 1, "column": 1}
                    for item in items
                ]
        return expect

    baseline_expect = {
        "exit": 0,
        "verdict": "pass",
        "statements": 3,
        "findings": 0,
        "diagnostics": 0,
        "unsupported": 0,
        "coverage": "complete",
        "statement_coverage": ["complete", "complete", "complete"],
        "statement_sql": list(T06_BASELINE_STATEMENTS),
        "statement_indices": [0, 1, 2],
        "evidence_gaps": 0,
        "fail_on_triggered": False,
    }

    cli_cases = []
    for dialect in ("mysql", "tidb"):
        cli_cases.append({
            "id": dialect,
            "dialect": dialect,
            "sql": T06_BASELINE_SQL,
            "expect": dict(baseline_expect),
        })
    for dialect in ("mysql", "tidb"):
        cli_cases += [
            {
                "id": f"t06-{dialect}-no-pk",
                "dialect": dialect,
                "sql": T06_NO_PK_SQL,
                "policy": T06_ISOLATED_PROFILE,
                "args": ["--fail-on", "blocker"],
                "expect": audit_expect(T06_NO_PK_SQL, True, loaded=1),
            },
            {
                "id": f"t06-{dialect}-inline-pk",
                "dialect": dialect,
                "sql": T06_INLINE_PK_SQL,
                "policy": T06_ISOLATED_PROFILE,
                "args": ["--fail-on", "blocker"],
                "expect": audit_expect(T06_INLINE_PK_SQL, False, loaded=1),
            },
            {
                "id": f"t06-{dialect}-table-pk",
                "dialect": dialect,
                "sql": T06_TABLE_PK_SQL,
                "policy": T06_ISOLATED_PROFILE,
                "args": ["--fail-on", "blocker"],
                "expect": audit_expect(T06_TABLE_PK_SQL, False, loaded=1),
            },
            {
                "id": f"t06-{dialect}-rule-off",
                "dialect": dialect,
                "sql": T06_NO_PK_SQL,
                "args": ["--fail-on", "blocker"],
                "expect": audit_expect(T06_NO_PK_SQL, False),
            },
            {
                "id": f"t06-{dialect}-required-false",
                "dialect": dialect,
                "sql": T06_NO_PK_SQL,
                "policy": T06_REQUIRED_FALSE_PROFILE,
                "args": ["--fail-on", "blocker"],
                "expect": audit_expect(T06_NO_PK_SQL, False, loaded=1),
            },
        ]
    pk_nn_finding = {
        "rule_id": T06_PK_NN_RULE,
        "message": 'primary key column "id" must be NOT NULL',
        "metadata": {"table": "t", "column": "id"},
    }
    default_finding = {
        "rule_id": T06_DEFAULT_RULE,
        "message": 'column "c" should define a default value',
        "metadata": {"table": "t", "column": "c", "type": "int(11)"},
    }
    for dialect in ("mysql", "tidb"):
        cli_cases += [
            {
                "id": f"t06-a2-{dialect}-table-single",
                "dialect": dialect,
                "sql": T06_TABLE_PK_SQL,
                "policy": T06_A2_PK_NULL_PROFILE,
                "args": ["--fail-on", "blocker"],
                "expect": audit_expect(T06_TABLE_PK_SQL, False, loaded=1),
            },
            {
                "id": f"t06-a2-{dialect}-table-composite",
                "dialect": dialect,
                "sql": T06_A2_COMPOSITE_PK_SQL,
                "policy": T06_A2_PK_NULL_PROFILE,
                "args": ["--fail-on", "blocker"],
                "expect": audit_expect(T06_A2_COMPOSITE_PK_SQL, False, loaded=1),
            },
            {
                "id": f"t06-a2-{dialect}-no-default",
                "dialect": dialect,
                "sql": T06_A2_NO_DEFAULT_SQL,
                "policy": T06_A2_DEFAULT_PROFILE,
                "args": ["--fail-on", "blocker"],
                "expect": audit_expect(T06_A2_NO_DEFAULT_SQL, True, loaded=1,
                                       finding=default_finding),
            },
            {
                "id": f"t06-a2-{dialect}-default-null",
                "dialect": dialect,
                "sql": T06_A2_DEFAULT_NULL_SQL,
                "policy": T06_A2_DEFAULT_PROFILE,
                "args": ["--fail-on", "blocker"],
                "expect": audit_expect(T06_A2_DEFAULT_NULL_SQL, False, loaded=1),
            },
        ]
    a3_default_finding = {
        "rule_id": T06_DEFAULT_RULE,
        "message": 'column "c" should define a default value',
        "metadata": {"table": "t", "column": "c", "type": "varchar(8)"},
    }
    for dialect in ("mysql", "tidb"):
        for variant, sql, rejects in (
            ("no-default", T06_A3_NO_DEFAULT_SQL, True),
            ("sql-null", T06_A3_SQL_NULL_SQL, False),
            ("text-null", T06_A3_TEXT_NULL_SQL, False),
            ("text-nil", T06_A3_TEXT_NIL_SQL, False),
        ):
            cli_cases.append({
                "id": f"t06-a3-{dialect}-{variant}",
                "dialect": dialect,
                "sql": sql,
                "policy": T06_A2_DEFAULT_PROFILE,
                "args": ["--fail-on", "blocker"],
                "expect": audit_expect(sql, rejects, loaded=1,
                                       finding=a3_default_finding if rejects else None),
            })

    # T06-A5 (issue #85): declared-length threshold proof. The 16-cell
    # offline matrix pins 7/8/9 under limit=8 plus the all-off control for
    # both types and dialects; the "off" case intentionally carries no
    # policy key so it binds the manifest's all-rules-disabled default.
    a5_findings = {
        "CHAR": {"rule_id": T06_A5_CHAR_RULE,
                 "message": T06_A5_CHAR_MESSAGE,
                 "metadata": {"table": "t", "column": "c", "limit": 8, "actual": 9}},
        "VARCHAR": {"rule_id": T06_A5_VARCHAR_RULE,
                    "message": T06_A5_VARCHAR_MESSAGE,
                    "metadata": {"table": "t", "column": "c", "limit": 8, "actual": 9}},
    }
    a5_profiles = {"CHAR": T06_A5_CHAR_PROFILE, "VARCHAR": T06_A5_VARCHAR_PROFILE}
    for dialect in ("mysql", "tidb"):
        for keyword in ("CHAR", "VARCHAR"):
            for variant, n in (("below", 7), ("at", 8), ("above", 9), ("off", 9)):
                sql = t06_a5_sql(keyword, n, anchored=False)
                case = {
                    "id": f"t06-a5-{dialect}-{keyword.lower()}-{variant}",
                    "dialect": dialect,
                    "sql": sql,
                    "args": ["--fail-on", "blocker"],
                }
                if variant == "off":
                    case["expect"] = audit_expect(sql, False)
                elif variant == "above":
                    case["policy"] = a5_profiles[keyword]
                    case["expect"] = audit_expect(sql, True, loaded=1, finding=a5_findings[keyword])
                else:
                    case["policy"] = a5_profiles[keyword]
                    case["expect"] = audit_expect(sql, False, loaded=1)
                cli_cases.append(case)

    # T06-A7 (issue #85): sixteen offline roles on each dialect pin the
    # column charset/collation rules in isolation and the new table
    # collation allowlist under both require_explicit values plus the
    # all-off control. The "off" cells bind the manifest's default
    # all-rules-disabled policy — capability classification no longer
    # depends on an enabled rule finding the declared option.
    a7_charset_finding = {
        "rule_id": T06_A7_CS_RULE,
        "message": T06_A7_CS_MESSAGE,
        "metadata": {
            "table": "t", "column": "c", "field": "charset",
            "value": "latin1", "allowed": ["utf8mb4"],
        },
    }
    a7_collation_finding = {
        "rule_id": T06_A7_CC_RULE,
        "message": T06_A7_CC_MESSAGE,
        "metadata": {
            "table": "t", "column": "c", "field": "collation",
            "value": "utf8mb4_general_ci", "allowed": ["utf8mb4_bin"],
        },
    }
    a7_together_finding = {
        "rule_id": T06_A7_CM_RULE,
        "message": T06_A7_CM_TOGETHER_MESSAGE,
        "metadata": {
            "table": "t", "column": "c", "charset": "",
            "collation": "utf8mb4_bin",
        },
    }
    a7_mismatch_finding = {
        "rule_id": T06_A7_CM_RULE,
        "message": T06_A7_CM_MISMATCH_MESSAGE,
        "metadata": {
            "table": "t", "column": "c", "charset": "utf8mb4",
            "collation": "latin1_swedish_ci",
        },
    }
    a7_table_finding = {
        "rule_id": T06_A7_TC_RULE,
        "message": T06_A7_TC_MESSAGE,
        "metadata": {
            "table": "t", "option": "collate",
            "actual": "utf8mb4_general_ci", "allowed": ["utf8mb4_bin"],
        },
    }
    a7_table_required_finding = {
        "rule_id": T06_A7_TC_RULE,
        "message": T06_A7_TC_MESSAGE,
        "metadata": {
            "table": "t", "option": "collate",
            "actual": "", "allowed": ["utf8mb4_bin"],
        },
    }
    a7_column_matrix = (
        ("cs-ok", T06_A7_CS_PROFILE, "CHARACTER SET utf8mb4 COLLATE utf8mb4_bin", None),
        ("cs-denied", T06_A7_CS_PROFILE, "CHARACTER SET latin1 COLLATE latin1_bin",
         a7_charset_finding),
        ("cs-off", None, "CHARACTER SET latin1 COLLATE latin1_bin", None),
        ("cc-ok", T06_A7_CC_PROFILE, "CHARACTER SET utf8mb4 COLLATE utf8mb4_bin", None),
        ("cc-denied", T06_A7_CC_PROFILE, "CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci",
         a7_collation_finding),
        ("cc-off", None, "CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci", None),
        ("match-pair", T06_A7_CM_PROFILE, "CHARACTER SET utf8mb4 COLLATE utf8mb4_bin", None),
        ("match-single", T06_A7_CM_PROFILE, "COLLATE utf8mb4_bin",
         a7_together_finding),
        ("match-empty", T06_A7_CM_PROFILE, None, None),
        ("match-mismatch", T06_A7_CM_PROFILE,
         "CHARACTER SET utf8mb4 COLLATE latin1_swedish_ci", a7_mismatch_finding),
        ("match-off", None, "CHARACTER SET utf8mb4 COLLATE latin1_swedish_ci", None),
    )
    a7_table_matrix = (
        ("table-ok", T06_A7_TC_PROFILE,
         "CREATE TABLE t (c INT) COLLATE=utf8mb4_bin;", None),
        ("table-denied", T06_A7_TC_PROFILE,
         "CREATE TABLE t (c INT) COLLATE=utf8mb4_general_ci;", a7_table_finding),
        ("table-empty", T06_A7_TC_PROFILE, "CREATE TABLE t (c INT);", None),
        ("table-required", T06_A7_TCR_PROFILE,
         "CREATE TABLE t (c INT);", a7_table_required_finding),
        ("table-off", None,
         "CREATE TABLE t (c INT) COLLATE=utf8mb4_general_ci;", None),
    )
    for dialect in ("mysql", "tidb"):
        for variant, profile, declared, finding in a7_column_matrix:
            sql = t06_a7_column_sql(declared)
            case = {
                "id": f"t06-a7-{dialect}-{variant}",
                "dialect": dialect,
                "sql": sql,
                "args": ["--fail-on", "blocker"],
            }
            if profile:
                case["policy"] = profile
                case["expect"] = audit_expect(
                    sql, finding is not None, loaded=1, finding=finding)
            else:
                case["expect"] = audit_expect(sql, False)
            cli_cases.append(case)
        for variant, profile, sql, finding in a7_table_matrix:
            case = {
                "id": f"t06-a7-{dialect}-{variant}",
                "dialect": dialect,
                "sql": sql,
                "args": ["--fail-on", "blocker"],
            }
            if profile:
                case["policy"] = profile
                case["expect"] = audit_expect(
                    sql, finding is not None, loaded=1, finding=finding)
            else:
                case["expect"] = audit_expect(sql, False)
            cli_cases.append(case)

    # T06-A8 (issue #85): twelve offline roles pin comment declaration
    # semantics — presence rules stay per-field (a table comment never
    # satisfies the column rule and vice versa), explicit empty and
    # whitespace-only column comments are "missing" because the require rule
    # trims the decoded content, and the max_length rule counts code points
    # so a three-rune Chinese comment is under the limit where nine UTF-8
    # bytes used to reject it.
    a8_table_finding = {
        "rule_id": T06_A8_TABLE_RULE,
        "message": T06_A8_TABLE_MESSAGE,
        "metadata": {"table": "t"},
    }
    a8_column_finding = {
        "rule_id": T06_A8_COLUMN_RULE,
        "message": T06_A8_COLUMN_MESSAGE,
        "metadata": {"table": "t", "column": "c"},
    }
    a8_length_finding = {
        "rule_id": T06_A8_LENGTH_RULE,
        "message": T06_A8_LENGTH_MESSAGE,
        "metadata": {"table": "t", "limit": 8, "actual": 9},
    }
    a8_matrix = (
        ("table-missing", T06_A8_TABLE_PROFILE,
         "CREATE TABLE t (c INT);", a8_table_finding),
        ("table-present", T06_A8_TABLE_PROFILE,
         "CREATE TABLE t (c INT) COMMENT='中文注';", None),
        ("column-missing", T06_A8_COLUMN_PROFILE,
         "CREATE TABLE t (c INT);", a8_column_finding),
        ("column-present", T06_A8_COLUMN_PROFILE,
         "CREATE TABLE t (c INT COMMENT '中文注');", None),
        ("column-empty", T06_A8_COLUMN_PROFILE,
         "CREATE TABLE t (c INT COMMENT '');", a8_column_finding),
        ("column-blank", T06_A8_COLUMN_PROFILE,
         "CREATE TABLE t (c INT COMMENT '   ');", a8_column_finding),
        ("length-at", T06_A8_LENGTH_PROFILE,
         "CREATE TABLE t (c INT) COMMENT='12345678';", None),
        ("length-above", T06_A8_LENGTH_PROFILE,
         "CREATE TABLE t (c INT) COMMENT='123456789';", a8_length_finding),
        ("length-multibyte", T06_A8_LENGTH_PROFILE,
         "CREATE TABLE t (c INT) COMMENT='中文注';", None),
        ("length-multibyte-at", T06_A8_LENGTH_PROFILE,
         "CREATE TABLE t (c INT) COMMENT='中文注中文注ab';", None),
        ("length-multibyte-above", T06_A8_LENGTH_PROFILE,
         "CREATE TABLE t (c INT) COMMENT='中文注中文注abc';", a8_length_finding),
        ("length-off", None,
         "CREATE TABLE t (c INT) COMMENT='123456789';", None),
    )
    for dialect in ("mysql", "tidb"):
        for variant, profile, sql, finding in a8_matrix:
            case = {
                "id": f"t06-a8-{dialect}-{variant}",
                "dialect": dialect,
                "sql": sql,
                "args": ["--fail-on", "blocker"],
            }
            if profile:
                case["policy"] = profile
                case["expect"] = audit_expect(
                    sql, finding is not None, loaded=1, finding=finding)
            else:
                case["expect"] = audit_expect(sql, False)
            cli_cases.append(case)

    # T06-A9 (issue #85): explicit audit-time-column roles. The six frozen
    # inputs pin role attribution from extracted facts — complete pair,
    # each single-role miss, the two-finding missing-both shape, the NOW()
    # synonym spellings, and the all-off control. missing-both is the first
    # T06 case whose frozen expectation is a two-entry finding multiset.
    a9_created_finding = {
        "rule_id": T06_A9_RULE,
        "message": T06_A9_CREATED_MESSAGE,
        "metadata": {"table": "t", "kind": "created"},
    }
    a9_updated_finding = {
        "rule_id": T06_A9_RULE,
        "message": T06_A9_UPDATED_MESSAGE,
        "metadata": {"table": "t", "kind": "updated"},
    }
    a9_matrix = (
        ("complete-pair",
         "CREATE TABLE t (id INT PRIMARY KEY, created_at DATETIME NOT NULL "
         "DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT "
         "CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP);",
         None),
        ("missing-created",
         "CREATE TABLE t (id INT PRIMARY KEY, updated_at DATETIME NOT NULL "
         "DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP);",
         a9_created_finding),
        ("missing-updated",
         "CREATE TABLE t (id INT PRIMARY KEY, created_at DATETIME NOT NULL "
         "DEFAULT CURRENT_TIMESTAMP);",
         a9_updated_finding),
        ("missing-both",
         "CREATE TABLE t (id INT PRIMARY KEY);",
         [a9_created_finding, a9_updated_finding]),
        ("now-spelling",
         "CREATE TABLE t (id INT PRIMARY KEY, created_at DATETIME NOT NULL "
         "DEFAULT NOW(), updated_at DATETIME NOT NULL DEFAULT NOW() "
         "ON UPDATE NOW());",
         None),
        ("disabled", "CREATE TABLE t (id INT PRIMARY KEY);", None),
    )
    for dialect in ("mysql", "tidb"):
        for variant, sql, finding in a9_matrix:
            case = {
                "id": f"t06-a9-{dialect}-{variant}",
                "dialect": dialect,
                "sql": sql,
                "args": ["--fail-on", "blocker"],
            }
            if variant == "disabled":
                case["expect"] = audit_expect(sql, False)
            else:
                case["policy"] = T06_A9_PROFILE
                case["expect"] = audit_expect(
                    sql, finding is not None, loaded=1, finding=finding)
            cli_cases.append(case)

    connects = {
        "mysql57": {
            "host": "127.0.0.1", "port": 23357, "user": "root",
            "password_env": "DS_T06_GOLDEN_PW", "password": "root",
            "schema": "golden",
        },
        "mysql80": {
            "host": "127.0.0.1", "port": 23380, "user": "root",
            "password_env": "DS_T06_GOLDEN_PW", "password": "root",
            "schema": "golden",
        },
        "mysql84": {
            "host": "127.0.0.1", "port": 23384, "user": "root",
            "password_env": "DS_T06_GOLDEN_PW", "password": "root",
            "schema": "golden",
        },
        "tidb85": {
            "host": "127.0.0.1", "port": 24000, "user": "root",
            "schema": "golden",
        },
    }

    def structure(anchor_key, pk):
        rows = [
            verify("exactly one user table", T06_TABLE_COUNT, "1"),
            verify("exactly one user column", T06_COLUMN_COUNT, "1"),
            verify(
                "column id identity",
                T06_COLUMN_ROW,
                "id:int:1:" + ("NO" if pk else "YES"),
            ),
            verify("primary key constraint count", T06_PK_CONSTRAINT, "1" if pk else "0"),
            verify("primary index part count", T06_PK_PARTS, "1" if pk else "0"),
        ]
        if pk:
            rows.append(verify("primary key member", T06_PK_MEMBER, "id:1"))
        if anchor_key != "tidb85":
            rows.append(verify("storage engine", T06_ENGINE, "InnoDB"))
        return rows

    def a2_structure(anchor_key, variant):
        rows = [verify("exactly one user table", T06_TABLE_COUNT, "1")]
        if variant == "table-composite":
            rows += [
                verify("exactly three user columns", T06_COLUMN_COUNT, "3"),
                verify(
                    "composite column order and nullability",
                    T06_A2_COLUMN_ROWS,
                    "a:int:1:NO,spare:int:2:YES,b:int:3:NO",
                ),
                verify("primary key constraint count", T06_PK_CONSTRAINT, "1"),
                verify("primary index part count", T06_PK_PARTS, "2"),
                verify("primary key members in key order", T06_A2_PK_MEMBERS, "b:1,a:2"),
            ]
        else:
            rows += [
                verify("exactly one user column", T06_COLUMN_COUNT, "1"),
                verify("column id identity", T06_COLUMN_ROW, "id:int:1:NO"),
                verify("primary key constraint count", T06_PK_CONSTRAINT, "1"),
                verify("primary index part count", T06_PK_PARTS, "1"),
                verify("primary key member", T06_PK_MEMBER, "id:1"),
            ]
        if anchor_key != "tidb85":
            rows.append(verify("storage engine", T06_ENGINE, "InnoDB"))
        return rows

    def a2_metadata_case(anchor_key, dialect, variant, sql, rejects):
        expect = audit_expect(sql, rejects, loaded=1,
                              finding=pk_nn_finding if rejects else None)
        if anchor_key in ("mysql80", "mysql84"):
            expect["instance_facts"] = dict(T06_GIPK_FACTS)
        case = {
            "id": f"t06-a2-{anchor_key}-{variant}",
            "anchor": anchor_key,
            "dialect": dialect,
            "sql": sql,
            "policy": T06_A2_PK_NULL_PROFILE,
            "connect": dict(connects[anchor_key]),
            "args": ["--fail-on", "blocker"],
            "setup": [
                {
                    "name": "ensure t absent",
                    "sql": "DROP TABLE IF EXISTS t",
                    "expect_rc": 0,
                    "verify": [verify("t absent before audit", T06_TABLE_COUNT, "0")],
                },
            ],
            "expect": expect,
            "post_verify": [
                verify("audit did not create t", T06_TABLE_COUNT, "0"),
            ],
            "teardown": [
                {
                    "name": "drop fixture",
                    "sql": "DROP TABLE IF EXISTS t",
                    "expect_rc": 0,
                    "verify": [verify("no residual t", T06_TABLE_COUNT, "0")],
                },
            ],
        }
        if rejects:
            # The explicit-NULL declaration conflict is a real native negative:
            # the driver must fail with ERROR 1171 (not a connectivity or
            # permission error) and the table must stay absent afterwards.
            case["execute"] = [
                {
                    "name": "driver rejects explicit-null primary key",
                    "sql": sql,
                    "expect_rc": 1,
                    "stderr_contains": ["ERROR 1171"],
                    "verify": [
                        verify("t absent after rejected create", T06_TABLE_COUNT, "0"),
                    ],
                },
            ]
        else:
            case["execute"] = [
                {
                    "name": "driver applies the audited create",
                    "sql": sql,
                    "expect_rc": 0,
                },
            ]
            case["structure"] = a2_structure(anchor_key, variant)
        return case

    metadata_cases = []
    for anchor_key in ("mysql57", "mysql80", "mysql84", "tidb85"):
        dialect = "tidb" if anchor_key == "tidb85" else "mysql"
        for variant, sql, has_pk in (
            ("no-pk", T06_NO_PK_SQL, False),
            ("inline-pk", T06_INLINE_PK_SQL, True),
            ("table-pk", T06_TABLE_PK_SQL, True),
        ):
            expect = audit_expect(sql, not has_pk, loaded=1)
            if anchor_key in ("mysql80", "mysql84"):
                expect["instance_facts"] = dict(T06_GIPK_FACTS)
            metadata_cases.append({
                "id": f"t06-{anchor_key}-{variant}",
                "anchor": anchor_key,
                "dialect": dialect,
                "sql": sql,
                "policy": T06_ISOLATED_PROFILE,
                "connect": dict(connects[anchor_key]),
                "args": ["--fail-on", "blocker"],
                "setup": [
                    {
                        "name": "ensure t absent",
                        "sql": "DROP TABLE IF EXISTS t",
                        "expect_rc": 0,
                        "verify": [verify("t absent before audit", T06_TABLE_COUNT, "0")],
                    },
                ],
                "expect": expect,
                "post_verify": [
                    verify("audit did not create t", T06_TABLE_COUNT, "0"),
                ],
                "execute": [
                    {
                        "name": "driver applies the audited create",
                        "sql": sql,
                        "expect_rc": 0,
                    },
                ],
                "structure": structure(anchor_key, has_pk),
                "teardown": [
                    {
                        "name": "drop fixture",
                        "sql": "DROP TABLE IF EXISTS t",
                        "expect_rc": 0,
                        "verify": [verify("no residual t", T06_TABLE_COUNT, "0")],
                    },
                ],
            })
        for variant, sql, rejects in (
            ("table-single", T06_TABLE_PK_SQL, False),
            ("table-composite", T06_A2_COMPOSITE_PK_SQL, False),
            ("explicit-null-table", T06_A2_NULL_TABLE_SQL, True),
            ("explicit-null-inline", T06_A2_NULL_INLINE_SQL, True),
        ):
            metadata_cases.append(a2_metadata_case(anchor_key, dialect, variant, sql, rejects))
        # A3 default-representation: the presence policy must still reject
        # only column `a`, while the server-side oracle distinguishes SQL
        # NULL (a and b store COLUMN_DEFAULT IS NULL) from the string
        # literals 'NULL' and '<nil>' via HEX bytes. The driver CREATE must
        # succeed even though the product audit rejects.
        a3_matrix_expect = audit_expect(T06_A3_MATRIX_SQL, True, loaded=1, finding={
            "rule_id": T06_DEFAULT_RULE,
            "message": 'column "a" should define a default value',
            "metadata": {"table": "t", "column": "a", "type": "varchar(8)"},
        })
        if anchor_key in ("mysql80", "mysql84"):
            a3_matrix_expect["instance_facts"] = dict(T06_GIPK_FACTS)
        metadata_cases.append({
            "id": f"t06-a3-{anchor_key}-default-representation",
            "anchor": anchor_key,
            "dialect": dialect,
            "sql": T06_A3_MATRIX_SQL,
            "policy": T06_A2_DEFAULT_PROFILE,
            "connect": dict(connects[anchor_key]),
            "args": ["--fail-on", "blocker"],
            "setup": [
                {
                    "name": "ensure t absent",
                    "sql": "DROP TABLE IF EXISTS t",
                    "expect_rc": 0,
                    "verify": [verify("t absent before audit", T06_TABLE_COUNT, "0")],
                },
            ],
            "expect": a3_matrix_expect,
            "post_verify": [
                verify("audit did not create t", T06_TABLE_COUNT, "0"),
            ],
            "execute": [
                {
                    "name": "driver applies the audited create",
                    "sql": T06_A3_MATRIX_SQL,
                    "expect_rc": 0,
                },
            ],
            "structure": [
                verify("exactly one user table", T06_TABLE_COUNT, "1"),
                verify("exactly four user columns", T06_COLUMN_COUNT, "4"),
                verify(
                    "four varchar(8) columns in declared order",
                    T06_A2_COLUMN_ROWS,
                    "a:varchar:1:YES,b:varchar:2:YES,c:varchar:3:YES,d:varchar:4:YES",
                ),
                verify(
                    "all four varchar lengths",
                    "SELECT GROUP_CONCAT(CHARACTER_MAXIMUM_LENGTH ORDER BY ORDINAL_POSITION) "
                    "FROM information_schema.COLUMNS "
                    "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t'",
                    "8,8,8,8",
                ),
                verify("no primary key constraint", T06_PK_CONSTRAINT, "0"),
                verify(
                    "default null flags and raw bytes",
                    T06_A3_DEFAULT_ROWS,
                    "a:1:-,b:1:-,c:0:4E554C4C,d:0:3C6E696C3E",
                ),
            ],
            "teardown": [
                {
                    "name": "drop fixture",
                    "sql": "DROP TABLE IF EXISTS t",
                    "expect_rc": 0,
                    "verify": [verify("no residual t", T06_TABLE_COUNT, "0")],
                },
            ],
        })
        # A3 null-drop-state: with the typed NULL fix the sibling DEFAULT NULL
        # column is provably independent of the dropped column, so the whole
        # three-statement batch stays complete/pass while the driver replays
        # each statement against the live anchor.
        a3_drop_expect = {
            "exit": 0,
            "verdict": "pass",
            "statements": 3,
            "findings": 0,
            "diagnostics": 0,
            "unsupported": 0,
            "coverage": "complete",
            "statement_coverage": ["complete", "complete", "complete"],
            "statement_sql": list(T06_A3_DROP_STATEMENTS),
            "statement_indices": [0, 1, 2],
            "evidence_gaps": 0,
            "fail_on_triggered": False,
            "rule_summary_loaded": 4,
        }
        if anchor_key in ("mysql80", "mysql84"):
            a3_drop_expect["instance_facts"] = dict(T06_GIPK_FACTS)
        metadata_cases.append({
            "id": f"t06-a3-{anchor_key}-null-drop-state",
            "anchor": anchor_key,
            "dialect": dialect,
            "sql": T06_A3_DROP_STATE_SQL,
            "policy": T06_A3_DROP_PROFILE,
            "connect": dict(connects[anchor_key]),
            "args": ["--fail-on", "blocker"],
            "setup": [
                {
                    "name": "ensure t absent",
                    "sql": "DROP TABLE IF EXISTS t",
                    "expect_rc": 0,
                    "verify": [verify("t absent before audit", T06_TABLE_COUNT, "0")],
                },
            ],
            "expect": a3_drop_expect,
            "post_verify": [
                verify("audit did not create t", T06_TABLE_COUNT, "0"),
            ],
            "execute": [
                {
                    "name": "driver applies audited create",
                    "sql": T06_A3_DROP_STATEMENTS[0],
                    "expect_rc": 0,
                    "verify": [
                        verify(
                            "three columns after create",
                            T06_A2_COLUMN_ROWS,
                            "id:int:1:NO,obsolete:int:2:YES,keep_c:varchar:3:YES",
                        ),
                        verify(
                            "keep_c stores SQL NULL default",
                            T06_A3_KEEPC_COLUMN,
                            "keep_c:varchar:8:YES:1",
                        ),
                        verify("primary key member", T06_PK_MEMBER, "id:1"),
                    ],
                },
                {
                    "name": "driver drops obsolete",
                    "sql": T06_A3_DROP_STATEMENTS[1],
                    "expect_rc": 0,
                    "verify": [
                        verify(
                            "obsolete dropped, order id/keep_c",
                            T06_A2_COLUMN_ROWS,
                            "id:int:1:NO,keep_c:varchar:2:YES",
                        ),
                        verify(
                            "keep_c still SQL NULL default",
                            T06_A3_KEEPC_COLUMN,
                            "keep_c:varchar:8:YES:1",
                        ),
                        verify("primary key member", T06_PK_MEMBER, "id:1"),
                    ],
                },
                {
                    "name": "driver creates index on keep_c",
                    "sql": T06_A3_DROP_STATEMENTS[2],
                    "expect_rc": 0,
                },
            ],
            "structure": [
                verify("exactly one user table", T06_TABLE_COUNT, "1"),
                verify(
                    "final columns id and keep_c",
                    T06_A2_COLUMN_ROWS,
                    "id:int:1:NO,keep_c:varchar:2:YES",
                ),
                verify(
                    "indexes PRIMARY(id) and idx_keep(keep_c)",
                    T06_A3_INDEX_ROWS,
                    "PRIMARY:id:1,idx_keep:keep_c:1",
                ),
            ],
            "teardown": [
                {
                    "name": "drop fixture",
                    "sql": "DROP TABLE IF EXISTS t",
                    "expect_rc": 0,
                    "verify": [verify("no residual t", T06_TABLE_COUNT, "0")],
                },
            ],
        })

        # T06-A4 (issue #85): provider default-identity on a live snapshot.
        # Unlike every earlier metadata case the physical CREATE lives in
        # setup, so the audited batch can only be answered from the real
        # information_schema read — the input AST carries no defaults at all.
        # Variants: drop-d-text-null = A (stored 'NULL' sibling stays
        # conservative → review/unverified), drop-c-text-nil = B (drop the
        # literal column itself, '<nil>' sibling also conservative), and
        # drop-d-null-control = C (stored SQL NULL sibling → pass/complete).
        for variant, setup_sql, setup_defaults, drop_sql, drop_statements, dropped, remaining_columns, remaining_defaults, expect in (
            (
                "drop-d-text-null", T06_A4_SETUP_AB_SQL, T06_A4_SETUP_DEFAULTS_AB,
                T06_A4_DROP_D_SQL, T06_A4_DROP_D_STATEMENTS, "d",
                T06_A4_AFTER_DROP_D_COLUMNS, T06_A4_AFTER_DROP_D_DEFAULTS_AB,
                {
                    "exit": 0,
                    "verdict": "review",
                    "statements": 2,
                    "findings": 0,
                    "diagnostics": 0,
                    "unsupported": 0,
                    "coverage": "unverified",
                    "statement_coverage": ["complete", "unverified"],
                    "statement_sql": list(T06_A4_DROP_D_STATEMENTS),
                    "statement_indices": [0, 1],
                    "evidence_gaps": 1,
                    "evidence_gap_entries": [{
                        "index": 1,
                        "rule_id": "ddl.create_index.columns.exists.require",
                        "reason_code": "unknown_table_state",
                        "required_facts": ["target_table.columns", "target_table.existence"],
                    }],
                    "fail_on_triggered": False,
                    "rule_summary_loaded": 3,
                },
            ),
            (
                "drop-c-text-nil", T06_A4_SETUP_AB_SQL, T06_A4_SETUP_DEFAULTS_AB,
                T06_A4_DROP_C_SQL, T06_A4_DROP_C_STATEMENTS, "c",
                T06_A4_AFTER_DROP_C_COLUMNS, T06_A4_AFTER_DROP_C_DEFAULTS_AB,
                {
                    "exit": 0,
                    "verdict": "review",
                    "statements": 2,
                    "findings": 0,
                    "diagnostics": 0,
                    "unsupported": 0,
                    "coverage": "unverified",
                    "statement_coverage": ["complete", "unverified"],
                    "statement_sql": list(T06_A4_DROP_C_STATEMENTS),
                    "statement_indices": [0, 1],
                    "evidence_gaps": 1,
                    "evidence_gap_entries": [{
                        "index": 1,
                        "rule_id": "ddl.create_index.columns.exists.require",
                        "reason_code": "unknown_table_state",
                        "required_facts": ["target_table.columns", "target_table.existence"],
                    }],
                    "fail_on_triggered": False,
                    "rule_summary_loaded": 3,
                },
            ),
            (
                "drop-d-null-control", T06_A4_SETUP_C_SQL, T06_A4_SETUP_DEFAULTS_C,
                T06_A4_DROP_D_SQL, T06_A4_DROP_D_STATEMENTS, "d",
                T06_A4_AFTER_DROP_D_COLUMNS, T06_A4_AFTER_DROP_D_DEFAULTS_C,
                {
                    "exit": 0,
                    "verdict": "pass",
                    "statements": 2,
                    "findings": 0,
                    "diagnostics": 0,
                    "unsupported": 0,
                    "coverage": "complete",
                    "statement_coverage": ["complete", "complete"],
                    "statement_sql": list(T06_A4_DROP_D_STATEMENTS),
                    "statement_indices": [0, 1],
                    "evidence_gaps": 0,
                    "fail_on_triggered": False,
                    "rule_summary_loaded": 3,
                },
            ),
        ):
            if anchor_key in ("mysql80", "mysql84"):
                expect["instance_facts"] = dict(T06_GIPK_FACTS)
            metadata_cases.append({
                "id": f"t06-a4-{anchor_key}-{variant}",
                "anchor": anchor_key,
                "dialect": dialect,
                "sql": drop_sql,
                "policy": T06_A4_PROFILE,
                "connect": dict(connects[anchor_key]),
                "args": ["--fail-on", "blocker"],
                "setup": [
                    {
                        "name": "ensure fixture absent",
                        "sql": f"DROP TABLE IF EXISTS {T06_A4_TABLE}",
                        "expect_rc": 0,
                        "verify": [verify("fixture absent before setup", T06_A4_TABLE_COUNT, "0")],
                    },
                    {
                        "name": "driver builds the audited initial state",
                        "sql": setup_sql,
                        "expect_rc": 0,
                        "verify": [
                            verify("five columns in declared order", T06_A4_COLUMN_ROWS, T06_A4_SETUP_COLUMNS),
                            verify("default null flags and raw bytes", T06_A4_DEFAULT_ROWS, setup_defaults),
                            verify("primary key member", T06_A4_PK_MEMBER, "id:1"),
                        ],
                    },
                ],
                "expect": expect,
                "post_verify": [
                    verify("audit did not change columns", T06_A4_COLUMN_ROWS, T06_A4_SETUP_COLUMNS),
                    verify("audit did not change defaults", T06_A4_DEFAULT_ROWS, setup_defaults),
                    verify("audit kept primary key member", T06_A4_PK_MEMBER, "id:1"),
                    verify("audit did not create idx_b", T06_A4_IDX_B_COUNT, "0"),
                ],
                "execute": [
                    {
                        "name": f"driver drops {dropped}",
                        "sql": drop_statements[0],
                        "expect_rc": 0,
                        "verify": [
                            verify(f"{dropped} dropped, order pinned", T06_A4_COLUMN_ROWS, remaining_columns),
                            verify("remaining defaults unchanged", T06_A4_DEFAULT_ROWS, remaining_defaults),
                            verify("primary key member", T06_A4_PK_MEMBER, "id:1"),
                        ],
                    },
                    {
                        "name": "driver creates index idx_b on b",
                        "sql": drop_statements[1],
                        "expect_rc": 0,
                    },
                ],
                "structure": [
                    verify("exactly one user table", T06_A4_TABLE_COUNT, "1"),
                    verify("final columns after replay", T06_A4_COLUMN_ROWS, remaining_columns),
                    verify("indexes PRIMARY(id) and idx_b(b)", T06_A4_INDEX_ROWS, T06_A4_FINAL_INDEXES),
                ],
                "teardown": [
                    {
                        "name": "drop fixture",
                        "sql": f"DROP TABLE IF EXISTS {T06_A4_TABLE}",
                        "expect_rc": 0,
                        "verify": [verify("no residual fixture", T06_A4_TABLE_COUNT, "0")],
                    },
                ],
            })

        # T06-A5 (issue #85): declared-length policy on a live connection.
        # The audited CREATE declares N under utf8mb4; the policy boundary
        # (reject for N=9, pass for 7/8) must coexist with the native CREATE
        # always succeeding — a team policy is not a database refusal. The
        # post-audit catalog proves the product never executed the DDL, then
        # the driver replays it and the structure oracle pins characters vs
        # octets as two independent fields.
        for kind_key, keyword, rule_id, profile, message in (
            ("char", "CHAR", T06_A5_CHAR_RULE, T06_A5_CHAR_PROFILE, T06_A5_CHAR_MESSAGE),
            ("varchar", "VARCHAR", T06_A5_VARCHAR_RULE, T06_A5_VARCHAR_PROFILE, T06_A5_VARCHAR_MESSAGE),
        ):
            for variant, n in (("below", 7), ("at", 8), ("above", 9)):
                sql = t06_a5_sql(keyword, n, anchored=True)
                expect = audit_expect(sql, n > 8, loaded=1, finding={
                    "rule_id": rule_id,
                    "message": message,
                    "metadata": {"table": "t", "column": "c", "limit": 8, "actual": 9},
                } if n > 8 else None)
                if anchor_key in ("mysql80", "mysql84"):
                    expect["instance_facts"] = dict(T06_GIPK_FACTS)
                structure_rows = [
                    verify("exactly one user table", T06_TABLE_COUNT, "1"),
                    verify("exactly one user column", T06_COLUMN_COUNT, "1"),
                    verify(
                        "column c identity",
                        T06_COLUMN_ROW,
                        f"c:{kind_key}:1:YES",
                    ),
                    verify(
                        "declared chars and octets are independent fields",
                        T06_A5_LENGTH_PAIR,
                        f"{n}:{4 * n}",
                    ),
                    verify(
                        "utf8mb4 charset and binary collation",
                        T06_A5_CHARSET,
                        "utf8mb4:utf8mb4_bin",
                    ),
                    verify("primary key constraint count", T06_PK_CONSTRAINT, "0"),
                    verify("primary index part count", T06_PK_PARTS, "0"),
                ]
                if anchor_key != "tidb85":
                    structure_rows.append(verify("storage engine", T06_ENGINE, "InnoDB"))
                metadata_cases.append({
                    "id": f"t06-a5-{anchor_key}-{kind_key}-{variant}",
                    "anchor": anchor_key,
                    "dialect": dialect,
                    "sql": sql,
                    "policy": profile,
                    "connect": dict(connects[anchor_key]),
                    "args": ["--fail-on", "blocker"],
                    "setup": [
                        {
                            "name": "ensure t absent",
                            "sql": "DROP TABLE IF EXISTS t",
                            "expect_rc": 0,
                            "verify": [verify("t absent before audit", T06_TABLE_COUNT, "0")],
                        },
                    ],
                    "expect": expect,
                    "post_verify": [
                        verify("audit did not create t", T06_TABLE_COUNT, "0"),
                    ],
                    "execute": [
                        {
                            "name": "driver applies the audited create",
                            "sql": sql,
                            "expect_rc": 0,
                        },
                    ],
                    "structure": structure_rows,
                    "teardown": [
                        {
                            "name": "drop fixture",
                            "sql": "DROP TABLE IF EXISTS t",
                            "expect_rc": 0,
                            "verify": [verify("no residual t", T06_TABLE_COUNT, "0")],
                        },
                    ],
                })

        # T06-A7 (issue #85): eight anchored roles separate product-side
        # declared-declaration facts from the native resolved metadata —
        # an empty product field is "not declared", never an observed
        # server default. The mismatch case is a true native negative
        # (ERROR 1253, table stays absent); every other role proves the
        # driver replay succeeds even under a product reject, because a
        # team policy is not a database refusal.
        a7_metadata_specs = (
            {
                "variant": "column-pair",
                "policy": T06_A7_CS_PROFILE,
                "sql": ("CREATE TABLE t (c VARCHAR(16) CHARACTER SET utf8mb4 "
                        "COLLATE utf8mb4_bin) CHARACTER SET utf8mb4 "
                        "COLLATE=utf8mb4_general_ci;"),
                "expect_reject": False,
                "finding": None,
                "native_rc": 0,
                "column_row": "c:varchar:1:YES",
                "length_pair": "16:64",
                "column_cc": "utf8mb4:utf8mb4_bin",
                "table_collate": "utf8mb4_general_ci",
            },
            {
                "variant": "column-charset-denied",
                "policy": T06_A7_CS_PROFILE,
                "sql": ("CREATE TABLE t (c VARCHAR(16) CHARACTER SET latin1 "
                        "COLLATE latin1_bin) CHARACTER SET utf8mb4 "
                        "COLLATE=utf8mb4_bin;"),
                "expect_reject": True,
                "finding": a7_charset_finding,
                "native_rc": 0,
                "column_row": "c:varchar:1:YES",
                "length_pair": "16:16",
                "column_cc": "latin1:latin1_bin",
                "table_collate": "utf8mb4_bin",
            },
            {
                "variant": "column-collation-denied",
                "policy": T06_A7_CC_PROFILE,
                "sql": ("CREATE TABLE t (c VARCHAR(16) CHARACTER SET utf8mb4 "
                        "COLLATE utf8mb4_general_ci) CHARACTER SET utf8mb4 "
                        "COLLATE=utf8mb4_bin;"),
                "expect_reject": True,
                "finding": a7_collation_finding,
                "native_rc": 0,
                "column_row": "c:varchar:1:YES",
                "length_pair": "16:64",
                "column_cc": "utf8mb4:utf8mb4_general_ci",
                "table_collate": "utf8mb4_bin",
            },
            {
                "variant": "column-partial",
                "policy": T06_A7_CM_PROFILE,
                "sql": ("CREATE TABLE t (c VARCHAR(16) COLLATE utf8mb4_bin) "
                        "CHARACTER SET latin1 COLLATE=latin1_bin;"),
                "expect_reject": True,
                "finding": a7_together_finding,
                "native_rc": 0,
                "column_row": "c:varchar:1:YES",
                "length_pair": "16:64",
                "column_cc": "utf8mb4:utf8mb4_bin",
                "table_collate": "latin1_bin",
            },
            {
                "variant": "column-mismatch",
                "policy": T06_A7_CM_PROFILE,
                "sql": ("CREATE TABLE t (c VARCHAR(16) CHARACTER SET utf8mb4 "
                        "COLLATE latin1_bin);"),
                "expect_reject": True,
                "finding": {
                    "rule_id": T06_A7_CM_RULE,
                    "message": T06_A7_CM_MISMATCH_ANCHOR_MESSAGE,
                    "metadata": {
                        "table": "t", "column": "c", "charset": "utf8mb4",
                        "collation": "latin1_bin",
                    },
                },
                "native_rc": 1,
                "native_stderr": ["ERROR 1253"],
                "column_row": None,
                "length_pair": None,
                "column_cc": None,
                "table_collate": None,
            },
            {
                "variant": "table-allowed",
                "policy": T06_A7_TC_PROFILE,
                "sql": "CREATE TABLE t (c VARCHAR(16)) COLLATE=utf8mb4_bin;",
                "expect_reject": False,
                "finding": None,
                "native_rc": 0,
                "column_row": "c:varchar:1:YES",
                "length_pair": "16:64",
                "column_cc": "utf8mb4:utf8mb4_bin",
                "table_collate": "utf8mb4_bin",
            },
            {
                "variant": "table-denied",
                "policy": T06_A7_TC_PROFILE,
                "sql": "CREATE TABLE t (c VARCHAR(16)) COLLATE=utf8mb4_general_ci;",
                "expect_reject": True,
                "finding": a7_table_finding,
                "native_rc": 0,
                "column_row": "c:varchar:1:YES",
                "length_pair": "16:64",
                "column_cc": "utf8mb4:utf8mb4_general_ci",
                "table_collate": "utf8mb4_general_ci",
            },
            {
                "variant": "table-inheritance",
                "policy": T06_A7_CM_PROFILE,
                "sql": ("CREATE TABLE t (c VARCHAR(16)) CHARACTER SET utf8mb4 "
                        "COLLATE=utf8mb4_bin;"),
                "expect_reject": False,
                "finding": None,
                "native_rc": 0,
                "column_row": "c:varchar:1:YES",
                "length_pair": "16:64",
                "column_cc": "utf8mb4:utf8mb4_bin",
                "table_collate": "utf8mb4_bin",
            },
        )
        for spec_row in a7_metadata_specs:
            expect = audit_expect(
                spec_row["sql"], spec_row["expect_reject"], loaded=1,
                finding=spec_row["finding"])
            if anchor_key in ("mysql80", "mysql84"):
                expect["instance_facts"] = dict(T06_GIPK_FACTS)
            case = {
                "id": f"t06-a7-{anchor_key}-{spec_row['variant']}",
                "anchor": anchor_key,
                "dialect": dialect,
                "sql": spec_row["sql"],
                "policy": spec_row["policy"],
                "connect": dict(connects[anchor_key]),
                "args": ["--fail-on", "blocker"],
                "setup": [
                    {
                        "name": "ensure t absent",
                        "sql": "DROP TABLE IF EXISTS t",
                        "expect_rc": 0,
                        "verify": [verify("t absent before audit", T06_TABLE_COUNT, "0")],
                    },
                ],
                "expect": expect,
                "post_verify": [
                    verify("audit did not create t", T06_TABLE_COUNT, "0"),
                ],
                "teardown": [
                    {
                        "name": "drop fixture",
                        "sql": "DROP TABLE IF EXISTS t",
                        "expect_rc": 0,
                        "verify": [verify("no residual t", T06_TABLE_COUNT, "0")],
                    },
                ],
            }
            if spec_row["native_rc"] == 0:
                case["execute"] = [
                    {
                        "name": "driver applies the audited create",
                        "sql": spec_row["sql"],
                        "expect_rc": 0,
                    },
                ]
                structure_rows = [
                    verify("exactly one user table", T06_TABLE_COUNT, "1"),
                    verify("exactly one user column", T06_COLUMN_COUNT, "1"),
                    verify("column c identity", T06_COLUMN_ROW,
                           spec_row["column_row"]),
                    verify(
                        "declared chars and octets are independent fields",
                        T06_A5_LENGTH_PAIR, spec_row["length_pair"]),
                    verify(
                        "resolved column charset and collation",
                        T06_A5_CHARSET, spec_row["column_cc"]),
                    verify(
                        "resolved table collation",
                        T06_A7_TABLE_COLLATION, spec_row["table_collate"]),
                    verify("primary key constraint count", T06_PK_CONSTRAINT, "0"),
                    verify("primary index part count", T06_PK_PARTS, "0"),
                ]
                if anchor_key != "tidb85":
                    structure_rows.append(verify("storage engine", T06_ENGINE, "InnoDB"))
                case["structure"] = structure_rows
            else:
                case["execute"] = [
                    {
                        "name": "driver rejects the mismatched collation",
                        "sql": spec_row["sql"],
                        "expect_rc": 1,
                        "stderr_contains": list(spec_row["native_stderr"]),
                        "verify": [
                            verify("t absent after rejected create",
                                   T06_TABLE_COUNT, "0"),
                        ],
                    },
                ]
            metadata_cases.append(case)

        # T06-A8 (issue #85): four anchored roles prove comment content is a
        # decoded string fact, not SQL literal text — the driver replays the
        # same audited CREATE under an explicit utf8mb4 session (recorded via
        # @@character_set_client stdout) and the structure oracle pins the
        # raw comment plus CHAR_LENGTH/OCTET_LENGTH/HEX so a missing row can
        # never masquerade as an empty comment. The expected HEX constants
        # below are frozen UTF-8 encodings of the declared literals, never
        # derived from product output.
        a8_metadata_specs = (
            {
                "variant": "column-empty",
                "policy": T06_A8_COLUMN_PROFILE,
                "sql": "CREATE TABLE t (c INT COMMENT '');",
                "expect_reject": True,
                "finding": a8_column_finding,
                "column_row": "c:int:1:YES",
                "table_comment": "",
                "table_stats": "0:0:",
                "column_comment": "",
                "column_stats": "0:0:",
            },
            {
                "variant": "column-decoded",
                "policy": T06_A8_COLUMN_PROFILE,
                "sql": ("CREATE TABLE t (c INT COMMENT 'it''s 中文') "
                        "COMMENT='表注';"),
                "expect_reject": False,
                "finding": None,
                "column_row": "c:int:1:YES",
                "table_comment": "表注",
                "table_stats": "2:6:E8A1A8E6B3A8",
                "column_comment": "it's 中文",
                "column_stats": "7:11:6974277320E4B8ADE69687",
            },
            {
                "variant": "table-rune-at",
                "policy": T06_A8_LENGTH_PROFILE,
                "sql": ("CREATE TABLE t (c INT COMMENT 'col') "
                        "COMMENT='中文注中文注ab';"),
                "expect_reject": False,
                "finding": None,
                "column_row": "c:int:1:YES",
                "table_comment": "中文注中文注ab",
                "table_stats": "8:20:E4B8ADE69687E6B3A8E4B8ADE69687E6B3A86162",
                "column_comment": "col",
                "column_stats": "3:3:636F6C",
            },
            {
                "variant": "table-rune-above",
                "policy": T06_A8_LENGTH_PROFILE,
                "sql": ("CREATE TABLE t (c INT COMMENT 'col') "
                        "COMMENT='中文注中文注abc';"),
                "expect_reject": True,
                "finding": a8_length_finding,
                "column_row": "c:int:1:YES",
                "table_comment": "中文注中文注abc",
                "table_stats": "9:21:E4B8ADE69687E6B3A8E4B8ADE69687E6B3A8616263",
                "column_comment": "col",
                "column_stats": "3:3:636F6C",
            },
        )
        for spec_row in a8_metadata_specs:
            expect = audit_expect(
                spec_row["sql"], spec_row["expect_reject"], loaded=1,
                finding=spec_row["finding"])
            if anchor_key in ("mysql80", "mysql84"):
                expect["instance_facts"] = dict(T06_GIPK_FACTS)
            structure_rows = [
                verify("exactly one user table", T06_TABLE_COUNT, "1"),
                verify("exactly one user column", T06_COLUMN_COUNT, "1"),
                verify("column c identity", T06_COLUMN_ROW,
                       spec_row["column_row"]),
                verify("raw table comment", T06_A8_TABLE_COMMENT_RAW,
                       spec_row["table_comment"]),
                verify("table comment chars:octets:hex",
                       T06_A8_TABLE_COMMENT_STATS, spec_row["table_stats"]),
                verify("raw column comment", T06_A8_COLUMN_COMMENT_RAW,
                       spec_row["column_comment"]),
                verify("column comment chars:octets:hex",
                       T06_A8_COLUMN_COMMENT_STATS, spec_row["column_stats"]),
                verify("primary key constraint count", T06_PK_CONSTRAINT, "0"),
                verify("primary index part count", T06_PK_PARTS, "0"),
            ]
            if anchor_key != "tidb85":
                structure_rows.append(verify("storage engine", T06_ENGINE, "InnoDB"))
            metadata_cases.append({
                "id": f"t06-a8-{anchor_key}-{spec_row['variant']}",
                "anchor": anchor_key,
                "dialect": dialect,
                "sql": spec_row["sql"],
                "policy": spec_row["policy"],
                "connect": dict(connects[anchor_key]),
                "args": ["--fail-on", "blocker"],
                "setup": [
                    {
                        "name": "ensure t absent",
                        "sql": "DROP TABLE IF EXISTS t",
                        "expect_rc": 0,
                        "verify": [verify("t absent before audit", T06_TABLE_COUNT, "0")],
                    },
                ],
                "expect": expect,
                "post_verify": [
                    verify("audit did not create t", T06_TABLE_COUNT, "0"),
                ],
                "execute": [
                    {
                        "name": "driver applies the audited create",
                        "sql": T06_A8_DELIVER_CHARSET + spec_row["sql"],
                        "expect_rc": 0,
                        "stdout_contains": ["utf8mb4"],
                    },
                ],
                "structure": structure_rows,
                "teardown": [
                    {
                        "name": "drop fixture",
                        "sql": "DROP TABLE IF EXISTS t",
                        "expect_rc": 0,
                        "verify": [verify("no residual t", T06_TABLE_COUNT, "0")],
                    },
                ],
            })

        # T06-A9 (issue #85): four anchored roles prove the driver stores the
        # declared audit columns — the product policy verdict (including the
        # single-role rejects B/C) never substitutes for native execution.
        # The oracle pins semantic booleans (frozen CURRENT_TIMESTAMP default
        # whitelist, normalized EXTRA for the ON UPDATE marker) while the raw
        # ordered column rows are recorded verbatim as an observation step.
        a9_pair_sql = (
            "CREATE TABLE t (id INT PRIMARY KEY, created_at DATETIME NOT NULL "
            "DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT "
            "CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP);"
        )
        a9_metadata_specs = (
            {
                "variant": "complete-pair",
                "sql": a9_pair_sql,
                "expect_reject": False,
                "finding": None,
                "column_list": "id:int:1:NO:-,created_at:datetime:2:NO:0,updated_at:datetime:3:NO:0",
                "column_count": "3",
                "roles": (("created_at", ""), ("updated_at", "on update current_timestamp")),
            },
            {
                "variant": "missing-created",
                "sql": ("CREATE TABLE t (id INT PRIMARY KEY, updated_at "
                        "DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP "
                        "ON UPDATE CURRENT_TIMESTAMP);"),
                "expect_reject": True,
                "finding": a9_created_finding,
                "column_list": "id:int:1:NO:-,updated_at:datetime:2:NO:0",
                "column_count": "2",
                "roles": (("updated_at", "on update current_timestamp"),),
            },
            {
                "variant": "missing-updated",
                "sql": ("CREATE TABLE t (id INT PRIMARY KEY, created_at "
                        "DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);"),
                "expect_reject": True,
                "finding": a9_updated_finding,
                "column_list": "id:int:1:NO:-,created_at:datetime:2:NO:0",
                "column_count": "2",
                "roles": (("created_at", ""),),
            },
            {
                "variant": "now-spelling",
                "sql": ("CREATE TABLE t (id INT PRIMARY KEY, created_at "
                        "DATETIME NOT NULL DEFAULT NOW(), updated_at "
                        "DATETIME NOT NULL DEFAULT NOW() ON UPDATE NOW());"),
                "expect_reject": False,
                "finding": None,
                "column_list": "id:int:1:NO:-,created_at:datetime:2:NO:0,updated_at:datetime:3:NO:0",
                "column_count": "3",
                "roles": (("created_at", ""), ("updated_at", "on update current_timestamp")),
            },
        )
        for spec_row in a9_metadata_specs:
            expect = audit_expect(
                spec_row["sql"], spec_row["expect_reject"], loaded=1,
                finding=spec_row["finding"])
            if anchor_key in ("mysql80", "mysql84"):
                expect["instance_facts"] = dict(T06_GIPK_FACTS)
            structure_rows = [
                verify("exactly one user table", T06_TABLE_COUNT, "1"),
                verify("exactly expected columns", T06_COLUMN_COUNT,
                       spec_row["column_count"]),
                verify("ordered column identity", T06_A9_COLUMN_LIST,
                       spec_row["column_list"]),
            ]
            for column, want_extra in spec_row["roles"]:
                structure_rows += [
                    verify(f"{column} default is frozen CURRENT_TIMESTAMP spelling",
                           t06_a9_column_default_ok(column), "1"),
                    verify(f"{column} extra normalized", t06_a9_extra_normalized(column),
                           want_extra),
                ]
            structure_rows += [
                verify("primary key constraint count", T06_PK_CONSTRAINT, "1"),
                verify("primary index part count", T06_PK_PARTS, "1"),
                verify("primary key member order", T06_PK_MEMBER, "id:1"),
                verify("no extra indexes",
                       "SELECT COUNT(*) FROM information_schema.STATISTICS "
                       "WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t' AND INDEX_NAME<>'PRIMARY'",
                       "0"),
                # Read-only scalar control: the same whitelist CASE over
                # legal and malformed literals must emit the frozen join —
                # proves the strict query, not a Python-side test oracle.
                verify("extra whitelist literal control",
                       T06_A9_EXTRA_LITERAL_SQL,
                       t06_a9_extra_literal_expect()),
            ]
            if anchor_key != "tidb85":
                structure_rows.append(verify("storage engine", T06_ENGINE, "InnoDB"))
            metadata_cases.append({
                "id": f"t06-a9-{anchor_key}-{spec_row['variant']}",
                "anchor": anchor_key,
                "dialect": dialect,
                "sql": spec_row["sql"],
                "policy": T06_A9_PROFILE,
                "connect": dict(connects[anchor_key]),
                "args": ["--fail-on", "blocker"],
                "setup": [
                    {
                        "name": "ensure t absent",
                        "sql": "DROP TABLE IF EXISTS t",
                        "expect_rc": 0,
                        "verify": [verify("t absent before audit", T06_TABLE_COUNT, "0")],
                    },
                ],
                "expect": expect,
                "post_verify": [
                    verify("audit did not create t", T06_TABLE_COUNT, "0"),
                ],
                "execute": [
                    {
                        "name": "driver applies the audited create",
                        "sql": spec_row["sql"],
                        "expect_rc": 0,
                    },
                    {
                        "name": "raw column rows observation",
                        "sql": T06_A9_COLUMN_RAW_ROWS,
                        "expect_rc": 0,
                    },
                ],
                "structure": structure_rows,
                "teardown": [
                    {
                        "name": "drop fixture",
                        "sql": "DROP TABLE IF EXISTS t",
                        "expect_rc": 0,
                        "verify": [verify("no residual t", T06_TABLE_COUNT, "0")],
                    },
                ],
            })

    required_case_ids = (
        [f"T06.db.{anchor}.{kind}" for anchor in T06_ANCHORS for kind in ("ddl", "syntax_negative")]
        + [f"T06.cli.{spec['id']}" for spec in cli_cases]
        + [f"T06.meta.{spec['id']}" for spec in metadata_cases]
    )

    return {
        "policy_profile": "all-rules-disabled",
        "policy_profiles": {
            T06_ISOLATED_PROFILE: {
                "enable": {
                    T06_PK_RULE: {"enabled": True, "level": "blocker", "params": {"required": True}}
                }
            },
            T06_REQUIRED_FALSE_PROFILE: {
                "enable": {
                    T06_PK_RULE: {"enabled": True, "level": "blocker", "params": {"required": False}}
                }
            },
            T06_A2_PK_NULL_PROFILE: {
                "enable": {
                    T06_PK_NN_RULE: {"enabled": True, "level": "blocker", "params": {"required": True}}
                }
            },
            T06_A2_DEFAULT_PROFILE: {
                "enable": {
                    T06_DEFAULT_RULE: {"enabled": True, "level": "blocker", "params": {"required": True}}
                }
            },
            T06_A3_DROP_PROFILE: {
                "enable": {
                    "ddl.table.exists.create.forbid": {"enabled": True, "level": "blocker", "params": {}},
                    "ddl.table.exists.alter.require": {"enabled": True, "level": "blocker", "params": {}},
                    "ddl.alter.drop_column.exists.require": {"enabled": True, "level": "blocker", "params": {}},
                    "ddl.create_index.columns.exists.require": {
                        "enabled": True, "level": "blocker", "params": {"required": True}
                    },
                }
            },
            # T06-A4: exactly the three blockers the implementation card
            # freezes — no create.forbid, since the audited input no longer
            # contains a CREATE at all.
            T06_A4_PROFILE: {
                "enable": {
                    "ddl.table.exists.alter.require": {"enabled": True, "level": "blocker", "params": {}},
                    "ddl.alter.drop_column.exists.require": {"enabled": True, "level": "blocker", "params": {}},
                    "ddl.create_index.columns.exists.require": {
                        "enabled": True, "level": "blocker", "params": {"required": True}
                    },
                }
            },
            # T06-A5: exactly one declared-length rule at limit=8 so a
            # blocker can only come from the rule under test. The shipped
            # defaults (char warning/limit 64, varchar blocker/limit 16383)
            # stay untouched — these profiles only isolate.
            T06_A5_CHAR_PROFILE: {
                "enable": {
                    T06_A5_CHAR_RULE: {"enabled": True, "level": "blocker", "params": {"limit": 8}},
                }
            },
            T06_A5_VARCHAR_PROFILE: {
                "enable": {
                    T06_A5_VARCHAR_RULE: {"enabled": True, "level": "blocker", "params": {"limit": 8}},
                }
            },
            # T06-A7: three single-rule column isolations plus the new
            # table collation allowlist under both require_explicit
            # values. The column rules keep their real parameter names —
            # charset/collation allowlists take `values`, the match rule
            # takes `required` — and the table rule ships disabled in the
            # default policy, so these profiles enable it explicitly.
            T06_A7_CS_PROFILE: {
                "enable": {
                    T06_A7_CS_RULE: {
                        "enabled": True, "level": "blocker",
                        "params": {"values": ["utf8mb4"]},
                    },
                }
            },
            T06_A7_CC_PROFILE: {
                "enable": {
                    T06_A7_CC_RULE: {
                        "enabled": True, "level": "blocker",
                        "params": {"values": ["utf8mb4_bin"]},
                    },
                }
            },
            T06_A7_CM_PROFILE: {
                "enable": {
                    T06_A7_CM_RULE: {
                        "enabled": True, "level": "blocker",
                        "params": {"required": True},
                    },
                }
            },
            T06_A7_TC_PROFILE: {
                "enable": {
                    T06_A7_TC_RULE: {
                        "enabled": True, "level": "blocker",
                        "params": {"values": ["utf8mb4_bin"],
                                   "require_explicit": False},
                    },
                }
            },
            T06_A7_TCR_PROFILE: {
                "enable": {
                    T06_A7_TC_RULE: {
                        "enabled": True, "level": "blocker",
                        "params": {"values": ["utf8mb4_bin"],
                                   "require_explicit": True},
                    },
                }
            },
            # T06-A8: three single-rule comment isolations. The require
            # rules keep their real `required` param and the length rule
            # keeps `limit` — shipped defaults stay untouched.
            T06_A8_TABLE_PROFILE: {
                "enable": {
                    T06_A8_TABLE_RULE: {
                        "enabled": True, "level": "blocker",
                        "params": {"required": True},
                    },
                }
            },
            T06_A8_COLUMN_PROFILE: {
                "enable": {
                    T06_A8_COLUMN_RULE: {
                        "enabled": True, "level": "blocker",
                        "params": {"required": True},
                    },
                }
            },
            T06_A8_LENGTH_PROFILE: {
                "enable": {
                    T06_A8_LENGTH_RULE: {
                        "enabled": True, "level": "blocker",
                        "params": {"limit": 8},
                    },
                }
            },
            # T06-A9: exactly the audit-columns rule at blocker — the frozen
            # two-role contract cannot be masked by any other rule.
            T06_A9_PROFILE: {
                "enable": {
                    T06_A9_RULE: {
                        "enabled": True, "level": "blocker",
                        "params": {"required": True},
                    },
                }
            },
        },
        "anchors": T06_ANCHORS,
        "ddl_steps": T06_DDL_STEPS,
        "syntax_negative": T06_SYNTAX_NEGATIVE,
        "cli_cases": cli_cases,
        "metadata_cases": metadata_cases,
        "required_case_ids": required_case_ids,
    }


def t06_a1_identity_map():
    """Frozen full-case-id → proof-role identity for T06-A1 (issue #85 R1):
    a required ID only counts when the record's self-declared kind, local
    case id, dialect, anchor, and policy profile equal the role that ID
    names — the presence of `T06.meta.t06-tidb85-no-pk` alone never proves
    the TiDB anchor executed that proof."""
    contract = t06_a1_contract()
    bound = {}
    for anchor_key in contract["anchors"]:
        bound[f"T06.db.{anchor_key}.ddl"] = {"kind": "db_ddl", "anchor": anchor_key}
        bound[f"T06.db.{anchor_key}.syntax_negative"] = {
            "kind": "db_syntax_negative", "anchor": anchor_key}
    for spec in contract["cli_cases"]:
        bound[f"T06.cli.{spec['id']}"] = {
            "kind": "cli_audit", "cli_case": spec["id"],
            "dialect": spec["dialect"], "sql": spec["sql"],
            "policy_profile": spec.get("policy") or contract["policy_profile"],
        }
    for spec in contract["metadata_cases"]:
        bound[f"T06.meta.{spec['id']}"] = {
            "kind": "cli_metadata", "cli_case": spec["id"],
            "dialect": spec["dialect"], "anchor": spec["anchor"], "sql": spec["sql"],
            "policy_profile": spec.get("policy") or contract["policy_profile"],
        }
    return bound


def t06_a1_identity_failures(artifact, manifest):
    """Bind every executed T06 record to the frozen proof role its full
    case_id names (issue #85 T06-A1-R1). Runs before kind-based dispatch so
    a record's self-declared kind/cli_case can never re-pick which manifest
    spec it satisfies: cross-anchor reuse, metadata slots refilled by
    offline records, isolated/required-false profile swaps, and duplicate
    or foreign executed records are all rejected here."""
    if manifest.get("task_id") != "T06":
        return []
    failures = []
    bound = t06_a1_identity_map()
    counts = {}
    for case in artifact.get("cases") or []:
        case_id = case.get("case_id")
        counts[case_id] = counts.get(case_id, 0) + 1
        role = bound.get(case_id)
        if role is None:
            continue  # the generic validator already reports unknown ids
        for field in ("kind", "cli_case", "dialect", "anchor", "policy_profile"):
            if field in role and case.get(field) != role[field]:
                failures.append(
                    f"T06-A1 {case_id}: identity {field} {case.get(field)!r} != frozen role {role[field]!r}")
        if "sql" in role and case.get("input_sql") != role["sql"]:
            failures.append(f"T06-A1 {case_id}: identity input_sql differs from the frozen statement")
    for case_id, count in sorted(counts.items()):
        if count > 1:
            failures.append(f"T06-A1 {case_id}: {count} duplicate executed records")
    return failures


def t06_a1_manifest_failures(manifest):
    if manifest.get("task_id") != "T06":
        return []
    contract = t06_a1_contract()
    failures = []
    if manifest.get("policy_profile") != contract["policy_profile"]:
        failures.append("T06-A1 default policy profile changed")
    if (manifest.get("policy") or {}).get("profiles") != contract["policy_profiles"]:
        failures.append("T06-A1 frozen policy profiles missing or changed")
    if manifest.get("anchors") != contract["anchors"]:
        failures.append("T06-A1 anchors differ from the frozen four-version fixture")
    if manifest.get("ddl_steps") != contract["ddl_steps"]:
        failures.append("T06-A1 baseline ddl_steps changed")
    if manifest.get("syntax_negative") != contract["syntax_negative"]:
        failures.append("T06-A1 baseline syntax_negative changed")
    required = manifest.get("required_case_ids") or []
    if required != contract["required_case_ids"]:
        failures.append("T06-A1 required_case_ids differ from the frozen 256-case denominator")
    for field, kind in (("cli_cases", "cli"), ("metadata_cases", "meta")):
        declared_list = [spec.get("id") for spec in manifest.get(field) or []]
        declared = {spec.get("id"): spec for spec in manifest.get(field) or []}
        if len(declared_list) != len(declared):
            failures.append(f"T06-A1 manifest {field} declares duplicate ids")
        frozen_ids = {spec["id"] for spec in contract[field]}
        extra_ids = sorted(set(declared) - frozen_ids)
        if extra_ids:
            failures.append(f"T06-A1 manifest {field} declares unfrozen ids {extra_ids}")
        for wanted in contract[field]:
            if declared.get(wanted["id"]) != wanted:
                failures.append(f"T06-A1 frozen oracle changed or missing: {wanted['id']}")
            if f"T06.{kind}.{wanted['id']}" not in required:
                failures.append(f"T06-A1 required case missing: {wanted['id']}")
    return failures


def t06_a1_artifact_failures(artifact, manifest):
    """T06-specific evidence assertions on top of the generic validator:
    statement identity (raw/normalized SQL, index, kind, no fabricated
    impact), exact finding identity (rule, level, message, location,
    metadata), summary counters, connectivity markers on stderr, and the
    product-reject/driver-accept coexistence for anchored no-PK cases."""
    if manifest.get("task_id") != "T06":
        return []
    contract = t06_a1_contract()
    frozen = {}
    for spec in contract["cli_cases"]:
        if spec["id"].startswith("t06-"):
            frozen[f"T06.cli.{spec['id']}"] = spec
    for spec in contract["metadata_cases"]:
        frozen[f"T06.meta.{spec['id']}"] = spec
    forbidden_stderr = (
        "Can't connect", "Access denied", "Unknown database",
        "Connection refused", "ERROR 1045", "ERROR 1044", "ERROR 1049",
        "ERROR 2002", "ERROR 2003", "ERROR 2005", "No such file",
    )
    failures = []
    for case in artifact.get("cases") or []:
        case_id = case.get("case_id")
        spec = frozen.get(case_id)
        if spec is None:
            continue
        actual = case.get("actual") or {}
        stderr = actual.get("stderr") or ""
        for marker in forbidden_stderr:
            if marker.lower() in stderr.lower():
                failures.append(f"T06-A1 {case_id}: stderr carries connectivity/permission marker {marker!r}")
        try:
            parsed = json.loads(actual.get("stdout") or "")
        except (json.JSONDecodeError, TypeError):
            continue  # the generic validator already reports unparseable stdout
        statements = parsed.get("statements") or []
        # Statement count and per-statement SQL identity derive from the
        # frozen spec — single-statement A1/A2 cases stay single, while the
        # A3 three-statement drop path must record all three verbatim.
        want_sql = spec["expect"].get("statement_sql") or [spec["sql"]]
        if len(statements) != len(want_sql):
            failures.append(f"T06-A1 {case_id}: statements {len(statements)} != frozen {len(want_sql)}")
            continue
        for idx, statement in enumerate(statements):
            want_raw = want_sql[idx]
            want_norm = want_raw[:-1] if want_raw.endswith(";") else want_raw
            if (statement.get("index") != idx or statement.get("kind") != "ddl"
                    or statement.get("raw_sql") != want_raw
                    or statement.get("normalized_sql") != want_norm):
                failures.append(f"T06-A1 {case_id}: statement {idx} identity mismatch: {statement!r}")
            if statement.get("impact") is not None:
                failures.append(f"T06-A1 {case_id}: statement {idx} impact must not be fabricated")
        findings = [f for s in statements for f in (s.get("findings") or [])]
        summary = parsed.get("summary") or {}
        rejects = spec["expect"]["exit"] == 1
        if rejects:
            entries_full = spec["expect"].get("finding_entries_full")
            if entries_full is not None:
                # Multi-finding contract (T06-A9): compare complete finding
                # entries as a multiset — statement index, statement kind,
                # rule, level, message, exact metadata map, and location per
                # entry. Ordering is normalized but multiplicities are
                # preserved, so duplicated or collapsed entries still fail.
                # Expected entries keep the frozen manifest shorthand
                # (index/line/column at top level); actual findings are
                # validated strictly — statement_kind must be present and
                # exactly "ddl", location must be a real object carrying
                # integer line/column, metadata must be a real map, and a
                # present statement_index must agree with its enclosing
                # statement (the omitempty-absent field stays legal as the
                # enclosing statement's index). No missing actual field is
                # ever back-filled from a default.
                indexed_findings = [
                    (s.get("index"), f)
                    for s in statements
                    for f in (s.get("findings") or [])
                ]
                for stmt_index, finding in indexed_findings:
                    if not isinstance(finding, dict):
                        failures.append(
                            f"T06-A1 {case_id}: finding is not an object: {finding!r}")
                        continue
                    if finding.get("statement_kind") != "ddl":
                        failures.append(
                            f"T06-A1 {case_id}: finding statement_kind must be present "
                            f"and 'ddl': {finding!r}")
                    if ("statement_index" in finding
                            and finding["statement_index"] != stmt_index):
                        failures.append(
                            f"T06-A1 {case_id}: finding statement_index "
                            f"{finding['statement_index']!r} disagrees with enclosing "
                            f"statement {stmt_index!r}")
                    location = finding.get("location")
                    if (not isinstance(location, dict)
                            or not isinstance(location.get("line"), int)
                            or not isinstance(location.get("column"), int)):
                        failures.append(
                            f"T06-A1 {case_id}: finding location missing or "
                            f"malformed: {location!r}")
                    if not isinstance(finding.get("metadata"), dict):
                        failures.append(
                            f"T06-A1 {case_id}: finding metadata missing or "
                            f"non-object: {finding.get('metadata')!r}")

                def canon_expect(entry):
                    return (
                        entry.get("index", 0), "ddl",
                        entry.get("rule_id"), entry.get("level"),
                        entry.get("message"),
                        entry.get("line"), entry.get("column"),
                        tuple(sorted((entry.get("metadata") or {}).items())),
                    )

                def canon_actual(pair):
                    stmt_index, finding = pair
                    if not isinstance(finding, dict):
                        return ("__malformed__", repr(finding))
                    location = finding.get("location")
                    metadata = finding.get("metadata")
                    return (
                        finding.get("statement_index", stmt_index),
                        finding.get("statement_kind"),
                        finding.get("rule_id"), finding.get("level"),
                        finding.get("message"),
                        location.get("line") if isinstance(location, dict) else None,
                        location.get("column") if isinstance(location, dict) else None,
                        tuple(sorted(metadata.items()))
                        if isinstance(metadata, dict) else None,
                    )
                got = sorted(repr(canon_actual(p)) for p in indexed_findings)
                want = sorted(repr(canon_expect(e)) for e in entries_full)
                if got != want:
                    failures.append(
                        f"T06-A1 {case_id}: finding multiset mismatch: got {got!r} want {want!r}")
                if len(findings) != len(entries_full):
                    failures.append(
                        f"T06-A1 {case_id}: findings {len(findings)} != {len(entries_full)}")
                if summary.get("blockers") != len(entries_full) or \
                        summary.get("warnings") != 0 or summary.get("notices") != 0:
                    failures.append(f"T06-A1 {case_id}: summary counters mismatch: {summary!r}")
            else:
                if len(findings) != 1:
                    failures.append(f"T06-A1 {case_id}: findings {len(findings)} != 1")
                else:
                    finding = findings[0]
                    location = finding.get("location") or {}
                    metadata = finding.get("metadata") or {}
                    want_rule = (spec["expect"].get("finding_entries") or [{}])[0].get("rule_id")
                    want_message = spec["expect"].get("finding_message")
                    want_meta = (spec["expect"].get("finding_metadata") or [{}])[0].get("metadata") or {}
                    if (finding.get("rule_id") != want_rule
                            or finding.get("level") != "blocker"
                            or finding.get("message") != want_message
                            or finding.get("statement_index", 0) != 0
                            or finding.get("statement_kind") != "ddl"
                            or location.get("line") != 1 or location.get("column") != 1
                            or any(metadata.get(k) != v for k, v in want_meta.items())):
                        failures.append(f"T06-A1 {case_id}: finding identity mismatch: {finding!r}")
                if summary.get("blockers") != 1 or summary.get("warnings") != 0 or summary.get("notices") != 0:
                    failures.append(f"T06-A1 {case_id}: summary counters mismatch: {summary!r}")
        else:
            if findings:
                failures.append(f"T06-A1 {case_id}: pass case carries findings {findings!r}")
            if summary.get("blockers") != 0 or summary.get("warnings") != 0 or summary.get("notices") != 0:
                failures.append(f"T06-A1 {case_id}: summary counters mismatch: {summary!r}")
        if parsed.get("global_findings"):
            failures.append(f"T06-A1 {case_id}: global findings must stay empty")
        if case.get("kind") == "cli_metadata":
            # Policy rejection never means the server refused the DDL: the
            # product exit and each driver return code must coexist exactly as
            # the frozen spec declares — including native negatives where the
            # product rejects AND the driver fails with the declared markers.
            execute = actual.get("execute") or []
            manifest_execs = spec.get("execute") or []
            if actual.get("exit") != spec["expect"]["exit"] or len(execute) != len(manifest_execs):
                failures.append(
                    f"T06-A1 {case_id}: product exit {actual.get('exit')!r} with {len(execute)} "
                    f"driver steps != frozen {spec['expect']['exit']}/{len(manifest_execs)}")
            for got, want in zip(execute, manifest_execs):
                if got.get("rc") != want.get("expect_rc", 0):
                    failures.append(
                        f"T06-A1 {case_id}: driver {want['name']} rc {got.get('rc')!r} != "
                        f"{want.get('expect_rc', 0)}")
    return failures


def validate_artifact(artifact, manifest, baseline=None, verify_binary=True):
    """Re-check an emitted artifact. Returns a list of failure strings."""
    failures = []
    required_top = ["task_id", "head_sha", "generated_at", "cli", "cases", "required_case_ids", "executed_count", "cleanup"]
    for field in required_top:
        if field not in artifact:
            failures.append(f"missing artifact field: {field}")
    if failures:
        return failures

    if baseline:
        failures.extend(baseline_manifest_failures(manifest, baseline))
    failures.extend(t05_a3_manifest_failures(manifest))
    failures.extend(t05_a4_manifest_failures(manifest))
    failures.extend(t05_a5_manifest_failures(manifest))
    failures.extend(t05_a6_manifest_failures(manifest))
    failures.extend(t06_a1_manifest_failures(manifest))
    failures.extend(t06_a1_identity_failures(artifact, manifest))
    failures.extend(t06_a1_artifact_failures(artifact, manifest))

    if artifact.get("external_blocker"):
        failures.append(f"external blocker recorded: {artifact['external_blocker']}")
    if artifact["task_id"] != manifest["task_id"]:
        failures.append(f"task_id mismatch: {artifact['task_id']!r} != {manifest['task_id']!r}")
    if artifact["required_case_ids"] != manifest["required_case_ids"]:
        failures.append("required_case_ids differ from manifest denominator")
    cases = artifact["cases"]
    if not cases:
        failures.append("zero executed cases")
    if artifact["executed_count"] != len(cases):
        failures.append(f"executed_count {artifact['executed_count']} != case records {len(cases)}")
    executed = {c.get("case_id") for c in cases}
    missing = [c for c in manifest["required_case_ids"] if c not in executed]
    if missing:
        failures.append(f"required cases not executed: {missing}")
    extra = [c for c in executed if c not in manifest["required_case_ids"]]
    if extra:
        failures.append(f"executed cases outside required set: {extra}")
    if baseline:
        failures.extend(baseline_executed_failures(artifact, executed, cases, baseline))

    cli = artifact["cli"]
    for field in ("path", "sha256", "build"):
        if field not in cli:
            failures.append(f"missing cli field: {field}")
    if "build" in cli:
        for field in ("go_version", "built_at", "cgo_enabled"):
            if field not in cli["build"]:
                failures.append(f"missing cli.build field: {field}")
        if cli["build"].get("head_sha") != artifact["head_sha"]:
            failures.append("cli.build.head_sha does not match artifact head_sha")
    if verify_binary and "path" in cli and "sha256" in cli:
        binary = pathlib.Path(cli["path"])
        if not binary.is_file():
            failures.append(f"cli binary missing on disk: {cli['path']} (stale or fabricated evidence)")
        else:
            actual_sha = hashlib.sha256(binary.read_bytes()).hexdigest()
            if actual_sha != cli["sha256"]:
                failures.append("cli binary sha256 mismatch (stale or fabricated evidence)")

    # Policy evidence: when the artifact carries a generated `policies` list,
    # every record must be complete — profile name, on-disk file, sha256,
    # catalog size, and an enabled map that agrees with both the manifest's
    # declared profile and the actual YAML on disk. The live `rules list`
    # catalog (from the recorded binary) pins the rule universe so a tampered
    # `catalog_rules` count cannot hide dropped or injected rules. Legacy
    # artifacts record only the artifact-level `policy_profile` dict (no
    # sha256): profile membership is still checked against it, but file
    # integrity is not — that shape predates the multi-profile contract and
    # its file may not survive.
    policies = artifact.get("policies") or []
    policy_by_name = {p.get("profile"): p for p in policies}
    legacy_policy = artifact.get("policy_profile")
    if isinstance(legacy_policy, dict) and legacy_policy.get("profile"):
        policy_by_name.setdefault(legacy_policy["profile"], legacy_policy)
    declared_enables = manifest_declared_policy_enables(manifest)
    # A named profile that collides with the reserved/default profile name
    # would silently redefine it in the declared map — fail closed at the
    # manifest level just like the runner does.
    reserved_profile_names = {manifest.get("policy_profile", "all-rules-disabled"), "all-rules-disabled"}
    for name in (manifest.get("policy") or {}).get("profiles") or {}:
        if name in reserved_profile_names:
            failures.append(f"manifest policy.profiles {name!r} collides with the default/reserved profile name")
    catalog_ids = None
    if policies and verify_binary and (cli.get("path") or "") and pathlib.Path(cli["path"]).is_file():
        crc, cout, cerr = run_cmd([cli["path"], "rules", "list", "--format", "json"], timeout=60)
        if crc != 0:
            failures.append(f"rules list for policy binding failed: {cerr.strip()}")
        else:
            try:
                catalog_ids = sorted({r["rule_id"] for r in json.loads(cout).get("rules", [])})
            except (json.JSONDecodeError, KeyError, TypeError):
                failures.append("rules list for policy binding returned unparseable catalog")
            if catalog_ids is not None and not catalog_ids:
                failures.append("rules list for policy binding returned empty catalog")
    for case in artifact.get("cases") or []:
        profile = case.get("policy_profile")
        if profile and profile not in policy_by_name:
            failures.append(f"case {case.get('case_id')}: policy_profile {profile!r} has no generated policy record")
    for profile, record in policy_by_name.items():
        if policies:
            for field in ("path", "sha256", "catalog_rules", "enabled_rules", "disabled_rules"):
                if field not in record:
                    failures.append(f"policy {profile}: missing generated-policy field {field}")
            if "path" not in record:
                continue
        elif "sha256" not in record:
            continue
        path = pathlib.Path(record.get("path") or "")
        if not path.is_file():
            failures.append(f"policy {profile}: file missing on disk: {path} (stale or fabricated evidence)")
            continue
        if sha256_file(path) != record.get("sha256"):
            failures.append(f"policy {profile}: sha256 mismatch (stale or fabricated evidence)")
        expected_enable = declared_enables.get(profile) or {}
        failures.extend(policy_semantics_failures(profile, record, path, expected_enable, catalog_ids))
    for name, enable_map in declared_enables.items():
        record = policy_by_name.get(name)
        if record is None:
            failures.append(f"manifest declares isolated profile {name!r} but artifact has no generated policy record")
        elif record.get("enabled_rules") != enable_map:
            failures.append(f"isolated policy {name!r} enabled_rules differ from manifest: {record.get('enabled_rules')!r} != {enable_map!r}")

    anchors = manifest["anchors"]
    covered_anchors = set()
    for case in cases:
        for field in ("case_id", "kind", "expected", "actual", "assertions", "status"):
            if field not in case:
                failures.append(f"case {case.get('case_id')}: missing field {field}")
                continue
        if case.get("status") != "pass":
            failures.append(f"case {case.get('case_id')}: status={case.get('status')!r}")
        assertions = case.get("assertions") or []
        if not assertions:
            failures.append(f"case {case.get('case_id')}: zero assertions")
        for assertion in assertions:
            if not assertion.get("ok"):
                failures.append(f"case {case.get('case_id')}: failed assertion {assertion.get('name')!r}: {assertion.get('detail')!r}")

        kind = case.get("kind")
        actual = case.get("actual") or {}
        case_id = case.get("case_id")
        expected = manifest_expected(manifest, case)
        if expected is None:
            failures.append(f"case {case_id}: cannot derive expected result from manifest for kind {kind!r}")
        elif case.get("expected") != expected:
            failures.append(f"case {case_id}: recorded expected differs from manifest-derived expectation")

        if kind == "db_ddl":
            db = actual.get("database") or {}
            anchor_key = case.get("anchor")
            covered_anchors.add(anchor_key)
            anchor = anchors.get(anchor_key)
            if anchor is None:
                failures.append(f"case {case_id}: unknown anchor {anchor_key!r}")
                continue
            want_sql = [s["sql"] for s in manifest["ddl_steps"]]
            if case.get("input_sql") != want_sql:
                failures.append(f"case {case_id}: recorded input_sql differs from manifest ddl_steps")
            if db.get("product") != anchor["product"]:
                failures.append(f"case {case_id}: product {db.get('product')!r} != {anchor['product']!r}")
            if not db.get("reachable"):
                failures.append(f"case {case_id}: database not reachable")
            if anchor["version_contains"] not in (db.get("version") or ""):
                failures.append(f"case {case_id}: version mismatch {db.get('version')!r} expected contains {anchor['version_contains']!r}")
            if not (db.get("image_digest") or "").startswith(anchor["image"].split(":")[0] + "@"):
                failures.append(f"case {case_id}: missing/mismatched image digest {db.get('image_digest')!r}")
            steps = actual.get("steps") or []
            if len(steps) != len(manifest["ddl_steps"]):
                failures.append(f"case {case_id}: step count {len(steps)} != manifest steps {len(manifest['ddl_steps'])}")
            for i, mstep in enumerate(manifest["ddl_steps"]):
                if i >= len(steps):
                    break
                step = steps[i]
                if step.get("name") != mstep["name"] or step.get("sql") != mstep["sql"]:
                    failures.append(f"case {case_id}: step {i} identity mismatch (name/sql differ from manifest)")
                if step.get("rc") != mstep["expect_rc"]:
                    failures.append(f"case {case_id}: step {mstep['name']} rc {step.get('rc')} != {mstep['expect_rc']}")
                verifies = step.get("verify") or []
                if len(verifies) != len(mstep["verify"]):
                    failures.append(f"case {case_id}: step {mstep['name']} verify count {len(verifies)} != manifest {len(mstep['verify'])}")
                for j, mverify in enumerate(mstep["verify"]):
                    if j >= len(verifies):
                        break
                    verify = verifies[j]
                    if verify.get("assert") != mverify["assert"] or verify.get("sql") != mverify["sql"]:
                        failures.append(f"case {case_id}: verify {j} identity mismatch (assert/sql differ from manifest)")
                    if verify.get("rc") != 0:
                        failures.append(f"case {case_id}: verify {mverify['assert']} rc {verify.get('rc')} != 0")
                    if verify.get("output") != mverify["expect"]:
                        failures.append(f"case {case_id}: verify {mverify['assert']} output {verify.get('output')!r} != {mverify['expect']!r}")
        elif kind == "db_syntax_negative":
            negative = manifest["syntax_negative"]
            if case.get("input_sql") != negative["sql"]:
                failures.append(f"case {case_id}: recorded input_sql differs from manifest syntax_negative")
            stderr = actual.get("stderr") or ""
            stdout = actual.get("stdout") or ""
            combined = stdout + stderr
            if actual.get("rc") == 0:
                failures.append(f"case {case_id}: negative returned success")
            match = re.search(r"ERROR\s+(\d+)", combined)
            observed_class = match.group(1) if match else ""
            if observed_class != negative["expect"]["error_class"]:
                failures.append(f"case {case_id}: error class {observed_class!r} != expected {negative['expect']['error_class']!r}")
            if "error_class" in actual and actual["error_class"] != observed_class:
                failures.append(f"case {case_id}: recorded error_class {actual['error_class']!r} disagrees with server output {observed_class!r}")
            for marker in negative["expect"].get("forbidden_markers") or []:
                if marker in combined:
                    failures.append(f"case {case_id}: forbidden marker present: {marker!r}")
        elif kind == "cli_audit":
            spec = cli_case_spec_for(manifest, case)
            if spec is None:
                failures.append(f"case {case_id}: no manifest cli case matches {case.get('cli_case') or case.get('dialect')!r}")
            else:
                if case.get("dialect") != spec.get("dialect"):
                    failures.append(f"case {case_id}: dialect {case.get('dialect')!r} != manifest spec {spec.get('dialect')!r}")
                if case.get("input_sql") != spec["sql"]:
                    failures.append(f"case {case_id}: recorded input_sql differs from manifest cli case")
            want_profile = expected_policy_profile(manifest, case, spec)
            if case.get("policy_profile") != want_profile:
                failures.append(f"case {case_id}: policy_profile {case.get('policy_profile')!r} differs from manifest-derived {want_profile!r}")
            command = case.get("command") or []
            if spec is not None:
                failures.extend(command_binding_failures(case, command, spec, policy_by_name, cli.get("path") or ""))
                cli_stdout_expect_failures(case, spec, failures)
            elif not command:
                failures.append(f"case {case_id}: no recorded command")
        elif kind == "cli_metadata":
            spec = metadata_case_spec_for(manifest, case)
            if spec is None:
                failures.append(f"case {case_id}: no manifest metadata case matches {case.get('cli_case')!r}")
                continue
            anchor_key = case.get("anchor")
            covered_anchors.add(anchor_key)
            anchor = anchors.get(anchor_key)
            if anchor is None or anchor_key != spec.get("anchor"):
                failures.append(f"case {case_id}: unknown/mismatched anchor {anchor_key!r}")
                continue
            if case.get("dialect") != spec.get("dialect"):
                failures.append(f"case {case_id}: dialect {case.get('dialect')!r} != manifest spec {spec.get('dialect')!r}")
            if case.get("input_sql") != spec["sql"]:
                failures.append(f"case {case_id}: recorded input_sql differs from manifest metadata case")
            want_profile = expected_policy_profile(manifest, case, spec)
            if case.get("policy_profile") != want_profile:
                failures.append(f"case {case_id}: policy_profile {case.get('policy_profile')!r} differs from manifest-derived {want_profile!r}")
            db = actual.get("database") or {}
            if db.get("product") != anchor["product"]:
                failures.append(f"case {case_id}: product {db.get('product')!r} != {anchor['product']!r}")
            if not db.get("reachable"):
                failures.append(f"case {case_id}: database not reachable")
            if anchor["version_contains"] not in (db.get("version") or ""):
                failures.append(f"case {case_id}: version mismatch {db.get('version')!r} expected contains {anchor['version_contains']!r}")
            if not (db.get("image_digest") or "").startswith(anchor["image"].split(":")[0] + "@"):
                failures.append(f"case {case_id}: missing/mismatched image digest {db.get('image_digest')!r}")
            command = case.get("command") or []
            conn = spec.get("connect") or {}
            failures.extend(command_binding_failures(case, command, spec, policy_by_name, cli.get("path") or "", connect=conn))
            secret = conn.get("password")
            if secret:
                # The password must travel via --password-env/--password-file
                # only. A token equal to the secret is legal solely as the
                # value of a declared non-secret slot (fixtures may set
                # user==password); a bare positional token or the value of a
                # password-source flag equal to the secret is a leak.
                value_flags = {"--host", "-H", "--port", "-P", "--user", "-u",
                               "--schema", "-D", "--dialect", "--sql",
                               "--config", "--format", "--fail-on"}
                password_flags = {"--password-env", "--password-file"}
                for pos, token in enumerate(command):
                    if token != secret:
                        continue
                    prev = command[pos - 1] if pos else ""
                    if prev in password_flags or prev not in value_flags:
                        failures.append(f"case {case_id}: command leaks the fixture password value")
                        break
            recorded_conn = case.get("connect") or {}
            if "password" in recorded_conn:
                failures.append(f"case {case_id}: recorded connect block leaks the password field")
            if recorded_conn.get("port") != conn.get("port") or recorded_conn.get("host") != conn.get("host"):
                failures.append(f"case {case_id}: recorded connect target differs from manifest")
            setups = actual.get("setup") or []
            manifest_setups = spec.get("setup") or []
            if len(setups) != len(manifest_setups):
                failures.append(f"case {case_id}: setup step count {len(setups)} != manifest {len(manifest_setups)}")
            for i, mstep in enumerate(manifest_setups):
                if i >= len(setups):
                    break
                step = setups[i]
                if step.get("sql") != mstep["sql"] or step.get("rc") != mstep.get("expect_rc", 0):
                    failures.append(f"case {case_id}: setup step {mstep['name']} identity/rc mismatch")
                verifies = step.get("verify") or []
                if len(verifies) != len(mstep.get("verify") or []):
                    failures.append(f"case {case_id}: setup {mstep['name']} verify count mismatch")
                for j, mverify in enumerate(mstep.get("verify") or []):
                    if j >= len(verifies):
                        break
                    verify = verifies[j]
                    if verify.get("assert") != mverify["assert"] or verify.get("sql") != mverify["sql"]:
                        failures.append(f"case {case_id}: setup verify {j} identity mismatch")
                    if verify.get("rc") != 0 or verify.get("output") != mverify["expect"]:
                        failures.append(f"case {case_id}: setup verify {mverify['assert']} output {verify.get('output')!r} != {mverify['expect']!r}")
            cli_stdout_expect_failures(case, spec, failures, anchor=anchor)
            # Declared instance facts must have been read live and recorded —
            # a missing or diverging record means the evidence was fabricated
            # or the read path silently broke (issue #83 T04-B).
            declared_facts = (spec.get("expect") or {}).get("instance_facts")
            if declared_facts is not None:
                recorded_facts = actual.get("instance_facts")
                if recorded_facts is None:
                    failures.append(f"case {case_id}: manifest declares instance_facts but none were recorded")
                else:
                    for fact_name, want in declared_facts.items():
                        if fact_name not in recorded_facts:
                            failures.append(f"case {case_id}: instance_facts missing declared key {fact_name!r}")
                        elif recorded_facts[fact_name] != want:
                            failures.append(f"case {case_id}: instance_facts[{fact_name!r}] {recorded_facts[fact_name]!r} != manifest {want!r}")
            posts = actual.get("post_verify") or []
            manifest_posts = spec.get("post_verify") or []
            if len(posts) != len(manifest_posts):
                failures.append(f"case {case_id}: post_verify count {len(posts)} != manifest {len(manifest_posts)}")
            for i, mcheck in enumerate(manifest_posts):
                if i >= len(posts):
                    break
                check = posts[i]
                if check.get("assert") != mcheck["assert"] or check.get("sql") != mcheck["sql"]:
                    failures.append(f"case {case_id}: post_verify {i} identity mismatch")
                if check.get("rc") != 0 or check.get("output") != mcheck["expect"]:
                    failures.append(f"case {case_id}: post_verify {mcheck['assert']} output {check.get('output')!r} != {mcheck['expect']!r}")
            # Driver-applied statements and the structure oracle are recorded
            # evidence, not decoration: every manifest-declared execute step
            # must appear in order with the expected return code, and every
            # structure query must appear in order with its exact expected
            # output. A dropped or rewritten step is a fabricated oracle.
            execs = actual.get("execute") or []
            manifest_execs = spec.get("execute") or []
            if len(execs) != len(manifest_execs):
                failures.append(f"case {case_id}: execute step count {len(execs)} != manifest {len(manifest_execs)}")
            for i, mstep in enumerate(manifest_execs):
                if i >= len(execs):
                    break
                step = execs[i]
                if step.get("name") != mstep["name"] or step.get("sql") != mstep["sql"]:
                    failures.append(f"case {case_id}: execute step {mstep['name']} identity mismatch")
                if step.get("rc") != mstep.get("expect_rc", 0):
                    failures.append(f"case {case_id}: execute step {mstep['name']} rc {step.get('rc')} != expected {mstep.get('expect_rc', 0)}")
                for field in ("stderr", "stdout"):
                    for marker in mstep.get(field + "_contains") or []:
                        if marker not in (step.get(field) or ""):
                            failures.append(f"case {case_id}: execute {mstep['name']} missing {field} marker {marker!r}")
                verifies = step.get("verify") or []
                if len(verifies) != len(mstep.get("verify") or []):
                    failures.append(f"case {case_id}: execute {mstep['name']} verify count mismatch")
                for j, mverify in enumerate(mstep.get("verify") or []):
                    if j >= len(verifies):
                        break
                    verify = verifies[j]
                    if verify.get("assert") != mverify["assert"] or verify.get("sql") != mverify["sql"]:
                        failures.append(f"case {case_id}: execute verify {j} identity mismatch")
                    if verify.get("rc") != 0 or verify.get("output") != mverify["expect"]:
                        failures.append(f"case {case_id}: execute verify {mverify['assert']} output mismatch")
            structures = actual.get("structure") or []
            manifest_structure = spec.get("structure") or []
            if len(structures) != len(manifest_structure):
                failures.append(f"case {case_id}: structure query count {len(structures)} != manifest {len(manifest_structure)}")
            for i, mcheck in enumerate(manifest_structure):
                if i >= len(structures):
                    break
                check = structures[i]
                if check.get("assert") != mcheck["assert"] or check.get("sql") != mcheck["sql"]:
                    failures.append(f"case {case_id}: structure {i} identity mismatch")
                if check.get("rc") != 0 or check.get("output") != mcheck["expect"]:
                    failures.append(f"case {case_id}: structure {mcheck['assert']} output {check.get('output')!r} != {mcheck['expect']!r}")
            teardowns = actual.get("teardown") or []
            manifest_teardowns = spec.get("teardown") or []
            if len(teardowns) != len(manifest_teardowns):
                failures.append(f"case {case_id}: teardown count {len(teardowns)} != manifest {len(manifest_teardowns)}")
            for i, mstep in enumerate(manifest_teardowns):
                if i >= len(teardowns):
                    break
                step = teardowns[i]
                if step.get("sql") != mstep["sql"] or step.get("rc") != mstep.get("expect_rc", 0):
                    failures.append(f"case {case_id}: teardown {mstep['name']} identity/rc mismatch")
                verifies = step.get("verify") or []
                if len(verifies) != len(mstep.get("verify") or []):
                    failures.append(f"case {case_id}: teardown {mstep['name']} verify count mismatch")
                for j, mverify in enumerate(mstep.get("verify") or []):
                    if j >= len(verifies):
                        break
                    verify = verifies[j]
                    if verify.get("assert") != mverify["assert"] or verify.get("sql") != mverify["sql"]:
                        failures.append(f"case {case_id}: teardown verify {j} identity mismatch")
                    if verify.get("rc") != 0 or verify.get("output") != mverify["expect"]:
                        failures.append(f"case {case_id}: teardown verify {mverify['assert']} output {verify.get('output')!r} != {mverify['expect']!r}")
        elif kind == "cli_error":
            spec = error_case_spec_for(manifest, case)
            if spec is None:
                failures.append(f"case {case_id}: no manifest error case matches {case.get('cli_case')!r}")
                continue
            if case.get("dialect") != spec.get("dialect"):
                failures.append(f"case {case_id}: dialect {case.get('dialect')!r} != manifest spec {spec.get('dialect')!r}")
            if case.get("input_sql") != spec["sql"]:
                failures.append(f"case {case_id}: recorded input_sql differs from manifest error case")
            want_profile = expected_policy_profile(manifest, case, spec)
            if case.get("policy_profile") != want_profile:
                failures.append(f"case {case_id}: policy_profile {case.get('policy_profile')!r} differs from manifest-derived {want_profile!r}")
            command = case.get("command") or []
            failures.extend(command_binding_failures(case, command, spec, policy_by_name, cli.get("path") or ""))
            if actual.get("exit") != spec["expect"]["exit"]:
                failures.append(f"case {case_id}: exit {actual.get('exit')} != expected {spec['expect']['exit']}")
            for marker in spec["expect"].get("stderr_contains") or []:
                if marker not in (actual.get("stderr") or ""):
                    failures.append(f"case {case_id}: stderr missing marker {marker!r}")
            # An input/provider error path never emits an audit result — a
            # verdict payload in stdout means an error was laundered into a
            # normal (or gap-shaped) audit record.
            stdout_blob = None
            try:
                stdout_blob = json.loads(actual.get("stdout") or "")
            except (json.JSONDecodeError, TypeError):
                stdout_blob = None
            if isinstance(stdout_blob, dict) and "verdict" in stdout_blob:
                failures.append(f"case {case_id}: error case stdout carries an audit result")
            error_anchor_key = spec.get("anchor")
            error_anchor = anchors.get(error_anchor_key) if error_anchor_key else None
            if error_anchor_key and error_anchor is None:
                failures.append(f"case {case_id}: manifest error case references unknown anchor {error_anchor_key!r}")
            if error_anchor is not None:
                if case.get("anchor") != error_anchor_key:
                    failures.append(f"case {case_id}: recorded anchor {case.get('anchor')!r} != manifest {error_anchor_key!r}")
                # A mismatch/conflict without observed evidence is
                # indistinguishable from a fabricated error — the raw banner
                # and its canonical form must both be present.
                if not (actual.get("observed_banner") or "").strip():
                    failures.append(f"case {case_id}: anchored error case missing observed_banner evidence")
            failures.extend(version_evidence_failures(case, spec, None, anchor=error_anchor))
        else:
            failures.append(f"case {case_id}: unknown kind {kind!r}")

    for anchor_key in anchors:
        if anchor_key not in covered_anchors:
            failures.append(f"required anchor {anchor_key} skipped (no executed db_ddl case)")

    cleanup = artifact["cleanup"]
    if cleanup.get("compose_down_rc") != 0:
        failures.append(f"cleanup compose down failed rc={cleanup.get('compose_down_rc')}")
    if cleanup.get("residual_containers"):
        failures.append(f"cleanup residual containers: {cleanup['residual_containers']}")
    return failures


def cmd_run(args):
    manifest = load_manifest(args.task)
    baseline = load_baseline()
    baseline_errors = baseline_manifest_failures(manifest, baseline)
    if baseline_errors:
        fail_run(f"manifest violates locked baseline: {baseline_errors}")
    artifact_root = pathlib.Path(args.artifact_dir).resolve() / args.task
    artifact_root.mkdir(parents=True, exist_ok=True)
    cases_dir = artifact_root / "cases"
    cases_dir.mkdir(exist_ok=True)

    head_rc, head_sha, _ = run_cmd(["git", "rev-parse", "HEAD"], cwd=ROOT_DIR)
    if head_rc != 0:
        fail_run("cannot resolve git HEAD")
    head_sha = head_sha.strip()

    rc, _, _ = run_cmd(["docker", "version"], timeout=30)
    if rc != 0:
        fail_run("Docker unavailable (external blocker)")

    cli = build_cli(artifact_root / "bin")
    cli["build"]["head_sha"] = head_sha
    policies = make_policies(cli["path"], artifact_root, manifest)

    artifact = {
        "task_id": manifest["task_id"],
        "issue": manifest.get("issue"),
        "head_sha": head_sha,
        "generated_at": utc_now(),
        "cli": cli,
        "policy_profile": policies[manifest.get("policy_profile", "all-rules-disabled")],
        "policies": [policies[name] for name in sorted(policies)],
        "required_case_ids": list(manifest["required_case_ids"]),
        "cases": [],
        "executed_count": 0,
        "cleanup": {},
    }

    services = []
    containers = []
    for anchor in manifest["anchors"].values():
        services.append(anchor["service"])
        containers.append(anchor["container"])
        if anchor.get("client_service"):
            services.append(anchor["client_service"])
            containers.append(anchor["client_container"])

    try:
        rc, out, err = compose("up", "-d", *services, timeout=300)
        if rc != 0:
            artifact["external_blocker"] = f"compose up failed: {err.strip()}"
        else:
            health = wait_for_healthy(containers)
            artifact["container_health"] = health
            unhealthy = {c: s for c, s in health.items() if s not in ("healthy", "running")}
            if unhealthy:
                artifact["external_blocker"] = f"required database containers not healthy: {unhealthy}"
            else:
                for anchor_key in manifest["anchors"]:
                    artifact["cases"].append(execute_ddl_case(manifest, anchor_key))
                    artifact["cases"].append(execute_negative_case(manifest, anchor_key))
                for spec in cli_case_specs(manifest):
                    artifact["cases"].append(execute_cli_case(manifest, cli["path"], policies, spec))
                for spec in metadata_case_specs(manifest):
                    artifact["cases"].append(execute_metadata_case(manifest, spec["anchor"], cli["path"], policies, spec))
                for spec in error_case_specs(manifest):
                    artifact["cases"].append(execute_error_case(manifest, cli["path"], policies, spec))
    finally:
        artifact["cleanup"] = compose_cleanup()
        artifact["executed_count"] = len(artifact["cases"])
        artifact["generated_at"] = utc_now()

    artifact_path = artifact_root / "artifact.json"
    with artifact_path.open("w", encoding="utf-8") as handle:
        json.dump(artifact, handle, indent=2, ensure_ascii=False)
        handle.write("\n")
    for case in artifact["cases"]:
        raw_path = cases_dir / f"{case['case_id']}.json"
        with raw_path.open("w", encoding="utf-8") as handle:
            json.dump(case, handle, indent=2, ensure_ascii=False)
            handle.write("\n")

    failures = validate_artifact(artifact, manifest, baseline=baseline)
    executed = artifact["executed_count"]
    assertions = sum(len(c.get("assertions") or []) for c in artifact["cases"])
    passed = sum(1 for c in artifact["cases"] if c.get("status") == "pass")
    print(f"[ddl-golden] task={args.task} head={head_sha[:12]} cases={executed} passed={passed} assertions={assertions}")
    print(f"[ddl-golden] artifact={artifact_path}")
    if failures:
        for item in failures:
            print(f"[ddl-golden][FAIL] {item}", file=sys.stderr)
        return 1
    print("[ddl-golden] PASS")
    return 0


def cmd_validate(args):
    artifact_path = pathlib.Path(args.artifact)
    if not artifact_path.is_file():
        fail_run(f"artifact not found: {artifact_path}")
    with artifact_path.open("r", encoding="utf-8") as handle:
        artifact = json.load(handle)
    task = artifact.get("task_id") or args.task
    manifest = load_manifest(task)
    baseline = load_baseline()
    failures = validate_artifact(artifact, manifest, baseline=baseline, verify_binary=not args.no_binary_check)
    if failures:
        for item in failures:
            print(f"[ddl-golden][FAIL] {item}", file=sys.stderr)
        return 1
    print("[ddl-golden] artifact valid")
    return 0


def main():
    parser = argparse.ArgumentParser(description="DeltaScope DDL golden-path runner")
    sub = parser.add_subparsers(dest="command", required=True)
    run = sub.add_parser("run")
    run.add_argument("--task", required=True)
    run.add_argument("--artifact-dir", required=True)
    val = sub.add_parser("validate")
    val.add_argument("--artifact", required=True)
    val.add_argument("--task", default="")
    val.add_argument("--no-binary-check", action="store_true")
    args = parser.parse_args()
    if args.command == "run":
        return cmd_run(args)
    return cmd_validate(args)


if __name__ == "__main__":
    sys.exit(main())
