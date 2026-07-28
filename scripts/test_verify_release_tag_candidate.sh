#!/usr/bin/env bash
# test_verify_release_tag_candidate.sh — Tests for the post-tag candidate gate.
# Usage: bash scripts/test_verify_release_tag_candidate.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
POSTTAG="$SCRIPT_DIR/verify_release_tag_candidate.sh"
P=0; F=0; TD=""

cleanup() { [ -n "$TD" ] && rm -rf "$TD"; }
trap cleanup EXIT

setup() {
  TD="$(mktemp -d)" && cd "$TD"
  git init -q && git config user.email "t@t" && git config user.name "T"
  echo x > f && git add f && git commit -q -m init
  git branch -M main 2>/dev/null || true
}

# Create .release-candidate commit on top of current HEAD, then tag it.
rc_tag() {
  local v="$1" p
  p="$(git rev-parse HEAD)"
  printf "version: %s\ncandidate_sha: %s\n" "$v" "$p" > .release-candidate
  git add .release-candidate && git commit -q -m "rc $v"
  git tag -a "$v" -m "Release $v"
}

pass() { echo "  PASS: $1"; P=$((P+1)); }
fail() { echo "  FAIL: $1"; F=$((F+1)); }
ok() { local d="$1"; shift; if "$@" >/dev/null 2>&1; then pass "$d"; else fail "$d"; fi }
ng() { local d="$1"; shift; if "$@" >/dev/null 2>&1; then fail "$d"; else pass "$d"; fi }

echo "=== test_verify_release_tag_candidate ==="

echo "P1: tag does not exist"; setup
ng "rejects absent tag" bash "$POSTTAG" v0.1.0

echo "P2: lightweight tag"; setup
echo y > g && git add g && git commit -q -m reviewed
git tag v0.1.0  # lightweight
ng "rejects lightweight tag" bash "$POSTTAG" v0.1.0

echo "P3: wrong version in file"; setup; rc_tag v0.2.0
ng "rejects version mismatch" bash "$POSTTAG" v0.1.0

echo "P4: candidate_sha != tag_target^"; setup
p="$(git rev-parse HEAD)"
printf "version: v0.1.0\ncandidate_sha: 0000000000000000000000000000000000000000\n" > .release-candidate
git add .release-candidate && git commit -q -m "rc bad SHA"
git tag -a v0.1.0 -m "Release v0.1.0"
ng "rejects wrong candidate SHA" bash "$POSTTAG" v0.1.0

echo "P5: extra files in tagged commit"; setup
echo reviewed > sneaky && git add sneaky && git commit -q -m reviewed
p="$(git rev-parse HEAD)"
printf "version: v0.1.0\ncandidate_sha: %s\n" "$p" > .release-candidate
echo unreviewed > sneaky2
git add .release-candidate sneaky2 && git commit -q -m "rc with extra"
git tag -a v0.1.0 -m "Release v0.1.0"
ng "rejects extra files" bash "$POSTTAG" v0.1.0

echo "P6: no .release-candidate in tagged commit"; setup
echo y > g && git add g && git commit -q -m reviewed
git tag -a v0.1.0 -m "no rc file"
ng "rejects missing file in tag" bash "$POSTTAG" v0.1.0

echo "P7: post-tag reads from tag, not worktree"; setup
# Create valid tag with .release-candidate
rc_tag v0.1.0
# Now corrupt the worktree version (simulates the v0.460 scenario)
printf "version: v0.1.0\ncandidate_sha: 0000000000000000000000000000000000000000\n" > .release-candidate
# Gate should still pass because it reads from the tag, not the worktree
ok "reads from tag not worktree" bash "$POSTTAG" v0.1.0

echo "P8: success"; setup; rc_tag v0.1.0
ok "passes all checks" bash "$POSTTAG" v0.1.0

echo ""; echo "Results: $P passed, $F failed"
[ "$F" -eq 0 ] && echo "All tests passed." || exit 1
