#!/usr/bin/env sh
# input: release version selectors, host os/arch facts, GitHub release metadata, and release archives from the DeltaScope repository
# output: installed DeltaScope release binaries under a local operator-selected bin directory
# pos: release-facing installer that resolves a validated release tag into local binaries
# note: if this file changes, update this header and module README.md.

set -eu

REPO="${DELTASCOPE_REPO:-Fanduzi/DeltaScope}"
VERSION="${DELTASCOPE_VERSION:-}"
INSTALL_DIR="${DELTASCOPE_INSTALL_DIR:-/usr/local/bin}"
BINARIES="${DELTASCOPE_BINARIES:-}"
INSTALL_DIR_SET="${DELTASCOPE_INSTALL_DIR:+1}"
BINARIES_SET="${DELTASCOPE_BINARIES:+1}"
PG_CAPABLE_LINUX_AMD64_VERSION="v0.16.3"
PG_CAPABLE_LINUX_ARM64_VERSION="v0.17.0"
PG_CAPABLE_DARWIN_VERSION="v0.17.0"

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

release_tag_from_api() {
  sed -n 's/.*"tag_name":[[:space:]]*"\(v[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\)".*/\1/p' | head -n 1
}

release_tag_from_redirect() {
  redirect_url="$1"
  redirect_prefix="https://github.com/${REPO}/releases/tag/"

  case "${redirect_url}" in
    "${redirect_prefix}"*) ;;
    *) return 1 ;;
  esac

  tag="${redirect_url#${redirect_prefix}}"
  valid_tag="$(printf '%s\n' "${tag}" | sed -n '/^v[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*$/p')"
  [ "${valid_tag}" = "${tag}" ] || return 1
  printf '%s\n' "${valid_tag}"
}

fetch_latest_redirect() {
  latest_url="https://github.com/${REPO}/releases/latest"

  if command -v curl >/dev/null 2>&1; then
    if curl -fsSL -o /dev/null -w '%{url_effective}\n' "${latest_url}" 2>/dev/null; then
      return 0
    fi
    return 1
  fi

  if command -v wget >/dev/null 2>&1; then
    redirect_headers="$(mktemp)"
    if wget --max-redirect=20 --server-response --spider "${latest_url}" >"${redirect_headers}" 2>&1; then
      sed -n 's/^[[:space:]]*[Ll]ocation:[[:space:]]*\(https:\/\/[^[:space:]]*\)\( \[following\]\)\{0,1\}$/\1/p' "${redirect_headers}" | tail -n 1
      redirect_status=0
    else
      redirect_status=2
    fi
    rm -f "${redirect_headers}"
    return "${redirect_status}"
  fi

  fail "missing downloader: need curl or wget"
}

fetch_latest_version() {
  need_cmd sed
  api_url="https://api.github.com/repos/${REPO}/releases/latest"
  api_body="$(mktemp)"
  api_tag=""

  if command -v curl >/dev/null 2>&1; then
    if curl -fsSL "${api_url}" >"${api_body}" 2>/dev/null; then
      api_tag="$(release_tag_from_api <"${api_body}")"
    fi
  elif command -v wget >/dev/null 2>&1; then
    if wget -qO "${api_body}" "${api_url}" 2>/dev/null; then
      api_tag="$(release_tag_from_api <"${api_body}")"
    fi
  else
    rm -f "${api_body}"
    fail "missing downloader: need curl or wget"
  fi
  rm -f "${api_body}"

  if [ -n "${api_tag}" ]; then
    printf '%s\n' "${api_tag}"
    return
  fi

  redirect_url=""
  redirect_status=0
  if redirect_url="$(fetch_latest_redirect 2>/dev/null)"; then
    :
  else
    redirect_status="$?"
  fi
  redirect_tag="$(release_tag_from_redirect "${redirect_url}" || true)"
  if [ -n "${redirect_tag}" ]; then
    printf '%s\n' "${redirect_tag}"
    return
  fi

  if [ "${redirect_status}" -eq 2 ]; then
    fail "could not resolve a release version; wget latest-release redirect fallback failed or is unsupported; use curl or set DELTASCOPE_VERSION=vX.Y.Z to bypass version discovery"
  fi

  fail "could not resolve a release version; set DELTASCOPE_VERSION=vX.Y.Z to bypass version discovery"
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

is_root() {
  [ "$(id -u)" -eq 0 ]
}

is_interactive() {
  [ -t 0 ]
}

confirm() {
  prompt="$1"
  default="${2:-y}"

  if [ "${default}" = "y" ]; then
    suffix="[Y/n]"
  else
    suffix="[y/N]"
  fi

  while true; do
    printf '%s %s ' "${prompt}" "${suffix}" >&2
    IFS= read -r answer || answer=""
    answer="$(printf '%s' "${answer}" | tr '[:upper:]' '[:lower:]')"
    if [ -z "${answer}" ]; then
      answer="${default}"
    fi
    case "${answer}" in
      y|yes) return 0 ;;
      n|no) return 1 ;;
    esac
  done
}

supports_mcp_binary() {
  version_gte "${VERSION}" "v0.7.0"
}

supports_pg_capable_main_archive() {
  if [ "${OS}" = "linux" ] && [ "${ARCH}" = "amd64" ] && version_gte "${VERSION}" "${PG_CAPABLE_LINUX_AMD64_VERSION}"; then
    return 0
  fi
  if [ "${OS}" = "linux" ] && [ "${ARCH}" = "arm64" ] && version_gte "${VERSION}" "${PG_CAPABLE_LINUX_ARM64_VERSION}"; then
    return 0
  fi
  [ "${OS}" = "darwin" ] && version_gte "${VERSION}" "${PG_CAPABLE_DARWIN_VERSION}"
}

prompt_binaries() {
  log "Select binaries to install:"
  log "  1) deltascope"
  log "  2) deltascope deltascope-server"
  if supports_mcp_binary; then
    log "  3) deltascope deltascope-server deltascope-mcp"
  else
    log "  note: ${VERSION} does not publish deltascope-mcp"
  fi
  printf '%s' "Selection [1]: " >&2
  IFS= read -r selection || selection=""
  case "${selection:-1}" in
    1) printf '%s' "deltascope" ;;
    2) printf '%s' "deltascope deltascope-server" ;;
    3)
      if supports_mcp_binary; then
        printf '%s' "deltascope deltascope-server deltascope-mcp"
      else
        fail "${VERSION} does not support deltascope-mcp"
      fi
      ;;
    *) fail "unsupported selection: ${selection}" ;;
  esac
}

prompt_install_dir() {
  printf '%s' "Install directory [${INSTALL_DIR}]: " >&2
  IFS= read -r selected_dir || selected_dir=""
  if [ -n "${selected_dir}" ]; then
    INSTALL_DIR="${selected_dir}"
  fi
}

resolve_binaries() {
  if [ -n "${BINARIES}" ]; then
    printf '%s' "${BINARIES}"
    return
  fi

  printf '%s' "deltascope"
}

summarize_install() {
  log "Version: ${VERSION}"
  log "Platform: ${OS}/${ARCH}"
  log "Binaries: ${BINARIES}"
  log "Install dir: ${INSTALL_DIR}"
  if supports_pg_capable_main_archive; then
    log "PostgreSQL offline: supported on this platform via the main release archive"
  else
    log "PostgreSQL offline: not shipped via this platform's main archive"
  fi
}

install_one() {
  src="$1"
  name="$2"

  if [ -w "${INSTALL_DIR}" ]; then
    install -m 0755 "${src}" "${INSTALL_DIR}/${name}"
    return
  fi

  if is_root; then
    install -m 0755 "${src}" "${INSTALL_DIR}/${name}"
    return
  fi

  if command -v sudo >/dev/null 2>&1; then
    log "Install directory ${INSTALL_DIR} is not writable; sudo is required to continue."
    if is_interactive && ! confirm "Continue with sudo install?" "y"; then
      fail "installation cancelled"
    fi
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

if is_interactive && [ -z "${BINARIES_SET}" ]; then
  BINARIES="$(prompt_binaries)"
fi

if is_interactive && [ -z "${INSTALL_DIR_SET}" ]; then
  prompt_install_dir
fi

summarize_install

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
