#!/usr/bin/env bash
# input: host-native PG-capable deltascope, deltascope-server, deltascope-mcp binaries plus release docs and a VERSION
# output: a host-platform main release archive and matching checksum file under dist/
# pos: packaging helper for native runner smoke validation before full cross-platform release convergence

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD_DIR="${BUILD_DIR:-bin}"
DIST_DIR="${DIST_DIR:-dist}"
VERSION="${VERSION:-}"
RAW_VERSION="${VERSION#v}"

resolve_path() {
  case "$1" in
    /*) printf '%s' "$1" ;;
    *) printf '%s/%s' "${ROOT_DIR}" "$1" ;;
  esac
}

fail() {
  printf '[host-release-package][FAIL] %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

detect_os() {
  case "$(uname -s)" in
    Darwin) printf 'darwin' ;;
    Linux) printf 'linux' ;;
    *) fail "unsupported operating system: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'amd64' ;;
    arm64|aarch64) printf 'arm64' ;;
    *) fail "unsupported architecture: $(uname -m)" ;;
  esac
}

main() {
  require_cmd cp
  require_cmd mkdir
  require_cmd mktemp
  require_cmd rm
  require_cmd tar
  require_cmd shasum

  [[ -n "${VERSION}" ]] || fail "VERSION is required (example: VERSION=v0.17.0)"
  [[ -n "${RAW_VERSION}" ]] || fail "VERSION must resolve to a non-empty raw version"

  local build_dir_abs dist_dir_abs
  build_dir_abs="$(resolve_path "${BUILD_DIR}")"
  dist_dir_abs="$(resolve_path "${DIST_DIR}")"

  local os arch archive_basename archive_path checksum_basename checksum_path package_dir
  os="$(detect_os)"
  arch="$(detect_arch)"
  archive_basename="deltascope_${RAW_VERSION}_${os}_${arch}.tar.gz"
  archive_path="${dist_dir_abs}/${archive_basename}"
  checksum_basename="deltascope_${RAW_VERSION}_${os}_${arch}_checksums.txt"
  checksum_path="${dist_dir_abs}/${checksum_basename}"

  [[ -f "${build_dir_abs}/deltascope" ]] || fail "missing binary: ${build_dir_abs}/deltascope"
  [[ -f "${build_dir_abs}/deltascope-server" ]] || fail "missing binary: ${build_dir_abs}/deltascope-server"
  [[ -f "${build_dir_abs}/deltascope-mcp" ]] || fail "missing binary: ${build_dir_abs}/deltascope-mcp"

  mkdir -p "${dist_dir_abs}"
  package_dir="$(mktemp -d "${dist_dir_abs}/host-package.XXXXXX")"
  trap 'if [[ -n "${package_dir:-}" ]]; then rm -rf "${package_dir}"; fi' EXIT

  cp "${build_dir_abs}/deltascope" "${package_dir}/deltascope"
  cp "${build_dir_abs}/deltascope-server" "${package_dir}/deltascope-server"
  cp "${build_dir_abs}/deltascope-mcp" "${package_dir}/deltascope-mcp"
  cp "${ROOT_DIR}/README.md" "${package_dir}/README.md"
  cp "${ROOT_DIR}/README_ZH.md" "${package_dir}/README_ZH.md"
  cp "${ROOT_DIR}/CHANGELOG.md" "${package_dir}/CHANGELOG.md"
  cp "${ROOT_DIR}/SECURITY.md" "${package_dir}/SECURITY.md"

  tar -C "${package_dir}" -czf "${archive_path}" \
    deltascope deltascope-server deltascope-mcp README.md README_ZH.md CHANGELOG.md SECURITY.md
  printf '%s  %s\n' "$(shasum -a 256 "${archive_path}" | cut -d' ' -f1)" "${archive_basename}" > "${checksum_path}"
}

main "$@"
