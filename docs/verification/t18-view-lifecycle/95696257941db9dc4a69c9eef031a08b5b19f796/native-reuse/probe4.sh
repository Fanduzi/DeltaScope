#!/usr/bin/env bash
set -u
name="t18r1tail2-mysql8046"
run() {
  label="$1"
  sql="$2"
  echo "===== $label ====="
  out="$(printf '%s\n' "$sql" | docker exec -i "$name" mysql -uroot --batch --raw --show-warnings 2>&1)"
  rc=$?
  printf 'rc=%s\n' "$rc"
  if [[ -n "$out" ]]; then printf '%s\n' "$out"; fi
}
run setup "DROP DATABASE IF EXISTS t18r1t2_g; DROP DATABASE IF EXISTS t18r1t2_src; CREATE DATABASE t18r1t2_g; CREATE DATABASE t18r1t2_src; CREATE TABLE t18r1t2_src.t(id INT); CREATE VIEW t18r1t2_src.a AS SELECT id FROM t18r1t2_src.t; CREATE VIEW t18r1t2_src.a2 AS SELECT id FROM t18r1t2_src.a;"
run main_via_cte_subquery "CREATE VIEW t18r1t2_g.main_via_cte_subquery AS WITH c AS (SELECT t.id FROM t18r1t2_src.t AS t) SELECT t.id FROM t18r1t2_src.t AS t WHERE EXISTS (SELECT 1 FROM c);"
run main_via_cte_derived "CREATE VIEW t18r1t2_g.main_via_cte_derived AS WITH c AS (SELECT t.id FROM t18r1t2_src.t AS t) SELECT t.id FROM t18r1t2_src.t AS t WHERE EXISTS (SELECT 1 FROM (SELECT c.id FROM c) AS dt);"
run main_via_view_derived "CREATE VIEW t18r1t2_g.main_via_view_derived AS SELECT t.id FROM t18r1t2_src.t AS t WHERE EXISTS (SELECT 1 FROM (SELECT a.id FROM t18r1t2_src.a AS a) AS dt);"
run main_via_nested_view "CREATE VIEW t18r1t2_g.main_via_nested_view AS SELECT t.id FROM t18r1t2_src.t AS t WHERE EXISTS (SELECT 1 FROM t18r1t2_src.a2);"
run catalog "SELECT table_schema,table_name,is_updatable FROM information_schema.views WHERE table_schema='t18r1t2_g' ORDER BY table_name;"
run cleanup "DROP DATABASE IF EXISTS t18r1t2_g; DROP DATABASE IF EXISTS t18r1t2_src;"
