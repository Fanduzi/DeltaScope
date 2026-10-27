#!/usr/bin/env bash
# p12 — T13-R3 bounded native confirmation for EXCHANGE effective-type
# equivalence pairs. Each arm rebuilds both fixtures independently and
# records only the audited DDL outcome (rc + stderr) plus a catalog type
# read per side — no post-state row claims.
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

arm() {
  local anchor="$1" label="$2" ptype="$3" xtype="$4" attrs="${5:-}"
  run "$anchor" "$label-drop" \
    "DROP TABLE IF EXISTS golden.t13d; DROP TABLE IF EXISTS golden.t13x"
  run "$anchor" "$label-create-t" \
    "CREATE TABLE golden.t13d (id INT, v $ptype) ENGINE=InnoDB PARTITION BY RANGE(id) (PARTITION p0 VALUES LESS THAN (10), PARTITION p1 VALUES LESS THAN (20))"
  run "$anchor" "$label-create-x" \
    "CREATE TABLE golden.t13x (id INT, v $xtype) ENGINE=InnoDB"
  run "$anchor" "$label-exchange" \
    "ALTER TABLE golden.t13d EXCHANGE PARTITION p0 WITH TABLE golden.t13x"
  run "$anchor" "$label-catalog" \
    "SELECT TABLE_NAME, COLUMN_TYPE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='golden' AND COLUMN_NAME='v' AND TABLE_NAME IN ('t13d','t13x') ORDER BY TABLE_NAME"
  run "$anchor" "$label-cleanup" \
    "DROP TABLE IF EXISTS golden.t13d; DROP TABLE IF EXISTS golden.t13x"
}

for anchor in mysql80 mysql57; do
  echo "############ ANCHOR $anchor ############"
  run "$anchor" version "SELECT VERSION()"
  run "$anchor" ensure-db "CREATE DATABASE IF NOT EXISTS golden"

  # Equivalence pairs — must execute.
  arm "$anchor" decM-vs-decM0 "DECIMAL(12)" "DECIMAL(12,0)"
  arm "$anchor" decBare-vs-dec100 "DECIMAL" "DECIMAL(10,0)"
  arm "$anchor" decM-uns-vs-decM0-uns "DECIMAL(12) UNSIGNED" "DECIMAL(12,0) UNSIGNED"
  arm "$anchor" int10-vs-int11 "INT(10)" "INT(11)"

  # Real-difference controls — must refuse 1736.
  arm "$anchor" dec121-vs-dec120 "DECIMAL(12,1)" "DECIMAL(12,0)"
  arm "$anchor" dec11-vs-dec12 "DECIMAL(11)" "DECIMAL(12)"
  arm "$anchor" decM-vs-decM-uns "DECIMAL(12)" "DECIMAL(12) UNSIGNED"
done

echo "############ DONE ############"
