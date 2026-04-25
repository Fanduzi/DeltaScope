#!/usr/bin/env bash
# input: DELTASCOPE_BIN (optional, defaults to bin/deltascope)
# output: release-blocking MySQL, TiDB, and PostgreSQL default-policy dialect smoke checks
# pos: release contract gate verifying dialect isolation — PG audits must not emit MySQL/TiDB-only rules and vice versa

set -euo pipefail

DELTASCOPE_BIN="${DELTASCOPE_BIN:-bin/deltascope}"

fail() {
  printf '[release-dialect-hygiene][FAIL] %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

# Extract findings arrays from audit JSON as a flat text block.
# DeltaScope JSON has global_findings (top-level array) and
# statements[].findings (per-statement array).  Other top-level
# keys like rule_summary contain all evaluated rule IDs and must
# not be searched.
extract_findings() {
  local json_file="$1"
  grep -o '"findings":\[[^]]*\]' "${json_file}" || true
  grep -o '"global_findings":\[[^]]*\]' "${json_file}" || true
}

# Pure-bash helper for checking findings via grep.
# Modes: no-rule, no-prefix, no-text
check_findings() {
  local json_file="$1"
  local mode="$2"
  shift 2

  local leaked=()

  case "${mode}" in
    no-rule)
      for v in "$@"; do
        if grep -q "\"rule_id\":\"${v}\"" "${json_file}"; then
          leaked+=("${v}")
        fi
      done
      ;;
    no-prefix)
      for v in "$@"; do
        if grep -q "\"rule_id\":\"${v}" "${json_file}"; then
          leaked+=("${v}")
        fi
      done
      ;;
    no-text)
      for v in "$@"; do
        if grep -qF "${v}" "${json_file}"; then
          leaked+=("${v}")
        fi
      done
      ;;
    *)
      fail "unknown mode: ${mode}"
      ;;
  esac

  if [ ${#leaked[@]} -gt 0 ]; then
    case "${mode}" in
      no-rule)   fail "forbidden rule IDs present: ${leaked[*]}" ;;
      no-prefix) fail "forbidden rule ID prefixes present: ${leaked[*]}" ;;
      no-text)   fail "forbidden finding text present: ${leaked[*]}" ;;
    esac
  fi
}

PG_SQL='CREATE TABLE pg_smoke (id bigint primary key);'

MYSQL_SQL='CREATE TABLE smoke_users (
  id bigint unsigned NOT NULL AUTO_INCREMENT,
  name varchar(64) NOT NULL DEFAULT '"'"''"'"',
  created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='"'"'smoke users'"'"';'

PG_FORBIDDEN_RULES=(
  ddl.table.primary_key.unsigned.require
  ddl.table.primary_key.auto_increment.require
  ddl.table.charset.allowlist
  ddl.column.charset.allowlist
  ddl.column.collation.allowlist
  ddl.column.charset_collation.match.require
  ddl.table.engine.allowlist
  ddl.table.row_format.allowlist
  ddl.table.primary_key.bigint.require
)

PG_FORBIDDEN_TOKENS=(
  UNSIGNED
  AUTO_INCREMENT
  auto_increment
  CHARSET
  charset
  COLLATE
  collation
  ENGINE
  ROW_FORMAT
  "ON UPDATE CURRENT_TIMESTAMP"
  "adaptive hash"
  "large prefix"
)

PG_ONLY_RULES=(
  ddl.alter.set_data_type.forbid
  ddl.alter.set_default.forbid
  ddl.alter.drop_default.forbid
  ddl.alter.set_not_null.forbid
  ddl.alter.drop_not_null.forbid
  ddl.alter.drop_expression.forbid
  ddl.alter.set_generated.forbid
  ddl.alter.drop_identity.forbid
  ddl.alter.set_default.explicit_default_change.forbid
  ddl.alter.drop_default.explicit_default_change.forbid
  ddl.alter.set_not_null.explicit_nullability_change.forbid
  ddl.alter.drop_not_null.explicit_nullability_change.forbid
)

PG_ONLY_TOKENS=(
  "VALIDATE CONSTRAINT"
  "NOT VALID"
  CONCURRENTLY
  "DROP EXPRESSION"
  "SET GENERATED"
  "DROP IDENTITY"
  "generated expression"
  rewrite
)

main() {
  require_cmd mktemp

  [[ -x "${DELTASCOPE_BIN}" ]] || fail "deltascope binary not found or not executable: ${DELTASCOPE_BIN}"

  local tmp_dir
  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "${tmp_dir:-}"' EXIT

  # --- PostgreSQL smoke ---
  local pg_json="${tmp_dir}/pg.json"
  "${DELTASCOPE_BIN}" audit --dialect postgresql --format json --fail-on none --sql "${PG_SQL}" >"${pg_json}"
  local pg_findings="${tmp_dir}/pg_findings.txt"
  extract_findings "${pg_json}" >"${pg_findings}"

  check_findings "${pg_findings}" no-rule "${PG_FORBIDDEN_RULES[@]}"
  check_findings "${pg_findings}" no-text "${PG_FORBIDDEN_TOKENS[@]}"

  # --- MySQL smoke ---
  local mysql_json="${tmp_dir}/mysql.json"
  "${DELTASCOPE_BIN}" audit --dialect mysql --format json --fail-on none --sql "${MYSQL_SQL}" >"${mysql_json}"
  local mysql_findings="${tmp_dir}/mysql_findings.txt"
  extract_findings "${mysql_json}" >"${mysql_findings}"

  check_findings "${mysql_findings}" no-prefix "ddl.pg."
  check_findings "${mysql_findings}" no-rule "${PG_ONLY_RULES[@]}"
  check_findings "${mysql_findings}" no-text "${PG_ONLY_TOKENS[@]}"

  # --- TiDB smoke ---
  local tidb_json="${tmp_dir}/tidb.json"
  "${DELTASCOPE_BIN}" audit --dialect tidb --format json --fail-on none --sql "${MYSQL_SQL}" >"${tidb_json}"
  local tidb_findings="${tmp_dir}/tidb_findings.txt"
  extract_findings "${tidb_json}" >"${tidb_findings}"

  check_findings "${tidb_findings}" no-prefix "ddl.pg."
  check_findings "${tidb_findings}" no-rule "${PG_ONLY_RULES[@]}"
  check_findings "${tidb_findings}" no-text "${PG_ONLY_TOKENS[@]}"
}

main "$@"
