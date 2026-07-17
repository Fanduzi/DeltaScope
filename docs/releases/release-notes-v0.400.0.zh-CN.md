# DeltaScope v0.400.0 发行说明

## 概要 - Common Pure-Effect Query Access

v0.400.0 扩展既有的**可选（opt-in）** Trusted PostgreSQL Query Access SDK。在调用方自有的 **PG17** `*sql.Conn` 上，只有在同连接元数据、类型、目录身份和已审计 manifest 都证明通过后，查询才可能返回 `read_only` + `admissible`。

新增已证明范围包括：`COUNT(*)`、`COUNT(base_column)`、直接基列上的带类型 `SUM` / `AVG` / `MIN` / `MAX`，以及直接基列窗口分区和排序依赖下的 `ROW_NUMBER` / `RANK` / `DENSE_RANK`。这仍不是全面的 PostgreSQL common SELECT 支持，也不是鉴权。

默认 SDK、CLI、HTTP 对带 effect 的 PostgreSQL 查询仍 fail-closed。MySQL 和 TiDB 尚未交付方言专属身份证明模型，因此聚合/窗口 effect 仍 fail-closed。MCP 仍没有 Query Access 工具。

## 改了什么

- PG17 trusted SDK manifest 现在可在调用方连接上完成目录身份校验后，证明这组有界聚合和排名窗口形态。
- 严格 requirements 提取现在会在准入判断前覆盖聚合参数、窗口 partition/order 依赖、`DISTINCT ON`、聚合 `FILTER` 和 LIMIT/OFFSET 子查询。
- 显式窗口 frame 会被识别并保持排除。`FILTER`、DISTINCT、命名窗口、ordered-set 聚合、嵌套表达式、cast、view、wildcard、参数、未限定关系、未解析元数据和非 manifest identity 都保持 `indeterminate`。
- MySQL/TiDB 的依赖提取补齐 derived subquery 和 ordering 缺口，但不会把带函数查询提升为 admissible；聚合/窗口结果仍是 `unknown_function_effect` / `indeterminate`。
- 决策记录：`docs/decisions/2026-07-16-query-access-common-pure-effects.md`（Accepted；Related milestone/version: v0.400.0）。

## 保持不变

- 公共 trusted 入口仍是 `NewPostgreSQLQueryAccessSessionFromConn` 和 `AnalyzePostgreSQLQueryAccessWithSession`；DeltaScope 不会关闭调用方自有连接。
- 默认 `AnalyzeQueryAccess`、CLI `query-access analyze` 和 HTTP `POST /v1/query-access/analyze` 不会打开 trusted metadata 连接，对上述 PostgreSQL effect 场景仍 fail-closed。
- Query Access 只输出静态 requirements，不认证调用方、不评估 grants、不执行 RLS、不脱敏列、不自动授权、不改写 SQL，也不保证后续执行快照。
- MCP 工具仍仅有 `audit_sql`、`describe_rule`、`list_rules`、`get_capabilities`。
- audit 规则目录和默认 audit 行为不变。`level` 仍是公开 audit 优先级字段；不引入 `severity` 字段。
- Query Access 结果不包含原始 SQL、字面量、凭据、连接串、OID、目录内部信息、会话数据或 parser 片段。

## 非目标

- 不做全面 PostgreSQL common SELECT 或任意函数支持。
- 不做 MySQL/TiDB identity manifest 或 trusted promotion 变更。
- 不做 CLI/HTTP trusted-session promotion 或 MCP Query Access 工具。
- 不做运行时鉴权、grant 评估、RLS、脱敏、自动授权、SQL 改写或执行亲和性保证。
- 不改变已注册 audit 规则目录，也不引入 `severity` 字段。

## 规则目录事实

已注册 audit 规则目录与 v0.390.0 相同。本版本只改变有界 Query Access 证明路径。

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

- SQL 语料：**582/582**，**100.0%**，**247** YAML 夹具文件。
- PostgreSQL ALTER TABLE 配置条目：**53**。
- DDL 覆盖目录：**400** 条目（mysql 61、tidb 54、postgresql 285、parser_upgrade_candidate 18）。

## 决策记录

- `docs/decisions/2026-07-16-query-access-common-pure-effects.md`（本版本）
- Trusted PostgreSQL SDK（v0.390.0）：`docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`
- Query Access foundation（v0.380.0）：`docs/decisions/2026-07-11-query-access-analysis-foundation.md`
