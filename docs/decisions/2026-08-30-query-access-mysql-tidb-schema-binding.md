# Decision: Bind MySQL/TiDB Query Access Schema Across CLI and HTTP

Date: 2026-08-30
Status: Accepted
Related issue: [#34](https://github.com/Fanduzi/DeltaScope/issues/34)
Implementation commits:
- [`d9e1b58f43f5c0253f85cf6a52bd04b11ea740b6`](https://github.com/Fanduzi/DeltaScope/commit/d9e1b58f43f5c0253f85cf6a52bd04b11ea740b6)
- [`e4742dfa6a14502d0b6390ddaedf1efef20498c9`](https://github.com/Fanduzi/DeltaScope/commit/e4742dfa6a14502d0b6390ddaedf1efef20498c9)
- [`42c34590dc53bc7fc388e42684f24d1a761a4aee`](https://github.com/Fanduzi/DeltaScope/commit/42c34590dc53bc7fc388e42684f24d1a761a4aee)
- [`321530cc90df833fd03b3e163c655fb48fb0577f`](https://github.com/Fanduzi/DeltaScope/commit/321530cc90df833fd03b3e163c655fb48fb0577f)
- [`5ee1cca4c2849aeb797cdfeb7ab9caf56ccebf97`](https://github.com/Fanduzi/DeltaScope/commit/5ee1cca4c2849aeb797cdfeb7ab9caf56ccebf97)
- [`d187d754f408656bbf15a4e76cf16a7839d0fdc9`](https://github.com/Fanduzi/DeltaScope/commit/d187d754f408656bbf15a4e76cf16a7839d0fdc9)
- [`67135f56f3a3d9bdd840061b37421e50f1883e72`](https://github.com/Fanduzi/DeltaScope/commit/67135f56f3a3d9bdd840061b37421e50f1883e72)
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
- `TestQueryAccessOnlineKeepsRequestDefaultSeparateFromCatalog`
- `TestQueryAccessOnline_TransportSmoke`

## Context

The online Query Access CLI used `--schema` for the MySQL/TiDB connection but
forwarded an empty `DefaultSchema` to the unified SDK session. An unqualified
relation therefore reached the same-connection resolver without its catalog
qualifier. HTTP named connections already supplied their schema to the request
but allowed a different request-level `default_schema` to silently replace it.

## Decision

Use one shared application-layer resolver for online MySQL/TiDB request
construction:

- MySQL/TiDB database and connection schema are catalog aliases; either one
  supplies the request default when `default_schema` (or CLI
  `--default-schema`) is omitted.
- An explicit matching value is accepted.
- Conflicting values return bounded guidance before the online connection is
  opened or analysis begins.
- The CLI and HTTP MySQL/TiDB connection configs pass the canonical catalog;
  an HTTP request-only default does not select a catalog.
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

- Online MySQL/TiDB CLI `--database app` or `--schema app` selects catalog
  `app` and resolves unqualified relations against `app` when
  `--default-schema` is omitted.
- Online HTTP named MySQL/TiDB connections use their configured `database` or
  `schema` alias for the same two purposes when `default_schema` is omitted.
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
bounded pre-open/pre-analysis conflicts, request/catalog separation, unchanged
SQL, and PostgreSQL database/schema preservation. The existing Query Access
corpus remains the semantic regression route; the live CLI TLS route exercises
an unqualified MySQL relation with `--schema`, and the real HTTP transport
smoke covers the same unqualified relation for MySQL and TiDB.

## Links

- Implementation: `internal/application/queryaccess/contracts.go`, `internal/interfaces/cli/query_access.go`, `internal/interfaces/http/query_access.go`
- CLI reference: `docs/reference/query-access-analysis.md`, `docs/reference/query-access-analysis_zh.md`
- HTTP reference: `docs/reference/http-api.md`, `docs/reference/http-api.zh-CN.md`
