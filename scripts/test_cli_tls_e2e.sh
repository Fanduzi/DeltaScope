#!/usr/bin/env bash
# TLS E2E tests for DeltaScope CLI audit and query-access.
# Generates certs, starts TLS database fixtures (no server), builds CLI,
# runs 12 test cases (MySQL 8.4 + PostgreSQL 17 x audit + query-access
# x trusted/untrusted/hostname-mismatch), and cleans up on exit.
#
# Uses Compose-assigned dynamic host ports to avoid collisions with other
# services or parallel test runs. Each run creates a unique Compose project
# and temporary workspace that are fully cleaned up on exit.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/docker/tls-e2e/docker-compose.yaml"
SCRIPT_DIR="${ROOT_DIR}/docker/tls-e2e"

# Unique identifiers for this run — prevents collisions with parallel runs.
RUN_ID="${DELTASCOPE_CLI_TLS_RUN_ID:-$$}"
PROJECT_NAME="cli-tls-e2e-${RUN_ID}"

# Workspace for all generated files (certs, config, override).
WORKSPACE_DIR=""

# Dynamically resolved host ports (set after compose up).
MYSQL_TLS_PORT=""
PG_TLS_PORT=""
MYSQL_TLS_UNTRUSTED_PORT=""
PG_TLS_UNTRUSTED_PORT=""

TLS_CERTS_DIR=""
TLS_OVERRIDE_FILE=""
TLS_RUNTIME_CONFIG=""
CLI_BIN=""
MODE="all"
DOCKER_OPTIONAL=false

log() {
  printf '[cli-tls-e2e] %s\n' "$*"
}

fail() {
  printf '[cli-tls-e2e][FAIL] %s\n' "$*" >&2
  exit 1
}

compose() {
  docker compose -p "${PROJECT_NAME}" -f "${COMPOSE_FILE}" -f "${TLS_OVERRIDE_FILE}" "$@"
}

cleanup() {
  local test_status=$?
  local cleanup_failed=false
  log "cleaning up (project: ${PROJECT_NAME})"

  # Tear down Compose resources.
  if [[ -n "${TLS_OVERRIDE_FILE}" && -f "${TLS_OVERRIDE_FILE}" ]]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi

  # Force-remove any scoped Docker resources that survived compose down.
  if command -v docker >/dev/null 2>&1; then
    local leftover_containers
    leftover_containers="$(docker ps -a --filter "label=com.docker.compose.project=${PROJECT_NAME}" --format '{{.Names}}' 2>/dev/null || true)"
    if [[ -n "${leftover_containers}" ]]; then
      log "force-removing leftover containers: ${leftover_containers}"
      docker rm -f ${leftover_containers} >/dev/null 2>&1 || true
      leftover_containers="$(docker ps -a --filter "label=com.docker.compose.project=${PROJECT_NAME}" --format '{{.Names}}' 2>/dev/null || true)"
      if [[ -n "${leftover_containers}" ]]; then
        log "ERROR: containers still remain after force-remove: ${leftover_containers}"
        cleanup_failed=true
      fi
    fi

    local leftover_networks
    leftover_networks="$(docker network ls --filter "label=com.docker.compose.project=${PROJECT_NAME}" --format '{{.Name}}' 2>/dev/null || true)"
    if [[ -n "${leftover_networks}" ]]; then
      log "force-removing leftover networks: ${leftover_networks}"
      docker network rm ${leftover_networks} >/dev/null 2>&1 || true
      leftover_networks="$(docker network ls --filter "label=com.docker.compose.project=${PROJECT_NAME}" --format '{{.Name}}' 2>/dev/null || true)"
      if [[ -n "${leftover_networks}" ]]; then
        log "ERROR: networks still remain after force-remove: ${leftover_networks}"
        cleanup_failed=true
      fi
    fi

    local leftover_volumes
    leftover_volumes="$(docker volume ls --filter "label=com.docker.compose.project=${PROJECT_NAME}" --format '{{.Name}}' 2>/dev/null || true)"
    if [[ -n "${leftover_volumes}" ]]; then
      log "force-removing leftover volumes: ${leftover_volumes}"
      docker volume rm ${leftover_volumes} >/dev/null 2>&1 || true
      leftover_volumes="$(docker volume ls --filter "label=com.docker.compose.project=${PROJECT_NAME}" --format '{{.Name}}' 2>/dev/null || true)"
      if [[ -n "${leftover_volumes}" ]]; then
        log "ERROR: volumes still remain after force-remove: ${leftover_volumes}"
        cleanup_failed=true
      fi
    fi
  fi

  # Remove generated workspace.
  if [[ -n "${WORKSPACE_DIR}" && -d "${WORKSPACE_DIR}" ]]; then
    rm -rf "${WORKSPACE_DIR}"
    if [[ -d "${WORKSPACE_DIR}" ]]; then
      log "ERROR: workspace directory still exists after cleanup: ${WORKSPACE_DIR}"
      cleanup_failed=true
    fi
  fi

  # Cleanup failures must fail the success path but never mask an original test failure.
  if [[ "${cleanup_failed}" == "true" && "${test_status}" -eq 0 ]]; then
    log "cleanup found residual resources — failing the run"
    exit 1
  fi

  exit "${test_status}"
}

trap cleanup EXIT INT TERM

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

# Preflight: verify Docker is available and functional.
docker_preflight() {
  log "checking Docker availability"
  require_cmd docker

  if ! docker info >/dev/null 2>&1; then
    fail "Docker daemon is not running or not accessible"
  fi

  if ! docker compose version >/dev/null 2>&1; then
    fail "docker compose v2 is not available"
  fi

  log "Docker preflight passed"
}

# Handle --docker-optional flag: skip only when Docker is unavailable
# and we are NOT in CI/release mode.
handle_docker_optional() {
  if [[ "${DOCKER_OPTIONAL}" != "true" ]]; then
    # Required mode — Docker must be available.
    docker_preflight
    return
  fi

  # Optional mode — check if we're in CI or required mode.
  if [[ "${CI:-}" == "true" || "${CI:-}" == "1" ]]; then
    fail "--docker-optional is not allowed in CI mode"
  fi
  if [[ "${DELTASCOPE_CLI_TLS_E2E_REQUIRED:-}" == "1" ]]; then
    fail "--docker-optional is not allowed when DELTASCOPE_CLI_TLS_E2E_REQUIRED=1"
  fi

  # Check if Docker is available.
  if ! command -v docker >/dev/null 2>&1 || ! docker info >/dev/null 2>&1; then
    log "Docker is not available, skipping (optional mode)"
    exit 0
  fi

  # Docker is available — continue normally.
  docker_preflight
}

wait_for_service_health() {
  local service="$1"
  local retries="${2:-60}"
  local delay="${3:-2}"
  local status=""

  for ((i = 1; i <= retries; i++)); do
    # Get container ID via compose, then inspect health.
    local container_id
    container_id="$(compose ps -q "${service}" 2>/dev/null || true)"
    if [[ -z "${container_id}" ]]; then
      sleep "${delay}"
      continue
    fi

    status="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${container_id}" 2>/dev/null || true)"
    if [[ "${status}" == "healthy" || "${status}" == "running" ]]; then
      return 0
    fi
    sleep "${delay}"
  done

  fail "service ${service} did not become ready (last status: ${status:-unknown})"
}

generate_certs() {
  log "generating TLS certificates"
  TLS_CERTS_DIR="${WORKSPACE_DIR}/certs"
  TLS_RUNTIME_CONFIG="${WORKSPACE_DIR}/runtime-config.yaml"
  mkdir -p "${TLS_CERTS_DIR}"
  echo "logging:" > "${TLS_RUNTIME_CONFIG}"
  export TLS_CERTS_DIR TLS_RUNTIME_CONFIG
  "${SCRIPT_DIR}/generate-certs.sh" "${TLS_CERTS_DIR}" "${TLS_RUNTIME_CONFIG}" >/dev/null
}

generate_port_override() {
  log "generating compose override (dynamic ports)"
  TLS_OVERRIDE_FILE="${WORKSPACE_DIR}/override.yaml"
  cat > "${TLS_OVERRIDE_FILE}" <<EOF
# Temporary override for CLI TLS E2E tests.
# Generated by test_cli_tls_e2e.sh — do not edit manually.
# Project: ${PROJECT_NAME}

services:
  mysql-tls:
    container_name: ${PROJECT_NAME}-mysql-tls
    ports:
      - "3306"

  postgresql-tls:
    container_name: ${PROJECT_NAME}-postgresql-tls
    ports:
      - "5432"

  mysql-tls-untrusted:
    container_name: ${PROJECT_NAME}-mysql-tls-untrusted
    ports:
      - "3306"

  postgresql-tls-untrusted:
    container_name: ${PROJECT_NAME}-postgresql-tls-untrusted
    ports:
      - "5432"
EOF
}

# Resolve dynamically assigned host ports from Compose.
resolve_ports() {
  log "resolving dynamic host ports"

  MYSQL_TLS_PORT="$(compose port mysql-tls 3306 | cut -d: -f2)"
  PG_TLS_PORT="$(compose port postgresql-tls 5432 | cut -d: -f2)"
  MYSQL_TLS_UNTRUSTED_PORT="$(compose port mysql-tls-untrusted 3306 | cut -d: -f2)"
  PG_TLS_UNTRUSTED_PORT="$(compose port postgresql-tls-untrusted 5432 | cut -d: -f2)"

  # Validate ports are numeric and non-empty.
  for port_var in MYSQL_TLS_PORT PG_TLS_PORT MYSQL_TLS_UNTRUSTED_PORT PG_TLS_UNTRUSTED_PORT; do
    local port_value="${!port_var}"
    if [[ -z "${port_value}" || ! "${port_value}" =~ ^[0-9]+$ ]]; then
      fail "failed to resolve ${port_var}: got '${port_value}'"
    fi
  done

  log "  mysql-tls:            ${MYSQL_TLS_PORT}"
  log "  postgresql-tls:       ${PG_TLS_PORT}"
  log "  mysql-tls-untrusted:  ${MYSQL_TLS_UNTRUSTED_PORT}"
  log "  postgresql-tls-untrusted: ${PG_TLS_UNTRUSTED_PORT}"

  # Machine-readable port line for regression harness parsing.
  printf 'CLI_TLS_E2E_PORTS mysql_tls=%s pg_tls=%s mysql_untrusted=%s pg_untrusted=%s\n' \
    "${MYSQL_TLS_PORT}" "${PG_TLS_PORT}" "${MYSQL_TLS_UNTRUSTED_PORT}" "${PG_TLS_UNTRUSTED_PORT}"
}

start_db_stack() {
  log "starting TLS database fixtures (project: ${PROJECT_NAME})"
  compose up -d mysql-tls postgresql-tls mysql-tls-untrusted postgresql-tls-untrusted

  wait_for_service_health "mysql-tls"
  wait_for_service_health "postgresql-tls"
  wait_for_service_health "mysql-tls-untrusted"
  wait_for_service_health "postgresql-tls-untrusted"

  resolve_ports
}

build_cli() {
  log "building CLI binary"
  CLI_BIN="${DELTASCOPE_CLI_BIN:-${ROOT_DIR}/bin/deltascope}"
  if [[ -n "${DELTASCOPE_CLI_BIN:-}" ]]; then
    log "using pre-built CLI: ${CLI_BIN}"
    return
  fi
  (
    cd "${ROOT_DIR}"
    CGO_ENABLED=1 go build -tags postgresql -o "${CLI_BIN}" ./cmd/deltascope
  )
}

run_cli_capture() {
  local stdout_file="$1"
  local stderr_file="$2"
  shift 2

  set +e
  (
    cd "${ROOT_DIR}"
    "${CLI_BIN}" "$@" >"${stdout_file}" 2>"${stderr_file}"
  )
  local exit_code=$?
  set -e
  return "${exit_code}"
}

assert_exit_code() {
  local actual="$1"
  local expected="$2"
  local label="${3:-}"
  if [[ "${actual}" != "${expected}" ]]; then
    if [[ -n "${label}" ]]; then
      fail "[${label}] expected exit code ${expected}, got ${actual}"
    fi
    fail "expected exit code ${expected}, got ${actual}"
  fi
}

assert_stderr_contains() {
  local stderr_file="$1"
  local pattern="$2"
  grep -qi -- "${pattern}" "${stderr_file}" || fail "expected stderr to contain '${pattern}'"
}

assert_stdout_contains() {
  local stdout_file="$1"
  local pattern="$2"
  grep -qi -- "${pattern}" "${stdout_file}" || fail "expected stdout to contain '${pattern}'"
}

assert_no_leak() {
  local stdout_file="$1"
  local stderr_file="$2"
  local label="$3"

  for token in \
    "root" \
    "mysql://root" \
    "postgresql://root" \
    "tls-cert.pem" \
    "tls-key.pem" \
    "ca-cert.pem" \
    "trusted-ca-cert.pem" \
    "untrusted-ca-cert.pem" \
    "BEGIN CERTIFICATE" \
    "BEGIN RSA PRIVATE KEY" \
    "driver:"; do
    if grep -qi -- "${token}" "${stdout_file}" 2>/dev/null; then
      fail "[${label}] stdout leaked sensitive token: ${token}"
    fi
    if grep -qi -- "${token}" "${stderr_file}" 2>/dev/null; then
      fail "[${label}] stderr leaked sensitive token: ${token}"
    fi
  done
}

# run_tls_case: execute one CLI TLS test case.
# Arguments:
#   $1  label
#   $2  subcommand (audit | query-access)
#   $3  dialect (mysql | postgresql)
#   $4  expected exit code
#   $5  host
#   $6  port
#   $7  CA cert file path (trusted or untrusted)
#   $8  SQL text
#   $9  extra args (space-separated, optional)
#   $10 stdout contains pattern (optional)
run_tls_case() {
  local label="$1"
  local subcmd="$2"
  local dialect="$3"
  local expected_exit="$4"
  local host="$5"
  local port="$6"
  local ca_file="$7"
  local sql="$8"
  local extra_args="${9:-}"
  local stdout_pattern="${10:-}"

  local stdout_file
  local stderr_file
  local exit_code

  stdout_file="$(mktemp)"
  stderr_file="$(mktemp)"

  log "TEST: ${label}"

  # shellcheck disable=SC2086
  if run_cli_capture "${stdout_file}" "${stderr_file}" \
    ${subcmd} \
    --sql "${sql}" \
    --dialect "${dialect}" \
    --host "${host}" \
    --port "${port}" \
    --user root \
    --password-env DELTASCOPE_CLI_TLS_PASSWORD \
    --tls-mode enabled \
    --tls-ca-file "${ca_file}" \
    ${extra_args}; then
    exit_code=0
  else
    exit_code=$?
  fi

  local result="PASS"
  if [[ "${exit_code}" != "${expected_exit}" ]]; then
    result="FAIL"
  fi

  log "  exit=${exit_code} expected=${expected_exit} [${result}]"
  if [[ "${result}" == "FAIL" ]]; then
    log "  stdout:"; cat "${stdout_file}" | head -20 >&2 || true
    log "  stderr:"; cat "${stderr_file}" | head -20 >&2 || true
  fi

  assert_exit_code "${exit_code}" "${expected_exit}" "${label}"

  # Failure cases must produce bounded error messages, not raw driver errors.
  if [[ "${expected_exit}" -ne 0 ]]; then
    assert_stderr_contains "${stderr_file}" "tls"
  fi

  # No sensitive data in output.
  assert_no_leak "${stdout_file}" "${stderr_file}" "${label}"

  if [[ -n "${stdout_pattern}" ]]; then
    assert_stdout_contains "${stdout_file}" "${stdout_pattern}"
  fi

  rm -f "${stdout_file}" "${stderr_file}"
}

run_mysql_audit_suite() {
  log "=== MySQL 8.4 audit TLS cases ==="

  local trusted_ca="${TLS_CERTS_DIR}/trusted/trusted-ca-cert.pem"
  local untrusted_ca="${TLS_CERTS_DIR}/untrusted/untrusted-ca-cert.pem"
  local port="${MYSQL_TLS_PORT}"
  local untrusted_port="${MYSQL_TLS_UNTRUSTED_PORT}"
  local audit_sql="ALTER TABLE app.users ADD COLUMN tls_e2e_col VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'tls e2e'"

  # Trusted CA — expect success (exit 0); --fail-on none so audit-policy findings don't affect exit code.
  run_tls_case \
    "mysql84-audit-trusted" \
    "audit" "mysql" 0 \
    "localhost" "${port}" "${trusted_ca}" \
    "${audit_sql}" \
    "--schema app --fail-on none" \
    "metadata-aware"

  # Untrusted CA — expect connection failure (exit 2; audit uses exitUser for connection failures)
  run_tls_case \
    "mysql84-audit-untrusted" \
    "audit" "mysql" 2 \
    "localhost" "${untrusted_port}" "${trusted_ca}" \
    "${audit_sql}" \
    "--schema app --fail-on none"

  # Hostname mismatch — cert has localhost SAN, connect via 127.0.0.1 (exit 2)
  run_tls_case \
    "mysql84-audit-hostname-mismatch" \
    "audit" "mysql" 2 \
    "127.0.0.1" "${port}" "${trusted_ca}" \
    "${audit_sql}" \
    "--schema app --fail-on none"
}

run_mysql_query_access_suite() {
  log "=== MySQL 8.4 query-access TLS cases ==="

  local trusted_ca="${TLS_CERTS_DIR}/trusted/trusted-ca-cert.pem"
  local untrusted_ca="${TLS_CERTS_DIR}/untrusted/untrusted-ca-cert.pem"
  local port="${MYSQL_TLS_PORT}"
  local untrusted_port="${MYSQL_TLS_UNTRUSTED_PORT}"
  local qa_sql="SELECT count(id) FROM app.users"

  # Trusted CA — expect success (exit 0)
  run_tls_case \
    "mysql84-query-access-trusted" \
    "query-access analyze" "mysql" 0 \
    "localhost" "${port}" "${trusted_ca}" \
    "${qa_sql}" \
    "--schema app"

  # Untrusted CA — expect usage error (exit 3; query-access uses exitQueryAccessUsageError)
  run_tls_case \
    "mysql84-query-access-untrusted" \
    "query-access analyze" "mysql" 3 \
    "localhost" "${untrusted_port}" "${trusted_ca}" \
    "${qa_sql}" \
    "--schema app"

  # Hostname mismatch (exit 3; query-access uses exitQueryAccessUsageError)
  run_tls_case \
    "mysql84-query-access-hostname-mismatch" \
    "query-access analyze" "mysql" 3 \
    "127.0.0.1" "${port}" "${trusted_ca}" \
    "${qa_sql}" \
    "--schema app"
}

run_pg_audit_suite() {
  log "=== PostgreSQL 17 audit TLS cases ==="

  local trusted_ca="${TLS_CERTS_DIR}/trusted/trusted-ca-cert.pem"
  local untrusted_ca="${TLS_CERTS_DIR}/untrusted/untrusted-ca-cert.pem"
  local port="${PG_TLS_PORT}"
  local untrusted_port="${PG_TLS_UNTRUSTED_PORT}"
  local audit_sql="ALTER TABLE app.users ADD COLUMN tls_e2e_col TEXT NOT NULL DEFAULT ''"

  # Trusted CA — expect success (exit 0); --fail-on none so audit-policy findings don't affect exit code.
  run_tls_case \
    "pg17-audit-trusted" \
    "audit" "postgresql" 0 \
    "localhost" "${port}" "${trusted_ca}" \
    "${audit_sql}" \
    "--schema app --database app --fail-on none" \
    "metadata-aware"

  # Untrusted CA — expect connection failure (exit 2; audit uses exitUser for connection failures)
  run_tls_case \
    "pg17-audit-untrusted" \
    "audit" "postgresql" 2 \
    "localhost" "${untrusted_port}" "${trusted_ca}" \
    "${audit_sql}" \
    "--schema app --database app --fail-on none"

  # Hostname mismatch — cert has localhost SAN, connect via 127.0.0.1 (exit 2)
  run_tls_case \
    "pg17-audit-hostname-mismatch" \
    "audit" "postgresql" 2 \
    "127.0.0.1" "${port}" "${trusted_ca}" \
    "${audit_sql}" \
    "--schema app --database app --fail-on none"
}

run_pg_query_access_suite() {
  log "=== PostgreSQL 17 query-access TLS cases ==="

  local trusted_ca="${TLS_CERTS_DIR}/trusted/trusted-ca-cert.pem"
  local untrusted_ca="${TLS_CERTS_DIR}/untrusted/untrusted-ca-cert.pem"
  local port="${PG_TLS_PORT}"
  local untrusted_port="${PG_TLS_UNTRUSTED_PORT}"
  local qa_sql="SELECT count(id) FROM app.users"

  # Trusted CA — expect success (exit 0)
  run_tls_case \
    "pg17-query-access-trusted" \
    "query-access analyze" "postgresql" 0 \
    "localhost" "${port}" "${trusted_ca}" \
    "${qa_sql}" \
    "--schema app --database app"

  # Untrusted CA — expect usage error (exit 3; query-access uses exitQueryAccessUsageError)
  run_tls_case \
    "pg17-query-access-untrusted" \
    "query-access analyze" "postgresql" 3 \
    "localhost" "${untrusted_port}" "${trusted_ca}" \
    "${qa_sql}" \
    "--schema app --database app"

  # Hostname mismatch (exit 3; query-access uses exitQueryAccessUsageError)
  run_tls_case \
    "pg17-query-access-hostname-mismatch" \
    "query-access analyze" "postgresql" 3 \
    "127.0.0.1" "${port}" "${trusted_ca}" \
    "${qa_sql}" \
    "--schema app --database app"
}

run_tls_tests() {
  log "running 12 CLI TLS E2E test cases"

  run_mysql_audit_suite
  run_mysql_query_access_suite
  run_pg_audit_suite
  run_pg_query_access_suite

  log "all 12 CLI TLS E2E tests passed"
}

main() {
  local test_failure_mode=false

  # Parse arguments.
  for arg in "$@"; do
    case "${arg}" in
      --test-failure) test_failure_mode=true ;;
      --docker-optional) DOCKER_OPTIONAL=true ;;
      all|mysql|postgresql) MODE="${arg}" ;;
      --*) fail "unknown flag: ${arg}" ;;
      *) fail "unknown argument: ${arg}" ;;
    esac
  done

  # Docker availability gate.
  handle_docker_optional

  # Create workspace for all generated files.
  WORKSPACE_DIR="$(mktemp -d /tmp/deltascope-cli-tls-e2e.XXXXXX)"
  export WORKSPACE_DIR
  log "workspace: ${WORKSPACE_DIR}"

  generate_certs
  generate_port_override
  start_db_stack
  build_cli

  export DELTASCOPE_CLI_TLS_PASSWORD="root"

  if [[ "${test_failure_mode}" == "true" ]]; then
    log "--test-failure flag set, simulating failure to verify cleanup"
    exit 1
  fi

  case "${MODE}" in
    all)
      run_tls_tests
      ;;
    mysql)
      run_mysql_audit_suite
      run_mysql_query_access_suite
      ;;
    postgresql)
      run_pg_audit_suite
      run_pg_query_access_suite
      ;;
    *)
      fail "usage: scripts/test_cli_tls_e2e.sh [--test-failure] [--docker-optional] [all|mysql|postgresql]"
      ;;
  esac
}

main "$@"
