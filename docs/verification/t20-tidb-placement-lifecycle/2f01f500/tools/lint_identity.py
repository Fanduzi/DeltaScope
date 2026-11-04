#!/usr/bin/env python3
"""Preserve the T20 diagnostic-line/file:linter:message identity algorithm."""

import argparse
import hashlib
import json
from pathlib import Path
import re


ROOT = Path("/Users/fan/GolangProjects/DeltaScope")
BASE = ROOT / "docs/verification/t20-tidb-placement-lifecycle/57476dc2/gates"
ALGORITHM = "diagnostic-line filter; file:linter:message; line:column removed; set identity"
DIAGNOSTIC = re.compile(r"^(.+\.go):\d+:\d+: (.+) \(([^()]+)\)$")
HEADER = re.compile(r"^.+\.go:\d+:\d+: ")


def extract(path):
    path = Path(path)
    data = path.read_bytes()
    raw = []
    for line in data.decode().splitlines():
        match = DIAGNOSTIC.match(line)
        if match:
            file, message, linter = match.groups()
            raw.append(f"{file}:{linter}:{message}")
        elif HEADER.match(line):
            raise ValueError(f"unrecognized diagnostic header in {path}: {line!r}")
    if not raw:
        raise ValueError(f"no diagnostic headers in {path}; do not count this as clean")
    return {
        "source": str(path),
        "sha256": hashlib.sha256(data).hexdigest(),
        "raw_count": len(raw),
        "normalized_count": len(set(raw)),
        "identities": sorted(set(raw)),
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=("freeze-baseline", "compare"))
    parser.add_argument("--baseline", required=True)
    parser.add_argument("--scoped")
    parser.add_argument("--full")
    parser.add_argument("--output")
    args = parser.parse_args()
    if args.mode == "freeze-baseline":
        result = {
            "algorithm": ALGORITHM,
            "product_sha": "57476dc2ea0078e7b07efc988c3505d93702aba3",
            "evidence_sha": "6ac1e918b3558d6d611d7d8a59a770d73341c2f9",
            "scoped": extract(BASE / "11-lint-scoped.stdout.txt"),
            "full": extract(BASE / "12-lint-fullrepo.stdout.txt"),
        }
        for name, count in (("scoped", 18), ("full", 33)):
            if result[name]["normalized_count"] != count:
                raise ValueError(f"{name} does not reproduce the archived identity count")
        Path(args.baseline).write_text(json.dumps(result, indent=2) + "\n")
        print(json.dumps({k: {n: result[k][n] for n in ("raw_count", "normalized_count", "sha256")} for k in ("scoped", "full")}, indent=2))
        return 0
    baseline = json.loads(Path(args.baseline).read_text())
    if baseline["algorithm"] != ALGORITHM:
        raise ValueError("baseline algorithm identity mismatch")
    result = {"algorithm": ALGORITHM, "baseline_product_sha": baseline["product_sha"]}
    changed = False
    for name, path in (("scoped", args.scoped), ("full", args.full)):
        if not path:
            raise ValueError(f"{name} output required")
        current = extract(path)
        old = baseline[name]
        added = sorted(set(current["identities"]) - set(old["identities"]))
        removed = sorted(set(old["identities"]) - set(current["identities"]))
        result[name] = {"baseline": old, "current": current, "added": added, "removed": removed}
        changed = changed or bool(added or removed)
        print(f"{name}: raw {old['raw_count']} -> {current['raw_count']}; "
              f"normalized {old['normalized_count']} -> {current['normalized_count']}; "
              f"added={len(added)} removed={len(removed)}")
    if not args.output:
        raise ValueError("comparison output file required")
    Path(args.output).write_text(json.dumps(result, indent=2) + "\n")
    return int(changed)


if __name__ == "__main__":
    raise SystemExit(main())
