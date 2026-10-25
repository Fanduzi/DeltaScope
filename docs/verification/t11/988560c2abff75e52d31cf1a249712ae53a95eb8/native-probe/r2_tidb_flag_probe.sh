#!/bin/bash
# T11-R2 §6.2 — complete TiDB tidb_enable_check_constraint OFF/ON cell.
# Every cell records: fixture SQL, actual variable value, executed SQL,
# real client rc, SHOW WARNINGS, before/after KCU, SHOW CREATE.
set -u
LOG=/tmp/r2_tidb_flag_probe.log
: > "$LOG"
T="docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot"
{
echo "## anchor: TiDB $($T -N -B -e 'select version()' 2>&1)"
$T -N -B -e "drop schema if exists t11r2f3; create schema t11r2f3" 2>&1
echo "-- fixture --"
$T -N -B -e "create table t11r2f3.t(id int not null, v int, c int, primary key(id), unique key uk_v(v), key ix_c(c), constraint fk_c foreign key(c) references t11r2f3.t(id))" 2>&1
echo "fixture rc=$?"
$T -e "show create table t11r2f3.t\G" 2>&1 | grep -v "password"
for FLAG in 0 1; do
  echo "===== tidb_enable_check_constraint=$FLAG ====="
  $T -N -B -e "set global tidb_enable_check_constraint=$FLAG" 2>&1
  echo "var=$($T -N -B -e 'select @@global.tidb_enable_check_constraint' 2>&1)"
  echo "KCU-before: $($T -N -B -e "select constraint_name,constraint_schema from information_schema.key_column_usage where table_schema='t11r2f3' and referenced_table_name is not null" 2>&1 | tr '\n' ';')"
  for KW in "drop constraint uk_v" "drop check uk_v" "drop constraint fk_c" "drop check fk_c" "drop constraint \`primary\`" "drop check \`primary\`"; do
    echo "-- \$ ALTER TABLE t11r2f3.t $KW"
    OUT=$($T -e "alter table t11r2f3.t $KW; show warnings" 2>&1); RC=$?
    echo "$OUT" | grep -v "password"
    echo "client_rc=$RC"
  done
  echo "KCU-after: $($T -N -B -e "select constraint_name from information_schema.key_column_usage where table_schema='t11r2f3' and referenced_table_name is not null" 2>&1 | tr '\n' ';')"
  echo "post-objects: pk=$($T -N -B -e "select count(*) from information_schema.table_constraints where table_schema='t11r2f3' and constraint_type='PRIMARY KEY'" 2>&1) uk=$($T -N -B -e "select count(*) from information_schema.statistics where table_schema='t11r2f3' and index_name='uk_v'" 2>&1)"
  $T -e "show create table t11r2f3.t\G" 2>&1 | grep -v "password" | sed -n '2,12p'
done
$T -N -B -e "set global tidb_enable_check_constraint=1" 2>&1
} | tee "$LOG"
