# Application Audit Metadata Preparation Module

Shared preparation helpers for metadata-aware audit requests before they enter the core DeltaScope audit service.

## Files

| File | Responsibility |
|------|---------------|
| `errors.go` | Defines typed metadata-preparation errors, including MySQL/TiDB alias conflicts and PostgreSQL schema/database validation, for adapter-level classification |
| `prepare.go` | Opens metadata clients, detects dialect, binds MySQL/TiDB catalog aliases through connresolve, validates PostgreSQL database/schema selection, resolves schema, and returns prepared audit context |
| `targets.go` | Infers the session schema from every valid statement's targets — all DDL `TableTargets()` for drop/truncate/create, the alter subject only (rename destinations are new names, not existing-object evidence), and the first DML MutationTarget — even when another bounded statement has a parser error; fails only when no statement can be parsed |
| `client.go` | Bridges MySQL-compatible infrastructure providers into the shared preparation client contract |
| `prepare_test.go` | Verifies shared metadata-aware preparation behavior, including schema inference from valid statements around one parser error |
| `audit_t06a4_test.go` | Verifies the T06-A4 AuditSQL contract with the real `mysqlmeta.Provider` behind a controlled database/sql driver: a stored `'NULL'` sibling keeps DROP COLUMN + CREATE INDEX conservative (`review`/`unverified`, one `unknown_table_state` gap, exactly one provider read), a stored `'<nil>'` sibling stays equally conservative, a stored SQL NULL sibling stays `pass`/`complete`, a parsed-source `DEFAULT 'NULL'` agrees with the provider outcome, provider errors propagate via `errors.Is` rather than degrading into gaps, `ddl.column.default.require` reads only submitted DDL, and all-disabled rules fabricate nothing |

## Exports

- `Client`
- `ConnectionConfig` — includes `ConnectTimeout` for metadata connection timeout
- `Request`
- `PreparedAudit`
- `Prepare(ctx, request)`
- `Error` / `ErrorKind` — typed preparation failures, including `ErrorMySQLDatabaseSchemaConflict` and `ErrorPostgreSQLDatabaseRequired`

## Dependencies

- Upstream: `internal/interfaces/cli`, `internal/interfaces/http`, `internal/interfaces/mcp`
- Downstream: `internal/application/audit`, `internal/application/connresolve`, `internal/domain/spec`, `internal/infrastructure/metadata/mysql`

## Update Rule

- If members/interfaces/dependencies change, update this file in same change.
