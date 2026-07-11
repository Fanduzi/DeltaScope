# MySQL Metadata Provider Module

MySQL-protocol metadata provider used for optional metadata-aware DeltaScope audits against MySQL and TiDB.

## Files

| File | Responsibility |
|------|---------------|
| provider.go | Opens MySQL-compatible metadata connections and loads normalized dialect, schema, instance-fact, and target-table snapshot data from information schema, including preserved per-index cardinality facts |
| provider_test.go | Verifies provider connection, dialect, normalization helpers, and index-cardinality accumulation behavior without a live database |
| query_access_resolver.go | Implements SchemaResolver for MySQL/TiDB by querying information_schema.tables and information_schema.columns for relation kind and column listing |
| query_access_resolver_test.go | Verifies resolver behavior for table/view existence, column listing, missing table, empty columns, and cancellation using a custom test driver |

## Exports

- `DefaultConnectTimeout`
- `ConnectionConfig`
- `OpenDBContext(ctx, config)`
- `OpenDB(config)`
- `Provider`
- `NewProvider(db *sql.DB)`
- `Provider.DetectDialect(ctx)`
- `Provider.FindSchemasForTable(ctx, table)`
- `QueryAccessResolver`
- `NewQueryAccessResolver(db *sql.DB)`
- `QueryAccessResolver.ResolveRelation(ctx, dialect, schema, name)`

## Dependencies
- Upstream: `internal/application/audit`, `internal/application/queryaccess`
- Downstream: `database/sql`, `net`, `github.com/go-sql-driver/mysql`, `internal/domain/spec`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
