# Go 库参考

`pkg/deltascope` 是 DeltaScope 面向工具、Agent 和 CI 流水线的稳定公开 Go API。它封装了与 CLI 和 HTTP 服务完全相同的审计引擎，因此相同的 SQL、方言和配置在任意使用方式下都会产生相同的审计结果。

## 导入

```go
import "github.com/Fanduzi/DeltaScope/pkg/deltascope"
```

---

## Audit 函数

```go
func Audit(ctx context.Context, request Request) (Result, error)
```

`Audit` 是所有审计操作的唯一入口。它对 SQL 进行解析、提取，可选地通过元数据进行增强，然后依据当前策略对各语句进行规则评估，最终返回结构化的 `Result`。

---

## Request

```go
type Request struct {
    SQL              string            // 待审计的 SQL 文本（必填）
    Dialect          Dialect           // "mysql"、"tidb" 或 "postgresql"（默认："mysql"）
    ConfigPath       string            // YAML 策略配置文件路径（可选）
    Schema           string            // 表名解析时使用的默认 Schema（可选）
    MetadataProvider MetadataProvider  // 实时元数据来源（可选）
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `SQL` | 是 | 待审计的一条或多条 SQL 语句。 |
| `Dialect` | 否 | `DialectMySQL`、`DialectTiDB` 或 `DialectPostgreSQL`。传入零值（`""`）时默认为 `DialectMySQL`。PostgreSQL 请求需要使用 PG-capable 构建。在公开 release 中，已收敛的 `linux/amd64` 主 archive 已直接提供该能力；`deltascope-pg` 仅继续作为 CLI 兼容/过渡 artifact 保留。 |
| `ConfigPath` | 否 | YAML 策略配置文件的路径。为空时使用内置默认策略。 |
| `Schema` | 否 | 元数据增强阶段用于解析非限定表名的默认 Schema 名称。 |
| `MetadataProvider` | 否 | 提供实时实例信息和表快照。为 `nil` 时，审计在离线模式下运行。 |

---

## Result 类型

### Result

```go
type Result struct {
    Verdict        Verdict           // "pass"、"review" 或 "reject"
    Summary        Summary           // 按级别汇总的计数
    Statements     []StatementResult // 各语句的审计发现
    GlobalFindings []Finding         // 全局规则（跨语句检查）产生的发现
    Explanation    *Explanation      // 审计产生发现时可选的结果级共享解释
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
    Explanation   *Explanation `json:"explanation,omitempty"` // 该语句产生 finding 时输出
    Impact        *Impact      `json:"impact,omitempty"`      // DeltaScope 能为 UPDATE/DELETE 给出保守估计时输出
}
```

| 字段 | 说明 |
|------|------|
| `Index` | 该语句在输入中的位置，从 0 开始计数。 |
| `Kind` | 规范化后的语句族，目前为 `ddl` 或 `dml`。 |
| `RawSQL` | 该语句的原始 SQL 文本。 |
| `NormalizedSQL` | 空白符规范化后的 SQL，可能为空字符串。 |
| `Findings` | 该语句的规则发现。为空时 JSON 中可能省略该字段。 |
| `Explanation` | 可选的语句级共享解释。内置审计流程在该语句产生一条或多条 finding 时会填充它。 |
| `Impact` | `UPDATE` / `DELETE` 语句的可选保守影响估计。 |

### DML 影响估算

当 DeltaScope 审计 `UPDATE` 或 `DELETE` 时，可能会在每条语句结果上附加一个 `impact` 对象。这个对象以保守估算为原则，包含 `estimated_rows`、`estimated_ratio`、`risk_level`、`confidence`、`source`、`reason_codes`，以及可选的 `notes`。

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

离线模式只使用 SQL 形状做估算。元数据感知模式可以基于只读表统计信息进一步收敛估算。DeltaScope 不会执行 DML，也不会运行 `EXPLAIN ANALYZE`。这个 payload 会在规则评估前附加到语句结果上。

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

| 字段 | 说明 |
|------|------|
| `EstimatedRows` | DeltaScope 能推导出时的保守影响行数估计。 |
| `EstimatedRatio` | DeltaScope 能推导出时，相对于目标表的保守影响比例估计。 |
| `RiskLevel` | `ImpactRiskLow`、`ImpactRiskMedium`、`ImpactRiskHigh` 或 `ImpactRiskUnknown`。 |
| `Confidence` | `ImpactConfidenceLow`、`ImpactConfidenceMedium` 或 `ImpactConfidenceHigh`。 |
| `Source` | 估算来源，通常是仅基于 SQL 形状，或在此基础上结合元数据。 |
| `ReasonCodes` | 稳定的原因码，用于说明估算路径，例如 `pk_equality`。 |
| `Notes` | 可选说明，用于补充细化过程、注意事项或元数据缺失。 |

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

| 字段 | 说明 |
|------|------|
| `RuleID` | 稳定的规则标识符，例如 `dml.where.require`。 |
| `Level` | `LevelBlocker`、`LevelWarning` 或 `LevelNotice`。 |
| `Message` | 问题的人类可读描述。 |
| `StatementIndex` | 附带语句上下文时，对应语句的 0-based 索引。 |
| `StatementKind` | 附带语句上下文时，对应语句的类型。 |
| `Location` | 原始 SQL 中的位置，不可用时为 `nil`。 |
| `Suggestion` | 建议的修复措施，不可用时为空字符串。 |
| `Metadata` | 规则特定的附加键值上下文，不存在时为 `nil`。 |
| `Explanation` | 可选的结构化解释，包含 `summary`、`why`、`risk`、`suggestion` 以及元数据可用性说明。 |

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

## 常量

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

| 值 | 含义 |
|----|------|
| `VerdictPass` | 所有语句均通过，且没有 warning 或 blocker 级别的发现。 |
| `VerdictReview` | 存在一条或多条 warning 级别的发现，且没有 blocker。 |
| `VerdictReject` | 存在一条或多条 blocker 级别的发现。 |

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

### Level

```go
const (
    LevelBlocker Level = "blocker"
    LevelWarning Level = "warning"
    LevelNotice  Level = "notice"
)
```

---

## 使用示例

### 基本审计

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

### 使用自定义策略配置

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

### 使用元数据提供者

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

## MetadataProvider 接口

```go
type MetadataProvider interface {
    LoadInstanceFacts(ctx context.Context, dialect Dialect, schema string) (*InstanceFacts, error)
    LoadTableSnapshot(ctx context.Context, dialect Dialect, schema string, table string) (*TableSnapshot, error)
}
```

实现 `MetadataProvider` 接口，即可从数据库连接中提供实时 Schema 和实例信息。将实现传入 `Request.MetadataProvider` 以启用元数据感知规则。

- `LoadInstanceFacts` 每次审计请求调用一次，返回服务器级别的配置信息，如默认字符集、InnoDB 行格式和自适应哈希索引设置。
- `LoadTableSnapshot` 针对 SQL 中引用的每张表调用一次，返回当前表定义的规范化快照，包含列、索引和表选项。

当 `MetadataProvider` 为 `nil` 时，所有元数据感知规则将被跳过，审计在离线模式下运行。离线审计无需数据库连接，始终可安全执行。

---

## 错误处理

当请求因输入或配置问题无法处理时，`Audit` 会返回非 `nil` 的错误。这些错误不是审计发现，而是表示审计无法正常启动的故障。

```go
result, err := deltascope.Audit(ctx, req)
if err != nil {
    // 处理输入/配置错误——这些不是审计发现
    log.Fatal("audit setup failed:", err)
}

// result.Verdict 体现审计结果
switch result.Verdict {
case deltascope.VerdictReject:
    os.Exit(1)
case deltascope.VerdictReview:
    // 根据策略决定是记录警告后继续，还是退出并返回 1
case deltascope.VerdictPass:
    // 全部通过
}
```

**错误条件：**

| 条件 | 原因 |
|------|------|
| SQL 为空 | `Request.SQL` 是空字符串 |
| 未知方言 | `Request.Dialect` 不是 `"mysql"`、`"tidb"` 或 `"postgresql"` |
| 配置加载失败 | `Request.ConfigPath` 指向的文件无法读取，或包含无效的 YAML |

以上错误条件对应 CLI 退出码 `2`。运行时或内部故障对应退出码 `3`。

---

## 一致性保证

库、CLI 和 HTTP 服务运行的是同一个审计引擎。在相同的 `SQL`、`Dialect` 和 `ConfigPath` 下：

- `deltascope.Audit(...)` （库）
- `deltascope audit --format json ...` （CLI）
- `POST /v1/audit` （HTTP）

产生共享的结果主体结构。库的 `Result` 类型序列化后与 HTTP 响应体主体一致；CLI `--format json` 在此基础上额外包裹顶层 `context` 字段，用于描述运行模式、方言来源和 schema 解析来源。
