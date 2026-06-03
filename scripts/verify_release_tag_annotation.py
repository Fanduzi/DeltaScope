#!/usr/bin/env python3
"""Release tag annotation verifier.

Checks that a given VERSION tag exists locally and is an annotated tag
(git object type 'tag'), not a lightweight tag (git object type 'commit').

Read-only — never creates, deletes, or moves tags.

Usage:
    VERSION=vX.Y.Z python3 scripts/verify_release_tag_annotation.py
"""

import os
import re
import subprocess
import sys


def validate_version(version: str) -> str:
    if not re.match(r"^v\d+\.\d+\.\d+$", version):
        print(
            "release-tag-annotation: FAIL — VERSION must look like vX.Y.Z, "
            f"got {version!r}"
        )
        sys.exit(1)
    return version


def _git(*args: str) -> str:
    result = subprocess.run(
        ["git", *args],
        capture_output=True,
        text=True,
    )
    return result.stdout.strip(), result.returncode


def check_tag_annotation(version: str) -> None:
    print(f"release-tag-annotation: VERSION={version}")

    stdout, rc = _git("tag", "-l", version)
    if rc != 0 or not stdout:
        print(f"release-tag-annotation: FAIL — tag {version} does not exist")
        sys.exit(1)

    stdout, rc = _git("cat-file", "-t", version)
    if rc != 0:
        print(f"release-tag-annotation: FAIL — cannot determine type of {version}")
        sys.exit(1)

    tag_type = stdout
    print(f"release-tag-annotation: type={tag_type}")

    if tag_type == "tag":
        peeled_stdout, _ = _git("rev-parse", f"{version}^{{commit}}")
        print(f"release-tag-annotation: peeled={peeled_stdout}")
        print("release-tag-annotation: PASS")
    elif tag_type == "commit":
        print(
            f"release-tag-annotation: FAIL — {version} is lightweight "
            "(git object type commit); release tags must be annotated"
        )
        sys.exit(1)
    else:
        print(
            f"release-tag-annotation: FAIL — unexpected object type "
            f"'{tag_type}' for {version}"
        )
        sys.exit(1)


def main() -> None:
    version = os.environ.get("VERSION", "").strip()
    if not version:
        for i, arg in enumerate(sys.argv[1:], 1):
            if arg.startswith("--version="):
                version = arg.split("=", 1)[1]
            elif arg == "--version" and i < len(sys.argv) - 1:
                version = sys.argv[i + 1]

    validate_version(version)
    check_tag_annotation(version)


if __name__ == "__main__":
    main()
