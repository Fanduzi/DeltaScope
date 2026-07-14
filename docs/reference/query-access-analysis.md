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
| `reason_codes` populated | Yes (`write_operation`, `function_call`, `parse_failure`, etc.) | No (empty) |
| `unresolved` populated | Yes (wildcards, ambiguous references) | No (empty for most cases) |

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
  -d '{"sql":"SELECT id FROM users","dialect":"mysql","mode":"strict"}'
```

The endpoint returns the same JSON structure as the SDK. Invalid mode returns `400` with `invalid_mode` error code.

## MCP Deferral

MCP surface integration for query access analysis is deferred. The current MCP server exposes `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities` only.

## Defense in Depth

**Warning**: Query access analysis supplements, but does not replace, database authorization. It is one layer in a defense-in-depth strategy and must be paired with:

- **Authentication**: Verify caller identity before analysis.
- **Database authorization**: Enforce database-level grants and permissions independently.
- **Grant evaluation**: Check the produced requirements against the caller's granted permissions.
- **Row-level security**: Apply row filters independently of column-level analysis.
- **Audit logging**: Record analysis results and authorization decisions for compliance.

Do not rely solely on static analysis for security-critical authorization decisions.
