#!/bin/bash
# T07-R1 native setter-cell sweeps against the pinned TiDB 8.5.0 anchor.
# Each cell records: ALTER rc, native error text, and the catalog-visible
# stored default — the (callee x authored-root x target-base) outcomes the
# assessment tables are bound to. Tables are created/dropped inside the
# throwaway `r1probe` schema; fixture credentials never appear in argv.
# Requires: deltascope-ddl-golden-tidb85{,-client} containers running.
set -u
exec_tidb() { docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot --default-character-set=utf8mb4 -Nse "$1" r1probe 2>&1; }
docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot -Nse "CREATE DATABASE IF NOT EXISTS r1probe"
sp() { local lbl="$1" prior="$2" set="$3" tbl="t_r1p_$(echo "$1" | tr 'A-Z' 'a-z')"
  exec_tidb "DROP TABLE IF EXISTS $tbl" >/dev/null; exec_tidb "$prior" >/dev/null
  out=$(exec_tidb "$set"); rc=$?
  cols=$(exec_tidb "SELECT GROUP_CONCAT(CONCAT(column_name,'=',IFNULL(column_default,'<NULL>')) ORDER BY ordinal_position) FROM information_schema.columns WHERE table_schema='r1probe' AND table_name='$tbl'")
  echo "### $lbl rc=$rc out=$(echo "$out"|tr '\n' '|') cols=$cols"; exec_tidb "DROP TABLE IF EXISTS $tbl" >/dev/null; }

# --- bare-call verbatim sweep: uuid() on every probed non-1101 family -----
sp U_CHAR  "CREATE TABLE t_r1p_u_char (v CHAR(36), x INT)"  "ALTER TABLE t_r1p_u_char ALTER COLUMN v SET DEFAULT (uuid())"
sp U_BIGINT "CREATE TABLE t_r1p_u_bigint (v BIGINT, x INT)" "ALTER TABLE t_r1p_u_bigint ALTER COLUMN v SET DEFAULT (uuid())"
sp U_DOUBLE "CREATE TABLE t_r1p_u_double (v DOUBLE, x INT)" "ALTER TABLE t_r1p_u_double ALTER COLUMN v SET DEFAULT (uuid())"
sp U_DATE  "CREATE TABLE t_r1p_u_date (v DATE, x INT)"     "ALTER TABLE t_r1p_u_date ALTER COLUMN v SET DEFAULT (uuid())"
sp U_TS    "CREATE TABLE t_r1p_u_ts (v TIMESTAMP NULL, x INT)" "ALTER TABLE t_r1p_u_ts ALTER COLUMN v SET DEFAULT (uuid())"
sp U_DT    "CREATE TABLE t_r1p_u_dt (v DATETIME, x INT)"   "ALTER TABLE t_r1p_u_dt ALTER COLUMN v SET DEFAULT (uuid())"
sp U_YEAR  "CREATE TABLE t_r1p_u_year (v YEAR, x INT)"     "ALTER TABLE t_r1p_u_year ALTER COLUMN v SET DEFAULT (uuid())"
sp U_BIT   "CREATE TABLE t_r1p_u_bit (v BIT(8), x INT)"    "ALTER TABLE t_r1p_u_bit ALTER COLUMN v SET DEFAULT (uuid())"
sp U_ENUM  "CREATE TABLE t_r1p_u_enum (v ENUM('a','b'), x INT)" "ALTER TABLE t_r1p_u_enum ALTER COLUMN v SET DEFAULT (uuid())"
sp U_TEXT  "CREATE TABLE t_r1p_u_text (v TEXT, x INT)"     "ALTER TABLE t_r1p_u_text ALTER COLUMN v SET DEFAULT (uuid())"
sp U_FLOAT "CREATE TABLE t_r1p_u_float (v FLOAT, x INT)"   "ALTER TABLE t_r1p_u_float ALTER COLUMN v SET DEFAULT (uuid())"
sp U_TIME  "CREATE TABLE t_r1p_u_time (v TIME, x INT)"     "ALTER TABLE t_r1p_u_time ALTER COLUMN v SET DEFAULT (uuid())"
sp U_SET   "CREATE TABLE t_r1p_u_set (v SET('a','b'), x INT)" "ALTER TABLE t_r1p_u_set ALTER COLUMN v SET DEFAULT (uuid())"
sp U_BIN   "CREATE TABLE t_r1p_u_bin (v VARBINARY(36), x INT)" "ALTER TABLE t_r1p_u_bin ALTER COLUMN v SET DEFAULT (uuid())"
sp U_TINY  "CREATE TABLE t_r1p_u_tiny (v TINYINT, x INT)"  "ALTER TABLE t_r1p_u_tiny ALTER COLUMN v SET DEFAULT (uuid())"

# --- bare-call verbatim: json_object / json_array targets ----------------
sp J_DEC   "CREATE TABLE t_r1p_j_dec (v DECIMAL(10,2), x INT)" "ALTER TABLE t_r1p_j_dec ALTER COLUMN v SET DEFAULT (json_object('k',1))"
sp J_DATE  "CREATE TABLE t_r1p_j_date (v DATE, x INT)"     "ALTER TABLE t_r1p_j_date ALTER COLUMN v SET DEFAULT (json_object('k',1))"
sp J_TS    "CREATE TABLE t_r1p_j_ts (v TIMESTAMP NULL, x INT)" "ALTER TABLE t_r1p_j_ts ALTER COLUMN v SET DEFAULT (json_object('k',1))"
sp JA_INT  "CREATE TABLE t_r1p_ja_int (v INT, x INT)"      "ALTER TABLE t_r1p_ja_int ALTER COLUMN v SET DEFAULT (json_array(1))"
sp S_JSON_ONINT "CREATE TABLE t_r1p_s_json_onint (j JSON, x INT)" "ALTER TABLE t_r1p_s_json_onint ALTER COLUMN x SET DEFAULT (json_object('k',1))"
sp S_JSON_ONJSON "CREATE TABLE t_r1p_s_json_onjson (j JSON, x INT)" "ALTER TABLE t_r1p_s_json_onjson ALTER COLUMN j SET DEFAULT (json_object('k',1))"
sp S_UUID_ONJSON "CREATE TABLE t_r1p_s_uuid_onjson (j JSON, x INT)" "ALTER TABLE t_r1p_s_uuid_onjson ALTER COLUMN j SET DEFAULT (uuid())"
sp S_JARR_ONJ    "CREATE TABLE t_r1p_s_jarr_onj (j JSON, x INT)" "ALTER TABLE t_r1p_s_jarr_onj ALTER COLUMN j SET DEFAULT (json_array(1))"
sp S_UUID_ONDEC  "CREATE TABLE t_r1p_s_uuid_ondec (v DECIMAL(10,2), x INT)" "ALTER TABLE t_r1p_s_uuid_ondec ALTER COLUMN v SET DEFAULT (uuid())"

# --- wrapped-call fold: ((uuid())) / ((lower('x'))) target outcomes ------
sp A_INT   "CREATE TABLE t_r1p_a_int (v INT, x INT)"       "ALTER TABLE t_r1p_a_int ALTER COLUMN v SET DEFAULT ((uuid()))"
sp A_BIGINT "CREATE TABLE t_r1p_a_bigint (v BIGINT, x INT)" "ALTER TABLE t_r1p_a_bigint ALTER COLUMN v SET DEFAULT ((uuid()))"
sp A_DATE  "CREATE TABLE t_r1p_a_date (v DATE, x INT)"     "ALTER TABLE t_r1p_a_date ALTER COLUMN v SET DEFAULT ((uuid()))"
sp A_DEC   "CREATE TABLE t_r1p_a_dec (v DECIMAL(10,2), x INT)" "ALTER TABLE t_r1p_a_dec ALTER COLUMN v SET DEFAULT ((uuid()))"
sp A_TS    "CREATE TABLE t_r1p_a_ts (v TIMESTAMP NULL, x INT)" "ALTER TABLE t_r1p_a_ts ALTER COLUMN v SET DEFAULT ((uuid()))"
sp A_CHAR  "CREATE TABLE t_r1p_a_char (v CHAR(36), x INT)" "ALTER TABLE t_r1p_a_char ALTER COLUMN v SET DEFAULT ((uuid()))"
sp A_ENUM  "CREATE TABLE t_r1p_a_enum (v ENUM('a','b'), x INT)" "ALTER TABLE t_r1p_a_enum ALTER COLUMN v SET DEFAULT ((uuid()))"
sp A_SET   "CREATE TABLE t_r1p_a_set (v SET('a','b'), x INT)" "ALTER TABLE t_r1p_a_set ALTER COLUMN v SET DEFAULT ((uuid()))"
sp A_BIN   "CREATE TABLE t_r1p_a_bin (v VARBINARY(36), x INT)" "ALTER TABLE t_r1p_a_bin ALTER COLUMN v SET DEFAULT ((uuid()))"
sp A_TIME  "CREATE TABLE t_r1p_a_time (v TIME, x INT)"     "ALTER TABLE t_r1p_a_time ALTER COLUMN v SET DEFAULT ((uuid()))"
sp A_TINY  "CREATE TABLE t_r1p_a_tiny (v TINYINT, x INT)"  "ALTER TABLE t_r1p_a_tiny ALTER COLUMN v SET DEFAULT ((uuid()))"
sp A_FLOAT "CREATE TABLE t_r1p_a_float (v FLOAT, x INT)"   "ALTER TABLE t_r1p_a_float ALTER COLUMN v SET DEFAULT ((uuid()))"
sp A_DBL   "CREATE TABLE t_r1p_a_dbl (v DOUBLE, x INT)"    "ALTER TABLE t_r1p_a_dbl ALTER COLUMN v SET DEFAULT ((uuid()))"
sp A_BINF  "CREATE TABLE t_r1p_a_binf (v BINARY(36), x INT)" "ALTER TABLE t_r1p_a_binf ALTER COLUMN v SET DEFAULT ((uuid()))"
sp A_YEAR  "CREATE TABLE t_r1p_a_year (v YEAR, x INT)"     "ALTER TABLE t_r1p_a_year ALTER COLUMN v SET DEFAULT ((uuid()))"
sp A_BIT   "CREATE TABLE t_r1p_a_bit (v BIT(8), x INT)"    "ALTER TABLE t_r1p_a_bit ALTER COLUMN v SET DEFAULT ((uuid()))"
sp A_DT    "CREATE TABLE t_r1p_a_dt (v DATETIME, x INT)"   "ALTER TABLE t_r1p_a_dt ALTER COLUMN v SET DEFAULT ((uuid()))"
sp A_LOWVC "CREATE TABLE t_r1p_a_lowvc (v VARCHAR(8), x INT)" "ALTER TABLE t_r1p_a_lowvc ALTER COLUMN v SET DEFAULT ((lower('X')))"
sp A_LOWINT "CREATE TABLE t_r1p_a_lowint (v INT, x INT)"   "ALTER TABLE t_r1p_a_lowint ALTER COLUMN v SET DEFAULT ((lower('X')))"

# --- constant-arithmetic fold: (1 + 2) target outcomes -------------------
sp C_ARITH_INT "CREATE TABLE t_r1p_c_arith_int (v INT)"    "ALTER TABLE t_r1p_c_arith_int ALTER COLUMN v SET DEFAULT (1 + 2)"
sp C_CHAR  "CREATE TABLE t_r1p_c_char (v CHAR(8), x INT)"  "ALTER TABLE t_r1p_c_char ALTER COLUMN v SET DEFAULT (1 + 2)"
sp C_BIGINT "CREATE TABLE t_r1p_c_bigint (v BIGINT, x INT)" "ALTER TABLE t_r1p_c_bigint ALTER COLUMN v SET DEFAULT (1 + 2)"
sp C_DEC   "CREATE TABLE t_r1p_c_dec (v DECIMAL(10,2), x INT)" "ALTER TABLE t_r1p_c_dec ALTER COLUMN v SET DEFAULT (1 + 2)"
sp C_FLOAT "CREATE TABLE t_r1p_c_float (v FLOAT, x INT)"   "ALTER TABLE t_r1p_c_float ALTER COLUMN v SET DEFAULT (1 + 2)"
sp C_TIME  "CREATE TABLE t_r1p_c_time (v TIME, x INT)"     "ALTER TABLE t_r1p_c_time ALTER COLUMN v SET DEFAULT (1 + 2)"
sp C_TS    "CREATE TABLE t_r1p_c_ts (v TIMESTAMP NULL, x INT)" "ALTER TABLE t_r1p_c_ts ALTER COLUMN v SET DEFAULT (1 + 2)"
sp C_ENUM  "CREATE TABLE t_r1p_c_enum (v ENUM('1','3','5'), x INT)" "ALTER TABLE t_r1p_c_enum ALTER COLUMN v SET DEFAULT (1 + 2)"
sp C_SET   "CREATE TABLE t_r1p_c_set (v SET('1','3','5'), x INT)" "ALTER TABLE t_r1p_c_set ALTER COLUMN v SET DEFAULT (1 + 2)"
sp C_BIN   "CREATE TABLE t_r1p_c_bin (v VARBINARY(8), x INT)" "ALTER TABLE t_r1p_c_bin ALTER COLUMN v SET DEFAULT (1 + 2)"
sp C_DT    "CREATE TABLE t_r1p_c_dt (v DATETIME, x INT)"   "ALTER TABLE t_r1p_c_dt ALTER COLUMN v SET DEFAULT (1 + 2)"
sp C_DBL   "CREATE TABLE t_r1p_c_dbl (v DOUBLE, x INT)"    "ALTER TABLE t_r1p_c_dbl ALTER COLUMN v SET DEFAULT (1 + 2)"
sp C_YEAR  "CREATE TABLE t_r1p_c_year (v YEAR, x INT)"     "ALTER TABLE t_r1p_c_year ALTER COLUMN v SET DEFAULT (1 + 2)"
sp C_BIT   "CREATE TABLE t_r1p_c_bit (v BIT(8), x INT)"    "ALTER TABLE t_r1p_c_bit ALTER COLUMN v SET DEFAULT (1 + 2)"
sp ARITH_CMP_VC "CREATE TABLE t_r1p_arith_cmp_vc (v VARCHAR(8))" "ALTER TABLE t_r1p_arith_cmp_vc ALTER COLUMN v SET DEFAULT (1 < 2)"

# --- foldable current_timestamp target outcomes --------------------------
sp CT_CHAR "CREATE TABLE t_r1p_ct_char (v CHAR(30), x INT)" "ALTER TABLE t_r1p_ct_char ALTER COLUMN v SET DEFAULT (current_timestamp)"
sp CT_DBL  "CREATE TABLE t_r1p_ct_dbl (v DOUBLE, x INT)"   "ALTER TABLE t_r1p_ct_dbl ALTER COLUMN v SET DEFAULT (current_timestamp)"
sp CT_INT  "CREATE TABLE t_r1p_ct_int (v INT, x INT)"      "ALTER TABLE t_r1p_ct_int ALTER COLUMN v SET DEFAULT (current_timestamp)"
sp CT_BIG  "CREATE TABLE t_r1p_ct_big (v BIGINT, x INT)"   "ALTER TABLE t_r1p_ct_big ALTER COLUMN v SET DEFAULT (current_timestamp)"
sp CT_DEC  "CREATE TABLE t_r1p_ct_dec (v DECIMAL(20,2), x INT)" "ALTER TABLE t_r1p_ct_dec ALTER COLUMN v SET DEFAULT (current_timestamp)"
sp CT_TIME "CREATE TABLE t_r1p_ct_time (v TIME, x INT)"    "ALTER TABLE t_r1p_ct_time ALTER COLUMN v SET DEFAULT (current_timestamp)"
sp CT_YR   "CREATE TABLE t_r1p_ct_yr (v YEAR, x INT)"      "ALTER TABLE t_r1p_ct_yr ALTER COLUMN v SET DEFAULT (current_timestamp)"

# --- uuid-CASE fold controls ---------------------------------------------
sp CS_BIG  "CREATE TABLE t_r1p_cs_big (v BIGINT, x INT)"   "ALTER TABLE t_r1p_cs_big ALTER COLUMN v SET DEFAULT (CASE WHEN 1 THEN uuid() ELSE 'x' END)"
sp CS_CHAR "CREATE TABLE t_r1p_cs_char (v CHAR(40), x INT)" "ALTER TABLE t_r1p_cs_char ALTER COLUMN v SET DEFAULT (CASE WHEN 1 THEN uuid() ELSE 'x' END)"

# --- double-wrapped NULL-accept cells ------------------------------------
sp DP_LOWER_VC "CREATE TABLE t_r1p_dp_lower_vc (v VARCHAR(36))" "ALTER TABLE t_r1p_dp_lower_vc ALTER COLUMN v SET DEFAULT ((lower('X')))"
sp DP_NOW_TS  "CREATE TABLE t_r1p_dp_now_ts (v TIMESTAMP NULL)" "ALTER TABLE t_r1p_dp_now_ts ALTER COLUMN v SET DEFAULT ((now()))"
