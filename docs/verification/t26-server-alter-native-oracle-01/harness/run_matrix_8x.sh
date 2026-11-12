#!/usr/bin/env bash
# T26-SERVER-ALTER-NATIVE-ORACLE-01 — 8.x case matrix driver.
# Usage: run_matrix_8x.sh <port> <outdir>
set -u
PORT="$1"; OUT="$2"; HERE="$(cd "$(dirname "$0")" && pwd)"; SQ=/tmp/na-sql
mkdir -p "$OUT" "$SQ"
R="$HERE/run_case.sh"

mk(){ printf '%s\n' "$2" > "$SQ/$1.sql"; }

# ---- C: 80-11 grant-scoped visibility ----
mk c11_a1 "alter server nao_a1 options (USER 'alt_sent_1');"
mk c11_a2 "alter server nao_miss options (USER 'alt_sent_2');"
mk c11_b1 "alter server nao_b1 options (USER 'sel_sent_1');"
mk c11_b2 "alter server nao_missb options (USER 'sel_sent_2');"
mk c11_d1 "alter server nao_d1 options (USER 'role_off');"
mk c11_d2 "alter server nao_d2 options (USER 'role_on');"
mk c11_e1 "alter server nao_e1 options (USER 'root_sent_1');"
mk c11_e2 "alter server nao_missr options (USER 'root_sent_2');"
$R "$PORT" "$OUT" c11_a1 na_alt:probe    nao_a1    "$SQ/c11_a1.sql"
$R "$PORT" "$OUT" c11_a2 na_alt:probe    nao_miss  "$SQ/c11_a2.sql"
$R "$PORT" "$OUT" c11_b1 na_altsel:probe nao_b1    "$SQ/c11_b1.sql"
$R "$PORT" "$OUT" c11_b2 na_altsel:probe nao_missb "$SQ/c11_b2.sql"
$R "$PORT" "$OUT" c11_d1 na_roleuser:probe nao_d1  "$SQ/c11_d1.sql" "set role none;"
$R "$PORT" "$OUT" c11_d2 na_roleuser:probe nao_d2  "$SQ/c11_d2.sql" "set role na_fedrole;"
$R "$PORT" "$OUT" c11_e1 root:probe      nao_e1    "$SQ/c11_e1.sql"
$R "$PORT" "$OUT" c11_e2 root:probe      nao_missr "$SQ/c11_e2.sql"

# ---- D: 80-7 privilege vs existence ordering ----
mk d7_c1 "alter server nao_r1 options (USER 'probe_u1');"
mk d7_c2 "alter server nao_missd options (USER 'probe_u2');"
mk d7_s1 "alter server nao_r2 options (USER 'probe_s1');"
mk d7_s2 "alter server nao_misss options (USER 'probe_s2');"
$R "$PORT" "$OUT" d7_c1 na_usage:probe   nao_r1    "$SQ/d7_c1.sql"
$R "$PORT" "$OUT" d7_c2 na_usage:probe   nao_missd "$SQ/d7_c2.sql"
$R "$PORT" "$OUT" d7_s1 na_selonly:probe nao_r2    "$SQ/d7_s1.sql"
$R "$PORT" "$OUT" d7_s2 na_selonly:probe nao_misss "$SQ/d7_s2.sql"

# ---- E: 80-1 absent-server + IF EXISTS grammar ----
mk e1_abs "alter server nao_e1miss options (USER 'probe');"
mk e1_ifx "alter server if exists nao_e1if options (USER 'probe');"
$R "$PORT" "$OUT" e1_abs root:probe nao_e1miss "$SQ/e1_abs.sql"
$R "$PORT" "$OUT" e1_ifx root:probe nao_e1if   "$SQ/e1_ifx.sql"

# ---- F: 80-9 merge vs replace (seeded nao_merge, alter USER only) ----
mk f9_mg "alter server nao_merge options (USER 'merge_new');"
$R "$PORT" "$OUT" f9_mg root:probe nao_merge "$SQ/f9_mg.sql"

# ---- G: 80-2/3/4 options ----
mk g2_dup "alter server nao_g2 options (USER 'first_val', USER 'second_val');"
mk g3_u  "alter server nao_g3u options (USER '');"
mk g3_h  "alter server nao_g3h options (HOST '');"
mk g3_p  "alter server nao_g3p options (PASSWORD '');"
mk g4_0  "alter server nao_p0 options (PORT 0);"
mk g4_1  "alter server nao_p1 options (PORT 1);"
mk g4_9  "alter server nao_p9 options (PORT 9999);"
mk g4_10 "alter server nao_p10 options (PORT 10000);"
mk g4_n1 "alter server nao_pn1 options (PORT -1);"
mk g4_q  "alter server nao_pq options (PORT '3306');"
mk g4_ov "alter server nao_pov options (PORT 18446744073709551616);"
$R "$PORT" "$OUT" g2_dup root:probe nao_g2  "$SQ/g2_dup.sql"
$R "$PORT" "$OUT" g3_u   root:probe nao_g3u "$SQ/g3_u.sql"
$R "$PORT" "$OUT" g3_h   root:probe nao_g3h "$SQ/g3_h.sql"
$R "$PORT" "$OUT" g3_p   root:probe nao_g3p "$SQ/g3_p.sql"
$R "$PORT" "$OUT" g4_0   root:probe nao_p0  "$SQ/g4_0.sql"
$R "$PORT" "$OUT" g4_1   root:probe nao_p1  "$SQ/g4_1.sql"
$R "$PORT" "$OUT" g4_9   root:probe nao_p9  "$SQ/g4_9.sql"
$R "$PORT" "$OUT" g4_10  root:probe nao_p10 "$SQ/g4_10.sql"
$R "$PORT" "$OUT" g4_n1  root:probe nao_pn1 "$SQ/g4_n1.sql"
$R "$PORT" "$OUT" g4_q   root:probe nao_pq  "$SQ/g4_q.sql"
$R "$PORT" "$OUT" g4_ov  root:probe nao_pov "$SQ/g4_ov.sql"

# ---- H: 80-5 case-fold/collation + 80-6 name-length ----
mk h5_l  "alter server mixedcase options (USER 'mc_lower');"
mk h5_u  "alter server MIXEDCASE options (USER 'mc_upper');"
mk h5_m  "alter server MixedCase options (USER 'mc_exact');"
mk h6_64 "alter server nao_h64_abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcd options (USER 'h64_new');"
mk h6_65 "alter server nao_h65_abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcde options (USER 'h65_new');"
$R "$PORT" "$OUT" h5_l  root:probe MixedCase "$SQ/h5_l.sql"
$R "$PORT" "$OUT" h5_u  root:probe MixedCase "$SQ/h5_u.sql"
$R "$PORT" "$OUT" h5_m  root:probe MixedCase "$SQ/h5_m.sql"
$R "$PORT" "$OUT" h6_64 root:probe nao_h64_abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcd "$SQ/h6_64.sql"
$R "$PORT" "$OUT" h6_65 root:probe nao_h65_abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcde "$SQ/h6_65.sql"

# ---- collation/schema capture (root, one file) ----
{
  echo "case_id: x_schema_collation"
  echo "== server_version =="; mysql -h127.0.0.1 -P"$PORT" -uroot -pprobe -N -e "select version()" 2>&1
  echo "== show create table mysql.servers =="; mysql -h127.0.0.1 -P"$PORT" -uroot -pprobe -e "show create table mysql.servers\G" 2>&1
  echo "== column collations =="; mysql -h127.0.0.1 -P"$PORT" -uroot -pprobe -e "select column_name, collation_name, character_set_name from information_schema.columns where table_schema='mysql' and table_name='servers' order by ordinal_position" 2>&1
  echo "== name-matching probe: does where Server_name='mixedcase' hit MixedCase? =="
  mysql -h127.0.0.1 -P"$PORT" -uroot -pprobe -e "select Server_name from mysql.servers where Server_name='mixedcase'; select Server_name from mysql.servers where Server_name='MIXEDCASE';" 2>&1
} > "$OUT/x_schema_collation.txt" 2>&1
echo "matrix done: $PORT -> $OUT"
