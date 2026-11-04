#!/usr/bin/env python3
"""Assemble only explicitly owned T20-R4 evidence, without altering raw receipts."""

import argparse
import hashlib
import json
from pathlib import Path
import shutil
import subprocess

ROOT = Path("/Users/fan/GolangProjects/DeltaScope")
STAGING = Path("/tmp/deltascope-t20r4-6ac1e918")
PRODUCT = "2f01f500d296bb29ee518bc113a58ec26172dc1a"
PHASES = (
    "baseline", "red", "native-environment", "native", "native-scheme",
    "native-scheme-v2", "green", "red-r1", "green-r1", "red-final",
    "green-final", "lead-controls-red", "lead-controls-green", "lint-repair", "lead-checks",
)
TOOLS = (
    "native_probe.py", "native_scheme_probe.py", "capture_check.py",
    "check_l2.py", "lint_identity.py", "lint-baseline.json",
    "formal_golden.py", "run_final_gates.py", "run_final_gates.initial.py",
    "verify_candidate.py", "verify_candidate.initial.py", "gate_reuse.py",
    "gate_reuse_final.py",
    "verify_resources.py", "verify_resources.initial.py", "capture_scopes.py", "assemble_evidence.py",
)


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--destination", required=True)
    args = parser.parse_args()
    destination = Path(args.destination).resolve()
    parent = ROOT / "docs/verification/t20-tidb-placement-lifecycle"
    if destination.parent != parent or destination.name != PRODUCT[:8]:
        raise ValueError("destination must be the new product's owned evidence directory")
    if not parent.is_dir() or destination.exists():
        raise ValueError("parent missing or destination already exists; never overwrite evidence")
    current = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
    if current != PRODUCT:
        raise ValueError("assemble against the frozen product before the evidence commit")
    verification = json.loads((STAGING / "lead-verification.json").read_text())
    if verification["product_sha"] != PRODUCT:
        raise ValueError("lead verification belongs to another product")
    final = STAGING / ("final-" + PRODUCT[:8])
    artifact = final / "golden/T20/artifact.json"
    if sha(artifact) != verification["artifact_sha256"]:
        raise ValueError("verified artifact changed; do not re-derive or silently update its figures")
    cli = Path(json.loads(artifact.read_text())["cli"]["path"]).resolve()
    entries = []
    omitted = []
    destination.mkdir()

    def copy_file(source, relative):
        source = Path(source)
        relative = Path(relative)
        if source.resolve() == cli:
            omitted.append({
                "source": str(source), "reason": "built CLI remains at its execution path",
                "verified_sha256": verification["cli_sha256"],
            })
            return
        if source.is_symlink():
            raise ValueError(f"unexpected symlink in owned evidence: {source}")
        if source.suffix == ".go":
            relative = relative.with_name(relative.name + ".txt")
        target = destination / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(source, target)
        digest = sha(source)
        if sha(target) != digest:
            raise ValueError(f"copied receipt differs: {source}")
        entries.append({"source": str(source), "path": relative.as_posix(), "sha256": digest})

    def copy_tree(source, prefix):
        source = Path(source)
        if not source.is_dir():
            raise ValueError(f"required owned evidence is absent: {source}")
        for item in sorted(source.rglob("*")):
            if item.is_file() or item.is_symlink():
                if "__pycache__" in item.parts:
                    continue
                copy_file(item, Path(prefix) / item.relative_to(source))

    for phase in PHASES:
        copy_tree(STAGING / phase, Path("development") / phase)
    copy_tree(STAGING / "final-5fc29853", "attempts/5fc29853-vet-failure")
    copy_tree(STAGING / "final-bb5e347c", "attempts/bb5e347c-lint-delta")
    copy_tree(final, ".")
    for name in TOOLS:
        copy_file(STAGING / name, Path("tools") / name)
    for name in ("lead-verification.json", "protected-resources.json",
                 "protected-resources-corrected.json", "scope-product.json"):
        copy_file(STAGING / name, name)
    source = ROOT / "internal/application/audit/batch_state_t20r4_test.go"
    copy_file(source, "source/batch_state_t20r4_test.go.txt")
    for name in ("rework-product-review.diff", "final-product-review.diff",
                 "final-review-delta.diff"):
        copy_file(STAGING / name, Path("development/review-snapshots") / name)
    if list(destination.rglob("*.go")):
        raise ValueError("archive polluted Go package discovery")
    ledger = {
        "product_sha": PRODUCT, "files": entries, "omitted_binaries": omitted,
        "counts_source": "lead-verification.json",
        "note": (
            "Raw receipt bytes and recorded execution HEADs are preserved. "
            "Only archived source filenames ending .go acquire .txt. "
            "README/REVIEW_HANDOFF prose is authored separately from this copy ledger."
        ),
    }
    (destination / "archive-index.json").write_text(json.dumps(ledger, indent=2) + "\n")
    print(json.dumps({
        "destination": str(destination), "copied_files": len(entries),
        "omitted_binaries": omitted, "product_sha": PRODUCT,
    }, indent=2))


if __name__ == "__main__":
    main()
