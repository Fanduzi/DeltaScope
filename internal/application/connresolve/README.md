# Application Connection Resolution Module

Shared Transport Connection Resolution path that stops before open.

## Files

| File | Responsibility |
|------|----------------|
| doc.go | Package boundary for ready-to-open connection configuration |
| resolve.go | Resolve: password, TLS shape, timeout, port default, catalog bind |
| catalog.go | MySQL/TiDB catalog alias binding shared by Audit and Query Access |
| classify.go | Connection Failure Class mapping for later opener errors |
| port.go | PostgreSQL omitted-port default |
| resolve_test.go | Resolve seam cases |
| catalog_test.go | Catalog alias and conflict cases |
| classify_test.go | Failure class cases |
| port_test.go | PostgreSQL omitted-port cases |

## Exports

- `Resolve()` / `Request` / `Config` / `Error` / `Options`
- `BindMySQLTiDBCatalog()` / `ErrCatalogConflict`
- `Classify()` / `FailureClass` and class constants
- `ApplyPostgreSQLDefaultPort()`

## Dependencies

- Upstream: `internal/application/auditmeta`, `internal/application/queryaccess`, `internal/application/online`, `internal/interfaces/cli`, `internal/interfaces/mcp`
- Downstream: Go standard library

## Update Rule

- If members/interfaces/dependencies change, update this file in the same change.
