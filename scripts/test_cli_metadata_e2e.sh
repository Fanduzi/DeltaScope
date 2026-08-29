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
MYSQL_PASSWORD="root"
export MYSQL_PASSWORD
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
  TMP_DIR=""
  CLI_BIN=""
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
    local has_healthcheck
    has_healthcheck="$(docker inspect --format '{{if .Config.Healthcheck.Test}}yes{{else}}no{{end}}' "${container}" 2>/dev/null || true)"

    if [[ "${has_healthcheck}" == "yes" ]]; then
      status="$(docker inspect --format '{{.State.Health.Status}}' "${container}" 2>/dev/null || true)"
      if [[ "${status}" == "healthy" ]]; then
        return 0
      fi
    else
      status="$(docker inspect --format '{{.State.Status}}' "${container}" 2>/dev/null || true)"
      if [[ "${status}" == "running" ]]; then
        return 0
      fi
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
    CGO_ENABLED=0 go build -o "${CLI_BIN}" ./cmd/deltascope
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

assert_findings_text_absent() {
  local json_file="$1"
  local text="$2"

  python3 - "$json_file" "$text" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as f:
    data = json.load(f)
text = sys.argv[2]
findings = list(data.get("global_findings", []))
for stmt in data.get("statements", []):
    findings.extend(stmt.get("findings", []))
block = json.dumps(findings, ensure_ascii=False)
if text in block:
    raise SystemExit(f"did not expect '{text}' in findings")
PY
}

assert_no_rule_prefix() {
  local json_file="$1"
  local prefix="$2"

  python3 - "$json_file" "$prefix" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as f:
    data = json.load(f)
prefix = sys.argv[2]
findings = list(data.get("global_findings", []))
for stmt in data.get("statements", []):
    findings.extend(stmt.get("findings", []))
for item in findings:
    rid = item.get("rule_id", "")
    if rid.startswith(prefix):
        raise SystemExit(f"did not expect rule_id with prefix '{prefix}', found: {rid}")
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
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "delete from orders where id = 1" --host 127.0.0.1 --port 3406 --user root --password-env MYSQL_PASSWORD --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0
  assert_json_field_eq "${stdout_file}" "context.mode" "metadata-aware"
  assert_json_field_eq "${stdout_file}" "context.dialect" "mysql"
  assert_json_field_eq "${stdout_file}" "context.schema" "app"

  stdout_file="$(mktemp "${TMP_DIR}/mysql-database-alias.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/mysql-database-alias.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "delete from users where id = 1" --host 127.0.0.1 --port 3406 --user root --password-env MYSQL_PASSWORD --database app --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0
  assert_json_field_eq "${stdout_file}" "context.schema" "app"
  assert_json_field_eq "${stdout_file}" "context.schema_source" "database"

  stdout_file="$(mktemp "${TMP_DIR}/mysql-ambiguous.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/mysql-ambiguous.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "delete from users where id = 1" --host 127.0.0.1 --port 3406 --user root --password-env MYSQL_PASSWORD --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 2
  assert_stderr_contains "${stderr_file}" "ambiguous"
  assert_stderr_contains "${stderr_file}" "--schema"

  stdout_file="$(mktemp "${TMP_DIR}/mysql-explicit-schema.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/mysql-explicit-schema.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "delete from users where id = 1" --host 127.0.0.1 --port 3406 --user root --password-env MYSQL_PASSWORD --schema archive --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0
  assert_json_field_eq "${stdout_file}" "context.schema" "archive"

  stdout_file="$(mktemp "${TMP_DIR}/mysql-qualified.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/mysql-qualified.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "delete from app.users where id = 1" --host 127.0.0.1 --port 3406 --user root --password-env MYSQL_PASSWORD --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0
  assert_json_field_eq "${stdout_file}" "context.schema" "app"

  stdout_file="$(mktemp "${TMP_DIR}/mysql-dialect-mismatch.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/mysql-dialect-mismatch.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "delete from app.users where id = 1" --host 127.0.0.1 --port 3406 --user root --password-env MYSQL_PASSWORD --dialect tidb --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 2
  assert_stderr_contains "${stderr_file}" "detected dialect"

  stdout_file="$(mktemp "${TMP_DIR}/mysql-exists.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/mysql-exists.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "create table app.users (id bigint unsigned not null auto_increment comment 'id', created_at timestamp not null default current_timestamp comment 'created', updated_at timestamp not null default current_timestamp on update current_timestamp comment 'updated', primary key (id)) comment='dup users'" --host 127.0.0.1 --port 3406 --user root --password-env MYSQL_PASSWORD --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 1
  assert_json_rule_present "${stdout_file}" "ddl.table.exists.create.forbid"

  stdout_file="$(mktemp "${TMP_DIR}/mysql-compat.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/mysql-compat.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "alter table app.accounts modify column email varchar(16) not null default '' comment 'email'" --host 127.0.0.1 --port 3406 --user root --password-env MYSQL_PASSWORD --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 1
  assert_json_rule_present "${stdout_file}" "ddl.alter.modify_column.compatibility.require"

  stdout_file="$(mktemp "${TMP_DIR}/mysql-nullability-same.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/mysql-nullability-same.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "alter table app.accounts modify column email varchar(320) not null" --host 127.0.0.1 --port 3406 --user root --password-env MYSQL_PASSWORD --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0
  assert_json_rule_absent "${stdout_file}" "ddl.alter.modify_column.explicit_nullability_change.forbid"
  assert_json_rule_absent "${stdout_file}" "ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory"

  stdout_file="$(mktemp "${TMP_DIR}/mysql-nullability-transition.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/mysql-nullability-transition.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "alter table app.accounts modify column email varchar(320) null" --host 127.0.0.1 --port 3406 --user root --password-env MYSQL_PASSWORD --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 1
  assert_json_rule_present "${stdout_file}" "ddl.alter.modify_column.explicit_nullability_change.forbid"
  assert_json_rule_absent "${stdout_file}" "ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory"

  stdout_file="$(mktemp "${TMP_DIR}/mysql-partial-create.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/mysql-partial-create.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "create table huge_profiles (id bigint unsigned not null auto_increment comment 'id', c1 varchar(16383) not null default '' comment 'c1', c2 varchar(16383) not null default '' comment 'c2', created_at timestamp not null default current_timestamp comment 'created', updated_at timestamp not null default current_timestamp on update current_timestamp comment 'updated', primary key (id)) engine=InnoDB default charset=utf8mb4 row_format=dynamic comment='huge profiles'" --host 127.0.0.1 --port 3406 --user root --password-env MYSQL_PASSWORD --schema app --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 1
  assert_json_field_eq "${stdout_file}" "context.schema" "app"
  assert_json_rule_present "${stdout_file}" "ddl.table.row_size.max_bytes.require"
  assert_json_rule_absent "${stdout_file}" "ddl.table.exists.create.forbid"

  # Hygiene: default-policy dialect — no PG-only leakage under MySQL dialect
  stdout_file="$(mktemp "${TMP_DIR}/mysql-hygiene.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/mysql-hygiene.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "CREATE TABLE smoke_users (id bigint unsigned NOT NULL AUTO_INCREMENT, name varchar(64) NOT NULL DEFAULT '', created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP, PRIMARY KEY (id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='smoke users';" --dialect mysql --format json --fail-on none; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "mysql-hygiene"
  assert_no_rule_prefix "${stdout_file}" "ddl.pg."
  for rule_id in \
    ddl.alter.set_data_type.forbid \
    ddl.alter.set_default.forbid \
    ddl.alter.drop_default.forbid \
    ddl.alter.set_not_null.forbid \
    ddl.alter.drop_not_null.forbid \
    ddl.alter.drop_expression.forbid \
    ddl.alter.set_generated.forbid \
    ddl.alter.drop_identity.forbid \
    ddl.alter.set_default.explicit_default_change.forbid \
    ddl.alter.drop_default.explicit_default_change.forbid \
    ddl.alter.set_not_null.explicit_nullability_change.forbid \
    ddl.alter.drop_not_null.explicit_nullability_change.forbid; do
    assert_json_rule_absent "${stdout_file}" "${rule_id}"
  done
  for token in \
    "VALIDATE CONSTRAINT" "NOT VALID" CONCURRENTLY \
    "DROP EXPRESSION" "SET GENERATED" "DROP IDENTITY" \
    "generated expression" rewrite; do
    assert_findings_text_absent "${stdout_file}" "${token}"
  done
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

  stdout_file="$(mktemp "${TMP_DIR}/tidb-database-alias.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/tidb-database-alias.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "delete from users where id = 1" --host 127.0.0.1 --port 4400 --user root --database app --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0
  assert_json_field_eq "${stdout_file}" "context.schema" "app"
  assert_json_field_eq "${stdout_file}" "context.schema_source" "database"

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

  stdout_file="$(mktemp "${TMP_DIR}/tidb-nullability-same.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/tidb-nullability-same.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "alter table app.accounts modify column email varchar(320) not null" --host 127.0.0.1 --port 4400 --user root --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0
  assert_json_rule_absent "${stdout_file}" "ddl.alter.modify_column.explicit_nullability_change.forbid"
  assert_json_rule_absent "${stdout_file}" "ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory"

  stdout_file="$(mktemp "${TMP_DIR}/tidb-nullability-transition.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/tidb-nullability-transition.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "alter table app.accounts modify column email varchar(320) null" --host 127.0.0.1 --port 4400 --user root --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 1
  assert_json_rule_present "${stdout_file}" "ddl.alter.modify_column.explicit_nullability_change.forbid"
  assert_json_rule_absent "${stdout_file}" "ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory"

  # Hygiene: default-policy dialect — no PG-only leakage under TiDB dialect
  stdout_file="$(mktemp "${TMP_DIR}/tidb-hygiene.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/tidb-hygiene.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "CREATE TABLE smoke_users (id bigint unsigned NOT NULL AUTO_INCREMENT, name varchar(64) NOT NULL DEFAULT '', created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP, PRIMARY KEY (id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='smoke users';" --dialect tidb --format json --fail-on none; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "tidb-hygiene"
  assert_no_rule_prefix "${stdout_file}" "ddl.pg."
  for rule_id in \
    ddl.alter.set_data_type.forbid \
    ddl.alter.set_default.forbid \
    ddl.alter.drop_default.forbid \
    ddl.alter.set_not_null.forbid \
    ddl.alter.drop_not_null.forbid \
    ddl.alter.drop_expression.forbid \
    ddl.alter.set_generated.forbid \
    ddl.alter.drop_identity.forbid \
    ddl.alter.set_default.explicit_default_change.forbid \
    ddl.alter.drop_default.explicit_default_change.forbid \
    ddl.alter.set_not_null.explicit_nullability_change.forbid \
    ddl.alter.drop_not_null.explicit_nullability_change.forbid; do
    assert_json_rule_absent "${stdout_file}" "${rule_id}"
  done
  for token in \
    "VALIDATE CONSTRAINT" "NOT VALID" CONCURRENTLY \
    "DROP EXPRESSION" "SET GENERATED" "DROP IDENTITY" \
    "generated expression" rewrite; do
    assert_findings_text_absent "${stdout_file}" "${token}"
  done
}

main() {
  require_cmd docker
  require_cmd python3
  require_cmd go
  trap cleanup EXIT

  case "${mode}" in
    mysql)
      build_cli
      start_mysql_stack
      run_mysql_suite
      ;;
    tidb)
      build_cli
      start_tidb_stack
      run_tidb_suite
      ;;
    all)
      build_cli
      start_mysql_stack
      run_mysql_suite
      cleanup
      build_cli
      start_tidb_stack
      run_tidb_suite
      ;;
    *)
      fail "usage: scripts/test_cli_metadata_e2e.sh [mysql|tidb|all]"
      ;;
  esac
}

main "$@"
