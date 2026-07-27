# DeltaScope v0.450.0 发行说明

## 概要 - 在线 MySQL/TiDB 字面量操作数精确形状扩展

v0.450.0 扩展了在线 MySQL/TiDB Query Access 的字面量操作数覆盖面。在调用方自有的 session（SDK）、在线 CLI 连接或 HTTP 注册的 `connection_id` 上，常见查询如 `SELECT COUNT(1) FROM app.orders` 或 `SELECT LOWER('x') FROM app.users` 现在返回 `admissible` 并附带精确的物理要求，而非 `indeterminate`。支持的 profile 为 MySQL 5.7、8.0、8.4 和 TiDB 8.5。

本版本不更改默认离线 SDK、CLI 或 HTTP 行为，MCP 仍无 Query Access 工具。Query Access 不执行用户 SQL。

## 变更内容

### 新增可接纳形状

MySQL/TiDB 内建语义清单现在在在线路径（调用方自有 `*sql.Conn`，通过 SDK、在线 CLI 连接或 HTTP `connection_id`）上接纳以下额外形状：

**一元纯字面量（`[const]` 操作数）：**

| 函数 | 示例 |
|------|------|
| `LOWER` | `SELECT LOWER('x') FROM app.users` |
| `UPPER` | `SELECT UPPER('x') FROM app.users` |
| `LENGTH` | `SELECT LENGTH('x') FROM app.users` |
| `CHAR_LENGTH` | `SELECT CHAR_LENGTH('x') FROM app.users` |
| `ABS` | `SELECT ABS(42) FROM app.users` |
| `CEIL` | `SELECT CEIL(42) FROM app.users` |
| `CEILING` | `SELECT CEILING(42) FROM app.users` |
| `FLOOR` | `SELECT FLOOR(42) FROM app.users` |

**聚合字面量（`[const]` 操作数）：**

| 函数 | 示例 |
|------|------|
| `COUNT(1)` | `SELECT COUNT(1) FROM app.orders` |

**反转二元（`[const, column]` 操作数）：**

| 函数 | 示例 |
|------|------|
| `COALESCE`、`NULLIF`、`IFNULL` | `SELECT NULLIF('x', name) FROM app.users` |

**全常量二元（`[const, const]` 操作数）：**

| 函数 | 示例 |
|------|------|
| `COALESCE`、`NULLIF`、`IFNULL` | `SELECT COALESCE('a', 'b') FROM app.users` |

每个形状在全部四个 profile 上被接纳（MySQL 5.7、8.0、8.4、TiDB 8.5），共计 60 个 profile-形状组合。

### 要求模型

每个被接纳的查询至少需要一个已解析的物理基关系：

- 已解析的物理基关系产生 `read_table`。
- 直接物理列产生 `read_column` 及其表读取。
- 字面量不产生表或列要求。
- 没有 `admissible` 结果会产生空要求列表。

例如，`SELECT NULLIF('x', name) FROM app.users` 要求 `app.users` 和 `app.users.name`；`SELECT COUNT(1) FROM app.orders` 仅要求 `app.orders`。

### 清单验证

`validateBuiltinSemanticEntry` 现在拒绝格式错误的固定 arity 条目：
- 当 `MinArity == 0` 且 `Arity > 0` 时，`len(OperandKinds) != Arity`
- Arity-0 条目带有非 star 操作数类型

回归测试：`TestBuiltinSemanticManifest_RejectsInvalidEntries`。

### 架构

MySQL/TiDB 路径完全绕过 `ValidatePhase1PureEffectCandidates`。在 `service.go` 中，MySQL/TiDB 内建网关（`proveBuiltinSemantics`）直接在候选上运行，不经过 Phase-1 准入过滤。PostgreSQL 路径（`resolveAndProveEffects`）调用 `ValidatePhase1PureEffectCandidates`，拒绝字面量操作数。这在为 MySQL/TiDB 启用字面量形状的同时保留了 PostgreSQL 边界。

### 此前已支持（自 v0.440.0 起不变）

`COALESCE(column, const)`、`NULLIF(column, const)` 和 `IFNULL(column, const)` 的 `[column, const]` 操作数顺序已在在线 MySQL/TiDB 路径上被接纳。v0.450.0 新增了反转的 `[const, column]` 和全常量 `[const, const]` 变体。

## 保持不变

- 默认离线 SDK、CLI 和 HTTP 行为不变。没有在线 session 时，含函数的查询仍为 `indeterminate`。
- CLI `--tls-mode` 默认为 `disabled`。启用 TLS 需要显式 `--tls-mode=enabled`。
- Query Access 仅发出结构化要求。它不认证调用方、不评估授权、不强制 RLS、不脱敏列、不自动授予权限、不重写 SQL、不保证后续执行快照。
- MCP 工具仍仅为 `audit_sql`、`describe_rule`、`list_rules` 和 `get_capabilities`。不添加 Query Access 工具。
- 审计规则目录和默认审计行为不变。`level` 仍是公开审计优先级字段；不引入 severity 字段。
- Query Access 结果不包含原始 SQL、字面量、函数名、DSN、凭证、驱动错误、会话数据、端点地址或密钥。

## 非目标

- 不是通用纯函数或 SELECT 准入。仅上述列出的精确形状被接纳；所有其他字面量、嵌套或多操作数形式仍为 `indeterminate`。
- 不是无关系的纯字面量 `SELECT`（无 FROM 子句）。每个被接纳的查询必须至少有一个已解析的物理基关系。
- 不是 3+ 操作数的 `COALESCE`/`NULLIF`/`IFNULL`。仅接纳恰好两个操作数。
- 不是 PostgreSQL 字面量操作数。字面量操作数扩展仅限 MySQL/TiDB 在线路径。
- 不是嵌套表达式、cast、参数、UDF、带引号/限定名的调用或任意函数支持。
- 不是 SQL 执行或数据返回 API。
- 不是数据库授权、grant、role、RLS、脱敏、重写或执行快照保证。
- 不是 MCP Query Access 工具。
- 不引入 severity 字段，注册的审计规则目录不变。

## 规则目录事实

注册的审计规则目录自 v0.440.0 起不变。本版本仅更改 Query Access 清单范围。

| 指标 | 数量 |
|------|------:|
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

- SQL 语料库：**582/582**，**100.0%**，**247** 个 YAML 测试夹具文件。
- PostgreSQL ALTER TABLE 配置条目：**53**。
- DDL 覆盖目录：**400** 条（mysql 61，tidb 54，postgresql 285，parser_upgrade_candidate 18）。

## 决策记录

- `docs/decisions/2026-07-26-query-access-literal-only-and-reversed-operands.md`（本版本）
- `docs/decisions/2026-07-22-query-access-cli-tls-and-credentials.md`（v0.440.0/v0.430.0）
- `docs/decisions/2026-07-20-query-access-online-connection-registry.md`（v0.420.0）
- MySQL/TiDB 内建语义清单（v0.410.0）：`docs/decisions/2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md`
- 通用纯效果 Query Access（v0.400.0）：`docs/decisions/2026-07-16-query-access-common-pure-effects.md`
- 受信 PostgreSQL SDK（v0.390.0）：`docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`
- Query Access 基础（v0.380.0）：`docs/decisions/2026-07-11-query-access-analysis-foundation.md`
