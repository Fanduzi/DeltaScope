#!/bin/bash
# T13-R2 probes — every arm resets its fixture; every step logs the full
# SQL, real rc, stderr, and the before/after member catalog + row state.
# Anchors: mysql57 5.7.44 / mysql80 8.0.46 / mysql84 8.4.10 / tidb85 v8.5.0.
set -u

run_sql() {
  local anchor="$1" name="$2" sql="$3"
  echo "=== [$anchor] $name"
  echo "--- SQL: $sql"
  local out rc
  case "$anchor" in
    tidb85)
      out=$(docker exec deltascope-ddl-golden-tidb85-client \
        mysql --protocol=tcp -h tidb85 -P 4000 -uroot \
        --default-character-set=utf8mb4 -e "$sql" 2>&1)
      rc=$?
      ;;
    *)
      out=$(docker exec --env MYSQL_PWD=root deltascope-ddl-golden-$anchor \
        mysql -uroot --default-character-set=utf8mb4 -e "$sql" 2>&1)
      rc=$?
      ;;
  esac
  echo "--- rc=$rc"
  echo "--- out: $out"
}

ANCHORS="mysql57 mysql80 mysql84 tidb85"

for a in $ANCHORS; do
  echo "############ ANCHOR $a ############"
  run_sql "$a" version "SELECT VERSION()"
  [ "$a" = "tidb85" ] && run_sql "$a" ensure-db "CREATE DATABASE IF NOT EXISTS golden"
  run_sql "$a" cleanup "DROP TABLE IF EXISTS golden.t13e; DROP TABLE IF EXISTS golden.t13x"
  run_sql "$a" create-t "CREATE TABLE golden.t13e (id INT, c INT) ENGINE=InnoDB PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (100), PARTITION p1 VALUES LESS THAN (200))"
  run_sql "$a" create-x-myisam "CREATE TABLE golden.t13x (id INT, c INT) ENGINE=MyISAM"
  run_sql "$a" x-myisam-exchange "ALTER TABLE golden.t13e EXCHANGE PARTITION p0 WITH TABLE golden.t13x"
  run_sql "$a" x-myisam-post "SELECT TABLE_NAME, ENGINE FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME IN ('t13e','t13x')"
  run_sql "$a" drop-x "DROP TABLE IF EXISTS golden.t13x"
  run_sql "$a" create-x-innodb "CREATE TABLE golden.t13x (id INT, c INT) ENGINE=InnoDB"
  run_sql "$a" x-innodb-exchange "ALTER TABLE golden.t13e EXCHANGE PARTITION p0 WITH TABLE golden.t13x"
  run_sql "$a" x-innodb-post "SELECT TABLE_NAME, COUNT(*) AS n FROM golden.t13x"
  run_sql "$a" cleanup2 "DROP TABLE IF EXISTS golden.t13e; DROP TABLE IF EXISTS golden.t13x"
done
