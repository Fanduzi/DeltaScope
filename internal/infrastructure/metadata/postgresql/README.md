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
| query_access_resolver.go | Thin `*sql.DB`-backed SchemaResolver adapter delegating PostgreSQL catalog resolution to the private core |
| query_access_conn_resolver.go | Thin caller-owned `*sql.Conn`-backed SchemaResolver adapter for same-session metadata resolution; no pool fallback |
| query_access_resolver_core.go | Private stateless PostgreSQL catalog core: relation/column SQL, scanning, lookup errors, relkind mapping, and foreign-table fail-closed policy |
| query_access_resolver_test.go | Parameterized DB/Conn behavior contract plus foreign-table, ordering, lifecycle, and concrete-field coverage |
| query_access_resolver_stub.go | Empty QueryAccessResolver struct for non-postgresql builds |
| query_access_conn_resolver_stub.go | Empty QueryAccessConnResolver struct for non-postgresql builds |
| query_access_conn_resolver_test.go | Adapter-specific conn lifecycle and concrete-field tests |
| query_access_conn_resolver_integration_test.go | PG17 Docker integration: same-backend-PID proof |
| effect_identity_session.go | Session-pinned `*sql.Conn` wrapper; live resolution context capture (db/role/version/backend/search_path OIDs) |
| effect_identity_resolver.go | Facts-only `EffectIdentityResolver` adapter (operator/function/cast exact catalog lookup + dedicated COUNT(integer_one) catalog proof + TOCTOU gate) |
| effect_identity_resolver_test.go | Unit tests with fake pinned catalog (no live PG claim) |
| effect_identity_resolver_integration_test.go | Optional PG17 Docker integration (`-tags postgresql,integration`) |

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
- `PinnedSession` / `NewPinnedSessionFromConn` / `PinSession` / `ErrSessionNotPinned`
- `EffectIdentityAdapter` / `NewEffectIdentityAdapter` (facts only; implements `ControlledEffectIdentityResolver`)
- `QueryAccessConnResolver` / `NewQueryAccessConnResolver` (conn-backed SchemaResolver; no `*sql.DB` field)

## Effect identity (T7)

- Requires a **single pinned session** (`*sql.Conn` via `PinnedSession`). Do not run identity lookups on a multi-connection `*sql.DB` pool.
- Implements `ControlledEffectIdentityResolver`: `CaptureExecutionBoundContext` returns the pinned session's live resolution context so the application can set explicit Resolution on the request.
- Flow: application captures context → sets on request → capture live context → exact catalog lookup → re-capture live → `GateIdentityBatchAgainstLiveContext`.
- Returns catalog **facts** only (OIDs, volatility, cast method, stamped database/server). Never `Trusted`, admission, or free-text errors on public Result JSON.
- Unqualified names walk ordered `NamespaceSearchOIDs`; never invent `pg_catalog.<name>` without path/context.
- Explicit schema skips search_path ranking only; still requires full session/db/role/server binding via gates.
- Arity-0 functions (e.g. count(*)) bypass `hasUnresolvedTypeKind` star check — no type OIDs needed.
- Exact `COUNT(1)` uses a dedicated session-bound `pg_proc`/`pg_type` catalog lookup for the `count(any)` aggregate; it never fabricates an operand OID or falls back to generic function overload resolution.
- T8 owns version-scoped manifest proof and admission promotion. Runtime integration is validated against the repo's **PostgreSQL 17** compose image; T2 research covered 14–17 for the closed manifest set, not as a multi-version CI claim for this adapter.

## Dependencies
- Upstream: `internal/application/audit`, `internal/application/queryaccess`
- Downstream: `database/sql`, `github.com/jackc/pgx/v5`, `github.com/jackc/pgx/v5/stdlib`, `internal/domain/spec`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
