#!/bin/bash
set -u
container="ds-t18r1tail-mysql80-$$"
cleanup() {
  echo "+ docker rm -v -f $container"
  docker rm -v -f "$container"
}
trap cleanup EXIT
run() {
  echo "+ $*"
  out=$("$@" 2>&1)
  rc=$?
  printf '%s\nrc=%d\n' "$out" "$rc"
}
run docker run -d --name "$container" -e MYSQL_ALLOW_EMPTY_PASSWORD=yes mysql:8.0.46
ready=0
for _ in $(seq 1 90); do
  if docker exec "$container" mysqladmin ping -uroot --silent >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
echo "ready_rc=$ready"
if [ "$ready" -ne 1 ]; then
  exit 1
fi
run docker exec "$container" mysql -uroot --batch --raw -e "SELECT VERSION()"
run docker exec "$container" mysql -uroot --batch --raw -e "DROP DATABASE IF EXISTS t18r1tail; CREATE DATABASE t18r1tail; CREATE TABLE t18r1tail.t(id INT)"
run docker exec "$container" mysql -uroot --batch --raw -e "CREATE ALGORITHM=TEMPTABLE VIEW t18r1tail.v AS SELECT id FROM t18r1tail.t"
run docker exec "$container" mysql -uroot --batch --raw -e "SELECT TABLE_NAME, IS_UPDATABLE, CHECK_OPTION, SECURITY_TYPE FROM information_schema.VIEWS WHERE TABLE_SCHEMA='t18r1tail' AND TABLE_NAME='v'"
run docker exec "$container" mysql -uroot --batch --raw -e "SHOW CREATE VIEW t18r1tail.v"
run docker exec "$container" mysql -uroot --batch --raw -e "CREATE VIEW t18r1tail.w AS SELECT id FROM t18r1tail.v WITH CHECK OPTION"
run docker exec "$container" mysql -uroot --batch --raw --force -e "CREATE VIEW t18r1tail.w2 AS SELECT id FROM t18r1tail.v WITH CHECK OPTION; SHOW WARNINGS"
run docker exec "$container" mysql -uroot --batch --raw -e "CREATE ALGORITHM=UNDEFINED VIEW t18r1tail.undef_v AS SELECT id FROM t18r1tail.t"
run docker exec "$container" mysql -uroot --batch --raw -e "SELECT TABLE_NAME, IS_UPDATABLE, CHECK_OPTION, SECURITY_TYPE FROM information_schema.VIEWS WHERE TABLE_SCHEMA='t18r1tail' ORDER BY TABLE_NAME"
run docker exec "$container" mysql -uroot --batch --raw -e "SELECT TABLE_NAME, TABLE_TYPE FROM information_schema.TABLES WHERE TABLE_SCHEMA='t18r1tail' ORDER BY TABLE_NAME"
run docker exec "$container" mysql -uroot --batch --raw -e "DROP DATABASE t18r1tail"
