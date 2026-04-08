# PostgreSQL Metadata Provider Module

PostgreSQL metadata provider used for optional metadata-aware DeltaScope audits against PostgreSQL.

## Files

| File | Responsibility |
|------|---------------|
| open.go | Formats PostgreSQL metadata connection DSNs and opens pgx stdlib `database/sql` handles |
| open_test.go | Verifies TCP and unix-socket DSN/address formatting helpers |
| provider.go | Loads normalized dialect, schema, instance-fact, table snapshot, and plain-`EXPLAIN` plan-estimate data from PostgreSQL catalogs and planner stats |
| provider_test.go | Verifies catalog-backed schema discovery, reltuples/statistics loading, PK constraint truth, and plain-`EXPLAIN` estimation without a live database |

## Exports

- `ConnectionConfig`
- `OpenDB(config)`
- `Provider`
- `NewProvider(db *sql.DB)`
- `Provider.DetectDialect(ctx)`
- `Provider.FindSchemasForTable(ctx, table)`
- `Provider.LoadPlanEstimate(ctx, statement)`

## Dependencies
- Upstream: `internal/application/audit`
- Downstream: `database/sql`, `github.com/jackc/pgx/v5`, `github.com/jackc/pgx/v5/stdlib`, `internal/domain/spec`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
