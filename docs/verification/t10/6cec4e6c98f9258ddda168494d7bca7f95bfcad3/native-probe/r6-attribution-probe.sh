#!/bin/bash
# input: host TiDB 8.5 anchor on 127.0.0.1:24000 (root, empty password)
# output: minimal catalog-shape facts for T10-R6 provider attribution —
#         same-sequence PK candidates and reversed-column-order candidates
# pos: T10-R6 bounded native corroboration; reuses archived pk2 basics
# note: if this file changes, update this header and module README.md.
set -u
MYSQL="mysql --protocol=tcp -h 127.0.0.1 -P 24000 -uroot --password= -Nse"
run() { echo "## $1"; shift; echo "$ $*"; "$@" 2>&1; echo "rc=$?"; echo; }

run "version" $MYSQL "SELECT VERSION()"
run "drop db" $MYSQL "DROP DATABASE IF EXISTS r6probe"
run "create db" $MYSQL "CREATE DATABASE r6probe"

# A1: same-sequence candidates — PK(id) NONCLUSTERED + UNIQUE aa(id),
# then rename the PK object to pk2. Catalog must show aa(id) + pk2(id)
# both clustered=NO with ordered KCU PRIMARY [id]: no catalog fact can
# uniquely attribute the PK object by column sequence alone.
run "a1 create" $MYSQL "CREATE TABLE r6probe.t1(id VARCHAR(32) NOT NULL, PRIMARY KEY(id) NONCLUSTERED, UNIQUE KEY aa(id))"
run "a1 rename" $MYSQL "ALTER TABLE r6probe.t1 RENAME INDEX \`PRIMARY\` TO pk2"
run "a1 statistics" $MYSQL "SELECT KEY_NAME, COLUMN_NAME, SEQ_IN_INDEX, CLUSTERED FROM information_schema.TIDB_INDEXES WHERE TABLE_SCHEMA='r6probe' AND TABLE_NAME='t1' ORDER BY KEY_NAME, SEQ_IN_INDEX"
run "a1 kcu" $MYSQL "SELECT CONSTRAINT_NAME, COLUMN_NAME, ORDINAL_POSITION FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA='r6probe' AND TABLE_NAME='t1' AND CONSTRAINT_NAME='PRIMARY' ORDER BY ORDINAL_POSITION"
run "a1 constraints" $MYSQL "SELECT CONSTRAINT_NAME, CONSTRAINT_TYPE FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA='r6probe' AND TABLE_NAME='t1' ORDER BY CONSTRAINT_NAME"
run "a1 show index" $MYSQL "SHOW INDEX FROM r6probe.t1"

# A2: reversed column order — PK(a,b) NONCLUSTERED + UNIQUE aa(b,a).
# Ordered KCU PRIMARY [a,b] must distinguish the PK object from the
# reversed member sequence [b,a] in STATISTICS.
run "a2 create" $MYSQL "CREATE TABLE r6probe.t2(a INT NOT NULL, b INT NOT NULL, PRIMARY KEY(a,b) NONCLUSTERED, UNIQUE KEY aa(b,a))"
run "a2 rename" $MYSQL "ALTER TABLE r6probe.t2 RENAME INDEX \`PRIMARY\` TO pk2"
run "a2 statistics" $MYSQL "SELECT KEY_NAME, COLUMN_NAME, SEQ_IN_INDEX, CLUSTERED FROM information_schema.TIDB_INDEXES WHERE TABLE_SCHEMA='r6probe' AND TABLE_NAME='t2' ORDER BY KEY_NAME, SEQ_IN_INDEX"
run "a2 kcu" $MYSQL "SELECT CONSTRAINT_NAME, COLUMN_NAME, ORDINAL_POSITION FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA='r6probe' AND TABLE_NAME='t2' AND CONSTRAINT_NAME='PRIMARY' ORDER BY ORDINAL_POSITION"

# A3: single clustered YES candidate control — clustered PK keeps
# CLUSTERED=YES on the object row while members report NO.
run "a3 create" $MYSQL "CREATE TABLE r6probe.t3(id INT NOT NULL PRIMARY KEY CLUSTERED, u INT, UNIQUE KEY uq(u))"
run "a3 statistics" $MYSQL "SELECT KEY_NAME, COLUMN_NAME, SEQ_IN_INDEX, CLUSTERED FROM information_schema.TIDB_INDEXES WHERE TABLE_SCHEMA='r6probe' AND TABLE_NAME='t3' ORDER BY KEY_NAME, SEQ_IN_INDEX"
run "a3 kcu" $MYSQL "SELECT CONSTRAINT_NAME, COLUMN_NAME, ORDINAL_POSITION FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA='r6probe' AND TABLE_NAME='t3' AND CONSTRAINT_NAME='PRIMARY' ORDER BY ORDINAL_POSITION"
