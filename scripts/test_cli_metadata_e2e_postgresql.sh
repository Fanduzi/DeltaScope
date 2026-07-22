#!/usr/bin/env bash
# input: docker compose PostgreSQL service, fixture SQL, and deltascope CLI invocations with --dialect postgresql
# output: repeatable metadata-aware CLI e2e execution for PostgreSQL with JSON assertion helpers
# pos: shell-based end-to-end harness for live metadata PG CLI validation
# note: if this file changes, update this header and module README.md.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/docker/pg-e2e-compose.yaml"
PG_CONTAINER="deltascope-pg-e2e"
PG_HOST="127.0.0.1"
PG_PORT="5500"
PG_USER="root"
PG_PASSWORD="root"
export PG_PASSWORD
TMP_DIR=""
CLI_BIN=""

log() {
  printf '[pg-cli-metadata-e2e] %s\n' "$*"
}

fail() {
  printf '[pg-cli-metadata-e2e][FAIL] %s\n' "$*" >&2
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
    status="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${container}" 2>/dev/null || true)"
    if [[ "${status}" == "healthy" || "${status}" == "running" ]]; then
      return 0
    fi
    sleep "${delay}"
  done

  fail "container ${container} did not become ready (last status: ${status:-unknown})"
}

build_cli() {
  TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/deltascope-pg-cli-e2e.XXXXXX")"
  CLI_BIN="${TMP_DIR}/deltascope"
  (
    cd "${ROOT_DIR}"
    CGO_ENABLED=1 go build -tags postgresql -o "${CLI_BIN}" ./cmd/deltascope
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
  local label="${3:-}"
  [[ "${actual}" == "${expected}" ]] || fail "expected exit code ${expected}, got ${actual}${label:+ (${label})}"
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

assert_json_field_exists() {
  local json_file="$1"
  local field_path="$2"

  python3 - "$json_file" "$field_path" <<'PY'
import json
import sys

path = sys.argv[2].split(".")
with open(sys.argv[1], "r", encoding="utf-8") as handle:
    data = json.load(handle)
value = data
for part in path:
    if isinstance(value, list):
        value = value[int(part)]
    else:
        value = value.get(part)
if value is None:
    raise SystemExit(f"expected {sys.argv[2]} to exist, got null")
PY
}

start_pg_stack() {
  log "starting PostgreSQL fixtures"
  compose up -d
  wait_for_health "${PG_CONTAINER}"
}

run_pg_suite() {
  local stdout_file
  local stderr_file
  local exit_code

  log "running PostgreSQL metadata-aware CLI cases"

  # Case 1: basic metadata-aware connection with qualified schema
  stdout_file="$(mktemp "${TMP_DIR}/pg-qualified.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/pg-qualified.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "delete from app.users where id = 1" --host "${PG_HOST}" --port "${PG_PORT}" --user "${PG_USER}" --password-env PG_PASSWORD --dialect postgresql --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "case1-qualified"
  assert_json_field_eq "${stdout_file}" "context.mode" "metadata-aware"
  assert_json_field_eq "${stdout_file}" "context.dialect" "postgresql"
  assert_json_field_eq "${stdout_file}" "context.schema" "app"

  # Case 2: explicit --schema flag
  stdout_file="$(mktemp "${TMP_DIR}/pg-explicit-schema.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/pg-explicit-schema.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "delete from users where id = 1" --host "${PG_HOST}" --port "${PG_PORT}" --user "${PG_USER}" --password-env PG_PASSWORD --dialect postgresql --schema archive --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "case2-explicit-schema"
  assert_json_field_eq "${stdout_file}" "context.schema" "archive"

  # Case 3: table existence check — create table that already exists
  stdout_file="$(mktemp "${TMP_DIR}/pg-exists.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/pg-exists.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "create table app.users (id bigserial primary key)" --host "${PG_HOST}" --port "${PG_PORT}" --user "${PG_USER}" --password-env PG_PASSWORD --dialect postgresql --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 1 "case3-table-exists"
  assert_json_rule_present "${stdout_file}" "ddl.table.exists.create.forbid"

  # Case 4: DELETE with plan estimation (planner impact)
  stdout_file="$(mktemp "${TMP_DIR}/pg-plan-delete.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/pg-plan-delete.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "delete from app.orders where user_id = 1" --host "${PG_HOST}" --port "${PG_PORT}" --user "${PG_USER}" --password-env PG_PASSWORD --dialect postgresql --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "case4-plan-delete"
  assert_json_field_exists "${stdout_file}" "statements.0.impact"
  assert_json_field_eq "${stdout_file}" "statements.0.impact.source" "plan"

  # Case 5: UPDATE with plan estimation
  stdout_file="$(mktemp "${TMP_DIR}/pg-plan-update.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/pg-plan-update.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "update app.users set name = 'x' where id = 1" --host "${PG_HOST}" --port "${PG_PORT}" --user "${PG_USER}" --password-env PG_PASSWORD --dialect postgresql --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "case5-plan-update"
  assert_json_field_exists "${stdout_file}" "statements.0.impact"
  assert_json_field_eq "${stdout_file}" "statements.0.impact.source" "plan"

  # Case 6: DROP CONSTRAINT → primary key mapping
  stdout_file="$(mktemp "${TMP_DIR}/pg-drop-constraint.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/pg-drop-constraint.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "alter table app.accounts drop constraint accounts_pkey" --host "${PG_HOST}" --port "${PG_PORT}" --user "${PG_USER}" --password-env PG_PASSWORD --dialect postgresql --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 1 "case6-drop-pk"
  assert_json_rule_present "${stdout_file}" "ddl.alter.drop_primary_key.forbid"

  # Case 7: rename column existence check
  stdout_file="$(mktemp "${TMP_DIR}/pg-rename-col.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/pg-rename-col.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "alter table app.users rename column missing_col to email" --host "${PG_HOST}" --port "${PG_PORT}" --user "${PG_USER}" --password-env PG_PASSWORD --dialect postgresql --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 1 "case7-rename-col"
  assert_json_rule_present "${stdout_file}" "ddl.alter.rename_column.exists.require"

  # Case 8: rename index fires forbid rule for an existing index
  stdout_file="$(mktemp "${TMP_DIR}/pg-rename-idx.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/pg-rename-idx.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "alter index idx_accounts_email rename to idx_new" --host "${PG_HOST}" --port "${PG_PORT}" --user "${PG_USER}" --password-env PG_PASSWORD --dialect postgresql --schema app --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 1 "case8-rename-idx"
  assert_json_rule_present "${stdout_file}" "ddl.alter.rename_index.forbid"

  # Case 9: drop column existence check — column does not exist
  stdout_file="$(mktemp "${TMP_DIR}/pg-drop-col.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/pg-drop-col.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "alter table app.users drop column missing_col" --host "${PG_HOST}" --port "${PG_PORT}" --user "${PG_USER}" --password-env PG_PASSWORD --dialect postgresql --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 1 "case9-drop-col"
  assert_json_rule_present "${stdout_file}" "ddl.alter.drop_column.exists.require"

  # Case 10: CREATE UNIQUE INDEX prefix rule (statement-local, offline)
  stdout_file="$(mktemp "${TMP_DIR}/pg-unique-idx-prefix.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/pg-unique-idx-prefix.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "CREATE UNIQUE INDEX bad_email_unique ON users (email);" --dialect postgresql --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "case10-unique-idx-prefix"
  assert_json_rule_present "${stdout_file}" "ddl.index.unique.prefix.require"

  # Case 11: ALTER TABLE ADD CONSTRAINT UNIQUE prefix rule (statement-local, offline)
  stdout_file="$(mktemp "${TMP_DIR}/pg-alter-add-constraint-unique.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/pg-alter-add-constraint-unique.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "ALTER TABLE users ADD CONSTRAINT bad_email_key UNIQUE (email);" --dialect postgresql --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "case11-alter-add-constraint-unique"
  assert_json_rule_present "${stdout_file}" "ddl.alter.add_index.unique.prefix.require"

  # Case 12: ALTER TABLE ADD CONSTRAINT FOREIGN KEY — forbid + cross-schema advisory (statement-local, offline)
  stdout_file="$(mktemp "${TMP_DIR}/pg-alter-add-fk.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/pg-alter-add-fk.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "ALTER TABLE public.orders ADD CONSTRAINT fk_orders_approver FOREIGN KEY (approver_id) REFERENCES auth.users(id);" --dialect postgresql --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 1 "case12-alter-add-fk"
  assert_json_rule_present "${stdout_file}" "ddl.table.foreign_key.forbid"
  assert_json_rule_present "${stdout_file}" "ddl.pg.table.foreign_key.cross_schema.advisory"

  # Case 13: ALTER TABLE ADD CONSTRAINT CHECK — not_valid advisory + prefix naming (with config)
  local check_config
  check_config="$(mktemp "${TMP_DIR}/pg-check-policy.XXXXXX.yaml")"
  cat >"${check_config}" <<'CFG'
rules:
  ddl.constraint.check.name.prefix.require:
    enabled: true
    params:
      prefix: ck_
CFG

  stdout_file="$(mktemp "${TMP_DIR}/pg-alter-add-check.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/pg-alter-add-check.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "ALTER TABLE orders ADD CONSTRAINT amount_positive CHECK (amount >= 0);" --dialect postgresql --config "${check_config}" --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "case13-alter-add-check"
  assert_json_rule_present "${stdout_file}" "ddl.pg.alter.add_check.not_valid.require"
  assert_json_rule_present "${stdout_file}" "ddl.constraint.check.name.prefix.require"

  # Case 14: ALTER TABLE ADD CONSTRAINT NOT VALID requires later VALIDATE (global rule, offline)
  stdout_file="$(mktemp "${TMP_DIR}/pg-not-valid-validate.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/pg-not-valid-validate.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "ALTER TABLE orders ADD CONSTRAINT chk_orders_amount CHECK (amount >= 0) NOT VALID;" --dialect postgresql --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "case14-not-valid-validate"
  assert_json_rule_present "${stdout_file}" "ddl.pg.alter.not_valid_constraint.validate.require"

  # Case 15: default-policy dialect hygiene — no MySQL-family leakage under PG dialect
  stdout_file="$(mktemp "${TMP_DIR}/pg-hygiene.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/pg-hygiene.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit --sql "CREATE TABLE pg_smoke (id bigint primary key);" --dialect postgresql --format json --fail-on none; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "case15-pg-hygiene"
  for rule_id in \
    ddl.table.charset.allowlist \
    ddl.table.engine.allowlist \
    ddl.table.row_format.allowlist \
    ddl.table.auto_increment.init_value.require \
    ddl.table.primary_key.unsigned.require \
    ddl.table.primary_key.auto_increment.require \
    ddl.table.primary_key.bigint.require \
    ddl.column.charset.allowlist \
    ddl.column.collation.allowlist \
    ddl.column.charset_collation.match.require \
    ddl.index.key_length.max_bytes.require; do
    assert_json_rule_absent "${stdout_file}" "${rule_id}"
  done
  for token in \
    UNSIGNED AUTO_INCREMENT auto_increment \
    CHARSET charset COLLATE collation \
    ENGINE ROW_FORMAT "ON UPDATE CURRENT_TIMESTAMP"; do
    assert_findings_text_absent "${stdout_file}" "${token}"
  done

  # Case 16: --database flag selects non-default database
  # The query_access_e2e database has a sentinel table app.query_access_only
  # that does NOT exist in the default postgres database.
  stdout_file="$(mktemp "${TMP_DIR}/pg-database.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/pg-database.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" query-access analyze --sql "SELECT id FROM app.query_access_only" --host "${PG_HOST}" --port "${PG_PORT}" --user "${PG_USER}" --password-env PG_PASSWORD --database query_access_e2e --schema app --dialect postgresql; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "case16-database-selection"
  assert_json_field_eq "${stdout_file}" "admission" "admissible"

  # Case 17: default postgres database does NOT have the sentinel table
  # Without --database, the CLI defaults to postgres where query_access_only
  # does not exist, so the query must fail or be indeterminate.
  stdout_file="$(mktemp "${TMP_DIR}/pg-database-default.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/pg-database-default.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" query-access analyze --sql "SELECT id FROM app.query_access_only" --host "${PG_HOST}" --port "${PG_PORT}" --user "${PG_USER}" --password-env PG_PASSWORD --schema app --dialect postgresql; then
    exit_code=0
  else
    exit_code=$?
  fi
  # The default postgres database does not have query_access_only, so
  # metadata resolution should fail or return indeterminate.
  if [[ "${exit_code}" == "0" ]]; then
    # If it succeeded, verify it's NOT admissible (indeterminate or rejected)
    local admission
    admission="$(python3 -c "import json; print(json.load(open('${stdout_file}')).get('admission', ''))")"
    if [[ "${admission}" == "admissible" ]]; then
      fail "case17-default-database: expected non-admissible for default postgres, got admissible"
    fi
  fi

  log "all PostgreSQL CLI metadata-aware e2e cases passed"
}

main() {
  require_cmd docker
  require_cmd python3
  require_cmd go
  trap cleanup EXIT

  build_cli
  start_pg_stack
  run_pg_suite
}

main "$@"
