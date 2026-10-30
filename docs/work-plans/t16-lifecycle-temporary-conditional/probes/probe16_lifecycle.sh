#!/usr/bin/env bash
# T16 native probe — table lifecycle (DROP/TRUNCATE/RENAME), CREATE TABLE LIKE,
# CTAS, temporary-table identity, IF EXISTS / IF NOT EXISTS conditional clauses.
# Executes fixed synthetic fixture SQL on the four frozen anchors and records
# per-label rc / stdout / error text / SHOW WARNINGS / catalog facts. Every
# object is a disposable p16* / tp16* / gp16* fixture inside the golden schema;
# user SQL is never executed.
#
# IMPORTANT: statements whose truth depends on session identity run inside ONE
# mysql invocation (one label = one session) so temporary tables survive to the
# later statements that observe them. mysql -e "s1;s2;..." stops at the first
# error — the error line identifies the failing statement.
#
# Usage: probe16_lifecycle.sh [anchor ...]   (anchors: mysql57 mysql80 mysql84 tidb85)
set -u
out="${OUT:-$(dirname "$0")/p16_lifecycle_matrix.log}"
: >"$out"

cli() { # $1 anchor tag, rest -> mysql client argv (with golden default db)
  local tag="$1"; shift
  case "$tag" in
    mysql57) docker exec deltascope-ddl-golden-mysql57 mysql -uroot -proot --default-character-set=utf8mb4 golden "$@" ;;
    mysql80) docker exec deltascope-ddl-golden-mysql80 mysql -uroot -proot --default-character-set=utf8mb4 golden "$@" ;;
    mysql84) docker exec deltascope-ddl-golden-mysql84 mysql -uroot -proot --default-character-set=utf8mb4 golden "$@" ;;
    tidb85) docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot --default-character-set=utf8mb4 golden "$@" ;;
  esac
}
clinodb() { # same, without a default database (for schema bootstrap)
  local tag="$1"; shift
  case "$tag" in
    mysql57) docker exec deltascope-ddl-golden-mysql57 mysql -uroot -proot "$@" ;;
    mysql80) docker exec deltascope-ddl-golden-mysql80 mysql -uroot -proot "$@" ;;
    mysql84) docker exec deltascope-ddl-golden-mysql84 mysql -uroot -proot "$@" ;;
    tidb85) docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot "$@" ;;
  esac
}
q() { cli "$1" -N -e "$2"; }
say() { printf '%s\n' "$*" | tee -a "$out"; }
cleanup() {
  clinodb "$1" -e "drop table if exists golden.p16_a,golden.p16_b,golden.p16_c,golden.p16_d,golden.p16_r1,golden.p16_r1x,golden.p16_r2,golden.p16_r2x,golden.p16_z,golden.p16_src,golden.p16_src2,golden.p16_src3,golden.p16_l1,golden.p16_l2,golden.p16_l3,golden.p16_l4,golden.p16_l5,golden.p16_l6,golden.p16_l7,golden.p16_c1,golden.p16_c2,golden.p16_c3,golden.p16_c4,golden.p16_c5,golden.p16_c6,golden.p16_c7,golden.p16_t,golden.p16_t2,golden.p16_ref,golden.p16_keep,golden.p16_missing,golden.p16_shadow,golden.tp16,golden.tp16l,golden.tp16t,golden.gp16; drop table if exists golden2.p16_a,golden2.p16_t,golden2.p16_r1x,golden2.p16_x" >/dev/null 2>&1
}
sess() { # $1 tag, $2 label, $3 sql batch — ONE session after fixture cleanup
  local tag="$1" label="$2" sql="$3" rc err
  cleanup "$tag"
  say "=== $tag :: $label"
  say "SQL> $sql"
  err=$(cli "$tag" -e "$sql" 2>&1); rc=$?
  say "rc=$rc"
  [ -n "$err" ] && say "out> $err"
}
obj() { say "catalog[$1::$2]> $(q "$1" "select engine,table_type,table_collation,auto_increment,create_options from information_schema.tables where table_schema='golden' and table_name='$2'" 2>&1 | tr '\n' '|')"; }
exists() { say "exists[$1::$2]> $(q "$1" "select count(*) from information_schema.tables where table_schema='golden' and table_name='$2'" 2>&1)"; }
cols() { say "columns[$1::$2]> $(q "$1" "select column_name,column_type,extra from information_schema.columns where table_schema='golden' and table_name='$2' order by ordinal_position" 2>&1 | tr '\n' '|')"; }
idx() { say "indexes[$1::$2]> $(q "$1" "select index_name,min(non_unique),group_concat(column_name order by seq_in_index) from information_schema.statistics where table_schema='golden' and table_name='$2' group by index_name" 2>&1 | tr '\n' '|')"; }
fk() { say "fks[$1::$2]> $(q "$1" "select constraint_name,referenced_table_name from information_schema.referential_constraints where constraint_schema='golden' and table_name='$2'" 2>&1 | tr '\n' '|')"; }
parts() { say "parts[$1::$2]> $(q "$1" "select partition_name,partition_description from information_schema.partitions where table_schema='golden' and table_name='$2' and partition_name is not null" 2>&1 | tr '\n' '|')"; }

anchors="${*:-mysql57 mysql80 mysql84 tidb85}"
for tag in $anchors; do
  say "################################################ $tag"
  clinodb "$tag" -e "create database if not exists golden; create database if not exists golden2" >/dev/null 2>&1
  say "version> $(q "$tag" "select version()" 2>&1)"

  # ---------- P1 multi-target DROP semantics ----------
  sess "$tag" "drop-multi-all-present" "create table p16_a(id int);create table p16_b(id int);drop table p16_a,p16_b"
  exists "$tag" "p16_a"; exists "$tag" "p16_b"
  sess "$tag" "drop-multi-second-missing" "create table p16_a(id int);drop table p16_a,p16_missing;show warnings"
  exists "$tag" "p16_a"; exists "$tag" "p16_missing"
  sess "$tag" "drop-multi-first-missing" "create table p16_b(id int);drop table p16_missing,p16_b;show warnings"
  exists "$tag" "p16_b"
  sess "$tag" "drop-multi-two-missing" "create table p16_a(id int);drop table p16_a,p16_missing,p16_z;show warnings"
  exists "$tag" "p16_a"
  sess "$tag" "drop-multi-ifexists-mixed" "create table p16_a(id int);create table p16_b(id int);drop table if exists p16_a,p16_missing,p16_b;show warnings"
  exists "$tag" "p16_a"; exists "$tag" "p16_b"
  sess "$tag" "drop-ifexists-one-missing" "drop table if exists p16_missing;show warnings"
  sess "$tag" "drop-duplicate-name" "create table p16_a(id int);drop table p16_a,p16_a;show warnings"
  exists "$tag" "p16_a"
  sess "$tag" "drop-restrict" "create table p16_a(id int);drop table p16_a restrict;show warnings"
  exists "$tag" "p16_a"
  sess "$tag" "drop-cascade" "create table p16_a(id int);drop table p16_a cascade;show warnings"
  exists "$tag" "p16_a"
  sess "$tag" "drop-restrict-missing" "drop table p16_missing restrict;show warnings"
  sess "$tag" "drop-table-absent-nocond" "drop table p16_missing"
  sess "$tag" "drop-temp-keyword-persistent" "create table p16_t(id int);drop temporary table p16_t;show warnings"
  exists "$tag" "p16_t"

  # ---------- P2 temporary identity (one session per label) ----------
  sess "$tag" "temp-create-select-drop" "create temporary table tp16(id int);insert into tp16 values(1);select count(*) c from tp16;drop temporary table tp16;show tables like 'tp16'"
  sess "$tag" "drop-temp-missing" "drop temporary table tp16;show warnings"
  sess "$tag" "drop-temp-ifexists-missing" "drop temporary table if exists tp16;show warnings"
  sess "$tag" "drop-plain-on-temp" "create temporary table tp16(id int);insert into tp16 values(1);drop table tp16;select count(*) c from tp16"
  sess "$tag" "temp-shadows-persistent" "create table p16_t(id int,x int);insert into p16_t values(1,1);create temporary table p16_t(id int);insert into p16_t values(2);select * from p16_t;drop table p16_t;select * from p16_t"
  sess "$tag" "temp-drop-keyword-shadowed" "create table p16_t(id int,x int);insert into p16_t values(1,1);create temporary table p16_t(id int);insert into p16_t values(2);drop temporary table p16_t;select * from p16_t"
  sess "$tag" "create-persistent-over-temp" "create temporary table p16_t(id int);create table p16_t(id int,x int);insert into p16_t values(1,1);select * from p16_t;drop temporary table p16_t;select * from p16_t"
  sess "$tag" "create-temp-over-persistent" "create table p16_t(id int,x int);create temporary table p16_t(id int);insert into p16_t values(2);select * from p16_t"
  sess "$tag" "create-temp-ifnotexists-over-persistent" "create table p16_t(id int,x int);create temporary table if not exists p16_t(id int);show warnings;insert into p16_t values(2);select * from p16_t"
  sess "$tag" "create-temp-duplicate" "create temporary table tp16(id int);create temporary table tp16(id int)"
  sess "$tag" "create-temp-ifnotexists" "create temporary table tp16(id int);create temporary table if not exists tp16(a int);show warnings;insert into tp16 values(9);select count(*) c from tp16"
  sess "$tag" "alter-on-temp" "create temporary table tp16(id int);alter table tp16 add column x int;describe tp16"
  sess "$tag" "truncate-temp" "create temporary table tp16(id int);insert into tp16 values(1);truncate table tp16;select count(*) c from tp16"
  sess "$tag" "rename-table-on-temp" "create temporary table tp16(id int);rename table tp16 to tp16t;select count(*) c from tp16t"
  sess "$tag" "alter-rename-on-temp" "create temporary table tp16(id int);alter table tp16 rename to tp16t;select count(*) c from tp16t"
  sess "$tag" "rename-table-shadowed" "create table p16_t(id int,x int);insert into p16_t values(1,1);create temporary table p16_t(id int);insert into p16_t values(2);rename table p16_t to p16_t2;select * from p16_t;select * from p16_t2"
  sess "$tag" "alter-rename-shadowed" "create table p16_t(id int,x int);insert into p16_t values(1,1);create temporary table p16_t(id int);insert into p16_t values(2);alter table p16_t rename to p16_t2;select * from p16_t;select * from p16_t2"
  sess "$tag" "temp-like" "create table p16_src(id int primary key,u int unique,k int,key(k));create temporary table tp16l like p16_src;describe tp16l"
  sess "$tag" "like-from-temp" "create temporary table tp16(id int);create table p16_l7 like tp16"
  sess "$tag" "ctas-from-temp" "create temporary table tp16(id int);insert into tp16 values(7);create table p16_c7 as select * from tp16;select * from p16_c7"
  sess "$tag" "temp-ctas" "create temporary table tp16t select 1 a;select * from tp16t"
  sess "$tag" "drop-multi-temp-and-persistent" "create temporary table tp16(id int);create table p16_a(id int);drop table tp16,p16_a;select count(*) c from tp16"
  exists "$tag" "p16_a"
  sess "$tag" "drop-multi-temp-missing" "create temporary table tp16(id int);create table p16_a(id int);drop table tp16,p16_missing;select count(*) c from tp16"
  exists "$tag" "p16_a"

  # ---------- P3 CREATE TABLE LIKE ----------
  sess "$tag" "mk-src" "create table p16_ref(id int primary key);create table p16_src(id int auto_increment primary key,n varchar(20) not null default 'x',g int generated always as (id*2) stored,u int unique,k int,key(k),fk int,foreign key(fk) references p16_ref(id),constraint ck check(n<>'')) engine=innodb row_format=dynamic comment='src' auto_increment=100"
  idx "$tag" "p16_src"; fk "$tag" "p16_src"
  sess "$tag" "mk-src-clean" "create table p16_ref(id int primary key);drop table if exists p16_src;create table p16_src(id int auto_increment primary key,n varchar(20) not null default 'x',g int generated always as (id*2) stored,u int unique,k int,key(k),fk int,foreign key(fk) references p16_ref(id),constraint ck check(n<>'')) engine=innodb row_format=dynamic comment='src' auto_increment=100"
  sess "$tag" "create-like" "drop table if exists p16_l1;create table p16_l1 like p16_src;show create table p16_l1"
  cols "$tag" "p16_l1"; idx "$tag" "p16_l1"; fk "$tag" "p16_l1"; obj "$tag" "p16_l1"
  sess "$tag" "like-missing-src" "create table p16_l2 like p16_missing;show warnings"
  exists "$tag" "p16_l2"
  sess "$tag" "like-ifnotexists-dest" "create table p16_l3(a int);create table if not exists p16_l3 like p16_src;show warnings;describe p16_l3"
  sess "$tag" "like-existing-dest" "create table p16_l4(a int);create table p16_l4 like p16_src"
  sess "$tag" "like-cross-schema" "create table golden2.p16_a like golden.p16_src;describe golden2.p16_a"
  sess "$tag" "like-partitioned" "create table p16_src2(id int) partition by range(id)(partition p0 values less than(10),partition p1 values less than(20));create table p16_l5 like p16_src2;show create table p16_l5"
  parts "$tag" "p16_l5"
  sess "$tag" "like-of-like" "create table p16_l1 like p16_src;create table p16_l6 like p16_l1;describe p16_l6"
  sess "$tag" "like-autoinc-counter" "create table p16_src(id int auto_increment primary key) auto_increment=100;create table p16_l1 like p16_src;insert into p16_l1 values(null);select * from p16_l1;show table status like 'p16_l1'"

  # ---------- P4 CTAS ----------
  sess "$tag" "ctas-as" "create table p16_c1 as select 1 a,'x' b;show create table p16_c1;select * from p16_c1"
  cols "$tag" "p16_c1"
  sess "$tag" "ctas-no-as" "create table p16_c2 select 1 a;describe p16_c2"
  sess "$tag" "ctas-ignore" "create table p16_src3(a int primary key);insert into p16_src3 values(1),(2);create table p16_c3 ignore select * from p16_src3;select * from p16_c3"
  sess "$tag" "ctas-ignore-dupkey" "create table p16_src3(a int primary key);insert into p16_src3 values(1),(2);create table p16_c4(a int primary key) ignore select 1 a union all select 1 union all select 3;select * from p16_c4"
  sess "$tag" "ctas-replace-fresh" "create table p16_c4(a int primary key) replace select 1 a union all select 1 union all select 3;select * from p16_c4"
  sess "$tag" "ctas-nokey-dup" "create table p16_c4(a int) select 1 a union all select 1;select * from p16_c4"
  sess "$tag" "ctas-defs-plus-select" "create table p16_src3(a int primary key);insert into p16_src3 values(1),(2);create table p16_c5(id int primary key,extra int) select a+100,a*10 from p16_src3;show create table p16_c5;select * from p16_c5"
  cols "$tag" "p16_c5"
  sess "$tag" "ctas-ifnotexists-existing" "create table p16_c6(a int);create table if not exists p16_c6 as select 1 b;show warnings;describe p16_c6"
  sess "$tag" "ctas-existing-nocond" "create table p16_a(a int);create table p16_a as select 1 b"
  sess "$tag" "ctas-missing-src" "create table p16_c1 as select * from p16_missing"
  exists "$tag" "p16_c1"
  sess "$tag" "ctas-onto-temp-name" "create temporary table p16_c1(id int);insert into p16_c1 values(5);create table p16_c1 as select 9 a;select * from p16_c1"

  # ---------- P5 RENAME ----------
  sess "$tag" "rename-multi-ok" "create table p16_r1(id int);create table p16_r2(id int);rename table p16_r1 to p16_r1x,p16_r2 to p16_r2x"
  exists "$tag" "p16_r1"; exists "$tag" "p16_r1x"; exists "$tag" "p16_r2"; exists "$tag" "p16_r2x"
  sess "$tag" "rename-multi-second-missing" "create table p16_r1(id int);rename table p16_r1 to p16_r1x,p16_missing to p16_z;show warnings"
  exists "$tag" "p16_r1"; exists "$tag" "p16_r1x"; exists "$tag" "p16_z"
  sess "$tag" "rename-multi-first-missing" "create table p16_r2(id int);rename table p16_missing to p16_z,p16_r2 to p16_r2x;show warnings"
  exists "$tag" "p16_r2"; exists "$tag" "p16_r2x"; exists "$tag" "p16_z"
  sess "$tag" "rename-dest-exists" "create table p16_r1(id int);create table p16_r2(id int);rename table p16_r1 to p16_r2;show warnings"
  exists "$tag" "p16_r1"; exists "$tag" "p16_r2"
  sess "$tag" "alter-rename-dest-exists" "create table p16_r1(id int);create table p16_r2(id int);alter table p16_r1 rename to p16_r2;show warnings"
  exists "$tag" "p16_r1"; exists "$tag" "p16_r2"
  sess "$tag" "rename-cross-schema" "create table p16_r1(id int);alter table golden.p16_r1 rename to golden2.p16_r1x"
  say "schemas[p16_r1x]> $(q "$tag" "select group_concat(table_schema,'.',table_name) from information_schema.tables where table_name='p16_r1x'" 2>&1)"
  exists "$tag" "p16_r1"
  sess "$tag" "rename-to-missing-schema" "create table p16_r1(id int);rename table p16_r1 to noschema.p16_r1x;show warnings"
  exists "$tag" "p16_r1"
  sess "$tag" "rename-swap" "create table p16_r1(id int);create table p16_r2(id int);rename table p16_r1 to p16_keep,p16_r2 to p16_r1,p16_keep to p16_r2;select count(*) a from p16_r1;select count(*) b from p16_r2"
  exists "$tag" "p16_keep"
  sess "$tag" "rename-onto-temp-name" "create temporary table tp16(id int);create table p16_r1(id int);rename table p16_r1 to tp16;select count(*) c from tp16;drop table tp16"
  sess "$tag" "alter-rename-to-same" "create table p16_r1(id int);alter table p16_r1 rename to p16_r1;show warnings"
  exists "$tag" "p16_r1"

  # ---------- P6 TRUNCATE ----------
  sess "$tag" "truncate-ifexists" "truncate table if exists p16_missing"
  sess "$tag" "truncate-missing" "truncate table p16_missing"
  sess "$tag" "truncate-multi" "create table p16_a(id int);create table p16_b(id int);truncate table p16_a,p16_b"
  sess "$tag" "truncate-resets-autoinc" "create table p16_a(id int auto_increment primary key);insert into p16_a values(null),(null);truncate table p16_a;insert into p16_a values(null);select * from p16_a"
  obj "$tag" "p16_a"
  sess "$tag" "truncate-fk-parent" "create table p16_ref(id int primary key);create table p16_a(id int,foreign key(id) references p16_ref(id));truncate table p16_a;show warnings"
  exists "$tag" "p16_a"
  sess "$tag" "truncate-temp-persistent" "create table p16_a(id int auto_increment primary key);insert into p16_a values(null);create temporary table tp16(id int auto_increment primary key);insert into tp16 values(null);truncate table tp16;insert into tp16 values(null);select * from tp16"

  # ---------- P7 dialect-specific temp shapes ----------
  sess "$tag" "global-temp" "create global temporary table gp16(id int) on commit delete rows;insert into gp16 values(1);select count(*) before_commit from gp16;commit;select count(*) after_commit from gp16;show create table gp16"
  exists "$tag" "gp16"
  sess "$tag" "global-temp-preserve" "create global temporary table gp16(id int) on commit preserve rows"
  sess "$tag" "global-temp-no-oncommit" "create global temporary table gp16(id int)"
  sess "$tag" "global-temp-persists" "create global temporary table gp16(id int) on commit delete rows;show tables like 'gp16'"
  exists "$tag" "gp16"
  sess "$tag" "drop-global-temp-keyword" "create global temporary table gp16(id int) on commit delete rows;drop global temporary table gp16;show tables like 'gp16'"
  sess "$tag" "drop-temp-on-globaltemp" "create global temporary table gp16(id int) on commit delete rows;drop temporary table gp16;show tables like 'gp16'"
  sess "$tag" "drop-plain-on-globaltemp" "create global temporary table gp16(id int) on commit delete rows;drop table gp16;show tables like 'gp16'"
  sess "$tag" "alter-on-globaltemp" "create global temporary table gp16(id int) on commit delete rows;alter table gp16 add column x int;describe gp16"
  sess "$tag" "truncate-globaltemp" "create global temporary table gp16(id int) on commit delete rows;insert into gp16 values(1);truncate table gp16;select count(*) c from gp16"
  sess "$tag" "flashback-recover" "create table p16_a(id int);drop table p16_a;flashback table p16_a to p16_b;show warnings"
  sess "$tag" "tidb-ctas" "create table p16_c1 select 1 a"
  sess "$tag" "tidb-ctas-as" "create table p16_c1 as select 1 a"
  sess "$tag" "tidb-ctas-ignore" "create table p16_c1 ignore select 1 a"
  sess "$tag" "tidb-ctas-defs" "create table p16_c1(id int) select 1 a"
  sess "$tag" "mysql-global-temp" "create global temporary table gp16(id int) on commit delete rows"
  exists "$tag" "gp16"
  sess "$tag" "like-on-localtemp" "create temporary table tp16(id int);create table p16_l7 like tp16"
  sess "$tag" "temp-like-from-temp" "create temporary table tp16(id int);create temporary table tp16l like tp16;describe tp16l"
  sess "$tag" "alter-localtemp-tidb" "create temporary table tp16(id int);alter table tp16 add column x int"
  sess "$tag" "rename-localtemp" "create temporary table tp16(id int);rename table tp16 to tp16t"
  sess "$tag" "alter-rename-localtemp" "create temporary table tp16(id int);alter table tp16 rename to tp16t;select count(*) c from tp16t"

  cleanup "$tag"
done
say "DONE"
