#!/usr/bin/env bash
# input: none — invoked via `make test-e2e-cli-tls-regression`
# output: exit 0 if the CLI TLS E2E fixture lifecycle regression passes; exit
#         nonzero with per-port ownership diagnostics on any failure
# pos: legacy ports may already be occupied by external listeners ([external],
#      reused as-is, owner never touched); the harness owns only the holders it
#      starts ([owned]) and releases exactly those on every exit path
# note: test-lifecycle ownership safety fix; no product behavior change, no new
#      decision record required (see scripts/README.md for lifecycle contract)
#
# Regression harness for CLI TLS E2E fixture lifecycle.
# Occupies legacy hardcoded ports (13306, 15432, 13307, 15433) to prove the
# suite uses dynamically allocated ports, and verifies cleanup after both
# passing and intentionally failed runs.
#
# Legacy ports are split by ownership at startup:
#   - [external] PREEXISTING_PORTS: already bound by an external listener when
#     the harness starts (e.g. another project's long-running container). The
#     harness records them, starts no holder, and never kills, stops, restarts
#     or cleans up the external owner.
#   - [owned] OWNED_PORTS: free at start; the harness starts Python holders and
#     must release them in every exit path (success, failure, INT, TERM).

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="${ROOT_DIR}/scripts/test_cli_tls_e2e.sh"

# Legacy ports the production suite must not depend on.
LEGACY_PORTS=(13306 15432 13307 15433)

PREEXISTING_PORTS=()
OWNED_PORTS=()
PORT_HOLDER_PIDS=()
PORT_HOLDERS_DIR=""
DYNAMIC_PORTS=()

log() {
  printf '[cli-tls-regression] %s\n' "$*"
}

fail() {
  printf '[cli-tls-regression][FAIL] %s\n' "$*" >&2
  exit 1
}

port_listening() {
  lsof -i ":${1}" -sTCP:LISTEN >/dev/null 2>&1
}

# Assert every legacy port is currently listening, tagging each failure with
# its ownership ([external]/[owned]). Used after holders start and again before
# each run so an external listener vanishing mid-run cannot hollow out the
# regression and produce a false pass.
assert_legacy_ports_occupied() {
  local missing=0
  for port in "${LEGACY_PORTS[@]}"; do
    if ! port_listening "${port}"; then
      local owner="[owned]"
      # bash 3.2 set -u: guard empty-array expansion before membership scan.
      if (( ${#PREEXISTING_PORTS[@]} > 0 )); then
        for p in "${PREEXISTING_PORTS[@]}"; do
          [[ "${p}" == "${port}" ]] && { owner="[external]"; break; }
        done
      fi
      printf '[cli-tls-regression][FAIL] %s port %s is no longer listening\n' "${owner}" "${port}" >&2
      missing=1
    fi
  done
  if [[ "${missing}" -eq 1 ]]; then
    fail "legacy port coverage lost — external listeners must keep running, harness holders are released by cleanup"
  fi
}

cleanup_port_holders() {
  # Only harness-owned holders are killed — never external listeners.
  # bash 3.2 set -u: guard empty-array expansion before the kill loop.
  if (( ${#PORT_HOLDER_PIDS[@]} > 0 )); then
    for pid in "${PORT_HOLDER_PIDS[@]}"; do
      kill "${pid}" 2>/dev/null || true
      wait "${pid}" 2>/dev/null || true
    done
  fi
  PORT_HOLDER_PIDS=()
  if [[ -n "${PORT_HOLDERS_DIR}" && -d "${PORT_HOLDERS_DIR}" ]]; then
    rm -rf "${PORT_HOLDERS_DIR}"
  fi
  verify_owned_ports_released
  verify_preexisting_ports_listening
}

verify_owned_ports_released() {
  local retries=10
  for ((i = 1; i <= retries; i++)); do
    local any_listening=false
    # bash 3.2 set -u: guard empty-array expansion when all ports are [external].
    if (( ${#OWNED_PORTS[@]} > 0 )); then
      for port in "${OWNED_PORTS[@]}"; do
        if port_listening "${port}"; then
          any_listening=true
          break
        fi
      done
    fi
    if [[ "${any_listening}" == "false" ]]; then
      log "[owned] harness holders released: ${OWNED_PORTS[*]:-none}"
      return 0
    fi
    sleep 0.3
  done
  printf '[cli-tls-regression][FAIL] [owned] ports still occupied after cleanup: %s\n' "${OWNED_PORTS[*]}" >&2
  exit 1
}

verify_preexisting_ports_listening() {
  local missing=0
  # bash 3.2 set -u: guard empty-array expansion when no port is [external].
  if (( ${#PREEXISTING_PORTS[@]} > 0 )); then
    for port in "${PREEXISTING_PORTS[@]}"; do
      if ! port_listening "${port}"; then
        printf '[cli-tls-regression][FAIL] [external] port %s is no longer listening — the external owner must have released it; the harness never modified it\n' "${port}" >&2
        missing=1
      fi
    done
  fi
  if [[ "${missing}" -eq 1 ]]; then
    fail "[external] pre-existing ports were not preserved"
  fi
  if [[ "${#PREEXISTING_PORTS[@]}" -gt 0 ]]; then
    log "[external] pre-existing ports preserved: ${PREEXISTING_PORTS[*]}"
  fi
}

# EXIT always cleans up; INT/TERM must terminate instead of resuming the
# regression with released holders (signal-specific handlers re-raise via exit).
trap cleanup_port_holders EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

# Start Python holders only on legacy ports that are free at start. Ports
# already occupied by external listeners are recorded as [external] and reused
# as-is. Afterwards every legacy port must be listening, proving the production
# suite still has to use dynamic ports.
start_port_holders() {
  PORT_HOLDERS_DIR="$(mktemp -d /tmp/deltascope-cli-tls-regression-ports.XXXXXX)"

  PREEXISTING_PORTS=()
  OWNED_PORTS=()
  PORT_HOLDER_PIDS=()

  for port in "${LEGACY_PORTS[@]}"; do
    if port_listening "${port}"; then
      PREEXISTING_PORTS+=("${port}")
      log "[external] port ${port} already occupied by an external listener — reusing as-is (owner untouched)"
    else
      OWNED_PORTS+=("${port}")
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
      PORT_HOLDER_PIDS+=($!)
    fi
  done

  # Wait for all owned holders to bind.
  local retries=20
  for ((i = 1; i <= retries; i++)); do
    local all_bound=true
    # bash 3.2 set -u: guard empty-array expansion when all ports are [external].
    if (( ${#OWNED_PORTS[@]} > 0 )); then
      for port in "${OWNED_PORTS[@]}"; do
        if ! port_listening "${port}"; then
          all_bound=false
          break
        fi
      done
    fi
    if [[ "${all_bound}" == "true" ]]; then
      break
    fi
    sleep 0.5
  done

  # Confirm every legacy port is now listening ([external] + [owned]).
  assert_legacy_ports_occupied
  log "legacy ports covered: [external] ${PREEXISTING_PORTS[*]:-none} / [owned] ${OWNED_PORTS[*]:-none}"
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

# Extract workspace path from test output.
extract_workspace() {
  local output="$1"
  echo "${output}" | grep -o '/tmp/deltascope-cli-tls-e2e\.[^ ]*' | head -1
}

# Parse the machine-readable dynamic port line from test output.
# Sets DYNAMIC_PORTS array. Fails closed if the line is missing or malformed.
extract_dynamic_ports() {
  local label="$1"
  local output="$2"

  local port_line
  port_line="$(echo "${output}" | grep '^CLI_TLS_E2E_PORTS ' | head -1)"
  if [[ -z "${port_line}" ]]; then
    fail "[${label}] CLI_TLS_E2E_PORTS line not found in output — cannot verify dynamic port release"
  fi

  # Parse key=value pairs.
  local mysql_tls="" pg_tls="" mysql_untrusted="" pg_untrusted=""
  for token in ${port_line#CLI_TLS_E2E_PORTS }; do
    case "${token}" in
      mysql_tls=*) mysql_tls="${token#mysql_tls=}" ;;
      pg_tls=*) pg_tls="${token#pg_tls=}" ;;
      mysql_untrusted=*) mysql_untrusted="${token#mysql_untrusted=}" ;;
      pg_untrusted=*) pg_untrusted="${token#pg_untrusted=}" ;;
    esac
  done

  # Validate all four are present and numeric.
  local ports=("${mysql_tls}" "${pg_tls}" "${mysql_untrusted}" "${pg_untrusted}")
  for p in "${ports[@]}"; do
    if [[ -z "${p}" || ! "${p}" =~ ^[0-9]+$ ]]; then
      fail "[${label}] invalid dynamic port value: '${p}' (line: ${port_line})"
    fi
  done

  # Validate uniqueness.
  local unique_count
  unique_count="$(printf '%s\n' "${ports[@]}" | sort -u | wc -l | tr -d ' ')"
  if [[ "${unique_count}" -ne 4 ]]; then
    fail "[${label}] dynamic ports are not all unique: ${ports[*]}"
  fi

  DYNAMIC_PORTS=("${ports[@]}")
}

# Verify no processes are listening on the dynamic ports from a completed run.
assert_no_port_listeners() {
  local label="$1"
  local output="$2"

  extract_dynamic_ports "${label}" "${output}"

  for port in "${DYNAMIC_PORTS[@]}"; do
    if lsof -i ":${port}" -sTCP:LISTEN >/dev/null 2>&1; then
      fail "[${label}] dynamic port ${port} still has a listener after cleanup"
    fi
  done

  log "[${label}] dynamic ports released: ${DYNAMIC_PORTS[*]}"
}

# Test 1: Normal run should pass with occupied legacy ports.
test_normal_run() {
  log "=== TEST 1: normal run with occupied legacy ports ==="

  assert_legacy_ports_occupied

  local run_id="regression-normal-$$"
  local project="cli-tls-e2e-${run_id}"
  local output
  output="$(DELTASCOPE_CLI_TLS_RUN_ID="${run_id}" "${SCRIPT}" 2>&1)" || {
    log "normal run output:"
    echo "${output}" | tail -30 >&2 || true
    fail "normal run failed (expected success after T2 hardening)"
  }

  local workspace
  workspace="$(extract_workspace "${output}")"

  assert_no_docker_resources "normal-run" "${project}"
  if [[ -n "${workspace}" ]]; then
    assert_no_generated_files "normal-run" "${workspace}"
  fi
  assert_no_port_listeners "normal-run" "${output}"

  log "normal run passed — cleanup verified"
}

# Test 2: Intentional failure run should exit nonzero and still clean up.
test_failure_run() {
  log "=== TEST 2: intentional failure run ==="

  assert_legacy_ports_occupied

  local run_id="regression-fail-$$"
  local project="cli-tls-e2e-${run_id}"
  local exit_code=0
  local output
  output="$(DELTASCOPE_CLI_TLS_RUN_ID="${run_id}" "${SCRIPT}" --test-failure 2>&1)" || exit_code=$?

  if [[ "${exit_code}" -eq 0 ]]; then
    fail "intentional failure run exited 0 (expected nonzero)"
  fi

  local workspace
  workspace="$(extract_workspace "${output}")"

  assert_no_docker_resources "failure-run" "${project}"
  if [[ -n "${workspace}" ]]; then
    assert_no_generated_files "failure-run" "${workspace}"
  fi
  assert_no_port_listeners "failure-run" "${output}"

  log "intentional failure run exited ${exit_code} (expected nonzero) — cleanup verified"
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
