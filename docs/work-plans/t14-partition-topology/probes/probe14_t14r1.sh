#!/usr/bin/env bash
# p15 — T14-R1 supplementary native matrix: same-statement duplicate member
# names, authored-count edges (0 / 8193), CREATE-scheme refusals, RANGE
# COLUMNS per-item MAXVALUE, collation-domain string bounds, LIST
# expression items, and MyISAM IMPORT/DISCARD. Bounded cells only — no
# giant-count fixtures, no unbounded resource arms.
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

dir() {
  local anchor="$1" table="$2"
  run "$anchor" "dir-$table" \
    "SELECT PARTITION_NAME, PARTITION_ORDINAL_POSITION, PARTITION_METHOD, PARTITION_DESCRIPTION FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='$table' AND PARTITION_NAME IS NOT NULL ORDER BY PARTITION_ORDINAL_POSITION"
}

reset_range2() {
  run "$1" reset-r2 \
    "DROP TABLE IF EXISTS golden.r2; CREATE TABLE golden.r2 (id INT) ENGINE=InnoDB PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (200), PARTITION p1 VALUES LESS THAN (400))"
}
reset_rangemax() {
  run "$1" reset-rm \
    "DROP TABLE IF EXISTS golden.rm; CREATE TABLE golden.rm (id INT) ENGINE=InnoDB PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (200), PARTITION pmax VALUES LESS THAN MAXVALUE)"
}
reset_hash4() {
  run "$1" reset-h4 \
    "DROP TABLE IF EXISTS golden.h4; CREATE TABLE golden.h4 (id INT) ENGINE=InnoDB PARTITION BY HASH(id) PARTITIONS 4"
}
reset_list2() {
  run "$1" reset-l2 \
    "DROP TABLE IF EXISTS golden.l2; CREATE TABLE golden.l2 (id INT) ENGINE=InnoDB PARTITION BY LIST(id) (PARTITION p0 VALUES IN (1), PARTITION p1 VALUES IN (2))"
}
reset_rcol() {
  run "$1" reset-rc \
    "DROP TABLE IF EXISTS golden.rc; CREATE TABLE golden.rc (a INT, b INT) ENGINE=InnoDB PARTITION BY RANGE COLUMNS(a,b) (PARTITION p0 VALUES LESS THAN (10,MAXVALUE))"
}
reset_lcol() {
  run "$1" reset-lc \
    "DROP TABLE IF EXISTS golden.lc; CREATE TABLE golden.lc (a VARCHAR(10)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci PARTITION BY LIST COLUMNS(a) (PARTITION p0 VALUES IN ('a'))"
}
reset_scol() {
  run "$1" reset-sc \
    "DROP TABLE IF EXISTS golden.sc; CREATE TABLE golden.sc (a VARCHAR(10)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci PARTITION BY RANGE COLUMNS(a) (PARTITION p0 VALUES LESS THAN ('B'))"
}
reset_myisam() {
  run "$1" reset-my \
    "DROP TABLE IF EXISTS golden.my; CREATE TABLE golden.my (id INT) ENGINE=MyISAM PARTITION BY HASH(id) PARTITIONS 2"
}
reset_innodb() {
  run "$1" reset-in \
    "DROP TABLE IF EXISTS golden.inn; CREATE TABLE golden.inn (id INT) ENGINE=InnoDB PARTITION BY HASH(id) PARTITIONS 2"
}

for anchor in mysql57 mysql80 mysql84 tidb85; do
  echo "########## $anchor ##########"
  run "$anchor" "schema" "CREATE DATABASE IF NOT EXISTS golden"

  # --- A2: same-statement duplicate member names (atomic refusal check) ---
  reset_range2 "$anchor"
  run "$anchor" "add-dup-newname" \
    "ALTER TABLE golden.r2 ADD PARTITION (PARTITION p2 VALUES LESS THAN (600), PARTITION p2 VALUES LESS THAN (800))"
  dir "$anchor" r2
  reset_rangemax "$anchor"
  run "$anchor" "reorg-dup-newname" \
    "ALTER TABLE golden.rm REORGANIZE PARTITION p0 INTO (PARTITION p1 VALUES LESS THAN (120), PARTITION p1 VALUES LESS THAN (200))"
  dir "$anchor" rm
  run "$anchor" "pby-dup-newname" \
    "ALTER TABLE golden.r2 PARTITION BY RANGE(id) (PARTITION q0 VALUES LESS THAN (200), PARTITION q0 VALUES LESS THAN MAXVALUE)"
  dir "$anchor" r2
  run "$anchor" "create-dup-name" \
    "DROP TABLE IF EXISTS golden.dn; CREATE TABLE golden.dn (id INT) ENGINE=InnoDB PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (200), PARTITION p0 VALUES LESS THAN (400))"
  run "$anchor" "create-dup-name-exists" \
    "SELECT COUNT(*) AS present FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='dn'"

  # --- A1: authored counts at the edges ---
  reset_hash4 "$anchor"
  run "$anchor" "add-partitions-0" "ALTER TABLE golden.h4 ADD PARTITION PARTITIONS 0"
  dir "$anchor" h4
  run "$anchor" "add-partitions-8193" "ALTER TABLE golden.h4 ADD PARTITION PARTITIONS 8193"
  dir "$anchor" h4
  run "$anchor" "coalesce-0" "ALTER TABLE golden.h4 COALESCE PARTITION 0"
  dir "$anchor" h4
  run "$anchor" "pby-hash-0" "ALTER TABLE golden.h4 PARTITION BY HASH(id) PARTITIONS 0"
  dir "$anchor" h4
  run "$anchor" "pby-hash-8193" "ALTER TABLE golden.h4 PARTITION BY HASH(id) PARTITIONS 8193"
  dir "$anchor" h4
  reset_range2 "$anchor"
  run "$anchor" "pby-range-count4" "ALTER TABLE golden.r2 PARTITION BY RANGE(id) PARTITIONS 4"
  dir "$anchor" r2
  run "$anchor" "create-hash-0" \
    "DROP TABLE IF EXISTS golden.c0; CREATE TABLE golden.c0 (id INT) ENGINE=InnoDB PARTITION BY HASH(id) PARTITIONS 0"
  run "$anchor" "create-hash-8193" \
    "DROP TABLE IF EXISTS golden.c8; CREATE TABLE golden.c8 (id INT) ENGINE=InnoDB PARTITION BY HASH(id) PARTITIONS 8193"
  run "$anchor" "create-range-count4" \
    "DROP TABLE IF EXISTS golden.c4; CREATE TABLE golden.c4 (id INT) ENGINE=InnoDB PARTITION BY RANGE(id) PARTITIONS 4"
  dir "$anchor" c4

  # --- A3: CREATE-scheme validity ---
  run "$anchor" "create-range-decreasing" \
    "DROP TABLE IF EXISTS golden.cd; CREATE TABLE golden.cd (id INT) ENGINE=InnoDB PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (200), PARTITION p1 VALUES LESS THAN (100))"
  run "$anchor" "create-range-decreasing-exists" \
    "SELECT COUNT(*) AS present FROM information_schema.TABLES WHERE TABLE_SCHEMA='golden' AND TABLE_NAME='cd'"
  run "$anchor" "create-list-dupvalue" \
    "DROP TABLE IF EXISTS golden.cl; CREATE TABLE golden.cl (id INT) ENGINE=InnoDB PARTITION BY LIST(id) (PARTITION p0 VALUES IN (1), PARTITION p1 VALUES IN (1))"
  run "$anchor" "create-maxvalue-mid" \
    "DROP TABLE IF EXISTS golden.cm; CREATE TABLE golden.cm (id INT) ENGINE=InnoDB PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN MAXVALUE, PARTITION p1 VALUES LESS THAN (400))"

  # --- B1: RANGE COLUMNS per-item MAXVALUE ---
  reset_rcol "$anchor"
  dir "$anchor" rc
  run "$anchor" "rcol-add-maxvalue2" \
    "ALTER TABLE golden.rc ADD PARTITION (PARTITION p1 VALUES LESS THAN (20,MAXVALUE))"
  dir "$anchor" rc
  run "$anchor" "rcol-add-scalar-tail" \
    "ALTER TABLE golden.rc ADD PARTITION (PARTITION p2 VALUES LESS THAN (MAXVALUE,MAXVALUE))"
  dir "$anchor" rc
  run "$anchor" "rcol-pby" \
    "ALTER TABLE golden.rc PARTITION BY RANGE COLUMNS(a,b) (PARTITION q0 VALUES LESS THAN (15,MAXVALUE), PARTITION q1 VALUES LESS THAN (30,MAXVALUE))"
  dir "$anchor" rc

  # --- B2: collation-domain string bounds (utf8mb4_general_ci) ---
  reset_scol "$anchor"
  run "$anchor" "scol-add-ci-equal" \
    "ALTER TABLE golden.sc ADD PARTITION (PARTITION p1 VALUES LESS THAN ('a'))"
  dir "$anchor" sc
  run "$anchor" "scol-reorg-ci-equal" \
    "ALTER TABLE golden.sc REORGANIZE PARTITION p0 INTO (PARTITION p1 VALUES LESS THAN ('a'))"
  dir "$anchor" sc
  reset_scol "$anchor"
  run "$anchor" "scol-add-higher" \
    "ALTER TABLE golden.sc ADD PARTITION (PARTITION p1 VALUES LESS THAN ('c'))"
  dir "$anchor" sc
  reset_lcol "$anchor"
  run "$anchor" "lcol-add-ci-dup" \
    "ALTER TABLE golden.lc ADD PARTITION (PARTITION p1 VALUES IN ('B'))"
  dir "$anchor" lc

  # --- B3: LIST expression items ---
  reset_list2 "$anchor"
  run "$anchor" "list-add-expr" \
    "ALTER TABLE golden.l2 ADD PARTITION (PARTITION p2 VALUES IN (1+0))"
  dir "$anchor" l2
  run "$anchor" "create-range-expr-bound" \
    "DROP TABLE IF EXISTS golden.ce; CREATE TABLE golden.ce (id INT) ENGINE=InnoDB PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (100+100), PARTITION p1 VALUES LESS THAN (150))"
  dir "$anchor" ce
  run "$anchor" "pby-expr-bound" \
    "ALTER TABLE golden.r2 PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (100+100), PARTITION p1 VALUES LESS THAN (150))"
  dir "$anchor" r2

  # --- B4: ADD non-increasing bound native code ---
  reset_range2 "$anchor"
  run "$anchor" "add-low-bound" \
    "ALTER TABLE golden.r2 ADD PARTITION (PARTITION p2 VALUES LESS THAN (300))"
  dir "$anchor" r2

  # --- D3: engine applicability on IMPORT/DISCARD ---
  reset_myisam "$anchor"
  run "$anchor" "myisam-discard" "ALTER TABLE golden.my DISCARD PARTITION p0 TABLESPACE"
  dir "$anchor" my
  run "$anchor" "myisam-import" "ALTER TABLE golden.my IMPORT PARTITION p0 TABLESPACE"
  reset_innodb "$anchor"
  run "$anchor" "innodb-discard" "ALTER TABLE golden.inn DISCARD PARTITION p0 TABLESPACE"
  dir "$anchor" inn
  run "$anchor" "innodb-import" "ALTER TABLE golden.inn IMPORT PARTITION p0 TABLESPACE"

  # --- D4: OPTIMIZE scope on InnoDB (named member vs whole table) ---
  reset_rangemax "$anchor"
  run "$anchor" "optimize-named" "ALTER TABLE golden.rm OPTIMIZE PARTITION p0"
  run "$anchor" "optimize-warnings" "SHOW WARNINGS"
done
echo "DONE"
