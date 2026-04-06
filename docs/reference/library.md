# Go Library Reference

`pkg/deltascope` is the stable public Go API for embedding DeltaScope in tools, agents, and CI pipelines. It wraps the same audit engine used by the CLI and the HTTP service, so the same SQL, dialect, and config produce the same findings regardless of which surface you use.

## Import

```go
import "github.com/Fanduzi/DeltaScope/pkg/deltascope"
```

---

## Audit Function

```go
func Audit(ctx context.Context, request Request) (Result, error)
```

`Audit` is the single entry point for all audit operations. It parses, extracts, optionally enriches with metadata, and evaluates the SQL against the active policy, returning a structured `Result`.

---

## Request

```go
type Request struct {
    SQL              string            // SQL text to audit (required)
    Dialect          Dialect           // "mysql", "tidb", or "postgresql" (default: "mysql")
    ConfigPath       string            // path to YAML policy config (optional)
    Schema           string            // default schema for table resolution (optional)
    MetadataProvider MetadataProvider  // live metadata source (optional)
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `SQL` | Yes | One or more SQL statements to audit. |
| `Dialect` | No | `DialectMySQL`, `DialectTiDB`, or `DialectPostgreSQL`. Defaults to `DialectMySQL` when the zero value (`""`) is provided. PostgreSQL requests require a PG-capable build (for example, building with `-tags postgresql` or using `deltascope-pg`). |
| `ConfigPath` | No | Path to a YAML policy config file. When empty, the built-in default policy is used. |
| `Schema` | No | Default schema name used to resolve unqualified table references during metadata enrichment. |
| `MetadataProvider` | No | Supplies live instance facts and table snapshots. When `nil`, the audit runs in offline mode. |

---

## Result Types

### Result

```go
type Result struct {
    Verdict        Verdict           // "pass", "review", or "reject"
    Summary        Summary           // aggregate counts by level
    Statements     []StatementResult // per-statement findings
    GlobalFindings []Finding         // cross-statement findings from global rules
    Explanation    *Explanation      // optional shared result-level explanation emitted when the audit produces findings
}
```

### Summary

```go
type Summary struct {
    Statements int `json:"statements"`
    Blockers   int `json:"blockers"`
    Warnings   int `json:"warnings"`
    Notices    int `json:"notices"`
}
```

### StatementResult

```go
type StatementResult struct {
    Index         int          `json:"index"`
    Kind          string       `json:"kind"`
    RawSQL        string       `json:"raw_sql,omitempty"`
    NormalizedSQL string       `json:"normalized_sql,omitempty"`
    Findings      []Finding    `json:"findings,omitempty"`
    Explanation   *Explanation `json:"explanation,omitempty"` // emitted when this statement produces findings
    Impact        *Impact      `json:"impact,omitempty"`      // emitted for UPDATE/DELETE when DeltaScope can derive a conservative estimate
}
```

| Field | Description |
|-------|-------------|
| `Index` | 0-based position of this statement in the input. |
| `Kind` | Normalized statement family, currently `ddl` or `dml`. |
| `RawSQL` | Original SQL text of this statement. |
| `NormalizedSQL` | Whitespace-normalized form of the SQL; may be empty. |
| `Findings` | Rule findings for this statement. May be omitted from JSON when empty. |
| `Explanation` | Optional shared explanation for the statement-level result. The built-in audit flow populates it whenever the statement produces one or more findings. |
| `Impact` | Optional conservative DML impact estimate for `UPDATE` and `DELETE` statements. |

### DML Impact Estimation

When DeltaScope audits `UPDATE` or `DELETE`, it may add an `impact` object to each statement result. The object is conservative by design and reports `estimated_rows`, `estimated_ratio`, `risk_level`, `confidence`, `source`, `reason_codes`, and optional `notes`.

```json
{
  "raw_sql": "DELETE FROM users WHERE id = 42",
  "impact": {
    "estimated_rows": 1,
    "estimated_ratio": 0.0001,
    "risk_level": "low",
    "confidence": "high",
    "source": "metadata",
    "reason_codes": ["pk_equality"],
    "notes": ["refined with table statistics"]
  }
}
```

Offline mode uses SQL shape only. Metadata-aware mode may refine the estimate with read-only table statistics. DeltaScope does not execute the DML and does not run `EXPLAIN ANALYZE`. The payload itself is attached in the audit flow before rule evaluation.

### Impact

```go
type Impact struct {
    EstimatedRows  *int64            `json:"estimated_rows,omitempty"`
    EstimatedRatio *float64          `json:"estimated_ratio,omitempty"`
    RiskLevel      ImpactRisk        `json:"risk_level,omitempty"`
    Confidence     ImpactConfidence  `json:"confidence,omitempty"`
    Source         ImpactSource      `json:"source,omitempty"`
    ReasonCodes    []string          `json:"reason_codes,omitempty"`
    Notes          []string          `json:"notes,omitempty"`
}
```

| Field | Description |
|-------|-------------|
| `EstimatedRows` | Conservative estimated affected-row count when DeltaScope can derive one. |
| `EstimatedRatio` | Conservative estimated affected-row ratio relative to the target table when DeltaScope can derive one. |
| `RiskLevel` | `ImpactRiskLow`, `ImpactRiskMedium`, `ImpactRiskHigh`, or `ImpactRiskUnknown`. |
| `Confidence` | `ImpactConfidenceLow`, `ImpactConfidenceMedium`, or `ImpactConfidenceHigh`. |
| `Source` | Estimate origin, typically SQL shape only or metadata-refined output. |
| `ReasonCodes` | Stable reason tokens that explain the estimate path, such as `pk_equality`. |
| `Notes` | Optional free-form notes that clarify refinements, caveats, or missing metadata. |

### Finding

```go
type Finding struct {
    RuleID         string              `json:"rule_id"`
    Level          Level               `json:"level"`
    Message        string              `json:"message"`
    StatementIndex int                 `json:"statement_index,omitempty"`
    StatementKind  string              `json:"statement_kind,omitempty"`
    Location       *Location           `json:"location,omitempty"`
    Suggestion     string              `json:"suggestion,omitempty"`
    Metadata       map[string]any      `json:"metadata,omitempty"`
    Explanation    *FindingExplanation `json:"explanation,omitempty"`
}
```

| Field | Description |
|-------|-------------|
| `RuleID` | Stable rule identifier, e.g. `dml.where.require`. |
| `Level` | `LevelBlocker`, `LevelWarning`, or `LevelNotice`. |
| `Message` | Human-readable description of the issue. |
| `StatementIndex` | 0-based index of the statement that produced this finding when that context is attached. |
| `StatementKind` | Kind of the statement that produced this finding when that context is attached. |
| `Location` | Source position in the original SQL text; `nil` when unavailable. |
| `Suggestion` | Recommended corrective action; empty string when not available. |
| `Metadata` | Additional key/value context specific to the rule; `nil` when not present. |
| `Explanation` | Optional structured explanation with `summary`, `why`, `risk`, `suggestion`, and metadata-availability notes. |

### Explanation

```go
type Explanation struct {
    Summary string   `json:"summary,omitempty"`
    Reasons []string `json:"reasons,omitempty"`
}
```

### FindingExplanation

```go
type FindingExplanation struct {
    Summary    string               `json:"summary,omitempty"`
    Why        string               `json:"why,omitempty"`
    Risk       string               `json:"risk,omitempty"`
    Suggestion string               `json:"suggestion,omitempty"`
    Metadata   *ExplanationMetadata `json:"metadata,omitempty"`
}
```

### ExplanationMetadata

```go
type ExplanationMetadata struct {
    Status string `json:"status,omitempty"`
    Note   string `json:"note,omitempty"`
}
```

### Location

```go
type Location struct {
    Line   int `json:"line,omitempty"`
    Column int `json:"column,omitempty"`
}
```

---

## Constants

### Dialect

```go
const (
    DialectMySQL Dialect = "mysql"
    DialectTiDB  Dialect = "tidb"
    DialectPostgreSQL Dialect = "postgresql"
)
```

### Verdict

```go
const (
    VerdictPass   Verdict = "pass"
    VerdictReview Verdict = "review"
    VerdictReject Verdict = "reject"
)
```

### ImpactSource

```go
const (
    ImpactSourceShape    ImpactSource = "shape"
    ImpactSourceMetadata ImpactSource = "metadata"
    ImpactSourcePlan     ImpactSource = "plan"
)
```

### ImpactRisk

```go
const (
    ImpactRiskLow     ImpactRisk = "low"
    ImpactRiskMedium  ImpactRisk = "medium"
    ImpactRiskHigh    ImpactRisk = "high"
    ImpactRiskUnknown ImpactRisk = "unknown"
)
```

### ImpactConfidence

```go
const (
    ImpactConfidenceLow    ImpactConfidence = "low"
    ImpactConfidenceMedium ImpactConfidence = "medium"
    ImpactConfidenceHigh   ImpactConfidence = "high"
)
```

| Value | Meaning |
|-------|---------|
| `VerdictPass` | All statements passed with no blocker- or warning-level findings. Notice-only results also remain `pass`. |
| `VerdictReview` | One or more warnings were found and no blockers were found. |
| `VerdictReject` | One or more blocker-level findings were found. |

### Level

```go
const (
    LevelBlocker Level = "blocker"
    LevelWarning Level = "warning"
    LevelNotice  Level = "notice"
)
```

---

## Usage Examples

### Basic Audit

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

func main() {
    result, err := deltascope.Audit(context.Background(), deltascope.Request{
        SQL:     "DELETE FROM users",
        Dialect: deltascope.DialectMySQL,
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Verdict:", result.Verdict)
    for _, stmt := range result.Statements {
        for _, f := range stmt.Findings {
            fmt.Printf("[%s] %s: %s\n", f.Level, f.RuleID, f.Message)
        }
    }
}
```

### With Custom Policy Config

```go
result, err := deltascope.Audit(ctx, deltascope.Request{
    SQL:        sql,
    Dialect:    deltascope.DialectMySQL,
    ConfigPath: "./deltascope.yaml",
})
if err != nil {
    log.Fatal(err)
}
```

### With Metadata Provider

```go
result, err := deltascope.Audit(ctx, deltascope.Request{
    SQL:              sql,
    Dialect:          deltascope.DialectMySQL,
    Schema:           "app",
    MetadataProvider: myProvider,
})
if err != nil {
    log.Fatal(err)
}
```

---

## MetadataProvider Interface

```go
type MetadataProvider interface {
    LoadInstanceFacts(ctx context.Context, dialect Dialect, schema string) (*InstanceFacts, error)
    LoadTableSnapshot(ctx context.Context, dialect Dialect, schema string, table string) (*TableSnapshot, error)
}
```

Implement `MetadataProvider` to supply live schema and instance facts from a database connection. Pass the implementation in `Request.MetadataProvider` to enable metadata-aware rules.

- `LoadInstanceFacts` is called once per audit request and returns server-level configuration values such as the default charset, InnoDB row format, and adaptive hash index setting.
- `LoadTableSnapshot` is called for each table referenced in the SQL and returns a normalized snapshot of the current table definition, including columns, indexes, and table options.

When `MetadataProvider` is `nil`, all metadata-aware rules are skipped and the audit runs in offline mode. Offline audits are always safe to run without a database connection.

---

## Error Handling

`Audit` returns a non-nil error when the request cannot be processed due to an input or configuration problem. These are not audit findings — they represent failures that prevent the audit from running at all.

```go
result, err := deltascope.Audit(ctx, req)
if err != nil {
    // handle input/config errors — these are not audit findings
    log.Fatal("audit setup failed:", err)
}

// result.Verdict conveys the audit outcome
switch result.Verdict {
case deltascope.VerdictReject:
    os.Exit(1)
case deltascope.VerdictReview:
    // log warnings and proceed, or exit 1 depending on your policy
case deltascope.VerdictPass:
    // all clear
}
```

**Error conditions:**

| Condition | Cause |
|-----------|-------|
| Empty SQL | `Request.SQL` is an empty string |
| Unknown dialect | `Request.Dialect` is not `"mysql"` or `"tidb"` |
| Config load failure | `Request.ConfigPath` points to a file that cannot be read or contains invalid YAML |

These error conditions correspond to CLI exit code `2`. Runtime or internal failures correspond to exit code `3`.

---

## Consistency Guarantee

The library, CLI, and HTTP service all run the same audit engine. Given the same `SQL`, `Dialect`, and `ConfigPath`:

- `deltascope.Audit(...)` (library)
- `deltascope audit --format json ...` (CLI)
- `POST /v1/audit` (HTTP)

produce the same audit findings and core `Result` fields. The library's `Result` type serializes to the same JSON shape as the HTTP response body. The CLI `--format json` output uses the same `Result` payload and adds a top-level `context` object describing the CLI audit run.
