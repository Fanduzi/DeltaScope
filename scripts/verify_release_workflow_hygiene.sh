#!/usr/bin/env bash
# input: none
# output: validation that release workflow Homebrew verification avoids noisy tolerated cleanup,
#         and that the provenance dependency DAG gates all mutation jobs.
# pos: release contract gate protecting verify-homebrew-cask-install from false GitHub Actions annotations
#      and enforcing the provenance dependency graph.
# note: if this file changes, update this header and scripts/README.md.

set -euo pipefail

WORKFLOW=".github/workflows/release.yml"

fail() {
  printf '[release-workflow-hygiene][FAIL] %s\n' "$*" >&2
  exit 1
}

require_line() {
  local needle="$1"
  local description="$2"
  grep -Fq "$needle" "$WORKFLOW" || fail "missing ${description}: ${needle}"
}

forbid_line() {
  local needle="$1"
  local description="$2"
  if grep -Fq "$needle" "$WORKFLOW"; then
    fail "forbidden ${description}: ${needle}"
  fi
}

main() {
  [[ -f "$WORKFLOW" ]] || fail "missing workflow file: ${WORKFLOW}"

  # Reject noisy tolerated cleanup patterns
  forbid_line '|| true' "tolerated failure fallback (|| true)"

  # Reject uppercase Homebrew tap token
  forbid_line 'Fanduzi/deltascope' "uppercase Homebrew tap token"

  # Require conditional cleanup probes
  require_line 'brew list --cask deltascope' "conditional cask cleanup probe"
  require_line 'grep -Fxq "fanduzi/deltascope"' "conditional tap cleanup probe"
  require_line 'brew untap fanduzi/deltascope' "lowercase conditional untap"
  require_line 'brew tap fanduzi/deltascope' "lowercase tap"
  require_line 'brew install --cask deltascope' "short cask install after explicit tap"

  # Run structural Homebrew trust contract checker
  python3 "$(dirname "$0")/verify_release_workflow_hygiene.py" "$(dirname "$0")/.."

  # Run provenance dependency DAG contract checker
  python3 "$(dirname "$0")/test_verify_release_workflow_provenance.py" "$(dirname "$0")/.."
}

main "$@"
