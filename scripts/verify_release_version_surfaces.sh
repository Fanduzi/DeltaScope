#!/usr/bin/env bash
# input: VERSION with leading v
# output: validation that source, npm, README, release-note, release index, and landing current-version surfaces match VERSION
# pos: release contract gate called by Makefile and release workflow before publishing

set -euo pipefail

VERSION="${VERSION:-}"

fail() {
  printf '[release-version-surfaces][FAIL] %s\n' "$*" >&2
  exit 1
}

require_version() {
  [[ -n "${VERSION}" ]] || fail "VERSION is required"
  [[ "${VERSION}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "VERSION must look like vX.Y.Z: ${VERSION}"
}

require_grep_literal() {
  local needle="$1"
  local file="$2"
  grep -Fq "${needle}" "${file}" || fail "${file} missing literal: ${needle}"
}

require_grep_regex() {
  local pattern="$1"
  local file="$2"
  grep -Eq "${pattern}" "${file}" || fail "${file} missing pattern: ${pattern}"
}

require_landing_current_surfaces() {
  local file="docs/landing/index.html"

  # DOM checks — static HTML surfaces
  require_grep_literal "<span class=\"tag tag-new\" data-i18n=\"hero.badge\">${VERSION}" "${file}"
  require_grep_literal "<div class=\"release-version\">${VERSION}</div>" "${file}"
  require_grep_literal "data-i18n=\"footer.links.latest\"" "${file}"
  require_grep_literal "Release Notes ${VERSION}" "${file}"
  require_grep_literal "release-notes-${VERSION}.md" "${file}"
  require_grep_literal "release-notes-${VERSION}.zh-CN.md" "${file}"

  # JS i18n checks — badge and footer latest
  require_grep_literal "badge: '${VERSION}" "${file}"
  require_grep_literal "footer: {" "${file}"
  require_grep_literal "latest: 'Release Notes ${VERSION}'" "${file}"
  require_grep_literal "latest: '${VERSION} 发布说明'" "${file}"

  # Current-release i18n text must exist. Historical release cards may mention older versions.
  require_grep_regex "currentTitle: '([^']+)'" "${file}"
  require_grep_regex "brand: '([^']|\\\\')*${VERSION}" "${file}"
}

main() {
  require_version

  local raw_version="${VERSION#v}"
  local en_notes="docs/releases/release-notes-${VERSION}.md"
  local zh_notes="docs/releases/release-notes-${VERSION}.zh-CN.md"

  [[ -f "${en_notes}" ]] || fail "missing English release notes: ${en_notes}"
  [[ -f "${zh_notes}" ]] || fail "missing Chinese release notes: ${zh_notes}"

  require_grep_literal "# DeltaScope ${VERSION} Release Notes" "${en_notes}"
  require_grep_literal "# DeltaScope ${VERSION} 发行说明" "${zh_notes}"

  require_grep_literal "DefaultVersion = \"${VERSION}\"" "pkg/deltascope/version.go"
  require_grep_literal "DefaultVersion\` is \`${VERSION}" "pkg/deltascope/README.md"

  local npm_version
  npm_version="$(node -p 'require("./packages/deltascope-mcp/package.json").version')"
  [[ "${npm_version}" == "${raw_version}" ]] || fail "npm package version ${npm_version} != ${raw_version}"

  require_grep_literal "https://raw.githubusercontent.com/Fanduzi/DeltaScope/${VERSION}/install.sh" "README.md"
  require_grep_literal "DELTASCOPE_VERSION=${VERSION} sh" "README.md"
  require_grep_literal "https://raw.githubusercontent.com/Fanduzi/DeltaScope/${VERSION}/install.sh" "README_ZH.md"
  require_grep_literal "DELTASCOPE_VERSION=${VERSION} sh" "README_ZH.md"

  require_grep_literal "release-notes-${VERSION}.md" "docs/releases/README.md"
  require_grep_literal "release-notes-${VERSION}.zh-CN.md" "docs/releases/README.md"

  require_landing_current_surfaces
}

main "$@"
