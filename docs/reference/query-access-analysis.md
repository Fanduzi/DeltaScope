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
Default SDK, CLI, and HTTP analysis remains offline. The production semantic
registry is enabled for `mysql-5.7`, `mysql-8.0`, `mysql-8.4`, and `tidb-8.5`;
each profile supports `COUNT(*)`, direct-column `COUNT`/`SUM`/`AVG`/`MIN`/`MAX`,
and the 8.x profiles additionally support `ROW_NUMBER`/`RANK`/`DENSE_RANK` with
direct partition and order columns. However, profiled function-bearing
MySQL/TiDB queries remain `indeterminate` on the default offline surface
because the default `Service` has no schema resolver or live connection.
Promotion is available only through the explicit same-connection SDK session
(`AnalyzeMySQLTiDBQueryAccessWithSession`). The profile is not included in
result JSON.

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

When analysis cannot determine the read classification or required permissions, the result is `indeterminate`. The authorization layer should treat `indeterminate` as denied by default. Specific fail-closed scenarios:

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

## MySQL/TiDB Session Boundary

The explicit SDK session API accepts a caller-owned `*sql.Conn`:

```go
session, err := deltascope.NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)
result, err := deltascope.AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, deltascope.QueryAccessRequest{
    SQL:             "SELECT id FROM app.users",
    Dialect:         deltascope.DialectMySQL,
    AnalysisProfile: deltascope.QueryAccessAnalysisProfileMySQL84,
    DefaultSchema:   "app",
})
```

The session does not own or expose the connection. It constructs relation
metadata resolution from that same connection, rejects an external
`SchemaResolver`, and is the only SDK boundary that can construct the private
semantic capability. The production registry is enabled for `mysql-5.7`,
`mysql-8.0`, `mysql-8.4`, and `tidb-8.5`. When a profiled query has complete
physical metadata through the session connection, proven entries promote to
`read_only + admissible`. The session does not expose catalog, manifest,
connection, or credential details.

## MCP Deferral

MCP surface integration for query access analysis is deferred. The current MCP server exposes `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities` only.

## Trusted PostgreSQL SDK Path

The trusted PostgreSQL SDK path enables manifest-gated admission promotion for PostgreSQL queries. This path is available only when built with the `postgresql` build tag.

### Session Construction

```go
session, err := deltascope.NewPostgreSQLQueryAccessSessionFromConn(ctx, conn)
```

- Accepts a caller-owned `*sql.Conn` (not `*sql.DB`)
- Validates connection liveness via `PingContext`
- Does not take ownership of the connection; caller must close it
- Returns `ErrPostgreSQLSessionNotAvailable` in non-postgresql builds

### Trusted Analysis

```go
result, err := deltascope.AnalyzePostgreSQLQueryAccessWithSession(ctx, session, req)
```

- Rejects nil context, nil session, non-PostgreSQL dialect, or non-nil `SchemaResolver`
- Creates all metadata, type, and effect-identity resolvers from the session's single `*sql.Conn`
- May return `read_only + admissible` when every effect is catalog-resolved and listed in the PG17 manifest
- Returns `ErrPostgreSQLSessionNotAvailable` in non-postgresql builds

### Admission Semantics

`admissible` means only that static analysis obtained complete known requirements and proved the bounded effect manifest against the supplied connection's catalog context. It does **not**:

- Authorize execution
- Evaluate grants or permissions
- Guarantee a later execution snapshot uses the same database state
- Account for row-level security, masking, or SQL rewrite

### Default Path

The default `AnalyzeQueryAccess` function (no session) remains fail-closed for PostgreSQL. CLI, HTTP, and MCP surfaces continue to use the default path and do not gain trusted promotion.

### Phase 1 Pure-Effect Matrix

The following matrix is the exact Phase 1 contract. “Characterized” means the
shape is observed by tests only; it is not a supported or admissible function
allowlist.

| Dialect | Surface | Phase 1 aggregates/windows |
|---|---|---|
| PostgreSQL | Default SDK/CLI/HTTP | `indeterminate` (unchanged) |
| PostgreSQL | Trusted SDK session only | `admissible` for proven `count`/`sum`/`avg`/`min`/`max`/`row_number`/`rank`/`dense_rank` with complete requirements |
| MySQL | Default SDK/CLI/HTTP | `indeterminate` with `unknown_function_effect` (offline fail-closed) |
| MySQL | Explicit SDK session (`AnalyzeMySQLTiDBQueryAccessWithSession`) with `mysql-5.7`/`mysql-8.0`/`mysql-8.4` profile | `admissible` for proven `COUNT(*)`, direct-column `COUNT`/`SUM`/`AVG`/`MIN`/`MAX`; 8.x profiles also `ROW_NUMBER`/`RANK`/`DENSE_RANK` with direct partition+order columns |
| TiDB | Default SDK/CLI/HTTP | `indeterminate` with `unknown_function_effect` (offline fail-closed) |
| TiDB | Explicit SDK session with `tidb-8.5` profile | `admissible` for proven `COUNT(*)`, direct-column `COUNT`/`SUM`/`AVG`/`MIN`/`MAX`, and `ROW_NUMBER`/`RANK`/`DENSE_RANK` with direct partition+order columns |

The trusted PostgreSQL subset requires exact catalog identity, session and
database context, complete strict-mode dependencies, and a PG17 manifest proof.
`DISTINCT`, `FILTER`, nested arguments, casts, frames, named windows, and
incomplete metadata remain `indeterminate`. MySQL/TiDB are not promoted by
syntax or function names.

## Defense in Depth

**Warning**: Query access analysis supplements, but does not replace, database authorization. It is one layer in a defense-in-depth strategy and must be paired with:

- **Authentication**: Verify caller identity before analysis.
- **Database authorization**: Enforce database-level grants and permissions independently.
- **Grant evaluation**: Check the produced requirements against the caller's granted permissions.
- **Row-level security**: Apply row filters independently of column-level analysis.
- **Audit logging**: Record analysis results and authorization decisions for compliance.

Do not rely solely on static analysis for security-critical authorization decisions.
