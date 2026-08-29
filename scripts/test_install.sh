#!/usr/bin/env bash
# input: install.sh plus local fake curl/wget clients and a fixture archive
# output: hermetic regression coverage for release discovery and pinned installs
# pos: shell-level contract test for the public POSIX installer boundary
# note: if this file changes, update this header and scripts/README.md.

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
INSTALLER="${SCRIPT_DIR}/../install.sh"
TEMP_DIR="$(mktemp -d)"
PASSED=0
FAILED=0

cleanup() {
  rm -rf "${TEMP_DIR}"
}
trap cleanup EXIT

pass() {
  printf '  PASS: %s\n' "$1"
  PASSED=$((PASSED + 1))
}

fail() {
  printf '  FAIL: %s\n' "$1"
  FAILED=$((FAILED + 1))
}

setup_case() {
  local name="$1" tool command_path

  CASE_DIR="${TEMP_DIR}/${name}"
  mkdir -p "${CASE_DIR}/tools" "${CASE_DIR}/fixture" "${CASE_DIR}/install"
  printf '#!/bin/sh\nprintf installed\\n\n' > "${CASE_DIR}/fixture/deltascope"
  chmod +x "${CASE_DIR}/fixture/deltascope"
  tar -czf "${CASE_DIR}/archive.tar.gz" -C "${CASE_DIR}/fixture" deltascope

  cat > "${CASE_DIR}/fake-downloader" <<'EOF'
#!/bin/sh
set -eu

client="${0##*/}"
log="${FAKE_LOG:?}"
url=""
output=""

while [ "$#" -gt 0 ]; do
  case "$1" in
    -o)
      output="$2"
      shift 2
      ;;
    -w)
      shift 2
      ;;
    -qO-)
      output="-"
      shift
      ;;
    -qO)
      output="$2"
      shift 2
      ;;
    --*)
      shift
      ;;
    -*)
      shift
      ;;
    *)
      url="$1"
      shift
      ;;
  esac
done

case "${url}" in
  https://api.github.com/repos/Fanduzi/DeltaScope/releases/latest)
    printf 'api\n' >> "${log}"
    response=''
    case "${FAKE_API_MODE}" in
      success) response='{"tag_name":"v1.2.3"}' ;;
      invalid) response='{"tag_name":"latest"}' ;;
      empty) response='' ;;
      failure)
        printf 'untrusted response body: rate limit exceeded\n' >&2
        exit 22
        ;;
      *) exit 1 ;;
    esac
    if [ "${output}" = '-' ] || [ -z "${output}" ]; then
      printf '%s\n' "${response}"
    else
      printf '%s\n' "${response}" > "${output}"
    fi
    ;;
  https://github.com/Fanduzi/DeltaScope/releases/latest)
    printf 'redirect\n' >> "${log}"
    case "${FAKE_REDIRECT_MODE}" in
      success) target='https://github.com/Fanduzi/DeltaScope/releases/tag/v1.2.3' ;;
      invalid) target='https://github.com/Fanduzi/DeltaScope/releases/tag/v1.2' ;;
      nonrelease) target='https://example.test/not-a-release' ;;
      empty) target='' ;;
      failure)
        printf 'untrusted response body: redirect failed\n' >&2
        exit 22
        ;;
      *) exit 1 ;;
    esac
    if [ "${client}" = 'curl' ]; then
      [ -n "${target}" ] && printf '%s\n' "${target}"
    elif [ -n "${target}" ]; then
      printf 'Location: %s [following]\n' "${target}" >&2
    fi
    ;;
  https://github.com/Fanduzi/DeltaScope/releases/download/*)
    printf 'asset\n' >> "${log}"
    [ -n "${output}" ] && [ "${output}" != '-' ] || exit 1
    cp "${FAKE_ARCHIVE}" "${output}"
    ;;
  *)
    printf 'unexpected URL: %s\n' "${url}" >&2
    exit 1
    ;;
esac
EOF
  chmod +x "${CASE_DIR}/fake-downloader"

  for tool in cp cut head id install mkdir mktemp rm sed tail tar tr uname; do
    command_path="$(command -v "${tool}")"
    ln -s "${command_path}" "${CASE_DIR}/tools/${tool}"
  done
}

run_case() {
  local name="$1" client="$2" api_mode="$3" redirect_mode="$4" version="$5"
  local status

  setup_case "${name}-${client}"
  ln -s "${CASE_DIR}/fake-downloader" "${CASE_DIR}/tools/${client}"
  : > "${CASE_DIR}/calls.log"

  set +e
  env \
    PATH="${CASE_DIR}/tools" \
    FAKE_LOG="${CASE_DIR}/calls.log" \
    FAKE_ARCHIVE="${CASE_DIR}/archive.tar.gz" \
    FAKE_API_MODE="${api_mode}" \
    FAKE_REDIRECT_MODE="${redirect_mode}" \
    DELTASCOPE_REPO='Fanduzi/DeltaScope' \
    DELTASCOPE_INSTALL_DIR="${CASE_DIR}/install" \
    DELTASCOPE_BINARIES='deltascope' \
    DELTASCOPE_VERSION="${version}" \
    /bin/sh "${INSTALLER}" > "${CASE_DIR}/stdout" 2> "${CASE_DIR}/stderr"
  status=$?
  set -e
  CASE_STATUS="${status}"
}

expect_success() {
  local description="$1"

  if [ "${CASE_STATUS}" -eq 0 ] && [ -f "${CASE_DIR}/install/deltascope" ] && grep -Fq 'Version: v1.2.3' "${CASE_DIR}/stdout"; then
    pass "${description}"
  else
    fail "${description}"
  fi
}

expect_failure_with_pin_hint() {
  local description="$1"

  if [ "${CASE_STATUS}" -ne 0 ] \
    && grep -Fq 'could not resolve a release version' "${CASE_DIR}/stderr" \
    && grep -Fq 'DELTASCOPE_VERSION=vX.Y.Z' "${CASE_DIR}/stderr" \
    && ! grep -Fq 'untrusted response body' "${CASE_DIR}/stderr"; then
    pass "${description}"
  else
    fail "${description}"
  fi
}

expect_wget_fallback_hint() {
  local description="$1"

  if grep -Fq 'wget latest-release redirect fallback failed or is unsupported' "${CASE_DIR}/stderr" \
    && grep -Fq 'use curl or set DELTASCOPE_VERSION=vX.Y.Z' "${CASE_DIR}/stderr"; then
    pass "${description}"
  else
    fail "${description}"
  fi
}

expect_discovery_calls() {
  local description="$1"
  shift
  local expected

  for expected in "$@"; do
    if ! grep -Fxq "${expected}" "${CASE_DIR}/calls.log"; then
      fail "${description}"
      return
    fi
  done
  pass "${description}"
}

expect_no_call() {
  local description="$1" forbidden="$2"

  if grep -Fxq "${forbidden}" "${CASE_DIR}/calls.log"; then
    fail "${description}"
  else
    pass "${description}"
  fi
}

expect_only_asset_call() {
  local description="$1"

  if [ "$(grep -Ec '^(api|redirect|asset)$' "${CASE_DIR}/calls.log")" -eq 1 ] \
    && grep -Fxq 'asset' "${CASE_DIR}/calls.log"; then
    pass "${description}"
  else
    fail "${description}"
  fi
}

echo '=== test_install (hermetic) ==='

for client in curl wget; do
  run_case api-success "${client}" success failure ''
  expect_success "${client}: API success resolves and installs"
  expect_discovery_calls "${client}: API success avoids redirect" api asset
  expect_no_call "${client}: API success does not call redirect" redirect

  run_case redirect-success "${client}" failure success ''
  expect_success "${client}: API failure falls back to latest redirect"
  expect_discovery_calls "${client}: fallback visits API, redirect, and asset" api redirect asset

  run_case invalid-api "${client}" invalid success ''
  expect_success "${client}: invalid API tag falls back to latest redirect"
  expect_discovery_calls "${client}: invalid API tag visits redirect" api redirect asset

  for redirect_mode in invalid empty nonrelease failure; do
    run_case "redirect-${redirect_mode}" "${client}" failure "${redirect_mode}" ''
    expect_failure_with_pin_hint "${client}: ${redirect_mode} redirect is rejected safely"
    if [ "${client}" = 'wget' ] && [ "${redirect_mode}" = 'failure' ]; then
      expect_wget_fallback_hint "${client}: failed redirect gives supported-client guidance"
    fi
  done

  run_case pinned "${client}" failure failure v1.2.3
  expect_success "${client}: pinned install succeeds without discovery"
  expect_only_asset_call "${client}: pinned install calls only the release asset"
done

printf '\nResults: %d passed, %d failed\n' "${PASSED}" "${FAILED}"
[ "${FAILED}" -eq 0 ] && echo 'All tests passed.' || exit 1
