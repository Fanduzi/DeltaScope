#!/bin/bash
# T12 native probe — CHECK constraint semantics across the four anchors.
# Real client rc captured from docker exec; fixture + pre/post catalog state
# + SHOW WARNINGS + DML effect proof where enforcement matters.
set -u
LOG="$1"; : > "$LOG"
cell() { echo "=== CELL $1" | tee -a "$LOG"; }
emit() { echo "$1" | tee -a "$LOG"; }
myq() { docker exec "$1" mysql -uroot -proot -N -B -e "$2" 2>&1; }
tiq() { docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot -N -B -e "$1" 2>&1; }
run_my() { local out rc; out="$(myq "$1" "$2")"; rc=$?; emit "$out"; emit "  rc=$rc"; }
run_ti() { local out rc; out="$(tiq "$1")"; rc=$?; emit "$out"; emit "  rc=$rc"; }
chk_count() { myq "$1" "select count(*) from information_schema.check_constraints where constraint_schema='t12'"; }
chk_enforced() { myq "$1" "select enforced from information_schema.check_constraints where constraint_schema='t12' and constraint_name='chk_id'"; }

# ---------- MySQL 8.0.46 + 8.4.10: full lifecycle ----------
for C in deltascope-ddl-golden-mysql80 deltascope-ddl-golden-mysql84; do
  cell "lifecycle ($C)"
  emit "version: $(myq "$C" "select @@version")"
  myq "$C" "drop schema if exists t12; create schema t12" >>"$LOG" 2>&1
  emit "\$ CREATE TABLE t12.t(id INT, CONSTRAINT chk_id CHECK (id > 0));"
  run_my "$C" "create table t12.t(id int, constraint chk_id check (id > 0))"
  emit "  catalog: n=$(chk_count "$C") enforced=$(chk_enforced "$C")"
  emit "\$ INSERT id=-1 (violating, proves enforcement)"
  run_my "$C" "insert into t12.t values(-1)"
  emit "\$ ALTER TABLE t12.t ALTER CHECK chk_id NOT ENFORCED;"
  run_my "$C" "alter table t12.t alter check chk_id not enforced"
  emit "  catalog: enforced=$(chk_enforced "$C")"
  emit "\$ INSERT id=-1 again (should land under NOT ENFORCED)"
  run_my "$C" "insert into t12.t values(-1)"
  emit "  rows=$(myq "$C" "select count(*) from t12.t")"
  emit "\$ ALTER TABLE t12.t ALTER CHECK chk_id ENFORCED;  (violating row present!)"
  run_my "$C" "alter table t12.t alter check chk_id enforced"
  emit "  catalog: enforced=$(chk_enforced "$C") rows=$(myq "$C" "select count(*) from t12.t")"
  emit "\$ ALTER TABLE t12.t ALTER CONSTRAINT chk_id NOT ENFORCED;  (PG-style alias)"
  run_my "$C" "alter table t12.t alter constraint chk_id not enforced"
  emit "\$ ALTER TABLE t12.t ALTER CHECK missing_x NOT ENFORCED;"
  run_my "$C" "alter table t12.t alter check missing_x not enforced"
  emit "\$ ALTER TABLE t12.t ALTER CHECK uk_v NOT ENFORCED;  (non-check name)"
  myq "$C" "create table t12.u(v int, unique key uk_v(v))" >>"$LOG" 2>&1
  run_my "$C" "alter table t12.u alter check uk_v not enforced"
  emit "\$ ALTER TABLE t12.t DROP CHECK chk_id; post catalog"
  run_my "$C" "alter table t12.t drop check chk_id"
  emit "  catalog: n=$(chk_count "$C")"
  emit "\$ ALTER TABLE t12.t ADD CONSTRAINT chk2 CHECK (id < 100) NOT ENFORCED;"
  run_my "$C" "alter table t12.t add constraint chk2 check (id < 100) not enforced"
  emit "  catalog n2=$(myq "$C" "select enforced from information_schema.check_constraints where constraint_schema='t12' and constraint_name='chk2'")"
done

# ---------- MySQL 5.7.44: parsed-but-ignored ----------
C=deltascope-ddl-golden-mysql57
cell "5.7 parsed-but-ignored ($C)"
emit "version: $(myq "$C" "select @@version")"
myq "$C" "drop schema if exists t12; create schema t12" >>"$LOG" 2>&1
emit "\$ CREATE TABLE t12.t(id INT, CONSTRAINT chk_id CHECK (id > 0));"
run_my "$C" "create table t12.t(id int, constraint chk_id check (id > 0))"
emit "  SHOW CREATE keeps it?"
run_my "$C" "show create table t12.t"
emit "\$ INSERT id=-1 (should LAND — check ignored on 5.7)"
run_my "$C" "insert into t12.t values(-1)"
emit "  rows=$(myq "$C" "select count(*) from t12.t")"
emit "\$ ALTER TABLE t12.t ALTER CHECK chk_id NOT ENFORCED;"
run_my "$C" "alter table t12.t alter check chk_id not enforced"
emit "\$ ALTER TABLE t12.t DROP CHECK chk_id;"
run_my "$C" "alter table t12.t drop check chk_id"
emit "\$ ALTER TABLE t12.t ADD CONSTRAINT chk2 CHECK (id < 100);"
run_my "$C" "alter table t12.t add constraint chk2 check (id < 100)"
emit "  SHOW CREATE post-add:"
run_my "$C" "show create table t12.t"

# ---------- TiDB v8.5.0: tidb_enable_check_constraint OFF/ON ----------
cell "TiDB v8.5.0 flag arms"
emit "version: $(tiq "select tidb_version()" | head -1)"
for FLAG in OFF ON; do
  emit "--- tidb_enable_check_constraint=$FLAG"
  tiq "set global tidb_enable_check_constraint=$FLAG" >>"$LOG" 2>&1
  emit "  var=$(tiq "select @@global.tidb_enable_check_constraint")"
  tiq "drop schema if exists t12; create schema t12" >>"$LOG" 2>&1
  emit "\$ CREATE TABLE t12.t(id INT, CONSTRAINT chk_id CHECK (id > 0));"
  run_ti "create table t12.t(id int, constraint chk_id check (id > 0))"
  emit "  SHOW WARNINGS:"; emit "$(tiq "show warnings")"
  emit "\$ INSERT id=-1"
  run_ti "insert into t12.t values(-1)"
  emit "  rows=$(tiq "select count(*) from t12.t")"
  emit "\$ ALTER TABLE t12.t ALTER CHECK chk_id NOT ENFORCED;"
  run_ti "alter table t12.t alter check chk_id not enforced"
  emit "  SHOW WARNINGS:"; emit "$(tiq "show warnings")"
  emit "\$ ALTER TABLE t12.t ALTER CHECK chk_id ENFORCED;"
  run_ti "alter table t12.t alter check chk_id enforced"
  emit "\$ ALTER TABLE t12.t ADD CONSTRAINT chk2 CHECK (id < 100);"
  run_ti "alter table t12.t add constraint chk2 check (id < 100)"
  emit "  SHOW WARNINGS:"; emit "$(tiq "show warnings")"
  emit "\$ ALTER TABLE t12.t DROP CHECK chk_id;"
  run_ti "alter table t12.t drop check chk_id"
  emit "  SHOW WARNINGS:"; emit "$(tiq "show warnings")"
  emit "  SHOW CREATE post:"
  run_ti "show create table t12.t"
done
