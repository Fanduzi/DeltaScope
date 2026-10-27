#!/bin/bash
# T13-R1 C + D2: ALGORITHM modifier combinability, HASH applicability,
# ADD-first mixed actions, EXCHANGE mixed actions.
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
    mysql57) docker exec deltascope-ddl-golden-mysql57 mysql -uroot -proot -N --default-character-set=utf8mb4 golden -e "$sql" 2>/dev/null;;
    mysql80) docker exec deltascope-ddl-golden-mysql80 mysql -uroot -proot -N --default-character-set=utf8mb4 golden -e "$sql" 2>/dev/null;;
    mysql84) docker exec deltascope-ddl-golden-mysql84 mysql -uroot -proot -N --default-character-set=utf8mb4 golden -e "$sql" 2>/dev/null;;
    tidb85)  docker exec deltascope-ddl-golden-tidb85-client mysql --protocol=tcp -h tidb85 -P 4000 -uroot -N --default-character-set=utf8mb4 golden -e "$sql" 2>/dev/null;;
  esac
}
for A in mysql57 mysql80 mysql84 tidb85; do
  echo "############ ANCHOR $A ############"
  run_case $A version "SELECT VERSION()"
  run_case $A cleanup "DROP TABLE IF EXISTS t13c; DROP TABLE IF EXISTS t13cx; DROP TABLE IF EXISTS t13h; DROP TABLE IF EXISTS t13hx"
  # C: ALGORITHM modifier + partition action
  run_case $A c-create "CREATE TABLE t13c (id INT, c INT) PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (100), PARTITION p1 VALUES LESS THAN (200))"
  run_case $A c-seed "INSERT INTO t13c VALUES (1,1),(150,1)"
  run_case $A c-algorithm-then-drop "ALTER TABLE t13c ALGORITHM=INPLACE, DROP PARTITION p0"
  echo "--- after algorithm-then-drop parts: [$(q $A "SELECT PARTITION_NAME FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13c'" | tr '\n' ';')] rows: [$(q $A "SELECT id FROM t13c ORDER BY id" | tr '\n' ';')]"
  run_case $A c-reset "DROP TABLE IF EXISTS t13c; CREATE TABLE t13c (id INT, c INT) PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (100), PARTITION p1 VALUES LESS THAN (200)); INSERT INTO t13c VALUES (1,1),(150,1)"
  run_case $A c-drop-then-algorithm "ALTER TABLE t13c DROP PARTITION p0, ALGORITHM=INPLACE"
  echo "--- after drop-then-algorithm parts: [$(q $A "SELECT PARTITION_NAME FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13c'" | tr '\n' ';')] rows: [$(q $A "SELECT id FROM t13c ORDER BY id" | tr '\n' ';')]"
  # D2: HASH partition trio
  run_case $A h-create "CREATE TABLE t13h (id INT, c INT) PARTITION BY HASH(id) PARTITIONS 2"
  run_case $A h-seed "INSERT INTO t13h VALUES (1,1),(2,2),(3,3)"
  run_case $A h-drop "ALTER TABLE t13h DROP PARTITION p0"
  run_case $A h-truncate "ALTER TABLE t13h TRUNCATE PARTITION p0"
  echo "--- after hash-truncate parts: [$(q $A "SELECT PARTITION_NAME FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13h'" | tr '\n' ';')] rows left: [$(q $A "SELECT COUNT(*) FROM t13h")]"
  run_case $A hx-create "CREATE TABLE t13hx (id INT, c INT)"
  run_case $A h-exchange "ALTER TABLE t13h EXCHANGE PARTITION p0 WITH TABLE t13hx"
  # D2: ADD-first structural mix
  run_case $A a-create "CREATE TABLE t13a (id INT, c INT) PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (100), PARTITION p1 VALUES LESS THAN (200))" || true
  run_case $A drop-table-a "DROP TABLE IF EXISTS t13a"
  run_case $A a-create "CREATE TABLE t13a (id INT, c INT) PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (100), PARTITION p1 VALUES LESS THAN (200))"
  run_case $A a-addcol-then-drop "ALTER TABLE t13a ADD COLUMN x INT, DROP PARTITION p0"
  echo "--- after addcol-then-drop parts: [$(q $A "SELECT PARTITION_NAME FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13a'" | tr '\n' ';')] cols: [$(q $A "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13a'")]"
  # D2: EXCHANGE mixed
  run_case $A m-create "CREATE TABLE t13m2 (id INT, c INT) PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (100), PARTITION p1 VALUES LESS THAN (200))"
  run_case $A mx-create "CREATE TABLE t13mx (id INT, c INT)"
  run_case $A m-exchange-then-dropcol "ALTER TABLE t13m2 EXCHANGE PARTITION p0 WITH TABLE t13mx, DROP COLUMN c"
  run_case $A m-dropcol-then-exchange "ALTER TABLE t13m2 DROP COLUMN c, EXCHANGE PARTITION p0 WITH TABLE t13mx"
  run_case $A final-cleanup "DROP TABLE IF EXISTS t13c; DROP TABLE IF EXISTS t13cx; DROP TABLE IF EXISTS t13h; DROP TABLE IF EXISTS t13hx; DROP TABLE IF EXISTS t13a; DROP TABLE IF EXISTS t13m2; DROP TABLE IF EXISTS t13mx"
done
