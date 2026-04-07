#!/usr/bin/env bash
# input: local git checkout, Docker daemon, manylinux image, and the PostgreSQL build tag
# output: repeatable Linux PG-capable builds for the converged primary binaries plus automated glibc baseline verification
# pos: reusable manylinux/glibc verification gate for the Linux amd64 converged release lane and transitional PG CLI artifact
# note: if this file changes, update this header and module README.md.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD_DIR="${BUILD_DIR:-bin}"
BASELINE="${PG_GLIBC_BASELINE:-GLIBC_2.17}"
MANYLINUX_IMAGE="${PG_MANYLINUX_IMAGE:-quay.io/pypa/manylinux2014_x86_64}"
MANYLINUX_PLATFORM="${PG_MANYLINUX_PLATFORM:-linux/amd64}"
GO_VERSION="${GO_VERSION:-$(go env GOVERSION | sed 's/^go//')}"

log() {
  printf '[pg-manylinux-baseline] %s\n' "$*"
}

fail() {
  printf '[pg-manylinux-baseline][FAIL] %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

compare_glibc() {
  local artifact_path="$1"
  local actual="$2"
  local baseline="$3"

  if [[ -z "${actual}" ]]; then
    fail "no GLIBC symbols found in ${artifact_path}; expected an ELF binary with a measurable glibc baseline"
  fi

  if [[ "$(printf '%s\n%s\n' "${baseline}" "${actual}" | sort -V | tail -1)" != "${baseline}" ]]; then
    fail "glibc baseline check failed for ${artifact_path}: found ${actual}, expected <= ${baseline}"
  fi
}

verify_artifact() {
  local artifact_path="$1"
  local max_glibc

  max_glibc="$(strings "${artifact_path}" | grep -o 'GLIBC_[0-9.]*' | sort -Vu | tail -1 || true)"
  compare_glibc "${artifact_path}" "${max_glibc}" "${BASELINE}"
  log "built ${artifact_path}"
  log "max glibc symbol: ${max_glibc}"
  log "baseline check passed (<= ${BASELINE})"
}

main() {
  require_cmd docker
  require_cmd strings
  require_cmd sort
  require_cmd grep
  require_cmd go

  mkdir -p "${ROOT_DIR}/${BUILD_DIR}"

  log "building converged Linux amd64 PG-capable binaries in ${MANYLINUX_IMAGE} (${MANYLINUX_PLATFORM})"
  docker run --rm \
    --platform "${MANYLINUX_PLATFORM}" \
    -v "${ROOT_DIR}:/src" \
    -w /src \
    -e GO_VERSION="${GO_VERSION}" \
    -e BUILD_DIR="${BUILD_DIR}" \
    "${MANYLINUX_IMAGE}" \
    bash -lc '
      set -euo pipefail
      GO_TARBALL="go${GO_VERSION}.linux-amd64.tar.gz"
      curl -fsSLo "/tmp/${GO_TARBALL}" "https://go.dev/dl/${GO_TARBALL}"
      rm -rf /usr/local/go
      tar -C /usr/local -xzf "/tmp/${GO_TARBALL}"
      export PATH="/usr/local/go/bin:${PATH}"
      mkdir -p "${BUILD_DIR}"
      CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -buildvcs=false -trimpath -tags postgresql -o "${BUILD_DIR}/deltascope-linux-amd64-pg" ./cmd/deltascope
      CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -buildvcs=false -trimpath -tags postgresql -o "${BUILD_DIR}/deltascope-server-linux-amd64-pg" ./cmd/deltascope-server
      CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -buildvcs=false -trimpath -tags postgresql -o "${BUILD_DIR}/deltascope-mcp-linux-amd64-pg" ./cmd/deltascope-mcp
    '

  cp "${ROOT_DIR}/${BUILD_DIR}/deltascope-linux-amd64-pg" "${ROOT_DIR}/${BUILD_DIR}/deltascope-pg-manylinux-amd64"

  verify_artifact "${ROOT_DIR}/${BUILD_DIR}/deltascope-linux-amd64-pg"
  verify_artifact "${ROOT_DIR}/${BUILD_DIR}/deltascope-server-linux-amd64-pg"
  verify_artifact "${ROOT_DIR}/${BUILD_DIR}/deltascope-mcp-linux-amd64-pg"
  verify_artifact "${ROOT_DIR}/${BUILD_DIR}/deltascope-pg-manylinux-amd64"
}

main "$@"
