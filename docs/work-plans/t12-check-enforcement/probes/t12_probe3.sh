#!/bin/bash
set -u
LOG="$1"; : > "$LOG"
cell() { echo "=== CELL $1" | tee -a "$LOG"; }
emit() { echo "$1" | tee -a "$LOG"; }
myq() { docker exec "$1" mysql -uroot -proot -N -B -e "$2" 2>&1; }
tiq() { docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot -N -B -e "$1" 2>&1; }
run_my() { local out rc; out="$(myq "$1" "$2")"; rc=$?; emit "$out"; emit "  rc=$rc"; }
run_ti() { local out rc; out="$(tiq "$1")"; rc=$?; emit "$out"; emit "  rc=$rc"; }
nchk() { myq "$1" "select count(*) from information_schema.table_constraints where constraint_schema='t12c' and table_name='t' and constraint_type='CHECK'"; }

for C in deltascope-ddl-golden-mysql80 deltascope-ddl-golden-mysql84; do
  cell "DROP CONSTRAINT vs DROP CHECK on STILL-PRESENT check ($C)"
  emit "version: $(myq "$C" "select @@version")"
  myq "$C" "drop schema if exists t12c; create schema t12c" >>"$LOG" 2>&1
  myq "$C" "create table t12c.t(id int, constraint chk_id check (id > 0), constraint chk2 check (id < 100), constraint chk3 check (id <> 7))" >>"$LOG" 2>&1
  emit "  n_checks=$(nchk "$C")"
  emit "\$ DROP CONSTRAINT chk_id (real check, present)"
  run_my "$C" "alter table t12c.t drop constraint chk_id"
  emit "  n_checks=$(nchk "$C")"
  emit "\$ DROP CHECK chk2 (real check, present)"
  run_my "$C" "alter table t12c.t drop check chk2"
  emit "  n_checks=$(nchk "$C")"
  emit "\$ ALTER TABLE t12c.t ADD CONSTRAINT chk3 CHECK (id < 200)  (duplicate name)"
  run_my "$C" "alter table t12c.t add constraint chk3 check (id < 200)"
  emit "\$ ALTER TABLE t12c.t ADD CHECK (id < 300)  (bare add auto-name)"
  run_my "$C" "alter table t12c.t add check (id < 300)"
  emit "  names=$(myq "$C" "select group_concat(constraint_name) from information_schema.table_constraints where constraint_schema='t12c' and table_name='t' and constraint_type='CHECK'")"
  emit "\$ ALTER CHECK chk3 NOT ENFORCED on table where chk3 is a CHECK — and TC.ENFORCED on TiDB-like catalog"
  emit "  enforced=$(myq "$C" "select enforced from information_schema.table_constraints where constraint_schema='t12c' and table_name='t' and constraint_name='chk3'")"
done

cell "TiDB flag ON: DROP keyword arms on present check + ALTER CONSTRAINT missing"
tiq "set global tidb_enable_check_constraint=ON" >>"$LOG" 2>&1
emit "  var=$(tiq "select @@global.tidb_enable_check_constraint")"
tiq "drop schema if exists t12c; create schema t12c" >>"$LOG" 2>&1
tiq "create table t12c.t(id int, constraint chk_id check (id > 0), constraint chk2 check (id < 100))" >>"$LOG" 2>&1
emit "  SHOW CREATE pre:"; run_ti "show create table t12c.t"
emit "\$ DROP CONSTRAINT chk_id (real check, present, TiDB ON)"
run_ti "alter table t12c.t drop constraint chk_id"
emit "  SHOW CREATE:"; run_ti "show create table t12c.t"
emit "\$ DROP CHECK chk2 (real check, present, TiDB ON)"
run_ti "alter table t12c.t drop check chk2"
emit "  SHOW CREATE:"; run_ti "show create table t12c.t"
emit "\$ ALTER CONSTRAINT missing_x NOT ENFORCED (TiDB ON)"
run_ti "alter table t12c.t alter constraint missing_x not enforced"

cell "TiDB flag OFF: ALTER/DROP on never-stored check"
tiq "set global tidb_enable_check_constraint=OFF" >>"$LOG" 2>&1
tiq "drop schema if exists t12d; create schema t12d; create table t12d.t(id int, constraint chk_id check (id > 0))" >>"$LOG" 2>&1
emit "\$ ALTER CHECK chk_id NOT ENFORCED (never stored)"
run_ti "alter table t12d.t alter check chk_id not enforced"
emit "\$ DROP CHECK chk_id (never stored)"
run_ti "alter table t12d.t drop check chk_id"
emit "\$ DROP CONSTRAINT chk_id (never stored)"
run_ti "alter table t12d.t drop constraint chk_id"
emit "  SHOW CREATE:"; run_ti "show create table t12d.t"
tiq "set global tidb_enable_check_constraint=ON" >>"$LOG" 2>&1
