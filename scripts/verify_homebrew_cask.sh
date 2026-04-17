#!/usr/bin/env bash
# input: rendered Homebrew cask plus release version and expected darwin archive sha256 values
# output: cask contract validation for version, platform URLs, sha256 values, and binary stanza
# pos: pre-publish guard for the Homebrew tap update path

set -euo pipefail

CASK="${CASK:-}"
VERSION="${VERSION:-}"
DARWIN_AMD64_SHA256="${DARWIN_AMD64_SHA256:-}"
DARWIN_ARM64_SHA256="${DARWIN_ARM64_SHA256:-}"

fail() {
  printf '[verify-homebrew-cask][FAIL] %s\n' "$*" >&2
  exit 1
}

require_grep() {
  local pattern="$1"
  local description="$2"

  grep -Fq "${pattern}" "${CASK}" || fail "missing ${description}: ${pattern}"
}

main() {
  [[ -n "${CASK}" ]] || fail "CASK is required"
  [[ -n "${VERSION}" ]] || fail "VERSION is required"
  [[ -n "${DARWIN_AMD64_SHA256}" ]] || fail "DARWIN_AMD64_SHA256 is required"
  [[ -n "${DARWIN_ARM64_SHA256}" ]] || fail "DARWIN_ARM64_SHA256 is required"
  [[ -f "${CASK}" ]] || fail "cask file not found: ${CASK}"

  local raw_version
  raw_version="${VERSION#v}"
  [[ -n "${raw_version}" ]] || fail "VERSION must resolve to a non-empty raw version"

  require_grep 'cask "deltascope" do' "cask declaration"
  require_grep "version \"${raw_version}\"" "cask version"
  require_grep 'url "https://github.com/Fanduzi/DeltaScope/releases/download/v#{version}/deltascope_#{version}_darwin_amd64.tar.gz"' "darwin amd64 URL template"
  require_grep 'url "https://github.com/Fanduzi/DeltaScope/releases/download/v#{version}/deltascope_#{version}_darwin_arm64.tar.gz"' "darwin arm64 URL template"
  require_grep "sha256 \"${DARWIN_AMD64_SHA256}\"" "darwin amd64 sha256"
  require_grep "sha256 \"${DARWIN_ARM64_SHA256}\"" "darwin arm64 sha256"
  require_grep 'binary "deltascope"' "binary stanza"
}

main "$@"
