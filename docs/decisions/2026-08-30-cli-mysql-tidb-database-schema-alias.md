# Decision: Treat MySQL/TiDB Database as the Catalog Alias

Date: 2026-08-30
Status: Accepted
Related milestone/version: issue #45
Related commits: `3f8f4b0` (initial implementation; follow-up fixes are in the same branch history)
Related tests:
- `TestPrepareUsesMySQLCompatibleDatabaseAsSchemaAlias`
- `TestPrepareRejectsConflictingMySQLCompatibleDatabaseAndSchema`
- `TestAuditCommandUsesMySQLCompatibleDatabaseAsSchemaAlias`
- `TestAuditCommandRejectsConflictingMySQLCompatibleDatabaseAndSchema`
Related docs:
- `docs/reference/cli.md`
- `docs/reference/cli.zh-CN.md`
- `README.md`
- `README_ZH.md`

## Context

MySQL and TiDB operators call the logical namespace a database, while the
metadata-aware CLI previously used only `--schema` for that catalog. The CLI
also dropped `--database` before opening its MySQL-compatible metadata client,
so a valid database-only selection could fall through to ambiguous schema
inference. PostgreSQL uses the same two words for different scopes, and issue
#31 requires that distinction to remain explicit.

## Decision

The shared `auditmeta.Prepare` seam treats `ConnectionConfig.Database` as the
MySQL/TiDB catalog alias when no explicit schema is supplied. Equal
`--database` and `--schema` values are accepted; conflicting values produce a
typed, bounded usage error without echoing either value. When the request
identifies a MySQL/TiDB path, this validation runs before the metadata opener;
callers that leave dialect selection to detection validate the same rule once
server identity is known.
The CLI MySQL/TiDB opener forwards `Database` to the driver so the selected
catalog is also the connection database. PostgreSQL continues to pass database
and schema independently through the existing #31 validation.

## Rationale

`auditmeta.Prepare` is already the shared pre-audit boundary for metadata-aware
adapters, so one normalization rule keeps schema resolution and result context
consistent without changing the public result shape or duplicating CLI guards.
Keeping PostgreSQL outside the alias branch preserves database selection and
schema selection as separate values.

## Public Contract

- MySQL/TiDB metadata-aware audits accept database-only, schema-only, and equal database/schema selections.
- MySQL/TiDB database-only selection reports the selected catalog in the existing `schema` result/context field with `schema_source: "database"`.
- Conflicting MySQL/TiDB selections fail with exit code 2 before opening a known-dialect metadata connection.
- PostgreSQL `--database` remains the catalog selector and `--schema` remains the schema selector; an explicit schema still requires a database.
- No new database field is added to audit results.

## Deferred / Out Of Scope

- Merging PostgreSQL database and schema.
- Changing Query Access behavior tracked by #34.
- Adding database fields to public audit results.
- Changing existence-rule levels or adding live database-existence validation.

## Verification Evidence

The shared preparation and CLI tests cover MySQL/TiDB aliases, schema-only
compatibility, equal values, bounded conflicts, no-opener conflict rejection,
auto-detected MySQL aliasing, and PostgreSQL database/schema separation.
`make test`, `make lint`, `make docs-example-gates`,
`make pg-unit-test-gates`, and `make test-e2e-cli` pass. The PostgreSQL CLI and
HTTP legs of `make pg-confidence-gates` pass; its MCP leg still fails the
pre-existing #31 fixture cases that omit PostgreSQL `--database`.

## Consequences

Future metadata-aware adapters that use `auditmeta.Prepare` inherit the
MySQL/TiDB alias behavior. They must continue to preserve PostgreSQL database
and schema as separate inputs and should use the existing typed preparation
errors for adapter-specific classification.

## Links

- Issue: https://github.com/Fanduzi/DeltaScope/issues/45
- Commits: `3f8f4b0` (initial implementation; follow-up fixes are in the same branch history)
- Tests: `internal/application/auditmeta/prepare_test.go`, `internal/interfaces/cli/audit_metadata_test.go`
- Docs: `docs/reference/cli.md`, `docs/reference/cli.zh-CN.md`, `README.md`, `README_ZH.md`
