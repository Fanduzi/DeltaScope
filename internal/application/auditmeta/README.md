# Application Audit Metadata Preparation Module

Shared preparation helpers for metadata-aware audit requests before they enter the core DeltaScope audit service.

## Files

| File | Responsibility |
|------|---------------|
| `errors.go` | Defines typed metadata-preparation errors, including MySQL/TiDB alias conflicts and PostgreSQL schema/database validation, for adapter-level classification |
| `prepare.go` | Opens metadata clients, detects dialect, normalizes known MySQL/TiDB database/schema catalog selection before opening and detected aliases after identity, validates PostgreSQL database/schema selection, resolves schema, and returns prepared audit context |
| `targets.go` | Infers target tables from every successfully parsed statement even when another bounded statement has a parser error; fails only when no statement can be parsed |
| `client.go` | Bridges MySQL-compatible infrastructure providers into the shared preparation client contract |
| `prepare_test.go` | Verifies shared metadata-aware preparation behavior, including schema inference from valid statements around one parser error |

## Exports

- `Client`
- `ConnectionConfig` — includes `ConnectTimeout` for metadata connection timeout
- `Request`
- `PreparedAudit`
- `Prepare(ctx, request)`
- `Error` / `ErrorKind` — typed preparation failures, including `ErrorMySQLDatabaseSchemaConflict` and `ErrorPostgreSQLDatabaseRequired`

## Dependencies

- Upstream: `internal/interfaces/cli`, `internal/interfaces/http`, `internal/interfaces/mcp`
- Downstream: `internal/application/audit`, `internal/domain/spec`, `internal/infrastructure/metadata/mysql`

## Update Rule

- If members/interfaces/dependencies change, update this file in same change.
