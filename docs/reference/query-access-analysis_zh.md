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

准入判定由读取分类推导，适用于所有方言。

## 模式

| 模式 | 列要求 | 使用场景 |
|---|---|---|
| `strict`（默认） | 每个被引用的列都需要 `read_column` 权限。 | 完整的列级访问控制。 |
| `projection_only` | 仅 SELECT 列表中出现的列（输出列）需要 `read_column` 权限。过滤、连接、分组和排序列不需要。 | 仅投影授权，调用者被信任进行过滤但不被信任查看非投影数据。 |

两种模式都要求每个权限承载关系（`read_table`）。

## 分析配置

`QueryAccessRequest.AnalysisProfile` 是可选的闭合集合。有效值包括空值（保留当前行为）、
`mysql-5.7`、`mysql-8.0`、`mysql-8.4` 和 `tidb-8.5`。MySQL 配置只能与 MySQL 方言搭配，
`tidb-8.5` 只能与 TiDB 搭配。未知值返回 `ErrInvalidQueryAccessAnalysisProfile`；方言不匹配返回
`ErrQueryAccessAnalysisProfileDialectMismatch`。

配置是兼容性目标，不是服务器身份或语义证明。默认 SDK、CLI 和 HTTP 分析保持离线。生产语义
registry 已为 `mysql-5.7`、`mysql-8.0`、`mysql-8.4` 和 `tidb-8.5` 启用；每个 profile 支持 `COUNT(*)`、
直接列 `COUNT`/`SUM`/`AVG`/`MIN`/`MAX`，8.x profile 额外支持带直接分区和排序列的
`ROW_NUMBER`/`RANK`/`DENSE_RANK`。但是，带配置的 MySQL/TiDB 函数查询在默认离线表面上仍为
`indeterminate`，因为默认 `Service` 没有 schema 解析器或活动连接。只有在显式同连接 SDK 会话
（`AnalyzeMySQLTiDBQueryAccessWithSession`）下才能提升。配置不会出现在结果 JSON 中。

### 推理风险

当存在非投影列时，projection_only 模式会发出 `projection_only_inference_risk` 警告。这警告调用者：仅被授权投影列的用户仍可通过 WHERE、JOIN 或 ORDER BY 子句推断数据。仅在授权层接受此权衡时使用 projection_only 模式。

## 表权限

strict 和 projection_only 模式都要求对每个基表和视图具有 `read_table` 权限。CTE 和派生表本身不需要权限；它们的权限要求来自它们引用的底层物理表和视图。

## 未绑定关系和列（PostgreSQL）

在 PostgreSQL 上，未限定模式的基表关系（没有 schema 限定符的关系）是**执行未绑定的**：分析器无法确定该关系在运行时解析到哪个 schema，因为 `search_path` 是会话控制的。为防止错误的权限证明，这些关系及其列在结果中标记为 `unbound: true`。

### 未绑定的含义

- 标记为 `unbound: true` 的关系**永远不会**产生 `read_table` 权限要求。
- 标记为 `unbound: true` 的列**永远不会**产生 `read_column` 权限要求。
- `unresolved` 中会出现 `unqualified_relation` 条目，原因为 `unqualified_relation_blocked`。
- 分类变为 `indeterminate`，准入变为 `indeterminate`。

**授权层不得基于未绑定关系或列授予访问权限。** `unbound` 字段表示该权限要求不是查询在运行时实际读取内容的可靠证明。

### 何时设置未绑定标记

| 场景 | 关系 | 列 |
|---|---|---|
| `SELECT id FROM users`（未限定，无解析器） | `users` → `unbound: true` | `users.id` → `unbound: true`（schema 为空，存在未绑定关系） |
| `SELECT users.id FROM users`（限定名称，未绑定关系） | `users` → `unbound: true` | `users.id` → `unbound: true` |
| `SELECT p.id, u.name FROM public.users p JOIN users u`（混合） | `public.users` → 非未绑定；`users` → `unbound: true` | `users.id`（通过限定条目解析，已赋值 schema）→ 非未绑定；`users.name`（schema 为空）→ `unbound: true` |
| `SELECT id FROM public.users`（限定） | `public.users` → 非未绑定 | `public.users.id` → 非未绑定 |
| MySQL/TiDB（任意） | 永不未绑定 | 永不未绑定 |

### 分析器如何处理混合查询

当查询包含对同一表名的限定和未限定引用时（例如 `public.users p JOIN users u`），PostgreSQL 解析器将别名解析为裸表名。`p.id` 和 `u.name` 都产生 `table: "users"`。分析器使用解析状态来区分：

- 如果解析映射中存在限定条目，列通过它解析（获得 schema 赋值）。
- 如果只有未绑定条目，则跳过解析，列保持无 schema 状态。

解析失败的列（在 schema 中找不到列）产生 `reason: column_not_found` 的 `unresolved` 条目，也被标记为 `unbound: true`。

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
| 由分类推导准入 | `read_only` → `admissible`，`not_read_only` → `rejected` | 与 MySQL/TiDB 相同 |
| CTE 权限要求 | `false` | `false` |
| WHERE 子句列用途 | `projection` + `filter` | `projection`（WHERE 列仅在 SELECT 中引用时才获得 `filter`） |
| 歧义列处理 | `indeterminate` 且有 `ambiguous_reference` 未解析项 | `read_only` 且有未限定列引用 |
| `reason_codes` 填充 | 是（`write_operation`、`function_call`、`parse_failure` 等） | 是（`unproven_operator_effect`、`unproven_function_effect`、`unproven_cast_effect`、`unqualified_relation_blocked`、`identity_*` 码） |
| `unresolved` 填充 | 是（通配符、歧义引用） | 是（`unqualified_relation` 条目，用于未限定模式的基表关系） |

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
    "context"
    "github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

result, err := deltascope.AnalyzeQueryAccess(context.Background(), deltascope.QueryAccessRequest{
    SQL:     "SELECT id, name FROM users",
    Dialect: deltascope.DialectMySQL,
    Mode:    deltascope.QueryAccessModeStrict,
})
```

## CLI 用法

查询访问分析可通过 CLI 使用：

```bash
deltascope query-access analyze --sql "SELECT id, name FROM users WHERE id = 1" --dialect mysql
deltascope query-access analyze --file ./query.sql --dialect postgresql --mode projection_only
```

退出码：`0` = 可准入，`1` = 已拒绝，`2` = 不确定，`3` = 用法错误。

## HTTP 用法

查询访问分析可通过 HTTP API 使用：

```bash
curl -X POST http://localhost:8083/v1/query-access/analyze \
  -H "Content-Type: application/json" \
  -d '{"sql":"SELECT id FROM users","dialect":"mysql","mode":"strict","profile":"mysql-8.4"}'
```

该端点返回与 SDK 相同的 JSON 结构。无效模式返回 `400` 和 `invalid_mode` 错误码；无效配置返回有界的 `400` 错误，
不会回显配置或 SQL。

## MySQL/TiDB 会话边界

显式 SDK 会话 API 接受调用者拥有的 `*sql.Conn`：

```go
session, err := deltascope.NewMySQLTiDBQueryAccessSessionFromConn(ctx, conn)
result, err := deltascope.AnalyzeMySQLTiDBQueryAccessWithSession(ctx, session, deltascope.QueryAccessRequest{
    SQL:             "SELECT id FROM app.users",
    Dialect:         deltascope.DialectMySQL,
    AnalysisProfile: deltascope.QueryAccessAnalysisProfileMySQL84,
    DefaultSchema:   "app",
})
```

会话不拥有或暴露连接。它只从同一连接构造关系元数据解析，拒绝外部 `SchemaResolver`，并且是唯一可以构造私有
语义能力的 SDK 边界。生产 registry 已为 `mysql-5.7`、`mysql-8.0`、`mysql-8.4` 和 `tidb-8.5` 启用。
当带配置的查询通过会话连接获得完整物理元数据时，已证明的条目可提升为 `read_only + admissible`。
会话不会暴露目录、manifest、连接或凭据细节。

## MCP 延迟

查询访问分析的 MCP 表面集成已延迟。当前 MCP 服务器仅暴露 `audit_sql`、`describe_rule`、`list_rules` 和 `get_capabilities`。

## 可信 PostgreSQL SDK 路径

可信 PostgreSQL SDK 路径支持 PostgreSQL 查询的 manifest 门控准入提升。此路径仅在使用 `postgresql` 构建标签构建时可用。

### 会话构建

```go
session, err := deltascope.NewPostgreSQLQueryAccessSessionFromConn(ctx, conn)
```

- 接受调用者拥有的 `*sql.Conn`（非 `*sql.DB`）
- 通过 `PingContext` 验证连接活性
- 不获取连接的所有权；调用者必须关闭它
- 在非 postgresql 构建中返回 `ErrPostgreSQLSessionNotAvailable`

### 可信分析

```go
result, err := deltascope.AnalyzePostgreSQLQueryAccessWithSession(ctx, session, req)
```

- 拒绝 nil 上下文、nil 会话、非 PostgreSQL 方言或非 nil `SchemaResolver`
- 从会话的单个 `*sql.Conn` 创建所有元数据、类型和效果标识解析器
- 当每个效果都已目录解析并列在 PG17 manifest 中时，可能返回 `read_only + admissible`
- 在非 postgresql 构建中返回 `ErrPostgreSQLSessionNotAvailable`

### 准入语义

`admissible` 仅表示静态分析获得了完整的已知要求，并针对提供的连接目录上下文证明了有界效果 manifest。它**不**：

- 授权执行
- 评估授权或许可
- 保证后续执行快照使用相同的数据库状态
- 考虑行级安全、屏蔽或 SQL 重写

### 默认路径

默认的 `AnalyzeQueryAccess` 函数（无会话）对 PostgreSQL 保持失败关闭。CLI、HTTP 和 MCP 表面继续使用默认路径，不会获得可信提升。

### Phase 1 纯效果矩阵

以下是 Phase 1 的精确契约。“仅表征”表示测试观察到了该形状；它不是
受支持或可准入的函数白名单。

| 方言 | 表面 | Phase 1 聚合/窗口 |
|---|---|---|
| PostgreSQL | 默认 SDK/CLI/HTTP | `indeterminate`（保持不变） |
| PostgreSQL | 仅可信 SDK 会话 | 在要求完整且证明通过时，`count`/`sum`/`avg`/`min`/`max`/`row_number`/`rank`/`dense_rank` 可为 `admissible` |
| MySQL | 默认 SDK/CLI/HTTP | `indeterminate`，原因是 `unknown_function_effect`（离线 fail-closed） |
| MySQL | 显式 SDK 会话（`AnalyzeMySQLTiDBQueryAccessWithSession`）配合 `mysql-5.7`/`mysql-8.0`/`mysql-8.4` profile | 已证明的 `COUNT(*)`、直接列 `COUNT`/`SUM`/`AVG`/`MIN`/`MAX` 可为 `admissible`；8.x profile 还支持带直接分区+排序列的 `ROW_NUMBER`/`RANK`/`DENSE_RANK` |
| TiDB | 默认 SDK/CLI/HTTP | `indeterminate`，原因是 `unknown_function_effect`（离线 fail-closed） |
| TiDB | 显式 SDK 会话配合 `tidb-8.5` profile | 已证明的 `COUNT(*)`、直接列 `COUNT`/`SUM`/`AVG`/`MIN`/`MAX`，以及带直接分区+排序列的 `ROW_NUMBER`/`RANK`/`DENSE_RANK` 可为 `admissible` |

可信 PostgreSQL 子集要求精确目录身份、会话和数据库上下文、完整的
strict 依赖，以及 PG17 manifest 证明。`DISTINCT`、`FILTER`、嵌套参数、
cast、窗口 frame、命名窗口和不完整元数据仍为 `indeterminate`。MySQL/TiDB
不会因语法或函数名称而获得提升。

## 纵深防御

查询访问分析是纵深防御授权策略中的一层。它确定查询触及哪些对象，但不评估调用者是否有权限访问这些对象。将其与以下措施配合使用：

- **认证**：在分析之前验证调用者身份。
- **授权评估**：将生成的需求与调用者已授予的权限进行检查。
- **行级安全**：独立于列级分析应用行过滤器。
- **审计日志**：记录分析结果和授权决策以供合规使用。
