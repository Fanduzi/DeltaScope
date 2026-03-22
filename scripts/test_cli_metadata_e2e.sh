#!/usr/bin/env bash
# input: docker compose services, fixture SQL, and deltascope CLI invocations against local MySQL/TiDB targets
# output: repeatable metadata-aware CLI e2e execution with JSON assertion helpers and deterministic cleanup
# pos: shell-based end-to-end harness for live metadata CLI validation
# note: if this file changes, update this header and module README.md.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/docker/cli-e2e-compose.yaml"
MYSQL_CONTAINER="deltascope-cli-e2e-mysql"
TIDB_CONTAINER="deltascope-cli-e2e-tidb"
CLIENT_CONTAINER="deltascope-cli-e2e-mysql-client"

mode="${1:-all}"

log() {
  printf '[cli-metadata-e2e] %s\n' "$*"
}

fail() {
  printf '[cli-metadata-e2e][FAIL] %s\n' "$*" >&2
  exit 1
}

compose() {
  docker compose -f "${COMPOSE_FILE}" "$@"
}

cleanup() {
  compose down -v --remove-orphans >/dev/null 2>&1 || true
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

wait_for_health() {
  local container="$1"
  local retries="${2:-60}"
  local delay="${3:-2}"
  local status=""

  for ((i = 1; i <= retries; i++)); do
    status="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${container}" 2>/dev/null || true)"
    if [[ "${status}" == "healthy" || "${status}" == "running" ]]; then
      return 0
    fi
    sleep "${delay}"
  done

  fail "container ${container} did not become ready (last status: ${status:-unknown})"
}

seed_tidb() {
  log "seeding TiDB fixtures"
  docker exec -i "${CLIENT_CONTAINER}" sh -lc 'mysql -h tidb -P 4000 -uroot' < "${ROOT_DIR}/docker/tidb/init.sql"
}

run_cli_capture() {
  local stdout_file="$1"
  local stderr_file="$2"
  shift 2

  set +e
  (
    cd "${ROOT_DIR}"
    go run ./cmd/deltascope "$@"
  ) >"${stdout_file}" 2>"${stderr_file}"
  local exit_code=$?
  set -e
  return "${exit_code}"
}

assert_exit_code() {
  local actual="$1"
  local expected="$2"
  [[ "${actual}" == "${expected}" ]] || fail "expected exit code ${expected}, got ${actual}"
}

assert_stderr_contains() {
  local stderr_file="$1"
  local expected="$2"
  grep -Fq -- "${expected}" "${stderr_file}" || fail "expected stderr to contain ${expected}"
}

assert_json_field_eq() {
  local json_file="$1"
  local field_path="$2"
  local expected="$3"

  python3 - "$json_file" "$field_path" "$expected" <<'PY'
import json
import sys

path = sys.argv[2].split(".")
expected = sys.argv[3]
with open(sys.argv[1], "r", encoding="utf-8") as handle:
    data = json.load(handle)
value = data
for part in path:
    if isinstance(value, list):
        value = value[int(part)]
    else:
        value = value.get(part)
actual = "" if value is None else str(value)
if actual != expected:
    raise SystemExit(f"expected {sys.argv[2]}={expected}, got {actual}")
PY
}

assert_json_rule_present() {
  local json_file="$1"
  local rule_id="$2"

  python3 - "$json_file" "$rule_id" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    data = json.load(handle)
rule_id = sys.argv[2]
findings = list(data.get("global_findings", []))
for statement in data.get("statements", []):
    findings.extend(statement.get("findings", []))
if not any(item.get("rule_id") == rule_id for item in findings):
    raise SystemExit(f"expected rule_id {rule_id} in findings")
PY
}

start_mysql_stack() {
  log "starting MySQL fixtures"
  compose up -d mysql
  wait_for_health "${MYSQL_CONTAINER}"
}

start_tidb_stack() {
  log "starting TiDB fixtures"
  compose up -d tidb mysql-client
  wait_for_health "${TIDB_CONTAINER}" 90 2
  wait_for_health "${CLIENT_CONTAINER}" 30 1
  seed_tidb
}

run_mysql_suite() {
  log "MySQL suite is not implemented yet"
}

run_tidb_suite() {
  log "TiDB suite is not implemented yet"
}

main() {
  require_cmd docker
  require_cmd python3
  trap cleanup EXIT

  case "${mode}" in
    mysql)
      start_mysql_stack
      run_mysql_suite
      ;;
    tidb)
      start_tidb_stack
      run_tidb_suite
      ;;
    all)
      start_mysql_stack
      run_mysql_suite
      cleanup
      start_tidb_stack
      run_tidb_suite
      ;;
    *)
      fail "usage: scripts/test_cli_metadata_e2e.sh [mysql|tidb|all]"
      ;;
  esac
}

main "$@"
