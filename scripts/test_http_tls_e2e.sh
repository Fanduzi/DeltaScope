#!/usr/bin/env bash
# TLS E2E test for DeltaScope HTTP audit.
# Starts TLS-enabled MySQL and PostgreSQL fixtures, then runs Go E2E tests.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/docker/tls-e2e/docker-compose.yaml"
SCRIPT_DIR="${ROOT_DIR}/docker/tls-e2e"

log() {
  printf '[tls-e2e] %s\n' "$*"
}

fail() {
  printf '[tls-e2e][FAIL] %s\n' "$*" >&2
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

generate_certs() {
  log "generating TLS certificates"
  "${SCRIPT_DIR}/generate-certs.sh"
}

start_tls_stack() {
  log "starting TLS fixtures"
  compose up -d --build
  wait_for_health "deltascope-tls-mysql"
  wait_for_health "deltascope-tls-postgresql"
}

run_tls_tests() {
  log "running TLS E2E tests"

  local tmp_dir
  tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/deltascope-tls-e2e.XXXXXX")"

  # Build the server binary
  local server_bin="${tmp_dir}/deltascope-server"
  (
    cd "${ROOT_DIR}"
    CGO_ENABLED=1 go build -tags postgresql -o "${server_bin}" ./cmd/deltascope-server
  )

  # Start the server with the TLS runtime config
  local server_addr="127.0.0.1:18932"
  export MYSQL_PASSWORD="root"
  export PG_PASSWORD="root"

  "${server_bin}" \
    -listen "${server_addr}" \
    -runtime-config "${SCRIPT_DIR}/runtime-config.yaml" &
  local server_pid=$!

  # Wait for server to be ready
  local retries=30
  for ((i = 1; i <= retries; i++)); do
    if curl -sf "http://${server_addr}/healthz" >/dev/null 2>&1; then
      break
    fi
    if [[ $i -eq $retries ]]; then
      kill "${server_pid}" 2>/dev/null || true
      fail "server did not become ready"
    fi
    sleep 1
  done

  # Run Go E2E tests
  (
    cd "${ROOT_DIR}"
    TLS_E2E_SERVER_ADDR="${server_addr}" \
    TLS_E2E_TRUSTED_CA="${SCRIPT_DIR}/certs/trusted/trusted-ca-cert.pem" \
    TLS_E2E_UNTRUSTED_CA="${SCRIPT_DIR}/certs/untrusted/untrusted-ca-cert.pem" \
    go test -tags='e2e,tls' -count=1 -run 'TestTLS' -v ./cmd/deltascope-server
  )
  local test_exit=$?

  # Cleanup
  kill "${server_pid}" 2>/dev/null || true
  rm -rf "${tmp_dir}"

  return "${test_exit}"
}

main() {
  require_cmd docker
  require_cmd go
  require_cmd curl
  require_cmd openssl
  trap cleanup EXIT

  generate_certs
  start_tls_stack
  run_tls_tests

  log "all TLS E2E tests passed"
}

main "$@"
