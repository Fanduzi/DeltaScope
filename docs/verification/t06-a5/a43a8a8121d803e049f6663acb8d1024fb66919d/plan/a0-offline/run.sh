#!/bin/bash
# T06-A5-PLAN offline probes: 2 types x 2 dialects x 4 controls
cd /tmp/ds-t06-a5
i=0
for type in CHAR VARCHAR; do
  tl=$(echo $type | tr 'A-Z' 'a-z')
  for dialect in mysql tidb; do
    for ctl in len7 len8 len9 alloff; do
      i=$((i+1))
      case $ctl in
        len7) n=7; pol=policy-$tl-limit8.yaml ;;
        len8) n=8; pol=policy-$tl-limit8.yaml ;;
        len9) n=9; pol=policy-$tl-limit8.yaml ;;
        alloff) n=9; pol=policy-all-off.yaml ;;
      esac
      sql="CREATE TABLE t (c $type($n));"
      name=$(printf "%02d-%s-%s-%s" $i $tl $dialect $ctl)
      echo "$sql" > runs/$name.sql
      cat > runs/$name.argv.txt <<ARGV
deltascope audit --dialect $dialect --sql "$sql" --config $pol --format json --fail-on blocker
ARGV
      ./deltascope audit --dialect $dialect --sql "$sql" --config $pol --format json --fail-on blocker > runs/$name.stdout.json 2> runs/$name.stderr.txt
      echo "$?" > runs/$name.rc.txt
    done
  done
done
echo "ran $i probes"
