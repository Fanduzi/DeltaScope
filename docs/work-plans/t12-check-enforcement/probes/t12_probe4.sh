#!/bin/bash
# T12 native probe 4 — RENAME COLUMN interaction with CHECK references.
# Frozen contract marked this cell unprobed -> unvouch. This probe decides
# whether the deterministic silent reference update holds (or falls back).
set -u
LOG="$1"; : > "$LOG"
cell() { echo "=== CELL $1" | tee -a "$LOG"; }
emit() { echo "$1" | tee -a "$LOG"; }
myq() { docker exec "$1" mysql -uroot -proot -N -B -e "$2" 2>&1; }
tiq() { docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot -N -B -e "$1" 2>&1; }
run_my() { local out rc; out="$(myq "$1" "$2")"; rc=$?; emit "$out"; emit "  rc=$rc"; }
run_ti() { local out rc; out="$(tiq "$1")"; rc=$?; emit "$out"; emit "  rc=$rc"; }

for C in deltascope-ddl-golden-mysql80 deltascope-ddl-golden-mysql84; do
  cell "RENAME COLUMN check reference update ($C)"
  emit "version: $(myq "$C" "select @@version")"
  myq "$C" "drop schema if exists t12r; create schema t12r" >>"$LOG" 2>&1
  emit '$ CREATE TABLE t12r.t(id INT, v INT, CONSTRAINT chk_id CHECK (id > 0));'
  run_my "$C" "create table t12r.t(id int, v int, constraint chk_id check (id > 0))"
  emit "  checks: $(myq "$C" "select constraint_name from information_schema.check_constraints where constraint_schema='t12r'")"
  emit '$ ALTER TABLE t12r.t RENAME COLUMN id TO id2;  (native reference update?)'
  run_my "$C" "alter table t12r.t rename column id to id2"
  emit "  SHOW CREATE:"
  run_my "$C" "show create table t12r.t"
  emit "  check still enforces on id2? insert violating id2=-1:"
  run_my "$C" "insert into t12r.t(id2,v) values(-1,0)"
  emit '$ RENAME COLUMN to a name bound by ANOTHER check (collision probe)'
  myq "$C" "alter table t12r.t add constraint chk_v check (v > 0)" >>"$LOG" 2>&1
  emit '$ ALTER TABLE t12r.t RENAME COLUMN v TO id2;  (v->id2 collides w/ chk_id refs)'
  run_my "$C" "alter table t12r.t rename column v to id2"
  emit "  SHOW CREATE:"
  run_my "$C" "show create table t12r.t"
done

cell "TiDB flag ON: RENAME COLUMN check reference update"
emit "var: $(tiq "show variables like 'tidb_enable_check_constraint'")"
tiq "drop database if exists t12r; create database t12r" >>"$LOG" 2>&1
emit '$ CREATE TABLE t12r.t(id INT, v INT, CONSTRAINT chk_id CHECK (id > 0));'
run_ti "create table t12r.t(id int, v int, constraint chk_id check (id > 0))"
emit '$ ALTER TABLE t12r.t RENAME COLUMN id TO id2;'
run_ti "alter table t12r.t rename column id to id2"
emit "  SHOW CREATE:"
run_ti "show create table t12r.t"
emit "  violating insert id2=-1:"
run_ti "insert into t12r.t(id2,v) values(-1,0)"
