#!/bin/bash
# T11-R2 targeted native probe — real client rc captured from docker exec,
# never from a log filter. Each cell prints fixture, SQL, rc, and post-state.
set -u
LOG="$1"
: > "$LOG"

run() { # container sql -> prints rc
  local c="$1" sql="$2"
  docker exec "$c" mysql -uroot -proot -N -B -e "$sql" 2>&1
  echo "  rc=$?"
}
q() { # container sql
  docker exec "$1" mysql -uroot -proot -N -B -e "$2" 2>&1
}
cell() { echo "=== CELL $1" | tee -a "$LOG"; }
emit() { echo "$1" | tee -a "$LOG"; }

for C in deltascope-ddl-golden-mysql84 deltascope-ddl-golden-mysql80; do
  emit "## anchor $C"
  emit "version/restrict: $(q "$C" "select @@version; select @@restrict_fk_on_non_standard_key" 2>&1 | tr '\n' ' ')"
done

# ---------- D cells: partial unique/PK key under restrict ON/OFF on 8.4 ----------
C=deltascope-ddl-golden-mysql84
for FLAG in OFF ON; do
  cell "D-$FLAG restrict_fk_on_non_standard_key=$FLAG (8.4)"
  emit "\$ SET GLOBAL restrict_fk_on_non_standard_key=$FLAG"
  q "$C" "set global restrict_fk_on_non_standard_key=$FLAG" >>"$LOG" 2>&1
  q "$C" "select @@global.restrict_fk_on_non_standard_key" | sed 's/^/  global=/ ' | tee -a "$LOG"
  q "$C" "drop schema if exists t11r2d; create schema t11r2d" >>"$LOG" 2>&1
  q "$C" "create table t11r2d.puq(a int not null,b int not null, unique key uq_ab(a,b));
         create table t11r2d.ppk(a int not null,b int not null, primary key(a,b));
         create table t11r2d.psc(a int not null,b int not null, key ix_ab(a,b));
         create table t11r2d.c1(c int);
         create table t11r2d.c2(c int);
         create table t11r2d.c3(c int);
         create table t11r2d.c4(a int,b int);
         create table t11r2d.c5(c int)" >>"$LOG" 2>&1
  cell "D-$FLAG partial UNIQUE: FK(c) REFERENCES puq(a) where unique is (a,b)"
  emit "\$ ALTER TABLE t11r2d.c1 ADD CONSTRAINT fk1 FOREIGN KEY(c) REFERENCES t11r2d.puq(a);"
  emit "$(run "$C" "alter table t11r2d.c1 add constraint fk1 foreign key(c) references t11r2d.puq(a)")"
  emit "  post KCU=$(q "$C" "select count(*) from information_schema.key_column_usage where table_schema='t11r2d' and referenced_table_name is not null")"
  cell "D-$FLAG partial PK: FK(c) REFERENCES ppk(a) where pk is (a,b)"
  emit "\$ ALTER TABLE t11r2d.c2 ADD CONSTRAINT fk2 FOREIGN KEY(c) REFERENCES t11r2d.ppk(a);"
  emit "$(run "$C" "alter table t11r2d.c2 add constraint fk2 foreign key(c) references t11r2d.ppk(a)")"
  emit "  post KCU=$(q "$C" "select count(*) from information_schema.key_column_usage where table_schema='t11r2d' and referenced_table_name is not null")"
  cell "D-$FLAG partial secondary: FK(c) REFERENCES psc(a) where index is (a,b)"
  emit "\$ ALTER TABLE t11r2d.c3 ADD CONSTRAINT fk3 FOREIGN KEY(c) REFERENCES t11r2d.psc(a);"
  emit "$(run "$C" "alter table t11r2d.c3 add constraint fk3 foreign key(c) references t11r2d.psc(a)")"
  emit "  post KCU=$(q "$C" "select count(*) from information_schema.key_column_usage where table_schema='t11r2d' and referenced_table_name is not null")"
  cell "D-$FLAG full UNIQUE: FK(a,b) REFERENCES puq(a,b)"
  emit "\$ ALTER TABLE t11r2d.c4 ADD CONSTRAINT fk4 FOREIGN KEY(a,b) REFERENCES t11r2d.puq(a,b);"
  emit "$(run "$C" "alter table t11r2d.c4 add constraint fk4 foreign key(a,b) references t11r2d.puq(a,b)")"
  emit "  post KCU=$(q "$C" "select count(*) from information_schema.key_column_usage where table_schema='t11r2d' and referenced_table_name is not null")"
  cell "D-$FLAG wrong-order UNIQUE: FK(c) REFERENCES puq(b)"
  emit "\$ ALTER TABLE t11r2d.c5 ADD CONSTRAINT fk5 FOREIGN KEY(c) REFERENCES t11r2d.puq(b);"
  emit "$(run "$C" "alter table t11r2d.c5 add constraint fk5 foreign key(c) references t11r2d.puq(b)")"
done
q "$C" "set global restrict_fk_on_non_standard_key=ON" >/dev/null 2>&1

# ---------- E cells ----------
for C in deltascope-ddl-golden-mysql80 deltascope-ddl-golden-mysql84 deltascope-ddl-golden-tidb85-client; do
  cell "E1 self-reference sole referenced coverer drop ($C)"
  if [ "$C" = deltascope-ddl-golden-tidb85-client ]; then
    HOST=tidb85; Q="docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot -N -B -e"
  else
    Q="docker exec $C mysql -uroot -proot -N -B -e"
  fi
  $Q "drop schema if exists t11r2e; create schema t11r2e" >>"$LOG" 2>&1
  emit "\$ CREATE TABLE t11r2e.t(id int not null, parent_id int null, primary key(id), key ix_parent(parent_id), constraint fk_parent foreign key(parent_id) references t11r2e.t(id));"
  emit "$( $Q "create table t11r2e.t(id int not null, parent_id int null, primary key(id), key ix_parent(parent_id), constraint fk_parent foreign key(parent_id) references t11r2e.t(id))" 2>&1; echo "  rc=$?")"
  emit "\$ ALTER TABLE t11r2e.t DROP PRIMARY KEY;"
  emit "$( $Q "alter table t11r2e.t drop primary key" 2>&1; echo "  rc=$?")"
  emit "  post: pk_exists=$( $Q "select count(*) from information_schema.table_constraints where table_schema='t11r2e' and constraint_type='PRIMARY KEY'" 2>&1)"
  emit "\$ ALTER TABLE t11r2e.t DROP INDEX ix_parent;"
  emit "$( $Q "alter table t11r2e.t drop index ix_parent" 2>&1; echo "  rc=$?")"
done

# E2: existing non-standard FK index drop under restrict ON (8.4)
C=deltascope-ddl-golden-mysql84
cell "E2 existing non-unique parent index drop under restrict ON (8.4)"
q "$C" "set global restrict_fk_on_non_standard_key=OFF" >>"$LOG" 2>&1
q "$C" "drop schema if exists t11r2e2; create schema t11r2e2;
        create table t11r2e2.p(id int not null, key ix_id(id));
        create table t11r2e2.t(c int, key ix_c(c), constraint fk_c foreign key(c) references t11r2e2.p(id))" >>"$LOG" 2>&1
emit "built under OFF; switching ON"
q "$C" "set global restrict_fk_on_non_standard_key=ON" >>"$LOG" 2>&1
q "$C" "select @@global.restrict_fk_on_non_standard_key" | sed 's/^/  global=/ ' | tee -a "$LOG"
emit "\$ ALTER TABLE t11r2e2.p DROP INDEX ix_id;"
emit "$(run "$C" "alter table t11r2e2.p drop index ix_id")"
emit "  post: ix_id_exists=$(q "$C" "select count(*) from information_schema.statistics where table_schema='t11r2e2' and index_name='ix_id'") fk_exists=$(q "$C" "select count(*) from information_schema.key_column_usage where table_schema='t11r2e2' and referenced_table_name='p'")"
emit "\$ ALTER TABLE t11r2e2.t DROP INDEX ix_c; (child supporting index)"
emit "$(run "$C" "alter table t11r2e2.t drop index ix_c")"
emit "  post: ix_c_exists=$(q "$C" "select count(*) from information_schema.statistics where table_schema='t11r2e2' and index_name='ix_c'")"

# ---------- F cells: DROP CHECK|CONSTRAINT target types ----------
for C in deltascope-ddl-golden-mysql80 deltascope-ddl-golden-mysql84; do
  cell "F target types ($C)"
  q "$C" "drop schema if exists t11r2f; create schema t11r2f;
         create table t11r2f.t(id int not null primary key, v int, u int, unique key uk_v(v), key ix_u(u), check (v>0));
         create table t11r2f.f(id int not null primary key, c int, constraint fk_c foreign key(c) references t11r2f.t(id))" >>"$LOG" 2>&1
  for KW in "drop constraint uk_v" "drop check uk_v" "drop constraint ix_u" "drop constraint missing_x" "drop check missing_x" "drop constraint primary" "drop check primary"; do
    emit "\$ ALTER TABLE t11r2f.t $KW;"
    emit "$(run "$C" "alter table t11r2f.t $KW")"
    emit "  post: uk_v=$(q "$C" "select count(*) from information_schema.statistics where table_schema='t11r2f' and index_name='uk_v'") ix_u=$(q "$C" "select count(*) from information_schema.statistics where table_schema='t11r2f' and index_name='ix_u'") pk=$(q "$C" "select count(*) from information_schema.table_constraints where table_schema='t11r2f' and constraint_type='PRIMARY KEY'") chk=$(q "$C" "select count(*) from information_schema.table_constraints where table_schema='t11r2f' and constraint_type='CHECK'")"
  done
  emit "\$ ALTER TABLE t11r2f.t DROP CONSTRAINT chk-named? (real CHECK survives above; drop by auto name)"
  chk_name=$(q "$C" "select constraint_name from information_schema.table_constraints where table_schema='t11r2f' and constraint_type='CHECK' limit 1")
  emit "  check_name=$chk_name"
  emit "$(run "$C" "alter table t11r2f.t drop constraint \`$chk_name\`")"
  emit "  post chk=$(q "$C" "select count(*) from information_schema.table_constraints where table_schema='t11r2f' and constraint_type='CHECK'")"
done
cell "F 5.7 syntax control"
emit "$(run deltascope-ddl-golden-mysql57 "alter table t11r2f.t drop constraint uk_v" 2>/dev/null; docker exec deltascope-ddl-golden-mysql57 mysql -uroot -proot -e "create schema if not exists t11r2f; create table if not exists t11r2f.t(id int primary key, v int, unique key uk_v(v))" >/dev/null 2>&1; docker exec deltascope-ddl-golden-mysql57 mysql -uroot -proot -N -B -e "alter table t11r2f.t drop constraint uk_v" 2>&1; echo "  rc=$?")"

# ---------- TiDB flag arms ----------
cell "TiDB tidb_enable_check_constraint OFF/ON (v8.5.0)"
Q="docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot -N -B -e"
emit "var=$( $Q "select @@tidb_enable_check_constraint" 2>&1)"
$Q "drop schema if exists t11r2t; create schema t11r2t;
   create table t11r2t.p(id int not null primary key);
   create table t11r2t.t(id int not null primary key, c int, constraint fk_c foreign key(c) references t11r2t.p(id))" >>"$LOG" 2>&1
for FLAG in 0 1; do
  emit "\$ SET GLOBAL tidb_enable_check_constraint=$FLAG"
  emit "$( $Q "set global tidb_enable_check_constraint=$FLAG; select @@global.tidb_enable_check_constraint" 2>&1)"
  emit "  session=$( $Q "select @@tidb_enable_check_constraint" 2>&1)"
  emit "\$ ALTER TABLE t11r2t.t DROP CONSTRAINT fk_c; SHOW WARNINGS;"
  emit "$( $Q "alter table t11r2t.t drop constraint fk_c; show warnings" 2>&1)"
  emit "  rc-of-ddl-alone: $( $Q "alter table t11r2t.t drop constraint fk_c" 2>&1; echo "  rc=$?")"
  emit "  post fk KCU=$( $Q "select count(*) from information_schema.key_column_usage where table_schema='t11r2t' and referenced_table_name is not null" 2>&1)"
  emit "\$ ALTER TABLE t11r2t.t DROP CHECK fk_c; SHOW WARNINGS;"
  emit "$( $Q "alter table t11r2t.t drop check fk_c; show warnings" 2>&1)"
  emit "  post fk KCU=$( $Q "select count(*) from information_schema.key_column_usage where table_schema='t11r2t' and referenced_table_name is not null" 2>&1)"
done
$Q "set global tidb_enable_check_constraint=OFF" >>"$LOG" 2>&1
emit "## DONE"
