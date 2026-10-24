#!/bin/bash
# T10-R4 bounded probe — pk-presence evidence for resolveTiDBPrimaryName.
# Pins why information_schema.tables.tidb_pk_type CANNOT prove PK existence
# (NONCLUSTERED covers both PK-less tables and real nonclustered primaries)
# and why information_schema.table_constraints + a `primary`-fold
# STATISTICS row is the correct binding evidence.
# Anchor: docker/ddl-golden-compose.yaml tidb85 (pingcap/tidb:v8.5.0 unistore)
# via the tidb85-client sidecar (mysql:8.4.10).
T="docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P4000 -uroot"
say() { echo; echo "=== $* ==="; }
run() { echo "\$ $2"; eval "$2" 2>&1; echo "rc=$?"; }

say "anchor version + fixture db"
run t "$T -e 'SELECT TIDB_VERSION(); CREATE DATABASE IF NOT EXISTS golden;'"

say "fixture set: pk-less / member-primary / real nonclustered PK / clustered handle / clustered common-handle"
run t "$T -e 'DROP TABLE IF EXISTS golden.pp_a,golden.pp_b,golden.pp_c,golden.pp_d,golden.pp_e;'"
run t "$T -e 'CREATE TABLE golden.pp_a(id INT, c INT, UNIQUE KEY u_a(c));'"
run t "$T -e 'CREATE TABLE golden.pp_b(c INT, KEY s(c)); ALTER TABLE golden.pp_b RENAME INDEX s TO \`primary\`;'"
run t "$T -e 'CREATE TABLE golden.pp_c(id INT PRIMARY KEY NONCLUSTERED, c INT);'"
run t "$T -e 'CREATE TABLE golden.pp_d(id INT PRIMARY KEY, c INT);'"
run t "$T -e 'CREATE TABLE golden.pp_e(id INT, b VARCHAR(8), PRIMARY KEY(id,b) CLUSTERED);'"

say "P1 tidb_pk_type per table — PK-less vs real PK conflation"
run t "$T -e \"SELECT table_name, tidb_pk_type FROM information_schema.tables WHERE table_schema='golden' AND table_name LIKE 'pp_%' ORDER BY table_name;\""

say "P2 table_constraints per table — real PK object rows"
run t "$T -e \"SELECT table_name, constraint_name, constraint_type FROM information_schema.table_constraints WHERE table_schema='golden' AND table_name LIKE 'pp_%' ORDER BY table_name;\""

say "P3 statistics primary-fold rows per table — object vs member"
run t "$T -e \"SELECT table_name, index_name, non_unique FROM information_schema.statistics WHERE table_schema='golden' AND table_name LIKE 'pp_%' ORDER BY table_name, index_name;\""

say "P4 tidb_indexes CLUSTERED per table — storage-mode fact (not PK presence)"
run t "$T -e \"SELECT table_name, key_name, clustered FROM information_schema.tidb_indexes WHERE table_schema='golden' AND table_name LIKE 'pp_%' ORDER BY table_name;\""

say "cleanup"
run t "$T -e 'DROP TABLE IF EXISTS golden.pp_a,golden.pp_b,golden.pp_c,golden.pp_d,golden.pp_e;'"
