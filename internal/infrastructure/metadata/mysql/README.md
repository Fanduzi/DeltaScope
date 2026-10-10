# MySQL Metadata Provider Module

MySQL-protocol metadata provider used for optional metadata-aware DeltaScope audits against MySQL and TiDB.

## Files

| File | Responsibility |
|------|---------------|
| provider.go | Opens MySQL-compatible metadata connections and loads normalized dialect, schema, instance-fact, and target-table snapshot data from information schema, including preserved per-index cardinality facts. Instance facts carry explicit known bits: MySQL reads `innodb_page_size` and `innodb_large_prefix` (5.7), TiDB banners additionally read the `max-index-length` tidb-server config via `SHOW CONFIG`, verifying every matching row (one per tidb-server instance): disagreeing, unparsable, non-positive, or absent values stay unknown, while query/scan/iteration failures propagate as provider errors rather than degrading into gaps (issue #83 T04-B/R2) |
| provider_test.go | Verifies provider connection, dialect, normalization helpers, information_schema column nullability mapping, index-cardinality accumulation, and known-bit instance-fact loading (16K/4K/missing/zero/unparseable page sizes, TiDB `max-index-length` single/multi-row consistency in both orders, zero/invalid rows, query/scan/mid-iteration error propagation including errors trailing invalid values, MySQL banners never probing TiDB config) without a live database |
| provider_t06a4_test.go | Verifies the T06-A4 default-identity contract through real `Provider.LoadTableSnapshot` over injected driver rows: a NULL `COLUMN_DEFAULT` (omitted clause or explicit `DEFAULT NULL` — catalog-indistinguishable) records `HasDefault=false`/empty fields, stored literals `NULL`/`null`/`NuLl`/`'<nil>'`/`''`/`'0'` keep `HasDefault=true` with raw bytes and never set `DefaultIsNull`, rows stay isolated, and the temporal/default presence semantics are untouched |
| provider_t06a10_test.go | Verifies the T06-A10 provenance boundary through real `Provider.LoadTableSnapshot` over constructed catalog rows: `TABLES.AUTO_INCREMENT` lands on `snapshot.Options["auto_increment"]` and `COLUMNS.EXTRA` lands on `Column.AutoIncrement` as catalog observations — a NULL allocator keeps the key absent rather than fabricating a declaration |
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
