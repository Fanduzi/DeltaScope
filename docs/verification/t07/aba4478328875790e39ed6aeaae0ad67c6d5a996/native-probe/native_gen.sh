#!/bin/bash
# Bounded native confirmation: generated-column reference premises.
# Owned tables only: t_r1g_* ; COUNT0 -> exec -> catalog -> drop -> COUNT0.
exec_mysql() { # container sql
  docker exec -e MYSQL_PWD=root "$1" mysql -uroot --default-character-set=utf8mb4 -Nse "$2" golden 2>&1
}
exec_tidb() { # sql
  docker exec deltascope-ddl-golden-tidb85-client mysql -h tidb85 -P 4000 -uroot --default-character-set=utf8mb4 -Nse "$1" golden 2>&1
}
exec_anchor() { # anchor sql
  if [ "$1" = tidb85 ]; then exec_tidb "$2"; else exec_mysql "deltascope-ddl-golden-$1" "$2"; fi
}
count0() { # anchor tbl
  exec_anchor "$1" "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='golden' AND table_name='$2'"
}
run_gen() { # label anchor tbl sql
  local label="$1" ak="$2" tbl="$3" sql="$4"
  local pre post cols out rc
  pre=$(count0 "$ak" "$tbl")
  echo "### $label $ak table=$tbl pre=$pre"
  out=$(exec_anchor "$ak" "$sql"); rc=$?
  echo "rc=$rc out=$(echo "$out" | tr '\n' '|')"
  cols=$(exec_anchor "$ak" "SELECT GROUP_CONCAT(CONCAT(column_name,':',ordinal_position) ORDER BY ordinal_position) FROM information_schema.columns WHERE table_schema='golden' AND table_name='$tbl'")
  echo "cols=$cols"
  if [ -n "$cols" ]; then exec_anchor "$ak" "DROP TABLE $tbl" >/dev/null; fi
  post=$(count0 "$ak" "$tbl")
  echo "post=$post"
}
