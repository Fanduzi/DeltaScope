import hashlib
import json
import pathlib
import subprocess
import sys

root = pathlib.Path("/Users/fan/GolangProjects/DeltaScope")
artifact_path = pathlib.Path(sys.argv[1])
expected_sha = sys.argv[2]
artifact = json.loads(artifact_path.read_text())
manifest = json.loads((root / "testdata/ddl-golden/T17.json").read_text())
old = json.loads(subprocess.check_output(
    ["git", "show", "adc1f732:testdata/ddl-golden/T17.json"], cwd=root))
required = manifest["required_case_ids"]
cases = artifact["cases"]
executed = [case["case_id"] for case in cases]
assert len(required) == len(set(required)) == 52
assert len(executed) == len(set(executed)) == 52
assert set(required) == set(executed) == set(artifact["required_case_ids"])
assert artifact["executed_count"] == 52
assert artifact["head_sha"] == artifact["cli"]["build"]["head_sha"] == expected_sha
binary = pathlib.Path(artifact["cli"]["path"])
assert hashlib.sha256(binary.read_bytes()).hexdigest() == artifact["cli"]["sha256"]
assert all(case["status"] == "pass" for case in cases)
assert all(assertion["ok"] is True for case in cases for assertion in case["assertions"])
assert artifact["cleanup"]["compose_down_rc"] == 0
assert artifact["cleanup"]["residual_containers"] == []
for field in ("cli_cases", "metadata_cases"):
    current = {case["id"]: case for case in manifest[field]}
    for original in old[field]:
        assert current[original["id"]] == original, original["id"]
for field in ("anchors", "ddl_steps", "syntax_negative"):
    assert manifest[field] == old[field], field
assert set(old["required_case_ids"]) < set(required)
new_ids = set(required) - set(old["required_case_ids"])
new_cases = {}
for case in cases:
    if case["kind"] in ("cli_audit", "cli_metadata"):
        raw = json.loads(case["actual"]["stdout"])
        assert raw == case["actual"]["parsed"], case["case_id"]
    if case["case_id"] not in new_ids:
        continue
    raw = json.loads(case["actual"]["stdout"])
    assert len(raw["statements"]) == 4
    new_cases[case["case_id"]] = {
        "exit": case["actual"]["exit"], "verdict": raw["verdict"],
        "coverage": raw["coverage"]["status"],
        "statements": len(raw["statements"]),
        "findings": sum(len(s.get("findings", [])) for s in raw["statements"]),
        "gaps": sum(len(s.get("evidence_gaps", [])) for s in raw["statements"]),
        "assertions": len(case["assertions"]),
    }
print(json.dumps({
    "head_sha": expected_sha, "original_cases_preserved": 47,
    "required_cases": len(required), "executed_cases": len(executed),
    "assertions": sum(len(case["assertions"]) for case in cases),
    "case_mix": {kind: sum(c["kind"] == kind for c in cases)
                 for kind in sorted({c["kind"] for c in cases})},
    "new_cases": new_cases, "cleanup": {
        "rc": artifact["cleanup"]["compose_down_rc"],
        "residual_containers": artifact["cleanup"]["residual_containers"],
    },
}, indent=2))
