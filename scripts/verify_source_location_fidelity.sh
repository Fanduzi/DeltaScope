#!/usr/bin/env bash
# input: DELTASCOPE_BIN (optional, defaults to bin/deltascope)
# output: validates source location fidelity across GitHub Actions, SARIF, GitLab Code Quality, and TiDB outputs
# pos: release contract gate for source location propagation in CI renderers

set -euo pipefail

DELTASCOPE_BIN="${DELTASCOPE_BIN:-bin/deltascope}"

fail() {
  printf '[source-location-smoke][FAIL] %s\n' "$*" >&2
  exit 1
}

log() {
  printf '[source-location-smoke] %s\n' "$*"
}

[[ -x "${DELTASCOPE_BIN}" ]] || fail "deltascope binary not found or not executable: ${DELTASCOPE_BIN}"

command -v python3 >/dev/null 2>&1 || fail "python3 is required for JSON validation"

DELTASCOPE_BIN_ABS="$(cd "$(dirname "${DELTASCOPE_BIN}")" && pwd)/$(basename "${DELTASCOPE_BIN}")"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir:-}"' EXIT

mkdir -p "${tmp_dir}/migrations"

# SQL fixture: "delete from users;" starts on line 9.
cat > "${tmp_dir}/migrations/location_fidelity.sql" <<'SQLEOF'
create table ok_users (
  id bigint unsigned not null auto_increment comment 'id',
  name varchar(32) not null default '' comment 'name',
  created_at datetime not null default current_timestamp comment 'created',
  updated_at datetime not null default current_timestamp on update current_timestamp comment 'updated',
  primary key (id)
) comment='ok users';

delete from users;
SQLEOF

sql_file="migrations/location_fidelity.sql"

log "SQL fixture: delete on line 9 in ${sql_file}"

# --- 1. GitHub Actions ---
log "checking github-actions format..."

(cd "${tmp_dir}" && "${DELTASCOPE_BIN_ABS}" audit \
  --dialect mysql \
  --file "${sql_file}" \
  --format github-actions \
  --fail-on none \
  > "${tmp_dir}/github-actions.txt")

ga_output="$(cat "${tmp_dir}/github-actions.txt")"

echo "${ga_output}" | grep -q "title=\[blocker\] dml.where.require" \
  || fail "github-actions: missing title=[blocker] dml.where.require"
echo "${ga_output}" | grep -q "file=${sql_file}" \
  || fail "github-actions: missing file=${sql_file}"
echo "${ga_output}" | grep -q "line=9" \
  || fail "github-actions: missing line=9"
echo "${ga_output}" | grep -q "col=1" \
  || fail "github-actions: missing col=1"
echo "${ga_output}" | grep "title=\[blocker\] dml.where.require" | grep -q "file=," \
  && fail "github-actions: found empty file= (file=,)"
echo "${ga_output}" | grep "title=\[blocker\] dml.where.require" | grep -q "line=2" \
  && fail "github-actions: found line=2 fallback instead of real line=9"

log "github-actions: OK"

# --- 2. SARIF ---
log "checking sarif format..."

(cd "${tmp_dir}" && "${DELTASCOPE_BIN_ABS}" audit \
  --dialect mysql \
  --file "${sql_file}" \
  --format sarif \
  --fail-on none \
  > "${tmp_dir}/sarif.json")

python3 - "${tmp_dir}/sarif.json" "${sql_file}" <<'PYEOF'
import json, sys

path, expected_uri = sys.argv[1], sys.argv[2]
with open(path) as f:
    doc = json.load(f)

runs = doc.get("runs", [])
if not runs:
    sys.exit("SARIF: no runs")

results = runs[0].get("results", [])
where = None
for r in results:
    if r.get("ruleId") == "dml.where.require":
        where = r
        break
if where is None:
    sys.exit("SARIF: dml.where.require result not found")

locs = where.get("locations", [])
if not locs:
    sys.exit("SARIF: no locations")

phys = locs[0].get("physicalLocation", {})
artifact = phys.get("artifactLocation", {})
if artifact.get("uri", "") != expected_uri:
    sys.exit(f"SARIF: artifactLocation.uri={artifact.get('uri')}, expected {expected_uri}")

region = phys.get("region", {})
if region.get("startLine") != 9:
    sys.exit(f"SARIF: startLine={region.get('startLine')}, expected 9")
if region.get("startColumn") != 1:
    sys.exit(f"SARIF: startColumn={region.get('startColumn')}, expected 1")

print("sarif: OK")
PYEOF

log "sarif: OK"

# --- 3. GitLab Code Quality ---
log "checking gitlab-codequality format..."

(cd "${tmp_dir}" && "${DELTASCOPE_BIN_ABS}" audit \
  --dialect mysql \
  --file "${sql_file}" \
  --format gitlab-codequality \
  --fail-on none \
  > "${tmp_dir}/gitlab-codequality.json")

python3 - "${tmp_dir}/gitlab-codequality.json" "${sql_file}" <<'PYEOF'
import json, sys

path, expected_path = sys.argv[1], sys.argv[2]
with open(path) as f:
    issues = json.load(f)

where = None
for issue in issues:
    if issue.get("check_name") == "dml.where.require":
        where = issue
        break
if where is None:
    sys.exit("gitlab-codequality: dml.where.require issue not found")

loc = where.get("location", {})
if loc.get("path", "") != expected_path:
    sys.exit(f"gitlab-codequality: location.path={loc.get('path')}, expected {expected_path}")

lines = loc.get("lines", {})
if lines.get("begin") != 9:
    sys.exit(f"gitlab-codequality: lines.begin={lines.get('begin')}, expected 9")

print("gitlab-codequality: OK")
PYEOF

log "gitlab-codequality: OK"

# --- 4. TiDB SARIF smoke ---
log "checking tidb sarif format..."

(cd "${tmp_dir}" && "${DELTASCOPE_BIN_ABS}" audit \
  --dialect tidb \
  --file "${sql_file}" \
  --format sarif \
  --fail-on none \
  > "${tmp_dir}/tidb-sarif.json")

python3 - "${tmp_dir}/tidb-sarif.json" "${sql_file}" <<'PYEOF'
import json, sys

path, expected_uri = sys.argv[1], sys.argv[2]
with open(path) as f:
    doc = json.load(f)

runs = doc.get("runs", [])
if not runs:
    sys.exit("TiDB SARIF: no runs")

results = runs[0].get("results", [])
where = None
for r in results:
    if r.get("ruleId") == "dml.where.require":
        where = r
        break
if where is None:
    sys.exit("TiDB SARIF: dml.where.require result not found")

locs = where.get("locations", [])
if not locs:
    sys.exit("TiDB SARIF: no locations")

phys = locs[0].get("physicalLocation", {})
artifact = phys.get("artifactLocation", {})
if artifact.get("uri", "") != expected_uri:
    sys.exit(f"TiDB SARIF: artifactLocation.uri={artifact.get('uri')}, expected {expected_uri}")

region = phys.get("region", {})
if region.get("startLine") != 9:
    sys.exit(f"TiDB SARIF: startLine={region.get('startLine')}, expected 9")

print("tidb sarif: OK")
PYEOF

log "tidb sarif: OK"

log "all checks passed"
