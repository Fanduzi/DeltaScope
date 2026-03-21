# MySQL Metadata Provider Module

MySQL-protocol metadata provider used for optional metadata-aware DeltaScope audits against MySQL and TiDB.

## Files

| File | Responsibility |
|------|---------------|
| provider.go | Loads normalized instance facts and target-table snapshots from information schema, including row-format, collation, auto-increment, and approximate table-row facts |
| provider_test.go | Verifies provider helper normalization without a live database |

## Exports

- `Provider`
- `NewProvider(db *sql.DB)`

## Dependencies
- Upstream: `internal/application/audit`
- Downstream: `database/sql`, `github.com/go-sql-driver/mysql`, `internal/domain/spec`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
