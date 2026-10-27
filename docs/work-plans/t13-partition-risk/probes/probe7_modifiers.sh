#!/bin/bash
run_case() {
  local anchor="$1" name="$2" sql="$3"
  echo "=== [$anchor] $name"; echo "--- SQL: $sql"
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
    mysql57) docker exec deltascope-ddl-golden-mysql57 mysql -uroot -proot -N --default-character-set=utf8mb4 golden -e "$sql" 2>/dev/null;;
    mysql80) docker exec deltascope-ddl-golden-mysql80 mysql -uroot -proot -N --default-character-set=utf8mb4 golden -e "$sql" 2>/dev/null;;
    mysql84) docker exec deltascope-ddl-golden-mysql84 mysql -uroot -proot -N --default-character-set=utf8mb4 golden -e "$sql" 2>/dev/null;;
    tidb85)  docker exec deltascope-ddl-golden-tidb85-client mysql --protocol=tcp -h tidb85 -P 4000 -uroot -N --default-character-set=utf8mb4 golden -e "$sql" 2>/dev/null;;
  esac
}
for A in mysql57 mysql80 mysql84 tidb85; do
  echo "############ ANCHOR $A ############"
  run_case $A version "SELECT VERSION()"
  run_case $A cleanup "DROP TABLE IF EXISTS t13c"
  run_case $A c-create "CREATE TABLE t13c (id INT, c INT) PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (100), PARTITION p1 VALUES LESS THAN (200))"
  run_case $A c-seed "INSERT INTO t13c VALUES (1,1),(150,1)"
  run_case $A lock-then-drop "ALTER TABLE t13c LOCK=DEFAULT, DROP PARTITION p0"
  echo "--- after lock-then-drop parts: [$(q $A "SELECT PARTITION_NAME FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13c'" | tr '\n' ';')]"
  run_case $A reset1 "DROP TABLE t13c; CREATE TABLE t13c (id INT, c INT) PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (100), PARTITION p1 VALUES LESS THAN (200)); INSERT INTO t13c VALUES (1,1),(150,1)"
  run_case $A engine-then-drop "ALTER TABLE t13c ENGINE=InnoDB, DROP PARTITION p0"
  echo "--- after engine-then-drop parts: [$(q $A "SELECT PARTITION_NAME FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13c'" | tr '\n' ';')]"
  run_case $A reset2 "DROP TABLE t13c; CREATE TABLE t13c (id INT, c INT) PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (100), PARTITION p1 VALUES LESS THAN (200)); INSERT INTO t13c VALUES (1,1),(150,1)"
  run_case $A algdefault-then-drop "ALTER TABLE t13c ALGORITHM=DEFAULT, DROP PARTITION p0"
  echo "--- after algdefault-then-drop parts: [$(q $A "SELECT PARTITION_NAME FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13c'" | tr '\n' ';')]"
  run_case $A final-cleanup "DROP TABLE IF EXISTS t13c"
done
