#!/bin/bash
exec_mysql() { docker exec "$1" sh -c "MYSQL_PWD=root mysql -uroot golden -e \"$2\"" 2>&1; }
exec_tidb() { docker exec deltascope-ddl-golden-tidb85-client sh -c "mysql --protocol=tcp -h tidb85 -P 4000 -uroot golden -e \"$1\"" 2>&1; }
for c in deltascope-ddl-golden-mysql57 deltascope-ddl-golden-mysql80 deltascope-ddl-golden-mysql84; do
  echo "=== $c ==="
  exec_mysql "$c" "DROP TABLE IF EXISTS t13e; CREATE TABLE t13e (id INT, c INT) PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (100), PARTITION p1 VALUES LESS THAN (200))" >/dev/null
  echo "--- ENGINE=InnoDB, DROP PARTITION p0"
  exec_mysql "$c" "ALTER TABLE t13e ENGINE=InnoDB, DROP PARTITION p0"; echo "rc=$?"
  echo "--- after: $(exec_mysql "$c" "SELECT GROUP_CONCAT(PARTITION_NAME) FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13e' AND PARTITION_NAME IS NOT NULL" | tail -1)"
  exec_mysql "$c" "DROP TABLE IF EXISTS t13e" >/dev/null
done
echo "=== tidb85 ==="
exec_tidb "DROP TABLE IF EXISTS t13e; CREATE TABLE t13e (id INT, c INT) PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (100), PARTITION p1 VALUES LESS THAN (200))" >/dev/null
echo "--- ENGINE=InnoDB, DROP PARTITION p0"
exec_tidb "ALTER TABLE t13e ENGINE=InnoDB, DROP PARTITION p0"; echo "rc=$?"
echo "--- after: $(exec_tidb "SELECT GROUP_CONCAT(PARTITION_NAME) FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='t13e' AND PARTITION_NAME IS NOT NULL" | tail -1)"
exec_tidb "DROP TABLE IF EXISTS t13e" >/dev/null
