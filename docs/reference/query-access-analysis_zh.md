# 查询访问分析参考

查询访问分析检查 SQL 语句并确定调用者必须被授权读取的数据库对象。它**不会**认证调用者、评估授权、执行行级安全或屏蔽敏感列。它生成一个结构化结果供授权层消费。

## 读取分类

每条被分析的语句会获得以下三种读取分类之一：

| 分类 | 含义 |
|---|---|
| `read_only` | 语句不包含写操作、锁定子句或需要运行时求值的函数。 |
| `not_read_only` | 语句包含至少一个写操作（`INSERT`、`UPDATE`、`DELETE`、`FOR UPDATE`、`INTO OUTFILE`、DDL 等）。 |
| `indeterminate` | 无法确定只读状态。常见原因：函数调用（`NOW()`）、未解析的通配符（无元数据时的 `SELECT *`）、歧义列引用、解析失败或空输入。 |

## 准入判定

准入判定由读取分类推导：

| 准入 | 条件 |
|---|---|
| `admissible` | 分类为 `read_only`。语句有资格进行授权检查。 |
| `rejected` | 分类为 `not_read_only`。语句无资格。 |
| `indeterminate` | 分类为 `indeterminate`。无法在缺少额外信息的情况下进行授权。 |

MySQL/TiDB 的准入判定由分类推导。PostgreSQL 的准入始终为 `indeterminate`，因为 PostgreSQL 提取器尚未根据分类计算 `admissible`/`rejected`。

## 模式

| 模式 | 列要求 | 使用场景 |
|---|---|---|
| `strict`（默认） | 每个被引用的列都需要 `read_column` 权限。 | 完整的列级访问控制。 |
| `projection_only` | 仅 SELECT 列表中出现的列（输出列）需要 `read_column` 权限。过滤、连接、分组和排序列不需要。 | 仅投影授权，调用者被信任进行过滤但不被信任查看非投影数据。 |

两种模式都要求每个权限承载关系（`read_table`）。

### 推理风险

当存在非投影列时，projection_only 模式会发出 `projection_only_inference_risk` 警告。这警告调用者：仅被授权投影列的用户仍可通过 WHERE、JOIN 或 ORDER BY 子句推断数据。仅在授权层接受此权衡时使用 projection_only 模式。

## 表权限

strict 和 projection_only 模式都要求对每个基表和派生表具有 `read_table` 权限。CTE 不需要权限（MySQL/TiDB）。PostgreSQL CTE 当前需要权限。

## 失败关闭行为

当分析无法确定读取分类或所需权限时，结果为 `indeterminate`。授权层应默认将 `indeterminate` 视为拒绝。特定的失败关闭场景：

- **解析失败**：`read_classification: indeterminate`，`reason_codes: [parse_failure]`
- **空输入**：`read_classification: indeterminate`，`reason_codes: [zero_statements]`
- **未解析通配符**：`read_classification: indeterminate`，`unresolved: [{reference: "*", reason: schema_unavailable}]`
- **歧义列**：`read_classification: indeterminate`，`unresolved: [{reference: "unqualified_column", reason: ambiguous_reference}]`

## 元数据要求

没有元数据时，通配符（`SELECT *`）保持未解析状态，分类变为 `indeterminate`。要解析通配符，需提供返回关系模式（表名、带序号的列）的 `SchemaResolver`。有元数据时：

- 通配符按序号顺序展开为单独的列引用。
- 当恰好一个源关系包含该列时，未限定列会被解析。
- 视图被检测并标记为 `RelationView` 类型。

## 方言差异

| 特性 | MySQL/TiDB | PostgreSQL |
|---|---|---|
| 由分类推导准入 | `read_only` → `admissible`，`not_read_only` → `rejected` | 始终为 `indeterminate` |
| CTE 权限要求 | `false` | `true` |
| WHERE 子句列用途 | `projection` + `filter` | `projection`（WHERE 列仅在 SELECT 中引用时才获得 `filter`） |
| 歧义列处理 | `indeterminate` 且有 `ambiguous_reference` 未解析项 | `read_only` 且有未限定列引用 |
| `reason_codes` 填充 | 是（`write_operation`、`function_call`、`parse_failure` 等） | 否（空） |
| `unresolved` 填充 | 是（通配符、歧义引用） | 否（大多数情况为空） |

## 结果结构

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

结果有意排除了原始 SQL、字面量值、密码和凭据。

## SDK 用法

```go
import (
    appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
    "context"
)

svc := &appqa.Service{}
result, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
    SQL:     "SELECT id, name FROM users",
    Dialect: "mysql",
    Mode:    "strict",
})
```

## CLI 用法

查询访问分析通过库 API 可用。CLI 和 HTTP 表面集成计划在未来版本中发布。

## MCP 延迟

查询访问分析的 MCP 表面集成已延迟。当前 MCP 服务器仅暴露 `audit_sql`、`describe_rule`、`list_rules` 和 `get_capabilities`。

## 纵深防御

查询访问分析是纵深防御授权策略中的一层。它确定查询触及哪些对象，但不评估调用者是否有权限访问这些对象。将其与以下措施配合使用：

- **认证**：在分析之前验证调用者身份。
- **授权评估**：将生成的需求与调用者已授予的权限进行检查。
- **行级安全**：独立于列级分析应用行过滤器。
- **审计日志**：记录分析结果和授权决策以供合规使用。
