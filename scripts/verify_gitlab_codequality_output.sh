#!/usr/bin/env bash
# input: DELTASCOPE_BIN (optional, defaults to bin/deltascope)
# output: validates --format gitlab-codequality JSON output contract
# pos: release contract gate for GitLab Code Quality output surface

set -euo pipefail

DELTASCOPE_BIN="${DELTASCOPE_BIN:-bin/deltascope}"

fail() {
  printf '[gitlab-codequality-smoke][FAIL] %s\n' "$*" >&2
  exit 1
}

[[ -x "${DELTASCOPE_BIN}" ]] || fail "deltascope binary not found or not executable: ${DELTASCOPE_BIN}"

command -v python3 >/dev/null 2>&1 || fail "python3 is required for JSON validation"

DELTASCOPE_BIN_ABS="$(cd "$(dirname "${DELTASCOPE_BIN}")" && pwd)/$(basename "${DELTASCOPE_BIN}")"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir:-}"' EXIT

# --- Inline SQL smoke ---
"${DELTASCOPE_BIN}" audit \
  --sql "delete from users" \
  --dialect mysql \
  --format gitlab-codequality \
  --fail-on none \
  > "${tmp_dir}/inline-report.json"

python3 - "${tmp_dir}/inline-report.json" <<'PYEOF'
import json, sys, re

path = sys.argv[1]
with open(path) as f:
    data = json.load(f)

if not isinstance(data, list):
    sys.exit(f"top-level is not a JSON array, got {type(data).__name__}")
if len(data) == 0:
    sys.exit("JSON array is empty, expected at least one issue")

valid_severities = {"info", "minor", "major", "critical", "blocker"}

check_names = []
for i, issue in enumerate(data):
    prefix = f"issue[{i}]"
    for key in ("description", "check_name", "fingerprint", "severity"):
        if key not in issue:
            sys.exit(f"{prefix} missing required key: {key}")
        if not isinstance(issue[key], str) or not issue[key]:
            sys.exit(f"{prefix}.{key} must be a non-empty string")
    if issue["severity"] not in valid_severities:
        sys.exit(f"{prefix}.severity={issue['severity']} not in {valid_severities}")
    if not re.fullmatch(r'[0-9a-f]{64}', issue["fingerprint"]):
        sys.exit(f"{prefix}.fingerprint is not a 64-char hex string")
    loc = issue.get("location")
    if not isinstance(loc, dict):
        sys.exit(f"{prefix}.location must be an object")
    if not isinstance(loc.get("path"), str) or not loc["path"]:
        sys.exit(f"{prefix}.location.path must be a non-empty string")
    lines = loc.get("lines")
    if not isinstance(lines, dict):
        sys.exit(f"{prefix}.location.lines must be an object")
    begin = lines.get("begin")
    if not isinstance(begin, int) or begin < 1:
        sys.exit(f"{prefix}.location.lines.begin must be a positive integer")
    check_names.append(issue["check_name"])

if "dml.where.require" not in check_names:
    sys.exit(f"expected check_name 'dml.where.require', got: {check_names}")

# Inline SQL fallback path must be deltascope.sql
paths = [issue["location"]["path"] for issue in data]
if "deltascope.sql" not in paths:
    sys.exit(f"expected location.path 'deltascope.sql' for inline SQL, got paths: {paths}")

print("inline SQL smoke: OK")
PYEOF

# --- File path smoke ---
mkdir -p "${tmp_dir}/migrations"
printf 'delete from users;\n' > "${tmp_dir}/migrations/gitlab.sql"
(cd "${tmp_dir}" && "${DELTASCOPE_BIN_ABS}" audit \
  --file migrations/gitlab.sql \
  --dialect mysql \
  --format gitlab-codequality \
  --fail-on none \
  > "${tmp_dir}/file-report.json")

python3 - "${tmp_dir}/file-report.json" <<'PYEOF'
import json, sys

path = sys.argv[1]
with open(path) as f:
    data = json.load(f)

if not isinstance(data, list) or len(data) == 0:
    sys.exit("file report: expected non-empty JSON array")

paths = [issue["location"]["path"] for issue in data]
if "migrations/gitlab.sql" not in paths:
    sys.exit(f"expected location.path 'migrations/gitlab.sql', got: {paths}")

print("file path smoke: OK")
PYEOF

printf '[gitlab-codequality-smoke] all checks passed\n'
