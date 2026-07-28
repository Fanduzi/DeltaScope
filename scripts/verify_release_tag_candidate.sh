#!/usr/bin/env bash
# verify_release_tag_candidate.sh — Post-tag verification gate.
# Runs AFTER `git tag` to confirm the tag points at a valid .release-candidate
# commit. Reads the file FROM THE TAGGED COMMIT, not the worktree.
#
# Verifies:
#   1. Tag exists and is annotated
#   2. Tag target is on main
#   3. Tagged commit contains .release-candidate with correct version
#   4. candidate_sha == tag_target^ (reviewed commit is the parent)
#   5. tag_target^..tag_target only changed .release-candidate
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

# --- Check 4: load .release-candidate FROM THE TAGGED COMMIT ---
RC_CONTENT="$(git show "${TAG_TARGET}:${RC_FILE}" 2>/dev/null)" || {
  echo "::error::Tagged commit $TAG_TARGET does not contain $RC_FILE."
  echo "::error::The .release-candidate file must be part of the tagged commit."
  exit 1
}

FILE_VERSION="$(echo "$RC_CONTENT" | sed -n 's/^version: *//p' | tr -d '[:space:]')"
FILE_SHA="$(echo "$RC_CONTENT" | sed -n 's/^candidate_sha: *//p' | tr -d '[:space:]')"

if [ -z "$FILE_SHA" ]; then
  echo "::error::$RC_FILE in tagged commit has empty candidate_sha."
  exit 1
fi

if [ -z "$FILE_VERSION" ]; then
  echo "::error::$RC_FILE in tagged commit has empty version."
  exit 1
fi

# Verify version matches tag.
EXPECTED_VERSION="${TAG#v}"
FILE_VERSION_NO_V="${FILE_VERSION#v}"
if [ "$FILE_VERSION_NO_V" != "$EXPECTED_VERSION" ]; then
  echo "::error::$RC_FILE version ($FILE_VERSION) in tagged commit does not match tag ($TAG)."
  exit 1
fi
echo "posttag-candidate: file version=$FILE_VERSION matches tag=$TAG PASS"

# --- Check 5: candidate_sha must be the parent of the tagged commit ---
TAG_PARENT="$(git rev-parse "${TAG_TARGET}^" 2>/dev/null)" || {
  echo "::error::Tag target $TAG_TARGET has no parent (root commit)."
  exit 1
}
if [ "$TAG_PARENT" != "$FILE_SHA" ]; then
  echo "::error::Reviewed candidate ($FILE_SHA) is not the parent of tag target ($TAG_PARENT)."
  echo "::error::The .release-candidate commit must be exactly one commit after the reviewed candidate."
  exit 1
fi
echo "posttag-candidate: reviewed candidate ($FILE_SHA) is parent of tag target PASS"

# --- Check 6: tag_target^..tag_target must only change .release-candidate ---
CHANGED_FILES="$(git diff-tree --no-commit-id --name-only -r "${TAG_TARGET}^..${TAG_TARGET}")"
UNREVIEWED="$(echo "$CHANGED_FILES" | grep -v "^${RC_FILE}$" || true)"
if [ -n "$UNREVIEWED" ]; then
  echo "::error::Unreviewed files changed between candidate and tagged commit:"
  echo "$UNREVIEWED"
  echo "::error::Only $RC_FILE should differ."
  exit 1
fi
echo "posttag-candidate: only .release-candidate changed PASS"

echo "posttag-candidate: all checks PASS for $TAG (reviewed candidate: $FILE_SHA)"
