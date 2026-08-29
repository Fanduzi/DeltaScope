# Decision: Preserve PostgreSQL Database Selection in MCP Connections

Date: 2026-08-30
Status: Accepted
Related milestone/version: issue #39
Related commits: [`5c6ff0c70aac824610fb58b0403ff4049d2e5641`](https://github.com/Fanduzi/DeltaScope/commit/5c6ff0c70aac824610fb58b0403ff4049d2e5641)
Related tests:
- `TestResolveAuditConnectionResolvesDirectPasswordEnv`
- `TestResolveAuditConnectionLoadsNamedConnectionFromConfig`
- `TestAuditSQLToolPassesConnectionDatabaseAndConnectTimeout`
- `TestGetCapabilitiesToolReturnsKnownSummary`
- `TestRunServesMetadataAwareAuditOverRealPostgreSQL`
Related docs:
- `docs/recipe/use-deltascope-mcp.md`
- `docs/recipe/use-deltascope-mcp.zh-CN.md`

## Context

MCP `audit_sql` accepted PostgreSQL host, user, dialect, and schema inputs but
could not carry the target database catalog. Inline JSON therefore lacked the
field, named `connections.yaml` entries could not preserve it, and the
metadata opener used PostgreSQL's default catalog even when the table existed
in another database. The shared metadata-preparation boundary already carries
database and schema separately and requires a database when PostgreSQL schema
is explicitly selected.

## Decision

Add optional `database` to the shared connection input contract with JSON and
YAML support. MCP resolution preserves it for direct and named connections,
and the MCP metadata audit passes it to the existing
`auditmeta.ConnectionConfig`. `get_capabilities` advertises
`connection.database`.

Database remains optional only when schema is also omitted, preserving default
catalog resolution. An explicit PostgreSQL schema without database continues to
return the bounded shared preparation error. Database and schema remain
distinct fields, and `context.database` is not added.

## Rationale

The existing opener and preparation seam already implement the required
PostgreSQL catalog behavior. Preserving the value at the MCP boundary fixes the
wrong-catalog path without a new resolver, a second validation rule, or changes
to CLI/HTTP semantics.

## Public Contract

- Direct `connection.database` is accepted by MCP `audit_sql`.
- Named `connections.yaml` profiles may define `database`, independently of
  `schema`.
- PostgreSQL metadata-aware audits use the supplied database and schema.
- Omitting both keeps the current default catalog behavior.
- Explicit PostgreSQL `schema` without `database` remains invalid with the
  shared bounded usage error.
- Capabilities list `connection.database`; audit context and result shapes do
  not gain a database field.

## Deferred / Out Of Scope

- Making PostgreSQL `database` mandatory for all MCP connections.
- Adding `context.database` or any database field to audit results.
- Changing CLI or HTTP connection semantics.
- Inferring a database from a schema or merging database/schema concepts.

## Verification Evidence

MCP unit tests cover direct and named JSON/YAML resolution, the generated
connection contract through a real `audit_sql` call, metadata-preparation
propagation, and capability discovery. The PostgreSQL MCP confidence fixture
covers explicit database-plus-schema requests and retains default-catalog
cases where both are omitted. The shared `auditmeta` tests continue to cover
the explicit-schema-without-database rejection.

## Consequences

Future MCP connection changes must preserve database and schema independently
through JSON, YAML, resolution, and metadata-opening seams. Public context must
not echo database unless a separate contract decision authorizes it.

## Links

- Commits: [`5c6ff0c70aac824610fb58b0403ff4049d2e5641`](https://github.com/Fanduzi/DeltaScope/commit/5c6ff0c70aac824610fb58b0403ff4049d2e5641)
- Issue: https://github.com/Fanduzi/DeltaScope/issues/39
- Tests: `internal/interfaces/mcp/connection_test.go`, `internal/interfaces/mcp/server_test.go`, `internal/interfaces/mcp/rule_tools_test.go`, `cmd/deltascope-mcp/main_e2e_postgresql_test.go`
- Shared validation: `internal/application/auditmeta/prepare.go`
- Docs: `docs/recipe/use-deltascope-mcp.md`, `docs/recipe/use-deltascope-mcp.zh-CN.md`
