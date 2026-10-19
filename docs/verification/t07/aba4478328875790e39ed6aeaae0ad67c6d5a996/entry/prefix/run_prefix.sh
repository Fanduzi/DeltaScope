#!/bin/bash
# Pre-fix entry capture at 994bd8eb — real parser + AuditSQL via CLI.
BIN=/tmp/ds-t07r1/bin/deltascope-prefix
POL=/tmp/ds-t07r1/policies/golden-policy-t07-all.yaml
OUT=/tmp/ds-t07r1/prefix
run() { # name dialect version sql
  "$BIN" audit --sql "$4" --dialect "$2" --target-version "$3" --config "$POL" --format json > "$OUT/$1.json" 2>"$OUT/$1.stderr"; echo "$1 rc=$?"
}
# R1-1 generated-reference group (mysql dialect, 8.0.46)
run G_VALID        mysql 8.0.46 "CREATE TABLE t (a INT, b INT AS (a + 1) STORED);"
run G_SELF         mysql 8.0.46 "CREATE TABLE t (a INT, b INT AS (b + 1) STORED);"
run G_MISSING      mysql 8.0.46 "CREATE TABLE t (a INT, b INT AS (missing_c + 1) STORED);"
run G_BASE_FWD     mysql 8.0.46 "CREATE TABLE t (b INT AS (a + 1) STORED, a INT);"
run G_GEN_FWD      mysql 8.0.46 "CREATE TABLE t (a INT, b INT AS (c + 1) STORED, c INT AS (a + 1) STORED);"
run G_CYCLE        mysql 8.0.46 "CREATE TABLE t (a INT, b INT AS (c + 1) STORED, c INT AS (b + 1) STORED);"
run G_LITERAL      mysql 8.0.46 "CREATE TABLE t (a INT, b VARCHAR(32) AS ('missing_c') VIRTUAL);"
# post-statement controls: illegal CREATE then an ordinary-structure check
run G_SELF_POST    mysql 8.0.46 "CREATE TABLE t (a INT, b INT AS (b + 1) STORED); CREATE INDEX idx_a ON t(a);"
run G_MISSING_POST mysql 8.0.46 "CREATE TABLE t (a INT, b INT AS (missing_c + 1) STORED); ALTER TABLE t ADD COLUMN z INT;"
run G_CYCLE_POST   mysql 8.0.46 "CREATE TABLE t (a INT, b INT AS (c + 1) STORED, c INT AS (b + 1) STORED); CREATE INDEX idx_a ON t(a);"
# R1-2 TiDB expression-structure group (prior: v VARCHAR(36), x INT)
PRE="CREATE TABLE t (v VARCHAR(36), x INT);"
run D_UUID     tidb 8.5.0 "$PRE ALTER TABLE t ALTER COLUMN v SET DEFAULT (uuid());"
run D_ARITH    tidb 8.5.0 "$PRE ALTER TABLE t ALTER COLUMN v SET DEFAULT (uuid() + 1);"
run D_CASE     tidb 8.5.0 "$PRE ALTER TABLE t ALTER COLUMN v SET DEFAULT (CASE WHEN 1 THEN uuid() ELSE 'x' END);"
run D_PARENS   tidb 8.5.0 "$PRE ALTER TABLE t ALTER COLUMN v SET DEFAULT ((uuid()));"
run D_CONSTANT tidb 8.5.0 "$PRE ALTER TABLE t ALTER COLUMN v SET DEFAULT (1 + 2);"
# MySQL dialect opposite controls
run M_ARITH    mysql 8.0.46 "$PRE ALTER TABLE t ALTER COLUMN v SET DEFAULT (uuid() + 1);"
run M_CASE     mysql 8.0.46 "$PRE ALTER TABLE t ALTER COLUMN v SET DEFAULT (CASE WHEN 1 THEN uuid() ELSE 'x' END);"
# CREATE JSON positive same-root check (tidb)
run C_JSON_WRAP tidb 8.5.0 "CREATE TABLE t (j JSON DEFAULT (json_object('k',1) + 1));"
run C_JSON_OK   tidb 8.5.0 "CREATE TABLE t (j JSON DEFAULT (JSON_OBJECT('k',1)));"
