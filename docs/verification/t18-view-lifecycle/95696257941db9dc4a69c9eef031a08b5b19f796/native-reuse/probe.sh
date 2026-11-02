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
run version "SELECT VERSION() AS version;"
run setup "DROP DATABASE IF EXISTS t18r1t2_g; DROP DATABASE IF EXISTS t18r1t2_src; CREATE DATABASE t18r1t2_g; CREATE DATABASE t18r1t2_src; CREATE TABLE t18r1t2_src.t(id INT); CREATE TABLE t18r1t2_src.u(id INT); CREATE TABLE t18r1t2_src.x(id INT); CREATE VIEW t18r1t2_src.a AS SELECT id FROM t18r1t2_src.t;"
run distinct_nested "CREATE VIEW t18r1t2_g.distinct_nested AS SELECT t.id FROM t18r1t2_src.t AS t WHERE EXISTS (SELECT 1 FROM t18r1t2_src.u);"
run reuse_where "CREATE VIEW t18r1t2_g.reuse_where AS SELECT t.id FROM t18r1t2_src.t AS t WHERE EXISTS (SELECT 1 FROM t18r1t2_src.t);"
run reuse_select_list "CREATE VIEW t18r1t2_g.reuse_select_list AS SELECT (SELECT 1 FROM t18r1t2_src.t LIMIT 1) AS x, t.id FROM t18r1t2_src.t AS t;"
run reuse_derived_subquery "CREATE VIEW t18r1t2_g.reuse_derived_subquery AS SELECT t.id FROM t18r1t2_src.t AS t WHERE EXISTS (SELECT 1 FROM (SELECT 1 FROM t18r1t2_src.t) AS dt);"
run derived_inner_reuse "CREATE VIEW t18r1t2_g.derived_inner_reuse AS SELECT dt.id FROM (SELECT t.id FROM t18r1t2_src.t AS t WHERE EXISTS (SELECT 1 FROM t18r1t2_src.t)) AS dt;"
run derived_outer_inner_same_leaf "CREATE VIEW t18r1t2_g.derived_outer_inner_same_leaf AS SELECT dt.id FROM (SELECT t.id FROM t18r1t2_src.t AS t) AS dt WHERE EXISTS (SELECT 1 FROM t18r1t2_src.t);"
run cte_inner_reuse "CREATE VIEW t18r1t2_g.cte_inner_reuse AS WITH c AS (SELECT t.id FROM t18r1t2_src.t AS t WHERE EXISTS (SELECT 1 FROM t18r1t2_src.t)) SELECT c.id FROM c;"
run cte_outer_inner_same_leaf "CREATE VIEW t18r1t2_g.cte_outer_inner_same_leaf AS WITH c AS (SELECT t.id FROM t18r1t2_src.t AS t) SELECT c.id FROM c WHERE EXISTS (SELECT 1 FROM t18r1t2_src.t);"
run cte_referenced_again "CREATE VIEW t18r1t2_g.cte_referenced_again AS WITH c AS (SELECT t.id FROM t18r1t2_src.t AS t) SELECT c.id FROM c WHERE EXISTS (SELECT 1 FROM c);"
run view_source_reused "CREATE VIEW t18r1t2_g.view_source_reused AS SELECT a.id FROM t18r1t2_src.a AS a WHERE EXISTS (SELECT 1 FROM t18r1t2_src.a);"
run catalog "SELECT table_schema,table_name,is_updatable FROM information_schema.views WHERE table_schema='t18r1t2_g' ORDER BY table_name; SELECT table_schema,table_name,table_type FROM information_schema.tables WHERE table_schema IN ('t18r1t2_g','t18r1t2_src') ORDER BY table_schema,table_name;"
run cleanup "DROP DATABASE IF EXISTS t18r1t2_g; DROP DATABASE IF EXISTS t18r1t2_src;"
