# Application Query Access Module

Application-level contracts for query access analysis, defining the schema resolver interface, request/result types, dialect-specific extraction adapters, and metadata-backed resolution.

## Files

| File | Responsibility |
|------|---------------|
| doc.go | Declares the queryaccess application package boundary |
| contracts.go | Defines SchemaResolver interface, RelationSchema, ColumnSchema, QueryAccessRequest, and QueryAccessResult |
| extract_tidb.go | Bridges TiDB infrastructure query access facts to domain types with admission computation |
| extract_tidb_test.go | Verifies TiDB extraction bridging: classification, admission, CTE permissions, mode normalization, and column usages |
| extract_postgresql.go | Bridges PostgreSQL infrastructure query access facts to domain types with admission computation |
| extract_postgresql_stub.go | Returns ErrPostgreSQLNotAvailable when built without the `postgresql` tag |
| service.go | Orchestrates query access analysis: extraction by dialect, optional metadata resolution, sorting, and validation |
| resolve.go | Implements metadata-backed resolution: request-scoped caching, wildcard expansion, alias resolution, column disambiguation, view detection, and output lineage enrichment |
| resolve_test.go | Verifies resolution logic with a fake resolver: schema defaulting, cache deduplication, qualified/unqualified columns, missing metadata, cancellation, star expansion, views, CTEs, derived tables, aliases, output lineage |
| service_test.go | Verifies service integration: offline mode, metadata mode, mode normalization, classification preservation, wildcard expansion |

## Exports

- `SchemaResolver`
- `RelationSchema`
- `ColumnSchema`
- `QueryAccessRequest`
- `QueryAccessResult`
- `Service`
- `ExtractTiDBQueryAccess()`
- `AnalyzePostgreSQL()`
- `ResolveMetadata()` (testing)

## Notes

- `SchemaResolver` is an optional interface; callers may pass `nil` when schema metadata is unavailable.
- `QueryAccessResult` wraps the domain `Result` for application-layer consumption.
- `QueryAccessRequest.Mode` is a string that the domain layer normalizes via `NormalizeMode`.
- `ExtractTiDBQueryAccess` computes admission from read classification: read_only → admissible, not_read_only → rejected, indeterminate → indeterminate.
- `AnalyzePostgreSQL` follows the same admission computation pattern as TiDB.
- CTE relations are marked with `PermissionRequired: false`; base tables and derived tables require permission.
- `Service.Analyze` routes by dialect, applies optional metadata resolution, sorts output, and validates the result.
- Resolution caches relation schemas per request (key: schema.name). CTEs and derived tables bypass resolution.
- Views are detected from metadata and marked as `RelationView` kind without definition expansion.
- Unqualified columns resolve only when exactly ONE source relation has the column.
- Wildcards expand in deterministic ordinal order when metadata is available.

## Dependencies
- Upstream: `internal/interfaces/*`
- Downstream: `internal/domain/queryaccess`, `internal/infrastructure/parser/tidb`, `internal/infrastructure/parser/postgresql`, `internal/infrastructure/metadata/mysql`, `internal/infrastructure/metadata/postgresql`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
