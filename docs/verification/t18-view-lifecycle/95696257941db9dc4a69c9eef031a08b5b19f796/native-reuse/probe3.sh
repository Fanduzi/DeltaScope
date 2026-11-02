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
run setup "DROP DATABASE IF EXISTS t18r1t2_g; DROP DATABASE IF EXISTS t18r1t2_src; CREATE DATABASE t18r1t2_g; CREATE DATABASE t18r1t2_src; CREATE TABLE t18r1t2_src.t(id INT); CREATE TABLE t18r1t2_src.u(id INT); CREATE VIEW t18r1t2_src.a AS SELECT id FROM t18r1t2_src.t;"
run with_unused_same "CREATE VIEW t18r1t2_g.with_unused_same AS WITH c AS (SELECT 1 FROM t18r1t2_src.t) SELECT t.id FROM t18r1t2_src.t AS t;"
run with_unused_diff "CREATE VIEW t18r1t2_g.with_unused_diff AS WITH c AS (SELECT 1 FROM t18r1t2_src.u) SELECT t.id FROM t18r1t2_src.t AS t;"
run main_via_view_subquery "CREATE VIEW t18r1t2_g.main_via_view_subquery AS SELECT t.id FROM t18r1t2_src.t AS t WHERE EXISTS (SELECT 1 FROM t18r1t2_src.a);"
run view_main_base_subquery "CREATE VIEW t18r1t2_g.view_main_base_subquery AS SELECT a.id FROM t18r1t2_src.a AS a WHERE EXISTS (SELECT 1 FROM t18r1t2_src.t);"
run same_table_second_alias "CREATE VIEW t18r1t2_g.same_table_second_alias AS SELECT t.id FROM t18r1t2_src.t AS t WHERE EXISTS (SELECT 1 FROM t18r1t2_src.t AS t2);"
run catalog "SELECT table_schema,table_name,is_updatable FROM information_schema.views WHERE table_schema='t18r1t2_g' ORDER BY table_name;"
run cleanup "DROP DATABASE IF EXISTS t18r1t2_g; DROP DATABASE IF EXISTS t18r1t2_src;"
