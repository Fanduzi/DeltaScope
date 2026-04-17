#!/usr/bin/env bash
# input: local git checkout, Docker daemon, manylinux image, and the PostgreSQL build tag
# output: repeatable Linux PG-capable builds for the converged primary binaries plus automated glibc baseline verification
# pos: reusable manylinux/glibc verification gate for the converged Linux PG release lanes
# note: if this file changes, update this header and module README.md.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD_DIR="${BUILD_DIR:-bin}"
BASELINE="${PG_GLIBC_BASELINE:-GLIBC_2.17}"
MANYLINUX_IMAGE="${PG_MANYLINUX_IMAGE:-quay.io/pypa/manylinux2014_x86_64}"
MANYLINUX_PLATFORM="${PG_MANYLINUX_PLATFORM:-linux/amd64}"
TARGET_ARCH="${PG_TARGET_ARCH:-amd64}"
GO_TARBALL_ARCH="${PG_GO_TARBALL_ARCH:-${TARGET_ARCH}}"
GO_VERSION="${GO_VERSION:-$(go env GOVERSION | sed 's/^go//')}"
VERSION="${VERSION:-}"

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

  log "building converged Linux ${TARGET_ARCH} PG-capable binaries in ${MANYLINUX_IMAGE} (${MANYLINUX_PLATFORM})"
  local docker_env_args=()
  local env_name env_value
  for env_name in \
    HTTP_PROXY HTTPS_PROXY ALL_PROXY NO_PROXY \
    http_proxy https_proxy all_proxy no_proxy \
    GOPROXY GOSUMDB GONOSUMDB GONOPROXY GOPRIVATE
  do
    env_value="${!env_name:-}"
    if [[ -n "${env_value}" ]]; then
      docker_env_args+=(-e "${env_name}=${env_value}")
    fi
  done

  docker run --rm \
    --platform "${MANYLINUX_PLATFORM}" \
    -v "${ROOT_DIR}:/src" \
    -w /src \
    -e GO_VERSION="${GO_VERSION}" \
    -e BUILD_DIR="${BUILD_DIR}" \
    -e TARGET_ARCH="${TARGET_ARCH}" \
    -e GO_TARBALL_ARCH="${GO_TARBALL_ARCH}" \
    -e VERSION="${VERSION}" \
    "${docker_env_args[@]}" \
    "${MANYLINUX_IMAGE}" \
    bash -lc '
      set -euo pipefail
      GO_TARBALL="go${GO_VERSION}.linux-${GO_TARBALL_ARCH}.tar.gz"
      curl -fsSLo "/tmp/${GO_TARBALL}" "https://go.dev/dl/${GO_TARBALL}"
      rm -rf /usr/local/go
      tar -C /usr/local -xzf "/tmp/${GO_TARBALL}"
      export PATH="/usr/local/go/bin:${PATH}"
      mkdir -p "${BUILD_DIR}"
      CLI_LDFLAGS=()
      MAIN_LDFLAGS=()
      if [[ -n "${VERSION}" ]]; then
        CLI_LDFLAGS=(-ldflags "-X github.com/Fanduzi/DeltaScope/internal/interfaces/cli.Version=${VERSION}")
        MAIN_LDFLAGS=(-ldflags "-X main.Version=${VERSION}")
      fi
      CGO_ENABLED=1 GOOS=linux GOARCH=${TARGET_ARCH} go build -buildvcs=false -trimpath -tags postgresql "${CLI_LDFLAGS[@]}" -o "${BUILD_DIR}/deltascope-linux-${TARGET_ARCH}-pg" ./cmd/deltascope
      CGO_ENABLED=1 GOOS=linux GOARCH=${TARGET_ARCH} go build -buildvcs=false -trimpath -tags postgresql "${MAIN_LDFLAGS[@]}" -o "${BUILD_DIR}/deltascope-server-linux-${TARGET_ARCH}-pg" ./cmd/deltascope-server
      CGO_ENABLED=1 GOOS=linux GOARCH=${TARGET_ARCH} go build -buildvcs=false -trimpath -tags postgresql "${MAIN_LDFLAGS[@]}" -o "${BUILD_DIR}/deltascope-mcp-linux-${TARGET_ARCH}-pg" ./cmd/deltascope-mcp
    '

  verify_artifact "${ROOT_DIR}/${BUILD_DIR}/deltascope-linux-${TARGET_ARCH}-pg"
  verify_artifact "${ROOT_DIR}/${BUILD_DIR}/deltascope-server-linux-${TARGET_ARCH}-pg"
  verify_artifact "${ROOT_DIR}/${BUILD_DIR}/deltascope-mcp-linux-${TARGET_ARCH}-pg"
}

main "$@"
