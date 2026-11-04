#!/usr/bin/env python3
"""One explicit full-test reuse for the proven unreachable-return deletion.

This is not a general skip/reuse policy. Both Git identities, the exact
one-file edit and the original successful command are pinned below.
"""

import hashlib
import json
from pathlib import Path
import shutil
import subprocess


ROOT = Path("/Users/fan/GolangProjects/DeltaScope")
SOURCE = "5fc298539006c21a8af7adbe2152ba591b8f2546"
TARGET = "bb5e347cd814c463a6873f6b0293613f91013867"
FILE = "internal/domain/rule/ddl/placement_rules.go"
COMMAND = ["go", "test", "-count=1", "./..."]
SUFFIXES = ("command.txt", "stdout.txt", "stderr.txt", "rc.txt",
            "head-before.txt", "head-after.txt")
OLD = "\tdefault:\n\t\treturn r.bindingGaps(statement)\n\t}\n\treturn nil\n}"
NEW = "\tdefault:\n\t\treturn r.bindingGaps(statement)\n\t}\n}"


def git(*args):
    return subprocess.check_output(["git", *args], cwd=ROOT)


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def verify_edit(target):
    if target != TARGET or git("rev-parse", TARGET + "^").decode().strip() != SOURCE:
        raise ValueError("full-test reuse is authorized only for the pinned direct child")
    if git("diff", "--name-only", SOURCE, TARGET).decode().splitlines() != [FILE]:
        raise ValueError("full-test reuse contains another changed file")
    before = git("show", f"{SOURCE}:{FILE}").decode()
    after = git("show", f"{TARGET}:{FILE}").decode()
    if before.count(OLD) != 1 or after != before.replace(OLD, NEW, 1):
        raise ValueError("target is not exactly the proven unreachable-return deletion")
    patch = git("diff", SOURCE, TARGET, "--", FILE)
    return {
        "changed_file": FILE,
        "source_blob": git("rev-parse", f"{SOURCE}:{FILE}").decode().strip(),
        "target_blob": git("rev-parse", f"{TARGET}:{FILE}").decode().strip(),
        "diff_sha256": hashlib.sha256(patch).hexdigest(),
    }


def adopt_full_test(source_directory, destination, target):
    edit = verify_edit(target)
    source_directory = Path(source_directory).resolve()
    destination = Path(destination).resolve()
    index = json.loads((source_directory / "gate-index.json").read_text())
    if index["product_sha"] != SOURCE or len(index["gates"]) != 2:
        raise ValueError("expected the preserved attempt that stopped at vet")
    test, vet = index["gates"]
    if (test["name"] != "01-test" or test["command"] != COMMAND or test["returncode"] != 0
            or test["head_before"] != SOURCE or test["head_after"] != SOURCE
            or vet["name"] != "02-vet" or vet["returncode"] != 1
            or vet["head_before"] != SOURCE or vet["head_after"] != SOURCE):
        raise ValueError("source execution identity or result is not the authorized evidence")
    stderr = source_directory / "gates/02-vet.stderr.txt"
    if f"{FILE}:510:2: unreachable code" not in stderr.read_text():
        raise ValueError("missing the specific vet diagnosis supporting equivalence")
    hashes = {}
    for suffix in SUFFIXES:
        source = source_directory / "gates" / f"01-test.{suffix}"
        target_file = destination / "gates" / f"01-test.{suffix}"
        if target_file.exists():
            raise ValueError("never overwrite an execution receipt")
        hashes[suffix] = digest(source)
        shutil.copy2(source, target_file)
    proof = {
        "source_product_sha": SOURCE, "target_product_sha": TARGET,
        "gate": "01-test", "command": COMMAND,
        "source_attempt_directory": str(source_directory),
        "source_execution": test, "receipt_sha256": hashes,
        "vet_diagnosis_sha256": digest(stderr),
        "equivalence": edit,
        "reason": (
            "All switch branches return; go vet identified the trailing return as unreachable. "
            "The target deletes that single statement and changes no other tracked file. "
            "The full-test result is reused with its actual source HEAD; vet and later gates "
            "execute at the target HEAD. This does not claim a full-test rerun at the target."
        ),
    }
    (destination / "full-test-reuse.json").write_text(json.dumps(proof, indent=2) + "\n")
    record = dict(test)
    record["reused_from_product"] = SOURCE
    record["equivalence_proof"] = "full-test-reuse.json"
    return record


def validate_full_test(directory, gate, target):
    directory = Path(directory)
    proof = json.loads((directory / "full-test-reuse.json").read_text())
    if (proof["source_product_sha"] != SOURCE or proof["target_product_sha"] != TARGET
            or proof["gate"] != "01-test" or proof["command"] != COMMAND
            or gate["name"] != "01-test" or gate["command"] != COMMAND
            or gate["head_before"] != SOURCE or gate["head_after"] != SOURCE
            or gate["returncode"] != 0 or gate.get("reused_from_product") != SOURCE
            or gate.get("equivalence_proof") != "full-test-reuse.json"):
        raise ValueError("reused gate identity differs from its pinned proof")
    if proof["equivalence"] != verify_edit(target):
        raise ValueError("equivalence proof differs from the actual frozen Git objects")
    raw_gate = {k: v for k, v in gate.items()
                if k not in ("reused_from_product", "equivalence_proof")}
    if raw_gate != proof["source_execution"]:
        raise ValueError("source test execution metadata was altered")
    for suffix in SUFFIXES:
        if digest(directory / "gates" / f"01-test.{suffix}") != proof["receipt_sha256"][suffix]:
            raise ValueError(f"reused raw receipt was altered: {suffix}")
    return {
        "gate": "01-test", "actual_execution_sha": SOURCE,
        "equivalent_target_sha": TARGET,
        "reason": proof["reason"], "equivalence": proof["equivalence"],
    }
