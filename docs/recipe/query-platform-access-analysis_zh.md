# 查询平台访问分析

本指南展示如何用 DeltaScope 的查询访问分析，确定一条 SQL 查询触及哪些数据库对象、调用者需要什么权限。

## 问题

你有一条 SQL 查询，想知道：

- 它是只读的吗？
- 它访问了哪些表和列？
- 调用者需要什么权限？

你希望在不执行查询的前提下做列级访问控制。

## 解决方案

用 `queryaccess.Service.Analyze()` 检查查询，产出一份结构化的访问结果。

> 注意：本指南用的是默认离线路径 `appqa.Service.Analyze()`，它不连数据库。带函数的 MySQL/TiDB 查询在这条路径上仍是 `indeterminate`。如果你想让 SDK 真正确认 MySQL/TiDB 的函数查询（例如 `COUNT(*)`），需要用统一的同连接在线会话 API（`NewOnlineQueryAccessSessionFromConn` 和 `AnalyzeOnlineQueryAccessWithSession`）；详见[查询访问分析参考](../reference/query-access-analysis_zh.md)中“让 SDK 真正确认 MySQL/TiDB 的函数查询”一节。

### 步骤 1：分析简单查询

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

输出：

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

### 步骤 2：解读结果

- `read_classification: read_only` — 查询不改数据。
- `admission: admissible` — 查询有资格进入授权检查。
- `relations` — 调用者需要对 `users` 有 `read_table` 权限。
- `requirements` — 调用者需要对 `users.id` 有 `read_column` 权限。

### 步骤 3：处理被拒绝的查询

```go
result, _ := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
    SQL:     "DELETE FROM users WHERE id = 1",
    Dialect: "mysql",
})

// result.DomainResult.ReadClassification == "not_read_only"
// result.DomainResult.Admission == "rejected"
```

被拒绝的查询不应该继续走授权，直接拒绝。

### 步骤 4：处理不确定的查询

```go
result, _ := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
    SQL:     "SELECT * FROM users",
    Dialect: "mysql",
})

// result.DomainResult.ReadClassification == "indeterminate"
// result.DomainResult.Admission == "indeterminate"
// result.DomainResult.Unresolved 包含 [{reference: "*", reason: "schema_unavailable"}]
```

不确定的查询需要元数据才能解析。提供 `SchemaResolver`：

```go
type myResolver struct{}

func (r *myResolver) ResolveRelation(ctx context.Context, dialect, schema, name string) (appqa.RelationSchema, error) {
    // 在你的元数据存储里查找关系。
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
// 通配符现在被解析成单独的列引用。
```

### 步骤 5：使用投影模式

当调用者可以参与过滤、但只能看到投影列时：

```go
result, _ := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
    SQL:     "SELECT id FROM users WHERE salary > 50000",
    Dialect: "mysql",
    Mode:    "projection_only",
})

// requirements 只包含 users.id，不包含 users.salary
// warnings 包含 "projection_only_inference_risk"
```

`projection_only_inference_risk` 警告表示 `salary` 用在了过滤条件里，但不需要授权。授权层必须接受这个权衡。

## 解读结果

| 字段 | 含义 |
|---|---|
| `read_classification` | 查询是只读、非只读，还是不确定。 |
| `admission` | 查询是否有资格进入授权。 |
| `relations` | 查询读取的表/CTE/派生表。 |
| `referenced_columns` | 列及其使用上下文（投影、过滤、连接等）。 |
| `outputs` | 输出列及其源血缘。 |
| `requirements` | 调用者需要的权限（`read_table`、`read_column`，或整体为 `indeterminate`）。 |
| `unresolved` | 没有元数据就无法解析的引用。 |
| `warnings` | 非致命警告（如推理风险）。 |
| `reason_codes` | 分类的机器可读原因。 |

## 安全默认值

- 空模式默认为 `strict`。
- 没有元数据时，通配符产生 `indeterminate` 分类。
- 默认 SDK/CLI/HTTP 在默认离线路径上不连数据库；带函数的 PostgreSQL、MySQL、TiDB 查询在这条路径上都保持 `indeterminate`。
- 要让带函数的查询从 `indeterminate` 提升为 `admissible`，必须用统一的同连接在线会话 API：`NewOnlineQueryAccessSessionFromConn` 和 `AnalyzeOnlineQueryAccessWithSession`。方言专用会话 API（`NewPostgreSQLQueryAccessSessionFromConn` / `AnalyzePostgreSQLQueryAccessWithSession`、`NewMySQLTiDBQueryAccessSessionFromConn` / `AnalyzeMySQLTiDBQueryAccessWithSession`）已弃用，迁移方式见[查询访问分析参考](../reference/query-access-analysis_zh.md)的迁移一节。提升需要真实的数据库连接；默认离线 SDK/CLI/HTTP 路径不会打开连接。
- MySQL/TiDB 会话提升要求被引用的基表是 schema-qualified（例如 `app.orders`）。未限定表名即使请求携带 `DefaultSchema` 也保持 `indeterminate`。语义 manifest 由观察到的服务器版本系列选定；SQL mode 不会被验证，因此运行非默认 SQL mode 的服务器可能与分析语义有差异。
- 在授权层里把 `indeterminate` 当作拒绝处理。

### Phase 1 表面矩阵

下面矩阵描述各表面在带函数查询上的默认行为。详细支持和仍然 `indeterminate` 的情况见[查询访问分析参考](../reference/query-access-analysis_zh.md)。

| 方言 | 表面 | Phase 1 聚合/窗口 |
|---|---|---|
| PostgreSQL | 默认 SDK/CLI/HTTP | `indeterminate`（保持不变） |
| PostgreSQL | 仅统一在线会话 | 在要求完整且证明通过时，count/sum/avg/min/max/row_number/rank/dense_rank 为 `admissible` |
| MySQL | 默认 SDK/CLI/HTTP | `indeterminate`，原因是 `unknown_function_effect`（离线保守） |
| MySQL | 统一在线会话（身份推导的 `mysql-5.7`/`mysql-8.0`/`mysql-8.4` profile） | 已证明的 `COUNT(*)`、直接列 `COUNT`/`SUM`/`AVG`/`MIN`/`MAX` 可为 `admissible`；8.x profile 还支持带直接分区+排序列的 ranking window |
| TiDB | 默认 SDK/CLI/HTTP | `indeterminate`，原因是 `unknown_function_effect`（离线保守） |
| TiDB | 统一在线会话（身份推导的 `tidb-8.5` profile） | 已证明的 `COUNT(*)`、直接列 `COUNT`/`SUM`/`AVG`/`MIN`/`MAX`，以及带直接分区+排序列的 ranking window 可为 `admissible` |

不要把仅表征的函数形状称为受支持。提升路径仅限 SDK，不新增 CLI/HTTP 数据库连接，也不新增 MCP 工具。

## 此功能不做的事情

- 不认证调用者。
- 不评估授权或权限。
- 不执行行级安全。
- 不屏蔽敏感列。
- 不展开视图定义。
- 不在 MCP 表面提供（已延迟）。
