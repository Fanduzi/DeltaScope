#!/usr/bin/env bash
# T16 supplemental probe — fixture fixes and remaining cells:
#  - LIKE member fidelity with a valid generated-column/FK fixture
#  - temp-target fate under a FAILED atomic multi-DROP (--force continuation)
#  - truncating an inbound-referenced parent (FK 1701)
#  - RENAME on a TiDB global temporary table
#  - MySQL-side DROP GLOBAL TEMPORARY keyword form
# Usage: probe16_supplement.sh [anchor ...]
set -u
out="${OUT:-$(dirname "$0")/p16_tail_supplement.log}"
: >"$out"

cli() {
  local tag="$1"; shift
  case "$tag" in
    mysql57) docker exec deltascope-ddl-golden-mysql57 mysql -uroot -proot --default-character-set=utf8mb4 golden "$@" ;;
    mysql80) docker exec deltascope-ddl-golden-mysql80 mysql -uroot -proot --default-character-set=utf8mb4 golden "$@" ;;
    mysql84) docker exec deltascope-ddl-golden-mysql84 mysql -uroot -proot --default-character-set=utf8mb4 golden "$@" ;;
    tidb85) docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot --default-character-set=utf8mb4 golden "$@" ;;
  esac
}
q() { cli "$1" -N -e "$2"; }
say() { printf '%s\n' "$*" | tee -a "$out"; }
clean() {
  cli "$1" -N -e "drop table if exists golden.p16_ref,golden.p16_src,golden.p16_l1,golden.p16_a,golden.p16_b,golden.tp16,golden.gp16,golden.gp16x,golden.p16_c7,golden.p16_l7" >/dev/null 2>&1
  cli "$1" -N -e "create database if not exists golden" >/dev/null 2>&1
}
sess() {
  local tag="$1" label="$2" sql="$3" rc err
  clean "$tag"
  say "=== $tag :: $label"
  say "SQL> $sql"
  err=$(cli "$tag" -e "$sql" 2>&1); rc=$?
  say "rc=$rc"; [ -n "$err" ] && say "out> $err"
}
sessf() { # same but mysql --force: later statements run despite errors
  local tag="$1" label="$2" sql="$3" rc err
  clean "$tag"
  say "=== $tag :: $label (--force)"
  say "SQL> $sql"
  err=$(cli "$tag" --force -e "$sql" 2>&1); rc=$?
  say "rc=$rc"; [ -n "$err" ] && say "out> $err"
}
exists() { say "exists[$1::$2]> $(q "$1" "select count(*) from information_schema.tables where table_schema='golden' and table_name='$2'" 2>&1)"; }

for tag in ${*:-mysql57 mysql80 mysql84 tidb85}; do
  say "######## $tag"
  sess "$tag" "mk-src-fixed" "create table p16_ref(id int primary key);create table p16_src(id int auto_increment primary key,n varchar(20) not null default 'x',g int generated always as (fk*2) stored,u int unique,k int,key(k),fk int,foreign key(fk) references p16_ref(id),constraint ck check(n<>'')) engine=innodb row_format=dynamic comment='src' auto_increment=100"
  sess "$tag" "like-members" "create table p16_ref(id int primary key);create table p16_src(id int auto_increment primary key,n varchar(20) not null default 'x',g int generated always as (fk*2) stored,u int unique,k int,key(k),fk int,foreign key(fk) references p16_ref(id),constraint ck check(n<>'')) engine=innodb row_format=dynamic comment='src' auto_increment=100;create table p16_l1 like p16_src;show create table p16_l1"
  say "fks[$tag::p16_l1]> $(q "$tag" "select constraint_name,referenced_table_name from information_schema.referential_constraints where constraint_schema='golden' and table_name='p16_l1'" 2>&1 | tr '\n' '|')"
  sess "$tag" "drop-failed-atomic-temp-fate" "create temporary table tp16(id int);create table p16_a(id int);drop table tp16,p16_missing;select count(*) c from tp16"
  sessf "$tag" "drop-failed-atomic-temp-fate-f" "create temporary table tp16(id int);create table p16_a(id int);drop table tp16,p16_missing;select count(*) c from tp16;select count(*) a from p16_a"
  sessf "$tag" "drop-missing-first-temp-fate-f" "create temporary table tp16(id int);create table p16_a(id int);drop table p16_missing,tp16;select count(*) c from tp16;select count(*) a from p16_a"
  sessf "$tag" "drop-multi-partial-57-f" "create table p16_a(id int);create table p16_b(id int);drop table p16_a,p16_missing,p16_b;select count(*) a from p16_a;select count(*) b from p16_b"
  sess "$tag" "truncate-inbound-parent" "create table p16_ref(id int primary key);create table p16_a(id int,foreign key(id) references p16_ref(id));truncate table p16_ref;show warnings"
  exists "$tag" "p16_ref"
  sess "$tag" "like-from-temp-shape" "create temporary table tp16(id int primary key,u int unique,key(u));create table p16_l7 like tp16;show create table p16_l7"
  sess "$tag" "rename-globaltemp-tidb" "create global temporary table gp16(id int) on commit delete rows;rename table gp16 to gp16x;show tables like 'gp16'"
  sess "$tag" "alter-rename-globaltemp-tidb" "create global temporary table gp16(id int) on commit delete rows;alter table gp16 rename to gp16x;describe gp16x"
  sess "$tag" "drop-global-kw-mysql" "drop global temporary table gp16"
  sess "$tag" "drop-temp-ifexists-mixed-temp" "create temporary table tp16(id int);drop temporary table if exists tp16,tp16x;show warnings"
  sess "$tag" "ctas-union-tables" "create table p16_a(id int);insert into p16_a values(1);create table p16_b(id2 int);insert into p16_b values(2);create table p16_c7 as select * from p16_a union all select * from p16_b;select * from p16_c7"
  sess "$tag" "create-temp-ifnotexists-noobj" "create temporary table if not exists tp16(id int);show warnings;insert into tp16 values(1);select count(*) c from tp16"
  sess "$tag" "alter-temp-addcol-global" "create global temporary table gp16(id int) on commit delete rows;alter table gp16 add column x int;describe gp16"
  sess "$tag" "rename-table-multi-tempsrc" "create temporary table tp16(id int);create table p16_a(id int);rename table tp16 to tp16x,p16_a to p16_b;show tables like 'p16_b'"
  exists "$tag" "p16_b"
done
say "DONE"
