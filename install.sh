#!/usr/bin/env sh
# input: release version selectors, host os/arch facts, and GitHub release archives from the DeltaScope repository
# output: installed DeltaScope release binaries under a local operator-selected bin directory
# pos: release-facing installer that resolves one trusted artifact contract into local binaries
# note: if this file changes, update this header and module README.md.

set -eu

REPO="${DELTASCOPE_REPO:-Fanduzi/DeltaScope}"
VERSION="${DELTASCOPE_VERSION:-}"
INSTALL_DIR="${DELTASCOPE_INSTALL_DIR:-/usr/local/bin}"
BINARIES="${DELTASCOPE_BINARIES:-}"

log() {
  printf '%s\n' "$*"
}

fail() {
  printf 'install.sh: %s\n' "$*" >&2
  exit 1
}

need_cmd() {
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

fetch_latest_version() {
  need_cmd sed
  api_url="https://api.github.com/repos/${REPO}/releases/latest"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "${api_url}" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- "${api_url}" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1
  else
    fail "missing downloader: need curl or wget"
  fi
}

download_file() {
  url="$1"
  output="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "${url}" -o "${output}"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "${output}" "${url}"
  else
    fail "missing downloader: need curl or wget"
  fi
}

version_gte() {
  left="${1#v}"
  right="${2#v}"

  parse() {
    printf '%s' "$1" | sed 's/[^0-9.].*$//' 
  }

  left="$(parse "${left}")"
  right="$(parse "${right}")"

  left_major="$(printf '%s' "${left}" | cut -d. -f1)"
  left_minor="$(printf '%s' "${left}" | cut -d. -f2)"
  left_patch="$(printf '%s' "${left}" | cut -d. -f3)"
  right_major="$(printf '%s' "${right}" | cut -d. -f1)"
  right_minor="$(printf '%s' "${right}" | cut -d. -f2)"
  right_patch="$(printf '%s' "${right}" | cut -d. -f3)"

  left_major="${left_major:-0}"
  left_minor="${left_minor:-0}"
  left_patch="${left_patch:-0}"
  right_major="${right_major:-0}"
  right_minor="${right_minor:-0}"
  right_patch="${right_patch:-0}"

  if [ "${left_major}" -gt "${right_major}" ]; then
    return 0
  fi
  if [ "${left_major}" -lt "${right_major}" ]; then
    return 1
  fi
  if [ "${left_minor}" -gt "${right_minor}" ]; then
    return 0
  fi
  if [ "${left_minor}" -lt "${right_minor}" ]; then
    return 1
  fi
  if [ "${left_patch}" -ge "${right_patch}" ]; then
    return 0
  fi
  return 1
}

resolve_binaries() {
  if [ -n "${BINARIES}" ]; then
    printf '%s' "${BINARIES}"
    return
  fi

  if version_gte "${VERSION}" "v0.7.0"; then
    printf '%s' "deltascope deltascope-server deltascope-mcp"
    return
  fi

  printf '%s' "deltascope deltascope-server"
}

install_one() {
  src="$1"
  name="$2"

  if [ -w "${INSTALL_DIR}" ]; then
    install -m 0755 "${src}" "${INSTALL_DIR}/${name}"
    return
  fi

  if command -v sudo >/dev/null 2>&1; then
    sudo install -m 0755 "${src}" "${INSTALL_DIR}/${name}"
    return
  fi

  fallback_dir="${HOME}/.local/bin"
  mkdir -p "${fallback_dir}"
  install -m 0755 "${src}" "${fallback_dir}/${name}"
  log "install directory ${INSTALL_DIR} is not writable; installed ${name} to ${fallback_dir}/${name}"
}

need_cmd tar
need_cmd mktemp
need_cmd install

OS="$(detect_os)"
ARCH="$(detect_arch)"

if [ -z "${VERSION}" ]; then
  VERSION="$(fetch_latest_version)"
fi

[ -n "${VERSION}" ] || fail "could not resolve a release version"
BINARIES="$(resolve_binaries)"

VERSION_NO_V="${VERSION#v}"
ARCHIVE="deltascope_${VERSION_NO_V}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"
TMP_DIR="$(mktemp -d)"
ARCHIVE_PATH="${TMP_DIR}/${ARCHIVE}"

cleanup() {
  rm -rf "${TMP_DIR}"
}

trap cleanup EXIT INT TERM

log "downloading ${URL}"
download_file "${URL}" "${ARCHIVE_PATH}"
tar -xzf "${ARCHIVE_PATH}" -C "${TMP_DIR}"

for binary in ${BINARIES}; do
  src_path="${TMP_DIR}/${binary}"
  [ -f "${src_path}" ] || fail "archive did not contain ${binary}"
  install_one "${src_path}" "${binary}"
  log "installed ${binary}"
done

log "DeltaScope ${VERSION} installed"
