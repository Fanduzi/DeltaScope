#!/bin/bash
exec_mysql() { docker exec -e MYSQL_PWD=root deltascope-ddl-golden-$1 mysql -uroot --default-character-set=utf8mb4 -Nse "$2" golden 2>&1; }
exec_tidb() { docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot --default-character-set=utf8mb4 -Nse "$1" golden 2>&1; }
mp() { local ak=$1 lbl=$2 sql=$3 tbl="t_r1m_$(echo $2|tr 'A-Z' 'a-z')"; exec_mysql $ak "DROP TABLE IF EXISTS $tbl" >/dev/null; out=$(exec_mysql $ak "$sql"); echo "### $lbl $ak rc=$? out=$(echo "$out"|tr '\n' '|')"; exec_mysql $ak "DROP TABLE IF EXISTS $tbl" >/dev/null; }
tp() { local lbl=$1 sql=$2 tbl="t_r1t_$(echo $1|tr 'A-Z' 'a-z')"; exec_tidb "DROP TABLE IF EXISTS $tbl" >/dev/null; out=$(exec_tidb "$sql"); echo "### $lbl tidb rc=$? out=$(echo "$out"|tr '\n' '|')"; exec_tidb "DROP TABLE IF EXISTS $tbl" >/dev/null; }
for ak in mysql80 mysql84 mysql57; do
 mp $ak M_CREF_CREATE "CREATE TABLE t_r1m_m_cref_create (a INT, b INT DEFAULT (a+1))"
 mp $ak M_CREF_ALTER  "CREATE TABLE t_r1m_m_cref_alter (a INT, b INT); ALTER TABLE t_r1m_m_cref_alter ALTER COLUMN b SET DEFAULT (a+1)"
 mp $ak M_PLIT_CREATE "CREATE TABLE t_r1m_m_plit_create (v VARCHAR(8) DEFAULT ('abc'))"
 mp $ak M_GEN_FWD2    "CREATE TABLE t_r1m_m_gen_fwd2 (b INT AS (a+1) STORED, a INT)"
done
tp T_UUIDS_ALTER "CREATE TABLE t_r1t_t_uuids_alter (v VARCHAR(36)); ALTER TABLE t_r1t_t_uuids_alter ALTER COLUMN v SET DEFAULT (uuid_short())"
tp T_JARR_ALTER  "CREATE TABLE t_r1t_t_jarr_alter (v VARCHAR(64)); ALTER TABLE t_r1t_t_jarr_alter ALTER COLUMN v SET DEFAULT (json_array(1))"
tp T_NOW_ALTER   "CREATE TABLE t_r1t_t_now_alter (v VARCHAR(64)); ALTER TABLE t_r1t_t_now_alter ALTER COLUMN v SET DEFAULT (now())"
tp T_SUBQ_CREATE "CREATE TABLE t_r1t_t_subq_create (v INT DEFAULT ((select 1)))"
tp T_PAREN_LIT   "CREATE TABLE t_r1t_t_paren_lit (v VARCHAR(8) DEFAULT ('abc'))"
tp T_QUAL_REF    "CREATE TABLE t_r1t_t_qual_ref (a INT, b INT AS (t_r1t_t_qual_ref.a + 1) STORED)"
tp T_GEN_TXT     "CREATE TABLE t_r1t_t_gen_txt (a INT, b INT AS (missing_c + 1) VIRTUAL)"
