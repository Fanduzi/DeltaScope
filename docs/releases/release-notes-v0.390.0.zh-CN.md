# DeltaScope v0.390.0 发行说明

## 概要 — Trusted PostgreSQL Query Access SDK

v0.390.0 提供**可选（opt-in）**的 Trusted PostgreSQL Query Access SDK 路径，面向已持有实时 `*sql.Conn` 的调用方。在 **PG17** 连接上，且仅对封闭、已审计的 effect-identity manifest 覆盖的形态，在完成同连接元数据、类型与身份证明后，该路径可返回 `read_only` + `admissible`。

这**不是**全面的 PostgreSQL common SELECT 支持。默认 SDK、CLI 与 HTTP 对带副作用的 PostgreSQL 查询仍保持 fail-closed。Query Access 仍不做调用方认证、不评估授权、不强制 RLS、不脱敏列、不改写 SQL，也不保证后续执行快照与分析时刻一致。

不存在 `severity` 字段。结果不包含原始 SQL、字面量、凭据、连接串、驱动/目录内部信息或 parser 片段。MCP 工具集合仍为 `audit_sql`、`describe_rule`、`list_rules`、`get_capabilities` 四项；仍不提供 query-access MCP 工具。

## 改了什么

- 公共 SDK（postgresql 构建标签；非 postgresql 构建暴露相同符号并返回 `ErrPostgreSQLSessionNotAvailable`）：
  - `NewPostgreSQLQueryAccessSessionFromConn`
  - `AnalyzePostgreSQLQueryAccessWithSession`
- 调用方自有 `*sql.Conn` 契约：session 不会关闭连接；元数据与身份证明绑定同一连接（同连接 session 契约）。
- PG17 **manifest 门控**的 pure-read 准入，仅覆盖已证明的窄范围子集。
- 在 trusted 路径上经公开 E2E 验证的形态包括：
  - 针对 schema 限定基表的 `count(*)`
  - schema 限定的基列比较 / JOIN
- 未使用 trusted session 时，默认 `AnalyzeQueryAccess`、CLI `query-access analyze` 与 HTTP `POST /v1/query-access/analyze` 对上述带副作用的 PostgreSQL 场景仍保持 fail-closed。
- HTTP Query Access 错误响应有界，不回显原始包装后的内部错误。
- 决策记录：`docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`（Accepted；Related milestone/version: v0.390.0）。

## 哪些没变

- 现有 audit 行为、默认策略与已注册规则目录不变。
- MySQL 与 TiDB 的 Query Access 行为不变。
- MCP 工具集合不变（仍为四个工具；无 query-access 工具）。
- audit finding 仍使用 `level` 作为公开优先级字段。不引入 `severity` 字段。
- 查询访问结果无 `severity` 字段（no `severity` field），也不是 audit finding。
- 隐私/不泄漏：结构化结果契约不包含原始 SQL、字面量、凭据、连接串或 parser 片段。
- v0.380.0 基础能力面仍可用：SDK `AnalyzeQueryAccess`、CLI `query-access analyze`、HTTP `POST /v1/query-access/analyze`。

## 非目标

- 不做默认路径上的全面 PostgreSQL common SELECT 准入。
- 不做运行时授权评估、调用方认证或数据库会话鉴权。
- 不做行级安全评估、列脱敏、自动授权或 SQL 改写。
- 不保证后续执行使用与分析连接相同的快照。
- 不做 CLI/HTTP 的 trusted session 提升路径。
- 不做 MCP query-access 工具。
- 不做 MySQL/TiDB trusted 身份提升变更。
- 不改变现有 audit 行为或已注册规则目录。
- 不引入 `severity` 字段（no `severity` field）。

## 规则目录事实

已注册 audit 规则目录与 v0.380.0 相同。本版本新增可选的 trusted PostgreSQL SDK 路径，不是已注册规则的变更。

| 指标 | 数量 |
|------|----:|
| 规则总数 | **371** |
| blocker | 72 |
| warning | 142 |
| notice | 157 |

| 方言范围 | 规则 |
|----------|----:|
| common | 177 |
| postgresql | 191 |
| mysql | 1 |
| tidb | 2 |

| 语句类型 | 规则 |
|----------|----:|
| ddl | 361 |
| dml | 10 |

## 未变指标

- SQL 语料：**582/582**，**100.0%**，**247** YAML 夹具文件（未变）。
- PostgreSQL ALTER TABLE 配置条目：**53**（未变）。
- DDL 覆盖目录：**400** 条目（未变；mysql 61、tidb 54、postgresql 285、parser_upgrade_candidate 18）。

## 决策记录

- `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`（本版本）
- 既有 foundation（v0.380.0）：`docs/decisions/2026-07-11-query-access-analysis-foundation.md`
- 既有 lineage（v0.380.0）：`docs/decisions/2026-07-11-cte-derived-table-lineage-resolution.md`
