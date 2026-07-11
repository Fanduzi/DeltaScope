# PostgreSQL Metadata Provider Module

PostgreSQL metadata provider used for optional metadata-aware DeltaScope audits against PostgreSQL.

## Files

| File | Responsibility |
|------|---------------|
| open.go | Formats PostgreSQL metadata connection DSNs and opens pgx stdlib `database/sql` handles |
| open_test.go | Verifies TCP and unix-socket DSN/address formatting helpers |
| provider.go | Loads normalized dialect, schema, instance-fact, table snapshot, and plain-`EXPLAIN` plan-estimate data from PostgreSQL catalogs and planner stats |
| provider_test.go | Verifies catalog-backed schema discovery, reltuples/statistics loading, PK constraint truth, and plain-`EXPLAIN` estimation without a live database |
| resolve_object.go | Resolves non-table database object metadata from PostgreSQL catalogs with schema-qualified ambiguity detection and privacy-safe attribute projection |
| resolve_object_test.go | Verifies object resolver behavior for all supported lookup types, statuses, sensitive attribute exclusion, and annotation target verification |
| query_access_resolver.go | Implements SchemaResolver for PostgreSQL by querying pg_catalog.pg_class, pg_namespace, and pg_attribute for relation kind and column listing |
| query_access_resolver_stub.go | Empty QueryAccessResolver struct for non-postgresql builds |

## Exports

- `DefaultConnectTimeout`
- `ConnectionConfig`
- `OpenDBContext(ctx, config)`
- `OpenDB(config)`
- `Provider`
- `NewProvider(db *sql.DB)`
- `Provider.DetectDialect(ctx)`
- `Provider.FindSchemasForTable(ctx, table)`
- `Provider.LoadPlanEstimate(ctx, statement)`
- `Provider.ResolveObject(ctx, dialect, request)`
- `QueryAccessResolver`
- `NewQueryAccessResolver(db *sql.DB)`
- `QueryAccessResolver.ResolveRelation(ctx, dialect, schema, name)`

## Dependencies
- Upstream: `internal/application/audit`, `internal/application/queryaccess`
- Downstream: `database/sql`, `github.com/jackc/pgx/v5`, `github.com/jackc/pgx/v5/stdlib`, `internal/domain/spec`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
