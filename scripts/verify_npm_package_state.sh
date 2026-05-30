#!/usr/bin/env bash
# npm package state helper.
# Checks whether @fanduzi/deltascope-mcp@VERSION exists on npm.
# Read-only — never publishes.

set -euo pipefail

VERSION="${VERSION:?VERSION is required (e.g. VERSION=v0.230.0)}"
VERSION_NO_V="${VERSION#v}"

REGISTRY="https://registry.npmjs.org"
PACKAGE="@fanduzi/deltascope-mcp"

output=$(npm view "${PACKAGE}@${VERSION_NO_V}" version --registry "$REGISTRY" 2>&1) || {
    case $? in
        *)
            # npm view exits non-zero for 404 / E404
            if echo "$output" | grep -qiE "404|E404|not found"; then
                echo "npm-package-state: absent ${VERSION_NO_V}"
                exit 0
            fi
            echo "npm-package-state: FAIL — npm view error: ${output}"
            exit 1
            ;;
    esac
}

# Trim whitespace
output="$(echo "$output" | tr -d '[:space:]')"

if [ "$output" = "$VERSION_NO_V" ]; then
    echo "npm-package-state: present ${VERSION_NO_V}"
    exit 0
fi

echo "npm-package-state: FAIL — version mismatch: expected ${VERSION_NO_V}, got ${output}"
exit 1
