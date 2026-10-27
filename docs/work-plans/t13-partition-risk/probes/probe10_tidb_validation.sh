#!/bin/bash
# T13-R2 D1 — TiDB EXCHANGE VALIDATION three arms, complete re-run.
# Every arm rebuilds an identical fixture from scratch; every step logs the
# full SQL, real rc, stderr, and before/after row contents + member catalog
# on BOTH sides. A failed fixture/query step must not masquerade as success:
# rc is captured per statement, never folded.
set -u

T() {
  local name="$1" sql="$2" out rc
  echo "=== [tidb85] $name"
  echo "--- SQL: $sql"
  out=$(docker exec deltascope-ddl-golden-tidb85-client \
    mysql --protocol=tcp -h tidb85 -P 4000 -uroot \
    --default-character-set=utf8mb4 -e "$sql" 2>&1)
  rc=$?
  echo "--- rc=$rc"
  echo "--- out: $out"
}

echo "############ ANCHOR tidb85 D1 validation arms ############"
T version "SELECT VERSION()"
T ensure-db "CREATE DATABASE IF NOT EXISTS golden"

for arm in absent with without; do
  echo "######## ARM $arm ########"
  T "$arm-fixture-drop" "DROP TABLE IF EXISTS golden.t13v; DROP TABLE IF EXISTS golden.t13w"
  T "$arm-fixture-t" "CREATE TABLE golden.t13v (id INT, c INT) PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (100), PARTITION p1 VALUES LESS THAN (200))"
  T "$arm-fixture-w" "CREATE TABLE golden.t13w (id INT, c INT)"
  T "$arm-seed-t" "INSERT INTO golden.t13v VALUES (1,1),(150,1)"
  T "$arm-seed-w-violating" "INSERT INTO golden.t13w VALUES (500,9)"
  T "$arm-pre-t-rows" "SELECT * FROM golden.t13v ORDER BY id"
  T "$arm-pre-w-rows" "SELECT * FROM golden.t13w ORDER BY id"
  T "$arm-pre-t-parts" "SELECT PARTITION_NAME FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13v' AND PARTITION_NAME IS NOT NULL ORDER BY PARTITION_NAME"
  case "$arm" in
    absent)   T "$arm-exchange" "ALTER TABLE golden.t13v EXCHANGE PARTITION p0 WITH TABLE golden.t13w" ;;
    with)     T "$arm-exchange" "ALTER TABLE golden.t13v EXCHANGE PARTITION p0 WITH TABLE golden.t13w WITH VALIDATION" ;;
    without)  T "$arm-exchange" "ALTER TABLE golden.t13v EXCHANGE PARTITION p0 WITH TABLE golden.t13w WITHOUT VALIDATION" ;;
  esac
  T "$arm-post-t-rows" "SELECT * FROM golden.t13v ORDER BY id"
  T "$arm-post-w-rows" "SELECT * FROM golden.t13w ORDER BY id"
  T "$arm-post-t-parts" "SELECT PARTITION_NAME FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13v' AND PARTITION_NAME IS NOT NULL ORDER BY PARTITION_NAME"
  T "$arm-cleanup" "DROP TABLE IF EXISTS golden.t13v; DROP TABLE IF EXISTS golden.t13w"
done
