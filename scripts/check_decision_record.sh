#!/usr/bin/env bash
# input: git diff range (default: main...HEAD), or staged/working-tree changes
# output: exit 0 if decision record discipline is satisfied, exit 1 if a record may be required
# pos: heuristic gate that checks whether changed paths + diff keywords suggest a decision record is needed

set -euo pipefail

# --- Override ---
if [[ "${DECISION_RECORD_NOT_REQUIRED:-0}" == "1" ]]; then
  echo "decision-record-gate: OVERRIDE set (DECISION_RECORD_NOT_REQUIRED=1)"
  echo "  The final task report MUST explain why no decision record is needed."
  exit 0
fi

# --- Determine diff source ---
REF="${1:-}"

if [[ -n "$REF" ]]; then
  # Explicit ref range, e.g. main...HEAD
  DIFF_FILES=$(git diff --name-only "$REF" 2>/dev/null) || {
    echo "decision-record-gate: cannot resolve ref range '$REF'" >&2
    exit 1
  }
  DIFF_TEXT=$(git diff "$REF" -- 2>/dev/null) || true
elif git diff --cached --quiet 2>/dev/null; then
  if git diff --quiet 2>/dev/null; then
    # No staged and no unstaged changes — fall back to main...HEAD
    DIFF_FILES=$(git diff --name-only main...HEAD 2>/dev/null) || true
    DIFF_TEXT=$(git diff main...HEAD -- 2>/dev/null) || true
  else
    # Unstaged changes only
    DIFF_FILES=$(git diff --name-only)
    DIFF_TEXT=$(git diff --)
  fi
else
  # Staged changes
  DIFF_FILES=$(git diff --cached --name-only)
  DIFF_TEXT=$(git diff --cached --)
fi

if [[ -z "$DIFF_FILES" ]]; then
  echo "decision-record-gate: no changes detected, PASS"
  exit 0
fi

# --- Check: already has docs/decisions/*.md? ---
if echo "$DIFF_FILES" | grep -qE '^docs/decisions/.*\.md$'; then
  echo "decision-record-gate: docs/decisions/*.md present in diff, PASS"
  exit 0
fi

# --- Trigger paths ---
TRIGGER_PATHS=(
  '^internal/domain/rule/'
  '^internal/infrastructure/parser/'
  '^internal/application/audit/'
  '^internal/domain/spec/'
  '^pkg/deltascope/'
  '^internal/interfaces/cli/'
  '^internal/interfaces/http/'
  '^internal/interfaces/mcp/'
  '^testdata/sql-corpus/'
  '^docs/reference/'
  '^docs/releases/'
  '^CHANGELOG\.md$'
  '^README\.md$'
  '^README_ZH\.md$'
)

PATH_HIT=false
for pattern in "${TRIGGER_PATHS[@]}"; do
  if echo "$DIFF_FILES" | grep -qE "$pattern"; then
    PATH_HIT=true
    break
  fi
done

if [[ "$PATH_HIT" != "true" ]]; then
  echo "decision-record-gate: no trigger paths in diff, PASS"
  exit 0
fi

# --- Trigger keywords ---
TRIGGER_KEYWORDS=(
  'defer'
  'deferred'
  'unsupported'
  'boundary'
  'payload'
  'no-leak'
  'leak'
  'public contract'
  'contract'
  'metadata'
  'privacy'
  'CLI'
  'HTTP'
  'MCP'
  'SDK'
  'finding_covered'
  'normalized_silent'
  'parser_error'
  'release notes'
  'non-goal'
  'not full'
  'full PostgreSQL'
)

KEYWORD_HIT=false
MATCHED_KEYWORD=""
for kw in "${TRIGGER_KEYWORDS[@]}"; do
  if echo "$DIFF_TEXT" | grep -qi -- "$kw"; then
    KEYWORD_HIT=true
    MATCHED_KEYWORD="$kw"
    break
  fi
done

if [[ "$KEYWORD_HIT" != "true" ]]; then
  echo "decision-record-gate: trigger paths hit but no trigger keywords in diff, PASS"
  exit 0
fi

# --- Fail: path + keyword triggered, but no decision record ---
echo "decision-record-gate: FAIL" >&2
echo "  Trigger paths and keyword '$MATCHED_KEYWORD' found in diff," >&2
echo "  but no docs/decisions/*.md is included." >&2
echo "" >&2
echo "  How to fix:" >&2
echo "    1. Add or update a docs/decisions/*.md in this change." >&2
echo "    2. If truly unnecessary, set DECISION_RECORD_NOT_REQUIRED=1 and" >&2
echo "       explain why in the final task report." >&2
exit 1
