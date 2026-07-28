#!/usr/bin/env bash
# verify_release_tag_candidate.sh — Pre-tag guard that verifies the tag target
# matches the approved release candidate SHA.
#
# Usage:
#   RELEASE_CANDIDATE_SHA=<full-sha> bash scripts/verify_release_tag_candidate.sh v0.460.0
#
# If RELEASE_CANDIDATE_SHA is not set, the guard only checks that:
#   1. The tag exists and is annotated (not lightweight).
#   2. The tag target is an ancestor of main (i.e., on the main branch).
#
# Exit codes:
#   0 — all checks pass
#   1 — guard failure (with diagnostic output)

set -euo pipefail

TAG="${1:?usage: $0 <tag>}"

# Check 1: tag must exist
if ! git rev-parse "$TAG" >/dev/null 2>&1; then
  echo "::error::Tag $TAG does not exist locally."
  exit 1
fi

# Check 2: tag must be annotated
TAG_TYPE="$(git cat-file -t "$TAG")"
if [ "$TAG_TYPE" != "tag" ]; then
  echo "::error::$TAG is a lightweight tag (object type: $TAG_TYPE). Use: git tag -a $TAG -m 'Release $TAG'"
  exit 1
fi
echo "verify-tag-candidate: tag type=$TAG_TYPE PASS"

# Check 3: peeled tag target must be on main
TAG_TARGET="$(git rev-parse "$TAG^{}")"
if ! git merge-base --is-ancestor "$TAG_TARGET" main 2>/dev/null; then
  echo "::error::Tag $TAG target $TAG_TARGET is not an ancestor of main."
  exit 1
fi
echo "verify-tag-candidate: tag target=$TAG_TARGET on main PASS"

# Check 4: if RELEASE_CANDIDATE_SHA is set, enforce exact match
if [ -n "${RELEASE_CANDIDATE_SHA:-}" ]; then
  if [ "$TAG_TARGET" != "$RELEASE_CANDIDATE_SHA" ]; then
    echo "::error::Tag $TAG target $TAG_TARGET does not match approved candidate $RELEASE_CANDIDATE_SHA."
    echo "::error::If a post-merge fix was needed, commit it on the release branch, re-run gates, ff-merge again, and tag the new HEAD."
    exit 1
  fi
  echo "verify-tag-candidate: tag target matches approved candidate PASS"
else
  echo "verify-tag-candidate: RELEASE_CANDIDATE_SHA not set, skipping exact-match check"
fi

echo "verify-tag-candidate: all checks PASS for $TAG"
