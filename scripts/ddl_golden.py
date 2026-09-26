#!/usr/bin/env python3
# input: task manifest testdata/ddl-golden/<TASK>.json, docker/ddl-golden-compose.yaml, freshly built deltascope CLI
# output: inspectable golden artifact (artifact.json + per-case raw evidence) validated against the manifest
# pos: DDL golden-path runner and artifact validator behind `make ddl-golden TASK=Txx ARTIFACT_DIR=...`
# note: if this file changes, update this header and module README.md.
"""DeltaScope DDL golden-path runner (milestone T02/#81).

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


def make_all_off_policy(binary, work_dir, profile_name):
    rc, out, err = run_cmd([binary, "rules", "list", "--format", "json"], timeout=60)
    if rc != 0:
        fail_run(f"rules list failed: {err.strip()}")
    catalog = json.loads(out)
    ids = sorted({r["rule_id"] for r in catalog.get("rules", [])})
    if not ids:
        fail_run("rules catalog is empty; cannot build isolation profile")
    lines = ["rules:"]
    lines += [f"  {json.dumps(rid)}:\n    enabled: false" for rid in ids]
    policy = work_dir / "golden-policy.yaml"
    policy.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return {"path": str(policy), "disabled_rules": len(ids), "profile": profile_name}


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
    checks = [
        ("exit code", parsed is not None and rc == expect["exit"], f"rc={rc} expected={expect['exit']}"),
        ("verdict", parsed is not None and parsed.get("verdict") == expect["verdict"], f"verdict={parsed.get('verdict') if parsed else None!r}"),
        ("top-level statements", parsed is not None and len(statements) == expect["statements"], f"statements={len(statements)} expected={expect['statements']}"),
        ("zero findings", parsed is not None and findings == expect["findings"], f"findings={findings}"),
        ("no diagnostics", parsed is not None and len(diagnostics) == expect["diagnostics"], f"diagnostics={diagnostics!r}"),
        ("no unsupported", parsed is not None and len(unsupported) == expect["unsupported"], f"unsupported={unsupported!r}"),
    ]
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
        # Entries may additionally pin "metadata": the matching actual entry
        # must carry exactly that bounded metadata map.
        for e in expect["unsupported_entries"]:
            if "metadata" not in e:
                continue
            key = (e.get("index", 0), e.get("feature"), e.get("reason"))
            match = [u for u in unsupported if (u.get("index", 0), u.get("feature"), u.get("reason")) == key]
            checks.append((
                f"unsupported metadata {key}",
                parsed is not None and len(match) == 1 and match[0].get("metadata") == e["metadata"],
                f"entry={match!r} expected_metadata={e['metadata']!r}",
            ))
    if "statement_sql" in expect:
        got = [s.get("raw_sql") for s in statements]
        checks.append(("statement raw SQL identity", parsed is not None and got == expect["statement_sql"], f"raw_sql={got!r} expected={expect['statement_sql']!r}"))
    # Every unsupported entry carries the original statement SQL under the
    # current public contract; when either side records it, the two must
    # agree at the entry's statement index. Synthetic artifacts that omit
    # both skip this check.
    for u in unsupported:
        idx = u.get("index", 0)
        bound = statements[idx].get("raw_sql") if 0 <= idx < len(statements) else None
        if u.get("sql") is None and bound is None:
            continue
        checks.append((
            f"unsupported sql identity index={idx}",
            parsed is not None and u.get("sql") == bound,
            f"sql={u.get('sql')!r} expected={bound!r}",
        ))
    return checks


def execute_cli_case(manifest, binary, policy, spec):
    case_id = f"{manifest['task_id']}.cli.{spec['id']}"
    args = [
        binary, "audit",
        "--dialect", spec["dialect"],
        "--sql", spec["sql"],
        "--config", policy["path"],
        "--format", "json",
    ]
    args.extend(spec.get("args") or [])
    rc, out, err = run_cmd(args, cwd=ROOT_DIR, timeout=120)
    case = {
        "case_id": case_id,
        "kind": "cli_audit",
        "cli_case": spec["id"],
        "dialect": spec["dialect"],
        "input_sql": spec["sql"],
        "policy_profile": policy["profile"],
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
    for name, ok, detail in cli_case_expect_checks(parsed, rc, spec["expect"]):
        case["assertions"].append({"name": name, "ok": bool(ok), "detail": detail})
    if not all(a["ok"] for a in case["assertions"]):
        case["status"] = "fail"
    return case


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
    return None


def cli_case_spec_for(manifest, case):
    """Find the manifest CLI spec a recorded case claims to satisfy."""
    case_name = case.get("cli_case") or case.get("dialect")
    for spec in cli_case_specs(manifest):
        if spec.get("id") == case_name:
            return spec
    return None


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
            if case.get("policy_profile") not in (None, manifest.get("policy_profile", "all-rules-disabled")):
                failures.append(f"case {case_id}: policy_profile {case.get('policy_profile')!r} differs from manifest")
            command = case.get("command") or []
            if "--dialect" not in command or case.get("dialect") not in command or "--config" not in command:
                failures.append(f"case {case_id}: recorded command lacks --dialect/--config evidence")
            raw = actual.get("stdout")
            try:
                reparsed = json.loads(raw) if isinstance(raw, str) else None
            except json.JSONDecodeError:
                reparsed = None
            if reparsed is None:
                failures.append(f"case {case_id}: CLI stdout is not JSON")
            elif spec is not None:
                recorded_parsed = actual.get("parsed")
                if recorded_parsed is not None and recorded_parsed != reparsed:
                    failures.append(f"case {case_id}: recorded parsed disagrees with raw stdout")
                expect = spec["expect"]
                for name, ok, detail in cli_case_expect_checks(reparsed, actual.get("exit"), expect):
                    if not ok:
                        failures.append(f"case {case_id}: {name}: {detail}")
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
    policy = make_all_off_policy(cli["path"], artifact_root, manifest.get("policy_profile", "all-rules-disabled"))

    artifact = {
        "task_id": manifest["task_id"],
        "issue": manifest.get("issue"),
        "head_sha": head_sha,
        "generated_at": utc_now(),
        "cli": cli,
        "policy_profile": policy,
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
                    artifact["cases"].append(execute_cli_case(manifest, cli["path"], policy, spec))
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
