# 查询访问分析参考

查询访问分析检查一条 SQL，列出它需要读取的表和列，供你的授权层拿去比对调用者权限。它不替你做认证，也不评估 grant、不强制行级安全、不脱敏列。它只产出一份结构化结果。

## 读取分类

每条被分析的语句会得到下面三种分类之一：

| 分类 | 含义 |
|---|---|
| `read_only` | 语句不含写操作、不含锁定子句、不含需要运行时求值的函数。 |
| `not_read_only` | 语句至少包含一个写操作（`INSERT`、`UPDATE`、`DELETE`、`FOR UPDATE`、`INTO OUTFILE`、DDL 等）。 |
| `indeterminate` | 无法判断是否只读。常见原因：函数调用（`NOW()`）、未解析的通配符（无元数据时的 `SELECT *`）、歧义列引用、解析失败、空输入。 |

## 准入判定

准入（admission）从读取分类推导：

| 准入 | 条件 |
|---|---|
| `admissible` | 分类为 `read_only`。语句可以进入授权检查。 |
| `rejected` | 分类为 `not_read_only`。语句不可进入授权。 |
| `indeterminate` | 分类为 `indeterminate`。信息不足，授权无法继续。 |

所有方言都按这个映射推导。

## 模式

| 模式 | 列要求 | 使用场景 |
|---|---|---|
| `strict`（默认） | 每个被引用的列都需要 `read_column` 权限。 | 完整的列级访问控制。 |
| `projection_only` | 仅 SELECT 列表里出现的列（输出列）需要 `read_column`。过滤、连接、分组、排序列不需要。 | 只授权投影列；调用者可以参与过滤，但不能看到非投影数据。 |

两种模式都要求每个承载权限的关系有 `read_table` 权限。

## 分析配置

`QueryAccessRequest.AnalysisProfile` 是可选的闭合集合。有效值有空值（保留默认行为）、`mysql-5.7`、`mysql-8.0`、`mysql-8.4`、`tidb-8.5`。MySQL 配置只能搭配 MySQL 方言，`tidb-8.5` 只能搭配 TiDB。未知值返回 `ErrInvalidQueryAccessAnalysisProfile`；方言不匹配返回 `ErrQueryAccessAnalysisProfileDialectMismatch`。

配置是一个兼容性目标，告诉分析器“按哪个引擎/版本的语义来看这条 SQL”。它不是服务器身份验证：当前实现不会去查询实际服务器的 `VERSION()` 或 SQL mode，也不会校验连接实际指向哪个版本。调用方要自己保证所选 profile 与实际 MySQL/TiDB 版本以及相关 SQL mode 匹配；选错 profile，分析仍会按那个 profile 的语义来判断，结果可能与真实服务器行为不一致。

profile 也不改变默认行为。DeltaScope 的默认路径不会自行创建数据库连接：默认 SDK 可以接受调用方提供的 `SchemaResolver` 来解析表名或展开通配符，但这不会启用 MySQL/TiDB 函数效果提升；CLI 和 HTTP 只在提供连接参数时走统一在线入口。生产语义 registry 已为 `mysql-5.7`、`mysql-8.0`、`mysql-8.4`、`tidb-8.5` 启用；每个 profile 支持 `COUNT(*)`、直接列 `COUNT`/`SUM`/`AVG`/`MIN`/`MAX`，8.x profile 还支持带直接分区和排序列的 `ROW_NUMBER`/`RANK`/`DENSE_RANK`。

但要注意一条边界：带了配置、又含函数的 MySQL/TiDB 查询，在默认离线表面上仍然是 `indeterminate`。原因是默认 `Service` 走的是离线路径，不会连数据库、也不会启用函数语义提升，无法确认这些函数会读取什么。配置不会出现在结果 JSON 里。

### 推理风险

projection_only 模式下如果存在非投影列，会发出 `projection_only_inference_risk` 警告。意思是一个只被授权看到投影列的用户，仍可能通过 WHERE、JOIN 或 ORDER BY 推测出其他列的数据。只有当你的授权层愿意接受这个权衡时再用 projection_only。

## 表权限

strict 和 projection_only 都要求每个基表和视图有 `read_table` 权限。CTE 和派生表本身不直接要求权限，它们的要求来自底层引用的物理表和视图。

## 默认行为和何时会“无法确认”

命令行、HTTP 和普通 SDK 调用（`AnalyzeQueryAccess`）在遇到信息不足以判断的情况时，会保守地返回 `indeterminate`（无法确认），不会直接放行。这表示当前分析结果不足以完整列出查询会读取什么，并不是说这条查询安全、只读或会写数据。

下面几种情况默认就会得到 `indeterminate`：

- **带函数的查询**，例如 `SELECT NOW()`、`SELECT COUNT(*) FROM app.users`，在默认离线表面上无法确认函数会读取什么（默认路径不启用函数效果提升；要提升必须走下文的同连接会话）。
- **`SELECT *`**：没有元数据时无法展开通配符，分类为 `indeterminate`，`unresolved` 里会出现 `{reference: "*", reason: schema_unavailable}`。
- **未限定的表名**（PostgreSQL）：`SELECT id FROM users` 这种没有 schema 限定符的关系，运行时到底解析到哪个 schema 取决于会话的 `search_path`，分析器无法确定，见后文“未绑定关系和列”。
- **未知的函数或运算符效果**：MySQL/TiDB 上带函数查询返回 `indeterminate`，原因是 `unknown_function_effect`。
- **解析失败**：`reason_codes: [parse_failure]`。
- **空输入**：`reason_codes: [zero_statements]`。
- **歧义列**：`unresolved: [{reference: "unqualified_column", reason: ambiguous_reference}]`。

授权层应该把 `indeterminate` 当作拒绝处理。

## 元数据要求

通配符（`SELECT *`）没有元数据时无法解析，分类为 `indeterminate`。要解析通配符，需提供返回关系模式（表名、带序号的列）的 `SchemaResolver`。有了元数据之后：

- 通配符按序号顺序展开为单独的列引用。
- 当恰好一个源关系包含该列时，未限定列会被解析。
- 视图会被检测出来，标记为 `RelationView` 类型。

## 未绑定关系和列（PostgreSQL）

PostgreSQL 上，没有 schema 限定符的基表关系是**执行未绑定的**：运行时它解析到哪个 schema，取决于会话的 `search_path`，而 `search_path` 是会话控制的，分析器无法确定。为避免给出错误的权限证明，这类关系和它们的列在结果里被标记为 `unbound: true`。

### 未绑定意味着什么

- 标记为 `unbound: true` 的关系**永远不会**产生 `read_table` 权限要求。
- 标记为 `unbound: true` 的列**永远不会**产生 `read_column` 权限要求。
- `unresolved` 中会出现 `unqualified_relation` 条目，原因是 `unqualified_relation_blocked`。
- 分类变为 `indeterminate`，准入变为 `indeterminate`。

授权层不得基于未绑定关系或列授予访问权限。`unbound` 字段表示这条权限要求不是查询在运行时实际读取内容的可靠证明。

### 何时设置未绑定标记

| 场景 | 关系 | 列 |
|---|---|---|
| `SELECT id FROM users`（未限定，无解析器） | `users` → `unbound: true` | `users.id` → `unbound: true`（schema 为空，存在未绑定关系） |
| `SELECT users.id FROM users`（限定名称，未绑定关系） | `users` → `unbound: true` | `users.id` → `unbound: true` |
| `SELECT p.id, u.name FROM public.users p JOIN users u`（混合） | `public.users` → 非未绑定；`users` → `unbound: true` | `users.id`（通过限定条目解析，已赋值 schema）→ 非未绑定；`users.name`（schema 为空）→ `unbound: true` |
| `SELECT id FROM public.users`（限定） | `public.users` → 非未绑定 | `public.users.id` → 非未绑定 |
| MySQL/TiDB（任意） | 永不未绑定 | 永不未绑定 |

### 分析器如何处理混合查询

当查询同时对同一表名做了限定和未限定引用（例如 `public.users p JOIN users u`），PostgreSQL 解析器会把别名解析回裸表名。`p.id` 和 `u.name` 都产生 `table: "users"`。分析器用解析状态来区分：

- 如果解析映射中存在限定条目，列通过它解析（获得 schema 赋值）。
- 如果只有未绑定条目，则跳过解析，列保持无 schema 状态。

解析失败的列（在 schema 中找不到列）产生 `reason: column_not_found` 的 `unresolved` 条目，也会被标记为 `unbound: true`。

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

该端点返回与 SDK 相同的 JSON 结构。无效模式返回 `400` 和 `invalid_mode` 错误码；无效配置返回有界的 `400` 错误，不会回显配置或 SQL。

## 让 SDK 真正确认 MySQL/TiDB 的函数查询（同连接会话）

如果你的 Go 程序已经连上了 MySQL 或 TiDB，可以把这条连接交给 SDK，SDK 才能确认真实的表和列，并在支持的函数范围内给出可用的权限清单。这是唯一能让 MySQL/TiDB 函数查询从“无法确认”提升为“可准入”的 SDK 路径。默认 SDK、CLI 和 HTTP 路径不会打开数据库连接，因此做不到这一点；CLI 和 HTTP 只有在提供连接参数、走在线模式时才能获得提升。

这里要分清两件事：你传入的连接用于识别服务器并解析真实的关系和列元数据；函数本身的语义来自由观察到的服务器身份选定的内置、不可变语义 manifest。当前连接不会被用来验证服务器版本或 SQL mode，也不会被用来证明函数语义——语义由 manifest 决定，连接只负责把表名、列名落到真实对象上。

最小示例（统一在线入口；`Dialect` 留空，由观察到的身份选择路由）：

```go
session, err := deltascope.NewOnlineQueryAccessSessionFromConn(ctx, conn)
result, err := deltascope.AnalyzeOnlineQueryAccessWithSession(ctx, session, deltascope.QueryAccessRequest{
    SQL:           "SELECT COUNT(*) FROM app.orders",
    Mode:          deltascope.QueryAccessModeStrict,
    DefaultSchema: "app",
})
```

注意示例里的表名写成 `app.orders`，带 schema 限定符。这是提升的硬性要求，见下文。

### 提升要求 schema 限定的基表

会话提升要求被引用的基表是 schema-qualified 的（例如 `app.orders`，而不是 `orders`）。即使请求里带了 `DefaultSchema`，未限定表名仍然保持 `indeterminate`，不会被提升。原因是提升路径要求把每个函数输入都严格解析到物理基表列，未限定表名无法稳定绑定到某个 schema，也就无法形成可靠的物理依赖。

### `COUNT(*)` 的正向示例

通过会话连接、profile 正确、表名 schema 限定且列元数据完整时，下面这类查询可以提升为 `read_only + admissible`：

- `SELECT COUNT(*) FROM app.orders`（四个 profile 都支持）
- `SELECT SUM(amount) FROM app.orders`（直接列 `SUM`/`AVG`/`MIN`/`MAX`）
- `SELECT ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at) FROM app.orders`（8.x profile，直接列分区和排序）

### 仍然“无法确认”的情况

即便用了会话连接，下面这些仍为 `indeterminate`：

- 表名未限定，例如 `SELECT COUNT(*) FROM orders`，即使请求携带 `DefaultSchema`。
- 查询引用了视图、CTE、派生表或通配符（`SELECT *`）。
- 使用了 `DISTINCT`、`FILTER`、嵌套表达式、cast、显式窗口 frame、命名窗口。
- ranking window 缺少 `ORDER BY`，或分区/排序不是直接基表列。
- 函数调用被 schema 限定（`app.COUNT(*)`）、被引号包裹（`` `COUNT`(*) ``）或间距不规范（`COUNT (id)`）。
- 元数据不完整，或函数/运算符不在所选 profile 的支持清单内。
- MySQL 5.7 上的 ranking-window 函数（该 profile 没有原生 ranking-window 支持，保持延迟）。

### `admissible` 不是授权

会话分析返回 `admissible`，只表示静态分析拿到了完整的已知要求：表名和列名通过你的连接解析到了真实物理对象，且查询里的每个函数效果都在身份推导的 profile 的支持清单内。它**不**：

- 授权执行这条查询。
- 评估 grant 或权限。
- 保证稍后真正执行时数据库状态和现在一致。
- 考虑行级安全、脱敏或 SQL 重写。
- 证明服务器的 SQL mode 与从观察到的版本选定的语义 manifest 一致（SQL mode 不会被验证）。

换句话说，`admissible` 是“我能完整列出它会读取什么”，不是“调用者被允许读取”，也不是“查询安全”或“只读执行结果保证”。

### 会话的连接归属和安全细节

会话不拥有也不暴露你传入的连接。它只从这条连接构造关系元数据解析，拒绝外部 `SchemaResolver`，是唯一能构造私有语义能力的 SDK 边界。会话不会暴露目录、manifest、连接或凭据细节。连接由你负责关闭。

## MCP 延迟

查询访问分析的 MCP 表面集成已延迟。当前 MCP 服务器只暴露 `audit_sql`、`describe_rule`、`list_rules` 和 `get_capabilities`。

## 可信 PostgreSQL SDK 路径

PostgreSQL 也有类似的同连接会话路径，支持 manifest 门控的准入提升。此路径仅在使用 `postgresql` 构建标签构建时可用。

### 会话构建

```go
session, err := deltascope.NewOnlineQueryAccessSessionFromConn(ctx, conn)
```

- 接受调用者拥有的 `*sql.Conn`（不是 `*sql.DB`）
- 通过 `PingContext` 验证连接活性并识别服务器
- 不获取连接的所有权；连接由调用者关闭
- 在非 postgresql 构建中返回 `ErrOnlineQueryAccessCapabilityUnsupported`

### 可信分析

```go
result, err := deltascope.AnalyzeOnlineQueryAccessWithSession(ctx, session, req)
```

- 拒绝 nil 上下文、nil 会话、不匹配的非空请求方言或非 nil `SchemaResolver`
- 从会话的单个 `*sql.Conn` 创建所有元数据、类型和效果标识解析器
- 当每个效果都已目录解析并列在 PG17 manifest 中时，可能返回 `read_only + admissible`
- 在非 postgresql 构建中返回 `ErrOnlineQueryAccessCapabilityUnsupported`

### 默认路径

默认的 `AnalyzeQueryAccess`（不带会话）对 PostgreSQL 保持“无法确认”的保守行为。CLI 和 HTTP 在未提供连接参数时走默认路径；提供连接参数时在线模式走统一在线入口并获得可信提升；MCP 没有 query-access 工具。

### Phase 1 纯效果矩阵

下面是 Phase 1 的精确契约。“仅表征”表示测试观察到了该形状；它不是受支持或可准入的函数白名单。

| 方言 | 表面 | Phase 1 聚合/窗口 |
|---|---|---|
| PostgreSQL | 默认 SDK/CLI/HTTP | `indeterminate`（保持不变） |
| PostgreSQL | 仅统一在线会话 | 在要求完整且证明通过时，`count`/`sum`/`avg`/`min`/`max`/`row_number`/`rank`/`dense_rank` 可为 `admissible` |
| MySQL | 默认 SDK/CLI/HTTP | `indeterminate`，原因是 `unknown_function_effect`（离线保守） |
| MySQL | 统一在线会话（身份推导的 `mysql-5.7`/`mysql-8.0`/`mysql-8.4` profile） | 已证明的 `COUNT(*)`、直接列 `COUNT`/`SUM`/`AVG`/`MIN`/`MAX` 可为 `admissible`；8.x profile 还支持带直接分区+排序列的 `ROW_NUMBER`/`RANK`/`DENSE_RANK` |
| TiDB | 默认 SDK/CLI/HTTP | `indeterminate`，原因是 `unknown_function_effect`（离线保守） |
| TiDB | 统一在线会话（身份推导的 `tidb-8.5` profile） | 已证明的 `COUNT(*)`、直接列 `COUNT`/`SUM`/`AVG`/`MIN`/`MAX`，以及带直接分区+排序列的 `ROW_NUMBER`/`RANK`/`DENSE_RANK` 可为 `admissible` |

可信 PostgreSQL 子集要求精确目录身份、会话和数据库上下文、完整的 strict 依赖，以及 PG17 manifest 证明。`DISTINCT`、`FILTER`、嵌套参数、cast、窗口 frame、命名窗口和不完整元数据仍为 `indeterminate`。MySQL/TiDB 不会仅因语法或函数名称而获得提升。

两种证明根不同：PostgreSQL 走 catalog identity（用连接的目录解析对象身份）；MySQL/TiDB 走内置的、与 profile 绑定的语义 manifest，连接只负责把 schema-qualified 表名和列名解析到真实物理对象，函数语义本身来自 profile，不依赖 catalog 身份。两者互不影响。

## 从方言专用会话 API 迁移

方言专用的会话类型、构造函数和分析函数已弃用，但仍保持导出且行为兼容：

| 已弃用 | 替代 |
|---|---|
| `PostgreSQLQueryAccessSession` | `OnlineQueryAccessSession` |
| `MySQLTiDBQueryAccessSession` | `OnlineQueryAccessSession` |
| `NewPostgreSQLQueryAccessSessionFromConn` | `NewOnlineQueryAccessSessionFromConn` |
| `NewMySQLTiDBQueryAccessSessionFromConn` | `NewOnlineQueryAccessSessionFromConn` |
| `AnalyzePostgreSQLQueryAccessWithSession` | `AnalyzeOnlineQueryAccessWithSession` |
| `AnalyzeMySQLTiDBQueryAccessWithSession` | `AnalyzeOnlineQueryAccessWithSession` |

让 `QueryAccessRequest.Dialect` 留空，由观察到的服务器身份选择 MySQL、TiDB 或 PostgreSQL 路由；非空方言只是一个可选的匹配约束。两种 API 中连接都由调用者拥有。统一入口返回自己的一组有界 `ErrOnlineQueryAccess...` 哨兵错误，它们不是方言专用错误的别名——请把 `errors.Is` 分支迁移到通用哨兵（例如 `ErrOnlineQueryAccessSessionUnavailable`、`ErrOnlineQueryAccessDialectMismatch`、`ErrOnlineQueryAccessCapabilityUnsupported`）。

## 纵深防御

查询访问分析是纵深防御授权策略中的一层。它确定查询触及哪些对象，但不评估调用者是否有权限访问这些对象。把它和下面这些措施配合使用：

- **认证**：在分析之前验证调用者身份。
- **授权评估**：把产出的要求与调用者已授予的权限比对。
- **行级安全**：独立于列级分析应用行过滤器。
- **审计日志**：记录分析结果和授权决策以供合规使用。

不要只靠静态分析做安全关键的授权决策。
