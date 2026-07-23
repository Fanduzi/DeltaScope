#!/usr/bin/env bash
# TLS E2E test for DeltaScope HTTP audit.
# Generates certs, starts TLS fixtures with server in compose network,
# runs Go E2E tests, and cleans up everything on exit.

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
  log "cleaning up"
  compose down -v --remove-orphans >/dev/null 2>&1 || true
  if [[ -n "${TLS_CERTS_DIR:-}" && -d "${TLS_CERTS_DIR}" ]]; then
    rm -rf "${TLS_CERTS_DIR}"
  fi
  if [[ -n "${TLS_RUNTIME_CONFIG:-}" && -f "${TLS_RUNTIME_CONFIG}" ]]; then
    rm -f "${TLS_RUNTIME_CONFIG}"
  fi
}

trap cleanup EXIT INT TERM

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

wait_for_health() {
  local container="$1"
  local retries="${2:-60}"
  local delay="${3:-2}"
  local status=""

  for ((i = 1; i <= retries; i++)); do
    local has_healthcheck
    has_healthcheck="$(docker inspect --format '{{if .Config.Healthcheck.Test}}yes{{else}}no{{end}}' "${container}" 2>/dev/null || true)"

    if [[ "${has_healthcheck}" == "yes" ]]; then
      status="$(docker inspect --format '{{.State.Health.Status}}' "${container}" 2>/dev/null || true)"
      if [[ "${status}" == "healthy" ]]; then
        return 0
      fi
    else
      status="$(docker inspect --format '{{.State.Status}}' "${container}" 2>/dev/null || true)"
      if [[ "${status}" == "running" ]]; then
        return 0
      fi
    fi
    sleep "${delay}"
  done

  fail "container ${container} did not become ready (last status: ${status:-unknown})"
}

generate_certs() {
  log "generating TLS certificates"
  TLS_CERTS_DIR="$(mktemp -d /tmp/deltascope-tls-e2e-certs.XXXXXX)"
  TLS_RUNTIME_CONFIG="${TLS_CERTS_DIR}/runtime-config.yaml"
  export TLS_CERTS_DIR TLS_RUNTIME_CONFIG
  "${SCRIPT_DIR}/generate-certs.sh" "${TLS_CERTS_DIR}" "${TLS_RUNTIME_CONFIG}"
}

start_tls_stack() {
  log "starting TLS fixtures"
  compose up -d --build
  wait_for_health "deltascope-tls-mysql"
  wait_for_health "deltascope-tls-postgresql"
  wait_for_health "deltascope-tls-mysql-untrusted"
  wait_for_health "deltascope-tls-postgresql-untrusted"
  wait_for_health "deltascope-tls-server"
}

run_tls_tests() {
  log "running TLS E2E tests"

  # The server is exposed on localhost at TLS_HTTP_PORT (default 18932)
  local server_addr="127.0.0.1:${TLS_HTTP_PORT:-18932}"

  # Verify server is reachable
  local retries=30
  for ((i = 1; i <= retries; i++)); do
    if curl -sf "http://${server_addr}/healthz" >/dev/null 2>&1; then
      break
    fi
    if [[ $i -eq $retries ]]; then
      fail "server did not become reachable at ${server_addr}"
    fi
    sleep 1
  done

  # Run Go E2E tests
  (
    cd "${ROOT_DIR}"
    TLS_E2E_SERVER_ADDR="${server_addr}" \
    go test -tags='e2e,tls,postgresql' -count=1 -run 'TestTLS' -v ./cmd/deltascope-server
  )
  local test_exit=$?

  # Show server logs on failure
  if [[ ${test_exit} -ne 0 ]]; then
    log "server logs:"
    compose logs deltascope-server 2>&1 | tail -30
    log "mysql logs:"
    compose logs mysql-tls 2>&1 | tail -10
    log "postgresql logs:"
    compose logs postgresql-tls 2>&1 | tail -10
  fi

  return "${test_exit}"
}

main() {
  local test_failure_mode=false
  for arg in "$@"; do
    case "${arg}" in
      --test-failure) test_failure_mode=true ;;
    esac
  done

  require_cmd docker
  require_cmd go
  require_cmd curl
  require_cmd openssl

  generate_certs
  start_tls_stack

  if [[ "${test_failure_mode}" == "true" ]]; then
    log "--test-failure flag set, simulating failure to verify cleanup"
    exit 1
  fi

  # Disable errexit so run_tls_tests return code is captured reliably.
  set +e
  run_tls_tests
  local test_exit=$?
  set -e

  if [[ ${test_exit} -eq 0 ]]; then
    log "all TLS E2E tests passed"
  fi

  # Cleanup is handled by the EXIT trap; exit with the test result.
  exit "${test_exit}"
}

main "$@"
