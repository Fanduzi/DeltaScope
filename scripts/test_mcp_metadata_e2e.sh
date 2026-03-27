#!/usr/bin/env bash
# input: docker compose MySQL fixtures plus tagged MCP end-to-end Go tests
# output: repeatable metadata-aware MCP smoke validation against a real MySQL target
# pos: shell-based Docker harness for slower MCP metadata e2e verification
# note: if this file changes, update this header and module README.md.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/docker/cli-e2e-compose.yaml"
MYSQL_CONTAINER="deltascope-cli-e2e-mysql"
TIDB_CONTAINER="deltascope-cli-e2e-tidb"
CLIENT_CONTAINER="deltascope-cli-e2e-mysql-client"

mode="${1:-all}"

log() {
  printf '[mcp-metadata-e2e] %s\n' "$*"
}

fail() {
  printf '[mcp-metadata-e2e][FAIL] %s\n' "$*" >&2
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
  log "running MCP metadata-aware MySQL e2e tests"
  (
    cd "${ROOT_DIR}"
    go test -tags=e2e -count=1 -run 'TestRunServesMetadataAwareAuditOverRealMySQL|TestRunServesMetadataAwareAuditOverConnectionRef' -v ./cmd/deltascope-mcp
  )
}

run_tidb_suite() {
  log "running MCP metadata-aware TiDB e2e tests"
  (
    cd "${ROOT_DIR}"
    go test -tags=e2e -count=1 -run 'TestRunServesMetadataAwareAuditOverRealTiDB|TestRunServesMetadataAwareAuditOverTiDBConnectionRef' -v ./cmd/deltascope-mcp
  )
}

main() {
  require_cmd docker
  require_cmd go

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
      start_tidb_stack
      run_tidb_suite
      ;;
    *)
      fail "usage: scripts/test_mcp_metadata_e2e.sh [mysql|tidb|all]"
      ;;
  esac
}

main "$@"
