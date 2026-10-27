#!/bin/bash
run() { docker exec "$1" sh -c "mysql -uroot -proot golden -e \"$2\"" 2>&1; }
for c in deltascope-ddl-golden-mysql57 deltascope-ddl-golden-mysql80 deltascope-ddl-golden-mysql84; do
  echo "=== $c ==="
  run "$c" "DROP TABLE IF EXISTS t13np; DROP TABLE IF EXISTS t13c; CREATE TABLE t13np (id INT, c INT); CREATE TABLE t13c (id INT, c INT);" >/dev/null
  echo "--- TRUNCATE PARTITION on non-partitioned"
  run "$c" "ALTER TABLE t13np TRUNCATE PARTITION p0"
  echo "rc=$?"
  echo "--- EXCHANGE PARTITION on non-partitioned"
  run "$c" "ALTER TABLE t13np EXCHANGE PARTITION p0 WITH TABLE t13c"
  echo "rc=$?"
  run "$c" "DROP TABLE t13np, t13c" >/dev/null
done
# tidb via client container
echo "=== tidb85 ==="
T=deltascope-ddl-golden-tidb85-client
docker exec $T sh -c "mysql --protocol=tcp -h tidb85 -P 4000 -uroot golden -e \"DROP TABLE IF EXISTS t13np; DROP TABLE IF EXISTS t13c; CREATE TABLE t13np (id INT, c INT); CREATE TABLE t13c (id INT, c INT);\"" 2>&1
echo "--- TRUNCATE PARTITION on non-partitioned"
docker exec $T sh -c "mysql --protocol=tcp -h tidb85 -P 4000 -uroot golden -e \"ALTER TABLE t13np TRUNCATE PARTITION p0\"" 2>&1
echo "rc=$?"
echo "--- EXCHANGE PARTITION on non-partitioned"
docker exec $T sh -c "mysql --protocol=tcp -h tidb85 -P 4000 -uroot golden -e \"ALTER TABLE t13np EXCHANGE PARTITION p0 WITH TABLE t13c\"" 2>&1
echo "rc=$?"
docker exec $T sh -c "mysql --protocol=tcp -h tidb85 -P 4000 -uroot golden -e \"DROP TABLE t13np, t13c\"" 2>&1
