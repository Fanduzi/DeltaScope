#!/usr/bin/env bash
# p13 — T14 partition topology + maintenance native matrix.
# Records per-statement rc/stdout/stderr, catalog ordinal positions,
# SHOW CREATE, SHOW WARNINGS where semantics are warning-sensitive, and
# real row placement for the golden REORGANIZE arm. Fixture rows live in
# schema `golden`; every arm rebuilds its own tables and cleans up.
set -u

run() {
  local anchor="$1"; shift
  local label="$1"; shift
  local sql="$1"
  echo "=== [$anchor] $label"
  echo "--- SQL: $sql"
  case "$anchor" in
    tidb85)
      out=$(docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P4000 -uroot -e "$sql" 2>&1)
      ;;
    *)
      out=$(docker exec "deltascope-ddl-golden-$anchor" mysql -uroot -proot -e "$sql" 2>&1)
      ;;
  esac
  rc=$?
  echo "--- rc=$rc"
  echo "--- out: $out"
}

reset_range() {
  local anchor="$1"
  run "$anchor" reset-t14r \
    "DROP TABLE IF EXISTS golden.t14r; CREATE TABLE golden.t14r (id INT) ENGINE=InnoDB PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (200), PARTITION pmax VALUES LESS THAN MAXVALUE); INSERT INTO golden.t14r VALUES (100),(150)"
}
reset_range_nomax() {
  local anchor="$1"
  run "$anchor" reset-t14rn \
    "DROP TABLE IF EXISTS golden.t14rn; CREATE TABLE golden.t14rn (id INT) ENGINE=InnoDB PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (200), PARTITION p1 VALUES LESS THAN (400))"
}
reset_list() {
  local anchor="$1"
  run "$anchor" reset-t14l \
    "DROP TABLE IF EXISTS golden.t14l; CREATE TABLE golden.t14l (id INT) ENGINE=InnoDB PARTITION BY LIST(id) (PARTITION p0 VALUES IN (1,2), PARTITION p1 VALUES IN (3,4))"
}
reset_hash() {
  local anchor="$1"
  run "$anchor" reset-t14h \
    "DROP TABLE IF EXISTS golden.t14h; CREATE TABLE golden.t14h (id INT) ENGINE=InnoDB PARTITION BY HASH(id) PARTITIONS 4; INSERT INTO golden.t14h VALUES (1),(2),(3),(4),(5)"
}
reset_plain() {
  local anchor="$1"
  run "$anchor" reset-t14n \
    "DROP TABLE IF EXISTS golden.t14n; CREATE TABLE golden.t14n (id INT) ENGINE=InnoDB; INSERT INTO golden.t14n VALUES (1),(2)"
}
directory() {
  local anchor="$1" table="$2"
  run "$anchor" "dir-$table" \
    "SELECT PARTITION_NAME, PARTITION_ORDINAL_POSITION, PARTITION_METHOD, PARTITION_DESCRIPTION, TABLE_ROWS FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='$table' AND PARTITION_NAME IS NOT NULL ORDER BY PARTITION_ORDINAL_POSITION"
}
rows_in() {
  local anchor="$1" table="$2"
  run "$anchor" "rows-$table" \
    "SELECT COUNT(*) AS total FROM golden.$table; SELECT id FROM golden.$table ORDER BY id"
}

for anchor in ${ANCHORS:-mysql57 mysql80 mysql84 tidb85}; do
  echo "############ ANCHOR $anchor ############"
  run "$anchor" version "SELECT VERSION()"
  run "$anchor" ensure-db "CREATE DATABASE IF NOT EXISTS golden"

  # ============ A. REORGANIZE (golden path + variants) ============
  reset_range "$anchor"
  run "$anchor" reorg-range-exec \
    "ALTER TABLE golden.t14r REORGANIZE PARTITION p0 INTO (PARTITION p1 VALUES LESS THAN (120), PARTITION p2 VALUES LESS THAN (200)); SHOW WARNINGS"
  directory "$anchor" t14r
  run "$anchor" reorg-range-placement "SELECT id FROM golden.t14r PARTITION(p1); SELECT id FROM golden.t14r PARTITION(p2); SELECT COUNT(*) AS total FROM golden.t14r"

  reset_range "$anchor"
  run "$anchor" reorg-range-missing-source \
    "ALTER TABLE golden.t14r REORGANIZE PARTITION pNOPE INTO (PARTITION q1 VALUES LESS THAN (120))"
  run "$anchor" reorg-range-name-collision \
    "ALTER TABLE golden.t14r REORGANIZE PARTITION p0 INTO (PARTITION pmax VALUES LESS THAN (120), PARTITION p2 VALUES LESS THAN (200))"
  run "$anchor" reorg-range-overlap-bound \
    "ALTER TABLE golden.t14r REORGANIZE PARTITION p0 INTO (PARTITION p1 VALUES LESS THAN (120), PARTITION p2 VALUES LESS THAN (500))"
  run "$anchor" reorg-range-shrink-bound \
    "ALTER TABLE golden.t14r REORGANIZE PARTITION p0 INTO (PARTITION p1 VALUES LESS THAN (120), PARTITION p2 VALUES LESS THAN (180))"
  run "$anchor" reorg-range-maxvalue-source \
    "ALTER TABLE golden.t14r REORGANIZE PARTITION pmax INTO (PARTITION pm1 VALUES LESS THAN (500), PARTITION pm2 VALUES LESS THAN MAXVALUE); SHOW WARNINGS"
  directory "$anchor" t14r
  run "$anchor" reorg-range-multi-src \
    "ALTER TABLE golden.t14r REORGANIZE PARTITION pm1, pm2 INTO (PARTITION pz VALUES LESS THAN MAXVALUE); SHOW WARNINGS"
  directory "$anchor" t14r

  reset_list "$anchor"
  run "$anchor" reorg-list-exec \
    "ALTER TABLE golden.t14l REORGANIZE PARTITION p0 INTO (PARTITION pa VALUES IN (1), PARTITION pb VALUES IN (2,5)); SHOW WARNINGS"
  directory "$anchor" t14l
  run "$anchor" reorg-list-drop-value \
    "ALTER TABLE golden.t14l REORGANIZE PARTITION pa INTO (PARTITION px VALUES IN (9))"

  reset_hash "$anchor"
  run "$anchor" reorg-hash \
    "ALTER TABLE golden.t14h REORGANIZE PARTITION p0 INTO (PARTITION q0)"

  reset_plain "$anchor"
  run "$anchor" reorg-nonpart \
    "ALTER TABLE golden.t14n REORGANIZE PARTITION p0 INTO (PARTITION q0 VALUES LESS THAN (10))"

  # ============ B. ADD PARTITION ============
  reset_range_nomax "$anchor"
  run "$anchor" add-range-defs \
    "ALTER TABLE golden.t14rn ADD PARTITION (PARTITION p2 VALUES LESS THAN (600)); SHOW WARNINGS"
  directory "$anchor" t14rn
  run "$anchor" add-range-name-collision \
    "ALTER TABLE golden.t14rn ADD PARTITION (PARTITION p2 VALUES LESS THAN (700))"
  run "$anchor" add-range-low-bound \
    "ALTER TABLE golden.t14rn ADD PARTITION (PARTITION p3 VALUES LESS THAN (100))"
  run "$anchor" add-range-maxvalue-tail \
    "ALTER TABLE golden.t14rn ADD PARTITION (PARTITION p4 VALUES LESS THAN MAXVALUE)"
  run "$anchor" add-range-partitions-n \
    "ALTER TABLE golden.t14rn ADD PARTITION PARTITIONS 2"

  reset_range "$anchor"
  run "$anchor" add-range-maxtail \
    "ALTER TABLE golden.t14r ADD PARTITION (PARTITION p3 VALUES LESS THAN (600))"

  reset_list "$anchor"
  run "$anchor" add-list-defs \
    "ALTER TABLE golden.t14l ADD PARTITION (PARTITION p2 VALUES IN (9,10))"
  directory "$anchor" t14l
  run "$anchor" add-list-dup-value \
    "ALTER TABLE golden.t14l ADD PARTITION (PARTITION p3 VALUES IN (9))"

  reset_hash "$anchor"
  run "$anchor" add-hash-partitions-n \
    "ALTER TABLE golden.t14h ADD PARTITION PARTITIONS 2; SHOW WARNINGS"
  directory "$anchor" t14h
  run "$anchor" add-hash-defs \
    "ALTER TABLE golden.t14h ADD PARTITION (PARTITION p9)"

  reset_plain "$anchor"
  run "$anchor" add-nonpart \
    "ALTER TABLE golden.t14n ADD PARTITION (PARTITION p0 VALUES LESS THAN (10))"

  # ============ C. COALESCE ============
  reset_hash "$anchor"
  run "$anchor" coalesce-hash-1 \
    "ALTER TABLE golden.t14h COALESCE PARTITION 1; SHOW WARNINGS"
  directory "$anchor" t14h
  rows_in "$anchor" t14h
  reset_hash "$anchor"
  run "$anchor" coalesce-hash-4 \
    "ALTER TABLE golden.t14h COALESCE PARTITION 4"
  run "$anchor" coalesce-hash-9 \
    "ALTER TABLE golden.t14h COALESCE PARTITION 9"
  reset_range_nomax "$anchor"
  run "$anchor" coalesce-range \
    "ALTER TABLE golden.t14rn COALESCE PARTITION 1"
  reset_plain "$anchor"
  run "$anchor" coalesce-nonpart \
    "ALTER TABLE golden.t14n COALESCE PARTITION 1"

  # ============ D. PARTITION BY redefine / REMOVE PARTITIONING ============
  reset_plain "$anchor"
  run "$anchor" pby-plain-hash \
    "ALTER TABLE golden.t14n PARTITION BY HASH(id) PARTITIONS 2; SHOW WARNINGS"
  directory "$anchor" t14n
  rows_in "$anchor" t14n
  reset_range "$anchor"
  run "$anchor" pby-range-to-hash \
    "ALTER TABLE golden.t14r PARTITION BY KEY(id) PARTITIONS 3; SHOW WARNINGS"
  directory "$anchor" t14r
  rows_in "$anchor" t14r
  reset_range "$anchor"
  run "$anchor" pby-range-to-range \
    "ALTER TABLE golden.t14r PARTITION BY RANGE(id) (PARTITION a VALUES LESS THAN (300), PARTITION b VALUES LESS THAN MAXVALUE); SHOW WARNINGS"
  directory "$anchor" t14r

  reset_range "$anchor"
  run "$anchor" remove-partitioning \
    "ALTER TABLE golden.t14r REMOVE PARTITIONING; SHOW WARNINGS"
  directory "$anchor" t14r
  rows_in "$anchor" t14r
  run "$anchor" remove-nonpart \
    "ALTER TABLE golden.t14r REMOVE PARTITIONING"
  reset_plain "$anchor"
  run "$anchor" remove-plain-nonpart \
    "ALTER TABLE golden.t14n REMOVE PARTITIONING"

  # ============ E. Maintenance ops ============
  reset_range "$anchor"
  run "$anchor" maint-rebuild-p0 \
    "ALTER TABLE golden.t14r REBUILD PARTITION p0; SHOW WARNINGS"
  run "$anchor" maint-rebuild-all \
    "ALTER TABLE golden.t14r REBUILD PARTITION ALL"
  run "$anchor" maint-rebuild-missing \
    "ALTER TABLE golden.t14r REBUILD PARTITION pNOPE"
  run "$anchor" maint-check-p0 \
    "ALTER TABLE golden.t14r CHECK PARTITION p0"
  run "$anchor" maint-check-all \
    "ALTER TABLE golden.t14r CHECK PARTITION ALL"
  run "$anchor" maint-optimize-p0 \
    "ALTER TABLE golden.t14r OPTIMIZE PARTITION p0; SHOW WARNINGS"
  run "$anchor" maint-analyze-p0 \
    "ALTER TABLE golden.t14r ANALYZE PARTITION p0"
  run "$anchor" maint-repair-p0 \
    "ALTER TABLE golden.t14r REPAIR PARTITION p0"
  run "$anchor" maint-discard-p0 \
    "ALTER TABLE golden.t14r DISCARD PARTITION p0 TABLESPACE; SHOW WARNINGS"
  directory "$anchor" t14r
  run "$anchor" maint-import-p0 \
    "ALTER TABLE golden.t14r IMPORT PARTITION p0 TABLESPACE"
  run "$anchor" maint-discard-missing \
    "ALTER TABLE golden.t14r DISCARD PARTITION pNOPE TABLESPACE"
  reset_hash "$anchor"
  run "$anchor" maint-rebuild-hash \
    "ALTER TABLE golden.t14h REBUILD PARTITION p1"
  run "$anchor" maint-check-hash \
    "ALTER TABLE golden.t14h CHECK PARTITION p1"
  reset_plain "$anchor"
  run "$anchor" maint-rebuild-nonpart \
    "ALTER TABLE golden.t14n REBUILD PARTITION p0"
  run "$anchor" maint-check-nonpart \
    "ALTER TABLE golden.t14n CHECK PARTITION p0"

  # ============ F. Multi-action / modifier combinations ============
  reset_range "$anchor"
  run "$anchor" mix-inplace-reorg \
    "ALTER TABLE golden.t14r ALGORITHM=INPLACE, REORGANIZE PARTITION p0 INTO (PARTITION p1 VALUES LESS THAN (120), PARTITION p2 VALUES LESS THAN (200))"
  run "$anchor" mix-copy-reorg \
    "ALTER TABLE golden.t14r ALGORITHM=COPY, REORGANIZE PARTITION p0 INTO (PARTITION p1 VALUES LESS THAN (120), PARTITION p2 VALUES LESS THAN (200))"
  run "$anchor" mix-lock-none-reorg \
    "ALTER TABLE golden.t14r LOCK=NONE, REORGANIZE PARTITION p0 INTO (PARTITION p1 VALUES LESS THAN (120), PARTITION p2 VALUES LESS THAN (200))"
  reset_range "$anchor"
  run "$anchor" mix-reorg-then-option \
    "ALTER TABLE golden.t14r REORGANIZE PARTITION p0 INTO (PARTITION p1 VALUES LESS THAN (120), PARTITION p2 VALUES LESS THAN (200)), ENGINE=InnoDB"
  run "$anchor" mix-reorg-then-drop-col \
    "ALTER TABLE golden.t14r REORGANIZE PARTITION p0 INTO (PARTITION p1 VALUES LESS THAN (120), PARTITION p2 VALUES LESS THAN (200)), DROP COLUMN id"
  run "$anchor" mix-two-partition-ops \
    "ALTER TABLE golden.t14r REORGANIZE PARTITION p0 INTO (PARTITION p1 VALUES LESS THAN (120), PARTITION p2 VALUES LESS THAN (200)), TRUNCATE PARTITION pmax"
  reset_range "$anchor"
  run "$anchor" mix-inplace-remove \
    "ALTER TABLE golden.t14r ALGORITHM=INPLACE, REMOVE PARTITIONING"
  run "$anchor" mix-inplace-add \
    "ALTER TABLE golden.t14r ALGORITHM=INPLACE, ADD PARTITION (PARTITION p3 VALUES LESS THAN (600))"
  reset_hash "$anchor"
  run "$anchor" mix-inplace-coalesce \
    "ALTER TABLE golden.t14h ALGORITHM=INPLACE, COALESCE PARTITION 1"
  reset_range "$anchor"
  run "$anchor" mix-inplace-rebuild \
    "ALTER TABLE golden.t14r ALGORITHM=INPLACE, REBUILD PARTITION p0"
  run "$anchor" mix-inplace-pby \
    "ALTER TABLE golden.t14r ALGORITHM=INPLACE, PARTITION BY HASH(id) PARTITIONS 2"

  # ============ G. FIRST/LAST interval forms + TiDB partition options ============
  reset_range "$anchor"
  run "$anchor" first-partition-drop \
    "ALTER TABLE golden.t14r DROP FIRST PARTITION"
  run "$anchor" last-partition-add \
    "ALTER TABLE golden.t14r ADD LAST PARTITION (PARTITION p3 VALUES LESS THAN (600))"
  run "$anchor" partition-attributes \
    "ALTER TABLE golden.t14r PARTITION p0 ATTRIBUTES = '{\"key\":\"v\"}'"
  run "$anchor" partition-options \
    "ALTER TABLE golden.t14r PARTITION p0 ENGINE = InnoDB"

  # ============ cleanup ============
  run "$anchor" cleanup "DROP TABLE IF EXISTS golden.t14r; DROP TABLE IF EXISTS golden.t14rn; DROP TABLE IF EXISTS golden.t14l; DROP TABLE IF EXISTS golden.t14h; DROP TABLE IF EXISTS golden.t14n"
done

echo "############ DONE ############"
