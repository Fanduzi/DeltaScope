import hashlib
import json
import pathlib
import subprocess

root = pathlib.Path(__file__).resolve().parents[5]
bundle = pathlib.Path(__file__).resolve().parent.parent
product = "83125653d7cfc4343221d6f47e89df4311101efe"
original = "adc1f7322e1c70524435ed687fe27b67b931abb5"
artifact = json.loads((bundle / "golden/T17.artifact.json").read_text())
assert artifact["head_sha"] == artifact["cli"]["build"]["head_sha"] == product
for case in artifact["cases"]:
    snapshot = bundle / "golden/cases" / (case["case_id"] + ".json")
    assert json.loads(snapshot.read_text()) == case, snapshot
for record in artifact["policies"]:
    snapshot = bundle / "golden" / pathlib.Path(record["path"]).name
    assert hashlib.sha256(snapshot.read_bytes()).hexdigest() == record["sha256"], snapshot
sources = list((bundle / "sources/current").glob("*.go.txt"))
assert {source.name for source in sources} == {
    "batch_state_t17r1_proof_test.go.txt", "batch_state_t17r1_test.go.txt",
    "metadata_object_validation_postgresql_tag_test.go.txt",
}
for source in sources:
    name = source.name.removesuffix(".txt")
    expected = subprocess.check_output(
        ["git", "show", product + ":internal/application/audit/" + name], cwd=root)
    assert source.read_bytes() == expected, source
assert (bundle / "sources/current/T17.json").read_bytes() == subprocess.check_output(
    ["git", "show", product + ":testdata/ddl-golden/T17.json"], cwd=root)
old_evidence = "docs/verification/t17-database-schema-lifecycle/"
assert subprocess.check_output(
    ["git", "diff", "175ab0e1", "--", old_evidence], cwd=root) == b""
for stem in ("ddl-golden-final", "decision-record-final", "l2-committed-final",
             "red-cleanup", "pg-unit-test-gates", "pg-confidence-gates"):
    for suffix in ("command", "stdout", "stderr", "rc"):
        assert (bundle / "gates" / f"{stem}.{suffix}.txt").is_file()
    assert (bundle / "gates" / f"{stem}.rc.txt").read_text().strip() == "0", stem
proof_name = "batch_state_t17r1_proof_test.go.txt"
assert (bundle / "sources/red" / proof_name).read_bytes() == (bundle / "sources/current" / proof_name).read_bytes()
assert not list(bundle.rglob("*.go")), "archived sources must not create Go packages"
red_receipt = (bundle / "gates/red-before-cleanup.stdout.txt").read_text()
assert original in red_receipt
assert "tracked unstaged clean" in red_receipt and "tracked staged clean" in red_receipt
assert (bundle / "gates/golden-original-before.sha256").read_bytes() == (bundle / "gates/golden-original-after.sha256").read_bytes()
assert (bundle / "gates/golden-precommit-before.sha256").read_bytes() == (bundle / "gates/golden-precommit-after.sha256").read_bytes()
print("Verified: product-bound artifacts/policies/cases, exact source snapshots, successful final gates, pristine red product, original evidence/artifact preservation, and non-buildable archive packaging.")
