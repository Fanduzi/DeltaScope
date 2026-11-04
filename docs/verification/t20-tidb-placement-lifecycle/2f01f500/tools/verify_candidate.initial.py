#!/usr/bin/env python3
"""Lead's independent identity/count checks over a completed candidate."""

import argparse
from collections import Counter
import hashlib
import json
from pathlib import Path
import subprocess


ROOT = Path("/Users/fan/GolangProjects/DeltaScope")
BASE = "6ac1e918b3558d6d611d7d8a59a770d73341c2f9"
HISTORY = "docs/verification/t20-tidb-placement-lifecycle"
REQUIRED_GATES = [
    "01-test", "02-vet", "03-race", "04-pg", "05-sql-corpus",
    "06-query-access-corpus", "07-inventory", "08-coverage-catalog",
    "09-parser-unsupported", "10-diagnostic-evidence", "11-privacy",
    "12-docs-examples", "13-decision-record", "14-l2",
    "15-golden-validator", "16-lint-scoped", "17-lint-fullrepo",
    "18-lint-identity", "19-golden", "20-artifact-validator",
]


def git(*args):
    return subprocess.check_output(["git", *args], cwd=ROOT, text=True).strip()


def must(condition, description):
    if not condition:
        raise ValueError(description)


def historical_blobs(ref):
    rows = git("ls-tree", "-r", "--full-tree", ref, "--", HISTORY).splitlines()
    return {line.split("\t", 1)[1]: line.split("\t", 1)[0] for line in rows}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--product", required=True)
    parser.add_argument("--evidence-dir", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    product = git("rev-parse", "--verify", args.product + "^{commit}")
    must(not git("diff", "--name-only"), "tracked working-tree changes after product freeze")
    must(not git("diff", "--cached", "--name-only"), "index must be clear for evidence verification")
    directory = Path(args.evidence_dir).resolve()
    artifact_path = directory / "golden/T20/artifact.json"
    artifact = json.loads(artifact_path.read_text())
    manifest = json.loads(git("show", f"{product}:testdata/ddl-golden/T20.json"))
    old = json.loads(git("show", f"{BASE}:testdata/ddl-golden/T20.json"))
    must(artifact["head_sha"] == product, "artifact product identity")
    must(artifact["cli"]["build"]["head_sha"] == product, "CLI build product identity")
    binary = Path(artifact["cli"]["path"])
    actual_hash = hashlib.sha256(binary.read_bytes()).hexdigest()
    must(actual_hash == artifact["cli"]["sha256"], "CLI actual bytes do not match artifact")

    for key in ("anchors", "ddl_steps", "syntax_negative", "policy", "policy_profile"):
        must(manifest[key] == old[key], f"historical manifest contract changed: {key}")
    for key in ("metadata_cases", "cli_cases"):
        current = {case["id"]: case for case in manifest[key]}
        must(len(current) == len(manifest[key]), f"duplicate {key} manifest ID")
        for case in old[key]:
            must(current.get(case["id"]) == case, f"original case changed: {case['id']}")
    cases = artifact["cases"]
    ids = [case["case_id"] for case in cases]
    duplicate_ids = sorted(k for k, count in Counter(ids).items() if count != 1)
    must(not duplicate_ids, f"duplicate executed IDs: {duplicate_ids}")
    must(set(ids) == set(manifest["required_case_ids"]), "required and executed sets differ")
    must(artifact["required_case_ids"] == manifest["required_case_ids"], "artifact required ID drift")
    must(type(artifact["executed_count"]) is int and artifact["executed_count"] == len(cases), "executed count drift")
    must(all(c["status"] == "pass" for c in cases), "failed or skipped case")
    must(all(a["ok"] is True for c in cases for a in c["assertions"]), "failed/nonboolean assertion")
    must(type(artifact["cleanup"]["compose_down_rc"]) is int and artifact["cleanup"]["compose_down_rc"] == 0, "cleanup failed")
    must(not artifact["cleanup"]["residual_containers"], "residual task containers")
    old_ids = set(old["required_case_ids"])
    must(old_ids.issubset(ids), "original ID missing")
    old_assertions = sum(len(c["assertions"]) for c in cases if c["case_id"] in old_ids)
    must(len(old_ids) == 25 and old_assertions == 286, "historical 25/286 denominator changed")
    assertions = sum(len(c["assertions"]) for c in cases)

    index = json.loads((directory / "gate-index.json").read_text())
    must(index["product_sha"] == product, "gate-index product identity")
    must(index["required_gate_names"] == REQUIRED_GATES, "required gate list changed")
    must([g["name"] for g in index["gates"]] == REQUIRED_GATES, "missing/duplicate gate")
    raw_codes = {}
    for gate in index["gates"]:
        name = gate["name"]
        must(gate["head_before"] == product and gate["head_after"] == product, f"gate HEAD drift: {name}")
        rc = gate["returncode"]
        must(type(rc) is int, f"noninteger gate returncode: {name}")
        allowed = {0, 1} if name in ("16-lint-scoped", "17-lint-fullrepo") else {0}
        must(rc in allowed, f"failed gate: {name}, rc={rc}")
        raw_codes[name] = rc
        for suffix in ("command.txt", "stdout.txt", "stderr.txt", "rc.txt", "head-before.txt", "head-after.txt"):
            must((directory / "gates" / f"{name}.{suffix}").is_file(), f"missing receipt {name}.{suffix}")
        must(int((directory / "gates" / f"{name}.rc.txt").read_text()) == rc, f"raw rc differs: {name}")
        must("[no tests to run]" not in (directory / "gates" / f"{name}.stdout.txt").read_text(), f"zero matching tests: {name}")
    lint = json.loads((directory / "gates/18-lint-identities.json").read_text())
    for scope in ("scoped", "full"):
        must(not lint[scope]["added"] and not lint[scope]["removed"], f"lint identity drift: {scope}")

    environment = json.loads((directory / "golden/runner-environment.json").read_text())
    must(environment["head_sha"] == product, "Golden environment product identity")
    must(environment["network_preserved"] is True, "preexisting network not preserved")
    must(environment["protected_network_id_before"] == environment["protected_network_id_after"], "network identity drift")
    original_blobs = historical_blobs(BASE)
    current_blobs = historical_blobs("HEAD")
    changed_history = [path for path, blob in original_blobs.items() if current_blobs.get(path) != blob]
    must(not changed_history, f"historical evidence Git blobs changed: {changed_history}")
    for path in directory.rglob("*.go"):
        raise ValueError(f"evidence source enters Go package discovery: {path}")
    result = {
        "product_sha": product, "verification_head": git("rev-parse", "HEAD"),
        "artifact": str(artifact_path), "artifact_sha256": hashlib.sha256(artifact_path.read_bytes()).hexdigest(),
        "cli_sha256": actual_hash, "executed_cases": len(cases),
        "passed_cases": len(cases), "assertions": assertions,
        "original_cases": len(old_ids), "original_assertions": old_assertions,
        "new_cases": len(cases) - len(old_ids), "new_assertions": assertions - old_assertions,
        "required_equals_executed": True, "duplicate_ids": duplicate_ids,
        "failed_assertions": [], "skipped_cases": [], "gate_returncodes": raw_codes,
        "historical_evidence_unchanged": True, "preexisting_network_preserved": True,
        "lint": {scope: {
            "baseline_raw": lint[scope]["baseline"]["raw_count"],
            "current_raw": lint[scope]["current"]["raw_count"],
            "baseline_normalized": lint[scope]["baseline"]["normalized_count"],
            "current_normalized": lint[scope]["current"]["normalized_count"],
            "added": lint[scope]["added"], "removed": lint[scope]["removed"],
        } for scope in ("scoped", "full")},
    }
    Path(args.output).write_text(json.dumps(result, indent=2) + "\n")
    print(json.dumps(result, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
