#!/usr/bin/env bash
# verify_pretag_candidate.sh — Pre-tag gate that MUST run before `git tag`.
# Verifies HEAD matches the approved release candidate SHA and that the
# working tree is clean. Fails closed: omission of RELEASE_CANDIDATE_SHA
# is a hard error, not a skip.
#
# Usage:
#   RELEASE_CANDIDATE_SHA=<full-sha> bash scripts/verify_pretag_candidate.sh v0.460.0
#
# Exit codes:
#   0 — all checks pass; safe to create the tag
#   1 — guard failure (with diagnostic output)

set -euo pipefail

TAG="${1:?usage: $0 <tag>}"

# Fail closed: RELEASE_CANDIDATE_SHA is mandatory.
if [ -z "${RELEASE_CANDIDATE_SHA:-}" ]; then
  echo "::error::RELEASE_CANDIDATE_SHA is not set. Pre-tag gate requires the approved candidate SHA."
  echo "::error::Usage: RELEASE_CANDIDATE_SHA=<full-sha> bash scripts/verify_pretag_candidate.sh $TAG"
  exit 1
fi

# Check 1: HEAD must match the approved candidate exactly.
HEAD_SHA="$(git rev-parse HEAD)"
if [ "$HEAD_SHA" != "$RELEASE_CANDIDATE_SHA" ]; then
  echo "::error::HEAD ($HEAD_SHA) does not match approved candidate ($RELEASE_CANDIDATE_SHA)."
  echo "::error::If a post-merge fix is needed: commit on the release branch, re-run gates, ff-merge again, then re-run this gate."
  exit 1
fi
echo "pretag-candidate: HEAD matches approved candidate PASS"

# Check 2: HEAD must be on main.
CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [ "$CURRENT_BRANCH" != "main" ]; then
  echo "::error::Current branch is '$CURRENT_BRANCH', expected 'main'. Tag must be created on main."
  exit 1
fi
echo "pretag-candidate: on main PASS"

# Check 3: Tag must not already exist.
if git rev-parse "$TAG" >/dev/null 2>&1; then
  echo "::error::Tag $TAG already exists locally. Delete it first if this is a retry: git tag -d $TAG"
  exit 1
fi
echo "pretag-candidate: tag $TAG does not exist PASS"

# Check 4: Working tree must be clean (tracked files only).
DIRTY="$(git status --porcelain --untracked-files=no)"
if [ -n "$DIRTY" ]; then
  echo "::error::Working tree has uncommitted changes:"
  echo "$DIRTY"
  exit 1
fi
echo "pretag-candidate: working tree clean PASS"

echo "pretag-candidate: all checks PASS — safe to create tag $TAG at $HEAD_SHA"
