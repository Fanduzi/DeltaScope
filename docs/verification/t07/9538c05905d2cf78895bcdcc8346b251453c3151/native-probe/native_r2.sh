#!/bin/bash
# T07-R2 bounded native confirmation: the frozen F01-F08 setter cells on the
# pinned TiDB 8.5.0 anchor plus the MySQL 8.4 F01/F04 opposite controls.
# Every role uses a dedicated, uniquely-named table (v INT, x INT) inside the
# throwaway r2probe schema. Procedure per role: absence check -> CREATE ->
# two-column identity check -> SET with real rc -> catalog read -> cleanup of
# only this run's table -> final COUNT=0. A pre-existing table aborts the
# role as a collision (never DROP IF EXISTS on unknown ownership).
# Credentials ride on MYSQL_PWD / container env, never argv.
# Requires: deltascope-ddl-golden-{tidb85,-client,mysql84} running.
set -u
SCHEMA=r2probe

exec_tidb() { docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot --default-character-set=utf8mb4 -Nse "$1" $SCHEMA 2>&1; }
exec_m84()  { docker exec -e MYSQL_PWD=root deltascope-ddl-golden-mysql84 mysql -uroot --default-character-set=utf8mb4 -Nse "$1" $SCHEMA 2>&1; }
q_tidb()    { docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot --default-character-set=utf8mb4 -Nse "$1" 2>&1; }
q_m84()     { docker exec -e MYSQL_PWD=root deltascope-ddl-golden-mysql84 mysql -uroot --default-character-set=utf8mb4 -Nse "$1" 2>&1; }

runner() { if [ "$1" = tidb85 ]; then exec_tidb "$2"; else exec_m84 "$2"; fi; }
query()  { if [ "$1" = tidb85 ]; then q_tidb "$2"; else q_m84 "$2"; fi; }

echo "=== session ==="
q_tidb "SELECT VERSION(), @@sql_mode, @@character_set_client"
q_m84  "SELECT VERSION(), @@sql_mode, @@character_set_client"
for a in tidb85 mysql84; do query $a "CREATE DATABASE IF NOT EXISTS $SCHEMA"; done

created=0
role() {
  local anchor="$1" name="$2" setexpr="$3"
  local tbl="t_r2_$(echo "$2" | tr 'A-Z' 'a-z')"
  # 1. absence
  local cnt; cnt=$(query "$anchor" "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='$SCHEMA' AND table_name='$tbl'"); local qrc=$?
  if [ $qrc -ne 0 ]; then echo "### $anchor/$name ABORT presence-query-failed: $(echo "$cnt"|tr '\n' '|')"; return; fi
  if [ "$cnt" != "0" ]; then echo "### $anchor/$name COLLISION count=$cnt"; return; fi
  # 2. create
  local out; out=$(runner "$anchor" "CREATE TABLE $tbl (v INT, x INT)"); local crc=$?
  if [ $crc -ne 0 ]; then echo "### $anchor/$name CREATE-FAILED rc=$crc out=$(echo "$out"|tr '\n' '|')"; return; fi
  created=$((created+1))
  # 3. identity
  local id; id=$(query "$anchor" "SELECT GROUP_CONCAT(CONCAT(column_name,':',column_type) ORDER BY ordinal_position SEPARATOR '|') FROM information_schema.columns WHERE table_schema='$SCHEMA' AND table_name='$tbl'"); local irc=$?
  if [ $irc -ne 0 ] || [ "$id" != "v:int|x:int" ]; then
    echo "### $anchor/$name IDENTITY-FAIL rc=$irc id=$id"; runner "$anchor" "DROP TABLE $tbl" >/dev/null; return
  fi
  # 4. set + catalog
  out=$(runner "$anchor" "ALTER TABLE $tbl ALTER COLUMN v SET DEFAULT $setexpr"); local src=$?
  local cat; cat=$(query "$anchor" "SELECT CONCAT('def=',IFNULL(column_default,'<NULL>'),'|hex=',IFNULL(HEX(column_default),'<NULL>'),'|extra=',extra) FROM information_schema.columns WHERE table_schema='$SCHEMA' AND table_name='$tbl' AND column_name='v'"); local catrc=$?
  if [ $catrc -ne 0 ]; then cat="CATALOG-QUERY-FAILED rc=$catrc out=$(echo "$cat"|tr '\n' '|')"; fi
  echo "### $anchor/$name set_rc=$src set_out=$(echo "$out"|tr '\n' '|') $cat"
  # 5. cleanup own table + final count
  out=$(runner "$anchor" "DROP TABLE $tbl"); local drc=$?
  local left; left=$(query "$anchor" "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='$SCHEMA' AND table_name='$tbl'")
  echo "### $anchor/$name cleanup drop_rc=$drc final_count=$left"
}

echo "=== TiDB 8.5.0 F-roles ==="
role tidb85 F01 "(CASE WHEN 1 THEN 1 ELSE 2 END)"
role tidb85 F02 "(CASE WHEN 1 THEN 1 ELSE uuid() END)"
role tidb85 F03 "(CASE WHEN 1 THEN uuid() ELSE 'x' END)"
role tidb85 F04 "((LOWER('1')))"
role tidb85 F05 "((LOWER('X')))"
role tidb85 F06 "(1 + 2)"
role tidb85 F07 "('X' + 1)"
role tidb85 F08 "(uuid())"
echo "=== MySQL 8.4 opposite controls ==="
role mysql84 F01 "(CASE WHEN 1 THEN 1 ELSE 2 END)"
role mysql84 F04 "((LOWER('1')))"
echo "=== done created=$created ==="
