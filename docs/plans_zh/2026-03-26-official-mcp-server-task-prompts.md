# 官方 MCP Server 任务提示词

> 用于 `Official MCP Server` 里程碑的逐任务实现与评审。
> 默认工作目录为 `/Users/fan/GolangProjects/DeltaScope`。

## Global Rules

- 共享审计引擎必须始终作为唯一事实来源。
- 除非任务明确重新评估该决策，否则 MCP 审计执行不得 shell 到 `deltascope` CLI。
- MCP `audit_sql` 成功返回必须保留 `v0.6.2` 审计结果主体，并且只新增顶层 `context` 字段。
- 保留 `summary`、`explanation`、`why`、`risk`、`suggestion`。
- metadata-aware 请求代表显式意图；连接失败时不得静默降级到 offline。
- 允许直接 `password` 输入，但日志、错误、示例文档与 tool 返回中绝不能暴露密码或完整 DSN。
- 所有非平凡的 MCP 契约、连接与传输改动都使用 TDD。
- 将 `three-level-doc` 作为硬门槛。
- 每个任务都返回修改文件、运行测试、状态与 commit hash。

## Milestone Focus

- 官方 `deltascope-mcp` stdio server
- 稳定的 MCP tool schema
- offline 与 metadata-aware `audit_sql`
- `connection_ref` 与直接 `connection`
- 对 secret 安全的连接处理
- 通过 `describe_rule` 与 `list_rules` 暴露规则发现能力
- 教用户如何配置与使用 MCP server 的文档

## Task Intent

### Task 1: 规划文档

- 以中英双语保存已确认的设计、实施计划与任务提示词。
- 让命名与范围与 `Official MCP Server` 保持一致。

### Task 2: MCP Runtime Bootstrap

- 接入本地 stdio server 所需的最小 MCP runtime。
- 保持传输层足够薄且便于测试。

### Task 3: MCP 契约

- 先定义工具请求、成功返回与错误结构，再填业务逻辑。
- 成功结果必须保留 DeltaScope 现有审计语义。

### Task 4: 共享 Metadata Helpers

- 抽取可复用的 metadata 准备逻辑，而不是复制 CLI 代码。
- 抽取后保持现有 CLI 路径行为稳定。

### Task 5: 连接解析

- 同时支持 `connection_ref` 与直接 `connection`。
- 清晰执行互斥校验与密码来源校验。

### Task 6: `audit_sql`

- 直接复用共享 DeltaScope 审计主路径。
- 对 metadata-aware 上下文保持诚实，绝不静默降级到 offline。

### Task 7: 规则工具

- 让 `describe_rule` 与 `list_rules` 足够支持 agent 的后续推理。
- 保持过滤集合小而稳定，并写清文档。

### Task 8: Secret Safety

- 用测试证明密码与 DSN 不会泄露。
- 保持错误码稳定，并对敏感信息做强脱敏。

### Task 9: Docs

- 教用户如何接入 MCP server，以及如何在 offline、`connection_ref` 与直接连接之间做选择。
- 保持中英文文档对齐。

### Task 10: Closure

- 重新运行聚焦验证与整体验证。
- 将里程碑结果记录到 handoff、progress 与 decisions。
- 让 MCP 里程碑保持可交付状态。
