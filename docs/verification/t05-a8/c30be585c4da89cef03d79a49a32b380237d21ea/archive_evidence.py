#!/usr/bin/env python3
# input: frozen A8 plan, red, iteration and final-code originals plus an absent evidence destination
# output: byte-identical whitelisted copies and original-to-archive SHA-256/size bindings
# pos: A8 evidence packaging, not a product runner or validator
# note: if this file changes, update the evidence README.md and verify every copied hash.
import argparse
import hashlib
import json
from pathlib import Path
import shutil

parser = argparse.ArgumentParser()
parser.add_argument('--source', required=True, type=Path)
parser.add_argument('--plan', required=True, type=Path)
parser.add_argument('--destination', required=True, type=Path)
args = parser.parse_args()
source, plan, destination = args.source.resolve(), args.plan.resolve(), args.destination.resolve()
if destination.exists():
    raise SystemExit('refusing to overwrite existing evidence')
red = ('identity-before.txt','identity-after.txt','new-file-attribution.txt','tracked-diff.txt','summary.txt','t05a8.command.txt','t05a8.stdout.txt','t05a8.stderr.txt','t05a8.rc.txt','batch_state_a8_first_path_test.go.txt')
pairs = [(source / name, Path('red') / name) for name in red]
for folder in ('iterations','precommit','final-gates','final-cli-1'):
    if not (source / folder).is_dir():
        raise SystemExit('missing required source folder: ' + folder)
    for original in sorted((source / folder).rglob('*')):
        if original.is_symlink():
            raise SystemExit('unexpected symlink: ' + str(original))
        if original.is_file() and original != source / 'final-cli-1/deltascope':
            pairs.append((original, original.relative_to(source)))
if not plan.is_dir():
    raise SystemExit('missing frozen plan originals')
for original in sorted(plan.rglob('*')):
    if original.is_symlink():
        raise SystemExit('unexpected plan symlink: ' + str(original))
    if original.is_file() and original != plan / 'deltascope':
        relative = original.relative_to(plan)
        if relative == Path('baseline.py'):
            relative = Path('baseline.py.txt')
        pairs.append((original, Path('plan') / relative))
for name in ('proof.py','verify_artifacts.py','archive_evidence.py','lead-artifact-review.json','lead-artifact-review.invocation.json'):
    pairs.append((source / name, Path(name)))
rows = []
for original, relative in sorted(pairs, key=lambda pair: str(pair[1])):
    if not original.is_file() or original.is_symlink():
        raise SystemExit('missing or symbolic source: ' + str(original))
    if '__pycache__' in relative.parts or relative.suffix.lower() in ('.html','.pyc','.go'):
        raise SystemExit('forbidden artifact: ' + str(relative))
    target = destination / relative
    if target.exists():
        raise SystemExit('duplicate target: ' + str(relative))
    target.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(original, target)
    source_hash = hashlib.sha256(original.read_bytes()).hexdigest()
    if source_hash != hashlib.sha256(target.read_bytes()).hexdigest():
        raise SystemExit('copy mismatch: ' + str(relative))
    rows.append({'original':str(original),'archived':relative.as_posix(),'sha256':source_hash,'bytes':target.stat().st_size})
(destination / 'archive-map.json').write_text(json.dumps({'tested_code_sha':'c30be585c4da89cef03d79a49a32b380237d21ea','baseline_sha':'affad1b1ae72fd0a910286499e14b8ae2e98a5f5','copies':rows},indent=2)+'\n')
print(json.dumps({'status':'copied-and-byte-verified','copied_files':len(rows),'destination':str(destination)}))
