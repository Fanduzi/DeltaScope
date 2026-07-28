# DeltaScope v0.460.0 发行说明

## 概要 - 在线 MySQL/TiDB 无关系纯字面量查询准入

v0.460.0 移除了在线 MySQL/TiDB session 上精确纯字面量清单条目对 FROM 子句的要求。调用方自有的 session（SDK）、在线 CLI 连接或 HTTP 注册的 `connection_id` 上，`SELECT LOWER('x')`、`SELECT COUNT(1)`、`SELECT COALESCE('a','b')` 等查询现在返回 `read_only` + `admissible`，要求为空——无表、无关系、无引用列。支持的 profile 为 MySQL 5.7、8.0、8.4 和 TiDB 8.5。

本版本不更改默认离线 SDK、CLI 或 HTTP 行为，MCP 仍无 Query Access 工具。Query Access 不执行用户 SQL。

## 变更内容

### 无关系纯字面量形状

v0.450.0 已接纳的精确纯字面量形状（带 FROM 子句）现在不带 FROM 子句也能通过。在线路径（调用方自有 `*sql.Conn`，通过 SDK、在线 CLI 连接或 HTTP `connection_id`）上，以下查询返回 `read_only` + `admissible`，要求为空：

**一元纯字面量（`[const]` 操作数）：**

| 函数 | 示例 |
|------|------|
| `LOWER` | `SELECT LOWER('x')` |
| `UPPER` | `SELECT UPPER('x')` |
| `LENGTH` | `SELECT LENGTH('x')` |
| `CHAR_LENGTH` | `SELECT CHAR_LENGTH('x')` |
| `ABS` | `SELECT ABS(42)` |
| `CEIL` | `SELECT CEIL(42)` |
| `CEILING` | `SELECT CEILING(42)` |
| `FLOOR` | `SELECT FLOOR(42)` |

**聚合字面量（`[const]` 操作数）：**

| 函数 | 示例 |
|------|------|
| `COUNT(1)` | `SELECT COUNT(1)` |

**全常量二元（`[const, const]` 操作数）：**

| 函数 | 示例 |
|------|------|
| `COALESCE`、`NULLIF`、`IFNULL` | `SELECT COALESCE('a', 'b')` |

每个形状在全部四个 profile 上被接纳（MySQL 5.7、8.0、8.4、TiDB 8.5）。无关系路径不在结果中添加表或列要求。

### 要求模型

无关系纯字面量查询产生空要求列表：

- 不存在已解析的物理基关系，因此不产生 `read_table` 条目。
- 未引用直接物理列，因此不产生 `read_column` 条目。
- 每个字面量操作数不产生要求。
- 结果为 `read_only` + `admissible`，要求、关系、引用列均为空。

要求为空表示静态分析未发现数据库对象读取。这不是授权、grant、RLS、脱敏、SQL 模式或执行权限。

### 此前已支持（自 v0.450.0 起不变）

v0.450.0 在带 FROM 子句时已接纳在线 MySQL/TiDB 路径上的纯字面量、反转和全常量操作数形状。该行为不变。v0.460.0 将相同的纯字面量形状扩展到无 FROM 子句的情况。

## 保持不变

- 默认离线 SDK、CLI 和 HTTP 行为不变。没有在线 session 时，含函数的查询仍为 `indeterminate`。
- CLI `--tls-mode` 默认为 `disabled`。启用 TLS 需要显式 `--tls-mode=enabled`。
- Query Access 仅发出结构化要求。它不认证调用方、不评估授权、不强制 RLS、不脱敏列、不自动授予权限、不重写 SQL、不保证后续执行快照。
- MCP 工具仍仅为 `audit_sql`、`describe_rule`、`list_rules` 和 `get_capabilities`。不添加 Query Access 工具。
- 审计规则目录和默认审计行为不变。`level` 仍是公开审计优先级字段；不引入 severity 字段。
- Query Access 结果不包含原始 SQL、字面量、函数名、DSN、凭证、驱动错误、会话数据、端点地址或密钥。
- Query Access 不执行用户 SQL，不返回数据。

## 非目标

- 不是通用纯函数或 SELECT 准入。仅上述列出的精确形状在无 FROM 子句时被接纳；所有其他纯字面量、嵌套或多操作数形式仍为 `indeterminate`。
- 不是 `SELECT 1`（无候选）。无候选查询保持现有行为，不属于本版本范围。
- 不是含关系或含列的查询。这些查询仍需要物理要求证明（不变）。
- 不是 3+ 操作数的 `COALESCE`/`NULLIF`/`IFNULL`。仅接纳恰好两个字面量操作数。
- 不是默认/离线 SDK、CLI、HTTP、PostgreSQL 或 MCP。这些路径保持不变且 fail-closed。
- 不是参数、cast、运算符、嵌套函数、子查询、UDF、带引号/限定名/非规范调用、未知函数或不支持的修饰符。
- 不是 JSON 字段、授权标志或免权限标志。
- 不是 SQL 执行或数据返回 API。
- 不是数据库授权、grant、role、RLS、脱敏、重写或执行快照保证。
- 不是 MCP Query Access 工具。
- 不引入 severity 字段，注册的审计规则目录不变。
- `tls_mode` 默认仍为 disabled。

## 规则目录事实

注册的审计规则目录自 v0.450.0 起不变。本版本仅更改 Query Access 清单范围。

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

- `docs/decisions/2026-07-28-query-access-relationless-literal-selects.md`（本版本）
- `docs/decisions/2026-07-26-query-access-literal-only-and-reversed-operands.md`（v0.450.0）
- `docs/decisions/2026-07-22-query-access-cli-tls-and-credentials.md`（v0.440.0/v0.430.0）
- `docs/decisions/2026-07-20-query-access-online-connection-registry.md`（v0.420.0）
- MySQL/TiDB 内建语义清单（v0.410.0）：`docs/decisions/2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md`
- 通用纯效果 Query Access（v0.400.0）：`docs/decisions/2026-07-16-query-access-common-pure-effects.md`
- 受信 PostgreSQL SDK（v0.390.0）：`docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`
- Query Access 基础（v0.380.0）：`docs/decisions/2026-07-11-query-access-analysis-foundation.md`
