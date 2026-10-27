#!/bin/bash
run_case() {
  local anchor="$1" name="$2" sql="$3"
  echo "=== [$anchor] $name"
  echo "--- SQL: $sql"
  case "$anchor" in
    mysql57) out=$(docker exec deltascope-ddl-golden-mysql57 mysql -uroot -proot --default-character-set=utf8mb4 golden -e "$sql" 2>&1);;
    mysql80) out=$(docker exec deltascope-ddl-golden-mysql80 mysql -uroot -proot --default-character-set=utf8mb4 golden -e "$sql" 2>&1);;
    mysql84) out=$(docker exec deltascope-ddl-golden-mysql84 mysql -uroot -proot --default-character-set=utf8mb4 golden -e "$sql" 2>&1);;
    tidb85)  out=$(docker exec deltascope-ddl-golden-tidb85-client mysql --protocol=tcp -h tidb85 -P 4000 -uroot --default-character-set=utf8mb4 golden -e "$sql" 2>&1);;
  esac
  echo "--- rc=$?"; echo "--- out: $out"
}
q() {
  local anchor="$1" sql="$2"
  case "$anchor" in
    mysql57) docker exec deltascope-ddl-golden-mysql57 mysql -uroot -proot -Nse "$sql" golden 2>/dev/null;;
    mysql80) docker exec deltascope-ddl-golden-mysql80 mysql -uroot -proot -Nse "$sql" golden 2>/dev/null;;
    mysql84) docker exec deltascope-ddl-golden-mysql84 mysql -uroot -proot -Nse "$sql" golden 2>/dev/null;;
    tidb85)  docker exec deltascope-ddl-golden-tidb85-client mysql --protocol=tcp -h tidb85 -P 4000 -uroot -Nse "$sql" golden 2>/dev/null;;
  esac
}
for A in mysql57 mysql80 mysql84 tidb85; do
  echo "############ ANCHOR $A ############"
  run_case $A cleanup "DROP TABLE IF EXISTS t13; DROP TABLE IF EXISTS t13x; DROP TABLE IF EXISTS t13x2; DROP TABLE IF EXISTS t13np"
  run_case $A create-range "CREATE TABLE t13 (id INT, c INT) PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (100), PARTITION p1 VALUES LESS THAN (200))"
  run_case $A insert "INSERT INTO t13 VALUES (1,10),(50,20),(150,30)"
  # EXCHANGE first (partitions intact)
  run_case $A create-exchange "CREATE TABLE t13x (id INT, c INT)"
  run_case $A x-insert "INSERT INTO t13x VALUES (5,99)"
  echo "--- t13 rows: $(q $A "SELECT CONCAT_WS(',',id,c) FROM t13 ORDER BY id" | tr '\n' ';') | t13x: $(q $A "SELECT CONCAT_WS(',',id,c) FROM t13x" | tr '\n' ';')"
  run_case $A exchange-p0 "ALTER TABLE t13 EXCHANGE PARTITION p0 WITH TABLE t13x"
  echo "--- t13 rows: $(q $A "SELECT CONCAT_WS(',',id,c) FROM t13 ORDER BY id" | tr '\n' ';') | t13x: $(q $A "SELECT CONCAT_WS(',',id,c) FROM t13x" | tr '\n' ';')"
  run_case $A exchange-back-noval "ALTER TABLE t13 EXCHANGE PARTITION p0 WITH TABLE t13x WITHOUT VALIDATION"
  run_case $A exchange-withval "ALTER TABLE t13 EXCHANGE PARTITION p0 WITH TABLE t13x WITH VALIDATION"
  # violating data: t13x row that does NOT satisfy p0's range (id<100)
  run_case $A x2-create "CREATE TABLE t13x2 (id INT, c INT)"
  run_case $A x2-violate "INSERT INTO t13x2 VALUES (999,1)"
  run_case $A exchange-violate-noval "ALTER TABLE t13 EXCHANGE PARTITION p0 WITH TABLE t13x2 WITHOUT VALIDATION"
  echo "--- violating exchange noval: t13=$(q $A "SELECT CONCAT_WS(',',id,c) FROM t13 ORDER BY id" | tr '\n' ';') t13x2=$(q $A "SELECT CONCAT_WS(',',id,c) FROM t13x2" | tr '\n' ';')"
  run_case $A exchange-violate-withval "ALTER TABLE t13 EXCHANGE PARTITION p0 WITH TABLE t13x2 WITH VALIDATION"
  # incompatible exchange table (different columns)
  run_case $A x3-create "CREATE TABLE t13x3 (id INT, c INT, extra INT)"
  run_case $A exchange-incompat "ALTER TABLE t13 EXCHANGE PARTITION p0 WITH TABLE t13x3"
  # partitioned exchange target
  run_case $A x4-create "CREATE TABLE t13x4 (id INT, c INT) PARTITION BY RANGE(id) (PARTITION q0 VALUES LESS THAN (300))"
  run_case $A exchange-partitioned-target "ALTER TABLE t13 EXCHANGE PARTITION p0 WITH TABLE t13x4"
  # missing exchange table
  run_case $A exchange-missing-table "ALTER TABLE t13 EXCHANGE PARTITION p0 WITH TABLE t13xmissing"
  # missing partition
  run_case $A exchange-missing-part "ALTER TABLE t13 EXCHANGE PARTITION p99 WITH TABLE t13x"
  # qualified exchange table (schema-qualified)
  run_case $A exchange-qualified "ALTER TABLE t13 EXCHANGE PARTITION p0 WITH TABLE golden.t13x"
  # DROP multi-name
  run_case $A drop-multi "ALTER TABLE t13 DROP PARTITION p0, p1"
  echo "--- parts after drop-multi: $(q $A "SELECT PARTITION_NAME FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13'" | tr '\n' ';')"
  # DROP missing on 2-partition table
  run_case $A create-2p "CREATE TABLE t13np (id INT) PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (100), PARTITION p1 VALUES LESS THAN (200))"
  run_case $A drop-missing-2p "ALTER TABLE t13np DROP PARTITION p99"
  run_case $A drop-multi-missing "ALTER TABLE t13np DROP PARTITION p0, p99"
  echo "--- parts after drop-missing-2p: $(q $A "SELECT PARTITION_NAME FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13np'" | tr '\n' ';')"
  run_case $A final-cleanup "DROP TABLE IF EXISTS t13; DROP TABLE IF EXISTS t13x; DROP TABLE IF EXISTS t13x2; DROP TABLE IF EXISTS t13x3; DROP TABLE IF EXISTS t13x4; DROP TABLE IF EXISTS t13np"
done
