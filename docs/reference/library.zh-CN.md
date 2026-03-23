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
    Dialect          Dialect           // "mysql" 或 "tidb"（默认："mysql"）
    ConfigPath       string            // YAML 策略配置文件路径（可选）
    Schema           string            // 表名解析时使用的默认 Schema（可选）
    MetadataProvider MetadataProvider  // 实时元数据来源（可选）
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `SQL` | 是 | 待审计的一条或多条 SQL 语句。 |
| `Dialect` | 否 | `DialectMySQL` 或 `DialectTiDB`。传入零值（`""`）时默认为 `DialectMySQL`。 |
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
    Index         int       `json:"index"`
    Kind          string    `json:"kind"`
    RawSQL        string    `json:"raw_sql,omitempty"`
    NormalizedSQL string    `json:"normalized_sql,omitempty"`
    Findings      []Finding `json:"findings,omitempty"`
}
```

| 字段 | 说明 |
|------|------|
| `Index` | 该语句在输入中的位置，从 1 开始计数。 |
| `Kind` | 语句类型，例如 `CREATE TABLE`、`ALTER TABLE`、`DELETE`、`UPDATE`。 |
| `RawSQL` | 该语句的原始 SQL 文本。 |
| `NormalizedSQL` | 空白符规范化后的 SQL，可能为空字符串。 |
| `Findings` | 该语句的规则发现。语句通过时为空切片。 |

### Finding

```go
type Finding struct {
    RuleID         string         `json:"rule_id"`
    Level          Level          `json:"level"`
    Message        string         `json:"message"`
    StatementIndex int            `json:"statement_index,omitempty"`
    StatementKind  string         `json:"statement_kind,omitempty"`
    Location       *Location      `json:"location,omitempty"`
    Suggestion     string         `json:"suggestion,omitempty"`
    Metadata       map[string]any `json:"metadata,omitempty"`
}
```

| 字段 | 说明 |
|------|------|
| `RuleID` | 稳定的规则标识符，例如 `dml.where.require`。 |
| `Level` | `LevelBlocker`、`LevelWarning` 或 `LevelNotice`。 |
| `Message` | 问题的人类可读描述。 |
| `StatementIndex` | 产生此发现的语句的 1-based 索引，在全局发现中使用。 |
| `StatementKind` | 产生此发现的语句类型，在全局发现中使用。 |
| `Location` | 原始 SQL 中的位置，不可用时为 `nil`。 |
| `Suggestion` | 建议的修复措施，不可用时为空字符串。 |
| `Metadata` | 规则特定的附加键值上下文，不存在时为 `nil`。 |

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
| `VerdictPass` | 所有语句均通过，无达到或超过当前阈值的发现。 |
| `VerdictReview` | 存在一条或多条 warning 或 notice 级别的发现，但无 blocker。 |
| `VerdictReject` | 存在一条或多条 blocker 级别的发现。 |

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
| 未知方言 | `Request.Dialect` 不是 `"mysql"` 或 `"tidb"` |
| 配置加载失败 | `Request.ConfigPath` 指向的文件无法读取，或包含无效的 YAML |

以上错误条件对应 CLI 退出码 `2`。运行时或内部故障对应退出码 `3`。

---

## 一致性保证

库、CLI 和 HTTP 服务运行的是同一个审计引擎。在相同的 `SQL`、`Dialect` 和 `ConfigPath` 下：

- `deltascope.Audit(...)` （库）
- `deltascope audit --format json ...` （CLI）
- `POST /v1/audit` （HTTP）

产生结构完全相同的 `Result` JSON。库的 `Result` 类型序列化后与 HTTP 响应体和 CLI `--format json` 输出具有相同的 JSON 结构。
