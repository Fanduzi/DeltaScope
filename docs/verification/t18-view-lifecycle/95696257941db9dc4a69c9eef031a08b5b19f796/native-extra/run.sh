#!/usr/bin/env bash
set -u
name="ds-t18r1tail2-native-extra"
schema="ds_t18_tail2x"
out="/tmp/t18r1tail2-evidence/native-extra"
mkdir -p "$out"
cleanup() { docker rm -f "$name" >/dev/null 2>&1 || true; }
cleanup
printf 'image=mysql:8.0.46\nname=%s\nschema=%s\n' "$name" "$schema" > "$out/prereq.txt"
docker run -d --name "$name" -e MYSQL_ALLOW_EMPTY_PASSWORD=1 mysql:8.0.46 >"$out/run.stdout" 2>"$out/run.stderr"
rc=$?; printf '%s\n' "$rc" > "$out/run.rc"; [ "$rc" -eq 0 ] || exit "$rc"
ready=1
for i in $(seq 1 90); do
  if docker logs "$name" 2>&1 | grep -q 'ready for connections.*port: 3306'; then
    docker exec "$name" mysqladmin ping -uroot --silent >/dev/null 2>&1 && ready=0 && break
  fi
  sleep 1
done
printf '%s\n' "$ready" > "$out/ready.rc"; docker exec "$name" mysqladmin ping -uroot --silent >"$out/ready.stdout" 2>"$out/ready.stderr" || true; [ "$ready" -eq 0 ] || { cleanup; exit "$ready"; }
mysql_exec() { local label="$1" sql="$2"; docker exec "$name" mysql -uroot --batch --raw --skip-column-names -e "$sql" >"$out/${label}.stdout" 2>"$out/${label}.stderr"; rc=$?; printf '%s\n' "$rc" >"$out/${label}.rc"; }
mysql_exec 00_drop "DROP DATABASE IF EXISTS ${schema}"
mysql_exec 01_schema "CREATE DATABASE ${schema}"
mysql_exec 02_t "CREATE TABLE ${schema}.t(id INT)"
mysql_exec 10_derived_direct "CREATE VIEW ${schema}.derived_direct_v AS SELECT x FROM (SELECT id AS x FROM ${schema}.t) dt"
mysql_exec 11_derived_expr "CREATE VIEW ${schema}.derived_expr_v AS SELECT x FROM (SELECT id + 1 AS x FROM ${schema}.t) dt"
mysql_exec 12_derived_mixed "CREATE VIEW ${schema}.derived_mixed_v AS SELECT id, x FROM (SELECT id, id + 1 AS x FROM ${schema}.t) dt"
mysql_exec 13_cte_expr "CREATE VIEW ${schema}.cte_expr_v AS WITH c AS (SELECT id + 1 AS x FROM ${schema}.t) SELECT x FROM c"
mysql_exec 14_view_col_alias "CREATE VIEW ${schema}.aliased_v(rx) AS SELECT id + 1 FROM ${schema}.t"
mysql_exec 90_catalog "SELECT TABLE_NAME, IS_UPDATABLE FROM information_schema.VIEWS WHERE TABLE_SCHEMA='${schema}' ORDER BY TABLE_NAME"
for v in derived_direct_v derived_expr_v derived_mixed_v cte_expr_v aliased_v; do mysql_exec "91_show_${v}" "SHOW CREATE VIEW ${schema}.${v}"; done
mysql_exec 92_drop "DROP DATABASE ${schema}"
docker rm -f "$name" >"$out/cleanup.stdout" 2>"$out/cleanup.stderr"; rc=$?; printf '%s\n' "$rc" >"$out/cleanup.rc"; exit "$rc"
