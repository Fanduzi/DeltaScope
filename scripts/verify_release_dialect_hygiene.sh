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

# Python helper for checking findings.
# Modes: no-rule, no-prefix, no-text
check_findings() {
  local json_file="$1"
  local mode="$2"
  shift 2

  python3 - "$json_file" "$mode" "$@" <<'PY'
import json
import sys

json_file = sys.argv[1]
mode = sys.argv[2]
values = sys.argv[3:]

with open(json_file, "r", encoding="utf-8") as handle:
    data = json.load(handle)

findings = list(data.get("global_findings", []))
for statement in data.get("statements", []):
    findings.extend(statement.get("findings", []))

if mode == "no-rule":
    rule_ids = {item.get("rule_id", "") for item in findings}
    leaked = [v for v in values if v in rule_ids]
    if leaked:
        raise SystemExit(f"forbidden rule IDs present: {leaked}")
elif mode == "no-prefix":
    leaked = [
        item.get("rule_id", "")
        for item in findings
        if any(item.get("rule_id", "").startswith(v) for v in values)
    ]
    if leaked:
        raise SystemExit(f"forbidden rule ID prefixes present: {leaked}")
elif mode == "no-text":
    block = json.dumps(findings, ensure_ascii=False)
    leaked = [v for v in values if v in block]
    if leaked:
        raise SystemExit(f"forbidden finding text present: {leaked}")
else:
    raise SystemExit(f"unknown mode: {mode}")
PY
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
  require_cmd python3
  require_cmd mktemp

  [[ -x "${DELTASCOPE_BIN}" ]] || fail "deltascope binary not found or not executable: ${DELTASCOPE_BIN}"

  local tmp_dir
  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "${tmp_dir:-}"' EXIT

  # --- PostgreSQL smoke ---
  local pg_json="${tmp_dir}/pg.json"
  "${DELTASCOPE_BIN}" audit --dialect postgresql --format json --fail-on none --sql "${PG_SQL}" >"${pg_json}"

  check_findings "${pg_json}" no-rule "${PG_FORBIDDEN_RULES[@]}"
  check_findings "${pg_json}" no-text "${PG_FORBIDDEN_TOKENS[@]}"

  # --- MySQL smoke ---
  local mysql_json="${tmp_dir}/mysql.json"
  "${DELTASCOPE_BIN}" audit --dialect mysql --format json --fail-on none --sql "${MYSQL_SQL}" >"${mysql_json}"

  check_findings "${mysql_json}" no-prefix "ddl.pg."
  check_findings "${mysql_json}" no-rule "${PG_ONLY_RULES[@]}"
  check_findings "${mysql_json}" no-text "${PG_ONLY_TOKENS[@]}"

  # --- TiDB smoke ---
  local tidb_json="${tmp_dir}/tidb.json"
  "${DELTASCOPE_BIN}" audit --dialect tidb --format json --fail-on none --sql "${MYSQL_SQL}" >"${tidb_json}"

  check_findings "${tidb_json}" no-prefix "ddl.pg."
  check_findings "${tidb_json}" no-rule "${PG_ONLY_RULES[@]}"
  check_findings "${tidb_json}" no-text "${PG_ONLY_TOKENS[@]}"
}

main "$@"
