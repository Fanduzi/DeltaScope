"""Lead-owned independent accounting over frozen T18 execution receipts."""
import argparse
import collections
import hashlib
import json
import pathlib
import re

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("artifact", type=pathlib.Path)
parser.add_argument("manifest", type=pathlib.Path)
parser.add_argument("expected_head")
parser.add_argument("--phase", choices=("wip", "final"), required=True)
args = parser.parse_args()
assert re.fullmatch(r"[0-9a-f]{40}", args.expected_head)
manifest_bytes = args.manifest.read_bytes()
manifest_sha = hashlib.sha256(manifest_bytes).hexdigest()
assert manifest_sha == "fef55d43bff8f0aa90e06fc55655afedbd6bf448a06e5ecfb2edee5d423ad3ac"
manifest = json.loads(manifest_bytes)
artifact = json.loads(args.artifact.read_bytes())
assert artifact["task_id"] == "T18" and artifact["head_sha"] == args.expected_head
required = manifest["required_case_ids"]
cases = artifact["cases"]
executed = [case["case_id"] for case in cases]
assert len(required) == len(set(required)) == 149
assert len(executed) == len(set(executed)) == len(required)
assert set(executed) == set(required) == set(artifact["required_case_ids"])
assert len(artifact["required_case_ids"]) == len(required)
assert artifact["executed_count"] == len(cases)
assert all(case["status"] == "pass" for case in cases)
assert all(case["assertions"] for case in cases)
assert all(assertion["ok"] is True for case in cases for assertion in case["assertions"])
assert artifact["cleanup"]["compose_down_rc"] == 0
assert artifact["cleanup"]["residual_containers"] == []
binary = pathlib.Path(artifact["cli"]["path"])
assert binary.is_file()
binary_sha = hashlib.sha256(binary.read_bytes()).hexdigest()
assert binary_sha == artifact["cli"]["sha256"]
policies = []
for policy in artifact["policies"]:
    assert hashlib.sha256(pathlib.Path(policy["path"]).read_bytes()).hexdigest() == policy["sha256"]
    assert policy["catalog_rules"] == policy["disabled_rules"] + len(policy["enabled_rules"])
    policies.append({"profile": policy["profile"], "sha256": policy["sha256"]})
versions = {}
coverage = collections.Counter()
verdicts = collections.Counter()
for case in cases:
    actual = case["actual"]
    if "database" in actual:
        database = actual["database"]
        anchor = manifest["anchors"][case["anchor"]]
        assert database["reachable"] is True
        assert database["product"] == anchor["product"]
        assert anchor["version_contains"] in database["version"]
        assert "sha256:" in database["image_digest"]
        observed = {key: database[key] for key in ("product", "version", "image", "image_digest")}
        assert case["anchor"] not in versions or versions[case["anchor"]] == observed
        versions[case["anchor"]] = observed
    if case["kind"] in ("cli_audit", "cli_metadata"):
        assert actual["parsed"] == json.loads(actual["stdout"])
        coverage[actual["parsed"]["coverage"]["status"]] += 1
        verdicts[actual["parsed"]["verdict"]] += 1
assert set(versions) == set(manifest["anchors"])
report = {
    "proof_layer": "CLI and native catalog/DDL; companion Go proofs counted separately",
    "phase": args.phase,
    "head_sha": artifact["head_sha"],
    "head_alone_binds_dirty_source": False if args.phase == "wip" else None,
    "artifact": str(args.artifact),
    "artifact_sha256": hashlib.sha256(args.artifact.read_bytes()).hexdigest(),
    "manifest_sha256": manifest_sha,
    "required": len(required),
    "executed": len(cases),
    "assertions": sum(len(case["assertions"]) for case in cases),
    "required_executed_sets_equal": True,
    "duplicate_ids": [],
    "cases_by_kind": dict(collections.Counter(case["kind"] for case in cases)),
    "cli_coverage": dict(coverage),
    "cli_verdicts": dict(verdicts),
    "cli_sha256": binary_sha,
    "policies": policies,
    "anchors": versions,
    "cleanup_rc": 0,
    "residual_containers": [],
}
print(json.dumps(report, indent=2))
