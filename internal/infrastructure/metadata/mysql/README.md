# MySQL Metadata Provider Module

MySQL-protocol metadata provider used for optional metadata-aware DeltaScope audits against MySQL and TiDB.

## Files

| File | Responsibility |
|------|---------------|
| provider.go | Opens MySQL-compatible metadata connections and loads normalized dialect, schema, instance-fact, and target-table snapshot data from information schema |
| provider_test.go | Verifies provider connection, dialect, and normalization helpers without a live database |

## Exports

- `ConnectionConfig`
- `OpenDB(config)`
- `Provider`
- `NewProvider(db *sql.DB)`
- `Provider.DetectDialect(ctx)`
- `Provider.FindSchemasForTable(ctx, table)`

## Dependencies
- Upstream: `internal/application/audit`
- Downstream: `database/sql`, `net`, `github.com/go-sql-driver/mysql`, `internal/domain/spec`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
