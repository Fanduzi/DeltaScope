"""Check the exact T16/T17 regression receipts required by the T18 contract."""
import hashlib
import json
import pathlib
import subprocess

ROOT = pathlib.Path("/Users/fan/GolangProjects/DeltaScope")
BASE = "7f67c5de42fb6096d9c80f51e2d0f217eb34e528"
PRODUCT = "879c49f58d8df02120875fffc901dc2d672210f0"
reports = []
for task, cases_expected, assertions_expected in (("T16", 60, 832), ("T17", 52, 834)):
    relative = f"testdata/ddl-golden/{task}.json"
    original = subprocess.check_output(["git", "show", f"{BASE}:{relative}"], cwd=ROOT)
    frozen = subprocess.check_output(["git", "show", f"{PRODUCT}:{relative}"], cwd=ROOT)
    assert original == frozen == (ROOT / relative).read_bytes()
    manifest = json.loads(frozen)
    path = pathlib.Path(f"/tmp/ddl-golden-t18-final-{task}-879c49f5/{task}/artifact.json")
    artifact = json.loads(path.read_bytes())
    assert artifact["task_id"] == task and artifact["head_sha"] == PRODUCT
    required = manifest["required_case_ids"]
    cases = artifact["cases"]
    executed = [case["case_id"] for case in cases]
    assert len(required) == len(set(required)) == cases_expected
    assert len(executed) == len(set(executed)) == cases_expected
    assert set(required) == set(executed) == set(artifact["required_case_ids"])
    assert len(artifact["required_case_ids"]) == cases_expected
    assert artifact["executed_count"] == len(cases)
    assert all(case["status"] == "pass" and case["assertions"] for case in cases)
    assert all(assertion["ok"] is True for case in cases for assertion in case["assertions"])
    total = sum(len(case["assertions"]) for case in cases)
    assert total == assertions_expected
    binary = pathlib.Path(artifact["cli"]["path"])
    assert hashlib.sha256(binary.read_bytes()).hexdigest() == artifact["cli"]["sha256"]
    for policy in artifact["policies"]:
        assert hashlib.sha256(pathlib.Path(policy["path"]).read_bytes()).hexdigest() == policy["sha256"]
    assert artifact["cleanup"]["compose_down_rc"] == 0
    assert artifact["cleanup"]["residual_containers"] == []
    reports.append({
        "task": task,
        "head_sha": PRODUCT,
        "manifest_sha256": hashlib.sha256(frozen).hexdigest(),
        "manifest_byte_identical_to_accepted_base": True,
        "artifact": str(path),
        "artifact_sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
        "executed": len(cases),
        "assertions": total,
        "required_executed_sets_equal": True,
        "duplicates": [],
        "cli_sha256": artifact["cli"]["sha256"],
        "cleanup_rc": 0,
        "residual_containers": [],
    })
print(json.dumps(reports, indent=2))
