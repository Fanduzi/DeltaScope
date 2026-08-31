# DeltaScope v0.510.3 发行说明

发布日期：2026-08-31

## 概要

v0.510.3 修正了增量 MCP 能力发现输出：[`get_capabilities.connection_inputs`](../../internal/interfaces/mcp/rule_tools.go) 现在会声明既有的公共 `audit_sql.connection` 输入 `connection.connect_timeout`。[Issue #64](https://github.com/Fanduzi/DeltaScope/issues/64) 由真实内存内 [`tools/list` 与 `get_capabilities` tool-call parity 测试](../../internal/interfaces/mcp/server_test.go) 防止再次漂移。

这仅是增量发现输出修正；不改变 timeout 解析、默认值、连接行为、错误、凭据、工具或输入 schema。

DeltaScope 仍是静态分析：不执行提交的 SQL、不返回查询结果，也不做授权判定。MCP 没有 Query Access 工具。已注册规则目录仍为 373 条。支持的 rule-and-dialect fixture coverage 仍为 586/586（100.0%），共 286 个 YAML fixture；这不是 SQL syntax 或 grammar coverage。

## 修复

- [#64](https://github.com/Fanduzi/DeltaScope/issues/64)：`get_capabilities.connection_inputs` 在 password 输入之后包含 `connection.connect_timeout`，与公共连接 schema 顺序一致。
- [`TestGetCapabilitiesConnectionInputsMatchAuditSQLSchema`](../../internal/interfaces/mcp/server_test.go) 比较真实内存内 `tools/list` connection properties 与真实 `get_capabilities` tool call；[`TestGetCapabilitiesToolReturnsKnownSummary`](../../internal/interfaces/mcp/rule_tools_test.go) 保留显式排序契约。

## 非目标

- 不改变 timeout 解析、校验、默认值、开连行为、错误处理或凭据处理。
- 不引入新的 MCP 工具、transport、capability version 或输入 schema 变更。
- 不执行 SQL，不做授权，不改变已注册规则目录，也不是 SQL syntax 或 grammar coverage 声明。

## 规则目录事实

| 指标 | 数量 |
|------|------:|
| 规则总数 | **373** |
| blocker | 73 |
| warning | 142 |
| notice | 158 |

## 语料与目录事实

- 支持的 rule-and-dialect fixture coverage：**586/586**、**100.0%**、**286** 个 YAML fixture；这不是 SQL syntax 或 grammar coverage。
- PostgreSQL ALTER TABLE 配置项：**53**。
- DDL 覆盖目录：**407** 条（mysql 62，tidb 55，postgresql 290，parser_upgrade_candidate 18）。

## 决策记录

- [2026-08-31 MCP connect timeout capability](../decisions/2026-08-31-mcp-connect-timeout-capability.md)（[#64](https://github.com/Fanduzi/DeltaScope/issues/64)）是既有的 Accepted 边界记录；本次发布准备不需要新增决策记录。
