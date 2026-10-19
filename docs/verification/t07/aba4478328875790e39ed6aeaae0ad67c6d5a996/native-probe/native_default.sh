#!/bin/bash
# Bounded native confirmation: TiDB expression-default whole-expression shape.
# Each SET gets a fresh owned prior table to avoid cross-contamination.
source /tmp/ds-t07r1/native_gen.sh
run_def() { # label anchor tbl prior create-sql set-sql
  local label="$1" ak="$2" tbl="$3" create="$4" set="$5"
  local pre post cols out rc cout crc
  pre=$(count0 "$ak" "$tbl")
  echo "### $label $ak table=$tbl pre=$pre"
  # fresh prior (create rc recorded — it is also the probe for CREATE-side cases)
  cout=$(exec_anchor "$ak" "$create"); crc=$?
  echo "create_rc=$crc create_out=$(echo "$cout" | tr '\n' '|')"
  out=$(exec_anchor "$ak" "$set"); rc=$?
  echo "rc=$rc out=$(echo "$out" | tr '\n' '|')"
  cols=$(exec_anchor "$ak" "SELECT GROUP_CONCAT(CONCAT(column_name,'=',IFNULL(column_default,'<NULL>')) ORDER BY ordinal_position) FROM information_schema.columns WHERE table_schema='golden' AND table_name='$tbl'")
  echo "cols=$cols"
  exec_anchor "$ak" "DROP TABLE IF EXISTS $tbl" >/dev/null
  post=$(count0 "$ak" "$tbl")
  echo "post=$post"
}
PRIOR="CREATE TABLE %s (v VARCHAR(36), x INT)"
for ak in tidb85 mysql80 mysql84; do
  run_def D_UUID     "$ak" t_r1d_uuid     "CREATE TABLE t_r1d_uuid (v VARCHAR(36), x INT)"     "ALTER TABLE t_r1d_uuid ALTER COLUMN v SET DEFAULT (uuid())"
  run_def D_ARITH    "$ak" t_r1d_arith    "CREATE TABLE t_r1d_arith (v VARCHAR(36), x INT)"    "ALTER TABLE t_r1d_arith ALTER COLUMN v SET DEFAULT (uuid() + 1)"
  run_def D_CASE     "$ak" t_r1d_case     "CREATE TABLE t_r1d_case (v VARCHAR(36), x INT)"     "ALTER TABLE t_r1d_case ALTER COLUMN v SET DEFAULT (CASE WHEN 1 THEN uuid() ELSE 'x' END)"
  run_def D_PARENS   "$ak" t_r1d_parens   "CREATE TABLE t_r1d_parens (v VARCHAR(36), x INT)"   "ALTER TABLE t_r1d_parens ALTER COLUMN v SET DEFAULT ((uuid()))"
  run_def D_CONSTANT "$ak" t_r1d_const    "CREATE TABLE t_r1d_const (v VARCHAR(36), x INT)"    "ALTER TABLE t_r1d_const ALTER COLUMN v SET DEFAULT (1 + 2)"
done
# CREATE-side same-root checks on TiDB
run_def C_JSON_OK    tidb85 t_r1c_json    "CREATE TABLE t_r1c_json (j JSON DEFAULT (JSON_OBJECT('k',1)))"      "SELECT 1"
run_def C_JSON_CASE  tidb85 t_r1c_jcase   "CREATE TABLE t_r1c_jcase (j JSON DEFAULT (CASE WHEN 1 THEN json_object('k',1) ELSE 'x' END))" "SELECT 1"
run_def C_UUID_CREATE tidb85 t_r1c_uuid   "CREATE TABLE t_r1c_uuid (v VARCHAR(36) DEFAULT (uuid()))"           "SELECT 1"
run_def C_UUID_WRAP  tidb85 t_r1c_uuidw   "CREATE TABLE t_r1c_uuidw (v VARCHAR(36) DEFAULT (uuid() + 1))"      "SELECT 1"
run_def C_UUID_CCASE tidb85 t_r1c_uuidc   "CREATE TABLE t_r1c_uuidc (v VARCHAR(36) DEFAULT (CASE WHEN 1 THEN uuid() ELSE 'x' END))" "SELECT 1"
