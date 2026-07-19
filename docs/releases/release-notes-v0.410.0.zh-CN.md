# DeltaScope v0.410.0 发行说明

## 概要 - MySQL/TiDB Builtin Semantic Manifests

v0.410.0 引入**可选（opt-in）** 的 MySQL/TiDB builtin semantic manifest 证明模型。在调用方自有的 MySQL 或 TiDB `*sql.Conn` 上，带配置的查询只有在同连接元数据解析、parser-native-form 证明、完整 candidate 闭包、以及不可变 manifest 查找都通过后，才可能返回 `read_only` + `admissible`。

已证明范围包括：四个 profile 的 `COUNT(*)`、直接列 `COUNT` / `SUM` / `AVG` / `MIN` / `MAX`，以及 MySQL 8.0、MySQL 8.4、TiDB 8.5 的 `ROW_NUMBER` / `RANK` / `DENSE_RANK`（直接列分区和排序依赖）。MySQL 5.7 没有原生 ranking-window 支持，其 ranking-window 条目保持延迟。这仍不是全面的 MySQL/TiDB common SELECT 支持，也不是鉴权。

默认 SDK、CLI、HTTP 对带函数的 MySQL/TiDB 查询仍 fail-closed。显式同连接 SDK 会话（`AnalyzeMySQLTiDBQueryAccessWithSession`）是唯一的提升路径。MCP 仍没有 Query Access 工具。PostgreSQL 行为保持不变。

## 变更内容

- 新增公共 SDK 会话边界：`NewMySQLTiDBQueryAccessSessionFromConn` 和 `AnalyzeMySQLTiDBQueryAccessWithSession` 接受调用方自有的 `*sql.Conn`，拒绝外部 schema resolver，内部构造私有语义能力。
- 新增闭合公共 profile 枚举：`QueryAccessAnalysisProfileMySQL57` / `MySQL80` / `MySQL84` / `TiDB85`。未知值和方言不匹配返回 bounded validation error。
- 不可变生产 builtin semantic registry 为四个 profile 填充，每个条目由 primary documentation 和 exact server image 的 live Docker 探针支持（`mysql:5.7.44`、`mysql:8.0.46`、`mysql:8.4.10`、`pingcap/tidb:v8.5.7`）。
- Parser effect-candidate 收集器遍历 projection、WHERE、HAVING、GROUP BY、ORDER BY、LIMIT/OFFSET、join 条件、derived table、CTE、标量子查询、集合操作、聚合修饰符和窗口分区/排序/frame 表达式。不支持的节点发出显式 fail-closed 标记。
- Gateway 强制 canonical native form（拒绝 quoted、schema-qualified、noncanonical spacing、`IGNORE_SPACE` 依赖歧义），完整 candidate 闭包（无重复/间隙/外来源 ordinal），严格物理要求（ModeStrict、无 unresolved、无 view/CTE/derived/wildcard、完整 `read_table`/`read_column` 要求），以及 ranking window 的 `RequireWindowPartition`/`RequireWindowOrder`。
- CLI `query-access analyze --profile` 和 HTTP `POST /v1/query-access/analyze` 接受 profile 输入但保持离线和 indeterminate（无显式会话 API）。
- 决策记录：`docs/decisions/2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md`（Accepted；关联 milestone/version：v0.410.0）。
- 证据账本：`docs/decisions/2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests-evidence-ledger.md`。

## 保持不变

- 默认 `AnalyzeQueryAccess`、CLI `query-access analyze`、HTTP `POST /v1/query-access/analyze` 不会打开数据库连接，对带函数的 MySQL/TiDB 查询保持 fail-closed。
- Query Access 只发出静态要求。它不鉴权调用者、不评估 grant、不强制 RLS、不掩码列、不自动授权、不重写 SQL、不保证后续执行快照。
- MCP 工具仍只有 `audit_sql`、`describe_rule`、`list_rules`、`get_capabilities`。
- 审计规则目录和默认审计行为保持不变。`level` 仍是公共审计优先级字段；不引入 `severity` 字段。
- Query Access 结果不包含 raw SQL、literals、函数名、manifest 内部、profile 内部、candidates、parser facts、DSN、凭据、driver 错误或会话数据。不引入 `severity` 字段。
- PostgreSQL 的 controlled-session catalog/OID 证明路径保持不变。MySQL/TiDB manifest 不能影响 PostgreSQL，反之亦然。

## 非目标

- 不是通用函数名 allowlist、volatility-only allowlist 或 caller-supplied manifest。
- 不支持任意或用户定义函数、stored function、plugin/loadable UDF、或 schema-qualified/quoted 调用。
- 不支持 MySQL/TiDB cast、operator、literal、parameter、嵌套表达式、`FILTER`、`DISTINCT`、aggregate-local `ORDER BY`、named window、explicit frame、ordered-set 行为或 broad common `SELECT`。
- 不是默认 SDK/CLI/HTTP 中的数据库连接或运行时身份查找。
- 不是 MCP Query Access 工具。
- 不是运行时鉴权、grant 评估、RLS、掩码、自动授权、SQL 重写或执行快照证明。
- 不改变已注册审计规则目录，也不引入 `severity` 字段。

## 支持矩阵

| Profile | 方言 | 聚合 | Ranking Window | Live Docker Image |
|---|---|---|---|---|
| `mysql-5.7` | mysql | `COUNT(*)`、`COUNT`/`SUM`/`AVG`/`MIN`/`MAX`（直接列） | 延迟（5.7 无原生支持） | `mysql:5.7.44` |
| `mysql-8.0` | mysql | 同 5.7 | `ROW_NUMBER`/`RANK`/`DENSE_RANK`（直接分区+排序列） | `mysql:8.0.46` |
| `mysql-8.4` | mysql | 同 5.7 | 同 8.0 | `mysql:8.4.10` |
| `tidb-8.5` | tidb | 独立证明 | 独立证明 | `pingcap/tidb:v8.5.7` |

Ranking-window 条目要求 `PARTITION BY` 和 `ORDER BY` 子句都有直接物理基列操作数。这比 MySQL 语法契约（接受无 `ORDER BY` 的 ranking window）更严格，以保持严格的分区/排序依赖边界。

## 规则目录事实

已注册审计规则目录自 v0.400.0 起不变。本版本仅变更 Query Access 的 MySQL/TiDB 语义证明路径。

| 指标 | 数量 |
|--------|------:|
| 总规则数 | **371** |
| blocker | 72 |
| warning | 142 |
| notice | 157 |

| 方言范围 | 规则数 |
|---------------|------:|
| common | 177 |
| postgresql | 191 |
| mysql | 1 |
| tidb | 2 |

| 语句类型 | 规则数 |
|----------------|------:|
| ddl | 361 |
| dml | 10 |

## 不变指标

- SQL corpus：**582/582**，**100.0%**，**247** YAML fixture 文件。
- PostgreSQL ALTER TABLE 配置条目：**53**。
- DDL 覆盖目录：**400** 条目（mysql 61、tidb 54、postgresql 285、parser_upgrade_candidate 18）。

## 决策记录

- `docs/decisions/2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md`（本版本）
- Common pure-effect Query Access（v0.400.0）：`docs/decisions/2026-07-16-query-access-common-pure-effects.md`
- Trusted PostgreSQL SDK（v0.390.0）：`docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`
- Query Access 基础（v0.380.0）：`docs/decisions/2026-07-11-query-access-analysis-foundation.md`
