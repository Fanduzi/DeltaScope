#!/bin/bash
# T10-R3 bounded re-verification probes — matrices NOT preserved in the
# R1/R2 consolidated pin log (blob c83362698c9b91664f5f7e6cd2a90890e38f4f72).
# Anchors: docker/ddl-golden-compose.yaml mysql80 (mysql:8.0.46), tidb85
# (pingcap/tidb:v8.5.0 unistore) via the tidb85-client sidecar (mysql:8.4.10).
M="docker exec --env MYSQL_PWD=root deltascope-ddl-golden-mysql80 mysql -uroot"
T="docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P4000 -uroot"
say() { echo; echo "=== $* ==="; }
run() { echo "\$ $2"; eval "$2" 2>&1; echo "rc=$?"; }

say "anchor versions + fixture db"
run m "$M -e 'SELECT VERSION();'"
run t "$T -e 'SELECT TIDB_VERSION(); CREATE DATABASE IF NOT EXISTS golden;'"

say "M1 fulltext prefix length (mysql80)"
run m "$M -e 'DROP TABLE IF EXISTS golden.t10r3_ft; CREATE TABLE golden.t10r3_ft(c TEXT);'"
run m "$M -e 'CREATE FULLTEXT INDEX ft5 ON golden.t10r3_ft(c(5));'"
run m "$M -e 'CREATE FULLTEXT INDEX ft0 ON golden.t10r3_ft(c(0));'"

say "M2 multi-valued cast dialect matrix (mysql80 then tidb85)"
run m "$M -e 'DROP TABLE IF EXISTS golden.t10r3_mv; CREATE TABLE golden.t10r3_mv(j JSON);'"
run m "$M -e 'CREATE INDEX mvd ON golden.t10r3_mv((CAST(j AS DECIMAL(10,2) ARRAY)));'"
run m "$M -e 'CREATE INDEX mvdbl ON golden.t10r3_mv((CAST(j AS DOUBLE ARRAY)));'"
run t "$T -e 'DROP TABLE IF EXISTS golden.t10r3_mv; CREATE TABLE golden.t10r3_mv(j JSON);'"
run t "$T -e 'CREATE INDEX mvd ON golden.t10r3_mv((CAST(j AS DECIMAL(10,2) ARRAY)));'"
run t "$T -e 'CREATE INDEX mvdbl ON golden.t10r3_mv((CAST(j AS DOUBLE ARRAY)));'"

say "M3 tidb clustered defaults + alter-primary-key + int_only handle (tidb85)"
run t "$T -e \"SHOW VARIABLES LIKE 'tidb_enable_clustered_index';\""
run t "$T -e 'SHOW CONFIG;' | grep -i 'alter-primary-key'"
run t "$T -e 'DROP TABLE IF EXISTS golden.t10r3_pk; CREATE TABLE golden.t10r3_pk(id INT PRIMARY KEY, c INT);'"
run t "$T -e 'SELECT TIDB_PK_TYPE FROM information_schema.tables WHERE table_schema=\"golden\" AND table_name=\"t10r3_pk\";'"
run t "$T -e \"SET GLOBAL tidb_enable_clustered_index='int_only';\""
run t "$T -e 'DROP TABLE IF EXISTS golden.t10r3_pk2; CREATE TABLE golden.t10r3_pk2(id INT PRIMARY KEY, c INT);'"
run t "$T -e 'SELECT TIDB_PK_TYPE FROM information_schema.tables WHERE table_schema=\"golden\" AND table_name=\"t10r3_pk2\";'"
run t "$T -e 'SELECT INDEX_NAME,CLUSTERED FROM information_schema.TIDB_INDEXES WHERE table_schema=\"golden\" AND table_name=\"t10r3_pk2\";'"
run t "$T -e \"SET GLOBAL tidb_enable_clustered_index='on';\""
run t "$T -e 'DROP TABLE IF EXISTS golden.t10r3_pk3; CREATE TABLE golden.t10r3_pk3(id INT PRIMARY KEY, c INT);'"
run t "$T -e 'SELECT TIDB_PK_TYPE FROM information_schema.tables WHERE table_schema=\"golden\" AND table_name=\"t10r3_pk3\";'"

say "M4 explicit PK frees later UNIQUE INVISIBLE (mysql80)"
run m "$M -e 'DROP TABLE IF EXISTS golden.t10r3_pk; CREATE TABLE golden.t10r3_pk(id INT PRIMARY KEY, c INT);'"
run m "$M -e 'ALTER TABLE golden.t10r3_pk ADD UNIQUE (c) INVISIBLE;'"
run m "$M -e 'SHOW INDEX FROM golden.t10r3_pk;'"

say "M5 unnamed index stored names (mysql80)"
run m "$M -e 'DROP TABLE IF EXISTS golden.t10r3_names; CREATE TABLE golden.t10r3_names(a INT NOT NULL, b INT NOT NULL, UNIQUE(a), UNIQUE(b) INVISIBLE);'"
run m "$M -e 'SHOW INDEX FROM golden.t10r3_names;'"
run m "$M -e 'DROP TABLE IF EXISTS golden.t10r3_names2; CREATE TABLE golden.t10r3_names2(c VARCHAR(32), KEY(c(3)), KEY(c(4)));'"
run m "$M -e 'SHOW INDEX FROM golden.t10r3_names2;'"

say "M6 expression member direction (mysql80)"
run m "$M -e 'DROP TABLE IF EXISTS golden.t10r3_expr; CREATE TABLE golden.t10r3_expr(a INT, b INT);'"
run m "$M -e 'CREATE INDEX ie ON golden.t10r3_expr( ((a+b)) DESC, b );'"
run m "$M -e 'SELECT INDEX_NAME,COLUMN_NAME,EXPRESSION,COLLATION,SEQ_IN_INDEX FROM information_schema.STATISTICS WHERE table_schema=\"golden\" AND table_name=\"t10r3_expr\";'"
