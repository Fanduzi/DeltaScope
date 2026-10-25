#!/bin/bash
# T11-R2-tail targeted native probe — fills ONLY the cells the earlier
# probes ran but never archived. Real client rc captured from docker exec,
# never from a log filter. Each cell prints fixture, SQL, rc, post-state.
# Cells covered here:
#   A: MySQL 8.0.46 / 8.4.10  ADD COLUMN+ADD PK both orders, MODIFY+ADD PK
#   A: TiDB v8.5.0            same-statement member touch -> 1072 / 8200
#   D: TiDB v8.5.0            partial UNIQUE / PK / secondary FK acceptance
#   F: MySQL 8.0.46 / 8.4.10  DROP CHECK on a STILL-PRESENT object
#     (the earlier r2_probe2.log cells ran DROP CHECK only after DROP
#     CONSTRAINT had already removed the target — see README caveat; the
#     cells below run the refusal while the object provably exists)
set -u
LOG="$1"
: > "$LOG"

cell() { echo "=== CELL $1" | tee -a "$LOG"; }
emit() { echo "$1" | tee -a "$LOG"; }

myq() { docker exec "$1" mysql -uroot -proot -N -B -e "$2" 2>&1; }
tiq() { docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot -N -B -e "$1" 2>&1; }

run_my() { # container sql
  local out rc
  out="$(myq "$1" "$2")"; rc=$?
  emit "$out"; emit "  rc=$rc"
}
run_ti() { # sql
  local out rc
  out="$(tiq "$1")"; rc=$?
  emit "$out"; emit "  rc=$rc"
}

# ---------- A cells: same-statement member declaration, MySQL ----------
for C in deltascope-ddl-golden-mysql80 deltascope-ddl-golden-mysql84; do
  cell "A same-statement decls ($C)"
  emit "version: $(myq "$C" "select @@version")"
  myq "$C" "drop schema if exists t11r2a; create schema t11r2a" >>"$LOG" 2>&1
  myq "$C" "create table t11r2a.t1(k int); create table t11r2a.t2(id int); create table t11r2a.t3(k int)" >>"$LOG" 2>&1
  emit "fixture: t1(k int), t2(id int), t3(k int)"
  emit "\$ ALTER TABLE t11r2a.t1 ADD COLUMN id INT, ADD PRIMARY KEY (id);"
  run_my "$C" "alter table t11r2a.t1 add column id int, add primary key (id)"
  emit "  post pk=$(myq "$C" "select count(*) from information_schema.table_constraints where table_schema='t11r2a' and table_name='t1' and constraint_type='PRIMARY KEY'")"
  emit "\$ ALTER TABLE t11r2a.t3 ADD PRIMARY KEY (id), ADD COLUMN id INT;   (member declared later in the same statement)"
  run_my "$C" "alter table t11r2a.t3 add primary key (id), add column id int"
  emit "  post pk=$(myq "$C" "select count(*) from information_schema.table_constraints where table_schema='t11r2a' and table_name='t3' and constraint_type='PRIMARY KEY'")"
  emit "\$ ALTER TABLE t11r2a.t2 MODIFY COLUMN id BIGINT NOT NULL, ADD PRIMARY KEY (id);"
  run_my "$C" "alter table t11r2a.t2 modify column id bigint not null, add primary key (id)"
  emit "  post pk=$(myq "$C" "select count(*) from information_schema.table_constraints where table_schema='t11r2a' and table_name='t2' and constraint_type='PRIMARY KEY'") coltype=$(myq "$C" "select data_type from information_schema.columns where table_schema='t11r2a' and table_name='t2' and column_name='id'")"
done

# ---------- A cells: same-statement member declaration, TiDB ----------
cell "A same-statement decls (TiDB v8.5.0)"
emit "version: $(tiq "select tidb_version()" | head -1)"
tiq "drop schema if exists t11r2a; create schema t11r2a" >>"$LOG" 2>&1
tiq "create table t11r2a.t1(k int); create table t11r2a.t2(id int)" >>"$LOG" 2>&1
emit "fixture: t1(k int), t2(id int)"
emit "\$ ALTER TABLE t11r2a.t1 ADD COLUMN id INT, ADD PRIMARY KEY (id);   (expect 1072 — member unknown to pre-state)"
run_ti "alter table t11r2a.t1 add column id int, add primary key (id)"
emit "  post pk=$(tiq "select count(*) from information_schema.table_constraints where table_schema='t11r2a' and table_name='t1' and constraint_type='PRIMARY KEY'")"
emit "\$ ALTER TABLE t11r2a.t2 MODIFY COLUMN id BIGINT NOT NULL, ADD PRIMARY KEY (id);   (expect 8200 clustered)"
run_ti "alter table t11r2a.t2 modify column id bigint not null, add primary key (id)"
emit "  post pk=$(tiq "select count(*) from information_schema.table_constraints where table_schema='t11r2a' and table_name='t2' and constraint_type='PRIMARY KEY'")"

# ---------- D cells: partial referenced key acceptance, TiDB ----------
cell "D partial referenced keys (TiDB v8.5.0 — no restrict variable)"
tiq "drop schema if exists t11r2d; create schema t11r2d" >>"$LOG" 2>&1
tiq "create table t11r2d.puq(a int not null,b int not null, unique key uq_ab(a,b));
     create table t11r2d.ppk(a int not null,b int not null, primary key(a,b));
     create table t11r2d.psc(a int not null,b int not null, key ix_ab(a,b));
     create table t11r2d.c1(c int);
     create table t11r2d.c2(c int);
     create table t11r2d.c3(c int)" >>"$LOG" 2>&1
emit "fixture: puq unique(a,b); ppk primary(a,b); psc key(a,b); children c1..c3(c int)"
emit "\$ ALTER TABLE t11r2d.c1 ADD CONSTRAINT fk1 FOREIGN KEY(c) REFERENCES t11r2d.puq(a);  (partial unique)"
run_ti "alter table t11r2d.c1 add constraint fk1 foreign key(c) references t11r2d.puq(a)"
emit "  post KCU=$(tiq "select count(*) from information_schema.key_column_usage where table_schema='t11r2d' and referenced_table_name is not null")"
emit "\$ ALTER TABLE t11r2d.c2 ADD CONSTRAINT fk2 FOREIGN KEY(c) REFERENCES t11r2d.ppk(a);  (partial pk)"
run_ti "alter table t11r2d.c2 add constraint fk2 foreign key(c) references t11r2d.ppk(a)"
emit "  post KCU=$(tiq "select count(*) from information_schema.key_column_usage where table_schema='t11r2d' and referenced_table_name is not null")"
emit "\$ ALTER TABLE t11r2d.c3 ADD CONSTRAINT fk3 FOREIGN KEY(c) REFERENCES t11r2d.psc(a);  (partial secondary)"
run_ti "alter table t11r2d.c3 add constraint fk3 foreign key(c) references t11r2d.psc(a)"
emit "  post KCU=$(tiq "select count(*) from information_schema.key_column_usage where table_schema='t11r2d' and referenced_table_name is not null")"

# ---------- F cells: DROP CHECK/CONSTRAINT on a STILL-PRESENT object ----------
for C in deltascope-ddl-golden-mysql80 deltascope-ddl-golden-mysql84; do
  cell "F present-object refusals ($C)"
  emit "version: $(myq "$C" "select @@version")"
  myq "$C" "drop schema if exists t11r2f3; create schema t11r2f3" >>"$LOG" 2>&1
  myq "$C" "create table t11r2f3.t(id int not null primary key, v int, u int, w int, unique key uk_v(v), key ix_u(u), key ix_w(w), constraint chk_w check (w > 0))" >>"$LOG" 2>&1
  emit "fixture: t(pk id, unique uk_v(v), secondary ix_u(u), secondary ix_w(w), check chk_w)"
  emit "  pre: uk_v=$(myq "$C" "select count(*) from information_schema.statistics where table_schema='t11r2f3' and index_name='uk_v'") ix_u=$(myq "$C" "select count(*) from information_schema.statistics where table_schema='t11r2f3' and index_name='ix_u'") chk=$(myq "$C" "select count(*) from information_schema.check_constraints where constraint_schema='t11r2f3'")"
  emit "\$ ALTER TABLE t11r2f3.t DROP CHECK uk_v;   (uk_v present — expect refusal, not removal)"
  run_my "$C" "alter table t11r2f3.t drop check uk_v"
  emit "  post: uk_v=$(myq "$C" "select count(*) from information_schema.statistics where table_schema='t11r2f3' and index_name='uk_v'") chk=$(myq "$C" "select count(*) from information_schema.check_constraints where constraint_schema='t11r2f3'")"
  emit "\$ ALTER TABLE t11r2f3.t DROP CHECK ix_u;   (ix_u present — expect refusal)"
  run_my "$C" "alter table t11r2f3.t drop check ix_u"
  emit "  post: ix_u=$(myq "$C" "select count(*) from information_schema.statistics where table_schema='t11r2f3' and index_name='ix_u'")"
  emit "\$ ALTER TABLE t11r2f3.t DROP CONSTRAINT ix_w;   (secondary present — expect 3940 refusal)"
  run_my "$C" "alter table t11r2f3.t drop constraint ix_w"
  emit "  post: ix_w=$(myq "$C" "select count(*) from information_schema.statistics where table_schema='t11r2f3' and index_name='ix_w'")"
  emit "\$ ALTER TABLE t11r2f3.t DROP CHECK chk_w;   (real check present — expect removal rc=0)"
  run_my "$C" "alter table t11r2f3.t drop check chk_w"
  emit "  post: chk=$(myq "$C" "select count(*) from information_schema.check_constraints where constraint_schema='t11r2f3'")"
done
