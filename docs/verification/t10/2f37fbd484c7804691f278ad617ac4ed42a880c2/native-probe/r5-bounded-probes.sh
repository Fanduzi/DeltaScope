#!/bin/bash
# input: live mysql80/tidb85 docker anchors and golden-schema fixtures
# output: renamed-PK successor ops, freed `primary` name reuse, DROP PRIMARY
#   KEY member routing, and auto-name 61-64 byte suffix boundaries with rc
# pos: T10-R5 native evidence complementing r4-m10/r4-pk-presence probes
# note: re-run of the R5 probe set — every case is idempotent (fresh table).
MY='docker exec --env MYSQL_PWD=root deltascope-ddl-golden-mysql80 mysql -uroot'
TI='docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P4000 -uroot'
run() { echo; echo "=== $1 ==="; echo "\$ $2"; eval "$2"; echo "rc=$?"; }

echo "=== anchors ==="
run "versions" "$MY -e 'SELECT VERSION();' && $TI -e 'SELECT TIDB_VERSION();'"

echo; echo "=== R5-A TiDB: renamed-PK object (pk2) successor ops ==="
run "a1a pk2 ALTER VISIBLE no-op" "$TI -e 'DROP TABLE IF EXISTS golden.r5a1; CREATE TABLE golden.r5a1(id VARCHAR(32) PRIMARY KEY NONCLUSTERED, c INT); ALTER TABLE golden.r5a1 RENAME INDEX \`PRIMARY\` TO pk2; ALTER TABLE golden.r5a1 ALTER INDEX pk2 VISIBLE;'"
run "a1b pk2 ALTER INVISIBLE refused" "$TI -e 'DROP TABLE IF EXISTS golden.r5a1; CREATE TABLE golden.r5a1(id VARCHAR(32) PRIMARY KEY NONCLUSTERED, c INT); ALTER TABLE golden.r5a1 RENAME INDEX \`PRIMARY\` TO pk2; ALTER TABLE golden.r5a1 ALTER INDEX pk2 INVISIBLE; SHOW INDEX FROM golden.r5a1;'"
run "a1c common-handle DROP INDEX pk2 drops the PK object" "$TI -e 'DROP TABLE IF EXISTS golden.r5a2; CREATE TABLE golden.r5a2(id VARCHAR(32) PRIMARY KEY CLUSTERED, c INT); ALTER TABLE golden.r5a2 RENAME INDEX \`PRIMARY\` TO pk2; DROP INDEX pk2 ON golden.r5a2; SHOW INDEX FROM golden.r5a2; SELECT COUNT(*) pk_left FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA=\"golden\" AND TABLE_NAME=\"r5a2\" AND CONSTRAINT_TYPE=\"PRIMARY KEY\";'"
run "a1c2 post-drop confirmations" "$TI -e 'ALTER TABLE golden.r5a2 DROP PRIMARY KEY; ALTER TABLE golden.r5a2 RENAME INDEX pk2 TO pk3;'"
run "a1d renamed-object renames again pk2->pk3" "$TI -e 'DROP TABLE IF EXISTS golden.r5a3; CREATE TABLE golden.r5a3(id VARCHAR(32) PRIMARY KEY NONCLUSTERED, c INT); ALTER TABLE golden.r5a3 RENAME INDEX \`PRIMARY\` TO pk2; ALTER TABLE golden.r5a3 RENAME INDEX pk2 TO pk3; SHOW INDEX FROM golden.r5a3;'"
run "a1e freed primary name after rename" "$TI -e 'DROP INDEX \`PRIMARY\` ON golden.r5a3;'"

echo; echo "=== R5-A TiDB: DROP PRIMARY KEY routes member-first ==="
# Nonclustered PK object owns `primary` — a member cannot take the name
# (1061), so the member-route probe uses a PKIsHandle table where the
# synthesized PRIMARY row does not occupy the name against members.
run "a2a nonclustered member-primary coexistence refused" "$TI -e 'DROP TABLE IF EXISTS golden.r5b0; CREATE TABLE golden.r5b0(id VARCHAR(32) PRIMARY KEY NONCLUSTERED, c INT, KEY s(c)); ALTER TABLE golden.r5b0 RENAME INDEX s TO \`primary\`;'"
run "a2a-fixed drop-primary-key on handle table hits member primary" "$TI -e 'DROP TABLE IF EXISTS golden.r5b1; CREATE TABLE golden.r5b1(id INT PRIMARY KEY, c INT, KEY s(c)); ALTER TABLE golden.r5b1 RENAME INDEX s TO \`primary\`; ALTER TABLE golden.r5b1 DROP PRIMARY KEY; SHOW INDEX FROM golden.r5b1; SELECT COUNT(*) pk_left FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA=\"golden\" AND TABLE_NAME=\"r5b1\" AND CONSTRAINT_TYPE=\"PRIMARY KEY\"; SELECT TIDB_PK_TYPE FROM information_schema.tables WHERE table_schema=\"golden\" AND table_name=\"r5b1\";'"
run "a2b drop-primary-key after PK rename misses" "$TI -e 'DROP TABLE IF EXISTS golden.r5b2; CREATE TABLE golden.r5b2(id VARCHAR(32) PRIMARY KEY NONCLUSTERED, c INT); ALTER TABLE golden.r5b2 RENAME INDEX \`PRIMARY\` TO pk2; ALTER TABLE golden.r5b2 DROP PRIMARY KEY;'"

echo; echo "=== R5-A TiDB: freed primary name usable by members ==="
run "a3a member takes freed primary name" "$TI -e 'DROP TABLE IF EXISTS golden.r5c1; CREATE TABLE golden.r5c1(id VARCHAR(32) PRIMARY KEY NONCLUSTERED, c INT, KEY s(c)); ALTER TABLE golden.r5c1 RENAME INDEX \`PRIMARY\` TO pk3; ALTER TABLE golden.r5c1 RENAME INDEX s TO \`primary\`; SHOW INDEX FROM golden.r5c1;'"
run "a3b PK rename back onto occupied primary refused" "$TI -e 'ALTER TABLE golden.r5c1 RENAME INDEX pk3 TO \`primary\`;'"

echo; echo "=== R5-C MySQL: auto-name 61-byte truncation suffix ==="
# Two unnamed indexes on one long-named column: index1 = column name,
# index2 = collided auto-name under make_unique_key_name.
run "c1 mysql 62c col collides -> 61c+_2=63" "$MY -e 'DROP TABLE IF EXISTS golden.r5d1; CREATE TABLE golden.r5d1(aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa INT, c INT, KEY(aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa), KEY(aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa)); SHOW INDEX FROM golden.r5d1;'"
run "c2 mysql 63c col no-collision control" "$MY -e 'DROP TABLE IF EXISTS golden.r5d2; CREATE TABLE golden.r5d2(aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa INT, KEY(aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa)); SHOW INDEX FROM golden.r5d2;'"

echo; echo "=== R5-C TiDB: auto-name suffix does NOT truncate ==="
run "c3 tidb 62c col collides -> 62c+_2=64" "$TI -e 'DROP TABLE IF EXISTS golden.r5e1; CREATE TABLE golden.r5e1(aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa INT, c INT, KEY(aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa), KEY(aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa)); SHOW INDEX FROM golden.r5e1;'"
run "c4 tidb 64c col collides -> derived exceeds limit" "$TI -e 'DROP TABLE IF EXISTS golden.r5e2; CREATE TABLE golden.r5e2(aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa INT, c INT, KEY(aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa), KEY(aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa));'"
