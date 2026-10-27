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
for A in mysql57 mysql80 mysql84 tidb85; do
  echo "############ ANCHOR $A ############"
  run_case $A cleanup "DROP TABLE IF EXISTS t13v; DROP TABLE IF EXISTS t13vx; DROP TABLE IF EXISTS t13m"
  run_case $A create-range "CREATE TABLE t13v (id INT, c INT) PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (100), PARTITION p1 VALUES LESS THAN (200))"
  run_case $A x-create "CREATE TABLE t13vx (id INT, c INT)"
  run_case $A x-violate "INSERT INTO t13vx VALUES (999,1)"
  run_case $A exchange-violate-withval "ALTER TABLE t13v EXCHANGE PARTITION p0 WITH TABLE t13vx WITH VALIDATION"
  run_case $A exchange-violate-noval "ALTER TABLE t13v EXCHANGE PARTITION p0 WITH TABLE t13vx WITHOUT VALIDATION"
  run_case $A exchange-violate-absent "ALTER TABLE t13v EXCHANGE PARTITION p0 WITH TABLE t13vx"
  # mixed: partition action + other action
  run_case $A create-mixed "CREATE TABLE t13m (id INT, c INT) PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (100), PARTITION p1 VALUES LESS THAN (200))"
  run_case $A mixed-drop-part-plus-col "ALTER TABLE t13m DROP PARTITION p0, DROP COLUMN c"
  run_case $A mixed-part-only "ALTER TABLE t13m DROP PARTITION p0, DROP PARTITION p1"
  run_case $A mixed-trunc-plus-col "ALTER TABLE t13m TRUNCATE PARTITION p1, DROP COLUMN c"
  echo "--- t13m parts: $(docker exec ${A/mysql57/deltascope-ddl-golden-mysql57} 2>/dev/null true)"
  run_case $A final-cleanup "DROP TABLE IF EXISTS t13v; DROP TABLE IF EXISTS t13vx; DROP TABLE IF EXISTS t13m"
done
