import hashlib
import json
import pathlib
import subprocess
import sys

root = pathlib.Path("/Users/fan/GolangProjects/DeltaScope")
artifact_path = pathlib.Path(sys.argv[1])
product = sys.argv[2]
receipts = pathlib.Path(sys.argv[3])
artifact = json.loads(artifact_path.read_text())
manifest_bytes = (root / "testdata/ddl-golden/T17.json").read_bytes()
assert manifest_bytes == subprocess.check_output(
    ["git", "show", "9da9f8e0:testdata/ddl-golden/T17.json"], cwd=root)
manifest = json.loads(manifest_bytes)
required = manifest["required_case_ids"]
executed = [c["case_id"] for c in artifact["cases"]]
assert len(required) == len(set(required)) == len(executed) == len(set(executed)) == 52
assert set(required) == set(executed) == set(artifact["required_case_ids"])
assert artifact["executed_count"] == 52
assert artifact["head_sha"] == artifact["cli"]["build"]["head_sha"] == product
binary = pathlib.Path(artifact["cli"]["path"])
assert hashlib.sha256(binary.read_bytes()).hexdigest() == artifact["cli"]["sha256"]
assert all(c["status"] == "pass" for c in artifact["cases"])
assert all(a["ok"] is True for c in artifact["cases"] for a in c["assertions"])
assert sum(len(c["assertions"]) for c in artifact["cases"]) == 834
assert artifact["cleanup"]["compose_down_rc"] == 0
assert artifact["cleanup"]["residual_containers"] == []
for case in artifact["cases"]:
    if case["kind"] in ("cli_audit", "cli_metadata"):
        assert json.loads(case["actual"]["stdout"]) == case["actual"]["parsed"]


def proofs(path):
    records = {}
    for line in path.read_text().splitlines():
        if "T17TAIL_PROOF " in line:
            record = json.loads(line.split("T17TAIL_PROOF ", 1)[1])
            assert record["test"] not in records
            records[record["test"]] = record
    return records


red = proofs(receipts / "red/proof.stdout.txt")
green = proofs(receipts / "gates/ddl-golden.stdout.txt")
assert set(red) == set(green) == {
    "TestT17R1TailCreateProjection", "TestT17R1TailAlterProjection"}
summary = {}
for test in sorted(red):
    old, new = red[test], green[test]
    for record in (old, new):
        version = record["parsed_version"]
        assert version["product"] == "mysql" and version["version"] == "5.7.44"
        assert version["validated_range"] is True
        target = record["second_metadata"]["schema_target"]
        assert target["exists"] is True and target["status"] == "confirmed"
        assert target["attributes"]["charset"] == "utf8mb4"
        assert record["provider_after"] == record["provider_original_fixture"]
        assert record["object_calls"] == 1
        assert record["final_schema_entry"]["attrs"] == {
            "charset": "utf8mb4", "collation": "utf8mb4_bin"}
    before = old["second_metadata"]["schema_target"]["attributes"]["collation"]
    after = new["second_metadata"]["schema_target"]["attributes"]["collation"]
    assert before == "utf8mb4_0900_ai_ci"
    assert after == "utf8mb4_general_ci"
    summary[test] = {"baseline": before, "fixed": after, "version": "mysql/5.7.44"}
native = (receipts / "native/defaults.stdout.txt").read_text()
for line in (
    "ds17tail_create\tutf8mb4\tutf8mb4_0900_ai_ci",
    "ds17tail_alter\tutf8mb4\tutf8mb4_0900_ai_ci",
    "ds17tail_general\tutf8mb4\tutf8mb4_general_ci",
    "ds17tail_alter\tutf8mb4\tutf8mb4_general_ci",
    "ds17tail_server\tlatin1\tlatin1_bin",
):
    assert line in native, line
historical = subprocess.check_output(
    ["git", "show", "9da9f8e0:docs/work-plans/t17-database-schema-lifecycle/probes/p17_schema_matrix.log"],
    cwd=root, text=True)
assert "=== mysql57 :: charset-lifecycle-steps" in historical
assert "p17a\tutf8mb4\tutf8mb4_general_ci" in historical
print(json.dumps({
    "product": product,
    "manifest_byte_identical_to": "9da9f8e02221b1e51bf275cf012ffc2ac88b594a",
    "required": 52, "executed": 52, "passing_cli_db_assertions": 834,
    "tail_cli_case_increase": 0,
    "tail_projection_proofs_separate_from_cli_assertions": summary,
    "business_probe": "MySQL 8.0.46 configurable CREATE/ALTER and server defaults confirmed",
    "cleanup_rc": 0, "residual_task_containers": [],
}, indent=2))
