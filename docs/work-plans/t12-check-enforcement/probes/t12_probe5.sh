#!/bin/bash
# T12 probe 5 — CHANGE COLUMN (renaming form) on a check-referenced column.
set -u
LOG="$1"; : > "$LOG"
emit() { echo "$1" | tee -a "$LOG"; }
myq() { docker exec "$1" mysql -uroot -proot -N -B -e "$2" 2>&1; }
tiq() { docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot -N -B -e "$1" 2>&1; }
run_my() { local out rc; out="$(myq "$1" "$2")"; rc=$?; emit "$out"; emit "  rc=$rc"; }
run_ti() { local out rc; out="$(tiq "$1")"; rc=$?; emit "$out"; emit "  rc=$rc"; }

for C in deltascope-ddl-golden-mysql80 deltascope-ddl-golden-mysql84; do
  emit "=== CELL CHANGE COLUMN check reference ($C)"
  emit "version: $(myq "$C" "select @@version")"
  myq "$C" "drop schema if exists t12c2; create schema t12c2" >>"$LOG" 2>&1
  run_my "$C" "create table t12c2.t(id int, v int, constraint chk_id check (id > 0))"
  emit '$ CHANGE COLUMN id TO id2 BIGINT (renaming, referenced)'
  run_my "$C" "alter table t12c2.t change column id id2 bigint"
  emit '$ CHANGE COLUMN id TO id BIGINT (same name retype, referenced)'
  run_my "$C" "alter table t12c2.t change column id id bigint"
  emit "  SHOW CREATE:"
  run_my "$C" "show create table t12c2.t"
done
emit "=== CELL CHANGE COLUMN check reference (tidb85 flag ON)"
emit "var: $(tiq "show variables like 'tidb_enable_check_constraint'")"
tiq "drop database if exists t12c2; create database t12c2" >>"$LOG" 2>&1
run_ti "create table t12c2.t(id int, v int, constraint chk_id check (id > 0))"
emit '$ CHANGE COLUMN id TO id2 BIGINT (renaming, referenced)'
run_ti "alter table t12c2.t change column id id2 bigint"
emit '$ CHANGE COLUMN id TO id BIGINT (same name retype, referenced)'
run_ti "alter table t12c2.t change column id id bigint"
emit "  SHOW CREATE:"
run_ti "show create table t12c2.t"
