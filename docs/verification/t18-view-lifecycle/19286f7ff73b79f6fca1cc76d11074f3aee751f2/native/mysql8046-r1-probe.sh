#!/usr/bin/env bash
set -u
name="$(cat /tmp/t18r1-container-name)"
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
run catalog_before "SELECT schema_name FROM information_schema.schemata WHERE schema_name IN ('t18r1_g','t18r1_src') ORDER BY schema_name; SELECT table_schema,table_name,table_type FROM information_schema.tables WHERE table_schema IN ('t18r1_g','t18r1_src') ORDER BY table_schema,table_name;"
run setup "DROP DATABASE IF EXISTS t18r1_g; DROP DATABASE IF EXISTS t18r1_src; CREATE DATABASE t18r1_g; CREATE DATABASE t18r1_src; CREATE TABLE t18r1_src.t(id INT); CREATE TABLE t18r1_src.u(uid INT); CREATE TABLE t18r1_src.a(id INT); CREATE TABLE t18r1_src.b(id INT);"
run f1_drop_schema "DROP DATABASE IF EXISTS t18r1_g;"
run f1_create_in_absent_schema "CREATE VIEW t18r1_g.v AS SELECT id FROM t18r1_src.t;"
run catalog_after_f1 "SELECT schema_name FROM information_schema.schemata WHERE schema_name IN ('t18r1_g','t18r1_src') ORDER BY schema_name; SELECT table_schema,table_name,table_type FROM information_schema.tables WHERE table_schema IN ('t18r1_g','t18r1_src') ORDER BY table_schema,table_name;"
run f1_recreate_schema "CREATE DATABASE t18r1_g;"
run f1_create_after_rebuild "CREATE VIEW t18r1_g.v AS SELECT id FROM t18r1_src.t;"
run f2_computed_only "CREATE VIEW t18r1_g.calc_v AS SELECT id + 1 AS x FROM t18r1_src.t WITH CHECK OPTION;"
run f2_outer_join "CREATE VIEW t18r1_g.join_v AS SELECT t.id, u.uid FROM t18r1_src.t LEFT JOIN t18r1_src.u ON t.id = u.uid WITH LOCAL CHECK OPTION;"
run f2_inner_join_control "CREATE VIEW t18r1_g.inner_v AS SELECT t.id, u.uid FROM t18r1_src.t JOIN t18r1_src.u ON t.id = u.uid WITH LOCAL CHECK OPTION;"
run f2_mixed_projection_control "CREATE VIEW t18r1_g.mixed_v AS SELECT t.id, t.id + 1 AS next_id FROM t18r1_src.t WITH CHECK OPTION;"
run f3_merge_create "CREATE ALGORITHM=MERGE VIEW t18r1_g.alg_v AS SELECT id FROM t18r1_src.t;"
run f3_alter_inherited "ALTER VIEW t18r1_g.alg_v AS SELECT COUNT(*) AS n FROM t18r1_src.t; SHOW WARNINGS; SHOW CREATE VIEW t18r1_g.alg_v;"
run f3_recreate_merge "DROP VIEW t18r1_g.alg_v; CREATE ALGORITHM=MERGE VIEW t18r1_g.alg_v AS SELECT id FROM t18r1_src.t;"
run f3_alter_undefined "ALTER ALGORITHM=UNDEFINED VIEW t18r1_g.alg_v AS SELECT COUNT(*) AS n FROM t18r1_src.t; SHOW WARNINGS; SHOW CREATE VIEW t18r1_g.alg_v;"
run f4_correlated "CREATE VIEW t18r1_g.corr_v AS SELECT a.id FROM t18r1_src.a AS a WHERE EXISTS (SELECT 1 FROM t18r1_src.b AS b WHERE EXISTS (SELECT 1 WHERE id > 0));"
run f4_prepare_sources "DROP VIEW IF EXISTS t18r1_g.corr_v; CREATE TABLE t18r1_src.a(id INT); CREATE TABLE t18r1_src.b(id INT); CREATE VIEW t18r1_g.corr_v AS SELECT a.id FROM t18r1_src.a AS a WHERE EXISTS (SELECT 1 FROM t18r1_src.b AS b WHERE EXISTS (SELECT 1 WHERE id > 0)); SHOW CREATE VIEW t18r1_g.corr_v;"
run catalog_final "SELECT table_schema,table_name,table_type FROM information_schema.tables WHERE table_schema IN ('t18r1_g','t18r1_src') ORDER BY table_schema,table_name;"
run cleanup "DROP DATABASE IF EXISTS t18r1_g; DROP DATABASE IF EXISTS t18r1_src;"
