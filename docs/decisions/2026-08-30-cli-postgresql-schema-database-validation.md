# Decision: Require PostgreSQL Database With an Explicit Schema

Date: 2026-08-30
Status: Accepted
Related milestone/version: issue #31
Related commits:
Related tests:
- `TestPrepareRejectsPostgreSQLSchemaWithoutDatabase`
- `TestPrepareKeepsPostgreSQLDatabaseSchemaCombinations`
- `TestAuditCommandRejectsPostgreSQLSchemaWithoutDatabase`
Related docs:
- `docs/reference/cli.md`
- `docs/reference/cli.zh-CN.md`

## Context

PostgreSQL metadata audits with an explicit schema but no database opened the
driver's default catalog. An existence check could therefore report a false
missing-table blocker when the table existed in the intended database.

## Decision

The shared metadata-preparation seam rejects an explicit schema with an empty
database whenever the observed dialect is PostgreSQL. It runs after dialect
detection and before schema resolution or audit rule evaluation, so both an
explicit PostgreSQL request and a safely detected PostgreSQL session use the
same validation. The typed error message is bounded:
`PostgreSQL schema and database are distinct; pass --database when --schema is explicitly set`.

The CLI passes this error through as a usage error (exit 2) and emits no audit
result body. A database alone, database plus schema, and omission of both keep
their existing behavior. MySQL and TiDB are unchanged.

## Rationale

`ConnectionConfig` already carries database selection, and `Prepare` is the
shared boundary used before all metadata-aware audit adapters call the audit
service. Validating there avoids duplicated CLI guards and covers dialects
selected after opening without probing services. Treating schema as a database
alias would preserve the wrong-catalog ambiguity rather than fixing it.

## Public Contract

- PostgreSQL `--schema` requires `--database` for metadata-aware CLI audits.
- The invalid combination exits 2, writes no audit body, and does not echo
  credentials, DSNs, or connection details.
- `--database` remains the catalog selector and `--schema` remains the schema
  selector; neither flag is merged or aliased.
- MySQL/TiDB behavior and PostgreSQL rule levels/results are unchanged for valid
  requests.

## Deferred / Out Of Scope

- Inferring a PostgreSQL database from `--schema`.
- Adding database fields to audit results or context.
- Changing existence-rule levels, offline ALTER wording, or connection probing.
- Reopening issue #30 or changing unrelated HTTP/MCP contracts.

## Verification Evidence

The shared preparation tests cover explicit and detected PostgreSQL rejection,
bounded/no-leak error text, client cleanup, database-only, database-plus-schema,
and omitted-both behavior. CLI tests cover explicit and detected PostgreSQL
exit-2/no-stdout behavior and skip rule evaluation. Existing MySQL/TiDB and
PostgreSQL-tagged metadata suites remain green.

## Consequences

Future PostgreSQL metadata connection inputs must preserve database and schema as
separate values. Adapters should route validation through `auditmeta.Prepare`
instead of reimplementing the combination check.

## Links

- Issue: https://github.com/Fanduzi/DeltaScope/issues/31
- Tests: `internal/application/auditmeta/prepare_test.go`, `internal/application/auditmeta/prepare_postgresql_tag_test.go`, `internal/interfaces/cli/audit_metadata_test.go`
- Docs: `docs/reference/cli.md`, `docs/reference/cli.zh-CN.md`
