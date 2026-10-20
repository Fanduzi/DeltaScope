#!/bin/bash
# T07-R2 green entry capture at final code SHA 9538c05905d2cf78895bcdcc8346b251453c3151.
# Binary: CGO_ENABLED=0 go build ./cmd/deltascope at that SHA
#   (sha256 39b76649b2f837a1cee60b5a0657dc54353dfe3ccd1efee4d49049763f52aacf,
#    identical to the Golden artifact cli.sha256; vcs.revision=9538c059...).
# Inputs: the exact F01-F08 fixture SQL from CONTRACT §12 — controlled
# provider-equivalent prior "CREATE TABLE t (v INT, x INT);" + one SET.
BIN=${BIN:-/tmp/ds-t07r2/bin/deltascope-final}
POL=${POL:-/tmp/ds-t07r1/policies/golden-policy-t07-compat.yaml}
OUT=${OUT:-/tmp/ds-t07r2/green-final}
mkdir -p "$OUT"
run() { # name sql
  "$BIN" audit --sql "$2" --dialect tidb --target-version 8.5.0 --config "$POL" --format json > "$OUT/$1.json" 2>"$OUT/$1.stderr"; echo "$1 rc=$?"
}
PRE="CREATE TABLE t (v INT, x INT);"
run F01 "$PRE ALTER TABLE t ALTER COLUMN v SET DEFAULT (CASE WHEN 1 THEN 1 ELSE 2 END);"
run F02 "$PRE ALTER TABLE t ALTER COLUMN v SET DEFAULT (CASE WHEN 1 THEN 1 ELSE uuid() END);"
run F03 "$PRE ALTER TABLE t ALTER COLUMN v SET DEFAULT (CASE WHEN 1 THEN uuid() ELSE 'x' END);"
run F04 "$PRE ALTER TABLE t ALTER COLUMN v SET DEFAULT ((LOWER('1')));"
run F05 "$PRE ALTER TABLE t ALTER COLUMN v SET DEFAULT ((LOWER('X')));"
run F06 "$PRE ALTER TABLE t ALTER COLUMN v SET DEFAULT (1 + 2);"
run F07 "$PRE ALTER TABLE t ALTER COLUMN v SET DEFAULT ('X' + 1);"
run F08 "$PRE ALTER TABLE t ALTER COLUMN v SET DEFAULT (uuid());"
