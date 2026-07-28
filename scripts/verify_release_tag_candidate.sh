#!/usr/bin/env bash
# verify_release_tag_candidate.sh — Post-tag verification gate.
# Runs AFTER `git tag` to confirm the tag points at the .release-candidate
# commit and that the reviewed candidate (HEAD^ at tag time) matches the file.
#
# Usage:
#   bash scripts/verify_release_tag_candidate.sh v0.460.0
#
# Exit codes:
#   0 — all checks pass
#   1 — guard failure

set -euo pipefail

TAG="${1:?usage: $0 <tag>}"
RC_FILE=".release-candidate"

# --- Resolve approved candidate SHA from file or env ---
APPROVED_SHA=""
if [ -f "$RC_FILE" ]; then
  APPROVED_SHA="$(sed -n 's/^candidate_sha: *//p' "$RC_FILE" | tr -d '[:space:]')"
fi
if [ -z "$APPROVED_SHA" ] && [ -n "${RELEASE_CANDIDATE_SHA:-}" ]; then
  APPROVED_SHA="$RELEASE_CANDIDATE_SHA"
fi

# --- Check 1: tag must exist ---
if ! git rev-parse "$TAG" >/dev/null 2>&1; then
  echo "::error::Tag $TAG does not exist locally."
  exit 1
fi

# --- Check 2: tag must be annotated ---
TAG_TYPE="$(git cat-file -t "$TAG")"
if [ "$TAG_TYPE" != "tag" ]; then
  echo "::error::$TAG is a lightweight tag (object type: $TAG_TYPE). Use: git tag -a $TAG -m 'Release $TAG'"
  exit 1
fi
echo "posttag-candidate: tag type=$TAG_TYPE PASS"

# --- Check 3: peeled tag target must be on main ---
TAG_TARGET="$(git rev-parse "$TAG^{}")"
if ! git merge-base --is-ancestor "$TAG_TARGET" main 2>/dev/null; then
  echo "::error::Tag $TAG target $TAG_TARGET is not an ancestor of main."
  exit 1
fi
echo "posttag-candidate: tag target=$TAG_TARGET on main PASS"

# --- Check 4: if approved SHA is known, verify it's the parent of the tag target ---
if [ -n "$APPROVED_SHA" ]; then
  TAG_PARENT="$(git rev-parse "${TAG_TARGET}^" 2>/dev/null)" || {
    echo "::error::Tag target $TAG_TARGET has no parent."
    exit 1
  }
  if [ "$TAG_PARENT" != "$APPROVED_SHA" ]; then
    echo "::error::Reviewed candidate ($APPROVED_SHA) is not the parent of tag target ($TAG_PARENT)."
    exit 1
  fi
  echo "posttag-candidate: reviewed candidate ($APPROVED_SHA) is parent of tag target PASS"
else
  echo "posttag-candidate: no approved SHA available, skipping candidate check"
fi

echo "posttag-candidate: all checks PASS for $TAG"
