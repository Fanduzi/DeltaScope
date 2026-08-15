# Public Package Module

Stable public package surface for library consumers.

## Files

| File | Responsibility |
|------|---------------|
| doc.go | Declares the public package placeholder |
| audit.go | Exposes the stable public audit API, optional metadata-provider hooks, and public result/request types |
| query_access.go | Exposes the stable public query access analysis API, schema resolver interface, and public result/request types |
| query_access_online_session.go | Exposes the opaque unified online query access session, generic analysis entry, five bounded sentinel errors, and MySQL/TiDB/PG17 routing through shared private proof cores |
| query_access_online_capability.go | Holds the single private capability-target routing definition for the unified online entry (MySQL/TiDB always linked; PG17 delegated to the build-tag leaf) |
| query_access_online_capability_postgresql.go | Reports PostgreSQL capability as linked when built with the postgresql tag (postgresql build tag) |
| query_access_online_capability_notag.go | Reports PostgreSQL capability as not linked when built without the postgresql tag; unified PG17 fails closed with the capability sentinel |
| query_access_session.go | Exposes the opaque PostgreSQL session wrapper for trusted query access plus the shared private PG17 proof core used by the unified entry (postgresql build tag) |
| query_access_session_mysql_tidb.go | Exposes the opaque MySQL/TiDB session boundary for same-connection metadata resolution plus the shared private MySQL/TiDB proof core used by the unified entry |
| query_access_session_stub.go | Provides PostgreSQL session stub when built without postgresql tag |
| query_access_session_integration_test.go | Deprecated PG17 session construction, validation, caller ownership, offline/default behavior, and same-connection compatibility against Docker |
| query_access_session_postgresql_recording_test.go | Recording driver shared by unified tagged tests, plus deprecated PG17 foreign-table and bounded-failure no-leak compatibility |
| query_access_session_mysql_tidb_live_e2e_test.go | Docker-backed unified MySQL 5.7/8.0/8.4 and TiDB 8.5 semantic matrix, plus per-target deprecated-session identity and result equivalence |
| query_access_online_session_test.go | Verifies the unified online session contract: signatures, opacity, ownership, validation priority, generic sentinels, direct MySQL/TiDB semantic and ordered recording matrices (including exact MySQL 8.4 SUM, unknown-function, rejected-write, and parse-failure classification/admission/requirements/reasons), and no-execution/no-leak evidence |
| query_access_online_session_postgresql_tag_test.go | Verifies PostgreSQL 17 routing through the unified entry: exact COUNT(1) admission, excluded-shape and foreign-table fail-closed behavior, ordered recording-driver no-execution/no-leak, ownership, validation, and bounded failures (postgresql build tag) |
| query_access_online_session_postgresql_notag_test.go | Verifies the no-tag build keeps the unified symbols, fails an observed PostgreSQL target closed, and preserves legacy PostgreSQL stubs |
| query_access_online_session_postgresql_integration_test.go | Real PG17 same-backend-session proof, COUNT(1)/excluded-shape/parse-failure/foreign-table evidence, and unified-versus-legacy equivalence for the unified online entry (postgresql + integration build tags) |
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
- `OnlineQueryAccessSession` (canonical)
  Opaque unified wrapper for a caller-owned `*sql.Conn`; construction pings and identifies the server and derives a private routing target. Exposes no identity, product, profile, capability, connection state, exported field, or getter, and marshals as `{}`
- `NewOnlineQueryAccessSessionFromConn(ctx, conn)` (canonical)
  Creates a unified online session from a caller-owned `*sql.Conn`; never opens, pools, closes, or retries the connection. Nil context/connection, failed liveness, and identity failure map to `ErrOnlineQueryAccessSessionUnavailable`; a recognized but unsupported capability (including PostgreSQL 17 in a no-postgresql-tag source build) maps to `ErrOnlineQueryAccessCapabilityUnsupported`. Official DeltaScope binaries are built with the postgresql tag and route PostgreSQL 17 through the same-connection trusted proof
- `AnalyzeOnlineQueryAccessWithSession(ctx, session, req)` (canonical)
  Unified online analysis entry with a fixed validation priority (session/context; dialect mismatch; profile; resolver; linked capability; existing request validation). Empty request dialect uses observed identity; a non-empty dialect is a constraint that must match. Routes MySQL 5.7/8.0/8.4, TiDB 8.5, and PostgreSQL 17 (postgresql build tag) through their existing private proof cores; the no-tag source build keeps PostgreSQL fail-closed with the capability sentinel
- `ErrOnlineQueryAccessSessionUnavailable`
  Bounded sentinel: context/session unusable (nil input, failed liveness, failed identity)
- `ErrOnlineQueryAccessDialectMismatch`
  Bounded sentinel: non-empty request dialect did not match observed identity
- `ErrOnlineQueryAccessProfileNotAllowed`
  Bounded sentinel: caller analysis profile rejected; capability derives from observed identity
- `ErrOnlineQueryAccessSchemaResolverNotAllowed`
  Bounded sentinel: external schema resolver rejected; online proof uses the same-connection resolver
- `ErrOnlineQueryAccessCapabilityUnsupported`
  Bounded sentinel: recognized but unsupported capability (for example PostgreSQL 16, or PostgreSQL 17 in a no-postgresql-tag source build)
- `PostgreSQLQueryAccessSession` (deprecated; use `OnlineQueryAccessSession`)
  Opaque wrapper for a caller-owned `*sql.Conn` for trusted PostgreSQL query access analysis (postgresql build tag only); the unified `OnlineQueryAccessSession` routes PG17 through the same private proof core
- `NewPostgreSQLQueryAccessSessionFromConn(ctx, conn)` (deprecated; use `NewOnlineQueryAccessSessionFromConn`)
  Creates an opaque session from a caller-owned `*sql.Conn` with context for liveness check; the session does not close the connection (postgresql build tag; stub returns `ErrPostgreSQLSessionNotAvailable` in non-postgresql builds)
- `AnalyzePostgreSQLQueryAccessWithSession(ctx, session, req)` (deprecated; use `AnalyzeOnlineQueryAccessWithSession`)
  Performs trusted PostgreSQL query access analysis using a caller-owned connection session; may return `read_only + admissible` when all effects are manifest-proven (postgresql build tag; stub returns `ErrPostgreSQLSessionNotAvailable` in non-postgresql builds)
- `MySQLTiDBQueryAccessSession` (deprecated; use `OnlineQueryAccessSession`)
  Opaque wrapper for a caller-owned MySQL/TiDB `*sql.Conn`; the connection remains caller-owned
- `NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)` (deprecated; use `NewOnlineQueryAccessSessionFromConn`)
  Creates an opaque MySQL/TiDB session after a liveness check
- `AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, req)` (deprecated; use `AnalyzeOnlineQueryAccessWithSession`)
  Resolves relation metadata through the session connection, rejects external schema resolvers, and remains the dialect-specific SDK boundary for the private MySQL/TiDB semantic capability. The production builtin semantic registry is enabled for `mysql-5.7`, `mysql-8.0`, `mysql-8.4`, and `tidb-8.5`. Each profile supports `COUNT(*)`, direct-column `COUNT`/`SUM`/`AVG`/`MIN`/`MAX`; the 8.x profiles additionally support `ROW_NUMBER`/`RANK`/`DENSE_RANK` with direct partition and order columns. Default `AnalyzeQueryAccess` remains offline and fail-closed; CLI and HTTP online mode route the same capability through `AnalyzeOnlineQueryAccessWithSession`.

## Migrating from the Dialect-Specific Session APIs

The six dialect-specific compatibility identifiers are deprecated. Use the
unified online entry instead:

| Deprecated | Replacement |
|---|---|
| `PostgreSQLQueryAccessSession` | `OnlineQueryAccessSession` |
| `MySQLTiDBQueryAccessSession` | `OnlineQueryAccessSession` |
| `NewPostgreSQLQueryAccessSessionFromConn` | `NewOnlineQueryAccessSessionFromConn` |
| `NewMySQLTiDBQueryAccessSessionFromConn` | `NewOnlineQueryAccessSessionFromConn` |
| `AnalyzePostgreSQLQueryAccessWithSession` | `AnalyzeOnlineQueryAccessWithSession` |
| `AnalyzeMySQLTiDBQueryAccessWithSession` | `AnalyzeOnlineQueryAccessWithSession` |

The unified session is constructed from the same caller-owned `*sql.Conn`;
construction pings and identifies the server, and the caller keeps full
connection lifecycle control. Leave `QueryAccessRequest.Dialect` empty so the
observed server identity selects the MySQL, TiDB, or PostgreSQL route; a
non-empty dialect is only an optional matching constraint. The unified entry
returns its own bounded `ErrOnlineQueryAccess...` sentinels, so migrate
`errors.Is` checks from the dialect-specific sentinels (for example
`ErrMySQLTiDBQueryAccessSessionUnavailable`) to the generic ones rather than
expecting one-to-one error aliases.

## Query Access Test Ownership

The unified online-session suite owns exhaustive semantic and detailed-probe evidence, including ordered recording-driver probes. Deprecated API tests retain only source, stub, exact-error, validation-order, caller-ownership, privacy, and one per-target equivalence contract. The committed ownership ledger in `docs/plans/2026-08-15-query-access-test-ownership-consolidation-implementation.md` names every authorized deletion and its focused green evidence.

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
