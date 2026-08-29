# DeltaScope v0.490.0 发行说明

## 概要 - 审计契约与统一在线 Query Access

v0.490.0 发布 v0.480.0 之后落地的操作员/agent 审计契约修复，以及统一的 Query Access 在线 session。离线 ALTER 不再声称已检查对象是否存在。MCP `audit_sql` 文本带上 findings，`list_rules` 返回精简目录行。CLI 空 `--sql`、元数据连接退出码、生成配置空字符串 YAML，以及内嵌 DDL 覆盖目录与文档契约对齐。Query Access 的 SDK、CLI、HTTP 共用一个在线入口；方言专用 session 构造函数仍可用，但已弃用。

这仍是静态分析。DeltaScope 不执行提交的 SQL、不取回查询结果，也不做授权、授权清单、RLS 或脱敏判定。MCP 仍然没有 Query Access 工具。已注册审核规则目录不变。Query Access 的不泄漏（No-leak）证据不变。

## 变更内容

### 统一 Query Access 在线 Session

- 新的公共构造函数：`NewOnlineQueryAccessSessionFromConn`。CLI 与 HTTP 在线 Query Access 走该共享入口。
- 方言专用 session API 仍保留，但已弃用，请改用统一构造函数。
- PostgreSQL resolver 所有权仅限连接。已移除 DB-backed 元数据 resolver。证明提升收束到一条编排管线。
- 默认/离线 SDK、CLI、HTTP 对带函数的 Query Access 仍 fail-closed / indeterminate，直到存在受支持的在线 session。
- No-leak：提交的 SQL 从不被执行、prepare 或 explain。公开结果与日志不暴露 SQL/字面量标记、连接数据、凭据、目录数据或原始驱动错误。

### MCP Agent 表面

- `audit_sql` 的 `content[0].text` 是精简发现摘要（结论、计数、`[level] rule_id: message`、suggestion），不是 `structuredContent` 的第二份 JSON。
- 离线 `audit_sql` 结构化 `context` 在未检查存在性时包含 `note` / `unproven`，精简文本带同一行限制说明。
- `list_rules` 返回精简目录行（`rule_id`、`level`、`dialect`、`kind`、`summary`）。完整正文请用 `describe_rule`。
- 已发布 MCP Registry 发现元数据（`server.json` / npm `mcpName`）。

### CLI 与 HTTP 审计契约

- 离线 CLI、HTTP、MCP 的 `context` 发出 `note: existence not checked (no database connection)` 和 `unproven: ["column_exists","table_exists"]`。元数据感知结果省略这两个字段。`pkg/deltascope.Result` 没有 `context`。
- 离线 `ALTER ... DROP COLUMN` notice 改为假设语气（`would drop column … if it exists`）。仅有 notice 的离线 ALTER 仍为 `pass`。
- 显式空 `--sql` 会拒绝，且不读取 stdin。
- 元数据连接失败映射为有界退出码 `3`；未知 flag 与 parser-error SQL 映射为退出码 `2`。
- `config init` 把空字符串参数编码为 YAML `""`。
- 已发布 CLI 二进制内嵌 DDL 覆盖目录。
- Markdown skip 原因聚合；不倾倒被跳过的规则 ID。

## 保持不变

- SQL 审核规则求值与已注册审核规则目录除离线 DROP COLUMN notice 文案外不变。`level` 仍是公开优先级字段；不引入 severity 字段。
- MCP 工具仍仅为 `audit_sql`、`describe_rule`、`list_rules`、`get_capabilities`。
- Query Access 仍不认证调用方、不评估授权、不执行 RLS、不脱敏、不改写 SQL、不自动授权，也不保证后续执行快照。
- 既有 MySQL/TiDB Query Access 接纳边界不变。
- 既有 release tag、GitHub Release、npm 包和 Homebrew cask 在本 tag 发布前保持不动。

## 非目标

- 不是 MCP Query Access 工具。
- 不改变 CREATE 策略包、默认规则级别，也不要求离线审计必须 `--host`。
- 不把 `context` 放进 `pkg/deltascope.Result`。
- 不把所有存在性相关 notice 都改成假设语气（DROP INDEX 仍写成删除已存在的索引）。
- 不是在已发布边界之外扩展通用 PostgreSQL Query Access。
- 不是授权、授权清单、角色、RLS、脱敏、改写、SQL 执行或返回数据的 API。
- 不是 severity 字段；不改变已注册审核规则目录计数。
- 不改变任何既有已发布产物或已有 tag。

## 规则目录事实

已注册审核规则目录相对 v0.480.0 不变。

| 指标 | 数量 |
|------|------:|
| 规则总数 | **371** |
| blocker | 72 |
| warning | 142 |
| notice | 157 |

| 方言范围 | 规则数 |
|----------|------:|
| common | 177 |
| postgresql | 191 |
| mysql | 1 |
| tidb | 2 |

| 语句种类 | 规则数 |
|----------|------:|
| ddl | 361 |
| dml | 10 |

## 未变指标

- 支持的 rule-and-dialect fixture coverage：**582/582**，**100.0%**，**247** 个 YAML fixture；这不是 SQL syntax 或 grammar coverage。
- PostgreSQL ALTER TABLE 配置项：**53**。
- DDL 覆盖目录：**400** 条（mysql 61，tidb 54，postgresql 285，parser_upgrade_candidate 18）。

## 决策记录

- `docs/decisions/2026-08-20-offline-existence-caveat-context.md`（本版本）
- `docs/decisions/2026-08-18-mcp-audit-sql-text.md`（本版本）
- `docs/decisions/2026-08-18-mcp-list-rules-compact-output.md`（本版本）
- `docs/decisions/2026-08-18-mcp-registry-discovery.md`（本版本）
- `docs/decisions/2026-08-18-cli-ddl-coverage-embedded-catalog.md`（本版本）
- `docs/decisions/2026-08-17-markdown-rule-summary-aggregation.md`（本版本）
- `docs/decisions/2026-08-17-cli-explicit-empty-sql-input-source.md`（本版本）
- `docs/decisions/2026-08-17-cli-metadata-connection-exit-mapping.md`（本版本）
- `docs/decisions/2026-08-17-cli-user-input-exit-mapping.md`（本版本）
- `docs/decisions/2026-08-17-generated-config-empty-string-encoding.md`（本版本）
- `docs/decisions/2026-08-16-query-access-proof-orchestration.md`（本版本）
- `docs/decisions/2026-08-16-query-access-remove-db-backed-resolvers.md`（本版本）
- `docs/decisions/2026-08-14-query-access-dialect-session-api-deprecation.md`（本版本）
- `docs/decisions/2026-08-12-query-access-online-analysis-entry.md`（本版本）
- `docs/decisions/2026-08-11-query-access-postgresql-resolver-core.md`（本版本）
- `docs/decisions/2026-08-03-query-access-pg17-count-online-surface-contract.md`（v0.480.0）
