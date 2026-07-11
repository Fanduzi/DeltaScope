# Application Query Access Module

Application-level contracts for query access analysis, defining the schema resolver interface, request/result types, and dialect-specific extraction adapters.

## Files

| File | Responsibility |
|------|---------------|
| doc.go | Declares the queryaccess application package boundary |
| contracts.go | Defines SchemaResolver interface, RelationSchema, ColumnSchema, QueryAccessRequest, and QueryAccessResult |
| extract_tidb.go | Bridges TiDB infrastructure query access facts to domain types with admission computation |
| extract_tidb_test.go | Verifies TiDB extraction bridging: classification, admission, CTE permissions, mode normalization, and column usages |
| extract_postgresql.go | Bridges PostgreSQL infrastructure query access facts to domain types with admission computation |
| extract_postgresql_stub.go | Returns ErrPostgreSQLNotAvailable when built without the `postgresql` tag |

## Exports

- `SchemaResolver`
- `RelationSchema`
- `ColumnSchema`
- `QueryAccessRequest`
- `QueryAccessResult`
- `ExtractTiDBQueryAccess()`
- `AnalyzePostgreSQL()`

## Notes

- `SchemaResolver` is an optional interface; callers may pass `nil` when schema metadata is unavailable.
- `QueryAccessResult` wraps the domain `Result` for application-layer consumption.
- `QueryAccessRequest.Mode` is a string that the domain layer normalizes via `NormalizeMode`.
- `ExtractTiDBQueryAccess` computes admission from read classification: read_only → admissible, not_read_only → rejected, indeterminate → indeterminate.
- `AnalyzePostgreSQL` follows the same admission computation pattern as TiDB.
- CTE relations are marked with `PermissionRequired: false`; base tables and derived tables require permission.

## Dependencies
- Upstream: `internal/interfaces/*`
- Downstream: `internal/domain/queryaccess`, `internal/infrastructure/parser/tidb`, `internal/infrastructure/parser/postgresql`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
