#!/usr/bin/env bash
# test_release_from_candidate.sh — Tests for the release orchestrator.
# Uses temporary repositories to verify valid paths, failure modes,
# dry-run safety, and the mutating path.
#
# The orchestrator calls `make` targets. In temp repos we provide a minimal
# Makefile stub so the control flow is exercised without the full repo.
#
# Usage: bash scripts/test_release_from_candidate.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ORCHESTRATOR="$SCRIPT_DIR/release_from_candidate.sh"
PRETAG="$SCRIPT_DIR/verify_pretag_candidate.sh"
POSTTAG="$SCRIPT_DIR/verify_release_tag_candidate.sh"

P=0; F=0; TD=""

cleanup() {
  [ -n "$TD" ] && rm -rf "$TD"
}
trap cleanup EXIT

setup() {
  TD="$(mktemp -d)" && cd "$TD"
  # Create a "remote" bare repo
  REMOTE="$TD/remote.git"
  git init --bare -q "$REMOTE"
  # Create a working clone
  git clone -q "$REMOTE" worktree && cd worktree
  git config user.email "t@t" && git config user.name "T"
  # Initial commit on main
  echo "initial" > README.md && git add README.md
  git commit -q -m "initial commit"
  git branch -M main 2>/dev/null || true
  git push -q origin main
  # Copy the scripts we need
  cp "$SCRIPT_DIR/verify_pretag_candidate.sh" .
  cp "$SCRIPT_DIR/verify_release_tag_candidate.sh" .
  # Create a minimal Makefile stub so the orchestrator's make calls succeed
  cat > Makefile << 'STUB'
.PHONY: release-test-gates release-contract-gates pretag-candidate-gate posttag-candidate-gate

release-test-gates:
	@echo "stub: release-test-gates PASS"

release-contract-gates:
	@echo "stub: release-contract-gates PASS"

pretag-candidate-gate:
	@test -n "$(VERSION)" || (echo "VERSION is required" >&2; exit 1)
	bash verify_pretag_candidate.sh "$(VERSION)"

posttag-candidate-gate:
	@test -n "$(VERSION)" || (echo "VERSION is required" >&2; exit 1)
	bash verify_release_tag_candidate.sh "$(VERSION)"
STUB
}

# Create .release-candidate commit on top of current HEAD.
rc() {
  local v="$1"
  local p
  p="$(git rev-parse HEAD)"
  printf "version: %s\ncandidate_sha: %s\n" "$v" "$p" > .release-candidate
  git add .release-candidate && git commit -q -m "rc $v"
  git push -q origin main
}

pass() { echo "  PASS: $1"; P=$((P+1)); }
fail() { echo "  FAIL: $1"; F=$((F+1)); }
ok() {
  local d="$1"; shift
  if "$@" >/dev/null 2>&1; then pass "$d"; else fail "$d"; fi
}
ng() {
  local d="$1"; shift
  if "$@" >/dev/null 2>&1; then fail "$d"; else pass "$d"; fi
}

echo "=== test_release_from_candidate ==="

# --- T1: Dry-run passes for valid candidate ---
echo "T1: dry-run valid candidate"; setup
rc v0.1.0
ok "dry-run passes" bash "$ORCHESTRATOR" v0.1.0 --dry-run
# Verify no tag was created
ng "no local tag after dry-run" git rev-parse v0.1.0
# Verify no remote tag
REMOTE_TAGS="$(git ls-remote --tags origin 2>/dev/null || true)"
if echo "$REMOTE_TAGS" | grep -q 'refs/tags/v0.1.0'; then
  fail "no remote tag after dry-run"
else
  pass "no remote tag after dry-run"
fi

# --- T2: Dry-run does not push ---
echo "T2: dry-run no push"; setup
rc v0.1.0
REMOTE_BEFORE="$(git ls-remote --heads origin main | head -1 | cut -f1)"
bash "$ORCHESTRATOR" v0.1.0 --dry-run >/dev/null 2>&1 || true
REMOTE_AFTER="$(git ls-remote --heads origin main | head -1 | cut -f1)"
if [ "$REMOTE_BEFORE" = "$REMOTE_AFTER" ]; then
  pass "remote unchanged after dry-run"
else
  fail "remote changed after dry-run"
fi

# --- T3: Missing .release-candidate ---
echo "T3: missing RC file"; setup
echo "no rc" > note.md && git add note.md && git commit -q -m "no rc"
git push -q origin main
ng "rejects missing RC" bash "$ORCHESTRATOR" v0.1.0 --dry-run

# --- T4: Candidate drift (unreviewed commit after RC) ---
echo "T4: candidate drift"; setup
rc v0.1.0
echo "drift" > drift.md && git add drift.md && git commit -q -m "drift"
git push -q origin main
ng "rejects drifted HEAD" bash "$ORCHESTRATOR" v0.1.0 --dry-run

# --- T5: Dirty tracked tree ---
echo "T5: dirty tree"; setup
rc v0.1.0
echo "dirty" > README.md
ng "rejects dirty tree" bash "$ORCHESTRATOR" v0.1.0 --dry-run

# --- T6: Local tag collision ---
echo "T6: local tag collision"; setup
rc v0.1.0
git tag -a v0.1.0 -m "existing"
ng "rejects local tag" bash "$ORCHESTRATOR" v0.1.0 --dry-run

# --- T7: Remote tag collision ---
echo "T7: remote tag collision"; setup
rc v0.1.0
git tag -a v0.1.0 -m "remote tag"
git push -q origin v0.1.0
# Remove local tag so the collision is detected from remote
git tag -d v0.1.0 2>/dev/null || true
ng "rejects remote tag" bash "$ORCHESTRATOR" v0.1.0 --dry-run

# --- T8: Not on main ---
echo "T8: not on main"; setup
rc v0.1.0
git checkout -q -b feature
ng "rejects non-main" bash "$ORCHESTRATOR" v0.1.0 --dry-run

# --- T9: Version mismatch ---
echo "T9: version mismatch"; setup
rc v0.2.0
ng "rejects wrong version" bash "$ORCHESTRATOR" v0.1.0 --dry-run

# --- T10: Missing version argument ---
echo "T10: missing version"; setup
rc v0.1.0
ng "rejects missing arg" bash "$ORCHESTRATOR"

# --- T11: Version without v prefix ---
echo "T11: version without v"; setup
rc v0.1.0
ng "rejects non-v prefix" bash "$ORCHESTRATOR" 0.1.0 --dry-run

# --- T12: Mutating path — tag created, pushed, main pushed first ---
echo "T12: mutating path success"; setup
rc v0.1.0
bash "$ORCHESTRATOR" v0.1.0 >/dev/null 2>&1
# Tag must exist locally
if git rev-parse v0.1.0 >/dev/null 2>&1; then
  pass "tag created locally"
else
  fail "tag not created locally"
fi
# Tag must be annotated
TAG_TYPE="$(git cat-file -t v0.1.0)"
if [ "$TAG_TYPE" = "tag" ]; then
  pass "tag is annotated"
else
  fail "tag is not annotated (type: $TAG_TYPE)"
fi
# Tag must exist on remote
REMOTE_TAGS="$(git ls-remote --tags origin 2>/dev/null || true)"
if echo "$REMOTE_TAGS" | grep -q 'refs/tags/v0.1.0'; then
  pass "tag pushed to remote"
else
  fail "tag not pushed to remote"
fi
# RC commit must be the tag target
TAG_TARGET="$(git rev-parse v0.1.0^{})"
RC_SHA="$(git rev-parse HEAD)"
if [ "$TAG_TARGET" = "$RC_SHA" ]; then
  pass "tag points at RC commit"
else
  fail "tag target ($TAG_TARGET) != HEAD ($RC_SHA)"
fi

# --- T13: Mutating path — stop on posttag failure, no tag push ---
echo "T13: stop on posttag failure"; setup
rc v0.1.0
# Replace posttag-candidate-gate stub with one that always fails
cat > Makefile << 'STUB_FAIL_POSTTAG'
.PHONY: release-test-gates release-contract-gates pretag-candidate-gate posttag-candidate-gate

release-test-gates:
	@echo "stub: release-test-gates PASS"

release-contract-gates:
	@echo "stub: release-contract-gates PASS"

pretag-candidate-gate:
	@echo "stub: pretag-candidate-gate PASS"

posttag-candidate-gate:
	@echo "stub: posttag-candidate-gate FAIL" >&2
	exit 1
STUB_FAIL_POSTTAG
# The orchestrator should fail at posttag gate
ng "fails at posttag gate" bash "$ORCHESTRATOR" v0.1.0
# Tag was created locally (step 5 happens before step 6)
if git rev-parse v0.1.0 >/dev/null 2>&1; then
  pass "tag exists locally after posttag failure"
else
  fail "tag missing locally after posttag failure"
fi
# Tag must NOT be on remote (push never happened)
REMOTE_TAGS="$(git ls-remote --tags origin 2>/dev/null || true)"
if echo "$REMOTE_TAGS" | grep -q 'refs/tags/v0.1.0'; then
  fail "tag should not be on remote after posttag failure"
else
  pass "tag not on remote after posttag failure"
fi

echo ""
echo "Results: $P passed, $F failed"
[ "$F" -eq 0 ] && echo "All tests passed." || exit 1
