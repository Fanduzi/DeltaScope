# Public Package Module

Stable public package surface for library consumers.

## Files

| File | Responsibility |
|------|---------------|
| doc.go | Declares the public package placeholder |
| audit.go | Exposes the stable public audit API, optional metadata-provider hooks, and public result/request types |
| query_access.go | Exposes the stable public query access analysis API, schema resolver interface, and public result/request types |
| query_access_session.go | Exposes the opaque PostgreSQL session wrapper for trusted query access (postgresql build tag) |
| query_access_session_mysql_tidb.go | Exposes the opaque MySQL/TiDB session boundary for same-connection metadata resolution |
| query_access_session_stub.go | Provides PostgreSQL session stub when built without postgresql tag |
| query_access_session_integration_test.go | PG17 Docker integration for caller-owned trusted query access, including exact COUNT(1) boundaries and the foreign-table negative path |
| query_access_session_postgresql_recording_test.go | Recording-driver proof that trusted COUNT(1) analysis never sends user SQL to the database and foreign tables fail closed before COUNT catalog proof |
| version.go | Publishes the default semantic version and canonical ASCII logo |
| audit_test.go | Verifies the public audit API with defaults, overrides, multi-statement input, PostgreSQL request routing, and metadata-aware request plumbing |
| query_access_test.go | Verifies the public query access API with dialect routing, mode handling, JSON structure parity, and context cancellation |
| query_access_probe_boundary_no_leak_test.go | No-leak regression for the MySQL/TiDB builtin-identity probe boundary: asserts injected markers, identity facts, candidates, session/context, manifest, raw SQL, and `severity` are absent from the SDK result and JSON mapping |

## Exports

- `Audit(ctx, request)`
- `Request`
- `MetadataProvider`
- `Metadata`
- `InstanceFacts`
- `TableSnapshot`
- `Table`
- `Column`
- `Index`
- `Constraint`
- `Result`
- `StatementResult`
- `Explanation`
- `Finding`
- `FindingExplanation`
- `ExplanationMetadata`
- `Level`
  Public finding level type for `blocker`, `warning`, and `notice`
- `Summary`
- `Location`
- `Dialect`
  Includes `DialectPostgreSQL` for PostgreSQL request routing support
- `Verdict`
- `DefaultVersion`
- `Logo`
- `AnalyzeQueryAccess(ctx, request)`
  Performs query access analysis and returns read classification, admission, and permission requirements
- `QueryAccessRequest`
  Input for query access analysis with SQL, dialect, mode, optional analysis profile, default schema, and optional schema resolver
- `QueryAccessAnalysisProfile`
  Closed compatibility targets: empty, `mysql-5.7`, `mysql-8.0`, `mysql-8.4`, and `tidb-8.5`
- `ErrInvalidQueryAccessAnalysisProfile`
  Returned when a profile is outside the closed set
- `ErrQueryAccessAnalysisProfileDialectMismatch`
  Returned when a profile is selected for another dialect
- `QueryAccessResult`
  Output of query access analysis with structured JSON fields for dialect, mode, classification, admission, relations, columns, outputs, requirements, unresolved references, and warnings
- `QueryAccessMode`
  Controls which column references become requirements: `strict` or `projection_only`
- `QueryAccessReadClassification`
  Describes whether SQL is read-only: `read_only`, `not_read_only`, or `indeterminate`
- `QueryAccessAdmission`
  Describes whether SQL is eligible for authorization: `admissible`, `rejected`, or `indeterminate`
- `QueryAccessSchemaResolver`
  Optional interface for resolving relation metadata during analysis
- `QueryAccessRelationReference`
  Relation reference with `Unbound` field marking relations that must not produce physical requirements
- `QueryAccessColumnReference`
  Column reference with `Unbound` field indicating the column could not be resolved to a qualified schema.table.column
- `PostgreSQLQueryAccessSession`
  Opaque wrapper for a caller-owned `*sql.Conn` for trusted PostgreSQL query access analysis (postgresql build tag only)
- `NewPostgreSQLQueryAccessSessionFromConn(ctx, conn)`
  Creates an opaque session from a caller-owned `*sql.Conn` with context for liveness check; the session does not close the connection (postgresql build tag; stub returns `ErrPostgreSQLSessionNotAvailable` in non-postgresql builds)
- `AnalyzePostgreSQLQueryAccessWithSession(ctx, session, req)`
  Performs trusted PostgreSQL query access analysis using a caller-owned connection session; may return `read_only + admissible` when all effects are manifest-proven (postgresql build tag; stub returns `ErrPostgreSQLSessionNotAvailable` in non-postgresql builds)
- `MySQLTiDBQueryAccessSession`
  Opaque wrapper for a caller-owned MySQL/TiDB `*sql.Conn`; the connection remains caller-owned
- `NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)`
  Creates an opaque MySQL/TiDB session after a liveness check
- `AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, req)`
  Resolves relation metadata through the session connection, rejects external schema resolvers, and is the only SDK boundary that can use the private MySQL/TiDB semantic capability. The production builtin semantic registry is enabled for `mysql-5.7`, `mysql-8.0`, `mysql-8.4`, and `tidb-8.5`. Each profile supports `COUNT(*)`, direct-column `COUNT`/`SUM`/`AVG`/`MIN`/`MAX`; the 8.x profiles additionally support `ROW_NUMBER`/`RANK`/`DENSE_RANK` with direct partition and order columns. Default `AnalyzeQueryAccess`, CLI, and HTTP remain offline and fail-closed.

## Notes

- `Request` now carries top-level `Schema` and `MetadataProvider` fields so CLI, HTTP, and library consumers can opt into metadata-aware audits without changing the offline call shape.
- Public `MetadataProvider` stays minimal; standalone PostgreSQL index-owner resolution remains an internal optional seam behind the application metadata enrichment layer.
- `Result` and `StatementResult` expose an optional `Explanation` field for additive shared result context without changing verdict semantics. The built-in audit flow populates these aggregate fields whenever findings are present.
- `Result` now also exposes `Unsupported` (`[]spec.UnsupportedDetail`) and `Diagnostics` (`[]spec.Diagnostic`) arrays so library consumers can inspect structured partial-support and parser-error/unsupported-statement outcomes.
- `ErrUnsupportedStatement` is returned when unsupported statements are present, while still returning a populated `Result` for supported statements.
- `Finding` now exposes an optional `Explanation` field so library consumers can read structured per-finding `why`, `risk`, `suggestion`, and metadata-status notes directly.
- `DefaultVersion` is `v0.480.0`, matching the current repository release baseline for source builds.
- Release surface gates verify that `DefaultVersion` stays aligned with the release tag so source-built binaries do not drift behind published artifacts.

## Dependencies
- Upstream: external library consumers
- Downstream: `context`, `internal/application/audit`, `internal/application/queryaccess`, `internal/domain/queryaccess`, `internal/domain/report`, `internal/domain/rule`, `internal/domain/spec`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
