#!/bin/bash
# T10-R4 M10: name-keyed ops targeting `primary` — DROP/RENAME/ADD/CREATE INDEX.
# Answers the review question: when a statement names `primary`, does the
# product hit the PK, a member, or refuse?
MY='docker exec --env MYSQL_PWD=root deltascope-ddl-golden-mysql80 mysql -uroot'
TI='docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P4000 -uroot'
run() { echo; echo "=== $1 ==="; echo "\$ $2"; eval "$2"; echo "rc=$?"; }

echo "=== anchors ==="
run "versions" "$MY -e 'SELECT VERSION();' && $TI -e 'SELECT TIDB_VERSION();'"

echo; echo "=== M10 MySQL: DROP INDEX \`PRIMARY\` (standalone + alter) ==="
run "mysql standalone drop-index-primary" "$MY -e 'DROP TABLE IF EXISTS golden.m10a; CREATE TABLE golden.m10a(id INT PRIMARY KEY, c INT, KEY s(c)); DROP INDEX \`PRIMARY\` ON golden.m10a;'"
run "mysql post-state" "$MY -e 'SHOW INDEX FROM golden.m10a; SELECT COUNT(*) pk_left FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA=\"golden\" AND TABLE_NAME=\"m10a\" AND CONSTRAINT_TYPE=\"PRIMARY KEY\";'"
run "mysql alter-form drop-index-primary" "$MY -e 'DROP TABLE IF EXISTS golden.m10b; CREATE TABLE golden.m10b(id INT PRIMARY KEY, c INT); ALTER TABLE golden.m10b DROP INDEX \`PRIMARY\`;'"
run "mysql alter post-state" "$MY -e 'SHOW INDEX FROM golden.m10b; SELECT COUNT(*) pk_left FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA=\"golden\" AND TABLE_NAME=\"m10b\" AND CONSTRAINT_TYPE=\"PRIMARY KEY\";'"

echo; echo "=== M10 MySQL: RENAME INDEX primary ==="
run "mysql rename primary -> pk2" "$MY -e 'DROP TABLE IF EXISTS golden.m10c; CREATE TABLE golden.m10c(id INT PRIMARY KEY, c INT); ALTER TABLE golden.m10c RENAME INDEX \`PRIMARY\` TO pk2;'"
run "mysql rename member -> primary" "$MY -e 'DROP TABLE IF EXISTS golden.m10d; CREATE TABLE golden.m10d(id INT PRIMARY KEY, c INT, KEY s(c)); ALTER TABLE golden.m10d RENAME INDEX s TO \`primary\`;'"
run "mysql rename member post" "$MY -e 'SHOW INDEX FROM golden.m10d;'"

echo; echo "=== M10 MySQL: ADD/CREATE INDEX named primary ==="
run "mysql add-index-primary" "$MY -e 'DROP TABLE IF EXISTS golden.m10e; CREATE TABLE golden.m10e(id INT PRIMARY KEY, c INT); ALTER TABLE golden.m10e ADD INDEX \`primary\`(c);'"
run "mysql create-index-primary" "$MY -e 'DROP TABLE IF EXISTS golden.m10f; CREATE TABLE golden.m10f(id INT PRIMARY KEY, c INT); CREATE INDEX \`primary\` ON golden.m10f(c);'"

echo; echo "=== M10 TiDB: DROP INDEX \`PRIMARY\` by PK shape ==="
run "tidb mode=on" "$TI -e \"SET GLOBAL tidb_enable_clustered_index='on';\""
run "tidb nonclustered standalone drop-index-primary" "$TI -e 'DROP TABLE IF EXISTS golden.m10g; CREATE TABLE golden.m10g(id VARCHAR(32) PRIMARY KEY NONCLUSTERED, c INT); DROP INDEX \`PRIMARY\` ON golden.m10g;'"
run "tidb nonclustered post-state" "$TI -e 'SHOW INDEX FROM golden.m10g; SELECT TIDB_PK_TYPE FROM information_schema.tables WHERE table_schema=\"golden\" AND table_name=\"m10g\";'"
run "tidb nonclustered alter-form drop" "$TI -e 'DROP TABLE IF EXISTS golden.m10h; CREATE TABLE golden.m10h(id VARCHAR(32) PRIMARY KEY NONCLUSTERED, c INT); ALTER TABLE golden.m10h DROP INDEX \`PRIMARY\`;'"
run "tidb common-handle drop-index-primary" "$TI -e 'DROP TABLE IF EXISTS golden.m10i; CREATE TABLE golden.m10i(id VARCHAR(32) PRIMARY KEY CLUSTERED, c INT); DROP INDEX \`PRIMARY\` ON golden.m10i;'"
run "tidb pkishandle drop-index-primary" "$TI -e 'DROP TABLE IF EXISTS golden.m10j; CREATE TABLE golden.m10j(id INT PRIMARY KEY, c INT); DROP INDEX \`PRIMARY\` ON golden.m10j;'"

echo; echo "=== M10 TiDB: RENAME INDEX primary ==="
run "tidb nonclustered rename primary -> pk2" "$TI -e 'DROP TABLE IF EXISTS golden.m10k; CREATE TABLE golden.m10k(id VARCHAR(32) PRIMARY KEY NONCLUSTERED, c INT); ALTER TABLE golden.m10k RENAME INDEX \`PRIMARY\` TO pk2; SHOW INDEX FROM golden.m10k;'"
run "tidb common-handle rename primary -> pk2" "$TI -e 'DROP TABLE IF EXISTS golden.m10l; CREATE TABLE golden.m10l(id VARCHAR(32) PRIMARY KEY CLUSTERED, c INT); ALTER TABLE golden.m10l RENAME INDEX \`PRIMARY\` TO pk2;'"
run "tidb pkishandle rename primary -> pk2" "$TI -e 'DROP TABLE IF EXISTS golden.m10m; CREATE TABLE golden.m10m(id INT PRIMARY KEY, c INT); ALTER TABLE golden.m10m RENAME INDEX \`PRIMARY\` TO pk2;'"
run "tidb rename member -> primary" "$TI -e 'DROP TABLE IF EXISTS golden.m10n; CREATE TABLE golden.m10n(id VARCHAR(32) PRIMARY KEY NONCLUSTERED, c INT, KEY s(c)); ALTER TABLE golden.m10n RENAME INDEX s TO \`primary\`; SHOW INDEX FROM golden.m10n;'"

echo; echo "=== M10 TiDB: member named primary takes name precedence ==="
run "tidb drop-index-primary w/ member+pk" "$TI -e 'DROP TABLE IF EXISTS golden.m10o; CREATE TABLE golden.m10o(id VARCHAR(32) PRIMARY KEY NONCLUSTERED, c INT, KEY s(c)); ALTER TABLE golden.m10o RENAME INDEX s TO \`primary\`; DROP INDEX \`primary\` ON golden.m10o; SHOW INDEX FROM golden.m10o; SELECT TIDB_PK_TYPE FROM information_schema.tables WHERE table_schema=\"golden\" AND table_name=\"m10o\";'"

echo; echo "=== M10 TiDB: ALTER INDEX PRIMARY INVISIBLE on real object ==="
run "tidb nonclustered invisible" "$TI -e 'DROP TABLE IF EXISTS golden.m10p; CREATE TABLE golden.m10p(id VARCHAR(32) PRIMARY KEY NONCLUSTERED, c INT); ALTER TABLE golden.m10p ALTER INDEX \`PRIMARY\` INVISIBLE;'"
run "tidb common-handle invisible" "$TI -e 'DROP TABLE IF EXISTS golden.m10q; CREATE TABLE golden.m10q(id VARCHAR(32) PRIMARY KEY CLUSTERED, c INT); ALTER TABLE golden.m10q ALTER INDEX \`PRIMARY\` INVISIBLE;'"
run "tidb pkishandle invisible" "$TI -e 'DROP TABLE IF EXISTS golden.m10r; CREATE TABLE golden.m10r(id INT PRIMARY KEY, c INT); ALTER TABLE golden.m10r ALTER INDEX \`PRIMARY\` INVISIBLE;'"
run "mysql alter-index-primary visible" "$MY -e 'DROP TABLE IF EXISTS golden.m10s; CREATE TABLE golden.m10s(id INT PRIMARY KEY, c INT); ALTER TABLE golden.m10s ALTER INDEX \`PRIMARY\` VISIBLE;'"

echo; echo "=== M10b TiDB: member `primary` coexists with PKIsHandle — name is free ==="

run "tidb pkishandle rename member -> primary" "$TI -e 'DROP TABLE IF EXISTS golden.m10t; CREATE TABLE golden.m10t(id INT PRIMARY KEY, c INT, KEY s(c)); ALTER TABLE golden.m10t RENAME INDEX s TO \`primary\`; SHOW INDEX FROM golden.m10t;'"
run "tidb pkishandle member-primary drop hits member not pk" "$TI -e 'DROP TABLE IF EXISTS golden.m10u; CREATE TABLE golden.m10u(id INT PRIMARY KEY, c INT, KEY s(c)); ALTER TABLE golden.m10u RENAME INDEX s TO \`primary\`; DROP INDEX \`primary\` ON golden.m10u; SHOW INDEX FROM golden.m10u; SELECT COUNT(*) pk_left FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA=\"golden\" AND TABLE_NAME=\"m10u\" AND CONSTRAINT_TYPE=\"PRIMARY KEY\";'"
run "tidb pkishandle member-primary drop UPPERCASE name" "$TI -e 'DROP TABLE IF EXISTS golden.m10v; CREATE TABLE golden.m10v(id INT PRIMARY KEY, c INT, KEY s(c)); ALTER TABLE golden.m10v RENAME INDEX s TO \`primary\`; DROP INDEX \`PRIMARY\` ON golden.m10v; SHOW INDEX FROM golden.m10v;'"
run "tidb member-primary visibility flip" "$TI -e 'DROP TABLE IF EXISTS golden.m10w; CREATE TABLE golden.m10w(id INT PRIMARY KEY, c INT, KEY s(c)); ALTER TABLE golden.m10w RENAME INDEX s TO \`primary\`; ALTER TABLE golden.m10w ALTER INDEX \`primary\` INVISIBLE; SHOW INDEX FROM golden.m10w;'"
run "tidb pk-less member-primary visibility" "$TI -e 'DROP TABLE IF EXISTS golden.m10x; CREATE TABLE golden.m10x(c INT, KEY s(c)); ALTER TABLE golden.m10x RENAME INDEX s TO \`primary\`; ALTER TABLE golden.m10x ALTER INDEX \`primary\` INVISIBLE; SHOW INDEX FROM golden.m10x;'"
run "tidb pk-less authored member primary refused" "$TI -e 'DROP TABLE IF EXISTS golden.m10y; CREATE TABLE golden.m10y(c INT); ALTER TABLE golden.m10y ADD INDEX \`primary\`(c);'"

echo; echo "=== cleanup ==="
run "cleanup mysql" "$MY -e 'DROP TABLE IF EXISTS golden.m10a,golden.m10b,golden.m10c,golden.m10d,golden.m10e,golden.m10f,golden.m10s;'"
run "cleanup tidb" "$TI -e 'DROP TABLE IF EXISTS golden.m10g,golden.m10h,golden.m10i,golden.m10j,golden.m10k,golden.m10l,golden.m10m,golden.m10n,golden.m10o,golden.m10p,golden.m10q,golden.m10r,golden.m10t,golden.m10u,golden.m10v,golden.m10w,golden.m10x,golden.m10y;' && $TI -e \"SET GLOBAL tidb_enable_clustered_index='int_only';\""
