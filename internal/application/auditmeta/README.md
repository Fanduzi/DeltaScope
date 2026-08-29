# Application Audit Metadata Preparation Module

Shared preparation helpers for metadata-aware audit requests before they enter the core DeltaScope audit service.

## Files

| File | Responsibility |
|------|---------------|
| `errors.go` | Defines typed metadata-preparation errors, including MySQL/TiDB alias conflicts and PostgreSQL schema/database validation, for adapter-level classification |
| `prepare.go` | Opens metadata clients, detects dialect, normalizes MySQL/TiDB database/schema aliases, validates PostgreSQL database/schema selection, resolves schema, and returns prepared audit context |
| `targets.go` | Infers target tables from SQL statements for schema resolution |
| `client.go` | Bridges MySQL-compatible infrastructure providers into the shared preparation client contract |
| `prepare_test.go` | Verifies shared metadata-aware preparation behavior for CLI and future MCP adapters |

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
