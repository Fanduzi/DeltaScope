#!/usr/bin/env python3
# input: check_provenance_contract and temporary workflow fixtures
# output: pass/fail evidence that provenance DAG and npm/Homebrew channel checks reject tampered workflows
# pos: adversarial tests for the tag-triggered release workflow provenance checker
# note: if this file changes, update this header and scripts/README.md.
"""Adversarial and negative tests for the provenance contract checker.

Table-driven fixtures covering:
- Valid workflow (positive control)
- Missing provenance job
- release-linux not depending on provenance
- Provenance job with write permissions
- Tag checkout without fetch origin/main
- Post-tag check without RELEASE_MAIN_REF
- Independent publisher path bypassing provenance
- RELEASE_MAIN_REF on wrong step (env/run split across steps)
- GoReleaser action-only publisher (uses: goreleaser/... with release args)
- Checkout without fetch-depth: 0
- npm publication depending on Homebrew verification
- npm publication missing a required platform-build direct need

Usage: python3 scripts/test_verify_release_workflow_provenance_negative.py
"""

import sys
import tempfile
from pathlib import Path

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


# --- Base valid workflow template ---

VALID_PROVENANCE = """\
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
        run: |
          set -euo pipefail
          make posttag-candidate-gate VERSION="${GITHUB_REF_NAME}"
"""

VALID_RELEASE_LINUX = """\
  release-linux:
    needs: provenance
    runs-on: ubuntu-latest
    steps:
      - run: goreleaser release --clean
"""

VALID_TAIL = """\
  release-linux-arm64:
    needs: release-linux
    runs-on: ubuntu-latest
    steps:
      - run: echo linux-arm64
  release-macos-arm64:
    needs: release-linux
    runs-on: ubuntu-latest
    steps:
      - run: echo macos-arm64
  release-macos-amd64:
    needs: release-linux
    runs-on: ubuntu-latest
    steps:
      - run: echo macos-amd64
  publish-homebrew-cask:
    needs:
      - release-linux
    runs-on: ubuntu-latest
    steps:
      - run: git push origin HEAD:main
  publish-mcp-launcher-package:
    needs:
      - release-linux
      - release-linux-arm64
      - release-macos-arm64
      - release-macos-amd64
    runs-on: ubuntu-latest
    steps:
      - run: npm publish --access public --provenance ./packages/deltascope-mcp
"""

HEADER = """\
name: release
on:
  push:
    tags: ["v*"]
permissions:
  contents: write
jobs:
"""


def make_workflow(provenance: str = VALID_PROVENANCE,
                  release_linux: str = VALID_RELEASE_LINUX,
                  tail: str = VALID_TAIL) -> str:
    return HEADER + provenance + release_linux + tail


# --- Table-driven test cases ---

print("=== test_verify_release_workflow_provenance (adversarial) ===")

# N0: positive control
print("N0: valid workflow")
expect_clean("valid workflow passes", make_workflow())

# N1: missing provenance job
print("N1: missing provenance job")
expect_violation("catches missing provenance job",
    HEADER + VALID_RELEASE_LINUX + VALID_TAIL,
    "missing 'provenance' job")

# N2: release-linux not depending on provenance
print("N2: release-linux not depending on provenance")
no_dep_rl = """\
  release-linux:
    runs-on: ubuntu-latest
    steps:
      - run: goreleaser release --clean
"""
expect_violation("catches missing dependency",
    make_workflow(release_linux=no_dep_rl),
    "release-linux needs=")

# N3: provenance job with write permissions
print("N3: provenance job with write permissions")
write_perm_prov = VALID_PROVENANCE.replace("contents: read", "contents: write")
expect_violation("catches write permissions on provenance",
    make_workflow(provenance=write_perm_prov),
    "permissions.contents is 'write'")

# N4: tag checkout without fetch origin/main
print("N4: tag checkout without fetch origin/main")
no_fetch_prov = """\
  provenance:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - name: Checkout
        uses: actions/checkout@v6
        with:
          fetch-depth: 0
      - name: Verify post-tag candidate provenance
        env:
          RELEASE_MAIN_REF: refs/remotes/origin/main
        run: |
          set -euo pipefail
          make posttag-candidate-gate VERSION="${GITHUB_REF_NAME}"
"""
expect_violation("catches missing fetch origin/main",
    make_workflow(provenance=no_fetch_prov),
    "does not fetch origin/main")

# N5: post-tag check without RELEASE_MAIN_REF
print("N5: post-tag check without RELEASE_MAIN_REF")
no_ref_prov = """\
  provenance:
    runs-on: ubuntu-latest
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
        run: |
          set -euo pipefail
          make posttag-candidate-gate VERSION="${GITHUB_REF_NAME}"
"""
expect_violation("catches missing RELEASE_MAIN_REF",
    make_workflow(provenance=no_ref_prov),
    "RELEASE_MAIN_REF=refs/remotes/origin/main")

# N6: independent publisher path bypassing provenance
print("N6: publisher path bypassing provenance")
rogue_tail = VALID_TAIL + """\
  rogue-publisher:
    runs-on: ubuntu-latest
    steps:
      - run: npm publish --access public
"""
expect_violation("catches rogue publisher",
    make_workflow(tail=rogue_tail),
    "rogue-publisher")

# N7: RELEASE_MAIN_REF on wrong step (env/run split)
print("N7: env on wrong step — adversarial split")
split_prov = """\
  provenance:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - name: Checkout
        uses: actions/checkout@v6
        with:
          fetch-depth: 0
      - name: Fetch origin main
        run: git fetch origin main
      - name: Set env (harmless)
        env:
          RELEASE_MAIN_REF: refs/remotes/origin/main
        run: echo "setting env"
      - name: Verify post-tag candidate provenance
        run: |
          set -euo pipefail
          make posttag-candidate-gate VERSION="${GITHUB_REF_NAME}"
"""
expect_violation("catches env/run split across steps",
    make_workflow(provenance=split_prov),
    "RELEASE_MAIN_REF=refs/remotes/origin/main")

# N8: GoReleaser action detection
print("N8: GoReleaser action-only publisher")
# GoReleaser action under provenance is valid
goreleaser_action_rl = """\
  release-linux:
    needs: provenance
    runs-on: ubuntu-latest
    steps:
      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
"""
expect_clean("GoReleaser action under provenance passes",
    make_workflow(release_linux=goreleaser_action_rl))

# Rogue GoReleaser action without provenance dependency
rogue_goreleaser = VALID_TAIL + """\
  rogue-goreleaser:
    runs-on: ubuntu-latest
    steps:
      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          args: release --clean
"""
expect_violation("catches rogue GoReleaser action",
    make_workflow(tail=rogue_goreleaser),
    "rogue-goreleaser")

# N9: Checkout without fetch-depth: 0
print("N9: checkout without fetch-depth: 0")
no_depth_prov = """\
  provenance:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - name: Checkout
        uses: actions/checkout@v6
      - name: Fetch origin main
        run: git fetch origin main
      - name: Verify post-tag candidate provenance
        env:
          RELEASE_MAIN_REF: refs/remotes/origin/main
        run: |
          set -euo pipefail
          make posttag-candidate-gate VERSION="${GITHUB_REF_NAME}"
"""
expect_violation("catches missing fetch-depth: 0",
    make_workflow(provenance=no_depth_prov),
    "fetch-depth: 0")

# N10: npm depends on Homebrew verification
print("N10: npm depends on Homebrew verification")
homebrew_coupled_tail = VALID_TAIL.replace(
    "      - release-macos-amd64\n    runs-on: ubuntu-latest\n    steps:\n"
    "      - run: npm publish --access public --provenance ./packages/deltascope-mcp\n",
    "      - release-macos-amd64\n"
    "      - verify-homebrew-cask-install\n    runs-on: ubuntu-latest\n    steps:\n"
    "      - run: npm publish --access public --provenance ./packages/deltascope-mcp\n",
)
expect_violation(
    "catches npm waiting on Homebrew verification",
    make_workflow(tail=homebrew_coupled_tail),
    "verify-homebrew-cask-install",
)

# N11: npm missing a required platform-build direct need
print("N11: npm missing a required platform-build direct need")
missing_platform_tail = VALID_TAIL.replace("      - release-linux-arm64\n", "")
expect_violation(
    "catches missing platform-build need",
    make_workflow(tail=missing_platform_tail),
    "release-linux-arm64",
)

print("")
print(f"Results: {PASS} passed, {FAIL} failed")
sys.exit(0 if FAIL == 0 else 1)
