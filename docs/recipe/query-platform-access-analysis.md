# Query Platform Access Analysis

This recipe shows how to use DeltaScope's query access analysis to determine what database objects a SQL query touches and what permissions a caller needs.

## Problem

You have a SQL query and need to know:
- Is it read-only?
- What tables and columns does it access?
- What permissions does the caller need?

You want to enforce column-level access control without executing the query.

## Solution

Use the `queryaccess.Service.Analyze()` API to inspect the query and produce a structured access result.

> Note: This recipe uses the default offline path `appqa.Service.Analyze()`, which does not connect to a database. Function-bearing MySQL/TiDB queries remain `indeterminate` on this path. To have the SDK actually confirm MySQL/TiDB function queries (e.g. `COUNT(*)`), use the same-connection session API; see the "Confirming MySQL/TiDB Function Queries via a Same-Connection Session" section in the [Query Access Analysis Reference](../reference/query-access-analysis.md).

### Step 1: Analyze a Simple Query

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"

    appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
)

func main() {
    svc := &appqa.Service{}
    result, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
        SQL:     "SELECT id, name FROM users WHERE id = 1",
        Dialect: "mysql",
        Mode:    "strict",
    })
    if err != nil {
        panic(err)
    }

    j, _ := json.MarshalIndent(result.DomainResult, "", "  ")
    fmt.Println(string(j))
}
```

Output:

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
    {"table": "users", "column": "id", "usages": ["projection", "filter"]}
  ],
  "requirements": [
    {"object": "users", "privilege": "read_table"},
    {"object": "users.id", "privilege": "read_column"}
  ]
}
```

### Step 2: Interpret the Result

- `read_classification: read_only` — the query does not modify data.
- `admission: admissible` — the query is eligible for authorization.
- `relations` — the caller needs `read_table` on `users`.
- `requirements` — the caller needs `read_column` on `users.id`.

### Step 3: Handle Rejected Queries

```go
result, _ := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
    SQL:     "DELETE FROM users WHERE id = 1",
    Dialect: "mysql",
})

// result.DomainResult.ReadClassification == "not_read_only"
// result.DomainResult.Admission == "rejected"
```

Rejected queries should not proceed to authorization. Deny immediately.

### Step 4: Handle Indeterminate Queries

```go
result, _ := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
    SQL:     "SELECT * FROM users",
    Dialect: "mysql",
})

// result.DomainResult.ReadClassification == "indeterminate"
// result.DomainResult.Admission == "indeterminate"
// result.DomainResult.Unresolved contains [{reference: "*", reason: "schema_unavailable"}]
```

Indeterminate queries need metadata to resolve. Provide a `SchemaResolver`:

```go
type myResolver struct{}

func (r *myResolver) ResolveRelation(ctx context.Context, dialect, schema, name string) (appqa.RelationSchema, error) {
    // Look up the relation in your metadata store.
    return appqa.RelationSchema{
        Schema: schema,
        Name:   name,
        Kind:   "table",
        Columns: []appqa.ColumnSchema{
            {Name: "id", Ordinal: 1},
            {Name: "name", Ordinal: 2},
        },
    }, nil
}

result, _ := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
    SQL:            "SELECT * FROM users",
    Dialect:        "mysql",
    DefaultSchema:  "app",
    SchemaResolver: &myResolver{},
})
// Wildcard is now resolved into individual column references.
```

### Step 5: Use Projection-Only Mode

When the caller is trusted for filtering but should only see projected columns:

```go
result, _ := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
    SQL:     "SELECT id FROM users WHERE salary > 50000",
    Dialect: "mysql",
    Mode:    "projection_only",
})

// requirements only includes users.id, not users.salary
// warnings includes "projection_only_inference_risk"
```

The `projection_only_inference_risk` warning indicates that `salary` is used in the filter but not required for authorization. The authorization layer must accept this trade-off.

## Interpreting Results

| Field | What it means |
|---|---|
| `read_classification` | Whether the query is read-only, not read-only, or indeterminate. |
| `admission` | Whether the query is eligible for authorization. |
| `relations` | Tables/CTEs/derived tables the query reads. |
| `referenced_columns` | Columns with their usage contexts (projection, filter, join, etc.). |
| `outputs` | Output columns with source lineage. |
| `requirements` | Permissions the caller needs (`read_table`, `read_column`, or `indeterminate`). |
| `unresolved` | References that could not be resolved without metadata. |
| `warnings` | Non-fatal warnings (e.g., inference risk). |
| `reason_codes` | Machine-readable reasons for the classification. |

## Safe Defaults

- Empty mode defaults to `strict`.
- Without metadata, wildcards produce `indeterminate` classification.
- The default SDK/CLI/HTTP path does not connect to a database; function-bearing PostgreSQL, MySQL, and TiDB queries stay `indeterminate` on this default path.
- To promote a function-bearing query from `indeterminate` to `admissible`, you must use an explicit same-connection session SDK: `AnalyzePostgreSQLQueryAccessWithSession` for PostgreSQL, `AnalyzeMySQLTiDBQueryAccessWithSession` for MySQL/TiDB. Promotion is SDK-only; CLI, HTTP, and MCP do not open database connections.
- Treat `indeterminate` as denied in your authorization layer.

### Phase 1 Surface Matrix

This matrix describes the default behavior of each surface for function-bearing queries. For the full supported set and what stays `indeterminate`, see the [Query Access Analysis Reference](../reference/query-access-analysis.md).

| Dialect | Surface | Phase 1 aggregates/windows |
|---|---|---|
| PostgreSQL | Default SDK/CLI/HTTP | `indeterminate` (unchanged) |
| PostgreSQL | Trusted SDK session only | `admissible` for proven count/sum/avg/min/max/row_number/rank/dense_rank with complete requirements |
| MySQL | Default SDK/CLI/HTTP | `indeterminate` with `unknown_function_effect` (offline fail-closed) |
| MySQL | Explicit SDK session with `mysql-5.7`/`mysql-8.0`/`mysql-8.4` profile | `admissible` for proven `COUNT(*)`, direct-column `COUNT`/`SUM`/`AVG`/`MIN`/`MAX`; 8.x profiles also support ranking windows with direct partition+order columns |
| TiDB | Default SDK/CLI/HTTP | `indeterminate` with `unknown_function_effect` (offline fail-closed) |
| TiDB | Explicit SDK session with `tidb-8.5` profile | `admissible` for proven `COUNT(*)`, direct-column `COUNT`/`SUM`/`AVG`/`MIN`/`MAX`, and ranking windows with direct partition+order columns |

Do not call characterized-only function shapes supported. The promotion path is
SDK-only and does not add CLI/HTTP database connections or an MCP tool.

## What This Does NOT Do

- Does not authenticate callers.
- Does not evaluate grants or permissions.
- Does not enforce row-level security.
- Does not mask sensitive columns.
- Does not expand view definitions.
- Does not support MCP surface (deferred).

## Defense in Depth

**Warning**: Query access analysis supplements, but does not replace, database authorization. It is one layer in a defense-in-depth strategy. Always pair this analysis with:

- Proper authentication to verify caller identity
- Database-level grant evaluation and enforcement
- Row-level security for fine-grained access control
- Audit logging for compliance and monitoring

Do not rely solely on static analysis for security-critical authorization decisions.
