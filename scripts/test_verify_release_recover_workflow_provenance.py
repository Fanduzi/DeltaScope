#!/usr/bin/env python3
"""Adversarial and negative tests for the recovery provenance contract checker.

Table-driven static fixtures covering:
- Valid recovery workflow (positive control)
- Missing preflight job
- Missing / wrong-value / late dispatch-ref guard
- Preflight default-branch checkout, shallow checkout, missing origin/main fetch
- Post-tag gate missing, env/run split across steps, gate after external work
- Missing tag_target_sha resolution / output export, resolution before the gate
- Publisher default checkout, input-tag checkout, main checkout, foreign SHA
- Mutation job without checkout, independent mutation path, GoReleaser action path
- Wide / missing / write-capable preflight permissions, publisher secrets in preflight
- Historical-version bypass literal

Behavior tests execute the REAL workflow's dispatch-ref guard step with
branch-ref and tag-ref dispatch values and prove it fails closed.

Usage: python3 scripts/test_verify_release_recover_workflow_provenance.py
"""

import os
import subprocess
import sys
import tempfile
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from test_verify_release_workflow_provenance import (
    _extract_job_names,
    _find_job_block_lines,
    _parse_steps_from_block,
)
from verify_release_recover_workflow_provenance import check_recover_provenance_contract

REPO_ROOT = Path(__file__).resolve().parent.parent

PASS = 0
FAIL = 0


def expect_violation(name: str, yaml_content: str, needle: str) -> None:
    """Write a temp workflow, run the checker, assert violation containing needle."""
    global PASS, FAIL
    with tempfile.NamedTemporaryFile(mode="w", suffix=".yml", delete=False) as f:
        f.write(yaml_content)
        f.flush()
        violations = check_recover_provenance_contract(Path(f.name))

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
        violations = check_recover_provenance_contract(Path(f.name))

    if not violations:
        print(f"  PASS: {name}")
        PASS += 1
    else:
        print(f"  FAIL: {name} — expected no violations, got: {violations}")
        FAIL += 1


# --- Base valid recovery workflow template ---

HEADER = """\
name: release-recover
on:
  workflow_dispatch:
    inputs:
      version:
        required: true
        type: string
      dry_run:
        required: true
        default: true
        type: boolean
permissions:
  contents: read
  id-token: write
jobs:
"""

GUARD_STEP = """\
      - name: Guard recovery dispatch ref
        run: |
          set -euo pipefail
          if [ "${GITHUB_REF}" != "refs/heads/main" ]; then
            echo "::error::bad dispatch ref"
            exit 1
          fi
"""

PREFLIGHT_TAIL = """\
      - name: Checkout release tag
        uses: actions/checkout@v6
        with:
          ref: refs/tags/${{ inputs.version }}
          fetch-depth: 0
      - name: Fetch origin main
        run: git fetch origin main
      - name: Verify post-tag candidate provenance
        env:
          RELEASE_MAIN_REF: refs/remotes/origin/main
          VERSION: ${{ inputs.version }}
        run: |
          set -euo pipefail
          make posttag-candidate-gate VERSION="${VERSION}"
      - name: Resolve verified tag target SHA
        id: tag-target
        run: |
          set -euo pipefail
          tag_target_sha="$(git rev-parse "refs/tags/${VERSION}^{commit}")"
          echo "tag_target_sha=${tag_target_sha}" >> "$GITHUB_OUTPUT"
      - name: Extract checksums from release
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          set -euo pipefail
          gh release download "${VERSION}"
"""

PREFLIGHT_HEAD = """\
  preflight:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    outputs:
      tag_target_sha: ${{ steps.tag-target.outputs.tag_target_sha }}
    steps:
"""

VALID_PREFLIGHT = PREFLIGHT_HEAD + GUARD_STEP + PREFLIGHT_TAIL

VALID_HOMEBREW = """\
  publish-homebrew-cask:
    needs: preflight
    runs-on: ubuntu-latest
    steps:
      - name: Checkout verified tag target
        uses: actions/checkout@v6
        with:
          ref: ${{ needs.preflight.outputs.tag_target_sha }}
      - name: Publish Homebrew cask
        run: |
          set -euo pipefail
          git push origin HEAD:main
"""

VALID_VERIFY = """\
  verify-homebrew-cask-install:
    needs: publish-homebrew-cask
    runs-on: macos-15
    steps:
      - name: Verify install
        run: |
          set -euo pipefail
          brew install --cask deltascope
"""

VALID_NPM = """\
  publish-mcp-launcher-package:
    needs: preflight
    runs-on: ubuntu-latest
    steps:
      - name: Checkout verified tag target
        uses: actions/checkout@v6
        with:
          ref: ${{ needs.preflight.outputs.tag_target_sha }}
      - name: Publish npm launcher package
        run: |
          set -euo pipefail
          npm publish --access public --provenance ./packages/deltascope-mcp
"""


def make_workflow(preflight: str = VALID_PREFLIGHT,
                  homebrew: str = VALID_HOMEBREW,
                  verify: str = VALID_VERIFY,
                  npm: str = VALID_NPM) -> str:
    return HEADER + preflight + homebrew + verify + npm


# --- Static fixture cases ---

print("=== test_verify_release_recover_workflow_provenance (adversarial) ===")

print("R0: valid recovery workflow")
expect_clean("valid recovery workflow passes", make_workflow())

print("R1: missing preflight job")
expect_violation("catches missing preflight job",
    HEADER + VALID_HOMEBREW + VALID_VERIFY + VALID_NPM,
    "missing 'preflight' job")

print("R2: missing dispatch-ref guard")
expect_violation("catches missing guard",
    make_workflow(preflight=PREFLIGHT_HEAD + PREFLIGHT_TAIL),
    "missing fail-closed dispatch-ref guard")

print("R3: wrong guard value")
wrong_guard = GUARD_STEP.replace('refs/heads/main', 'refs/heads/master')
expect_violation("catches wrong guard ref",
    make_workflow(preflight=PREFLIGHT_HEAD + wrong_guard + PREFLIGHT_TAIL),
    "dispatch-ref guard")

print("R4: near-miss guard value (refs/heads/main2)")
near_guard = GUARD_STEP.replace('"refs/heads/main"', '"refs/heads/main2"')
expect_violation("catches near-miss guard ref",
    make_workflow(preflight=PREFLIGHT_HEAD + near_guard + PREFLIGHT_TAIL),
    "dispatch-ref guard")

print("R5: guard placed after external work")
checkout_first = PREFLIGHT_HEAD + """\
      - name: Checkout release tag
        uses: actions/checkout@v6
        with:
          ref: refs/tags/${{ inputs.version }}
          fetch-depth: 0
""" + GUARD_STEP + """\
      - name: Fetch origin main
        run: git fetch origin main
      - name: Verify post-tag candidate provenance
        env:
          RELEASE_MAIN_REF: refs/remotes/origin/main
          VERSION: ${{ inputs.version }}
        run: |
          set -euo pipefail
          make posttag-candidate-gate VERSION="${VERSION}"
      - name: Resolve verified tag target SHA
        id: tag-target
        run: |
          set -euo pipefail
          tag_target_sha="$(git rev-parse "refs/tags/${VERSION}^{commit}")"
          echo "tag_target_sha=${tag_target_sha}" >> "$GITHUB_OUTPUT"
"""
expect_violation("catches guard not first",
    make_workflow(preflight=checkout_first),
    "must be the FIRST step")

print("R6: preflight default-branch checkout")
default_checkout = VALID_PREFLIGHT.replace("""\
        with:
          ref: refs/tags/${{ inputs.version }}
          fetch-depth: 0
""", "")
expect_violation("catches default-branch checkout in preflight",
    make_workflow(preflight=default_checkout),
    "missing checkout of ref: refs/tags/${{ inputs.version }}")

print("R7: shallow tag checkout (no fetch-depth: 0)")
shallow = VALID_PREFLIGHT.replace("          fetch-depth: 0\n", "")
expect_violation("catches shallow checkout",
    make_workflow(preflight=shallow),
    "missing fetch-depth: 0")

print("R8: missing origin/main fetch")
no_fetch = VALID_PREFLIGHT.replace("""\
      - name: Fetch origin main
        run: git fetch origin main
""", "")
expect_violation("catches missing origin/main fetch",
    make_workflow(preflight=no_fetch),
    "does not fetch origin/main")

print("R9: missing post-tag gate")
no_gate = VALID_PREFLIGHT.replace("posttag-candidate-gate", "echo skipped")
expect_violation("catches missing post-tag gate",
    make_workflow(preflight=no_gate),
    "posttag-candidate-gate")

print("R10: env/run split across steps")
split_env = VALID_PREFLIGHT.replace("""\
      - name: Verify post-tag candidate provenance
        env:
          RELEASE_MAIN_REF: refs/remotes/origin/main
          VERSION: ${{ inputs.version }}
        run: |
          set -euo pipefail
          make posttag-candidate-gate VERSION="${VERSION}"
""", """\
      - name: Set env only
        env:
          RELEASE_MAIN_REF: refs/remotes/origin/main
        run: echo "env set on the wrong step"
      - name: Verify post-tag candidate provenance
        run: |
          set -euo pipefail
          make posttag-candidate-gate VERSION="${VERSION}"
""")
expect_violation("catches env/run split",
    make_workflow(preflight=split_env),
    "on the same step")

print("R11: post-tag gate after external release-state work")
gate_late = PREFLIGHT_HEAD + GUARD_STEP + """\
      - name: Checkout release tag
        uses: actions/checkout@v6
        with:
          ref: refs/tags/${{ inputs.version }}
          fetch-depth: 0
      - name: Fetch origin main
        run: git fetch origin main
      - name: Extract checksums from release
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          set -euo pipefail
          gh release download "${VERSION}"
      - name: Verify post-tag candidate provenance
        env:
          RELEASE_MAIN_REF: refs/remotes/origin/main
          VERSION: ${{ inputs.version }}
        run: |
          set -euo pipefail
          make posttag-candidate-gate VERSION="${VERSION}"
      - name: Resolve verified tag target SHA
        id: tag-target
        run: |
          set -euo pipefail
          tag_target_sha="$(git rev-parse "refs/tags/${VERSION}^{commit}")"
          echo "tag_target_sha=${tag_target_sha}" >> "$GITHUB_OUTPUT"
"""
expect_violation("catches gate after external work",
    make_workflow(preflight=gate_late),
    "must run before external release-state work")

print("R12: missing tag_target_sha resolution step")
no_resolve = VALID_PREFLIGHT.replace("""\
      - name: Resolve verified tag target SHA
        id: tag-target
        run: |
          set -euo pipefail
          tag_target_sha="$(git rev-parse "refs/tags/${VERSION}^{commit}")"
          echo "tag_target_sha=${tag_target_sha}" >> "$GITHUB_OUTPUT"
""", "")
expect_violation("catches missing SHA resolution",
    make_workflow(preflight=no_resolve),
    "does not resolve tag_target_sha")

print("R13: tag_target_sha resolved before the gate")
resolve_early = PREFLIGHT_HEAD + GUARD_STEP + """\
      - name: Checkout release tag
        uses: actions/checkout@v6
        with:
          ref: refs/tags/${{ inputs.version }}
          fetch-depth: 0
      - name: Resolve verified tag target SHA
        id: tag-target
        run: |
          set -euo pipefail
          tag_target_sha="$(git rev-parse "refs/tags/${VERSION}^{commit}")"
          echo "tag_target_sha=${tag_target_sha}" >> "$GITHUB_OUTPUT"
      - name: Fetch origin main
        run: git fetch origin main
      - name: Verify post-tag candidate provenance
        env:
          RELEASE_MAIN_REF: refs/remotes/origin/main
          VERSION: ${{ inputs.version }}
        run: |
          set -euo pipefail
          make posttag-candidate-gate VERSION="${VERSION}"
"""
expect_violation("catches SHA resolved before gate",
    make_workflow(preflight=resolve_early),
    "AFTER the post-tag candidate gate")

print("R14: tag_target_sha not exported through job outputs")
no_output = VALID_PREFLIGHT.replace("""\
    outputs:
      tag_target_sha: ${{ steps.tag-target.outputs.tag_target_sha }}
""", "")
expect_violation("catches missing output export",
    make_workflow(preflight=no_output),
    "do not export tag_target_sha")

print("R15: publisher default checkout")
hb_default = VALID_HOMEBREW.replace("""\
        with:
          ref: ${{ needs.preflight.outputs.tag_target_sha }}
""", "")
expect_violation("catches publisher default checkout",
    make_workflow(homebrew=hb_default),
    "checkout ref must be exactly")

print("R16: publisher checks out the input tag ref")
hb_tag = VALID_HOMEBREW.replace(
    "ref: ${{ needs.preflight.outputs.tag_target_sha }}",
    "ref: refs/tags/${{ inputs.version }}")
expect_violation("catches publisher input-tag checkout",
    make_workflow(homebrew=hb_tag),
    "checkout ref must be exactly")

print("R17: publisher checks out main")
npm_main = VALID_NPM.replace(
    "ref: ${{ needs.preflight.outputs.tag_target_sha }}",
    "ref: main")
expect_violation("catches publisher main checkout",
    make_workflow(npm=npm_main),
    "checkout ref must be exactly")

print("R18: publisher checks out a non-preflight SHA")
npm_sha = VALID_NPM.replace(
    "ref: ${{ needs.preflight.outputs.tag_target_sha }}",
    "ref: 0123456789abcdef0123456789abcdef01234567")
expect_violation("catches publisher foreign SHA checkout",
    make_workflow(npm=npm_sha),
    "checkout ref must be exactly")

print("R19: mutation job without any checkout")
npm_no_checkout = """\
  publish-mcp-launcher-package:
    needs: preflight
    runs-on: ubuntu-latest
    steps:
      - name: Publish npm launcher package
        run: |
          set -euo pipefail
          npm publish --access public --provenance ./packages/deltascope-mcp
"""
expect_violation("catches mutation job without checkout",
    make_workflow(npm=npm_no_checkout),
    "has no checkout pinned")

print("R20: independent mutation path bypassing preflight")
rogue = VALID_NPM + """\
  rogue-publisher:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout verified tag target
        uses: actions/checkout@v6
        with:
          ref: ${{ needs.preflight.outputs.tag_target_sha }}
      - name: Publish
        run: |
          set -euo pipefail
          npm publish --access public
"""
expect_violation("catches rogue publisher",
    make_workflow(npm=rogue),
    "rogue-publisher")

print("R21: GoReleaser action mutation path bypassing preflight")
rogue_goreleaser = VALID_NPM + """\
  rogue-goreleaser:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout verified tag target
        uses: actions/checkout@v6
        with:
          ref: ${{ needs.preflight.outputs.tag_target_sha }}
      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          args: release --clean
"""
expect_violation("catches rogue GoReleaser action",
    make_workflow(npm=rogue_goreleaser),
    "rogue-goreleaser")

print("R22: preflight write permissions")
write_perm = VALID_PREFLIGHT.replace("      contents: read\n", "      contents: write\n", 1)
expect_violation("catches write permissions",
    make_workflow(preflight=write_perm),
    "expected 'read'")

print("R23: preflight missing job-level permissions")
no_perm = VALID_PREFLIGHT.replace("""\
    permissions:
      contents: read
""", "")
expect_violation("catches missing job-level permissions",
    make_workflow(preflight=no_perm),
    "missing job-level permissions")

print("R24: preflight job-level id-token: write")
idtoken = VALID_PREFLIGHT.replace("""\
    permissions:
      contents: read
""", """\
    permissions:
      contents: read
      id-token: write
""")
expect_violation("catches write-capable id-token permission",
    make_workflow(preflight=idtoken),
    "write-capable permission")

print("R25: preflight references a publisher secret")
secret_pre = VALID_PREFLIGHT.replace("""\
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
""", """\
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
""")
expect_violation("catches publisher secret in preflight",
    make_workflow(preflight=secret_pre),
    "publisher secret")

print("R26: historical-version bypass literal")
bypass_hb = VALID_HOMEBREW.replace("""\
          set -euo pipefail
          git push origin HEAD:main
""", """\
          set -euo pipefail
          if [ "${VERSION}" = "v0.240.0" ]; then echo "legacy allowlist"; fi
          git push origin HEAD:main
""")
expect_violation("catches historical tag allowlist",
    make_workflow(homebrew=bypass_hb),
    "historical tag")


# --- Guard behavior tests against the REAL workflow ---

print("G: dispatch-ref guard behavior (real workflow, branch/tag dispatch refs)")


def run_real_guard(env_overrides: dict) -> int:
    workflow = REPO_ROOT / ".github" / "workflows" / "release-recover.yml"
    content = workflow.read_text(encoding="utf-8")
    job_names = _extract_job_names(content)
    block, indent = _find_job_block_lines(content, "preflight", job_names)
    steps = _parse_steps_from_block(block, indent)
    guard = steps[0]
    script = "\n".join(guard.run_lines) + "\n"
    with tempfile.NamedTemporaryFile(mode="w", suffix=".sh", delete=False) as f:
        f.write(script)
        path = f.name
    env = dict(os.environ)
    env.update(env_overrides)
    proc = subprocess.run(["bash", path], env=env,
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    return proc.returncode


def expect_guard(name: str, env_overrides: dict, expect_ok: bool) -> None:
    global PASS, FAIL
    rc = run_real_guard(env_overrides)
    ok = (rc == 0) if expect_ok else (rc != 0)
    if ok:
        print(f"  PASS: {name}")
        PASS += 1
    else:
        print(f"  FAIL: {name} — guard exited {rc}")
        FAIL += 1


expect_guard("main dispatch with valid version passes",
             {"GITHUB_REF": "refs/heads/main", "VERSION": "v9.9.9"}, True)
expect_guard("branch-ref dispatch fails closed",
             {"GITHUB_REF": "refs/heads/feature-x", "VERSION": "v9.9.9"}, False)
expect_guard("tag-ref dispatch fails closed",
             {"GITHUB_REF": "refs/tags/v0.460.0", "VERSION": "v0.460.0"}, False)
expect_guard("near-miss branch ref fails closed",
             {"GITHUB_REF": "refs/heads/main2", "VERSION": "v9.9.9"}, False)
expect_guard("malformed version input fails closed",
             {"GITHUB_REF": "refs/heads/main", "VERSION": "v9.9.9; rm -rf /"}, False)

print("")
print(f"Results: {PASS} passed, {FAIL} failed")
sys.exit(0 if FAIL == 0 else 1)
