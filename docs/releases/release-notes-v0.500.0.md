# DeltaScope v0.500.0 Release Notes

## Summary - Parser Recovery, Query Access Contracts, and Metadata Closure

v0.500.0 publishes the #31–#53 source work that landed after v0.490.0. Mixed migrations keep valid statements when one statement fails to parse. A leading UTF-8 BOM is stripped before audit and Query Access. Query Access empty `--sql`, audit-only flags, MySQL/TiDB schema binding, and the PostgreSQL 17 version boundary now match the documented contracts. PostgreSQL database/schema/port/MCP catalog selection and bounded offline DML impact are aligned across surfaces. MySQL/TiDB metadata-aware rules cover missing DML targets, MODIFY nullability restatements, column-level PRIMARY KEY, and ALTER TABLE ADD INDEX. The pull-request workflow runs the SQL corpus gate. Representative MySQL and PostgreSQL fixtures were expanded.

This is still static analysis. DeltaScope does not execute submitted SQL, retrieve query results, or decide authorization, grants, RLS, or masking. MCP still has no Query Access tool. PostgreSQL 16 and every version outside the trusted PG17 series stay outside the Query Access trust boundary. The 85/100 source-readiness dogfood is a statement about this source milestone, not an SLA and not SQL grammar coverage. Published v0.490.0 remains the previous package at 58/100 because it predates these changes.

## What Changed

### Parser Recovery and BOM

- Multi-statement input is split at bounded lexical semicolon boundaries, then each slice is parsed independently. Valid siblings keep findings, impact estimates, and source locations in original order.
- Each failed slice contributes one `parser_error` diagnostic with `audited=false` and optional 1-based `line` / `column`. Any failed slice keeps the audit error non-nil and CLI exit 2.
- SDK, CLI, HTTP, and MCP serialize the same partial result. Diagnostics contain no raw SQL or parser internals.
- Exactly one leading UTF-8 BOM is removed on the shared input boundary before empty-input validation and parser dispatch. BOM-only input is empty.

### Query Access Contracts

- Explicit empty or whitespace-only `query-access analyze --sql` exits 3 with `SQL input must not be empty` and does not read stdin.
- `--format` and `--fail-on` remain audit-only. Passing them to Query Access is an unsupported usage error with exit 3.
- Complete effect-free `read_only` results return `admissible`. `not_read_only` returns `rejected`. Unresolved requirements stay `indeterminate` with a stable reason code.
- Online MySQL/TiDB Query Access binds database and connection schema as catalog aliases. Conflicting values fail before analysis. PostgreSQL database and schema stay distinct.
- PostgreSQL 16 and other non-PG17 servers stay outside Query Access. The shared identity parser returns a bounded version requirement instead of collapsing it to a generic connection failure. Audit connectivity is unchanged.

### PostgreSQL Boundary and Impact

- Metadata-aware PostgreSQL rejects an explicit schema without `--database`. Database alone, both, or neither keep existing behavior.
- Explicit `--dialect postgresql` with an omitted port resolves to `5432`. Explicit ports and auto-detected MySQL/TiDB keep `3306`.
- MCP `audit_sql` accepts optional `connection.database` for direct and named connections. `get_capabilities` advertises `connection.database`.
- Offline PostgreSQL `DELETE`/`UPDATE` with single-table `id =` literal or `$1` equality now use the existing shape estimate (`estimated_rows=1`, `source=shape`, `reason_codes=["pk_equality"]`). Live planner output remains `source=plan` and takes precedence. Other unproven shapes stay conservative.

### MySQL/TiDB Metadata Rules

- Column-level `PRIMARY KEY` populates `DDL.PrimaryKey` and satisfies `ddl.table.primary_key.require`. Column-level `UNIQUE` is not a primary key.
- Metadata-aware MySQL/TiDB `INSERT`/`UPDATE`/`DELETE` emit `dml.table.exists.require` when the provider confirms the target is missing. Offline and PostgreSQL requests produce no finding.
- Confirmed `MODIFY COLUMN` nullability restatements no longer fire the transition blocker. Unknown prior state emits notice `ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory`.
- `ALTER TABLE ADD INDEX` / `ADD KEY` normalize to `add_index` and `ddl.create_index.notice`. True `ADD CONSTRAINT` stays `add_constraint`.
- Shared lifecycle notices render normalized identifiers for `RENAME TABLE`, `CREATE INDEX`, and `DROP INDEX`.
- CLI `--database` is the MySQL/TiDB catalog alias when `--schema` is omitted. Conflicting values are a bounded usage error.

### Operator, Corpus, and CI

- Unpinned `install.sh` falls back from GitHub REST `releases/latest` to the public latest-release redirect when the REST lookup fails. `DELTASCOPE_VERSION` remains the first path.
- TLS metadata connection failures distinguish hostname mismatch, unknown CA, and a server that did not offer TLS, while keeping exit 3.
- Pull requests run `make sql-corpus-gates`. A reported 100% value is supported rule-and-dialect fixture coverage, not SQL syntax or grammar coverage.
- Representative MySQL and PostgreSQL corpus fixtures were expanded (MySQL 32 YAML, TiDB 24, PostgreSQL 230).

## What Stayed the Same

- MCP tools remain `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities` only.
- Query Access still does not authenticate callers, evaluate grants, enforce RLS, mask columns, rewrite SQL, auto-grant privileges, or guarantee a later execution snapshot.
- Existing MySQL/TiDB Query Access admission envelopes and the exact PG17 `COUNT(1)` envelope from v0.480.0 are unchanged except the contracts listed above.
- `level` remains the public audit priority field; no severity field is introduced.
- Existing release tags, GitHub Releases, npm packages, and Homebrew casks are untouched until this tag publishes.

## Non-Goals

- Not an MCP Query Access tool.
- Not authorization, grants, roles, RLS, masking, rewrite, SQL execution, or data-returning APIs.
- Not general PostgreSQL Query Access expansion beyond the already-published PG17 envelopes.
- Not PostgreSQL 16 Query Access support.
- Not a fallback grammar, token-to-AST recovery, or semantic guessing for parser-failed statements.
- Not SQL syntax or grammar coverage; 100.0% is supported rule-and-dialect fixture coverage.
- Not an SLA. Source-readiness 85/100 describes this source milestone after DBA, application, and CI dogfood. It is not an automated metric and is not the previous published-package score.
- Not a severity field; not putting `context` on `pkg/deltascope.Result`.
- Not a change to any previously published artifact or existing tag.

## Rule Catalog Facts

The registered audit rule catalog adds two MySQL/TiDB rules relative to v0.490.0: `dml.table.exists.require` (blocker) and `ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory` (notice).

| Metric | Count |
|--------|------:|
| Total rules | **373** |
| blocker | 73 |
| warning | 142 |
| notice | 158 |

| Dialect scope | Rules |
|---------------|------:|
| common | 177 |
| postgresql | 191 |
| mysql | 1 |
| tidb | 2 |
| mysql and tidb | 2 |

| Statement kind | Rules |
|----------------|------:|
| ddl | 362 |
| dml | 11 |

## Corpus and Catalog Facts

- Supported rule-and-dialect fixture coverage: **586/586**, **100.0%**, **286** YAML fixture files. This is not SQL syntax or grammar coverage.
- PostgreSQL ALTER TABLE config entries: **53**.
- DDL coverage catalog: **407** entries (mysql 62, tidb 55, postgresql 290, parser_upgrade_candidate 18).

## Decision Records

- `docs/decisions/2026-08-30-partial-parser-error-recovery.md` (this release)
- `docs/decisions/2026-08-30-leading-utf8-bom-sql-input.md` (this release)
- `docs/decisions/2026-08-17-cli-explicit-empty-sql-input-source.md` (this release; #32)
- `docs/decisions/2026-08-30-query-access-cli-flag-ownership.md` (this release)
- `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md` (this release; #35 amendment)
- `docs/decisions/2026-08-30-query-access-mysql-tidb-schema-binding.md` (this release)
- `docs/decisions/2026-08-30-query-access-postgresql-version-error-contract.md` (this release)
- `docs/decisions/2026-08-30-cli-postgresql-schema-database-validation.md` (this release)
- `docs/decisions/2026-08-30-cli-postgresql-default-port.md` (this release)
- `docs/decisions/2026-08-30-mcp-postgresql-database-selection.md` (this release)
- `docs/decisions/2026-08-30-postgresql-offline-impact-contract.md` (this release)
- `docs/decisions/2026-08-30-mysql-tidb-column-primary-key-extraction.md` (this release)
- `docs/decisions/2026-08-30-mysql-tidb-dml-table-existence.md` (this release)
- `docs/decisions/2026-08-30-mysql-tidb-modify-nullability-state.md` (this release)
- `docs/decisions/2026-08-30-mysql-tidb-alter-index-action-normalization.md` (this release)
- `docs/decisions/2026-08-30-issue-48-lifecycle-notice-identifiers.md` (this release)
- `docs/decisions/2026-08-30-cli-mysql-tidb-database-schema-alias.md` (this release)
- `docs/decisions/2026-08-30-cli-tls-metadata-error-categories.md` (this release)
- `docs/decisions/2026-08-30-release-installer-latest-fallback.md` (this release)
- `docs/decisions/2026-08-30-pr-sql-corpus-coverage-contract.md` (this release)
- `docs/decisions/2026-08-30-source-readiness-85-dogfood.md` (this release)
- `docs/decisions/2026-08-20-offline-existence-caveat-context.md` (v0.490.0)
