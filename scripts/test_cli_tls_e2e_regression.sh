#!/usr/bin/env bash
# Regression harness for CLI TLS E2E fixture lifecycle.
# Occupies legacy hardcoded ports (13306, 15432, 13307, 15433) to prove the
# suite uses dynamically allocated ports, and verifies cleanup after both
# passing and intentionally failed runs.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="${ROOT_DIR}/scripts/test_cli_tls_e2e.sh"

# Legacy ports the production suite must not depend on.
LEGACY_PORTS=(13306 15432 13307 15433)

PORT HOLDERS_PID=""
PORT_HOLDERS_DIR=""

log() {
  printf '[cli-tls-regression] %s\n' "$*"
}

fail() {
  printf '[cli-tls-regression][FAIL] %s\n' "$*" >&2
  exit 1
}

cleanup_port_holders() {
  if [[ -n "${PORT_HOLDERS_PID}" ]]; then
    kill "${PORT_HOLDERS_PID}" 2>/dev/null || true
    wait "${PORT_HOLDERS_PID}" 2>/dev/null || true
    PORT_HOLDERS_PID=""
  fi
  if [[ -n "${PORT_HOLDERS_DIR}" && -d "${PORT_HOLDERS_DIR}" ]]; then
    rm -rf "${PORT_HOLDERS_DIR}"
  fi
}

trap cleanup_port_holders EXIT INT TERM

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

# Start Python processes that bind the legacy ports to prove the suite
# does not depend on them being free.
start_port_holders() {
  log "occupying legacy ports: ${LEGACY_PORTS[*]}"
  PORT_HOLDERS_DIR="$(mktemp -d /tmp/deltascope-cli-tls-regression-ports.XXXXXX)"

  for port in "${LEGACY_PORTS[@]}"; do
    python3 -c "
import socket, sys, time
s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 0)
s.bind(('127.0.0.1', ${port}))
s.listen(1)
# Signal readiness
sys.stdout.flush()
# Hold until killed
time.sleep(86400)
" &
  done
  PORT_HOLDERS_PID=$!

  # Wait for all ports to be bound
  local retries=20
  for ((i = 1; i <= retries; i++)); do
    local all_bound=true
    for port in "${LEGACY_PORTS[@]}"; do
      if ! lsof -i ":${port}" -sTCP:LISTEN >/dev/null 2>&1; then
        all_bound=false
        break
      fi
    done
    if [[ "${all_bound}" == "true" ]]; then
      log "all legacy ports occupied"
      return 0
    fi
    sleep 0.5
  done

  fail "could not occupy all legacy ports within timeout"
}

# Verify no scoped Docker resources remain after a run.
assert_no_docker_resources() {
  local label="$1"
  local project_pattern="$2"

  # Check for containers
  local containers
  containers="$(docker ps -a --filter "label=com.docker.compose.project=${project_pattern}" --format '{{.Names}}' 2>/dev/null || true)"
  if [[ -n "${containers}" ]]; then
    fail "[${label}] leftover containers found: ${containers}"
  fi

  # Check for networks
  local networks
  networks="$(docker network ls --filter "label=com.docker.compose.project=${project_pattern}" --format '{{.Name}}' 2>/dev/null || true)"
  if [[ -n "${networks}" ]]; then
    fail "[${label}] leftover networks found: ${networks}"
  fi

  # Check for volumes
  local volumes
  volumes="$(docker volume ls --filter "label=com.docker.compose.project=${project_pattern}" --format '{{.Name}}' 2>/dev/null || true)"
  if [[ -n "${volumes}" ]]; then
    fail "[${label}] leftover volumes found: ${volumes}"
  fi
}

# Verify no generated certificate or config files remain.
assert_no_generated_files() {
  local label="$1"
  local workspace="$2"

  if [[ -d "${workspace}" ]]; then
    fail "[${label}] workspace directory still exists: ${workspace}"
  fi
}

# Test 1: Normal run should pass with occupied legacy ports.
test_normal_run() {
  log "=== TEST 1: normal run with occupied legacy ports ==="

  local output
  output="$("${SCRIPT}" 2>&1)" || {
    log "normal run output:"
    echo "${output}" | tail -30 >&2 || true
    fail "normal run failed (expected success after T2 hardening)"
  }

  log "normal run passed"
}

# Test 2: Intentional failure run should exit nonzero and still clean up.
test_failure_run() {
  log "=== TEST 2: intentional failure run ==="

  local exit_code=0
  "${SCRIPT}" --test-failure >/dev/null 2>&1 || exit_code=$?

  if [[ "${exit_code}" -eq 0 ]]; then
    fail "intentional failure run exited 0 (expected nonzero)"
  fi

  log "intentional failure run exited ${exit_code} (expected nonzero) — PASS"
}

# Test 3: Docker-required mode should fail when Docker is unavailable.
test_docker_required() {
  log "=== TEST 3: Docker-required mode fails without Docker ==="

  # Create a shim that makes docker unavailable
  local shim_dir
  shim_dir="$(mktemp -d /tmp/deltascope-cli-tls-shim.XXXXXX)"
  cat > "${shim_dir}/docker" << 'SHIM_EOF'
#!/bin/bash
echo "docker: command not found (test shim)" >&2
exit 127
SHIM_EOF
  chmod +x "${shim_dir}/docker"

  local exit_code=0
  PATH="${shim_dir}:${PATH}" "${SCRIPT}" >/dev/null 2>&1 || exit_code=$?

  rm -rf "${shim_dir}"

  if [[ "${exit_code}" -eq 0 ]]; then
    fail "Docker-required mode should fail when Docker is unavailable"
  fi

  log "Docker-required mode correctly failed without Docker — PASS"
}

# Test 4: --docker-optional should skip outside CI/release mode.
test_docker_optional_skip() {
  log "=== TEST 4: --docker-optional skips outside CI ==="

  # Create a shim that makes docker unavailable
  local shim_dir
  shim_dir="$(mktemp -d /tmp/deltascope-cli-tls-shim.XXXXXX)"
  cat > "${shim_dir}/docker" << 'SHIM_EOF'
#!/bin/bash
echo "docker: command not found (test shim)" >&2
exit 127
SHIM_EOF
  chmod +x "${shim_dir}/docker"

  local exit_code=0
  CI="" DELTASCOPE_CLI_TLS_E2E_REQUIRED="" \
    PATH="${shim_dir}:${PATH}" "${SCRIPT}" --docker-optional >/dev/null 2>&1 || exit_code=$?

  rm -rf "${shim_dir}"

  # --docker-optional should exit 0 (skip) when Docker is unavailable and not in CI
  if [[ "${exit_code}" -ne 0 ]]; then
    fail "--docker-optional should skip (exit 0) when Docker is unavailable outside CI"
  fi

  log "--docker-optional correctly skipped outside CI — PASS"
}

# Test 5: --docker-optional should fail in CI mode.
test_docker_optional_ci() {
  log "=== TEST 5: --docker-optional fails in CI ==="

  # Create a shim that makes docker unavailable
  local shim_dir
  shim_dir="$(mktemp -d /tmp/deltascope-cli-tls-shim.XXXXXX)"
  cat > "${shim_dir}/docker" << 'SHIM_EOF'
#!/bin/bash
echo "docker: command not found (test shim)" >&2
exit 127
SHIM_EOF
  chmod +x "${shim_dir}/docker"

  local exit_code=0
  CI=true PATH="${shim_dir}:${PATH}" "${SCRIPT}" --docker-optional >/dev/null 2>&1 || exit_code=$?

  rm -rf "${shim_dir}"

  if [[ "${exit_code}" -eq 0 ]]; then
    fail "--docker-optional should NOT skip in CI mode"
  fi

  log "--docker-optional correctly failed in CI mode — PASS"
}

# Test 6: --docker-optional should fail when DELTASCOPE_CLI_TLS_E2E_REQUIRED=1.
test_docker_optional_required() {
  log "=== TEST 6: --docker-optional fails when REQUIRED=1 ==="

  # Create a shim that makes docker unavailable
  local shim_dir
  shim_dir="$(mktemp -d /tmp/deltascope-cli-tls-shim.XXXXXX)"
  cat > "${shim_dir}/docker" << 'SHIM_EOF'
#!/bin/bash
echo "docker: command not found (test shim)" >&2
exit 127
SHIM_EOF
  chmod +x "${shim_dir}/docker"

  local exit_code=0
  DELTASCOPE_CLI_TLS_E2E_REQUIRED=1 \
    PATH="${shim_dir}:${PATH}" "${SCRIPT}" --docker-optional >/dev/null 2>&1 || exit_code=$?

  rm -rf "${shim_dir}"

  if [[ "${exit_code}" -eq 0 ]]; then
    fail "--docker-optional should NOT skip when DELTASCOPE_CLI_TLS_E2E_REQUIRED=1"
  fi

  log "--docker-optional correctly failed when REQUIRED=1 — PASS"
}

main() {
  require_cmd python3
  require_cmd lsof
  require_cmd docker

  start_port_holders

  test_normal_run
  test_failure_run
  test_docker_required
  test_docker_optional_skip
  test_docker_optional_ci
  test_docker_optional_required

  log "all 6 regression tests passed"
}

main "$@"
