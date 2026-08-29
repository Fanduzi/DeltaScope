# MySQL Metadata Provider Module

MySQL-protocol metadata provider used for optional metadata-aware DeltaScope audits against MySQL and TiDB.

## Files

| File | Responsibility |
|------|---------------|
| provider.go | Opens MySQL-compatible metadata connections and loads normalized dialect, schema, instance-fact, and target-table snapshot data from information schema, including preserved per-index cardinality facts |
| provider_test.go | Verifies provider connection, dialect, normalization helpers, information_schema column nullability mapping, and index-cardinality accumulation behavior without a live database |
| provider_integration_test.go | Verifies provider connection pool configuration and connection-leak behavior against a live MySQL service (build tag `integration`) |
| query_access_conn_resolver.go | Implements SchemaResolver for a caller-owned MySQL/TiDB `*sql.Conn` |
| query_access_resolver_test.go | Verifies conn resolver behavior for table/view kind, full column order, missing relation, empty columns, cancellation, and unsupported relation kind using a custom test driver |
| pure_effect_feasibility_test.go | Locks the STATIC Phase-1 pure-effect feasibility assumption for MySQL/TiDB; superseded by live probes in `builtin_effect_identity_live_probes_test.go` |
| pure_effect_defer_test.go | Locks the STATIC Phase-1 pure-effect deferral assumption; superseded by live probes which established the final DEFER dispositions |
| builtin_effect_identity_live_probes_test.go | Runs REAL Docker-backed MySQL 8.4 and TiDB 8.5 builtin-effect identity feasibility probes over a caller-owned `*sql.Conn`; locks independent live server evidence and the final DEFER dispositions (build tag `integration`) |
| builtin_semantic_live_probes_test.go | Runs independent aggregate and ranking-window evidence probes for MySQL 5.7, 8.0, 8.4, and TiDB 8.5 (build tag `integration`) |
| builtin_semantic_boundary_live_probes_test.go | Runs independent collision, qualification, quoting, spacing, comment, and SQL-mode boundary probes for each semantic profile (build tag `integration`) |

## Exports

- `DefaultConnectTimeout`
- `ConnectionConfig`
- `OpenDBContext(ctx, config)`
- `OpenDB(config)`
- `Provider`
- `NewProvider(db *sql.DB)`
- `Provider.DetectDialect(ctx)`
- `Provider.FindSchemasForTable(ctx, table)`
- `QueryAccessConnResolver`
- `NewQueryAccessConnResolver(conn)`
- `QueryAccessConnResolver.ResolveRelation(ctx, dialect, schema, name)`

## Dependencies
- Upstream: `internal/application/audit`, `internal/application/queryaccess`
- Downstream: `database/sql`, `net`, `github.com/go-sql-driver/mysql`, `internal/domain/spec`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
