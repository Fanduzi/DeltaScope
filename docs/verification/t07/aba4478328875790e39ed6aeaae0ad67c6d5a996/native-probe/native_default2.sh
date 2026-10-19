#!/bin/bash
source /tmp/ds-t07r1/native_default.sh >/dev/null 2>&1 || source /tmp/ds-t07r1/native_gen.sh
exec_tidb() { docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot --default-character-set=utf8mb4 -Nse "$1" golden 2>&1; }
probe() { # label create-sql
  local tbl="t_r1x_$(echo "$1" | tr 'A-Z' 'a-z')"
  docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot --default-character-set=utf8mb4 -Nse "DROP TABLE IF EXISTS $tbl" golden >/dev/null 2>&1
  out=$(exec_tidb "$2"); rc=$?
  cols=$(exec_tidb "SELECT GROUP_CONCAT(CONCAT(column_name,'=',IFNULL(column_default,'<NULL>'))) FROM information_schema.columns WHERE table_schema='golden' AND table_name='$tbl'")
  echo "### $1 rc=$rc out=$(echo "$out"|tr '\n' '|') cols=$cols"
  exec_tidb "DROP TABLE IF EXISTS $tbl" >/dev/null
}
probe P_ARITH   "CREATE TABLE t_r1x_p_arith (v INT DEFAULT (1 + 2))"
probe P_LIT     "CREATE TABLE t_r1x_p_lit (v VARCHAR(8) DEFAULT ('abc'))"
probe P_NUM     "CREATE TABLE t_r1x_p_num (v INT DEFAULT (7))"
probe P_UUID    "CREATE TABLE t_r1x_p_uuid (v VARCHAR(36) DEFAULT (uuid()))"
probe P_UUIDS   "CREATE TABLE t_r1x_p_uuids (v VARCHAR(36) DEFAULT (uuid_short()))"
probe P_JARR    "CREATE TABLE t_r1x_p_jarr (j JSON DEFAULT (json_array()))"
probe P_NOW     "CREATE TABLE t_r1x_p_now (v VARCHAR(36) DEFAULT (now()))"
probe P_WRAPFN  "CREATE TABLE t_r1x_p_wrapfn (v VARCHAR(64) DEFAULT (lower(uuid())))"
probe P_JWRAP   "CREATE TABLE t_r1x_p_jwrap (j JSON DEFAULT (json_object('k', json_array(1))))"
