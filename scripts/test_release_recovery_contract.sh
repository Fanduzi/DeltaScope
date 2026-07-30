#!/usr/bin/env bash
# test_release_recovery_contract.sh — Hermetic release recovery contract test.
# Simulates the recovery admission flow in temporary git repos: the post-tag
# candidate gate must pass BEFORE any publisher work runs. No network, no
# GitHub Release, no npm registry, no historical repo tags.
# Usage: bash scripts/test_release_recovery_contract.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
POSTTAG="$SCRIPT_DIR/verify_release_tag_candidate.sh"
HYGIENE="$SCRIPT_DIR/verify_release_workflow_hygiene.sh"
P=0; F=0; TD=""; REMOTE=""

cleanup() {
  cd /
  if [ -n "$TD" ]; then rm -rf "$TD"; fi
  if [ -n "$REMOTE" ]; then rm -rf "$REMOTE"; fi
}
trap cleanup EXIT

pass() { echo "  PASS: $1"; P=$((P+1)); }
fail() { echo "  FAIL: $1"; F=$((F+1)); }

# Fresh temp repo with a bare remote acting as origin (simulates the
# fetched origin/main the recovery workflow verifies against).
setup() {
  cleanup; TD=""; REMOTE=""
  TD="$(mktemp -d)" && cd "$TD"
  git init -q && git config user.email "t@t" && git config user.name "T"
  echo x > f && git add f && git commit -q -m init
  git branch -M main 2>/dev/null || true
  REMOTE="$(mktemp -d)"
  git init --bare -q "$REMOTE"
  git remote add origin "$REMOTE"
  git push -q origin main
  git fetch -q origin main
}

# Create a valid RC chain: reviewed commit, then .release-candidate commit,
# annotated tag, push main + tag to origin, fetch origin/main.
rc_tag() {
  local v="$1" p
  echo reviewed > "reviewed-$v" && git add "reviewed-$v" && git commit -q -m reviewed
  p="$(git rev-parse HEAD)"
  printf "version: %s\ncandidate_sha: %s\n" "$v" "$p" > .release-candidate
  git add .release-candidate && git commit -q -m "rc $v"
  git tag -a "$v" -m "Release $v"
  git push -q origin main --tags
  git fetch -q origin main
}

# Simulated recovery admission: run the post-tag candidate gate exactly as the
# recovery workflow does (explicit RELEASE_MAIN_REF), and only on success run
# the publisher stub, which drops a marker file. No real publishing happens.
recover() {
  local v="$1" marker="$2"
  if RELEASE_MAIN_REF=refs/remotes/origin/main bash "$POSTTAG" "$v" >/dev/null 2>&1; then
    : > "$marker"
    return 0
  fi
  return 1
}

echo "=== test_release_recovery_contract (hermetic) ==="

echo "C1: future-valid RC chain is recoverable"
setup; rc_tag v9.9.9
if recover v9.9.9 published.marker && [ -f published.marker ]; then
  pass "gate admits valid future RC chain and publisher stub runs"
else
  fail "gate admits valid future RC chain and publisher stub runs"
fi

echo "C2: v0.240.0 without RC chain fails closed"
setup
echo y > g && git add g && git commit -q -m "pre-contract release"
git tag -a v0.240.0 -m "Release v0.240.0"
git push -q origin main --tags && git fetch -q origin main
if recover v0.240.0 published.marker; then
  fail "gate rejects v0.240.0 lacking candidate provenance"
elif [ -f published.marker ]; then
  fail "publisher stub must not run after gate failure (v0.240.0)"
else
  pass "gate rejects v0.240.0 lacking candidate provenance; no publish"
fi

echo "C3: v0.460.0 with broken RC chain fails closed"
setup
echo y > g && git add g && git commit -q -m reviewed
printf "version: v0.460.0\ncandidate_sha: 0000000000000000000000000000000000000000\n" > .release-candidate
git add .release-candidate && git commit -q -m "rc bad SHA"
git tag -a v0.460.0 -m "Release v0.460.0"
git push -q origin main --tags && git fetch -q origin main
if recover v0.460.0 published.marker; then
  fail "gate rejects v0.460.0 with broken candidate chain"
elif [ -f published.marker ]; then
  fail "publisher stub must not run after gate failure (v0.460.0)"
else
  pass "gate rejects v0.460.0 with broken candidate chain; no publish"
fi

echo "C4: lightweight future tag fails closed"
setup
echo y > g && git add g && git commit -q -m reviewed
p="$(git rev-parse HEAD)"
printf "version: v9.9.9\ncandidate_sha: %s\n" "$p" > .release-candidate
git add .release-candidate && git commit -q -m "rc v9.9.9"
git tag v9.9.9  # lightweight
git push -q origin main --tags && git fetch -q origin main
if recover v9.9.9 published.marker; then
  fail "gate rejects lightweight tag"
elif [ -f published.marker ]; then
  fail "publisher stub must not run after gate failure (lightweight)"
else
  pass "gate rejects lightweight tag; no publish"
fi

echo "C5: tag not on origin/main fails closed"
setup; rc_tag v9.9.9
git checkout -q -b divergent
echo z > h && git add h && git commit -q -m divergent
p="$(git rev-parse HEAD)"
printf "version: v8.8.8\ncandidate_sha: %s\n" "$p" > .release-candidate
git add .release-candidate && git commit -q -m "rc v8.8.8"
git tag -a v8.8.8 -m "Release v8.8.8"
if recover v8.8.8 published.marker; then
  fail "gate rejects tag not on origin/main"
elif [ -f published.marker ]; then
  fail "publisher stub must not run after gate failure (off-main)"
else
  pass "gate rejects tag not on origin/main; no publish"
fi

echo "W1: hygiene script wires the recovery provenance checker (non-comment line)"
if grep -Eq '^[^#]*verify_release_recover_workflow_provenance\.py' "$HYGIENE"; then
  pass "hygiene script invokes recovery provenance checker"
else
  fail "hygiene script invokes recovery provenance checker"
fi

echo "W2: checker invocation is load-bearing (mutating wiring test)"
# Build a fixture repo whose recovery workflow violates the provenance
# contract (publisher pinned to main instead of the verified SHA). The
# original hygiene script must FAIL on it; a mutated hygiene script with the
# recovery checker invocation removed must PASS — proving the invocation is
# what enforces the contract.
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
FIX="$(mktemp -d)"
mkdir -p "$FIX/scripts" "$FIX/.github/workflows"
cp "$SCRIPT_DIR/verify_release_workflow_hygiene.sh" \
   "$SCRIPT_DIR/verify_release_workflow_hygiene.py" \
   "$SCRIPT_DIR/verify_release_recover_workflow_provenance.py" \
   "$SCRIPT_DIR/test_verify_release_workflow_provenance.py" \
   "$FIX/scripts/"
cp "$REPO_ROOT/.github/workflows/release.yml" "$FIX/.github/workflows/"
sed 's|ref: ${{ needs.preflight.outputs.tag_target_sha }}|ref: main|' \
  "$REPO_ROOT/.github/workflows/release-recover.yml" \
  > "$FIX/.github/workflows/release-recover.yml"

if (cd "$FIX" && bash scripts/verify_release_workflow_hygiene.sh >/dev/null 2>&1); then
  fail "original hygiene script fails on contract-violating recovery workflow"
else
  pass "original hygiene script fails on contract-violating recovery workflow"
fi

grep -Ev '^[^#]*verify_release_recover_workflow_provenance\.py' \
  "$SCRIPT_DIR/verify_release_workflow_hygiene.sh" \
  > "$FIX/scripts/verify_release_workflow_hygiene.sh"
if (cd "$FIX" && bash scripts/verify_release_workflow_hygiene.sh >/dev/null 2>&1); then
  pass "mutated hygiene script without checker invocation misses the violation"
else
  fail "mutated hygiene script without checker invocation misses the violation"
fi
rm -rf "$FIX"

echo ""; echo "Results: $P passed, $F failed"
[ "$F" -eq 0 ] && echo "All tests passed." || exit 1
