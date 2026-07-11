# Application Query Access Module

Application-level contracts for query access analysis, defining the schema resolver interface, request/result types, and TiDB extraction adapter.

## Files

| File | Responsibility |
|------|---------------|
| doc.go | Declares the queryaccess application package boundary |
| contracts.go | Defines SchemaResolver interface, RelationSchema, ColumnSchema, QueryAccessRequest, and QueryAccessResult |
| extract_tidb.go | Bridges TiDB infrastructure query access facts to domain types with admission computation |
| extract_tidb_test.go | Verifies TiDB extraction bridging: classification, admission, CTE permissions, mode normalization, and column usages |

## Exports

- `SchemaResolver`
- `RelationSchema`
- `ColumnSchema`
- `QueryAccessRequest`
- `QueryAccessResult`
- `ExtractTiDBQueryAccess()`

## Notes

- `SchemaResolver` is an optional interface; callers may pass `nil` when schema metadata is unavailable.
- `QueryAccessResult` wraps the domain `Result` for application-layer consumption.
- `QueryAccessRequest.Mode` is a string that the domain layer normalizes via `NormalizeMode`.
- `ExtractTiDBQueryAccess` computes admission from read classification: read_only → admissible, not_read_only → rejected, indeterminate → indeterminate.
- CTE relations are marked with `PermissionRequired: false`; base tables and derived tables require permission.

## Dependencies
- Upstream: `internal/interfaces/*`
- Downstream: `internal/domain/queryaccess`, `internal/infrastructure/parser/tidb`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
