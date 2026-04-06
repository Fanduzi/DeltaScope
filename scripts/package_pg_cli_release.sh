#!/usr/bin/env bash
# input: a verified manylinux-built deltascope-pg binary, release version, and repository docs included in public archives
# output: stable public-release packaging for the single approved PG v1 artifact plus checksum sidecar
# pos: Phase 7 Slice 4 packaging helper for the public deltascope-pg release path only
# note: if this file changes, update this header and module README.md.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD_DIR="${BUILD_DIR:-bin}"
DIST_DIR="${DIST_DIR:-dist}"
VERSION="${VERSION:-}"
RAW_VERSION="${VERSION#v}"
SOURCE_BINARY="${ROOT_DIR}/${BUILD_DIR}/deltascope-pg-manylinux-amd64"
STAGED_BINARY="${ROOT_DIR}/${DIST_DIR}/deltascope-pg"
ARCHIVE_BASENAME="deltascope-pg_${RAW_VERSION}_linux_amd64.tar.gz"
ARCHIVE_PATH="${ROOT_DIR}/${DIST_DIR}/${ARCHIVE_BASENAME}"
CHECKSUM_BASENAME="deltascope-pg_${RAW_VERSION}_linux_amd64_checksums.txt"
CHECKSUM_PATH="${ROOT_DIR}/${DIST_DIR}/${CHECKSUM_BASENAME}"

log() {
  printf '[pg-release-package] %s\n' "$*"
}

fail() {
  printf '[pg-release-package][FAIL] %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

main() {
  require_cmd cp
  require_cmd mkdir
  require_cmd mktemp
  require_cmd rm
  require_cmd tar
  require_cmd shasum

  [[ -n "${VERSION}" ]] || fail "VERSION is required (example: VERSION=v0.15.0)"
  [[ -n "${RAW_VERSION}" ]] || fail "VERSION must not resolve to an empty raw version"
  [[ -f "${SOURCE_BINARY}" ]] || fail "missing manylinux baseline artifact: ${SOURCE_BINARY}; run make smoke-pg-cli-manylinux-baseline first"

  mkdir -p "${ROOT_DIR}/${DIST_DIR}"
  cp "${SOURCE_BINARY}" "${STAGED_BINARY}"
  chmod +x "${STAGED_BINARY}"

  local package_dir=""
  package_dir="$(mktemp -d "${ROOT_DIR}/${DIST_DIR}/pg-package.XXXXXX")"
  trap 'if [[ -n "${package_dir:-}" ]]; then rm -rf "${package_dir}"; fi' EXIT

  cp "${STAGED_BINARY}" "${package_dir}/deltascope-pg"
  cp "${ROOT_DIR}/README.md" "${package_dir}/README.md"
  cp "${ROOT_DIR}/README_ZH.md" "${package_dir}/README_ZH.md"
  cp "${ROOT_DIR}/CHANGELOG.md" "${package_dir}/CHANGELOG.md"
  cp "${ROOT_DIR}/SECURITY.md" "${package_dir}/SECURITY.md"

  tar -C "${package_dir}" -czf "${ARCHIVE_PATH}" deltascope-pg README.md README_ZH.md CHANGELOG.md SECURITY.md
  printf '%s  %s\n' "$(shasum -a 256 "${ARCHIVE_PATH}" | cut -d' ' -f1)" "${ARCHIVE_BASENAME}" > "${CHECKSUM_PATH}"

  log "staged binary: ${STAGED_BINARY}"
  log "release archive: ${ARCHIVE_PATH}"
  log "checksum file: ${CHECKSUM_PATH}"
}

main "$@"
