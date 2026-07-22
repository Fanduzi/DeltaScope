#!/bin/bash
# Documentation gate: ensure HTTP API docs only show connection_id, not inline connection objects
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FAIL=0

echo "[connection-id-docs] checking HTTP API documentation..."

# Check for old connection object patterns in HTTP docs (excluding CLI docs which still use direct flags)
for f in \
  "$ROOT_DIR/README.md" \
  "$ROOT_DIR/README_ZH.md" \
  "$ROOT_DIR/docs/reference/http-api.md" \
  "$ROOT_DIR/docs/reference/http-api.zh-CN.md"
do
  if [ ! -f "$f" ]; then
    continue
  fi
  
  # Look for JSON examples with inline connection object (host/port/password_env)
  if grep -qE '"connection"\s*:\s*\{' "$f" 2>/dev/null; then
    echo "[connection-id-docs][FAIL] $f contains inline connection object JSON example"
    FAIL=1
  fi
  
  # Look for connection_inputs in capabilities (should be connection_id now)
  if grep -q '"connection_inputs"' "$f" 2>/dev/null; then
    echo "[connection-id-docs][FAIL] $f contains old connection_inputs capability"
    FAIL=1
  fi
done

if [ "$FAIL" -eq 0 ]; then
  echo "[connection-id-docs] PASS"
else
  echo "[connection-id-docs] FAIL - see errors above"
  exit 1
fi
