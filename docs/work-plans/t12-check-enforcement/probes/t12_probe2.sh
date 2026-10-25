#!/bin/bash
set -u
LOG="$1"; : > "$LOG"
cell() { echo "=== CELL $1" | tee -a "$LOG"; }
emit() { echo "$1" | tee -a "$LOG"; }
myq() { docker exec "$1" mysql -uroot -proot -N -B -e "$2" 2>&1; }
tiq() { docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot -N -B -e "$1" 2>&1; }
run_my() { local out rc; out="$(myq "$1" "$2")"; rc=$?; emit "$out"; emit "  rc=$rc"; }
run_ti() { local out rc; out="$(tiq "$1")"; rc=$?; emit "$out"; emit "  rc=$rc"; }
tc_enforced() { myq "$1" "select enforced from information_schema.table_constraints where constraint_schema='t12b' and table_name='t' and constraint_name='$2'"; }

for C in deltascope-ddl-golden-mysql80 deltascope-ddl-golden-mysql84; do
  cell "enforced column + dependent column ($C)"
  emit "version: $(myq "$C" "select @@version")"
  myq "$C" "drop schema if exists t12b; create schema t12b" >>"$LOG" 2>&1
  run_my "$C" "create table t12b.t(id int, v int, constraint chk_id check (id > 0), check (v > 0))"
  emit "  TC.ENFORCED chk_id=$(tc_enforced "$C" chk_id)"
  emit "  SHOW CREATE (auto-name of bare CHECK):"
  run_my "$C" "show create table t12b.t"
  emit "\$ ALTER CHECK chk_id NOT ENFORCED -> TC.ENFORCED + SHOW CREATE marker"
  run_my "$C" "alter table t12b.t alter check chk_id not enforced"
  emit "  TC.ENFORCED chk_id=$(tc_enforced "$C" chk_id)"
  emit "\$ ALTER CHECK chk_id NOT ENFORCED again (no-op arm)"
  run_my "$C" "alter table t12b.t alter check chk_id not enforced"
  emit "\$ ALTER CHECK chk_id ENFORCED (clean data — expect rc=0)"
  run_my "$C" "alter table t12b.t alter check chk_id enforced"
  emit "  TC.ENFORCED chk_id=$(tc_enforced "$C" chk_id)"
  emit "\$ DROP COLUMN id (referenced by chk_id — dependency refusal?)"
  run_my "$C" "alter table t12b.t drop column id"
  emit "\$ DROP COLUMN v (referenced by bare check)"
  run_my "$C" "alter table t12b.t drop column v"
  emit "  columns left: $(myq "$C" "select group_concat(column_name) from information_schema.columns where table_schema='t12b' and table_name='t'")"
  emit "  checks left: $(myq "$C" "select group_concat(constraint_name) from information_schema.table_constraints where constraint_schema='t12b' and table_name='t' and constraint_type='CHECK'")"
  emit "\$ ALTER TABLE t12b.t DROP CONSTRAINT chk_id;  (CONSTRAINT keyword on real check)"
  run_my "$C" "alter table t12b.t drop constraint chk_id"
  emit "  checks left: $(myq "$C" "select group_concat(constraint_name) from information_schema.table_constraints where constraint_schema='t12b' and table_name='t' and constraint_type='CHECK'")"
done

cell "TiDB flag ON bindings"
tiq "set global tidb_enable_check_constraint=ON" >>"$LOG" 2>&1
tiq "drop schema if exists t12b; create schema t12b" >>"$LOG" 2>&1
tiq "create table t12b.t(id int, v int, constraint chk_id check (id > 0), check (v > 0)); create table t12b.u(w int, unique key uk_w(w))" >>"$LOG" 2>&1
emit "  SHOW CREATE (auto-name + stored?)"
run_ti "show create table t12b.t"
emit "\$ ALTER CHECK chk_id NOT ENFORCED"
run_ti "alter table t12b.t alter check chk_id not enforced"
emit "\$ ALTER CHECK missing_x NOT ENFORCED"
run_ti "alter table t12b.t alter check missing_x not enforced"
emit "\$ ALTER CHECK uk_w NOT ENFORCED (non-check name)"
run_ti "alter table t12b.u alter check uk_w not enforced"
emit "\$ ALTER CONSTRAINT chk_id NOT ENFORCED (alias on TiDB)"
run_ti "alter table t12b.t alter constraint chk_id not enforced"
emit "\$ DROP COLUMN id (referenced)"
run_ti "alter table t12b.t drop column id"
emit "\$ DROP CONSTRAINT chk_id (CONSTRAINT keyword on real check)"
run_ti "alter table t12b.t drop constraint chk_id"
emit "  SHOW CREATE post:"
run_ti "show create table t12b.t"
emit "\$ ALTER CHECK ENFORCED on non-enforced chk2 (clean data)"
tiq "alter table t12b.t add constraint chk2 check (v < 100) not enforced" >>"$LOG" 2>&1
run_ti "alter table t12b.t alter check chk2 enforced"
