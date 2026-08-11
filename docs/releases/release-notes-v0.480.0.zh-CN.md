# DeltaScope v0.480.0 发行说明

## 概要 - PG17 `COUNT(1)` 在线 Query Access

v0.480.0 证明并上线一条精确的 PostgreSQL 17 Query Access 语句边界：在调用方持有的在线 session 上，仅支持 `SELECT COUNT(1) FROM <一个 schema 限定的物理基表>`。同一条基于目录证明的会话路径，可通过受信任 SDK、CLI 既有在线 PostgreSQL 连接选项，以及带操作员授权 PostgreSQL 17 `connection_id`（用途为 `query_access`）的 HTTP 使用。分析成功时返回 `read_only` + `admissible`，且只产生该基表的一条 `read_table` 要求。

这只是静态需求分析结果。DeltaScope 不执行提交的 SQL、不取回查询结果，也不做授权、授权清单、RLS 或脱敏判定。默认离线 SDK/CLI/HTTP、其他一切 PostgreSQL 字面量/聚合形态、MySQL/TiDB Query Access、审核规则目录以及 MCP 均保持不变。MCP 仍然没有 Query Access 工具。

## 变更内容

### 精确的 PG17 `COUNT(1)` 语句边界

本次新接纳的 PostgreSQL 语句形态只有：

```sql
SELECT COUNT(1) FROM app.orders
```

其中 `app.orders` 必须是一个 schema 限定且已解析的物理基表。要求：

- 方言与服务器身份：调用方持有的在线 PostgreSQL 17 session。
- 聚合身份：会话绑定的 `pg_catalog.count(any)` 目录证明。
- 参数：仅未加 cast 的整数常量 `1`。解析器可记录内部、不序列化的 `integer_one` 语法事实；不保留、不暴露字面量原文。
- 关系：恰好一个 schema 限定的物理基表。不允许 join、逗号 join、CTE、视图、派生表或未限定名。
- 子句：不允许 `WHERE`、`GROUP BY`、`ORDER BY`、`LIMIT`、`DISTINCT`、`FILTER`、窗口、嵌套调用、额外选择列表项、集合运算，以及参数/cast/表达式。

成功结果：`read_only` + `admissible`，仅含该基表的 `read_table` 要求，无引用列。

### 在线表面

| 表面 | 支持路径 | 不变 / fail-closed |
|------|----------|--------------------|
| 受信任 SDK | 调用方持有的在线 `*sql.Conn` PostgreSQL 17 session | 默认离线 SDK 对该查询仍为 indeterminate |
| CLI | 既有在线 PostgreSQL 连接选项 | 默认/离线 CLI 仍为 indeterminate |
| HTTP | 操作员配置且已授权的 `query_access` 用途 `connection_id` | 无 `connection_id` 的 HTTP 仍为 indeterminate |
| MCP | — | 无 Query Access 工具；不变 |

CLI 与 HTTP 复用同一条会话绑定的目录证明，不新增传输层特性开关、解析路径或信任根。HTTP 客户端从不提供 endpoint、凭据、密钥来源、TLS 设置、profile 或服务器版本声明；`connection_id` 选择始终由操作员控制并完成授权。

### 不执行与不泄漏保证

- 提交的 SQL 不会被执行、prepare 或 explain。
- 公开结果与日志不暴露 SQL/字面量标记、连接信息、凭据、目录数据或原始驱动错误。
- 录制驱动与真实传输证据覆盖正例、排除形态、连接失败、目录查找失败，以及 HTTP 未授权/未知 `connection_id` 路径。

### Fail-closed 排除项

以下形态仍为 `indeterminate`（不被接纳）：

- `COUNT(NULL)`、`COUNT(2)`、`COUNT('1')`、参数、cast、表达式、嵌套调用与任意元数
- 无关系 `SELECT COUNT(1)`
- 未限定 `FROM orders`、视图、派生表、CTE、join 与多关系形态
- `FILTER`、窗口、`DISTINCT`、排序、分组、limit、额外选择列表项与集合运算
- 默认/离线 SDK、CLI、HTTP
- 任意路径上的 MCP
- 除上述精确边界以外的一切 PostgreSQL 字面量、标量、二元函数或聚合形态

既有 MySQL/TiDB 在线纯字面量与无关系形态保持不变，不属于本次 PostgreSQL 证明范围。

## 保持不变

- SQL 审核行为、已注册审核规则目录与默认审核输出不变。`level` 仍是公开审核优先级字段；不引入 severity 字段。
- 默认离线 SDK、CLI、HTTP 对带函数的 Query Access 保持 fail-closed，直到操作员或本地用户主动建立受支持的在线 session。
- MCP 工具仍仅为 `audit_sql`、`describe_rule`、`list_rules`、`get_capabilities`。
- Query Access 仍不认证调用方、不评估授权清单、不强制 RLS、不脱敏列、不改写 SQL、不自动授权，也不保证后续执行快照。
- 既有 MySQL/TiDB Query Access 契约不变。
- 既有 release tag、GitHub Release、npm 包与 Homebrew cask 均未改动。

## 非目标

- 不是通用 PostgreSQL 字面量支持、纯函数接纳或聚合接纳。
- 不是授权、授权清单、角色、RLS、脱敏、改写或执行快照保证。
- 不是 SQL 执行、对用户 SQL 的 prepare/explain，或返回数据的 API。
- 不是把默认/离线 SDK、CLI、HTTP 扩展到该查询。
- 不是 MCP Query Access 工具。
- 不是无关系 PostgreSQL `COUNT(1)`，也不是其他字面量、cast、参数、修饰符、join、CTE、视图、派生表或未限定来源。
- 不是把 MySQL/TiDB 的 profile/manifest 模型复用为 PostgreSQL 证明。
- 不是 severity 字段；不是已注册审核规则目录变更。
- 不是改动任何既有已发布产物或既有 tag。

## 规则目录事实

已注册审核规则目录相对 v0.470.0 不变。本版本仅变更 PostgreSQL Query Access 证明与在线表面契约。

| 指标 | 数量 |
|------|-----:|
| 规则总数 | **371** |
| blocker | 72 |
| warning | 142 |
| notice | 157 |

| 方言范围 | 规则数 |
|----------|-------:|
| common | 177 |
| postgresql | 191 |
| mysql | 1 |
| tidb | 2 |

| 语句类型 | 规则数 |
|----------|-------:|
| ddl | 361 |
| dml | 10 |

## 不变指标

- SQL corpus：**582/582**，**100.0%**，**247** 个 YAML fixture 文件。
- PostgreSQL ALTER TABLE 配置条目：**53**。
- DDL 覆盖目录：**400** 条（mysql 61，tidb 54，postgresql 285，parser_upgrade_candidate 18）。

## 决策记录

- `docs/decisions/2026-08-03-query-access-pg17-count-online-surface-contract.md`（本版本）
- `docs/decisions/2026-07-31-query-access-pg17-count-literal-proof.md`（本版本）
- `docs/decisions/2026-07-30-release-recovery-provenance-enforcement.md`（v0.470.0）
- `docs/decisions/2026-07-28-query-access-relationless-literal-selects.md`（v0.460.0）
