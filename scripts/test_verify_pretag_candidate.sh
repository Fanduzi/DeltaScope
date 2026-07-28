#!/usr/bin/env bash
# test_verify_pretag_candidate.sh — Tests for the pre-tag candidate gate.
# Usage: bash scripts/test_verify_pretag_candidate.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PRETAG="$SCRIPT_DIR/verify_pretag_candidate.sh"
P=0; F=0; TD=""

cleanup() { [ -n "$TD" ] && rm -rf "$TD"; }
trap cleanup EXIT

setup() {
  TD="$(mktemp -d)" && cd "$TD"
  git init -q && git config user.email "t@t" && git config user.name "T"
  echo x > f && git add f && git commit -q -m init
  git branch -M main 2>/dev/null || true
}

rc() {
  local v="$1" p
  p="$(git rev-parse HEAD)"
  printf "version: %s\ncandidate_sha: %s\n" "$v" "$p" > .release-candidate
  git add .release-candidate && git commit -q -m "rc $v"
}

pass() { echo "  PASS: $1"; P=$((P+1)); }
fail() { echo "  FAIL: $1"; F=$((F+1)); }
ok() { local d="$1"; shift; if "$@" >/dev/null 2>&1; then pass "$d"; else fail "$d"; fi }
ng() { local d="$1"; shift; if "$@" >/dev/null 2>&1; then fail "$d"; else pass "$d"; fi }

echo "=== test_verify_pretag_candidate ==="

echo "T1: missing file"; setup
ng "rejects absent file" bash "$PRETAG" v0.1.0

echo "T2: version mismatch"; setup; rc v0.2.0
ng "rejects wrong version" bash "$PRETAG" v0.1.0

echo "T3: unreviewed commit after RC"; setup; rc v0.1.0
echo extra >> f && git add f && git commit -q -m unreviewed
ng "rejects HEAD drift" bash "$PRETAG" v0.1.0

echo "T4: not on main"; setup; rc v0.1.0; git checkout -q -b feature
ng "rejects non-main" bash "$PRETAG" v0.1.0

echo "T5: tag exists"; setup; rc v0.1.0; git tag -a v0.1.0 -m dup
ng "rejects existing tag" bash "$PRETAG" v0.1.0

echo "T6: dirty tree"; setup; rc v0.1.0; echo dirty > f
ng "rejects dirty tree" bash "$PRETAG" v0.1.0

echo "T7: env conflicts file"; setup; rc v0.1.0
ng "rejects conflicting env" env RELEASE_CANDIDATE_SHA=0000000000000000000000000000000000000000 bash "$PRETAG" v0.1.0

echo "T8: empty SHA"; setup
printf "version: v0.1.0\ncandidate_sha:\n" > .release-candidate
git add .release-candidate && git commit -q -m bad
ng "rejects empty SHA" bash "$PRETAG" v0.1.0

echo "T9: success"; setup; rc v0.1.0
ok "passes all checks" bash "$PRETAG" v0.1.0

echo "T10: success with agreeing env"; setup; rc v0.1.0
SHA="$(sed -n 's/^candidate_sha: *//p' .release-candidate | tr -d '[:space:]')"
ok "passes with env" env RELEASE_CANDIDATE_SHA="$SHA" bash "$PRETAG" v0.1.0

echo "T11: extra files in RC commit"; setup
echo reviewed > sneaky && git add sneaky && git commit -q -m reviewed
p="$(git rev-parse HEAD)"
printf "version: v0.1.0\ncandidate_sha: %s\n" "$p" > .release-candidate
echo unreviewed > sneaky2
git add .release-candidate sneaky2 && git commit -q -m "rc with extra"
ng "rejects extra files" bash "$PRETAG" v0.1.0

echo ""; echo "Results: $P passed, $F failed"
[ "$F" -eq 0 ] && echo "All tests passed." || exit 1
