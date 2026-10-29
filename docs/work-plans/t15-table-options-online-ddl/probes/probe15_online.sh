#!/usr/bin/env bash
# T15 native probe — online-DDL modifier + table-option matrix.
# Executes fixed synthetic fixture SQL on the four frozen anchors and records
# per-statement rc / stderr / SHOW WARNINGS / catalog facts. Every object is a
# disposable t15*/v15* fixture; user SQL is never executed.
# Usage: probes/probe15_online.sh [anchor ...]   (anchors: mysql57 mysql80 mysql84 tidb85)
set -u
out="${OUT:-$(dirname "$0")/p15_online_matrix.log}"
: >"$out"

say() { printf '%s\n' "$*" | tee -a "$out"; }
run() {
  # $1 anchor tag, $2 label, $3 sql. Executes inside the anchor's client.
  local tag="$1" label="$2" sql="$3"
  say "=== $tag :: $label"
  say "SQL> $sql"
  local rc err
  case "$tag" in
    mysql57) err=$(docker exec deltascope-ddl-golden-mysql57 mysql -uroot -proot --default-character-set=utf8mb4 golden -e "$sql" 2>&1); rc=$? ;;
    mysql80) err=$(docker exec deltascope-ddl-golden-mysql80 mysql -uroot -proot --default-character-set=utf8mb4 golden -e "$sql" 2>&1); rc=$? ;;
    mysql84) err=$(docker exec deltascope-ddl-golden-mysql84 mysql -uroot -proot --default-character-set=utf8mb4 golden -e "$sql" 2>&1); rc=$? ;;
    tidb85) err=$(docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot --default-character-set=utf8mb4 golden -e "$sql" 2>&1); rc=$? ;;
  esac
  say "rc=$rc"
  if [ -n "$err" ]; then say "stderr/stdout> $err"; fi
}
warn() {
  local tag="$1"
  case "$tag" in
    mysql57) docker exec deltascope-ddl-golden-mysql57 mysql -uroot -proot --default-character-set=utf8mb4 golden -e "SHOW WARNINGS" 2>&1 | tee -a "$out" >/dev/null ;;
    mysql80) docker exec deltascope-ddl-golden-mysql80 mysql -uroot -proot --default-character-set=utf8mb4 golden -e "SHOW WARNINGS" 2>&1 | tee -a "$out" >/dev/null ;;
    mysql84) docker exec deltascope-ddl-golden-mysql84 mysql -uroot -proot --default-character-set=utf8mb4 golden -e "SHOW WARNINGS" 2>&1 | tee -a "$out" >/dev/null ;;
    tidb85) docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot --default-character-set=utf8mb4 golden -e "SHOW WARNINGS" 2>&1 | tee -a "$out" >/dev/null ;;
  esac
  say "warn> $(case "$tag" in
    mysql57) docker exec deltascope-ddl-golden-mysql57 mysql -uroot -proot -N --default-character-set=utf8mb4 golden -e "SHOW WARNINGS" 2>&1 ;;
    mysql80) docker exec deltascope-ddl-golden-mysql80 mysql -uroot -proot -N --default-character-set=utf8mb4 golden -e "SHOW WARNINGS" 2>&1 ;;
    mysql84) docker exec deltascope-ddl-golden-mysql84 mysql -uroot -proot -N --default-character-set=utf8mb4 golden -e "SHOW WARNINGS" 2>&1 ;;
    tidb85) docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot -N --default-character-set=utf8mb4 golden -e "SHOW WARNINGS" 2>&1 ;;
  esac)"
}
cat_row() {
  local tag="$1" t="$2"
  case "$tag" in
    mysql57) r=$(docker exec deltascope-ddl-golden-mysql57 mysql -uroot -proot -N --default-character-set=utf8mb4 -e "select engine,row_format,table_collation,table_comment,auto_increment,table_rows,create_options from information_schema.tables where table_schema='golden' and table_name='$t'" 2>&1) ;;
    mysql80) r=$(docker exec deltascope-ddl-golden-mysql80 mysql -uroot -proot -N --default-character-set=utf8mb4 -e "select engine,row_format,table_collation,table_comment,auto_increment,table_rows,create_options from information_schema.tables where table_schema='golden' and table_name='$t'" 2>&1) ;;
    mysql84) r=$(docker exec deltascope-ddl-golden-mysql84 mysql -uroot -proot -N --default-character-set=utf8mb4 -e "select engine,row_format,table_collation,table_comment,auto_increment,table_rows,create_options from information_schema.tables where table_schema='golden' and table_name='$t'" 2>&1) ;;
    tidb85) r=$(docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot -N --default-character-set=utf8mb4 -e "select engine,row_format,table_collation,table_comment,auto_increment,table_rows,create_options from information_schema.tables where table_schema='golden' and table_name='$t'" 2>&1) ;;
  esac
  say "catalog[$t]> $r"
}
col_row() {
  local tag="$1" t="$2"
  case "$tag" in
    mysql57) r=$(docker exec deltascope-ddl-golden-mysql57 mysql -uroot -proot -N --default-character-set=utf8mb4 -e "select column_name,column_type,character_set_name,collation_name from information_schema.columns where table_schema='golden' and table_name='$t' order by ordinal_position" 2>&1) ;;
    mysql80) r=$(docker exec deltascope-ddl-golden-mysql80 mysql -uroot -proot -N --default-character-set=utf8mb4 -e "select column_name,column_type,character_set_name,collation_name from information_schema.columns where table_schema='golden' and table_name='$t' order by ordinal_position" 2>&1) ;;
    mysql84) r=$(docker exec deltascope-ddl-golden-mysql84 mysql -uroot -proot -N --default-character-set=utf8mb4 -e "select column_name,column_type,character_set_name,collation_name from information_schema.columns where table_schema='golden' and table_name='$t' order by ordinal_position" 2>&1) ;;
    tidb85) r=$(docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot -N --default-character-set=utf8mb4 -e "select column_name,column_type,character_set_name,collation_name from information_schema.columns where table_schema='golden' and table_name='$t' order by ordinal_position" 2>&1) ;;
  esac
  say "columns[$t]> $(printf '%s' "$r" | tr '\n' '|')"
}

anchors=("${@:-mysql57 mysql80 mysql84 tidb85}")
for tag in $anchors; do
  say "###### anchor $tag"
  run "$tag" "version" "select version()"
  run "$tag" "ensure-db" "create database if not exists golden"
done

for tag in $anchors; do
  say "########## $tag — online-DDL modifier matrix"
  run "$tag" "fixture" "drop table if exists t15ol; create table t15ol (id int primary key, s varchar(20)) engine=InnoDB"
  run "$tag" "seed-rows" "insert into t15ol values (1,'a'),(2,'b')"
  cat_row "$tag" "t15ol"
  for cell in \
    "add-default|ALTER TABLE t15ol ADD COLUMN d0 INT" \
    "add-instant|ALTER TABLE t15ol ADD COLUMN d1 INT, ALGORITHM=INSTANT" \
    "add-inplace|ALTER TABLE t15ol ADD COLUMN d2 INT, ALGORITHM=INPLACE" \
    "add-copy|ALTER TABLE t15ol ADD COLUMN d3 INT, ALGORITHM=COPY" \
    "add-algdefault|ALTER TABLE t15ol ADD COLUMN d4 INT, ALGORITHM=DEFAULT" \
    "add-locknone|ALTER TABLE t15ol ADD COLUMN d5 INT, LOCK=NONE" \
    "add-lockshared|ALTER TABLE t15ol ADD COLUMN d6 INT, LOCK=SHARED" \
    "add-lockexclusive|ALTER TABLE t15ol ADD COLUMN d7 INT, LOCK=EXCLUSIVE" \
    "add-lockdefault|ALTER TABLE t15ol ADD COLUMN d8 INT, LOCK=DEFAULT" \
    "seed-copy-excl|ALTER TABLE t15ol ADD COLUMN d9 INT, ALGORITHM=COPY, LOCK=EXCLUSIVE" \
    "copy-locknone|ALTER TABLE t15ol ADD COLUMN d10 INT, ALGORITHM=COPY, LOCK=NONE" \
    "copy-lockshared|ALTER TABLE t15ol ADD COLUMN d11 INT, ALGORITHM=COPY, LOCK=SHARED" \
    "inplace-locknone|ALTER TABLE t15ol ADD COLUMN d12 INT, ALGORITHM=INPLACE, LOCK=NONE" \
    "instant-locknone|ALTER TABLE t15ol ADD COLUMN d13 INT, ALGORITHM=INSTANT, LOCK=NONE" \
    "instant-lockexclusive|ALTER TABLE t15ol ADD COLUMN d14 INT, ALGORITHM=INSTANT, LOCK=EXCLUSIVE" \
    "alg-alone|ALTER TABLE t15ol ALGORITHM=COPY" \
    "lock-alone|ALTER TABLE t15ol LOCK=EXCLUSIVE" \
    "alg-dup|ALTER TABLE t15ol ADD COLUMN d15 INT, ALGORITHM=COPY, ALGORITHM=INSTANT" \
    "lock-dup|ALTER TABLE t15ol ADD COLUMN d16 INT, LOCK=NONE, LOCK=EXCLUSIVE" \
    "idx-instant|ALTER TABLE t15ol ADD INDEX ix1(id), ALGORITHM=INSTANT" \
    "idx-inplace-locknone|ALTER TABLE t15ol ADD INDEX ix2(s), ALGORITHM=INPLACE, LOCK=NONE" \
    "idx-copy|ALTER TABLE t15ol ADD INDEX ix3(id), ALGORITHM=COPY" \
    "modify-inplace|ALTER TABLE t15ol MODIFY COLUMN s VARCHAR(30), ALGORITHM=INPLACE" \
    "modify-instant|ALTER TABLE t15ol MODIFY COLUMN s VARCHAR(40), ALGORITHM=INSTANT" \
    "rename-instant|ALTER TABLE t15ol RENAME COLUMN s TO s2, ALGORITHM=INSTANT" \
    "dropcol-instant|ALTER TABLE t15ol DROP COLUMN d4, ALGORITHM=INSTANT" \
    "dropcol-copy|ALTER TABLE t15ol DROP COLUMN d5, ALGORITHM=COPY" \
    "multi-alg|ALTER TABLE t15ol ADD COLUMN d17 INT, DROP COLUMN d0, ALGORITHM=INPLACE" \
    "engine-inplace|ALTER TABLE t15ol ENGINE=InnoDB, ALGORITHM=INPLACE" \
    "engine-copy|ALTER TABLE t15ol ENGINE=InnoDB, ALGORITHM=COPY" \
  ; do
    name="${cell%%|*}"; sql="${cell#*|}"
    run "$tag" "$name" "$sql"
    warn "$tag"
  done
  col_row "$tag" "t15ol"
  cat_row "$tag" "t15ol"
  run "$tag" "cleanup" "drop table if exists t15ol"
done

for tag in $anchors; do
  say "########## $tag — alter-table-option matrix"
  run "$tag" "opt-fixture" "drop table if exists t15opt; create table t15opt (id int primary key, s varchar(20) charset utf8mb4, n int) engine=InnoDB"
  run "$tag" "opt-rows" "insert into t15opt values (2,'b',1),(1,'a',2)"
  cat_row "$tag" "t15opt"
  col_row "$tag" "t15opt"
  for cell in \
    "orderby|ALTER TABLE t15opt ORDER BY id" \
    "orderby-cols|ALTER TABLE t15opt ORDER BY id DESC, s" \
    "force|ALTER TABLE t15opt FORCE" \
    "disable-keys|ALTER TABLE t15opt DISABLE KEYS" \
    "enable-keys|ALTER TABLE t15opt ENABLE KEYS" \
    "engine-myisam|ALTER TABLE t15opt ENGINE=MyISAM" \
    "engine-back|ALTER TABLE t15opt ENGINE=InnoDB" \
    "rowformat|ALTER TABLE t15opt ROW_FORMAT=DYNAMIC" \
    "rowformat-compact|ALTER TABLE t15opt ROW_FORMAT=COMPACT" \
    "autoinc|ALTER TABLE t15opt AUTO_INCREMENT=100" \
    "comment|ALTER TABLE t15opt COMMENT='hello'" \
    "default-charset|ALTER TABLE t15opt DEFAULT CHARACTER SET utf8mb4" \
    "default-charset-collate|ALTER TABLE t15opt DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin" \
    "convert-utf8mb4|ALTER TABLE t15opt CONVERT TO CHARACTER SET utf8mb4" \
    "convert-latin1|ALTER TABLE t15opt CONVERT TO CHARACTER SET latin1" \
    "convert-collate|ALTER TABLE t15opt CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_bin" \
    "kbs|ALTER TABLE t15opt KEY_BLOCK_SIZE=8" \
    "compression|ALTER TABLE t15opt COMPRESSION='zlib'" \
    "encryption|ALTER TABLE t15opt ENCRYPTION='Y'" \
    "stats-persistent|ALTER TABLE t15opt STATS_PERSISTENT=1" \
    "stats-auto-recalc|ALTER TABLE t15opt STATS_AUTO_RECALC=0" \
    "stats-sample-pages|ALTER TABLE t15opt STATS_SAMPLE_PAGES=25" \
    "tablespace|ALTER TABLE t15opt TABLESPACE=innodb_file_per_table" \
    "pack-multi|ALTER TABLE t15opt ENGINE=InnoDB, COMMENT='two', AUTO_INCREMENT=200" \
  ; do
    name="${cell%%|*}"; sql="${cell#*|}"
    run "$tag" "$name" "$sql"
    warn "$tag"
    cat_row "$tag" "t15opt"
  done
  col_row "$tag" "t15opt"
  run "$tag" "cleanup" "drop table if exists t15opt"
done

for tag in $anchors; do
  say "########## $tag — myisam option matrix"
  run "$tag" "my-fixture" "drop table if exists t15my; create table t15my (id int primary key, s varchar(20)) engine=MyISAM"
  for cell in \
    "myisam-disable|ALTER TABLE t15my DISABLE KEYS" \
    "myisam-enable|ALTER TABLE t15my ENABLE KEYS" \
    "myisam-orderby|ALTER TABLE t15my ORDER BY id" \
    "myisam-rowformat|ALTER TABLE t15my ROW_FORMAT=FIXED" \
    "myisam-kbs|ALTER TABLE t15my KEY_BLOCK_SIZE=8" \
    "myisam-delay|ALTER TABLE t15my DELAY_KEY_WRITE=1" \
    "myisam-pack|ALTER TABLE t15my PACK_KEYS=1" \
  ; do
    name="${cell%%|*}"; sql="${cell#*|}"
    run "$tag" "$name" "$sql"
    warn "$tag"
    cat_row "$tag" "t15my"
  done
  run "$tag" "cleanup" "drop table if exists t15my"
done

for tag in $anchors; do
  say "########## $tag — create-table-option matrix"
  for cell in \
    "ct-base|drop table if exists t15c; create table t15c (id int) engine=InnoDB" \
    "ct-kbs|drop table if exists t15k; create table t15k (id int) engine=InnoDB key_block_size=8" \
    "ct-compression|drop table if exists t15z; create table t15z (id int) engine=InnoDB compression='zlib'" \
    "ct-encryption|drop table if exists t15e; create table t15e (id int) engine=InnoDB encryption='Y'" \
    "ct-stats-persist|drop table if exists t15sp; create table t15sp (id int) engine=InnoDB stats_persistent=1" \
    "ct-stats-recalc|drop table if exists t15sa; create table t15sa (id int) engine=InnoDB stats_auto_recalc=0" \
    "ct-stats-pages|drop table if exists t15sg; create table t15sg (id int) engine=InnoDB stats_sample_pages=25" \
    "ct-tablespace|drop table if exists t15ts; create table t15ts (id int) engine=InnoDB tablespace=innodb_file_per_table" \
    "ct-myisam-kbs|drop table if exists t15mk; create table t15mk (id int) engine=MyISAM key_block_size=8" \
    "ct-myisam-compression|drop table if exists t15mz; create table t15mz (id int) engine=MyISAM compression='zlib'" \
    "ct-myisam-stats|drop table if exists t15ms; create table t15ms (id int) engine=MyISAM stats_persistent=1" \
  ; do
    name="${cell%%|*}"; sql="${cell#*|}"
    run "$tag" "$name" "$sql"
    warn "$tag"
  done
  for t in t15c t15k t15z t15e t15sp t15sa t15sg t15ts t15mk t15mz t15ms; do
    cat_row "$tag" "$t"
  done
  run "$tag" "cleanup" "drop table if exists t15c,t15k,t15z,t15e,t15sp,t15sa,t15sg,t15ts,t15mk,t15mz,t15ms"
done
say "###### probe complete"
