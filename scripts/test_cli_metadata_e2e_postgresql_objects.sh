#!/usr/bin/env bash
# input: docker compose PostgreSQL service, object metadata fixtures, and deltascope CLI invocations with explicit database/schema selection
# output: repeatable metadata-object-aware CLI e2e execution for PostgreSQL with JSON assertion helpers
# pos: shell-based end-to-end harness for live object metadata PG CLI validation
# note: if this file changes, update this header and module README.md.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/docker/pg-e2e-compose.yaml"
PG_CONTAINER="deltascope-pg-e2e"
PG_HOST="127.0.0.1"
PG_PORT="5500"
PG_USER="root"
PG_DATABASE="postgres"
PG_PASSWORD="root"
export PG_PASSWORD
TMP_DIR=""
CLI_BIN=""

log() {
  printf '[pg-cli-object-metadata-e2e] %s\n' "$*"
}

fail() {
  printf '[pg-cli-object-metadata-e2e][FAIL] %s\n' "$*" >&2
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

build_cli() {
  TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/deltascope-pg-obj-e2e.XXXXXX")"
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

# assert_finding_metadata_eq finds a rule_id in any finding, then checks a metadata key.
assert_finding_metadata_eq() {
  local json_file="$1"
  local rule_id="$2"
  local meta_key="$3"
  local expected="$4"

  python3 - "$json_file" "$rule_id" "$meta_key" "$expected" <<'PY'
import json
import sys

json_file = sys.argv[1]
rule_id = sys.argv[2]
meta_key = sys.argv[3]
expected = sys.argv[4]

with open(json_file, "r", encoding="utf-8") as f:
    data = json.load(f)

findings = list(data.get("global_findings", []))
for stmt in data.get("statements", []):
    findings.extend(stmt.get("findings", []))

for finding in findings:
    if finding.get("rule_id") == rule_id:
        meta = finding.get("metadata", {})
        actual = meta.get(meta_key)
        actual_str = "" if actual is None else str(actual)
        if actual_str != expected:
            raise SystemExit(
                f"rule {rule_id}: expected metadata.{meta_key}={expected}, got {actual_str}"
            )
        sys.exit(0)

raise SystemExit(f"rule_id {rule_id} not found in findings")
PY
}

# assert_finding_metadata_exists checks that a metadata key exists (not null).
assert_finding_metadata_exists() {
  local json_file="$1"
  local rule_id="$2"
  local meta_key="$3"

  python3 - "$json_file" "$rule_id" "$meta_key" <<'PY'
import json
import sys

json_file = sys.argv[1]
rule_id = sys.argv[2]
meta_key = sys.argv[3]

with open(json_file, "r", encoding="utf-8") as f:
    data = json.load(f)

findings = list(data.get("global_findings", []))
for stmt in data.get("statements", []):
    findings.extend(stmt.get("findings", []))

for finding in findings:
    if finding.get("rule_id") == rule_id:
        meta = finding.get("metadata", {})
        if meta_key not in meta or meta[meta_key] is None:
            raise SystemExit(
                f"rule {rule_id}: expected metadata.{meta_key} to exist, got {meta.get(meta_key)}"
            )
        sys.exit(0)

raise SystemExit(f"rule_id {rule_id} not found in findings")
PY
}

# assert_finding_metadata_not_contains checks that no sensitive string appears in finding metadata values.
assert_no_sensitive_leak() {
  local json_file="$1"
  local rule_id="$2"
  shift 2

  python3 - "$json_file" "$rule_id" "$@" <<'PY'
import json
import sys

json_file = sys.argv[1]
rule_id = sys.argv[2]
sensitive_keys = sys.argv[3:]

with open(json_file, "r", encoding="utf-8") as f:
    data = json.load(f)

findings = list(data.get("global_findings", []))
for stmt in data.get("statements", []):
    findings.extend(stmt.get("findings", []))

for finding in findings:
    if finding.get("rule_id") != rule_id:
        continue
    meta = finding.get("metadata", {})
    block = json.dumps(meta, ensure_ascii=False).lower()
    for sk in sensitive_keys:
        if sk.lower() in block:
            raise SystemExit(
                f"rule {rule_id}: sensitive pattern '{sk}' leaked into finding metadata: {block}"
            )
    sys.exit(0)

raise SystemExit(f"rule_id {rule_id} not found in findings")
PY
}

start_pg_stack() {
  log "starting PostgreSQL fixtures"
  compose up -d
  wait_for_health "${PG_CONTAINER}"
}

run_object_suite() {
  local stdout_file
  local stderr_file
  local exit_code

  SENSITIVE_KEYS="password secret conninfo connection host port query body definition label"
  # 'comment' excluded from SENSITIVE_KEYS: the word appears as object_type/operation names,
  # not as leaked text. Case 10 asserts the actual comment text does not appear.

  log "case 1: confirmed extension — DROP EXTENSION pgcrypto"
  stdout_file="$(mktemp "${TMP_DIR}/obj-ext-confirmed.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/obj-ext-confirmed.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit \
    --sql "DROP EXTENSION pgcrypto" \
    --host "${PG_HOST}" --port "${PG_PORT}" \
    --user "${PG_USER}" --password-env PG_PASSWORD \
    --dialect postgresql --database "${PG_DATABASE}" --schema public --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "case1-ext-confirmed"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_extension.advisory" "metadata_status" "confirmed"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_extension.advisory" "metadata_object_type" "extension"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_extension.advisory" "metadata_object_name" "pgcrypto"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_extension.advisory" "metadata_exists" "True"
  assert_finding_metadata_exists "${stdout_file}" "ddl.pg.drop_extension.advisory" "metadata_extension_version"

  log "case 2: not_found — DROP SCHEMA missing_schema"
  stdout_file="$(mktemp "${TMP_DIR}/obj-schema-notfound.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/obj-schema-notfound.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit \
    --sql "DROP SCHEMA missing_schema" \
    --host "${PG_HOST}" --port "${PG_PORT}" \
    --user "${PG_USER}" --password-env PG_PASSWORD \
    --dialect postgresql --database "${PG_DATABASE}" --schema public --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "case2-schema-notfound"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_schema.advisory" "metadata_status" "not_found"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_schema.advisory" "metadata_object_type" "schema"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_schema.advisory" "metadata_object_name" "missing_schema"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_schema.advisory" "metadata_exists" "False"

  log "case 3: ambiguous type — ALTER TYPE address (exists in app + archive)"
  stdout_file="$(mktemp "${TMP_DIR}/obj-type-ambiguous.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/obj-type-ambiguous.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit \
    --sql "ALTER TYPE address RENAME TO old_address" \
    --host "${PG_HOST}" --port "${PG_PORT}" \
    --user "${PG_USER}" --password-env PG_PASSWORD \
    --dialect postgresql --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "case3-type-ambiguous"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.alter_type.composite_rename.notice" "metadata_status" "ambiguous"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.alter_type.composite_rename.notice" "metadata_object_type" "type"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.alter_type.composite_rename.notice" "metadata_object_name" "address"
  assert_finding_metadata_exists "${stdout_file}" "ddl.pg.alter_type.composite_rename.notice" "metadata_ambiguous_candidates"

  log "case 4: confirmed domain — DROP DOMAIN app.email_address"
  stdout_file="$(mktemp "${TMP_DIR}/obj-domain-confirmed.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/obj-domain-confirmed.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit \
    --sql "DROP DOMAIN app.email_address" \
    --host "${PG_HOST}" --port "${PG_PORT}" \
    --user "${PG_USER}" --password-env PG_PASSWORD \
    --dialect postgresql --database "${PG_DATABASE}" --schema app --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "case4-domain-confirmed"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_domain.advisory" "metadata_status" "confirmed"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_domain.advisory" "metadata_object_type" "domain"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_domain.advisory" "metadata_object_name" "email_address"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_domain.advisory" "metadata_exists" "True"

  log "case 5: confirmed publication — DROP PUBLICATION e2e_test_pub"
  stdout_file="$(mktemp "${TMP_DIR}/obj-pub-confirmed.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/obj-pub-confirmed.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit \
    --sql "DROP PUBLICATION e2e_test_pub" \
    --host "${PG_HOST}" --port "${PG_PORT}" \
    --user "${PG_USER}" --password-env PG_PASSWORD \
    --dialect postgresql --database "${PG_DATABASE}" --schema public --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "case5-pub-confirmed"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_publication.warn" "metadata_status" "confirmed"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_publication.warn" "metadata_object_type" "publication"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_publication.warn" "metadata_object_name" "e2e_test_pub"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_publication.warn" "metadata_exists" "True"

  log "case 6: not_found publication — DROP PUBLICATION missing_pub"
  stdout_file="$(mktemp "${TMP_DIR}/obj-pub-notfound.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/obj-pub-notfound.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit \
    --sql "DROP PUBLICATION missing_pub" \
    --host "${PG_HOST}" --port "${PG_PORT}" \
    --user "${PG_USER}" --password-env PG_PASSWORD \
    --dialect postgresql --database "${PG_DATABASE}" --schema public --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "case6-pub-notfound"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_publication.warn" "metadata_status" "not_found"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_publication.warn" "metadata_object_type" "publication"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_publication.warn" "metadata_object_name" "missing_pub"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_publication.warn" "metadata_exists" "False"

  log "case 7: confirmed sequence — DROP SEQUENCE app.ticket_seq"
  stdout_file="$(mktemp "${TMP_DIR}/obj-seq-confirmed.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/obj-seq-confirmed.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit \
    --sql "DROP SEQUENCE app.ticket_seq" \
    --host "${PG_HOST}" --port "${PG_PORT}" \
    --user "${PG_USER}" --password-env PG_PASSWORD \
    --dialect postgresql --database "${PG_DATABASE}" --schema app --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "case7-seq-confirmed"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_sequence.advisory" "metadata_status" "confirmed"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_sequence.advisory" "metadata_object_type" "sequence"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_sequence.advisory" "metadata_object_name" "ticket_seq"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_sequence.advisory" "metadata_exists" "True"

  log "case 8: confirmed foreign server — DROP SERVER fs_test (no-leak)"
  stdout_file="$(mktemp "${TMP_DIR}/obj-fsrv-confirmed.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/obj-fsrv-confirmed.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit \
    --sql "DROP SERVER fs_test" \
    --host "${PG_HOST}" --port "${PG_PORT}" \
    --user "${PG_USER}" --password-env PG_PASSWORD \
    --dialect postgresql --database "${PG_DATABASE}" --schema public --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "case8-fsrv-confirmed"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_foreign_server.warn" "metadata_status" "confirmed"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_foreign_server.warn" "metadata_object_type" "foreign_server"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_foreign_server.warn" "metadata_object_name" "fs_test"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_foreign_server.warn" "metadata_exists" "True"
  assert_finding_metadata_exists "${stdout_file}" "ddl.pg.drop_foreign_server.warn" "metadata_has_options"
  assert_no_sensitive_leak "${stdout_file}" "ddl.pg.drop_foreign_server.warn" ${SENSITIVE_KEYS}

  log "case 9: confirmed user mapping — DROP USER MAPPING FOR root SERVER fs_test (no-leak)"
  stdout_file="$(mktemp "${TMP_DIR}/obj-umap-confirmed.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/obj-umap-confirmed.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit \
    --sql "DROP USER MAPPING FOR root SERVER fs_test" \
    --host "${PG_HOST}" --port "${PG_PORT}" \
    --user "${PG_USER}" --password-env PG_PASSWORD \
    --dialect postgresql --database "${PG_DATABASE}" --schema public --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "case9-umap-confirmed"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_user_mapping.warn" "metadata_status" "confirmed"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_user_mapping.warn" "metadata_object_type" "user_mapping"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_user_mapping.warn" "metadata_object_name" "root@fs_test"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_user_mapping.warn" "metadata_exists" "True"
  assert_no_sensitive_leak "${stdout_file}" "ddl.pg.drop_user_mapping.warn" ${SENSITIVE_KEYS}

  log "case 10: comment on table — no-leak of comment text"
  stdout_file="$(mktemp "${TMP_DIR}/obj-comment-noleak.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/obj-comment-noleak.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit \
    --sql "COMMENT ON TABLE app.users IS 'new comment text'" \
    --host "${PG_HOST}" --port "${PG_PORT}" \
    --user "${PG_USER}" --password-env PG_PASSWORD \
    --dialect postgresql --database "${PG_DATABASE}" --schema app --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "case10-comment-noleak"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.comment_on.notice" "metadata_status" "confirmed"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.comment_on.notice" "metadata_object_type" "comment"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.comment_on.notice" "metadata_object_name" "users"
  assert_no_sensitive_leak "${stdout_file}" "ddl.pg.comment_on.notice" ${SENSITIVE_KEYS}
  # Assert the actual comment text does not leak into metadata.
  assert_no_sensitive_leak "${stdout_file}" "ddl.pg.comment_on.notice" "new comment text"

  log "case 11: confirmed materialized view — DROP MATERIALIZED VIEW app.user_summary"
  stdout_file="$(mktemp "${TMP_DIR}/obj-mv-confirmed.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/obj-mv-confirmed.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit \
    --sql "DROP MATERIALIZED VIEW app.user_summary" \
    --host "${PG_HOST}" --port "${PG_PORT}" \
    --user "${PG_USER}" --password-env PG_PASSWORD \
    --dialect postgresql --database "${PG_DATABASE}" --schema app --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "case11-mv-confirmed"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_materialized_view.advisory" "metadata_status" "confirmed"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_materialized_view.advisory" "metadata_object_type" "materialized_view"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_materialized_view.advisory" "metadata_object_name" "user_summary"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_materialized_view.advisory" "metadata_exists" "True"

  log "case 12: confirmed enum type — DROP TYPE app.color"
  stdout_file="$(mktemp "${TMP_DIR}/obj-enum-confirmed.XXXXXX.json")"
  stderr_file="$(mktemp "${TMP_DIR}/obj-enum-confirmed.XXXXXX.stderr")"
  if run_cli_capture "${stdout_file}" "${stderr_file}" audit \
    --sql "DROP TYPE app.color" \
    --host "${PG_HOST}" --port "${PG_PORT}" \
    --user "${PG_USER}" --password-env PG_PASSWORD \
    --dialect postgresql --database "${PG_DATABASE}" --schema app --format json; then
    exit_code=0
  else
    exit_code=$?
  fi
  assert_exit_code "${exit_code}" 0 "case12-enum-confirmed"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_type.advisory" "metadata_status" "confirmed"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_type.advisory" "metadata_object_type" "type"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_type.advisory" "metadata_object_name" "color"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_type.advisory" "metadata_exists" "True"
  assert_finding_metadata_eq "${stdout_file}" "ddl.pg.drop_type.advisory" "metadata_type_kind" "enum"

  log "all PostgreSQL CLI object metadata e2e cases passed"
}

main() {
  require_cmd docker
  require_cmd python3
  require_cmd go
  trap cleanup EXIT

  build_cli
  start_pg_stack
  run_object_suite
}

main "$@"
