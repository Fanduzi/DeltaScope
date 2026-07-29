#!/usr/bin/env python3
"""Tests for the release workflow provenance contract checker.

Negative tests verify that the checker catches:
- Missing provenance job
- release-linux not depending on provenance
- Tag checkout without fetch origin/main
- Provenance job with write permissions
- Post-tag check after GoReleaser
- Independent publisher path bypassing provenance

Usage: python3 scripts/test_verify_release_workflow_provenance_negative.py
"""

import sys
import tempfile
from pathlib import Path

# Import the checker
sys.path.insert(0, str(Path(__file__).parent))
from test_verify_release_workflow_provenance import check_provenance_contract


PASS = 0
FAIL = 0


def expect_violation(name: str, yaml_content: str, needle: str) -> None:
    """Write a temp workflow, run the checker, assert violation containing needle."""
    global PASS, FAIL
    with tempfile.NamedTemporaryFile(mode="w", suffix=".yml", delete=False) as f:
        f.write(yaml_content)
        f.flush()
        violations = check_provenance_contract(Path(f.name))

    found = any(needle in v for v in violations)
    if found:
        print(f"  PASS: {name}")
        PASS += 1
    else:
        print(f"  FAIL: {name} — expected violation containing '{needle}', got: {violations}")
        FAIL += 1


def expect_clean(name: str, yaml_content: str) -> None:
    """Write a temp workflow, run the checker, assert no violations."""
    global PASS, FAIL
    with tempfile.NamedTemporaryFile(mode="w", suffix=".yml", delete=False) as f:
        f.write(yaml_content)
        f.flush()
        violations = check_provenance_contract(Path(f.name))

    if not violations:
        print(f"  PASS: {name}")
        PASS += 1
    else:
        print(f"  FAIL: {name} — expected no violations, got: {violations}")
        FAIL += 1


VALID_WORKFLOW = """\
name: release
on:
  push:
    tags: ["v*"]
permissions:
  contents: write
jobs:
  provenance:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    permissions:
      contents: read
    steps:
      - name: Checkout
        uses: actions/checkout@v6
        with:
          fetch-depth: 0
      - name: Fetch origin main
        run: git fetch origin main
      - name: Verify post-tag candidate provenance
        env:
          RELEASE_MAIN_REF: refs/remotes/origin/main
        run: make posttag-candidate-gate VERSION="${GITHUB_REF_NAME}"
  release-linux:
    needs: provenance
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - name: GoReleaser
        run: goreleaser release --clean
      - name: Upload
        run: gh release upload "${GITHUB_REF_NAME}" dist/foo --clobber
  publish-homebrew-cask:
    needs:
      - release-linux
    runs-on: ubuntu-latest
    steps:
      - run: git push origin HEAD:main
  publish-mcp-launcher-package:
    needs:
      - publish-homebrew-cask
    runs-on: ubuntu-latest
    steps:
      - run: npm publish --access public --provenance ./packages/deltascope-mcp
"""

print("=== test_verify_release_workflow_provenance (negative + positive) ===")

# --- Positive: valid workflow passes ---
print("N0: valid workflow")
expect_clean("valid workflow passes", VALID_WORKFLOW)

# --- Negative: missing provenance job ---
print("N1: missing provenance job")
MISSING_PROV = """\
name: release
on:
  push:
    tags: ["v*"]
permissions:
  contents: write
jobs:
  release-linux:
    runs-on: ubuntu-latest
    steps:
      - run: goreleaser release --clean
"""
expect_violation("catches missing provenance job", MISSING_PROV, "missing 'provenance' job")

# --- Negative: release-linux does not depend on provenance ---
print("N2: release-linux not depending on provenance")
NO_DEP = """\
name: release
on:
  push:
    tags: ["v*"]
permissions:
  contents: write
jobs:
  provenance:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0
      - run: git fetch origin main
      - env:
          RELEASE_MAIN_REF: refs/remotes/origin/main
        run: make posttag-candidate-gate VERSION="${GITHUB_REF_NAME}"
  release-linux:
    runs-on: ubuntu-latest
    steps:
      - run: goreleaser release --clean
"""
expect_violation("catches missing dependency", NO_DEP, "release-linux needs=")

# --- Negative: provenance job has write permissions ---
print("N3: provenance job with write permissions")
WRITE_PERM = """\
name: release
on:
  push:
    tags: ["v*"]
permissions:
  contents: write
jobs:
  provenance:
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0
      - run: git fetch origin main
      - env:
          RELEASE_MAIN_REF: refs/remotes/origin/main
        run: make posttag-candidate-gate VERSION="${GITHUB_REF_NAME}"
  release-linux:
    needs: provenance
    runs-on: ubuntu-latest
    steps:
      - run: goreleaser release --clean
"""
expect_violation("catches write permissions on provenance", WRITE_PERM, "permissions.contents is 'write'")

# --- Negative: provenance doesn't fetch origin/main ---
print("N4: tag checkout without fetch origin/main")
NO_FETCH = """\
name: release
on:
  push:
    tags: ["v*"]
permissions:
  contents: write
jobs:
  provenance:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0
      - env:
          RELEASE_MAIN_REF: refs/remotes/origin/main
        run: make posttag-candidate-gate VERSION="${GITHUB_REF_NAME}"
  release-linux:
    needs: provenance
    runs-on: ubuntu-latest
    steps:
      - run: goreleaser release --clean
"""
expect_violation("catches missing fetch origin/main", NO_FETCH, "does not fetch origin/main")

# --- Negative: provenance doesn't set RELEASE_MAIN_REF ---
print("N5: post-tag check without RELEASE_MAIN_REF")
NO_REF = """\
name: release
on:
  push:
    tags: ["v*"]
permissions:
  contents: write
jobs:
  provenance:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0
      - run: git fetch origin main
      - run: make posttag-candidate-gate VERSION="${GITHUB_REF_NAME}"
  release-linux:
    needs: provenance
    runs-on: ubuntu-latest
    steps:
      - run: goreleaser release --clean
"""
expect_violation("catches missing RELEASE_MAIN_REF", NO_REF, "RELEASE_MAIN_REF=refs/remotes/origin/main")

# --- Negative: independent publisher path bypassing provenance ---
print("N6: publisher path bypassing provenance")
BYPASS = """\
name: release
on:
  push:
    tags: ["v*"]
permissions:
  contents: write
jobs:
  provenance:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0
      - run: git fetch origin main
      - env:
          RELEASE_MAIN_REF: refs/remotes/origin/main
        run: make posttag-candidate-gate VERSION="${GITHUB_REF_NAME}"
  release-linux:
    needs: provenance
    runs-on: ubuntu-latest
    steps:
      - run: goreleaser release --clean
  rogue-publisher:
    runs-on: ubuntu-latest
    steps:
      - run: npm publish --access public
"""
expect_violation("catches rogue publisher", BYPASS, "rogue-publisher")

print("")
print(f"Results: {PASS} passed, {FAIL} failed")
sys.exit(0 if FAIL == 0 else 1)
