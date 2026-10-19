#!/bin/bash
source /tmp/ds-t07r1/native_gen.sh
exec_tidb() { docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot --default-character-set=utf8mb4 -Nse "$1" golden 2>&1; }
setprobe() { # label tbl set-sql [prior-sql]
  local tbl="$2" prior="${4:-CREATE TABLE $2 (v VARCHAR(36), x INT)}"
  exec_tidb "DROP TABLE IF EXISTS $tbl" >/dev/null
  exec_tidb "$prior" >/dev/null
  out=$(exec_tidb "$3"); rc=$?
  cols=$(exec_tidb "SELECT GROUP_CONCAT(CONCAT(column_name,'=',IFNULL(column_default,'<NULL>'))) FROM information_schema.columns WHERE table_schema='golden' AND table_name='$tbl'")
  echo "### $1 rc=$rc out=$(echo "$out"|tr '\n' '|') cols=$cols"
  exec_tidb "DROP TABLE IF EXISTS $tbl" >/dev/null
}
setprobe S_JSON     t_r1s_json    "ALTER TABLE t_r1s_json ALTER COLUMN v SET DEFAULT (json_object('k',1))"
setprobe S_XREF     t_r1s_xref    "ALTER TABLE t_r1s_xref ALTER COLUMN v SET DEFAULT (x + 1)"
setprobe S_UUID_INT t_r1s_uuidint "ALTER TABLE t_r1s_uuidint ALTER COLUMN x SET DEFAULT (uuid())"
setprobe S_UUID_INT2 t_r1s_uuidi2 "ALTER TABLE t_r1s_uuidi2 ALTER COLUMN v SET DEFAULT (uuid())" "CREATE TABLE t_r1s_uuidi2 (v INT, x INT)"
setprobe S_LOWER    t_r1s_lower   "ALTER TABLE t_r1s_lower ALTER COLUMN v SET DEFAULT (lower('X'))"
setprobe S_SUBQ     t_r1s_subq    "ALTER TABLE t_r1s_subq ALTER COLUMN v SET DEFAULT ((SELECT 1))"
setprobe S_CASEX    t_r1s_casex   "ALTER TABLE t_r1s_casex ALTER COLUMN v SET DEFAULT (CASE WHEN x>0 THEN 'a' ELSE 'b' END)"
setprobe S_LIT      t_r1s_lit     "ALTER TABLE t_r1s_lit ALTER COLUMN v SET DEFAULT ('abc')"
setprobe S_LITNUM   t_r1s_litnum  "ALTER TABLE t_r1s_litnum ALTER COLUMN v SET DEFAULT (42)"
setprobe S_UUID_ARG t_r1s_uarg    "ALTER TABLE t_r1s_uarg ALTER COLUMN v SET DEFAULT (concat(uuid(),'-'))"
setprobe S_JSON_ONJSON t_r1s_jj "ALTER TABLE t_r1s_jj ALTER COLUMN j SET DEFAULT (json_object('k',1))" "CREATE TABLE t_r1s_jj (j JSON, x INT)"
setprobe S_UUID_ONJSON t_r1s_uj "ALTER TABLE t_r1s_uj ALTER COLUMN j SET DEFAULT (uuid())" "CREATE TABLE t_r1s_uj (j JSON, x INT)"
setprobe S_JARR_ONJ    t_r1s_ja "ALTER TABLE t_r1s_ja ALTER COLUMN j SET DEFAULT (json_array(1))" "CREATE TABLE t_r1s_ja (j JSON, x INT)"
setprobe S_UUID_ONDEC  t_r1s_ud "ALTER TABLE t_r1s_ud ALTER COLUMN v SET DEFAULT (uuid())" "CREATE TABLE t_r1s_ud (v DECIMAL(10,2), x INT)"
setprobe S_JSON_ONINT  t_r1s_ji "ALTER TABLE t_r1s_ji ALTER COLUMN x SET DEFAULT (json_object('k',1))" "CREATE TABLE t_r1s_ji (j JSON, x INT)"
