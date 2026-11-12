#!/usr/bin/env bash
# T26-SERVER-ALTER-NATIVE-ORACLE-01 — per-case native evidence runner.
#
# Usage: run_case.sh <port> <outdir> <case_id> <user:pass> <srv_name> <target_sql_file> [pre_session_sql]
#
# Emits <outdir>/<case_id>.txt — J-format capture:
#   identity block; account-session transcript (verbatim -v echo; the
#   target statement; SHOW WARNINGS immediately after it; then
#   @@warning_count — SHOW WARNINGS is read first because issuing a
#   SELECT before it would replace the diagnostics area); a separate
#   single-statement rc-probe for a clean exit code + error line;
#   root pre/post catalog truth (full row incl. option columns —
#   sentinel values only).
# The ALTER statement is deterministic-idempotent (same options,
# same row) so the rc-probe second execution cannot diverge; every
# such second run is labelled in the transcript.
set -u
PORT="$1"; OUT="$2"; CID="$3"; ACCT="$4"; SRV="$5"; SQLF="$6"; PRE="${7:-}"
ROLEQ="${NA_ROLEQ-, current_role() as ACTIVE_ROLE}"
PVQ="${NA_PVQ-select version() as server_version, @@hostname as container, @@partial_revokes as partial_revokes}"
ROOT="mysql -h127.0.0.1 -P${PORT} -uroot -pprobe --connect-timeout=5"
USERPASS=${ACCT%%:*}
PASS=${ACCT##*:}
F="$OUT/${CID}.txt"
TGT="$(cat "$SQLF")"
{
  echo "case_id: $CID"
  echo "executed_at: $(date -u +%FT%TZ)"
  echo "port: $PORT"
  echo "account_identity: $USERPASS"
  echo "srv_name: $SRV"
  echo "literal_sql_target: $TGT"
  [ -n "$PRE" ] && echo "session_prefix: $PRE"
  echo "== root truth: server_version / config =="
  $ROOT -N -e "$PVQ" 2>&1
  echo "== root truth: pre catalog (target row + total) =="
  $ROOT -e "select * from mysql.servers where Server_name='${SRV}'; select count(*) as servers_total from mysql.servers" 2>&1
  echo "== account-view pre read (separate probe; a denied row-abort cannot block the target) =="
  mysql -h127.0.0.1 -P${PORT} -u"$USERPASS" -p"$PASS" --connect-timeout=5 -e "select Server_name from mysql.servers where Server_name='${SRV}'" 2>&1
  echo "account_view_pre_rc: $?"
  echo "== literal_argv (account session) =="
  echo "mysql -h127.0.0.1 -P${PORT} -u${USERPASS} -p<redacted> --connect-timeout=5 --force -v -e '<session script>'"
  echo "== session script (as executed) =="
  {
    [ -n "$PRE" ] && echo "$PRE"
    echo "select user() as SESSION_USER, current_user() as AUTH_USER${ROLEQ};"
    echo "show grants for current_user();"
    cat "$SQLF"
    echo "show warnings;"
    echo "select @@warning_count as warning_count;"
  } | tee /tmp/na-sess-$CID.sql | sed 's/^/sess-sql> /'
  echo "== account session output (stdout, -v verbatim) =="
  mysql -h127.0.0.1 -P${PORT} -u"$USERPASS" -p"$PASS" --connect-timeout=5 --force -v -e "$(cat /tmp/na-sess-$CID.sql)" 2>/tmp/na-err-$CID.txt
  BRC=$?
  echo "== account session stderr =="
  cat /tmp/na-err-$CID.txt
  echo "batch_exit_code: $BRC"
  echo "== account-view post read (separate probe) =="
  mysql -h127.0.0.1 -P${PORT} -u"$USERPASS" -p"$PASS" --connect-timeout=5 -e "select Server_name from mysql.servers where Server_name='${SRV}'" 2>&1
  echo "account_view_post_rc: $?"
  echo "== standalone rc-probe (identical target, second execution) =="
  echo "argv: mysql -h127.0.0.1 -P${PORT} -u${USERPASS} -p<redacted> --connect-timeout=5 -e '$TGT'"
  mysql -h127.0.0.1 -P${PORT} -u"$USERPASS" -p"$PASS" --connect-timeout=5 -e "$TGT" >/tmp/na-rc-out-$CID.txt 2>/tmp/na-rc-err-$CID.txt
  RC=$?
  echo "rc_probe_stdout:"; cat /tmp/na-rc-out-$CID.txt
  echo "rc_probe_stderr:"; cat /tmp/na-rc-err-$CID.txt
  echo "exit_code: $RC"
  echo "== root truth: post catalog (target row + total) =="
  $ROOT -e "select * from mysql.servers where Server_name='${SRV}'; select count(*) as servers_total from mysql.servers" 2>&1
} > "$F" 2>&1
rm -f /tmp/na-sess-$CID.sql /tmp/na-err-$CID.txt /tmp/na-rc-out-$CID.txt /tmp/na-rc-err-$CID.txt
echo "wrote $F"
