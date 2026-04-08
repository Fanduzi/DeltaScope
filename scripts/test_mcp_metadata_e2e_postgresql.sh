#!/usr/bin/env bash
# input: docker compose PostgreSQL service plus tagged MCP end-to-end Go tests
# output: repeatable metadata-aware MCP smoke validation against a real PostgreSQL target
# pos: shell-based Docker harness for slower MCP metadata PG e2e verification
# note: if this file changes, update this header and module README.md.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/docker/pg-e2e-compose.yaml"
PG_CONTAINER="deltascope-pg-e2e"

log() {
  printf '[mcp-metadata-e2e-pg] %s\n' "$*"
}

fail() {
  printf '[mcp-metadata-e2e-pg][FAIL] %s\n' "$*" >&2
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

start_pg_stack() {
  log "starting PostgreSQL fixtures"
  compose up -d
  wait_for_health "${PG_CONTAINER}"
}

run_pg_suite() {
  log "running MCP metadata-aware PostgreSQL e2e tests"
  (
    cd "${ROOT_DIR}"
    CGO_ENABLED=1 go test -tags='e2e,postgresql' -count=1 -run 'TestRunServesMetadataAwareAuditOverRealPostgreSQL' -v ./cmd/deltascope-mcp
  )
}

main() {
  require_cmd docker
  require_cmd go

  trap cleanup EXIT

  start_pg_stack
  run_pg_suite
}

main "$@"
