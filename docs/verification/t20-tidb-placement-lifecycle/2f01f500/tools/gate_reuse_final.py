#!/usr/bin/env python3
"""Pin reuse of completed gates across the two lint-only source edits."""

import hashlib
import json
from pathlib import Path
import shutil
import subprocess

from gate_reuse import SUFFIXES, validate_full_test

ROOT = Path("/Users/fan/GolangProjects/DeltaScope")
SOURCE = "bb5e347cd814c463a6873f6b0293613f91013867"
TARGET = "2f01f500d296bb29ee518bc113a58ec26172dc1a"
ELIGIBLE = {
    "01-test", "03-race", "04-pg", "05-sql-corpus", "06-query-access-corpus",
    "07-inventory", "08-coverage-catalog", "09-parser-unsupported",
    "10-diagnostic-evidence", "11-privacy", "12-docs-examples",
    "15-golden-validator",
}
HELPER_FILE = "internal/application/audit/batch_state_placement.go"
SWITCH_FILE = "internal/application/audit/partition_transitions.go"
DEAD_HELPER = """// placementRemoveRef drops one proven inbound reference. Dropping a
// bound object also drops its reference — the set stays proven when the
// drop is proven.
func (s *batchState) placementRemoveRef(policy, ref string) {
\tentry := s.placementPolicies()[policy]
\tif entry == nil {
\t\treturn
\t}
\tdelete(entry.refs, ref)
}

"""
OLD_DISPATCH = """\tif alter.Action == "partition" {
\t\t// The declared scheme replaces every member binding for this
\t\t// table — retire all old partition refs and restart the map.
\t\ts.placementRefsUnder("partition:" + strings.ToLower(displaySchema) + "." + table + ".")
\t\tnext.PartitionPlacements = map[string]string{}
\t} else if alter.Action == "reorganize_partition" {
\t\tfor _, member := range alter.Partitions {
\t\t\tname := strings.ToLower(member)
\t\t\tif next.PartitionPlacements != nil {
\t\t\t\tdelete(next.PartitionPlacements, name)
\t\t\t}
\t\t\ts.placementRemoveTargetRef(placementPartitionRef(displaySchema, table, name))
\t\t}
\t}
"""
NEW_DISPATCH = OLD_DISPATCH.replace(
    '\tif alter.Action == "partition" {',
    '\tswitch alter.Action {\n\tcase "partition":',
).replace(
    '\t} else if alter.Action == "reorganize_partition" {',
    '\tcase "reorganize_partition":',
)


def git(*args):
    return subprocess.check_output(["git", *args], cwd=ROOT)


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def verify_edit(target):
    if target != TARGET or git("rev-parse", TARGET + "^").decode().strip() != SOURCE:
        raise ValueError("lint-only reuse is restricted to the pinned direct child")
    changed = set(git("diff", "--name-only", SOURCE, TARGET).decode().splitlines())
    if changed != {HELPER_FILE, SWITCH_FILE}:
        raise ValueError("lint-only reuse contains another changed file")
    for path, old, new in (
        (HELPER_FILE, DEAD_HELPER, ""),
        (SWITCH_FILE, OLD_DISPATCH, NEW_DISPATCH),
    ):
        before = git("show", f"{SOURCE}:{path}").decode()
        after = git("show", f"{TARGET}:{path}").decode()
        if before.count(old) != 1 or after != before.replace(old, new, 1):
            raise ValueError(f"unexpected source edit in {path}")
    return {
        "changed_files": sorted(changed),
        "diff_sha256": hashlib.sha256(git("diff", SOURCE, TARGET)).hexdigest(),
        "basis": (
            "Delete a private function diagnosed as unused, with no callers, and replace "
            "two mutually exclusive comparisons of immutable alter.Action with the "
            "corresponding tagged switch. Branch bodies and all other tracked bytes "
            "are unchanged. Fresh focused T20-R4 regression passes; vet, committed "
            "ADR/L2 scope checks, lint and Golden run at the target."
        ),
    }


def adopt_completed(source_directory, destination, target):
    edit = verify_edit(target)
    source_directory, destination = Path(source_directory), Path(destination)
    index = json.loads((source_directory / "gate-index.json").read_text())
    if (index["product_sha"] != SOURCE or len(index["gates"]) != 18
            or index["gates"][-1]["name"] != "18-lint-identity"
            or index["gates"][-1]["returncode"] != 1):
        raise ValueError("expected preserved attempt ending at the known lint delta")
    lint = json.loads((source_directory / "gates/18-lint-identities.json").read_text())
    expected = {
        f"{HELPER_FILE}:unused:func (*batchState).placementRemoveRef is unused",
        f"{SWITCH_FILE}:staticcheck:QF1003: could use tagged switch on alter.Action",
    }
    for scope in ("scoped", "full"):
        if set(lint[scope]["added"]) != expected or lint[scope]["removed"]:
            raise ValueError("lint source evidence differs from the two reviewed diagnostics")
    shutil.copy2(source_directory / "full-test-reuse.json", destination / "full-test-reuse.json")
    records, source_records, hashes = {}, {}, {}
    for gate in index["gates"]:
        name = gate["name"]
        if name not in ELIGIBLE:
            continue
        if name == "01-test":
            validate_full_test(source_directory, gate, SOURCE)
        elif gate["head_before"] != SOURCE or gate["head_after"] != SOURCE or gate["returncode"] != 0:
            raise ValueError(f"cannot reuse failed or misidentified gate {name}")
        source_records[name] = gate
        hashes[name] = {}
        for suffix in SUFFIXES:
            source = source_directory / "gates" / f"{name}.{suffix}"
            target_file = destination / "gates" / f"{name}.{suffix}"
            if target_file.exists():
                raise ValueError("never overwrite raw receipts")
            hashes[name][suffix] = digest(source)
            shutil.copy2(source, target_file)
        record = dict(gate)
        if name != "01-test":
            record["reused_from_product"] = SOURCE
            record["equivalence_proof"] = "gate-reuse-final.json"
        record["post_lint_reuse_proof"] = "gate-reuse-final.json"
        records[name] = record
    if set(records) != ELIGIBLE:
        raise ValueError("a required source gate is missing")
    proof = {
        "source_product_sha": SOURCE, "target_product_sha": TARGET,
        "source_attempt_directory": str(source_directory),
        "equivalence": edit, "source_executions": source_records,
        "receipt_sha256": hashes,
    }
    (destination / "gate-reuse-final.json").write_text(json.dumps(proof, indent=2) + "\n")
    return records


def validate_completed(directory, gate, target):
    directory = Path(directory)
    name = gate["name"]
    proof = json.loads((directory / "gate-reuse-final.json").read_text())
    if (name not in ELIGIBLE or proof["source_product_sha"] != SOURCE
            or proof["target_product_sha"] != TARGET
            or gate.get("post_lint_reuse_proof") != "gate-reuse-final.json"
            or proof["equivalence"] != verify_edit(target)):
        raise ValueError("reuse identity or exact lint edit proof does not match")
    source = proof["source_executions"][name]
    expected_record = dict(source)
    if name == "01-test":
        validate_full_test(directory, source, SOURCE)
    else:
        if source["head_before"] != SOURCE or source["head_after"] != SOURCE or source["returncode"] != 0:
            raise ValueError("source gate was not a successful execution at the pinned source")
        expected_record["reused_from_product"] = SOURCE
        expected_record["equivalence_proof"] = "gate-reuse-final.json"
    expected_record["post_lint_reuse_proof"] = "gate-reuse-final.json"
    if gate != expected_record:
        raise ValueError("source execution metadata was altered")
    for suffix in SUFFIXES:
        if digest(directory / "gates" / f"{name}.{suffix}") != proof["receipt_sha256"][name][suffix]:
            raise ValueError(f"raw reused receipt changed: {name}/{suffix}")
    return {
        "gate": name, "actual_execution_sha": source["head_before"],
        "equivalent_target_sha": TARGET, "equivalence": proof["equivalence"],
        "inherited_full_test_proof": name == "01-test",
    }
