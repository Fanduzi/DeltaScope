#!/usr/bin/env bash
# input: a DeltaScope release archive, optional checksum sidecar, expected VERSION, and optional GLIBC baseline
# output: release-archive contract validation for packaged binaries, docs, checksums, version output, PostgreSQL CLI smoke, and Linux glibc baseline
# pos: release artifact verifier used by Makefile package targets before upload/install surfaces consume archives

set -euo pipefail

ARCHIVE="${ARCHIVE:-}"
CHECKSUM="${CHECKSUM:-}"
VERSION="${VERSION:-}"
GLIBC_BASELINE="${GLIBC_BASELINE:-}"

fail() {
  printf '[release-archive-verify][FAIL] %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

compare_glibc() {
  local binary_path="$1"
  local actual="$2"

  [[ -n "${actual}" ]] || fail "no GLIBC symbols found in ${binary_path}"
  if [[ "$(printf '%s\n%s\n' "${GLIBC_BASELINE}" "${actual}" | sort -V | tail -1)" != "${GLIBC_BASELINE}" ]]; then
    fail "glibc baseline check failed for ${binary_path}: found ${actual}, expected <= ${GLIBC_BASELINE}"
  fi
}

main() {
  require_cmd grep
  require_cmd mktemp
  require_cmd rm
  require_cmd tar

  [[ -n "${ARCHIVE}" ]] || fail "ARCHIVE is required"
  [[ -n "${VERSION}" ]] || fail "VERSION is required"
  [[ -f "${ARCHIVE}" ]] || fail "archive not found: ${ARCHIVE}"

  if [[ -n "${CHECKSUM}" ]]; then
    [[ -f "${CHECKSUM}" ]] || fail "checksum sidecar not found: ${CHECKSUM}"
    grep -q "  $(basename "${ARCHIVE}")$" "${CHECKSUM}" || fail "checksum sidecar does not mention $(basename "${ARCHIVE}")"
  fi

  local contents="" extract_dir=""
  contents="$(mktemp)"
  extract_dir="$(mktemp -d)"
  trap 'rm -f "${contents:-}"; rm -rf "${extract_dir:-}"' EXIT

  tar -tzf "${ARCHIVE}" > "${contents}"
  grep -q '^deltascope$' "${contents}" || fail "archive missing deltascope"
  grep -q '^deltascope-server$' "${contents}" || fail "archive missing deltascope-server"
  grep -q '^deltascope-mcp$' "${contents}" || fail "archive missing deltascope-mcp"
  grep -q '^README.md$' "${contents}" || fail "archive missing README.md"
  grep -q '^README_ZH.md$' "${contents}" || fail "archive missing README_ZH.md"
  grep -q '^CHANGELOG.md$' "${contents}" || fail "archive missing CHANGELOG.md"
  grep -q '^SECURITY.md$' "${contents}" || fail "archive missing SECURITY.md"

  tar -xzf "${ARCHIVE}" -C "${extract_dir}"

  local cli_version server_version mcp_version
  cli_version="$("${extract_dir}/deltascope" --version)"
  [[ "${cli_version}" == *"${VERSION}"* ]] || fail "deltascope --version mismatch: got ${cli_version}, expected ${VERSION}"
  [[ "${cli_version}" == *"postgresql"* ]] || fail "deltascope --version does not expose PostgreSQL support: ${cli_version}"

  server_version="$("${extract_dir}/deltascope-server" --version)"
  [[ "${server_version}" == "${VERSION}" ]] || fail "deltascope-server --version mismatch: got ${server_version}, expected ${VERSION}"

  mcp_version="$("${extract_dir}/deltascope-mcp" -version)"
  [[ "${mcp_version}" == "${VERSION}" ]] || fail "deltascope-mcp -version mismatch: got ${mcp_version}, expected ${VERSION}"

  "${extract_dir}/deltascope" audit \
    --dialect postgresql \
    --sql 'create table pg_smoke (id bigint primary key);' \
    --format json \
    --fail-on none >/dev/null

  if [[ -n "${GLIBC_BASELINE}" ]]; then
    require_cmd file
    require_cmd sort
    require_cmd strings
    for binary in deltascope deltascope-server deltascope-mcp; do
      local binary_path max_glibc
      binary_path="${extract_dir}/${binary}"
      if file "${binary_path}" | grep -q 'ELF'; then
        max_glibc="$(strings "${binary_path}" | grep -o 'GLIBC_[0-9.]*' | sort -Vu | tail -1 || true)"
        compare_glibc "${binary_path}" "${max_glibc}"
      fi
    done
  fi
}

main "$@"
