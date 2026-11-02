#!/usr/bin/env bash
set -u
name="ds-t18r1tail2-native-ok"
schema="ds_t18_tail2"
out="/tmp/t18r1tail2-evidence/native-ok"
mkdir -p "$out"
cleanup() {
  docker rm -f "$name" >/dev/null 2>&1 || true
}
cleanup
printf 'image=mysql:8.0.46\nname=%s\nschema=%s\n' "$name" "$schema" > "$out/prereq.txt"
docker run -d --name "$name" -e MYSQL_ALLOW_EMPTY_PASSWORD=1 mysql:8.0.46 >"$out/run.stdout" 2>"$out/run.stderr"
rc=$?; printf '%s\n' "$rc" > "$out/run.rc"
if [ "$rc" -ne 0 ]; then exit "$rc"; fi
ready=1
for i in $(seq 1 90); do
  logs=$(docker logs "$name" 2>&1 || true)
  if printf '%s' "$logs" | grep -q 'ready for connections.*port: 3306'; then
    docker exec "$name" mysqladmin ping -uroot --silent >/dev/null 2>&1 && ready=0 && break
  fi
  sleep 1
done
printf '%s\n' "$ready" > "$out/ready.rc"
docker exec "$name" mysqladmin ping -uroot --silent >"$out/ready.stdout" 2>"$out/ready.stderr" || true
if [ "$ready" -ne 0 ]; then cleanup; exit "$ready"; fi
mysql_exec() {
  local label="$1" sql="$2"
  docker exec "$name" mysql -uroot --batch --raw --skip-column-names -e "$sql" >"$out/${label}.stdout" 2>"$out/${label}.stderr"
  rc=$?; printf '%s\n' "$rc" > "$out/${label}.rc"
}
mysql_exec 00_drop "DROP DATABASE IF EXISTS ${schema}"
mysql_exec 01_schema "CREATE DATABASE ${schema}"
mysql_exec 02_t_amount "CREATE TABLE ${schema}.t_amount(x INT)"
mysql_exec 03_t_base "CREATE TABLE ${schema}.t_base(id INT)"
mysql_exec 10_agg_v "CREATE VIEW ${schema}.agg_v AS SELECT SUM(x) AS total FROM ${schema}.t_amount"
mysql_exec 11_base_v "CREATE VIEW ${schema}.base_v AS SELECT id FROM ${schema}.t_base"
mysql_exec 12_joined_v "CREATE VIEW ${schema}.joined_v AS SELECT agg_v.total, base_v.id FROM ${schema}.agg_v JOIN ${schema}.base_v ON agg_v.total = base_v.id"
mysql_exec 13_checked_v "CREATE VIEW ${schema}.checked_v AS SELECT id FROM ${schema}.joined_v WITH CHECK OPTION"
mysql_exec 14_joined_bad_v "CREATE VIEW ${schema}.joined_bad_v AS SELECT agg_v.total FROM ${schema}.agg_v JOIN ${schema}.base_v ON agg_v.total = base_v.id"
mysql_exec 20_mixed_v "CREATE VIEW ${schema}.mixed_v AS SELECT id, id + 1 AS x FROM ${schema}.t_base"
mysql_exec 21_bad_check_v "CREATE VIEW ${schema}.bad_check_v AS SELECT x FROM ${schema}.mixed_v WITH CHECK OPTION"
mysql_exec 22_expr_v "CREATE VIEW ${schema}.expr_v AS SELECT x FROM ${schema}.mixed_v"
mysql_exec 23_next_check_v "CREATE VIEW ${schema}.next_check_v AS SELECT x FROM ${schema}.expr_v WITH CHECK OPTION"
mysql_exec 24_good_check_v "CREATE VIEW ${schema}.good_check_v AS SELECT id FROM ${schema}.mixed_v WITH CHECK OPTION"
mysql_exec 25_mixed_out_v "CREATE VIEW ${schema}.mixed_out_v AS SELECT id, x FROM ${schema}.mixed_v"
mysql_exec 90_catalog "SELECT TABLE_NAME, IS_UPDATABLE FROM information_schema.VIEWS WHERE TABLE_SCHEMA='${schema}' ORDER BY TABLE_NAME"
for v in agg_v base_v joined_v checked_v joined_bad_v mixed_v bad_check_v expr_v next_check_v good_check_v mixed_out_v; do
  mysql_exec "91_show_${v}" "SHOW CREATE VIEW ${schema}.${v}"
done
mysql_exec 92_tables "SELECT TABLE_NAME, TABLE_TYPE FROM information_schema.TABLES WHERE TABLE_SCHEMA='${schema}' ORDER BY TABLE_NAME"
mysql_exec 93_drop_schema "DROP DATABASE ${schema}"
docker rm -f "$name" >"$out/cleanup.stdout" 2>"$out/cleanup.stderr"
rc=$?; printf '%s\n' "$rc" > "$out/cleanup.rc"
exit "$rc"
