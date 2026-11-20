# PostgreSQL Metadata Provider Module

PostgreSQL metadata provider used for optional metadata-aware DeltaScope audits against PostgreSQL.

## Files

The table snapshot's identity query uses the exact catalog namespace and
relation name across all relation kinds, with a built-in heap marker for D10. Its private `PGRelation` observation
can distinguish a sequence or index from a missing relation while the existing
shape loader still returns table shapes for its original relation kinds.
`pg_temp` remains unobserved because it is a session alias. The column query
reads `pg_class.reloftype` for D8 typed-table scope and uses system-qualified
comparisons for the member and scope binding.

| File | Responsibility |
|------|---------------|
| open.go | Formats PostgreSQL metadata connection DSNs and opens pgx stdlib `database/sql` handles |
| open_test.go | Verifies TCP and unix-socket DSN/address formatting helpers |
| column_state.go | Loads exact PG generation attributes, builtin type identity and actual `pg_type.typstorage`, with D7's attribute filter/type join bound to system operators, ordinary-table scope and PK membership, and live attached replica identity key slots, observed negative-attnum system members separately from user known-empty, with dependency-bound `pg_sequence` stable definitions; no name guessing, expression or issuance-state read; errors never publish partial proof |
| column_state_test.go | Pins quoted/duplicate/empty/unknown/scope facts, separate column/sequence types, int64 values, serial/domain controls, SQL privacy and query/scan/iteration/close error arms |
| instance_namespace_test.go | Pins actual pg_catalog version/settings and CASE/membership/row operators rather than search_path namesakes; unknown capability stays unknown |
| column_format_test.go | Pins observed type/system directories and empty-user preservation, exact quoted names, enum-derived lz4 known/false/unknown, system-qualified attribute/type comparisons and non-last-wins/error arms without privileged pg_config |
| table_owner.go | Observes exact PG17 table owner targets using role identity and sameness with no permission or membership reads |
| bulk_tablespace.go | Reads both exact named spaces and ordered role existence together using `pg_catalog.pg_tablespace` and `pg_catalog.pg_roles` |
| column_type.go | Reads a qualified PG17 type, its associated array when requested, and immediate pseudo-type facts using system catalogs |
| table_namespace.go | Observes exact PG17 table rename and schema destinations, including primary relation and row-type collisions |
| table_persistence.go | Reads observed PG17 persistence, direct publication and directional foreign-key preconditions for ordinary tables without reading dependency names |
| table_access_method.go | Reads the named or effective-default PG17 access method and compares it to the bound ordinary table’s actual method without invoking handlers |
| table_tablespace.go | Reads one exact PostgreSQL destination and compares it with the bound ordinary table’s current or database-default space. Excludes shared and mapped relations, with no location paths or privilege reads |
| foreign_key_keys.go | Reads live valid plain unique keys for one exact ordinary PG17 referenced table, including primary/immediate flags and ordered key slots |
| constraint_using_index.go | Observes one exact PostgreSQL index for PRIMARY KEY or UNIQUE attachment, including internal constraint ownership and key sort, collation and options facts |
| replica_identity.go | Resolves one real PostgreSQL replica-identity index candidate in its owning table schema, with uniqueness, predicate/expression presence and actual key nullability |
| relation_state_test.go | Verifies exact relation reads, absent and non-table kinds, unbound scopes, error propagation and JSON privacy |
| constraint_state.go | Loads target-bound `pg_constraint` names, kinds and validated flags using exact parameterized schema and table comparisons bound to `pg_catalog`. Read errors prevent publication of the directory, and duplicate rows stay visible |
| constraint_state_test.go | Pins complete/empty/quoted/duplicate catalog reads and query/scan/iteration errors without a live database |
| provider.go | Loads normalized dialect, schema, instance-fact (including private known/support lz4 facts from the existing pg_catalog.pg_settings enum read with system-qualified version and comparison operators), table snapshot, and plain-`EXPLAIN` plan-estimate data from PostgreSQL catalogs and planner stats |
| provider_test.go | Verifies catalog-backed schema discovery, reltuples/statistics loading, PK constraint truth, and plain-`EXPLAIN` estimation without a live database |
| resolve_object.go | Resolves non-table database object metadata from PostgreSQL catalogs with schema-qualified ambiguity detection and privacy-safe attribute projection |
| resolve_object_test.go | Verifies object resolver behavior for all supported lookup types, statuses, sensitive attribute exclusion, and annotation target verification |
| query_access_conn_resolver.go | Thin caller-owned `*sql.Conn`-backed SchemaResolver adapter for same-session metadata resolution; no pool fallback |
| query_access_resolver_core.go | Private stateless PostgreSQL catalog core behind the conn adapter: relation/column SQL, scanning, lookup errors, relkind mapping, and foreign-table fail-closed policy |
| query_access_resolver_test.go | Conn-only behavior contract (relation kinds, errors, query order, foreign-table fail-closed) plus lifecycle and concrete-field coverage |
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
- `PinnedSession` / `NewPinnedSessionFromConn` / `PinSession` / `ErrSessionNotPinned`
- `EffectIdentityAdapter` / `NewEffectIdentityAdapter` (facts only; implements `ControlledEffectIdentityResolver`)
- `QueryAccessConnResolver` / `NewQueryAccessConnResolver` (conn-backed SchemaResolver; no `*sql.DB` field)
- `QueryAccessConnResolver.ResolveRelation(ctx, dialect, schema, name)`

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
