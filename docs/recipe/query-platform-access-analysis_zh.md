# 查询平台访问分析

本指南展示如何使用 DeltaScope 的查询访问分析来确定 SQL 查询触及的数据库对象以及调用者需要的权限。

## 问题

你有一条 SQL 查询，需要知道：
- 它是否是只读的？
- 它访问了哪些表和列？
- 调用者需要什么权限？

你希望在不执行查询的情况下执行列级访问控制。

## 解决方案

使用 `queryaccess.Service.Analyze()` API 来检查查询并生成结构化的访问结果。

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

- `read_classification: read_only` — 查询不修改数据。
- `admission: admissible` — 查询有资格进行授权。
- `relations` — 调用者需要对 `users` 具有 `read_table` 权限。
- `requirements` — 调用者需要对 `users.id` 具有 `read_column` 权限。

### 步骤 3：处理被拒绝的查询

```go
result, _ := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
    SQL:     "DELETE FROM users WHERE id = 1",
    Dialect: "mysql",
})

// result.DomainResult.ReadClassification == "not_read_only"
// result.DomainResult.Admission == "rejected"
```

被拒绝的查询不应继续进行授权。应立即拒绝。

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

不确定的查询需要元数据来解析。提供 `SchemaResolver`：

```go
type myResolver struct{}

func (r *myResolver) ResolveRelation(ctx context.Context, dialect, schema, name string) (appqa.RelationSchema, error) {
    // 在元数据存储中查找关系。
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
// 通配符现在被解析为单独的列引用。
```

### 步骤 5：使用投影模式

当调用者被信任进行过滤但只能看到投影列时：

```go
result, _ := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
    SQL:     "SELECT id FROM users WHERE salary > 50000",
    Dialect: "mysql",
    Mode:    "projection_only",
})

// requirements 仅包含 users.id，不包含 users.salary
// warnings 包含 "projection_only_inference_risk"
```

`projection_only_inference_risk` 警告表示 `salary` 在过滤条件中使用但不需要授权。授权层必须接受此权衡。

## 解读结果

| 字段 | 含义 |
|---|---|
| `read_classification` | 查询是只读、非只读还是不确定。 |
| `admission` | 查询是否有资格进行授权。 |
| `relations` | 查询读取的表/CTE/派生表。 |
| `referenced_columns` | 列及其使用上下文（投影、过滤、连接等）。 |
| `outputs` | 输出列及其源血缘。 |
| `requirements` | 调用者需要的权限（`read_table`、`read_column` 或 `indeterminate`）。 |
| `unresolved` | 没有元数据就无法解析的引用。 |
| `warnings` | 非致命警告（如推理风险）。 |
| `reason_codes` | 分类的机器可读原因。 |

## 安全默认值

- 空模式默认为 `strict`。
- 没有元数据时，通配符产生 `indeterminate` 分类。
- PostgreSQL 准入始终为 `indeterminate`。
- 在授权层中将 `indeterminate` 视为拒绝。

## 此功能不做的事情

- 不认证调用者。
- 不评估授权或权限。
- 不执行行级安全。
- 不屏蔽敏感列。
- 不展开视图定义。
- 不支持 MCP 表面（已延迟）。
