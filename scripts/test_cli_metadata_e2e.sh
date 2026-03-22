#!/usr/bin/env bash
# input: docker compose services, fixture SQL, and deltascope CLI invocations against local MySQL/TiDB targets
# output: repeatable metadata-aware CLI e2e execution with JSON assertion helpers and deterministic cleanup
# pos: shell-based end-to-end harness for live metadata CLI validation
# note: if this file changes, update this header and module README.md.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/docker/cli-e2e-compose.yaml"
MYSQL_CONTAINER="deltascope-cli-e2e-mysql"
TIDB_CONTAINER="deltascope-cli-e2e-tidb"
CLIENT_CONTAINER="deltascope-cli-e2e-mysql-client"
TMP_DIR=""
CLI_BIN=""

mode="${1:-all}"

log() {
  printf '[cli-metadata-e2e] %s\n' "$*"
}

fail() {
  printf '[cli-metadata-e2e][FAIL] %s\n' "$*" >&2
  exit 1
}

compose() {
  docker compose -f "${COMPOSE_FILE}" "$@"
}

cleanup() {
  compose down -v --remove-orphans >/dev/null 2>&1 || true
  if [[ -n "${TMP_DIR}" && -d "${TMP_DIR}" ]]; then
    rm -rf "${TMP_DIR}"
  fi
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

wait_for_health() {
  local container="$1"
  local retries="${2:-60}"
  local delay="${3:-2}"
  local status=""

  for ((i = 1; i <= retries; i++)); do
    status="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${container}" 2>/dev/null || true)"
    if [[ "${status}" == "healthy" || "${status}" == "running" ]]; then
      return 0
    fi
    sleep "${delay}"
  done

  fail "container ${container} did not become ready (last status: ${status:-unknown})"
}

seed_tidb() {
  log "seeding TiDB fixtures"
  docker exec -i "${CLIENT_CONTAINER}" sh -lc 'mysql -h tidb -P 4000 -uroot' < "${ROOT_DIR}/docker/tidb/init.sql"
}

build_cli() {
  TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/deltascope-cli-metadata-e2e.XXXXXX")"
  CLI_BIN="${TMP_DIR}/deltascope"
  (
    cd "${ROOT_DIR}"
    go build -o "${CLI_BIN}" ./cmd/deltascope
  )
}

run_cli_capture() {
  local stdout_file="$1"
  local stderr_file="$2"
  shift 2

  set +e
  (
    cd "${ROOT_DIR}"
    "${CLI_BIN}" "$@"
  ) >"${stdout_file}" 2>"${stderr_file}"
  local exit_code=$?
  set -e
  return "${exit_code}"
}

assert_exit_code() {
  local actual="$1"
  local expected="$2"
  [[ "${actual}" == "${expected}" ]] || fail "expected exit code ${expected}, got ${actual}"
}

assert_stderr_contains() {
  local stderr_file="$1"
  local expected="$2"
  grep -Fq -- "${expected}" "${stderr_file}" || fail "expected stderr to contain ${expected}"
}

assert_json_field_eq() {
  local json_file="$1"
  local field_path="$2"
  local expected="$3"

  python3 - "$json_file" "$field_path" "$expected" <<'PY'
import json
import sys

path = sys.argv[2].split(".")
expected = sys.argv[3]
with open(sys.argv[1], "r", encoding="utf-8") as handle:
    data = json.load(handle)
value = data
for part in path:
    if isinstance(value, list):
        value = value[int(part)]
    else:
        value = value.get(part)
actual = "" if value is None else str(value)
if actual != expected:
    raise SystemExit(f"expected {sys.argv[2]}={expected}, got {actual}")
PY
}

assert_json_rule_present() {
  local json_file="$1"
  local rule_id="$2"

  python3 - "$json_file" "$rule_id" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    data = json.load(handle)
rule_id = sys.argv[2]
findings = list(data.get("global_findings", []))
for statement in data.get("statements", []):
    findings.extend(statement.get("findings", []))
if not any(item.get("rule_id") == rule_id for item in findings):
    raise SystemExit(f"expected rule_id {rule_id} in findings")
PY
}

assert_json_rule_absent() {
  local json_file="$1"
  local rule_id="$2"

  python3 - "$json_file" "$rule_id" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    data = json.load(handle)
rule_id = sys.argv[2]
findings = list(data.get("global_findings", []))
for statement in data.get("statements", []):
    findings.extend(statement.get("findings", []))
if any(item.get("rule_id") == rule_id for item in findings):
    raise SystemExit(f"did not expect rule_id {rule_id} in findings")
PY
}

start_mysql_stack() {
  log "starting MySQL fixtures"
  compose up -d mysql
  wait_for_health "${MYSQL_CONTAINER}"
}

start_tidb_stack() {
  log "starting TiDB fixtures"
  compose up -d tidb mysql-client
  wait_for_health "${TIDB_CONTAINER}" 90 2
  wait_for_health "${CLIENT_CONTAINER}" 30 1
  seed_tidb
}

run_mysql_suite() {
  local stdout_file
  local stderr_file
  local exit_code

  log "running MySQL metadata-aware CLI cases"

  stdout_file="$(mktemp "${TMP_DIR}/mysql-infer.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/mysql-infer.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "delete from orders where id = 1" --host 127.0.0.1 --port 3406 --user root --password root --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0
  assert_json_field_eq "${stdout_file}" "context.mode" "metadata-aware"
  assert_json_field_eq "${stdout_file}" "context.dialect" "mysql"
  assert_json_field_eq "${stdout_file}" "context.schema" "app"

  stdout_file="$(mktemp "${TMP_DIR}/mysql-ambiguous.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/mysql-ambiguous.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "delete from users where id = 1" --host 127.0.0.1 --port 3406 --user root --password root --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 2
  assert_stderr_contains "${stderr_file}" "ambiguous"
  assert_stderr_contains "${stderr_file}" "--schema"

  stdout_file="$(mktemp "${TMP_DIR}/mysql-explicit-schema.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/mysql-explicit-schema.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "delete from users where id = 1" --host 127.0.0.1 --port 3406 --user root --password root --schema archive --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0
  assert_json_field_eq "${stdout_file}" "context.schema" "archive"

  stdout_file="$(mktemp "${TMP_DIR}/mysql-qualified.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/mysql-qualified.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "delete from app.users where id = 1" --host 127.0.0.1 --port 3406 --user root --password root --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0
  assert_json_field_eq "${stdout_file}" "context.schema" "app"

  stdout_file="$(mktemp "${TMP_DIR}/mysql-dialect-mismatch.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/mysql-dialect-mismatch.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "delete from app.users where id = 1" --host 127.0.0.1 --port 3406 --user root --password root --dialect tidb --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 2
  assert_stderr_contains "${stderr_file}" "detected dialect"

  stdout_file="$(mktemp "${TMP_DIR}/mysql-exists.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/mysql-exists.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "create table app.users (id bigint unsigned not null auto_increment comment 'id', created_at timestamp not null default current_timestamp comment 'created', updated_at timestamp not null default current_timestamp on update current_timestamp comment 'updated', primary key (id)) comment='dup users'" --host 127.0.0.1 --port 3406 --user root --password root --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 1
  assert_json_rule_present "${stdout_file}" "ddl.table.exists.create.forbid"

  stdout_file="$(mktemp "${TMP_DIR}/mysql-compat.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/mysql-compat.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "alter table app.accounts modify column email varchar(16) not null default '' comment 'email'" --host 127.0.0.1 --port 3406 --user root --password root --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 1
  assert_json_rule_present "${stdout_file}" "ddl.alter.modify_column.compatibility.require"

  stdout_file="$(mktemp "${TMP_DIR}/mysql-partial-create.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/mysql-partial-create.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "create table huge_profiles (id bigint unsigned not null auto_increment comment 'id', c1 varchar(16383) not null default '' comment 'c1', c2 varchar(16383) not null default '' comment 'c2', created_at timestamp not null default current_timestamp comment 'created', updated_at timestamp not null default current_timestamp on update current_timestamp comment 'updated', primary key (id)) engine=InnoDB default charset=utf8mb4 row_format=dynamic comment='huge profiles'" --host 127.0.0.1 --port 3406 --user root --password root --schema app --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 1
  assert_json_field_eq "${stdout_file}" "context.schema" "app"
  assert_json_rule_present "${stdout_file}" "ddl.table.row_size.max_bytes.require"
  assert_json_rule_absent "${stdout_file}" "ddl.table.exists.create.forbid"
}

run_tidb_suite() {
  local stdout_file
  local stderr_file
  local exit_code

  log "running TiDB metadata-aware CLI cases"

  stdout_file="$(mktemp "${TMP_DIR}/tidb-infer.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/tidb-infer.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "delete from orders where id = 1" --host 127.0.0.1 --port 4400 --user root --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0
  assert_json_field_eq "${stdout_file}" "context.mode" "metadata-aware"
  assert_json_field_eq "${stdout_file}" "context.dialect" "tidb"
  assert_json_field_eq "${stdout_file}" "context.schema" "app"

  stdout_file="$(mktemp "${TMP_DIR}/tidb-ambiguous.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/tidb-ambiguous.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "delete from users where id = 1" --host 127.0.0.1 --port 4400 --user root --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 2
  assert_stderr_contains "${stderr_file}" "ambiguous"
  assert_stderr_contains "${stderr_file}" "--schema"

  stdout_file="$(mktemp "${TMP_DIR}/tidb-explicit-schema.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/tidb-explicit-schema.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "delete from users where id = 1" --host 127.0.0.1 --port 4400 --user root --schema archive --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0
  assert_json_field_eq "${stdout_file}" "context.schema" "archive"

  stdout_file="$(mktemp "${TMP_DIR}/tidb-qualified.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/tidb-qualified.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "delete from app.users where id = 1" --host 127.0.0.1 --port 4400 --user root --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0
  assert_json_field_eq "${stdout_file}" "context.schema" "app"

  stdout_file="$(mktemp "${TMP_DIR}/tidb-exists.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/tidb-exists.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "create table app.users (id bigint unsigned not null auto_increment comment 'id', created_at timestamp not null default current_timestamp comment 'created', updated_at timestamp not null default current_timestamp on update current_timestamp comment 'updated', primary key (id)) comment='dup users'" --host 127.0.0.1 --port 4400 --user root --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 1
  assert_json_rule_present "${stdout_file}" "ddl.table.exists.create.forbid"

  stdout_file="$(mktemp "${TMP_DIR}/tidb-instance-fact.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/tidb-instance-fact.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "create table huge_profiles (id bigint unsigned not null auto_increment comment 'id', c1 varchar(16383) not null default '' comment 'c1', c2 varchar(16383) not null default '' comment 'c2', created_at timestamp not null default current_timestamp comment 'created', updated_at timestamp not null default current_timestamp on update current_timestamp comment 'updated', primary key (id)) comment='huge profiles'" --host 127.0.0.1 --port 4400 --user root --schema app --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 1
  assert_json_field_eq "${stdout_file}" "context.schema" "app"
  assert_json_rule_present "${stdout_file}" "ddl.table.row_size.max_bytes.require"
  assert_json_rule_absent "${stdout_file}" "ddl.table.exists.create.forbid"
}

main() {
  require_cmd docker
  require_cmd python3
  require_cmd go
  trap cleanup EXIT
  build_cli

  case "${mode}" in
    mysql)
      start_mysql_stack
      run_mysql_suite
      ;;
    tidb)
      start_tidb_stack
      run_tidb_suite
      ;;
    all)
      start_mysql_stack
      run_mysql_suite
      cleanup
      start_tidb_stack
      run_tidb_suite
      ;;
    *)
      fail "usage: scripts/test_cli_metadata_e2e.sh [mysql|tidb|all]"
      ;;
  esac
}

main "$@"
