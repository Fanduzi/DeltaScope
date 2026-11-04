#!/usr/bin/env python3
"""Require real committed Go-package changes and corresponding module READMEs."""

import argparse
import json
from pathlib import PurePosixPath
import subprocess


ROOT = "/Users/fan/GolangProjects/DeltaScope"


def git(*args):
    return subprocess.check_output(["git", "-C", ROOT, *args], text=True).strip()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", default="6ac1e918b3558d6d611d7d8a59a770d73341c2f9")
    parser.add_argument("--head", required=True)
    args = parser.parse_args()
    base = git("rev-parse", "--verify", args.base + "^{commit}")
    head = git("rev-parse", "--verify", args.head + "^{commit}")
    if base == head:
        raise ValueError("empty committed comparison cannot satisfy L2")
    changed = set(git("diff", "--name-only", base, head).splitlines())
    go_files = sorted(p for p in changed if p.endswith(".go") and p.startswith(("internal/", "pkg/")))
    if not go_files:
        raise ValueError("no changed Go files; reject an empty L2 scope")
    required = sorted({str(PurePosixPath(p).parent / "README.md") for p in go_files})
    missing = sorted(set(required) - changed)
    print(json.dumps({
        "base_sha": base, "head_sha": head, "changed_go_files": go_files,
        "required_module_readmes": required, "missing_readme_updates": missing,
        "l3_headers_required": False,
    }, indent=2))
    return int(bool(missing))


if __name__ == "__main__":
    raise SystemExit(main())
