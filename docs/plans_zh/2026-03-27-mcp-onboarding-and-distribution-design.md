# MCP Onboarding 与 Distribution 设计

## 目标

定义 DeltaScope 下一阶段的 MCP 里程碑，让 `deltascope-mcp` 不仅“能跑”，而且成为一个用户可以低摩擦发现、安装、配置并接入主流 MCP 客户端的产品面。

## 背景

DeltaScope `v0.7.0` 已经具备：

- 官方 `deltascope-mcp` stdio server
- 稳定的 MCP 工具：`audit_sql`、`describe_rule`、`list_rules`、`get_capabilities`
- direct connection 与 `connection_ref` 的 metadata-aware 支持
- GoReleaser 产物与 installer 支持
- 基于真实 stdio、MySQL、TiDB 的端到端验证

当前短板不在 server 正确性，而在 onboarding 与 distribution 体验。

今天的新用户仍然要额外理解很多事情：

- 怎么安装 `deltascope-mcp`
- 怎么接到 Claude Code、Codex 或其它 MCP 客户端
- direct connection 和 `connection_ref` 什么时候该用哪个
- `connections.yaml` 到底该长什么样
- 怎么覆盖默认配置路径

这和当前 MCP 市场里常见的“一条 `npx -y ...` 就能接上”的体验相比，差距仍然明显。

## 问题定义

DeltaScope 已经有官方 MCP server，但还没有形成主流 MCP 产品应有的接入体验。项目需要一个统一的 onboarding 与 distribution 模型，让以下三条路径都足够清晰：

1. 面向主流 MCP 客户端的零摩擦推荐路径
2. 面向不带 helper 命令客户端的手动 stdio 配置路径
3. 面向 CI、本地自动化与运维用户的原生二进制路径

## 非目标

本里程碑不做：

- 重设计 `deltascope-mcp` 的 tool contract
- 新增审计规则或新的 metadata provider
- 替换 GoReleaser 的二进制发布体系
- 增加 hosted transport、remote auth 或 SaaS 能力
- 将 MCP server 重写成 Node.js
- 一次性为所有 MCP 客户端写专门文档

## 候选方案

### 方案 A：只补文档

保持现有二进制分发不变，只加强 README 和 recipe 文档。

优点：

- 最快
- 不增加新的发布面

缺点：

- 仍明显落后于 `npx -y ...` 这种主流 onboarding 体验
- 用户依旧要先安装、配 PATH
- 客户端示例仍不够“复制即用”

### 方案 B：坚持原生二进制唯一入口

把 `deltascope-mcp` 当作纯 binary-first MCP server，只优化本地安装路径。

优点：

- 运维上最简单
- 只有一套产物模型

缺点：

- 与当前 MCP 客户端的主流接入习惯不匹配
- 不利于 Claude Code / Codex 这种一条命令接入
- 可发现性和复制体验仍偏弱

### 方案 C：混合分发模型

保留 `deltascope-mcp` 作为规范 Go stdio server，继续发布 release binaries，同时增加 npm launcher 包，在用户本地下载、缓存并执行正确平台的 DeltaScope 二进制。

优点：

- 对齐当前主流 MCP onboarding 习惯
- 保留现有 Go 实现与 release assets
- 继续兼容 CI 和运维用户的 binary-first 工作流
- 允许 README 示例达到接近 `npx -y @upstash/context7-mcp` 的复制体验

缺点：

- 增加第二个 distribution surface
- 需要设计版本选择与缓存策略
- Node 与 Go 两套发布面都要补验证和文档

## 推荐方案

选择方案 C。

DeltaScope 需要交付的是“一个 MCP 产品”，而不是“一个 server 二进制加一堆零散说明”。正确模型应是：

- 规范 runtime：`deltascope-mcp`
- 推荐用户入口：`npx -y @fanduzi/deltascope-mcp`
- 手动/兜底入口：原生 `deltascope-mcp`

这能对齐当前 MCP 用户的预期，同时不需要把 server 本身改写掉。

## 设计

### 1. 产品定义

MCP 产品面现在应同时提供两种入口方式：

- **推荐入口**：npm launcher 包 `@fanduzi/deltascope-mcp`
- **规范 runtime**：原生 `deltascope-mcp` 二进制

launcher 不是第二套 MCP 实现，而是启动真正 DeltaScope 二进制的 bootstrap 层。

### 2. 分发模型

#### 规范 Runtime

真正的 server 仍是 Go 二进制：

- `deltascope-mcp`

它继续作为以下能力的唯一事实来源：

- tool schema
- connection handling
- metadata-aware 行为
- release 验证

#### Launcher 包

npm launcher 应负责：

- 检测当前 OS 与架构
- 解析 DeltaScope 版本
- 从 GitHub Releases 下载匹配 release asset
- 缓存解压后的二进制
- 启动 `deltascope-mcp`，并保持 `stdout` 专用于 MCP 协议，同时允许 bootstrap 诊断日志写到 `stderr`

它不应该：

- 重写 MCP tool
- 自己解析 SQL
- 添加和原生二进制不同的产品语义

#### 版本模型

推荐行为：

- 默认：launcher 使用与 npm package 版本一致的 DeltaScope release
- 覆盖：允许通过环境变量或 launcher flag 显式指定版本
- cache key：release version + platform + architecture

这样既能保持 onboarding 的确定性，也能满足高级用户的可复现要求。

### 3. Onboarding 路径

项目应明确支持四条用户入口路径。

#### 路径 A：Claude Code

主示例：

```bash
claude mcp add --scope user deltascope -- npx -y @fanduzi/deltascope-mcp
```

#### 路径 B：Codex

主示例：

```bash
codex mcp add deltascope -- npx -y @fanduzi/deltascope-mcp
```

#### 路径 C：其它主流 MCP 客户端

提供一个可迁移的通用 stdio 配置示例，用于 TOML/JSON 类配置文件客户端。

示例：

```toml
[mcp_servers.deltascope]
command = "npx"
args = ["-y", "@fanduzi/deltascope-mcp"]
startup_timeout_sec = 20
```

#### 路径 D：手动原生二进制

面向以下场景保留 direct binary 路径：

- CI
- air-gapped 或受控环境
- 不想依赖 Node.js 的用户

示例：

```toml
[mcp_servers.deltascope]
command = "deltascope-mcp"
args = []
startup_timeout_sec = 20
```

### 4. 配置模型

文档必须把配置讲成两种等价支持模式，而不是一种“推荐模式”加一种“隐藏 fallback”。

#### Direct Connection

适合：

- 快速试用
- 临时会话
- 一次性的 metadata-aware 调用

用户在 MCP tool input 中直接传 `connection`。

#### `connection_ref`

适合：

- 高频使用
- 更安全的 secret 管理
- 具名环境配置

默认配置文件仍是：

- `~/.config/deltascope/connections.yaml`

文档必须给出最小可复制示例：

```yaml
connections:
  prod_readonly:
    host: 127.0.0.1
    port: 3306
    user: app_readonly
    password_env: MYSQL_PASSWORD
    schema: app
```

也必须说明如何覆盖配置路径：

```bash
deltascope-mcp -connections-path /path/to/connections.yaml
```

以及如何放进 MCP client config：

```toml
[mcp_servers.deltascope]
command = "deltascope-mcp"
args = ["-connections-path", "/path/to/connections.yaml"]
startup_timeout_sec = 20
```

### 5. 文档架构

文档应按用户意图拆分。

#### Root README

只保留短小的 MCP quick start：

- DeltaScope MCP 是什么
- 一条 Claude Code 示例
- 一条 Codex 示例
- 指向完整 MCP 使用文档的链接

#### 专门的 MCP Guide

新增真正面向 onboarding 的独立文档，而不是继续堆在 agent recipe 里。

建议文件名：

- `docs/recipe/use-deltascope-mcp.md`
- `docs/recipe/use-deltascope-mcp.zh-CN.md`

内容应覆盖：

- 安装与前置要求
- Claude Code
- Codex
- 通用 TOML/JSON stdio 配置
- 原生 binary 路径
- direct connection
- `connection_ref`
- `connections.yaml`
- 常见失败与排查

#### 现有 AI-Agent Recipe

`docs/recipe/use-with-ai-agents.md` 应继续聚焦 agent workflow 和 DeltaScope 语义，不再承担全部 onboarding 内容，而是链接到专门的 MCP guide。

### 6. 成功标准

本里程碑成功的标志是：

- 新用户能复制一条 Claude Code 命令接入 DeltaScope MCP
- 新用户能复制一条 Codex 命令接入 DeltaScope MCP
- 没有 helper CLI 的用户也能通过通用 stdio 配置示例接入
- 用户不需要猜就能在 direct connection 和 `connection_ref` 之间做选择
- 文档里有最小可用 `connections.yaml`
- npm launcher 与原生二进制被讲成一个连贯的产品故事

## 风险

### Distribution Drift

如果版本解析或 asset naming 不够明确，launcher 可能和原生 release contract 漂移。

缓解：

- 使用明确的资产命名合同
- 对 launcher 做真实 release 验证

### Documentation Fragmentation

如果 README、MCP guide 和 agent guide 都试图“讲全”，它们会继续漂移。

缓解：

- 每份文档只承担一种职责
- README 保持短
- 专门的 MCP guide 作为主 onboarding 文档

### 过度围绕单一客户端

如果文档只服务 Claude Code 或只服务 Codex，产品面会变窄。

缓解：

- 始终提供通用 stdio 配置
- 把 Claude 和 Codex 当作优先示例，而不是全部用户

## 结论

下一阶段的 MCP 里程碑应聚焦 onboarding 与 distribution UX，而不是继续扩展 server 内核。DeltaScope 应采用混合模型：Go 二进制保持规范 runtime，npm launcher 成为推荐入口；文档则围绕主流 MCP 客户端的复制式接入，以及清晰的手动配置/连接管理路径重组。
