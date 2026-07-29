#!/usr/bin/env bash
# release_from_candidate.sh — Local release orchestrator.
# The documented, repository-owned path for creating a release tag.
#
# Mutating sequence (non dry-run):
#   1. Preflight: on main, clean tree, no existing tag, VERSION provided
#   2. Full release test gates (make release-test-gates)
#   3. Full release contract gates (make release-contract-gates)
#   4. Pre-tag candidate gate (make pretag-candidate-gate)
#   5. Create annotated tag at HEAD
#   6. Post-tag candidate gate (make posttag-candidate-gate)
#   7. Push main (without tags)
#   8. Push only the new tag
#
# Any failure stops the process. No automatic tag deletion, retry,
# force push, or recovery is performed.
#
# --dry-run executes read-only preflight and pre-tag gate only.
# It does not create, delete, or push any Git ref.
#
# Usage:
#   bash scripts/release_from_candidate.sh v0.461.0
#   bash scripts/release_from_candidate.sh v0.461.0 --dry-run
#
# Exit codes:
#   0 — success (or dry-run passed)
#   1 — failure at any step

set -euo pipefail

VERSION=""
DRY_RUN=false

usage() {
  echo "usage: $0 <version> [--dry-run]" >&2
  echo "  version must start with v (e.g. v0.461.0)" >&2
  exit 1
}

# --- Parse arguments ---
for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=true ;;
    -*) echo "error: unknown flag: $arg" >&2; usage ;;
    *)
      if [ -z "$VERSION" ]; then
        VERSION="$arg"
      else
        echo "error: unexpected argument: $arg" >&2
        usage
      fi
      ;;
  esac
done

if [ -z "$VERSION" ]; then
  echo "error: VERSION is required" >&2
  usage
fi

case "$VERSION" in
  v*) ;;
  *) echo "error: VERSION must start with v (got: $VERSION)" >&2; exit 1 ;;
esac

MODE="release"
if [ "$DRY_RUN" = true ]; then
  MODE="dry-run"
fi

echo "=== release orchestrator: $MODE for $VERSION ==="

# --- Preflight checks ---
echo "--- preflight ---"

CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [ "$CURRENT_BRANCH" != "main" ]; then
  echo "::error::Not on main (on: $CURRENT_BRANCH)." >&2
  exit 1
fi
echo "preflight: on main PASS"

DIRTY="$(git status --porcelain --untracked-files=no)"
if [ -n "$DIRTY" ]; then
  echo "::error::Working tree has uncommitted tracked changes:" >&2
  echo "$DIRTY" >&2
  exit 1
fi
echo "preflight: working tree clean PASS"

# Check local tag collision
if git rev-parse "$VERSION" >/dev/null 2>&1; then
  echo "::error::Tag $VERSION already exists locally." >&2
  exit 1
fi

# Check remote tag collision
REMOTE_TAG="$(git ls-remote --tags origin "refs/tags/${VERSION}" 2>/dev/null | head -1 || true)"
if [ -n "$REMOTE_TAG" ]; then
  echo "::error::Tag $VERSION already exists on origin." >&2
  exit 1
fi
echo "preflight: no tag collision PASS"

echo "--- release test gates ---"
make release-test-gates VERSION="$VERSION"
echo "release test gates: PASS"

echo "--- release contract gates ---"
make release-contract-gates VERSION="$VERSION"
echo "release contract gates: PASS"

echo "--- pretag-candidate-gate ---"
make pretag-candidate-gate VERSION="$VERSION"
echo "pretag-candidate-gate: PASS"

# --- Dry-run stops here ---
if [ "$DRY_RUN" = true ]; then
  echo ""
  echo "=== dry-run complete: all preflight and pre-tag checks passed ==="
  echo "=== No tag created, no push performed, no remote mutations ==="
  exit 0
fi

# --- Mutating steps ---
HEAD_SHA="$(git rev-parse HEAD)"
echo "--- creating annotated tag $VERSION at $HEAD_SHA ---"
git tag -a "$VERSION" -m "Release $VERSION"
echo "tag created: $VERSION at $HEAD_SHA"

echo "--- posttag-candidate-gate ---"
make posttag-candidate-gate VERSION="$VERSION"
echo "posttag-candidate-gate: PASS"

echo "--- push main ---"
git push origin main
echo "push main: PASS"

echo "--- push tag $VERSION ---"
git push origin "$VERSION"
echo "push tag: PASS"

echo ""
echo "=== release orchestrator: $VERSION complete ==="
echo "tag=$VERSION sha=$HEAD_SHA"
