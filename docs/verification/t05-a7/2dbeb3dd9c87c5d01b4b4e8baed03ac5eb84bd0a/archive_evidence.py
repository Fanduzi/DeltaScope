#!/usr/bin/env python3
# input: frozen T05-A7 proof originals and an absent evidence destination
# output: byte-identical evidence copies and a source-to-archive SHA-256 map
# pos: task-local evidence packaging, never the T05 runner or validator
# note: if this file changes, update the evidence README.md and verify every copied hash.
import argparse
import hashlib
import json
from pathlib import Path
import shutil

parser = argparse.ArgumentParser()
parser.add_argument("--source", required=True, type=Path)
parser.add_argument("--destination", required=True, type=Path)
args = parser.parse_args()
source = args.source.resolve()
destination = args.destination.resolve()
if destination.exists():
    raise SystemExit("refusing to overwrite evidence destination")
folders = ("red", "iterations", "precommit", "precommit-fixed", "identity", "final-gates", "final-cli-1")
files = ("proof.py", "verify_artifacts.py", "archive_evidence.py", "lead-artifact-review.json", "lead-artifact-review.invocation.json")
paths = []
for folder in folders:
    if not (source / folder).is_dir():
        raise SystemExit("missing required source directory: " + folder)
    paths.extend(path for path in (source / folder).rglob("*") if path.is_file())
paths.extend(source / name for name in files)
rows = []
for original in sorted(paths):
    if original.is_symlink():
        raise SystemExit("unexpected source symlink: " + str(original))
    relative = original.relative_to(source)
    if relative == Path("final-cli-1/deltascope"):
        continue
    if "__pycache__" in relative.parts or relative.suffix.lower() in (".html", ".pyc"):
        raise SystemExit("unexpected artifact type: " + str(relative))
    archived = relative
    if relative.parts[0] == "red" and relative.suffix == ".go":
        archived = relative.with_suffix(".go.txt")
    target = destination / archived
    target.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(original, target)
    source_hash = hashlib.sha256(original.read_bytes()).hexdigest()
    archive_hash = hashlib.sha256(target.read_bytes()).hexdigest()
    if source_hash != archive_hash:
        raise SystemExit("copy hash mismatch: " + str(relative))
    rows.append({"original": str(original), "archived": archived.as_posix(), "sha256": source_hash, "bytes": target.stat().st_size})
(destination / "archive-map.json").write_text(json.dumps({"tested_code_sha": "2dbeb3dd9c87c5d01b4b4e8baed03ac5eb84bd0a", "copies": rows}, indent=2) + "\n")
print(json.dumps({"status": "copied-and-byte-verified", "copied_files": len(rows), "destination": str(destination)}))
