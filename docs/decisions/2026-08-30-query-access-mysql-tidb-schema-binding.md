# Decision: Bind MySQL/TiDB Query Access Schema Across CLI and HTTP

Date: 2026-08-30
Status: Accepted
Related issue: [#34](https://github.com/Fanduzi/DeltaScope/issues/34)
Implementation commits:
- [`0766cd6d79cb39d438202756df3e050f5b4b22fe`](https://github.com/Fanduzi/DeltaScope/commit/0766cd6d79cb39d438202756df3e050f5b4b22fe)
- [`70ec6dd46db7c0ab1b8d338c14d37477c147fdc6`](https://github.com/Fanduzi/DeltaScope/commit/70ec6dd46db7c0ab1b8d338c14d37477c147fdc6)
- [`630afee584e2c398543e2ec009a2cbccda4b7137`](https://github.com/Fanduzi/DeltaScope/commit/630afee584e2c398543e2ec009a2cbccda4b7137)
Related decisions:
- [Scope CLI rendering and threshold flags to audit](2026-08-30-query-access-cli-flag-ownership.md)
- [Treat MySQL/TiDB database as the catalog alias](2026-08-30-cli-mysql-tidb-database-schema-alias.md)
- [Route online Query Access through one public session entry](2026-08-12-query-access-online-analysis-entry.md)
Related tests:
- `TestQueryAccessOnlineBindsMySQLTiDBSchema`
- `TestQueryAccessOnlineRejectsConflictingMySQLTiDBSchemaBeforeAnalysis`
- `TestHandlerQueryAccessOnlineBindsMySQLTiDBSchema`
- `TestHandlerQueryAccessOnlineRejectsConflictingMySQLTiDBSchemaBeforeOpen`
- `TestHandlerQueryAccessOnlinePreservesPostgreSQLDatabaseAndSchema`

## Context

The online Query Access CLI used `--schema` for the MySQL/TiDB connection but
forwarded an empty `DefaultSchema` to the unified SDK session. An unqualified
relation therefore reached the same-connection resolver without its catalog
qualifier. HTTP named connections already supplied their schema to the request
but allowed a different request-level `default_schema` to silently replace it.

## Decision

Use one shared application-layer resolver for online MySQL/TiDB request
construction:

- A connection schema supplies the request default when `default_schema` (or
  CLI `--default-schema`) is omitted.
- An explicit matching value is accepted.
- Conflicting values return bounded guidance before the online connection is
  opened or analysis begins.
- The CLI and HTTP MySQL/TiDB connection configs pass the selected schema as
  the catalog; an HTTP request-only default does not select a catalog.
- Qualified SQL remains unchanged, and PostgreSQL keeps its separate
  database/schema behavior.

The change does not add result or request fields, alter proof eligibility, or
expand the MySQL/TiDB semantic manifest.

## Rationale

The application query-access contract is the smallest shared seam available to
both transport adapters. Keeping the normalization there prevents the CLI and
HTTP paths from drifting while leaving connection lifecycle, authorization,
error presentation, and the opaque SDK session boundary in their owning
adapters.

## Public Contract

- Online MySQL/TiDB CLI `--schema app` selects catalog `app` and resolves
  unqualified relations against `app` when `--default-schema` is omitted.
- Online HTTP named MySQL/TiDB connections use their configured `schema` for
  the same two purposes when `default_schema` is omitted.
- An HTTP request-only `default_schema` remains a qualifier hint and does not
  change a named connection's configured catalog.
- Equal explicit and connection schema values are valid.
- Conflicts fail closed with a bounded usage/`invalid_request` response and no
  selected values, SQL, credentials, or connection details echoed.
- Qualified relations, PostgreSQL, offline Query Access, MCP, and proof rules
  keep their existing contracts.

## Deferred / Out Of Scope

- Adding Query Access result identity or database fields.
- Merging PostgreSQL database and schema concepts.
- Adding an MCP Query Access tool.
- Changing function/operator proof rules or manifest contents.

## Verification Evidence

Focused CLI and HTTP tests cover MySQL/TiDB schema-only and matching values,
bounded pre-open/pre-analysis conflicts, unchanged SQL, and PostgreSQL
database/schema preservation. The existing Query Access corpus remains the
semantic regression route; the live CLI TLS route exercises an unqualified
MySQL relation with `--schema`.

## Links

- Implementation: `internal/application/queryaccess/contracts.go`, `internal/interfaces/cli/query_access.go`, `internal/interfaces/http/query_access.go`
- CLI reference: `docs/reference/query-access-analysis.md`, `docs/reference/query-access-analysis_zh.md`
- HTTP reference: `docs/reference/http-api.md`, `docs/reference/http-api.zh-CN.md`
