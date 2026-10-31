#!/usr/bin/env bash
# Committed-range L2 module-README synchronization check — same rule as
# /Users/fan/.agents/skills/check-three-level-doc/scripts/check_three_level_doc.sh
# applied to a committed diff range instead of the worktree.
# Usage: l2_committed_range_check.sh <range>  (e.g. abc123..def456)
set -euo pipefail

RANGE="${1:?usage: l2_committed_range_check.sh <git-range>}"
ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

is_source_file() {
  case "$1" in
    *.go|*.sh|*.py|*.js|*.ts|*.tsx|*.java|*.rs|*.c|*.cc|*.cpp|*.h|*.hpp)
      return 0 ;;
    *) return 1 ;;
  esac
}

mapfile -t changed < <(git diff --name-only --diff-filter=ACMR "$RANGE")
echo "range: $RANGE"
echo "changed files: ${#changed[@]}"
if [[ "${#changed[@]}" -eq 0 ]]; then
  echo "[three-level-doc] no changed files"
  exit 0
fi

declare -A changed_map impacted_modules
for f in "${changed[@]}"; do
  changed_map["$f"]=1
  [[ -f "$f" ]] || continue
  if is_source_file "$f"; then
    impacted_modules["$(dirname "$f")"]=1
  fi
done

status=0
for mod in "${!impacted_modules[@]}"; do
  module_doc="$mod/README.md"
  if [[ ! -f "$module_doc" ]]; then
    echo "[three-level-doc][L2] missing module README.md: $module_doc" >&2
    status=1
    continue
  fi
  if [[ -z "${changed_map[$module_doc]:-}" ]]; then
    echo "[three-level-doc][L2] module changed but README.md not updated: $module_doc" >&2
    status=1
  fi
done
if [[ "$status" -ne 0 ]]; then
  echo "[three-level-doc] FAILED" >&2
  exit "$status"
fi
echo "[three-level-doc] OK"
