#!/bin/bash
# Bounded native confirmation: R1 SET-DEFAULT fold/temporal/parens boundary cells
# not captured in earlier rounds. Owned tables only: t_r1p_* / t_r1q_*.
source /tmp/ds-t07r1/native_gen.sh
exec_tidb() { docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot --default-character-set=utf8mb4 -Nse "$1" golden 2>&1; }
exec_anchor() { if [ "$1" = tidb85 ]; then exec_tidb "$2"; else docker exec -e MYSQL_PWD=root "deltascope-ddl-golden-$1" mysql -uroot --default-character-set=utf8mb4 -Nse "$2" golden 2>&1; fi; }

sp() { # label anchor prior-sql set-sql
  local lbl="$1" ak="$2" prior="$3" set="$4" tbl="t_r1p_$(echo "$1" | tr 'A-Z' 'a-z')"
  exec_anchor "$ak" "DROP TABLE IF EXISTS $tbl" >/dev/null
  exec_anchor "$ak" "$prior" >/dev/null
  out=$(exec_anchor "$ak" "$set"); rc=$?
  cols=$(exec_anchor "$ak" "SELECT GROUP_CONCAT(CONCAT(column_name,'=',IFNULL(column_default,'<NULL>')) ORDER BY ordinal_position) FROM information_schema.columns WHERE table_schema='golden' AND table_name='$tbl'")
  echo "### $lbl $ak rc=$rc out=$(echo "$out" | tr '\n' '|') cols=$cols"
  exec_anchor "$ak" "DROP TABLE IF EXISTS $tbl" >/dev/null
}
cp() { # label anchor create-sql
  local lbl="$1" ak="$2" sql="$3" tbl="t_r1q_$(echo "$1" | tr 'A-Z' 'a-z')"
  exec_anchor "$ak" "DROP TABLE IF EXISTS $tbl" >/dev/null
  out=$(exec_anchor "$ak" "$sql"); rc=$?
  cols=$(exec_anchor "$ak" "SELECT GROUP_CONCAT(CONCAT(column_name,'=',IFNULL(column_default,'<NULL>')) ORDER BY ordinal_position) FROM information_schema.columns WHERE table_schema='golden' AND table_name='$tbl'")
  echo "### $lbl $ak rc=$rc out=$(echo "$out" | tr '\n' '|') cols=$cols"
  exec_anchor "$ak" "DROP TABLE IF EXISTS $tbl" >/dev/null
}

# --- foldable timestamp type gates (contract: ts/datetime/date/varchar accept; numeric 1067)
sp FTS_TS    tidb85 "CREATE TABLE t_r1p_fts_ts (v TIMESTAMP NULL)" \
  "ALTER TABLE t_r1p_fts_ts ALTER COLUMN v SET DEFAULT (current_timestamp)"
sp FTS_DT    tidb85 "CREATE TABLE t_r1p_fts_dt (v DATETIME NULL)" \
  "ALTER TABLE t_r1p_fts_dt ALTER COLUMN v SET DEFAULT (current_timestamp)"
sp FTS_DATE  tidb85 "CREATE TABLE t_r1p_fts_date (v DATE NULL)" \
  "ALTER TABLE t_r1p_fts_date ALTER COLUMN v SET DEFAULT (current_timestamp)"
sp FTS_VC    tidb85 "CREATE TABLE t_r1p_fts_vc (v VARCHAR(36))" \
  "ALTER TABLE t_r1p_fts_vc ALTER COLUMN v SET DEFAULT (current_timestamp)"
sp FTS_INT   tidb85 "CREATE TABLE t_r1p_fts_int (v INT)" \
  "ALTER TABLE t_r1p_fts_int ALTER COLUMN v SET DEFAULT (current_timestamp)"
# --- constant arithmetic on DATE (claimed fold-positive target)
sp C_ARITH_DATE tidb85 "CREATE TABLE t_r1p_c_arith_date (v DATE NULL)" \
  "ALTER TABLE t_r1p_c_arith_date ALTER COLUMN v SET DEFAULT (1 + 2)"
# --- case+uuid on INT (claimed 1067)
sp CASE_UUID_INT tidb85 "CREATE TABLE t_r1p_case_uuid_int (v INT)" \
  "ALTER TABLE t_r1p_case_uuid_int ALTER COLUMN v SET DEFAULT (CASE WHEN 1 THEN uuid() ELSE 'x' END)"
# --- double-paren wrapped calls
sp DP_UUID_VC  tidb85 "CREATE TABLE t_r1p_dp_uuid_vc (v VARCHAR(36))" \
  "ALTER TABLE t_r1p_dp_uuid_vc ALTER COLUMN v SET DEFAULT ((uuid()))"
sp DP_UUID_INT tidb85 "CREATE TABLE t_r1p_dp_uuid_int (v INT)" \
  "ALTER TABLE t_r1p_dp_uuid_int ALTER COLUMN v SET DEFAULT ((uuid()))"
sp DP_JSON_VC  tidb85 "CREATE TABLE t_r1p_dp_json_vc (v VARCHAR(64))" \
  "ALTER TABLE t_r1p_dp_json_vc ALTER COLUMN v SET DEFAULT ((json_object('k',1)))"
sp DP_CTS_TS   tidb85 "CREATE TABLE t_r1p_dp_cts_ts (v TIMESTAMP NULL)" \
  "ALTER TABLE t_r1p_dp_cts_ts ALTER COLUMN v SET DEFAULT ((current_timestamp))"
sp DP_ARITH_VC tidb85 "CREATE TABLE t_r1p_dp_arith_vc (v VARCHAR(36))" \
  "ALTER TABLE t_r1p_dp_arith_vc ALTER COLUMN v SET DEFAULT ((1 + 2))"
# --- bare (non-parens) setter forms: 1064 claimed on all three anchors
for ak in mysql80 mysql84 tidb85; do
  sp BARE_UUID_$ak $ak "CREATE TABLE t_r1p_bare_uuid_$ak (v VARCHAR(36))" \
    "ALTER TABLE t_r1p_bare_uuid_$ak ALTER COLUMN v SET DEFAULT uuid()"
done
sp BARE_NOW_TS tidb85 "CREATE TABLE t_r1p_bare_now_ts (v TIMESTAMP NULL)" \
  "ALTER TABLE t_r1p_bare_now_ts ALTER COLUMN v SET DEFAULT NOW()"
# --- bare CREATE DEFAULT now() on TIMESTAMP (claimed positive, all anchors)
for ak in mysql57 mysql80 mysql84 tidb85; do
  cp BARE_CREATE_NOW_$ak $ak "CREATE TABLE t_r1q_bare_create_now_$ak (v TIMESTAMP NULL DEFAULT NOW())"
done
# --- CREATE double-parens
cp CDP_UUID tidb85 "CREATE TABLE t_r1q_cdp_uuid (v VARCHAR(36) DEFAULT ((uuid())))"
