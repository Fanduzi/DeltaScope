# DeltaScope v0.511.0 发行说明

发布日期：2026-09-07

## 概要

v0.511.0 把 metadata-aware Audit 和 Online Query Access 的连库工作收到打开前的同一条 path，再给 Catalog / Loaded / Suppression、Mutation Target、不完整审计的 Markdown completeness，以及一次 Observed Server Identity 探测起名。CLI、HTTP、MCP 仍用各自的失败文案。MCP 仍然没有 Query Access 工具，也没有 TLS 字段。

DeltaScope 仍是静态分析：不执行提交的 SQL、不返回查询结果，也不做授权判定。Rule Catalog 为 **376** 条，其中三条默认关闭的 `dml.impact.*` opt-in 规则。Default Policy 不启用这三条。支持的 rule-and-dialect fixture coverage 仍为 586/586（100.0%），共 286 个 YAML fixture；这不是 SQL syntax 或 grammar coverage。

## 新功能

- 一条共用的连库 path 负责密码、TLS 形状、timeout、PostgreSQL 省略端口默认值、MySQL/TiDB catalog 别名，以及 Connection Failure Class。CLI、HTTP、MCP 都走它。path 在打开前结束。metadata-aware Audit 仍开连接池；Online Query Access 仍钉住一条 session。
- `rulepresence.Of` 通过实际的 `ddl.Register` / `dml.Register` 和 `rule.Registry.Contains` 回答 Catalog、Default Policy、Loaded、Suppression。config status 展示这四个事实，不把它们并成一个。
- `markdown.Render` 拥有 Unsupported 和 Diagnostics。CLI Markdown 只前置 Audit Context。CI adapter 比较 `spec.DiagnosticParserError`。
- `spec.DML.MutationTargets` 给 DML 写入表起名。metadata、auditmeta 和 `dml.table.exists.require` 读 `MutationTargetTables()`。
- 已经 identify 过钉住连接的 transport 调用 `deltascope.NewOnlineQueryAccessSessionFromIdentifiedConn`。这个构造函数不再 ping，也不再查 `VERSION`。只有 `*sql.Conn` 的调用方仍用 `NewOnlineQueryAccessSessionFromConn`。
- 没有 PostgreSQL adapter 调用者的 identity-resolver helper 取消导出。adapter 真正调用的函数仍导出。不加 proof-engine seam。

## 非目标

- 不给 MCP 公开连接合同加 TLS 字段。
- 不统一 CLI、HTTP、MCP 面向用户的失败句子。
- 不把 Audit 连接池 opener 和 Query Access 钉住 opener 合成一个。
- 不增加 MCP Query Access 工具。
- 不把 `Parse`/`Extract` 藏起来，也不删那个没人用的 JSON output adapter。
- 不把 `online.ErrPostgreSQLQueryAccessVersionUnsupported` 和 `deltascope.ErrOnlineQueryAccessPostgreSQLVersionUnsupported` 合成一个。
- 不执行 SQL，不做授权，也不是 SQL syntax 或 grammar coverage 声明。

## 规则目录事实

| 指标 | 数量 |
|------|------:|
| 规则总数 | **376** |
| blocker | 73 |
| warning | 144 |
| notice | 159 |

总数包含三条默认关闭的 `dml.impact.*` Catalog 行。它不是 Default Policy 计数，也不是 Loaded。

## 语料与目录事实

- 支持的 rule-and-dialect fixture coverage：**586/586**、**100.0%**、**286** 个 YAML fixture；这不是 SQL syntax 或 grammar coverage。
- PostgreSQL ALTER TABLE 配置项：**53**。
- DDL 覆盖目录：**407** 条（mysql 62，tidb 55，postgresql 290，parser_upgrade_candidate 18）。

## 决策记录

- [2026-09-06 transport connection resolution](../decisions/2026-09-06-transport-connection-resolution.md) 是打开前共用 path 的 Accepted 边界。
- [2026-09-07 architecture-review follow-through](../decisions/2026-09-07-architecture-review-follow-through.md) 是其余五个具名事实的 Accepted 边界。
