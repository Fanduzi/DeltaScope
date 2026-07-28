#!/usr/bin/env bash
# verify_pretag_candidate.sh — Pre-tag gate that MUST run before `git tag`.
#
# Reads the approved candidate SHA from .release-candidate (committed on the
# release branch during prep). The file records the LAST REVIEWED commit.
# The .release-candidate commit itself is the commit AFTER the reviewed one.
#
# Verifies:
#   1. candidate_sha == HEAD^  (the reviewed commit is HEAD's parent)
#   2. HEAD^..HEAD only changed .release-candidate  (no unreviewed files)
#   3. HEAD is on main
#   4. Tag does not already exist
#   5. Working tree is clean
#
# This prevents:
#   - Unreviewed commits between the candidate and tag
#   - Tampering with the candidate SHA after the fact
#   - Omitting the .release-candidate file (fail-closed)
#
# Usage:
#   bash scripts/verify_pretag_candidate.sh v0.460.0
#
# Exit codes:
#   0 — all checks pass; safe to create the tag
#   1 — guard failure (with diagnostic output)

set -euo pipefail

TAG="${1:?usage: $0 <tag>}"
RC_FILE=".release-candidate"

# --- Load approved candidate SHA from the committed file ---

if [ ! -f "$RC_FILE" ]; then
  echo "::error::$RC_FILE does not exist. The release prep must commit this file on the release branch before merging to main."
  exit 1
fi

FILE_VERSION="$(sed -n 's/^version: *//p' "$RC_FILE" | tr -d '[:space:]')"
FILE_SHA="$(sed -n 's/^candidate_sha: *//p' "$RC_FILE" | tr -d '[:space:]')"

if [ -z "$FILE_SHA" ]; then
  echo "::error::$RC_FILE is missing or has empty candidate_sha."
  exit 1
fi

if [ -z "$FILE_VERSION" ]; then
  echo "::error::$RC_FILE is missing or has empty version."
  exit 1
fi

# Verify file version matches the tag being created.
EXPECTED_VERSION="${TAG#v}"
FILE_VERSION_NO_V="${FILE_VERSION#v}"
if [ "$FILE_VERSION_NO_V" != "$EXPECTED_VERSION" ]; then
  echo "::error::$RC_FILE version ($FILE_VERSION) does not match tag ($TAG)."
  exit 1
fi
echo "pretag-candidate: file version=$FILE_VERSION matches tag=$TAG PASS"

# --- If env var is set, it must agree with the file (no override allowed) ---

if [ -n "${RELEASE_CANDIDATE_SHA:-}" ]; then
  if [ "$RELEASE_CANDIDATE_SHA" != "$FILE_SHA" ]; then
    echo "::error::RELEASE_CANDIDATE_SHA ($RELEASE_CANDIDATE_SHA) conflicts with $RC_FILE ($FILE_SHA)."
    echo "::error::The file is the source of truth. Do not override it."
    exit 1
  fi
  echo "pretag-candidate: env var agrees with file PASS"
fi

# --- Core checks ---

HEAD_SHA="$(git rev-parse HEAD)"

# Check 1: HEAD must be on main.
CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [ "$CURRENT_BRANCH" != "main" ]; then
  echo "::error::Current branch is '$CURRENT_BRANCH', expected 'main'. Tag must be created on main."
  exit 1
fi
echo "pretag-candidate: on main PASS"

# Check 2: candidate_sha must be HEAD's parent (the reviewed commit).
PARENT_SHA="$(git rev-parse HEAD^ 2>/dev/null)" || {
  echo "::error::HEAD has no parent (root commit). Cannot verify candidate."
  exit 1
}
if [ "$PARENT_SHA" != "$FILE_SHA" ]; then
  echo "::error::Reviewed candidate ($FILE_SHA) is not HEAD's parent ($PARENT_SHA)."
  echo "::error::Expected: .release-candidate commit is exactly one commit after the reviewed candidate."
  echo "::error::If additional commits were added between the candidate and the RC file, re-do the release prep."
  exit 1
fi
echo "pretag-candidate: reviewed candidate ($FILE_SHA) is HEAD^ PASS"

# Check 3: HEAD^..HEAD must only differ by .release-candidate (no unreviewed files).
CHANGED_FILES="$(git diff-tree --no-commit-id --name-only -r HEAD^..HEAD)"
UNREVIEWED="$(echo "$CHANGED_FILES" | grep -v "^${RC_FILE}$" || true)"
if [ -n "$UNREVIEWED" ]; then
  echo "::error::Unreviewed files changed between candidate and HEAD:"
  echo "$UNREVIEWED"
  echo "::error::Only .release-candidate should differ. Re-do the release prep."
  exit 1
fi
echo "pretag-candidate: only .release-candidate changed PASS"

# Check 4: Tag must not already exist (local or remote).
if git rev-parse "$TAG" >/dev/null 2>&1; then
  echo "::error::Tag $TAG already exists locally. Delete it first if this is a retry: git tag -d $TAG"
  exit 1
fi
REMOTE_TAG="$(git ls-remote --tags origin "refs/tags/${TAG}" 2>/dev/null | head -1 || true)"
if [ -n "$REMOTE_TAG" ]; then
  echo "::error::Tag $TAG already exists on origin ($REMOTE_TAG). Cannot push a duplicate tag."
  exit 1
fi
echo "pretag-candidate: tag $TAG does not exist locally or on origin PASS"
# Check 5: Working tree must be clean (tracked files only).
DIRTY="$(git status --porcelain --untracked-files=no)"
if [ -n "$DIRTY" ]; then
  echo "::error::Working tree has uncommitted changes:"
  echo "$DIRTY"
  exit 1
fi
echo "pretag-candidate: working tree clean PASS"

echo "pretag-candidate: all checks PASS — safe to create tag $TAG at $HEAD_SHA (reviewed candidate: $FILE_SHA)"
