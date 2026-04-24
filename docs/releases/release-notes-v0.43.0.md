# DeltaScope v0.43.0 Release Notes

## Summary

DeltaScope v0.43.0 introduces default policy dialect isolation. When `--dialect postgresql` is set, DeltaScope no longer emits MySQL/TiDB-only rule IDs or MySQL-specific remediation text. When `--dialect mysql` or `--dialect tidb` is set, DeltaScope no longer emits PostgreSQL-only rule IDs. Isolation is enforced at the rule `AppliesTo` gate level, not by post-filtering reports.

## Added

- Default policy isolates rules by `--dialect` across MySQL, TiDB, and PostgreSQL.
- PostgreSQL audits skip MySQL-family rules:
  - `ddl.table.engine.allowlist`
  - `ddl.table.charset.allowlist`
  - `ddl.table.row_format.allowlist`
  - `ddl.table.auto_increment.init_value.require`
  - `ddl.table.primary_key.unsigned.require`
  - `ddl.table.primary_key.auto_increment.require`
  - `ddl.table.primary_key.not_null.require`
  - `ddl.table.partition.forbid`
  - `ddl.table.create_as.forbid`
  - `ddl.table.create_like.forbid`
  - `ddl.column.charset.allowlist`
  - `ddl.column.collation.allowlist`
  - `ddl.column.charset_collation.match.require`
  - `ddl.alter.change_column.forbid`
  - `ddl.alter.modify_column.forbid`
- PostgreSQL `CREATE TABLE` audits no longer suggest MySQL-only `ON UPDATE CURRENT_TIMESTAMP` for the updated-time audit column check.
- MySQL/TiDB audits exclude all `ddl.pg.*` rules and PostgreSQL-only unprefixed dialect-gated rules.
- Service-level tests assert cross-dialect rule isolation:
  - `TestPostgreSQLDefaultAuditExcludesMySQLFamilyRules`
  - `TestPostgreSQLDefaultAuditExcludesMySQLRemediationText`
  - `TestMySQLDefaultAuditExcludesPostgreSQLRules`
- SQL corpus PostgreSQL probe includes a negative `exclude:` block listing MySQL-family rules that must never appear.

## Example

PostgreSQL audit before v0.43.0 could emit MySQL-only findings:

```text
[blocker] ddl.table.charset.allowlist: table charset is not in the allowlist
[blocker] ddl.table.primary_key.unsigned.require: single primary-key column should be unsigned bigint
```

PostgreSQL audit with v0.43.0 produces only dialect-appropriate findings:

```bash
deltascope audit \
  --dialect postgresql \
  --sql "CREATE TABLE users (id bigint PRIMARY KEY, name varchar(64) NOT NULL);"
```

## Rule Contract

| Field | Value |
|------|-------|
| Kind | Default policy dialect isolation |
| Enforcement | Rule `AppliesTo` gate level |
| Scope | MySQL-family rules excluded from PostgreSQL; PostgreSQL-only rules excluded from MySQL/TiDB |
| Shared rules | Remain active across all dialects (comments, naming, PK presence, PK column count) |

## Non-Goals

- New rule IDs
- New parser features
- New public API contracts
- Live schema validation
- Cross-database or cross-deployment tracking
- MySQL/TiDB behavior changes beyond dialect isolation
- PostgreSQL type canonicalization (`pg_catalog.int8` normalization)

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.43.0/install.sh | \
  DELTASCOPE_VERSION=v0.43.0 sh
```
