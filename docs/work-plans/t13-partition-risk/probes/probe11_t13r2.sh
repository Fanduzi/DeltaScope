#!/bin/bash
# T13-R2 — A2 (subpartition directory / TiDB RANGE INTERVAL), A3 (unbounded
# PARTITIONS n), D2 (ALGORITHM=INPLACE + TRUNCATE/EXCHANGE modifier arms).
set -u

Q() {
  local anchor="$1" name="$2" sql="$3" out rc
  echo "=== [$anchor] $name"
  echo "--- SQL: $sql"
  case "$anchor" in
    tidb85)
      out=$(docker exec deltascope-ddl-golden-tidb85-client \
        mysql --protocol=tcp -h tidb85 -P 4000 -uroot \
        --default-character-set=utf8mb4 -e "$sql" 2>&1)
      rc=$? ;;
    *)
      out=$(docker exec --env MYSQL_PWD=root deltascope-ddl-golden-$anchor \
        mysql -uroot --default-character-set=utf8mb4 -e "$sql" 2>&1)
      rc=$? ;;
  esac
  echo "--- rc=$rc"
  echo "--- out: $out"
}

PARTS="SELECT PARTITION_NAME, SUBPARTITION_NAME FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13s' AND PARTITION_NAME IS NOT NULL ORDER BY PARTITION_NAME, SUBPARTITION_NAME"

for a in mysql57 mysql80 mysql84 tidb85; do
  echo "############ ANCHOR $a A2 subpartition ############"
  [ "$a" = "tidb85" ] && Q "$a" ensure-db "CREATE DATABASE IF NOT EXISTS golden"
  Q "$a" cleanup "DROP TABLE IF EXISTS golden.t13s; DROP TABLE IF EXISTS golden.t13x"
  Q "$a" create-sub "CREATE TABLE golden.t13s (id INT) PARTITION BY RANGE(id) SUBPARTITION BY HASH(id) SUBPARTITIONS 2 (PARTITION p0 VALUES LESS THAN (10), PARTITION p1 VALUES LESS THAN (20))"
  Q "$a" create-x "CREATE TABLE golden.t13x (id INT)"
  Q "$a" catalog "$PARTS"
  Q "$a" xchg-leaf "ALTER TABLE golden.t13s EXCHANGE PARTITION p0sp0 WITH TABLE golden.t13x"
  Q "$a" xchg-top "ALTER TABLE golden.t13s EXCHANGE PARTITION p0 WITH TABLE golden.t13x"
  Q "$a" cleanup2 "DROP TABLE IF EXISTS golden.t13s; DROP TABLE IF EXISTS golden.t13x"
done

echo "############ ANCHOR tidb85 A2 RANGE INTERVAL ############"
Q tidb85 ivl-cleanup "DROP TABLE IF EXISTS golden.t13i"
Q tidb85 ivl-create "CREATE TABLE golden.t13i (id INT) PARTITION BY RANGE(id) INTERVAL(10) FIRST PARTITION LESS THAN(10) LAST PARTITION LESS THAN(30)"
Q tidb85 ivl-catalog "SELECT PARTITION_NAME FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13i' AND PARTITION_NAME IS NOT NULL ORDER BY PARTITION_NAME"
Q tidb85 ivl-truncate "ALTER TABLE golden.t13i TRUNCATE PARTITION p_lt_10"
Q tidb85 ivl-cleanup2 "DROP TABLE IF EXISTS golden.t13i"

echo "############ ANCHOR mysql80 A3 unbounded PARTITIONS ############"
Q mysql80 big-cleanup "DROP TABLE IF EXISTS golden.t13b"
Q mysql80 big-create "CREATE TABLE golden.t13b (id INT) PARTITION BY HASH(id) PARTITIONS 18446744073709551615"
Q mysql80 big-parts-max "SELECT @@max_partitions" 2>/dev/null || true
Q mysql80 big-create2 "CREATE TABLE golden.t13b (id INT) PARTITION BY HASH(id) PARTITIONS 8193"
Q mysql80 big-catalog-count "SELECT COUNT(*) AS n FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13b' AND PARTITION_NAME IS NOT NULL"
Q mysql80 big-cleanup2 "DROP TABLE IF EXISTS golden.t13b"

for a in mysql57 mysql80 mysql84 tidb85; do
  echo "############ ANCHOR $a D2 INPLACE+TRUNCATE/EXCHANGE ############"
  [ "$a" = "tidb85" ] && Q "$a" ensure-db2 "CREATE DATABASE IF NOT EXISTS golden"
  Q "$a" d2-cleanup "DROP TABLE IF EXISTS golden.t13d; DROP TABLE IF EXISTS golden.t13y"
  Q "$a" d2-create "CREATE TABLE golden.t13d (id INT, c INT) PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (100), PARTITION p1 VALUES LESS THAN (200)); INSERT INTO golden.t13d VALUES (1,1),(150,1)"
  Q "$a" d2-inplace-trunc "ALTER TABLE golden.t13d ALGORITHM=INPLACE, TRUNCATE PARTITION p0"
  Q "$a" d2-trunc-post "SELECT PARTITION_NAME FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13d' AND PARTITION_NAME IS NOT NULL ORDER BY PARTITION_NAME"
  Q "$a" d2-create-y "CREATE TABLE golden.t13y (id INT, c INT); INSERT INTO golden.t13y VALUES (5,5)"
  Q "$a" d2-inplace-xchg "ALTER TABLE golden.t13d ALGORITHM=INPLACE, EXCHANGE PARTITION p0 WITH TABLE golden.t13y"
  Q "$a" d2-xchg-post-t "SELECT * FROM golden.t13d ORDER BY id"
  Q "$a" d2-xchg-post-y "SELECT * FROM golden.t13y ORDER BY id"
  Q "$a" d2-cleanup2 "DROP TABLE IF EXISTS golden.t13d; DROP TABLE IF EXISTS golden.t13y"
done
