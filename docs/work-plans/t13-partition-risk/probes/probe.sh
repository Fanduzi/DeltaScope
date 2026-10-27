#!/bin/bash
# T13 native partition probes — bounded disposable fixtures, four anchors.
# Each step records SQL, rc, stderr, and before/after catalog+data.
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
  rc=$?
  echo "--- rc=$rc"
  echo "--- out: $out"
}
q() {  # query helper
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
  q $A "SELECT VERSION()" | head -1 | xargs echo "version:"
  run_case $A cleanup "DROP TABLE IF EXISTS t13; DROP TABLE IF EXISTS t13x; DROP TABLE IF EXISTS t13np; DROP TABLE IF EXISTS t13l"
  run_case $A create-range "CREATE TABLE t13 (id INT, c INT) PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (100), PARTITION p1 VALUES LESS THAN (200))"
  run_case $A insert "INSERT INTO t13 VALUES (1,10),(50,20),(150,30)"
  echo "--- parts before: $(q $A "SELECT PARTITION_NAME,TABLE_ROWS FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13'" | tr '\n' ';')"
  echo "--- rows before: $(q $A "SELECT COUNT(*) FROM t13")"
  run_case $A drop-p0 "ALTER TABLE t13 DROP PARTITION p0"
  echo "--- parts after drop: $(q $A "SELECT PARTITION_NAME,TABLE_ROWS FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13'" | tr '\n' ';')"
  echo "--- rows after drop: $(q $A "SELECT COUNT(*) FROM t13")"
  run_case $A drop-missing "ALTER TABLE t13 DROP PARTITION p99"
  run_case $A drop-last "ALTER TABLE t13 DROP PARTITION p1"
  echo "--- parts after drop-last attempt: $(q $A "SELECT PARTITION_NAME FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13'" | tr '\n' ';')"
  # non-partitioned
  run_case $A create-nonpart "CREATE TABLE t13np (id INT, c INT)"
  run_case $A drop-on-nonpart "ALTER TABLE t13np DROP PARTITION p0"
  # TRUNCATE
  run_case $A truncate-p1 "ALTER TABLE t13 TRUNCATE PARTITION p1"
  echo "--- rows after truncate: $(q $A "SELECT COUNT(*) FROM t13") ; parts: $(q $A "SELECT PARTITION_NAME FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13'" | tr '\n' ';')"
  run_case $A insert2 "INSERT INTO t13 VALUES (110,1),(120,2)"
  run_case $A truncate-all "ALTER TABLE t13 TRUNCATE PARTITION ALL"
  echo "--- rows after truncate-all: $(q $A "SELECT COUNT(*) FROM t13") ; parts: $(q $A "SELECT PARTITION_NAME FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13'" | tr '\n' ';')"
  # EXCHANGE
  run_case $A create-exchange "CREATE TABLE t13x (id INT, c INT)"
  run_case $A x-insert "INSERT INTO t13x VALUES (5,99)"
  run_case $A p-insert "INSERT INTO t13 VALUES (10,1)"
  echo "--- t13 rows: $(q $A "SELECT * FROM t13" | tr '\n' ';') | t13x rows: $(q $A "SELECT * FROM t13x" | tr '\n' ';')"
  run_case $A exchange "ALTER TABLE t13 EXCHANGE PARTITION p0 WITH TABLE t13x"
  echo "--- t13 rows after: $(q $A "SELECT * FROM t13" | tr '\n' ';') | t13x rows after: $(q $A "SELECT * FROM t13x" | tr '\n' ';')"
  run_case $A exchange-noval "ALTER TABLE t13 EXCHANGE PARTITION p0 WITH TABLE t13x WITHOUT VALIDATION"
  run_case $A exchange-withval "ALTER TABLE t13 EXCHANGE PARTITION p0 WITH TABLE t13x WITH VALIDATION"
  # LIST partition
  run_case $A create-list "CREATE TABLE t13l (id INT, c INT) PARTITION BY LIST(id) (PARTITION pA VALUES IN (1,2), PARTITION pB VALUES IN (3,4))"
  run_case $A drop-list "ALTER TABLE t13l DROP PARTITION pA"
  # mixed
  run_case $A mixed "ALTER TABLE t13 DROP PARTITION p1, DROP COLUMN c"
  echo "--- cols after mixed: $(q $A "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13'") ; parts: $(q $A "SELECT PARTITION_NAME FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13'" | tr '\n' ';')"
  run_case $A final-cleanup "DROP TABLE IF EXISTS t13; DROP TABLE IF EXISTS t13x; DROP TABLE IF EXISTS t13np; DROP TABLE IF EXISTS t13l"
done
