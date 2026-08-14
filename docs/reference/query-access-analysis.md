# Query Access Analysis Reference

Query access analysis inspects a SQL statement and determines what database objects a caller must be authorized to read. It does **not** authenticate callers, evaluate grants, enforce row-level security, or mask sensitive columns. It produces a structured result that an authorization layer consumes.

## Read Classification

Every analyzed statement receives one of three read classifications:

| Classification | Meaning |
|---|---|
| `read_only` | The statement contains no write operations, no locking clauses, and no functions that require runtime evaluation. |
| `not_read_only` | The statement contains at least one write operation (`INSERT`, `UPDATE`, `DELETE`, `FOR UPDATE`, `INTO OUTFILE`, DDL, etc.). |
| `indeterminate` | The read-only status could not be determined. Common causes: function calls (`NOW()`), unresolved wildcards (`SELECT *` without metadata), ambiguous column references, parse failures, or empty input. |

## Admission

Admission is derived from read classification:

| Admission | Condition |
|---|---|
| `admissible` | Classification is `read_only`. The statement is eligible for authorization checks. |
| `rejected` | Classification is `not_read_only`. The statement is not eligible. |
| `indeterminate` | Classification is `indeterminate`. Authorization cannot proceed without additional information. |

Admission is derived from read classification for all dialects.

## Modes

| Mode | Column requirements | Use case |
|---|---|---|
| `strict` (default) | Every referenced column requires `read_column` permission. | Full column-level access control. |
| `projection_only` | Only columns that appear in the SELECT list (output) require `read_column` permission. Filter, join, grouping, and ordering columns do not. | Projection-only authorization where the caller is trusted for filtering but not for seeing non-projected data. |

Both modes require every permission-bearing relation (`read_table`).

## Analysis Profiles

`QueryAccessRequest.AnalysisProfile` is optional and closed. Valid values are
the empty value (current behavior), `mysql-5.7`, `mysql-8.0`, `mysql-8.4`, and
`tidb-8.5`. A MySQL profile requires the MySQL dialect; `tidb-8.5` requires
TiDB. Unknown values return `ErrInvalidQueryAccessAnalysisProfile`; a dialect
mismatch returns `ErrQueryAccessAnalysisProfileDialectMismatch`.

Profiles are compatibility targets, not server identity or semantic proof.
The implementation does not query the actual server's `VERSION()` or SQL mode,
and a profile is not validated against the real server the connection points
at. The caller is responsible for ensuring the chosen profile matches the
actual MySQL/TiDB version and relevant SQL mode; a mismatched profile is still
analyzed under that profile's semantics and may diverge from real server
behavior.

A profile also does not change the default path. DeltaScope's default path does
not create a database connection on its own: the default SDK may accept a
caller-supplied `SchemaResolver` to resolve table names or expand wildcards,
but this does not enable MySQL/TiDB function-effect promotion; CLI and HTTP
use the unified online entry only when connection flags are present. The production semantic registry is enabled
for `mysql-5.7`, `mysql-8.0`, `mysql-8.4`, and `tidb-8.5`; each profile
supports `COUNT(*)`, direct-column `COUNT`/`SUM`/`AVG`/`MIN`/`MAX`, and the
8.x profiles additionally support `ROW_NUMBER`/`RANK`/`DENSE_RANK` with direct
partition and order columns. However, profiled function-bearing MySQL/TiDB
queries remain `indeterminate` on the default offline surface because the
default path does not connect to a database and does not enable function
semantic promotion. Promotion is available only through the explicit
same-connection SDK session (`AnalyzeOnlineQueryAccessWithSession`). The
profile is not included in result JSON.

### Inference Risk

Projection-only mode emits a `projection_only_inference_risk` warning when non-projected columns exist. This warns the caller that a user authorized only for projected columns could still infer data through WHERE, JOIN, or ORDER BY clauses. Use projection-only mode only when the authorization layer accepts this trade-off.

## Table Permissions

Both strict and projection-only modes require `read_table` permission for every base table and view. CTEs and derived tables do not require permission directly; their permission requirements come from the underlying physical tables and views they reference.

## Unbound Relations and Columns (PostgreSQL)

On PostgreSQL, unqualified base relations (those without a schema qualifier) are **execution-unbound**: the analyzer cannot determine which schema the relation resolves to at runtime because `search_path` is session-controlled. To prevent false permission proofs, these relations and their columns are marked as `unbound: true` in the result.

### What Unbound Means

- A relation with `unbound: true` will **never** produce `read_table` requirements.
- A column with `unbound: true` will **never** produce `read_column` requirements.
- An `unqualified_relation` entry appears in `unresolved` with `reason: unqualified_relation_blocked`.
- Classification becomes `indeterminate` and admission becomes `indeterminate`.

**Authorization layers must not grant access based on unbound relations or columns.** The `unbound` field is a signal that the permission requirement is not a reliable proof of what the query actually reads at runtime.

### When Unbound Is Set

| Scenario | Relations | Columns |
|---|---|---|
| `SELECT id FROM users` (unqualified, no resolver) | `users` → `unbound: true` | `users.id` → `unbound: true` (schema empty, unbound relations present) |
| `SELECT users.id FROM users` (qualified name, unbound relation) | `users` → `unbound: true` | `users.id` → `unbound: true` |
| `SELECT p.id, u.name FROM public.users p JOIN users u` (mixed) | `public.users` → not unbound; `users` → `unbound: true` | `users.id` (resolved via qualified entry, schema assigned) → not unbound; `users.name` (schema empty) → `unbound: true` |
| `SELECT id FROM public.users` (qualified) | `public.users` → not unbound | `public.users.id` → not unbound |
| MySQL/TiDB (any) | Never unbound | Never unbound |

### How the Analyzer Resolves Mixed Queries

When a query contains both qualified and unqualified references to the same table name (e.g., `public.users p JOIN users u`), the PostgreSQL parser resolves aliases to bare table names. Both `p.id` and `u.name` produce `table: "users"`. The analyzer uses the resolution state to distinguish:

- If a qualified entry exists in the resolution map, the column resolves through it (gets schema assigned).
- If only unbound entries exist, resolution is skipped and the column remains schema-less.

Columns that fail to resolve (column not found in the schema) produce an `unresolved` entry with `reason: column_not_found` and are also marked `unbound: true`.

## Fail-Closed Behavior

When analysis cannot determine the read classification or required permissions, the result is `indeterminate`. The authorization layer should treat `indeterminate` as denied by default. This means the analysis cannot fully enumerate what the query reads — it does **not** mean the query is safe, read-only, or that it will write data.

Common default `indeterminate` scenarios (see specific entries below):

- **Parse failure**: `read_classification: indeterminate`, `reason_codes: [parse_failure]`
- **Empty input**: `read_classification: indeterminate`, `reason_codes: [zero_statements]`
- **Unresolved wildcard**: `read_classification: indeterminate`, `unresolved: [{reference: "*", reason: schema_unavailable}]`
- **Ambiguous column**: `read_classification: indeterminate`, `unresolved: [{reference: "unqualified_column", reason: ambiguous_reference}]`

## Metadata Requirements

Without metadata, wildcards (`SELECT *`) remain unresolved and the classification becomes `indeterminate`. To resolve wildcards, provide a `SchemaResolver` that returns relation schemas (table name, columns with ordinal positions). With metadata:

- Wildcards expand into individual column references in ordinal order.
- Unqualified columns resolve when exactly one source relation contains the column.
- Views are detected and marked as `RelationView` kind.

## Dialect Differences

| Feature | MySQL/TiDB | PostgreSQL |
|---|---|---|
| Admission from classification | `read_only` → `admissible`, `not_read_only` → `rejected` | Same as MySQL/TiDB |
| CTE permission required | `false` | `false` |
| WHERE clause column usages | `projection` + `filter` | `projection` (WHERE columns get `filter` only if referenced in SELECT) |
| Ambiguous column handling | `indeterminate` with `ambiguous_reference` unresolved | `read_only` with unqualified column reference |
| `reason_codes` populated | Yes (`write_operation`, `function_call`, `parse_failure`, etc.) | Yes (`unproven_operator_effect`, `unproven_function_effect`, `unproven_cast_effect`, `unqualified_relation_blocked`, `identity_*` codes) |
| `unresolved` populated | Yes (wildcards, ambiguous references) | Yes (`unqualified_relation` entries for unqualified base relations) |

## Result Structure

```json
{
  "dialect": "mysql",
  "mode": "strict",
  "read_classification": "read_only",
  "admission": "admissible",
  "relations": [
    {"name": "users", "kind": "table", "permission_required": true}
  ],
  "referenced_columns": [
    {"table": "users", "column": "id", "usages": ["projection"]}
  ],
  "outputs": [
    {"name": "id", "sources": ["users.id"]}
  ],
  "requirements": [
    {"object": "users", "privilege": "read_table"},
    {"object": "users.id", "privilege": "read_column"}
  ],
  "unresolved": [],
  "warnings": [],
  "reason_codes": []
}
```

The result intentionally excludes raw SQL, literal values, passwords, and credentials.

## SDK Usage

```go
import (
    "context"
    "github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

result, err := deltascope.AnalyzeQueryAccess(context.Background(), deltascope.QueryAccessRequest{
    SQL:     "SELECT id, name FROM users",
    Dialect: deltascope.DialectMySQL,
    Mode:    deltascope.QueryAccessModeStrict,
})
```

## CLI Usage

Query access analysis is available through the CLI:

```bash
deltascope query-access analyze --sql "SELECT id, name FROM users WHERE id = 1" --dialect mysql
deltascope query-access analyze --file ./query.sql --dialect postgresql --mode projection_only
```

Exit codes: `0` = admissible, `1` = rejected, `2` = indeterminate, `3` = usage error.

## HTTP Usage

Query access analysis is available through the HTTP API:

```bash
curl -X POST http://localhost:8083/v1/query-access/analyze \
  -H "Content-Type: application/json" \
  -d '{"sql":"SELECT id FROM users","dialect":"mysql","mode":"strict","profile":"mysql-8.4"}'
```

The endpoint returns the same JSON structure as the SDK. Invalid mode returns
`400` with `invalid_mode`; invalid profiles return bounded `400` errors without
echoing the profile or SQL.

## Confirming MySQL/TiDB Function Queries via a Same-Connection Session

If your Go program is already connected to MySQL or TiDB, you can hand that connection to the SDK. The SDK can then confirm the real tables and columns, and — within the supported function set — produce a usable permission list. This is the only path that can promote a MySQL/TiDB function-bearing query from `indeterminate` to `admissible` in the SDK. The default SDK, CLI, and HTTP paths do not open a database connection and cannot do this; CLI and HTTP gain promotion only in online mode when connection flags are present.

Two things must be distinguished: the connection you pass in is used to identify the server and to resolve real relations and column metadata; the function semantics themselves come from the built-in, immutable semantic manifest selected by the observed server identity. The connection is not used to verify the server version or SQL mode, and it is not used to prove function semantics — the manifest supplies those, and the connection only grounds table and column names onto real objects.

Minimal example (the unified online entry; leave `Dialect` empty so observed identity selects the route):

```go
session, err := deltascope.NewOnlineQueryAccessSessionFromConn(ctx, conn)
result, err := deltascope.AnalyzeOnlineQueryAccessWithSession(ctx, session, deltascope.QueryAccessRequest{
    SQL:           "SELECT COUNT(*) FROM app.orders",
    Mode:          deltascope.QueryAccessModeStrict,
    DefaultSchema: "app",
})
```

Note the table is written as `app.orders`, with a schema qualifier. That is a hard requirement for promotion, explained next.

### Promotion Requires a Schema-Qualified Base Relation

Session promotion requires the referenced base table to be schema-qualified (e.g. `app.orders`, not `orders`). Even when the request carries `DefaultSchema`, an unqualified table name stays `indeterminate` and is not promoted. The reason is that promotion requires every function input to resolve strictly to a physical base-table column; an unqualified table name cannot be bound stably to a schema and therefore cannot form a reliable physical dependency.

### Positive `COUNT(*)` Example

With a session connection, a correct profile, schema-qualified tables, and complete column metadata, queries like the following can promote to `read_only + admissible`:

- `SELECT COUNT(*) FROM app.orders` (all four profiles)
- `SELECT SUM(amount) FROM app.orders` (direct-column `SUM`/`AVG`/`MIN`/`MAX`)
- `SELECT ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at) FROM app.orders` (8.x profiles, direct partition and order columns)

### What Still Returns `indeterminate`

Even with a session connection, the following remain `indeterminate`:

- An unqualified table name, e.g. `SELECT COUNT(*) FROM orders`, even when the request carries `DefaultSchema`.
- The query references views, CTEs, derived tables, or wildcards (`SELECT *`).
- It uses `DISTINCT`, `FILTER`, nested expressions, casts, explicit window frames, or named windows.
- A ranking window is missing `ORDER BY`, or its partition/order operands are not direct base-table columns.
- The function call is schema-qualified (`app.COUNT(*)`), quoted (`` `COUNT`(*) ``), or has noncanonical spacing (`COUNT (id)`).
- Metadata is incomplete, or the function/operator is outside the supported set for the chosen profile.
- Ranking-window functions on MySQL 5.7 (that profile has no native ranking-window support and stays deferred).

### `admissible` Is Not Authorization

A session analysis returning `admissible` only means static analysis obtained the complete known requirements: table and column names were resolved to real physical objects through your connection, and every function effect in the query is within the supported set of the identity-derived profile. It does **not**:

- Authorize execution of the query.
- Evaluate grants or permissions.
- Guarantee a later execution snapshot matches the current database state.
- Account for row-level security, masking, or SQL rewrite.
- Prove that the server's SQL mode matches the semantic manifest selected from the observed version (SQL mode is not verified).

In other words, `admissible` means "I can fully enumerate what this query reads," not "the caller is permitted to read it," and not "the query is safe."

### Connection Ownership and Safety

The session does not own or expose the connection you pass in. It constructs relation metadata resolution from that same connection, rejects an external `SchemaResolver`, and is the only SDK boundary that can construct the private semantic capability. The session does not expose catalog, manifest, connection, or credential details. The caller is responsible for closing the connection. The production registry is enabled for `mysql-5.7`, `mysql-8.0`, `mysql-8.4`, and `tidb-8.5`.

## MCP Deferral

MCP surface integration for query access analysis is deferred. The current MCP server exposes `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities` only.

## Trusted PostgreSQL SDK Path

The trusted PostgreSQL SDK path enables manifest-gated admission promotion for PostgreSQL queries. This path is available only when built with the `postgresql` build tag.

### Session Construction

```go
session, err := deltascope.NewOnlineQueryAccessSessionFromConn(ctx, conn)
```

- Accepts a caller-owned `*sql.Conn` (not `*sql.DB`)
- Validates connection liveness via `PingContext` and identifies the server
- Does not take ownership of the connection; caller must close it
- Returns `ErrOnlineQueryAccessCapabilityUnsupported` in non-postgresql builds

### Trusted Analysis

```go
result, err := deltascope.AnalyzeOnlineQueryAccessWithSession(ctx, session, req)
```

- Rejects nil context, nil session, mismatched non-empty request dialect, or non-nil `SchemaResolver`
- Creates all metadata, type, and effect-identity resolvers from the session's single `*sql.Conn`
- May return `read_only + admissible` when every effect is catalog-resolved and listed in the PG17 manifest
- Returns `ErrOnlineQueryAccessCapabilityUnsupported` in non-postgresql builds

### Admission Semantics

`admissible` means only that static analysis obtained complete known requirements and proved the bounded effect manifest against the supplied connection's catalog context. It does **not**:

- Authorize execution
- Evaluate grants or permissions
- Guarantee a later execution snapshot uses the same database state
- Account for row-level security, masking, or SQL rewrite

### Default Path

The default `AnalyzeQueryAccess` function (no session) remains fail-closed for PostgreSQL. CLI and HTTP use the default path unless connection flags are present, in which case online mode routes through the unified online entry and gains trusted promotion; MCP has no query-access tool.

### Phase 1 Pure-Effect Matrix

The following matrix is the exact Phase 1 contract. “Characterized” means the
shape is observed by tests only; it is not a supported or admissible function
allowlist.

| Dialect | Surface | Phase 1 aggregates/windows |
|---|---|---|
| PostgreSQL | Default SDK/CLI/HTTP | `indeterminate` (unchanged) |
| PostgreSQL | Unified online session only | `admissible` for proven `count`/`sum`/`avg`/`min`/`max`/`row_number`/`rank`/`dense_rank` with complete requirements |
| MySQL | Default SDK/CLI/HTTP | `indeterminate` with `unknown_function_effect` (offline fail-closed) |
| MySQL | Unified online session (identity-derived `mysql-5.7`/`mysql-8.0`/`mysql-8.4` profile) | `admissible` for proven `COUNT(*)`, direct-column `COUNT`/`SUM`/`AVG`/`MIN`/`MAX`; 8.x profiles also `ROW_NUMBER`/`RANK`/`DENSE_RANK` with direct partition+order columns |
| TiDB | Default SDK/CLI/HTTP | `indeterminate` with `unknown_function_effect` (offline fail-closed) |
| TiDB | Unified online session (identity-derived `tidb-8.5` profile) | `admissible` for proven `COUNT(*)`, direct-column `COUNT`/`SUM`/`AVG`/`MIN`/`MAX`, and `ROW_NUMBER`/`RANK`/`DENSE_RANK` with direct partition+order columns |

The trusted PostgreSQL subset requires exact catalog identity, session and
database context, complete strict-mode dependencies, and a PG17 manifest proof.
`DISTINCT`, `FILTER`, nested arguments, casts, frames, named windows, and
incomplete metadata remain `indeterminate`. MySQL/TiDB are not promoted by
syntax or function names.

The two proof roots differ: PostgreSQL uses catalog identity (the connection's
catalog resolves object identity); MySQL/TiDB uses the built-in, profile-bound
semantic manifest, where the connection only grounds schema-qualified table and
column names onto real physical objects and the function semantics come from the
profile, not from catalog identity. The two paths do not affect each other.

## Migrating from the Dialect-Specific Session APIs

The dialect-specific session types, constructors, and analyzers are deprecated
but remain exported and behavior-compatible:

| Deprecated | Replacement |
|---|---|
| `PostgreSQLQueryAccessSession` | `OnlineQueryAccessSession` |
| `MySQLTiDBQueryAccessSession` | `OnlineQueryAccessSession` |
| `NewPostgreSQLQueryAccessSessionFromConn` | `NewOnlineQueryAccessSessionFromConn` |
| `NewMySQLTiDBQueryAccessSessionFromConn` | `NewOnlineQueryAccessSessionFromConn` |
| `AnalyzePostgreSQLQueryAccessWithSession` | `AnalyzeOnlineQueryAccessWithSession` |
| `AnalyzeMySQLTiDBQueryAccessWithSession` | `AnalyzeOnlineQueryAccessWithSession` |

Leave `QueryAccessRequest.Dialect` empty so the observed server identity selects
the MySQL, TiDB, or PostgreSQL route; a non-empty dialect is only an optional
matching constraint. The connection remains caller-owned in both APIs. The
unified entry returns its own bounded `ErrOnlineQueryAccess...` sentinels, which
are not aliases of the dialect-specific errors — migrate `errors.Is` branches
to the generic sentinels (for example `ErrOnlineQueryAccessSessionUnavailable`,
`ErrOnlineQueryAccessDialectMismatch`, and
`ErrOnlineQueryAccessCapabilityUnsupported`).

## Defense in Depth

**Warning**: Query access analysis supplements, but does not replace, database authorization. It is one layer in a defense-in-depth strategy and must be paired with:

- **Authentication**: Verify caller identity before analysis.
- **Database authorization**: Enforce database-level grants and permissions independently.
- **Grant evaluation**: Check the produced requirements against the caller's granted permissions.
- **Row-level security**: Apply row filters independently of column-level analysis.
- **Audit logging**: Record analysis results and authorization decisions for compliance.

Do not rely solely on static analysis for security-critical authorization decisions.
