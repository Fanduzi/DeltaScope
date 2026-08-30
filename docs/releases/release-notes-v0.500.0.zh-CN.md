# DeltaScope v0.500.0 发行说明

## 概要 - 解析恢复、Query Access 契约与元数据闭合

v0.500.0 发布 v0.490.0 之后落地的 #31–#53 源码工作。混合迁移在一条语句解析失败时仍保留有效语句。审计与 Query Access 会去掉一个前导 UTF-8 BOM。Query Access 空 `--sql`、仅审计 flag、MySQL/TiDB schema 绑定，以及 PostgreSQL 17 版本边界与文档契约对齐。PostgreSQL 的 database/schema/port/MCP 目录选择与有界离线 DML 影响估计在各接入面一致。MySQL/TiDB 元数据感知规则覆盖缺失的 DML 目标、MODIFY 可空性重述、列级 PRIMARY KEY，以及 ALTER TABLE ADD INDEX。PR 工作流会跑 SQL corpus 门禁。代表性 MySQL 与 PostgreSQL fixture 已扩充。

这仍是静态分析。DeltaScope 不执行提交的 SQL、不取回查询结果，也不做授权、授权清单、RLS 或脱敏判定。MCP 仍然没有 Query Access 工具。PostgreSQL 16 以及受信任 PG17 系列之外的版本仍在 Query Access 信任边界之外。85/100 源码就绪度（source-readiness）是对本源码里程碑的陈述，不是 SLA，也不是 SQL 语法覆盖率。已发布的 v0.490.0 仍是此前的包，分数为 58/100，因为它早于这些变更。

## 变更内容

### 解析恢复与 BOM

- 多语句输入先按有界词法分号边界切开，再由所选方言 parser 独立解析。有效相邻语句按原文顺序保留 findings、影响估计和源位置。
- 每个失败切片贡献一条 `parser_error` 诊断，`audited=false`，可选 1-based `line` / `column`。任一失败切片仍使审计错误非空，CLI 退出码为 2。
- SDK、CLI、HTTP、MCP 序列化同一份部分结果。诊断不含原始 SQL 或 parser 内部信息。
- 共享输入边界会去掉恰好一个前导 UTF-8 BOM，再做空输入校验和 parser 分发。仅含 BOM 的输入视为空。

### Query Access 契约

- 显式空或仅空白的 `query-access analyze --sql` 以退出码 3 失败，消息为 `SQL input must not be empty`，且不读取 stdin。
- `--format` 与 `--fail-on` 仍仅属于审计。传给 Query Access 是不支持的用法错误，退出码 3。
- 完整、无效果的 `read_only` 结果返回 `admissible`。`not_read_only` 返回 `rejected`。未解决的要求保持 `indeterminate`，并带稳定 reason code。
- 在线 MySQL/TiDB Query Access 把 database 与连接 schema 当作目录别名。冲突值在分析前失败。PostgreSQL 的 database 与 schema 保持独立。
- PostgreSQL 16 及其他非 PG17 服务器仍在 Query Access 之外。共享身份解析返回有界版本要求，而不是折叠成泛化的连接失败。审计连通性不变。

### PostgreSQL 边界与影响估计

- 元数据感知 PostgreSQL 在显式 schema 但没有 `--database` 时拒绝。仅 database、两者都给、或两者都不给，行为不变。
- 显式 `--dialect postgresql` 且未给 port 时解析为 `5432`。显式 port 以及自动探测的 MySQL/TiDB 仍为 `3306`。
- MCP `audit_sql` 接受可选的 `connection.database`（直接连接与命名连接）。`get_capabilities` 公布 `connection.database`。
- 离线 PostgreSQL `DELETE`/`UPDATE` 在单表 `id =` 字面量或 `$1` 等值时使用既有形状估计（`estimated_rows=1`，`source=shape`，`reason_codes=["pk_equality"]`）。在线 planner 输出仍为 `source=plan` 并优先。其他未证明形状保持保守。

### MySQL/TiDB 元数据规则

- 列级 `PRIMARY KEY` 填入 `DDL.PrimaryKey`，并满足 `ddl.table.primary_key.require`。列级 `UNIQUE` 不是主键。
- 元数据感知的 MySQL/TiDB `INSERT`/`UPDATE`/`DELETE` 在 provider 确认目标缺失时发出 `dml.table.exists.require`。离线与 PostgreSQL 请求不产生该 finding。
- 已确认的 `MODIFY COLUMN` 可空性重述不再触发转换 blocker。前置状态未知时发出 notice `ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory`。
- `ALTER TABLE ADD INDEX` / `ADD KEY` 规范化为 `add_index` 与 `ddl.create_index.notice`。真正的 `ADD CONSTRAINT` 仍为 `add_constraint`。
- 共享生命周期 notice 为 `RENAME TABLE`、`CREATE INDEX`、`DROP INDEX` 渲染规范化标识符。
- CLI `--database` 在省略 `--schema` 时是 MySQL/TiDB 的目录别名。冲突值是有界用法错误。

### 操作员、语料与 CI

- 未固定版本的 `install.sh` 在 GitHub REST `releases/latest` 失败时回退到公开 latest-release 重定向。`DELTASCOPE_VERSION` 仍是第一条路径。
- TLS 元数据连接失败区分主机名不匹配、未知 CA、以及服务器未提供 TLS，退出码仍为 3。
- 每个 PR 运行 `make sql-corpus-gates`。报告的 100% 是支持的 rule-and-dialect fixture coverage，不是 SQL syntax 或 grammar coverage。
- 代表性 MySQL 与 PostgreSQL corpus fixture 已扩充（MySQL 32 YAML，TiDB 24，PostgreSQL 230）。

## 保持不变

- MCP 工具仍仅为 `audit_sql`、`describe_rule`、`list_rules`、`get_capabilities`。
- Query Access 仍不认证调用方、不评估授权、不执行 RLS、不脱敏、不改写 SQL、不自动授权，也不保证后续执行快照。
- 既有 MySQL/TiDB Query Access 接纳边界，以及 v0.480.0 的精确 PG17 `COUNT(1)` 边界，除上文契约外不变。
- `level` 仍是公开优先级字段；不引入 severity 字段。
- 既有 release tag、GitHub Release、npm 包和 Homebrew cask 在本 tag 发布前保持不动。

## 非目标

- 不是 MCP Query Access 工具。
- 不是授权、授权清单、角色、RLS、脱敏、改写、SQL 执行或返回数据的 API。
- 不是在已发布 PG17 边界之外扩展通用 PostgreSQL Query Access。
- 不是 PostgreSQL 16 Query Access 支持。
- 不是失败语句的回退语法、token-to-AST 恢复或语义猜测。
- 不是 SQL syntax 或 grammar coverage；100.0% 是支持的 rule-and-dialect fixture coverage。
- 不是 SLA。源码就绪度 85/100 描述本源码里程碑在 DBA、应用与 CI dogfood 之后的状态。它不是自动指标，也不是此前已发布包的分数。
- 不是 severity 字段；不把 `context` 放进 `pkg/deltascope.Result`。
- 不改变任何既有已发布产物或已有 tag。

## 规则目录事实

已注册审核规则目录相对 v0.490.0 新增两条 MySQL/TiDB 规则：`dml.table.exists.require`（blocker）和 `ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory`（notice）。

| 指标 | 数量 |
|------|------:|
| 规则总数 | **373** |
| blocker | 73 |
| warning | 142 |
| notice | 158 |

| 方言范围 | 规则数 |
|----------|------:|
| common | 177 |
| postgresql | 191 |
| mysql | 1 |
| tidb | 2 |
| mysql and tidb | 2 |

| 语句种类 | 规则数 |
|----------|------:|
| ddl | 362 |
| dml | 11 |

## 语料与目录事实

- 支持的 rule-and-dialect fixture coverage：**586/586**，**100.0%**，**286** 个 YAML fixture；这不是 SQL syntax 或 grammar coverage。
- PostgreSQL ALTER TABLE 配置项：**53**。
- DDL 覆盖目录：**407** 条（mysql 62，tidb 55，postgresql 290，parser_upgrade_candidate 18）。

## 决策记录

- `docs/decisions/2026-08-30-partial-parser-error-recovery.md`（本版本）
- `docs/decisions/2026-08-30-leading-utf8-bom-sql-input.md`（本版本）
- `docs/decisions/2026-08-17-cli-explicit-empty-sql-input-source.md`（本版本；#32）
- `docs/decisions/2026-08-30-query-access-cli-flag-ownership.md`（本版本）
- `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`（本版本；#35 修正）
- `docs/decisions/2026-08-30-query-access-mysql-tidb-schema-binding.md`（本版本）
- `docs/decisions/2026-08-30-query-access-postgresql-version-error-contract.md`（本版本）
- `docs/decisions/2026-08-30-cli-postgresql-schema-database-validation.md`（本版本）
- `docs/decisions/2026-08-30-cli-postgresql-default-port.md`（本版本）
- `docs/decisions/2026-08-30-mcp-postgresql-database-selection.md`（本版本）
- `docs/decisions/2026-08-30-postgresql-offline-impact-contract.md`（本版本）
- `docs/decisions/2026-08-30-mysql-tidb-column-primary-key-extraction.md`（本版本）
- `docs/decisions/2026-08-30-mysql-tidb-dml-table-existence.md`（本版本）
- `docs/decisions/2026-08-30-mysql-tidb-modify-nullability-state.md`（本版本）
- `docs/decisions/2026-08-30-mysql-tidb-alter-index-action-normalization.md`（本版本）
- `docs/decisions/2026-08-30-issue-48-lifecycle-notice-identifiers.md`（本版本）
- `docs/decisions/2026-08-30-cli-mysql-tidb-database-schema-alias.md`（本版本）
- `docs/decisions/2026-08-30-cli-tls-metadata-error-categories.md`（本版本）
- `docs/decisions/2026-08-30-release-installer-latest-fallback.md`（本版本）
- `docs/decisions/2026-08-30-pr-sql-corpus-coverage-contract.md`（本版本）
- `docs/decisions/2026-08-30-source-readiness-85-dogfood.md`（本版本）
- `docs/decisions/2026-08-20-offline-existence-caveat-context.md`（v0.490.0）
