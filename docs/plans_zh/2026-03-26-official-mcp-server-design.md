# 官方 MCP Server 设计

## 目标

交付官方 `deltascope-mcp`，使 AI agent 能通过稳定的 MCP 工具界面调用 DeltaScope，并同时支持离线与 metadata-aware 的 SQL 审计流程。

## 背景

DeltaScope `v0.6.2` 已经具备构建 MCP 界面所需的核心能力：

- CLI、HTTP 与 `pkg/deltascope` 共享的稳定审计引擎
- 带有 `summary`、`explanation`、`why`、`risk`、`suggestion` 的可解释审计结果
- 通过公共 `MetadataProvider` 提供的可选 metadata-aware 审计能力
- 通过已发布规则目录提供的稳定规则发现基础

当前缺口不在审计正确性，而在官方 agent 集成。

今天，agent 用户可以手工包装 CLI，但这仍留下几个产品问题：

- 没有官方 MCP 工具契约
- 没有面向 agent 调用方的一方错误分类
- 没有官方的 metadata-aware 连接选择方式
- 没有统一的连接安全、密码处理与结果上下文方案

## 非目标

本里程碑不包括：

- hosted service、auth 或 multi-tenancy
- 新的 DDL 或 DML 规则
- 替代现有 CLI 或 HTTP 产品
- MCP 之外的大范围 agent framework 适配
- 首个版本中加入 streamable HTTP/SSE 传输
- 重新设计 DeltaScope 的核心审计结果模型

## 考虑过的方案

### 方案 A：基于现有 CLI 的 MCP 包装层

由 MCP server 启动 `deltascope audit` 与 `deltascope rules ...` 子进程。

优点：

- 首个原型最快
- 新业务逻辑最少

缺点：

- MCP 行为被 CLI 进程编排与 exit code 处理绑住
- 类型化错误映射较弱
- metadata-aware 连接编排更别扭
- 作为官方产品面，长期维护摩擦更大

### 方案 B：直接构建在共享 Go Library 之上的 MCP Server

将 MCP server 作为 `pkg/deltascope`、规则目录与共享应用辅助逻辑之上的薄适配层。

优点：

- 传输边界更清晰
- 直接复用稳定的共享审计主路径
- 错误控制与测试性更好
- 最适合作为官方产品面长期演进

缺点：

- 相比 CLI 包装层，开发量略大
- 需要补少量 MCP 专属 wiring

### 方案 C：扩展现有 HTTP 服务以同时支持 MCP

让 `deltascope-server` 在同一进程中同时提供 HTTP API 与 MCP。

优点：

- 二进制更少
- 只有一个偏网络服务的入口

缺点：

- 过早混淆两个独立产品面
- 把传输问题推进到不合适的里程碑
- 范围增加但首版收益有限

## 推荐

选择方案 B。

DeltaScope 已具备清晰的 library-first 架构。官方 MCP server 应成为另一个薄接口层，而不是子进程壳，也不是负担过重的 HTTP 服务。这能让里程碑聚焦在稳定传输适配，并让共享审计契约继续作为唯一事实来源。

## 设计

### 1. 产品定义

首个官方 MCP 版本应以本地 stdio server 交付：

- 二进制：`deltascope-mcp`
- 范围：本地 agent 集成
- 运行模型：单个长生命周期进程中的无状态请求处理

MCP server 成为第四个正式产品面，与以下界面并列：

- `deltascope` CLI
- `deltascope-server`
- `pkg/deltascope`
- `deltascope-mcp`

### 2. 工具面

首版应交付三个必需工具与一个可选辅助工具。

必需：

- `audit_sql`
- `describe_rule`
- `list_rules`

可选但推荐：

- `get_capabilities`

#### `audit_sql`

供 agent 使用的主审计工具。

输入：

- `sql`（必填）
- `dialect`（可选）
- `config_path`（可选）
- `connection_ref`（可选）
- `connection`（可选）

规则：

- `connection_ref` 与 `connection` 互斥
- 当两者都未提供时，走离线审计
- 当提供任一者时，走 metadata-aware 审计

#### `describe_rule`

按单个 `rule_id` 返回已发布规则元数据，方便 agent 做后续解释与修复推理。

#### `list_rules`

返回已发布规则，并支持按 statement kind、level、metadata awareness 或关键词进行可选过滤。

#### `get_capabilities`

向需要预先推理能力范围的 agent 返回精简能力摘要，包括支持的方言、模式与产品面。

### 3. 结果结构

MCP server 应保留现有 `v0.6.2` 审计结果主体，仅新增一个顶层增量字段：`context`。

推荐的 `audit_sql` 成功返回形状：

- 现有顶层字段保持不变：
  - `verdict`
  - `summary`
  - `statements`
  - `global_findings`
  - `explanation`
- 新增：
  - `context`

`context` 只负责描述结果是如何产生的，而不是重复审计事实：

- `mode`：`offline` 或 `metadata-aware`
- `dialect`：最终生效方言
- `dialect_source`：`default`、`request`、`detected` 或 `connection`
- `schema`：最终生效 schema（如果有）
- `schema_source`：`none`、`request`、`connection` 或 `inferred`
- `metadata_source`：`none`、`connection_ref` 或 `direct`

重要约束：

- 保留 `summary`、statement 级 `explanation` 与 finding 级 `explanation`
- 保留 `why`、`risk`、`suggestion`
- 不创建第二套 MCP 专属 explanation 模型
- 不在 `context` 中泄露连接参数

### 4. 错误模型

MCP 失败时应返回结构化错误体，而不是部分成功的审计结果。

推荐稳定错误码：

- `bad_request`
- `connection_invalid`
- `connection_failed`
- `config_invalid`
- `internal_error`

规则：

- 调用方可以依赖 `code`
- 调用方不能解析 `message`
- metadata-aware 请求在连接失败时不得静默回退到 offline 模式

### 5. Metadata-Aware 连接设计

MCP server 应支持两种连接输入方式。

#### 推荐方式：`connection_ref`

用户提供一个命名连接引用，server 从本地配置文件解析真实连接信息。

推荐本地文件：

- `~/.config/deltascope/connections.yaml`

结构：

- `connections.<name>` 映射到一个连接定义

这样用户就可以说“用 `prod_readonly`”，而不需要在 prompt 中暴露密钥。

#### 直接方式：`connection`

用户以内联方式提供连接参数，适用于本地或临时场景。

推荐字段：

- `host`
- `port`
- `socket`（可选）
- `user`
- `schema`（可选）
- `dialect`（可选）
- 且密码来源三选一：
  - `password`
  - `password_env`
  - `password_file`

### 6. Secret Handling

官方 MCP server 必须允许直接传 `password` 以保证易用性，但不能把内联密码视为默认推荐路径。

推荐引导顺序：

1. `connection_ref`
2. `password_env`
3. `password_file`
4. `password`

必须具备的保护措施：

- 日志、错误与 tool 返回中绝不回显密码或完整 DSN
- 校验只能存在一个密码来源
- 配置文件中的明文密码只作为便捷路径支持，不作为推荐默认值

### 7. 共享代码复用

本里程碑应尽量复用现有审计与 metadata 逻辑。

预期新增结构：

- `cmd/deltascope-mcp`
  - 进程入口
- `internal/interfaces/mcp`
  - MCP server wiring
  - 工具 schema
  - 请求校验
  - 结果与错误整形
- 如有必要，从 CLI metadata-preparation 代码中抽取共享 helper

MCP 层不应复制：

- 规则求值
- 结果聚合
- 公共 explanation 整形
- metadata provider 行为

### 8. 文档策略

本里程碑应说明：

- 如何将 `deltascope-mcp` 配到 MCP 客户端
- `audit_sql` 如何选择 offline 与 metadata-aware 模式
- `connection_ref` 与直接 `connection` 的用法
- 结果中的 `context` 字段
- prompt 与本地配置文件中的 secret-handling 建议

## 验收标准

满足以下条件时，本里程碑完成：

1. `deltascope-mcp` 能作为 MCP stdio server 运行。
2. `audit_sql`、`describe_rule`、`list_rules` 可通过 MCP 使用。
3. `audit_sql` 支持 offline、`connection_ref` 与直接 `connection` 三种模式。
4. metadata-aware 运行复用共享 DeltaScope 审计主路径，而不是走 CLI 子进程路径。
5. MCP 成功响应保留 `v0.6.2` 审计结果结构，并且只新增顶层 `context`。
6. 密码与 DSN 不会出现在日志、返回 payload 或错误信息中。
7. metadata-aware 连接失败时返回结构化错误，且不会静默降级为 offline。
8. 中英文文档都解释了安装、工具使用、连接处理与结果解读方式。
