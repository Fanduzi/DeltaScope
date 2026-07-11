# Application Query Access Module

Application-level contracts for query access analysis, defining the schema resolver interface and request/result types.

## Files

| File | Responsibility |
|------|---------------|
| doc.go | Declares the queryaccess application package boundary |
| contracts.go | Defines SchemaResolver interface, RelationSchema, ColumnSchema, QueryAccessRequest, and QueryAccessResult |

## Exports

- `SchemaResolver`
- `RelationSchema`
- `ColumnSchema`
- `QueryAccessRequest`
- `QueryAccessResult`

## Notes

- `SchemaResolver` is an optional interface; callers may pass `nil` when schema metadata is unavailable.
- `QueryAccessResult` wraps the domain `Result` for application-layer consumption.
- `QueryAccessRequest.Mode` is a string that the domain layer normalizes via `NormalizeMode`.

## Dependencies
- Upstream: `internal/interfaces/*`
- Downstream: `internal/domain/queryaccess`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
