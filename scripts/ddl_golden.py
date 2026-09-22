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


def execute_cli_case(manifest, binary, policy, dialect):
    spec = manifest["cli_audit"]
    case_id = f"{manifest['task_id']}.cli.{dialect}"
    args = [
        binary, "audit",
        "--dialect", dialect,
        "--sql", spec["sql"],
        "--config", policy["path"],
        "--format", "json",
    ]
    rc, out, err = run_cmd(args, cwd=ROOT_DIR, timeout=120)
    case = {
        "case_id": case_id,
        "kind": "cli_audit",
        "dialect": dialect,
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
    expect = spec["expect"]
    statements = (parsed or {}).get("statements", [])
    findings = sum(len(s.get("findings", [])) for s in statements) + len((parsed or {}).get("global_findings", []))
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
    for name, ok, detail in checks:
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


def validate_artifact(artifact, manifest, verify_binary=True):
    """Re-check an emitted artifact. Returns a list of failure strings."""
    failures = []
    required_top = ["task_id", "head_sha", "generated_at", "cli", "cases", "required_case_ids", "executed_count", "cleanup"]
    for field in required_top:
        if field not in artifact:
            failures.append(f"missing artifact field: {field}")
    if failures:
        return failures

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
        expected = case.get("expected") or {}
        if kind == "db_ddl":
            db = actual.get("database") or {}
            anchor_key = case.get("anchor")
            covered_anchors.add(anchor_key)
            anchor = anchors.get(anchor_key)
            if not db.get("reachable"):
                failures.append(f"case {case.get('case_id')}: database not reachable")
            if anchor:
                if anchor["version_contains"] not in (db.get("version") or ""):
                    failures.append(f"case {case.get('case_id')}: version mismatch {db.get('version')!r} expected contains {anchor['version_contains']!r}")
                if not (db.get("image_digest") or "").startswith(anchor["image"].split(":")[0] + "@"):
                    failures.append(f"case {case.get('case_id')}: missing/mismatched image digest {db.get('image_digest')!r}")
            for step, step_expected in zip(actual.get("steps") or [], expected.get("steps") or []):
                if step.get("rc") != step_expected.get("rc"):
                    failures.append(f"case {case.get('case_id')}: step {step.get('name')} rc {step.get('rc')} != {step_expected.get('rc')}")
                for verify, verify_expected in zip(step.get("verify") or [], step_expected.get("verify") or []):
                    if verify.get("output") != verify_expected.get("expect"):
                        failures.append(f"case {case.get('case_id')}: verify {verify_expected.get('assert')} output {verify.get('output')!r} != {verify_expected.get('expect')!r}")
            if len(actual.get("steps") or []) != len(expected.get("steps") or []):
                failures.append(f"case {case.get('case_id')}: step count mismatch")
        elif kind == "db_syntax_negative":
            stderr = actual.get("stderr") or ""
            stdout = actual.get("stdout") or ""
            combined = stdout + stderr
            if actual.get("rc") == 0:
                failures.append(f"case {case.get('case_id')}: negative returned success")
            if (expected.get("error_class") or "") not in combined:
                failures.append(f"case {case.get('case_id')}: expected error class {expected.get('error_class')!r} absent from server output")
            for marker in expected.get("forbidden_markers") or []:
                if marker in combined:
                    failures.append(f"case {case.get('case_id')}: forbidden marker present: {marker!r}")
        elif kind == "cli_audit":
            parsed = actual.get("parsed")
            if parsed is None:
                failures.append(f"case {case.get('case_id')}: CLI stdout is not JSON")
            else:
                statements = parsed.get("statements") or []
                if actual.get("exit") != expected.get("exit"):
                    failures.append(f"case {case.get('case_id')}: exit {actual.get('exit')} != {expected.get('exit')}")
                if parsed.get("verdict") != expected.get("verdict"):
                    failures.append(f"case {case.get('case_id')}: verdict {parsed.get('verdict')!r} != {expected.get('verdict')!r}")
                if len(statements) != expected.get("statements"):
                    failures.append(f"case {case.get('case_id')}: statements {len(statements)} != {expected.get('statements')}")
                findings = sum(len(s.get("findings", [])) for s in statements) + len(parsed.get("global_findings") or [])
                if findings != expected.get("findings"):
                    failures.append(f"case {case.get('case_id')}: findings {findings} != {expected.get('findings')}")
                if len(parsed.get("diagnostics") or []) != expected.get("diagnostics"):
                    failures.append(f"case {case.get('case_id')}: unexpected diagnostics")
                if len(parsed.get("unsupported") or []) != expected.get("unsupported"):
                    failures.append(f"case {case.get('case_id')}: unexpected unsupported")
        else:
            failures.append(f"case {case.get('case_id')}: unknown kind {kind!r}")

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
                for dialect in manifest["cli_audit"]["dialects"]:
                    artifact["cases"].append(execute_cli_case(manifest, cli["path"], policy, dialect))
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

    failures = validate_artifact(artifact, manifest)
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
    failures = validate_artifact(artifact, manifest, verify_binary=not args.no_binary_check)
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
