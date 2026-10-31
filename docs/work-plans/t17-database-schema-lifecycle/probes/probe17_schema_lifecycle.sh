#!/usr/bin/env bash
# T17 native probe — DATABASE/SCHEMA lifecycle (CREATE|ALTER|DROP), synonym
# normalization, name-less ALTER binding, IF [NOT] EXISTS conditional arms,
# default charset/collation derivation, versioned ENCRYPTION, MySQL-only
# READ ONLY, TiDB placement binding, system-schema protection, cascade
# scope, and drop+recreate identity. Executes fixed synthetic fixture SQL on
# the four frozen anchors and records per-label rc / stderr / SHOW WARNINGS /
# information_schema.SCHEMATA facts. Every object is a disposable p17* fixture;
# user SQL is never executed.
#
# Statements whose truth depends on one session (name-less ALTER, SHOW
# WARNINGS) run inside ONE mysql invocation per label.
#
# WARNING: the 'sys-drop-57' label runs `DROP DATABASE mysql` on the 5.7
# anchor. 5.7 has NO native protection — the schema is really dropped and the
# fixture container must be rebuilt afterwards. Run that anchor's probe only
# on a disposable container.
#
# Usage: probe17_schema_lifecycle.sh [anchor ...]  (mysql57 mysql80 mysql84 tidb85)
set -u
out="${OUT:-$(dirname "$0")/p17_schema_matrix.log}"
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
clinodb() { # same, without a default database (schema-level statements)
  local tag="$1"; shift
  case "$tag" in
    mysql57) docker exec deltascope-ddl-golden-mysql57 mysql -uroot -proot "$@" ;;
    mysql80) docker exec deltascope-ddl-golden-mysql80 mysql -uroot -proot "$@" ;;
    mysql84) docker exec deltascope-ddl-golden-mysql84 mysql -uroot -proot "$@" ;;
    tidb85) docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot "$@" ;;
  esac
}
q() { cli "$1" -N -e "$2"; }
qn() { clinodb "$1" -N -e "$2"; }
say() { printf '%s\n' "$*" | tee -a "$out"; }
cleanup() {
  clinodb "$1" -e "drop database if exists p17a; drop database if exists p17b; drop database if exists p17casc; drop database if exists p17enc; drop database if exists p17p; drop database if exists p17rc; drop database if exists p17gone" >/dev/null 2>&1
}
sess() { # $1 tag, $2 label, $3 sql batch — ONE clinodb session after cleanup
  local tag="$1" label="$2" sql="$3" rc err
  cleanup "$tag"
  say "=== $tag :: $label"
  say "SQL> $sql"
  err=$(clinodb "$tag" -e "$sql" 2>&1); rc=$?
  say "rc=$rc"
  [ -n "$err" ] && say "out> $err"
}
sessdb() { # same as sess but WITH golden as the session default database
  local tag="$1" label="$2" sql="$3" rc err
  cleanup "$tag"
  say "=== $tag :: $label"
  say "SQL> $sql   [default-db=golden]"
  err=$(cli "$tag" -e "$sql" 2>&1); rc=$?
  say "rc=$rc"
  [ -n "$err" ] && say "out> $err"
}
schemata() { say "schemata[$1::$2]> $(qn "$1" "select schema_name,default_character_set_name,default_collation_name from information_schema.schemata where schema_name='$2'" 2>&1 | tr '\n' '|')"; }
enc() { say "enc[$1::$2]> $(qn "$1" "select default_encryption from information_schema.schemata where schema_name='$2'" 2>&1)"; }
placement() { say "placement[$1::$2]> $(qn "$1" "select tidb_placement_policy_name from information_schema.schemata where schema_name='$2'" 2>&1)"; }

anchors="${*:-mysql57 mysql80 mysql84 tidb85}"
for tag in $anchors; do
  say "################################################ $tag"
  clinodb "$tag" -e "create database if not exists golden" >/dev/null 2>&1
  say "version> $(qn "$tag" "select version()" 2>&1)"

  sess "$tag" "charset-lifecycle" \
    "create database p17a character set utf8mb4; alter database p17a character set latin1; drop database p17a"
  sess "$tag" "charset-lifecycle-steps" \
    "create database p17a character set utf8mb4"
  schemata "$tag" "p17a"
  sess "$tag" "charset-alter-only" \
    "create database p17a character set utf8mb4; alter database p17a character set latin1"
  schemata "$tag" "p17a"
  sess "$tag" "collate-only-derives-charset" \
    "create database p17a; alter database p17a collate latin1_bin"
  schemata "$tag" "p17a"
  sess "$tag" "synonym-schema-lifecycle" \
    "create schema p17a character set utf8mb4; alter schema p17a collate utf8mb4_bin; drop schema p17a"
  sess "$tag" "nameless-alter-no-default-db" \
    "create database p17a; alter database character set gbk"
  sessdb "$tag" "nameless-alter-binds-default-db" \
    "alter database character set gbk"
  schemata "$tag" "golden"
  sessdb "$tag" "nameless-alter-restore" \
    "alter database character set utf8mb4"
  sess "$tag" "missing-alter-refusal" \
    "alter database p17gone character set utf8mb4"
  sess "$tag" "missing-drop-refusal" \
    "drop database p17gone"
  sess "$tag" "ifexists-noop-missing" \
    "drop database if exists p17gone; show warnings"
  sess "$tag" "duplicate-create-refusal" \
    "create database p17a; create database p17a"
  sess "$tag" "ifnotexists-noop" \
    "create database p17a character set latin1; create database if not exists p17a character set utf8mb4; show warnings"
  schemata "$tag" "p17a"
  sess "$tag" "recreate-drops-stale-attrs" \
    "create database p17rc character set latin1; drop database p17rc; create database p17rc character set gbk"
  schemata "$tag" "p17rc"
  sess "$tag" "table-inherits-schema-default" \
    "create database p17a character set gbk; create table p17a.t (id int)"
  say "tablecoll[$tag]> $(qn "$tag" "select table_collation from information_schema.tables where table_schema='p17a' and table_name='t'" 2>&1)"
  sess "$tag" "cascade-drop-kills-tables" \
    "create database p17casc; create table p17casc.t (id int); drop database p17casc"
  say "tables-after[$tag]> $(qn "$tag" "select count(*) from information_schema.tables where table_schema='p17casc'" 2>&1)"
  sess "$tag" "orphan-table-ops-on-missing-schema" \
    "drop table p17casc.t"
  sess "$tag" "orphan-create-on-missing-schema" \
    "create table p17casc.t (id int)"
  sess "$tag" "encryption-create" \
    "create database p17enc default encryption='Y'; show warnings"
  enc "$tag" "p17enc"
  sess "$tag" "encryption-alter" \
    "create database p17enc; alter database p17enc default encryption='N'; show warnings"
  enc "$tag" "p17enc"
  sess "$tag" "read-only-alter" \
    "create database p17a; alter database p17a read only=1"
  sess "$tag" "read-only-create-refusal" \
    "create database p17b read only=1"
  sess "$tag" "upgrade-data-directory-name" \
    "alter database p17a upgrade data directory name"
  sess "$tag" "placement-create" \
    "create database p17p placement policy=p1"
  placement "$tag" "p17p"
  sess "$tag" "placement-alter-missing-policy" \
    "create database p17p; alter database p17p placement policy=p1"
  placement "$tag" "p17p"
  sess "$tag" "sys-mysql-alter" \
    "alter database mysql character set utf8mb4"
  schemata "$tag" "mysql"
  sess "$tag" "sys-information-schema-alter" \
    "alter database information_schema character set utf8mb4"
  if [ "$tag" = mysql57 ]; then
    say "=== $tag :: sys-mysql-drop"
    say "SKIPPED (destructive on 5.7: no native protection — earlier live probe dropped mysql at rc=0 and the fixture container had to be rebuilt; fact already proven interactively 2026)"
  else
    sess "$tag" "sys-mysql-drop" \
      "drop database mysql"
    say "mysql-after[$tag]> $(qn "$tag" "select count(*) from information_schema.schemata where schema_name='mysql'" 2>&1)"
  fi
  sess "$tag" "quoted-identifier" \
    "create database \`Weird-Name\`; drop database \`Weird-Name\`"
  cleanup "$tag"
done
say "ALL DONE"
