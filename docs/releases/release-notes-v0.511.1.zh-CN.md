# DeltaScope v0.511.1 发行说明

发布日期：2026-09-16

## 概要

v0.511.1 把 CLI 和 HTTP 各自写过两遍的三处 helper 收成一份：catalog YAML 标量、`--sql`/`--file`/stdin 加载，以及 online Query Access session 附着。公开 Audit、Query Access、MCP 契约不变。

DeltaScope 仍是静态分析：不执行提交的 SQL、不返回查询结果，也不做授权判定。MCP 仍然没有 Query Access 工具，也没有 TLS 字段。Rule Catalog 为 **376** 条，其中三条默认关闭的 `dml.impact.*` opt-in 规则。Default Policy 不启用这三条。支持的 rule-and-dialect fixture coverage 仍为 586/586（100.0%），共 286 个 YAML fixture；这不是 SQL syntax 或 grammar coverage。

## 变更

- `catalog.FormatYAMLScalar` 是唯一的 YAML 标量渲染。CLI `rules explain` 用它，不再留本地副本。
- CLI `audit` 和 `query-access analyze` 共用 `resolveCLISQL`。空 SQL 文案仍按命令区分（`audit:` 与 `query-access:`）。读文件失败的句子保持原样。
- CLI 和 HTTP 通过 `metadata.AttachOnlineQueryAccessSession` 附着已打开的 online session。仍复用 Observed Server Identity；transport 仍注入各自的 from-conn 构造器。

## 非目标

- 不在 `pkg/deltascope` 增加公开 attach helper。
- 不缓存 Default Policy，也不缓存每次 audit 的 rule registry。
- 不统一 CLI、HTTP、MCP 面向用户的失败句子。
- 不增加 MCP Query Access 工具。
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

没有新的决策记录。这次补丁不改公开契约。
