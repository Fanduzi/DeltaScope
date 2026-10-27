#!/bin/bash
# T13-R1 D1: EXCHANGE PARTITION validation three-arm re-probe.
# Each arm rebuilds an identical fixture so the counterpart always holds a
# violating row (id=999, outside p1's <200 bound) at exchange time.
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
  run_case $A cleanup "DROP TABLE IF EXISTS t13v; DROP TABLE IF EXISTS t13vx"
  for ARM in withval noval absent; do
    echo "===== ARM $ARM ====="
    # fresh identical fixture per arm: counterpart always carries the
    # violating row before the exchange
    run_case $A ${ARM}-reset "DROP TABLE IF EXISTS t13v; DROP TABLE IF EXISTS t13vx"
    run_case $A ${ARM}-create "CREATE TABLE t13v (id INT, c INT) PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (100), PARTITION p1 VALUES LESS THAN (200))"
    run_case $A ${ARM}-xcreate "CREATE TABLE t13vx (id INT, c INT)"
    run_case $A ${ARM}-xseed "INSERT INTO t13vx VALUES (999,1)"
    run_case $A ${ARM}-pseed "INSERT INTO t13v VALUES (50,1),(150,1)"
    echo "--- pre: t13vx=[$(q $A "SELECT CONCAT_WS(',',id,c) FROM t13vx")] t13v=[$(q $A "SELECT CONCAT_WS(',',id,c) FROM t13v ORDER BY id" | tr '\n' ';')]"
    case "$ARM" in
      withval) run_case $A ${ARM}-exchange "ALTER TABLE t13v EXCHANGE PARTITION p0 WITH TABLE t13vx WITH VALIDATION";;
      noval)   run_case $A ${ARM}-exchange "ALTER TABLE t13v EXCHANGE PARTITION p0 WITH TABLE t13vx WITHOUT VALIDATION";;
      absent)  run_case $A ${ARM}-exchange "ALTER TABLE t13v EXCHANGE PARTITION p0 WITH TABLE t13vx";;
    esac
    echo "--- post: t13vx=[$(q $A "SELECT CONCAT_WS(',',id,c) FROM t13vx")] t13v=[$(q $A "SELECT CONCAT_WS(',',id,c) FROM t13v ORDER BY id" | tr '\n' ';')]"
    echo "--- post parts: [$(q $A "SELECT CONCAT_WS(',',PARTITION_NAME,TABLE_ROWS) FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13v'" | tr '\n' ';')]"
  done
  run_case $A final-cleanup "DROP TABLE IF EXISTS t13v; DROP TABLE IF EXISTS t13vx"
done
