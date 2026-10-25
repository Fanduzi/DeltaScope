#!/bin/bash
# T12 probe 6: TiDB v8.5.0 CHECK catalog shape — where members and the
# enforcement bit actually live. Run against the shared ddl-golden anchor
# with deltascope-ddl-golden-tidb85 + -client containers healthy.
set -u
LOG="$1"; : > "$LOG"
emit() { echo "$1" | tee -a "$LOG"; }
tiq() { docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot -N -B -e "$1" 2>&1; }

emit "=== TiDB v8.5.0 CHECK catalog shape"
emit "version: $(tiq "select @@version")"
tiq "set global tidb_enable_check_constraint=ON" >>"$LOG" 2>&1
tiq "create schema if not exists t12q; drop table if exists t12q.t; create table t12q.t(id int, constraint chk_id check (id > 0))" >>"$LOG" 2>&1
emit "  flag=$(tiq "select @@global.tidb_enable_check_constraint")"
emit "  table_constraints CHECK rows: $(tiq "select count(*) from information_schema.table_constraints where constraint_schema='t12q' and table_name='t' and constraint_type='CHECK'")"
emit "  check_constraints rows:       $(tiq "select count(*) from information_schema.check_constraints where constraint_schema='t12q' and constraint_name='chk_id'")"
emit "  tidb_check_constraints row:   $(tiq "select constraint_name, table_name, check_clause from information_schema.tidb_check_constraints where constraint_schema='t12q' and constraint_name='chk_id'")"
emit "  check_constraints columns:"; tiq "show columns from information_schema.check_constraints" | tee -a "$LOG"
emit "  tidb_check_constraints columns:"; tiq "show columns from information_schema.tidb_check_constraints" | tee -a "$LOG"
emit "\$ ALTER CHECK chk_id NOT ENFORCED"
tiq "alter table t12q.t alter check chk_id not enforced" >>"$LOG" 2>&1; emit "  rc=$?"
emit "  SHOW CREATE after flip:"; run=$(tiq "show create table t12q.t"); emit "  $run"
emit "  tidb_check_constraints post-flip: $(tiq "select check_clause from information_schema.tidb_check_constraints where constraint_schema='t12q' and constraint_name='chk_id'")"
tiq "drop schema t12q" >>"$LOG" 2>&1
