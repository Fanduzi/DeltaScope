# DeltaScope HTTP API 服务设计

## 目标

将现有的库优先审计引擎公开为长期运行的 HTTP 服务，而不扰乱核心解析器中立审计流程。

## 当前状态

`DeltaScope` 已有：

- 通过 `pkg/deltascope` 的稳定库入口
- 稳定的策略加载和配置初始化
- Markdown 和 JSON 输出渲染器
- CLI 优先工作流

下一个服务层应适配那些能力，而不是重新实现它们。

## 推荐方向

构建一个薄的 HTTP 适配器，依赖于 CLI 使用的相同应用/库核心。

### 必需的服务职责

- 通过 HTTP 接受 SQL 和审计选项
- 通过现有 Viper 支持的路径加载和热重载配置
- 返回结构化 JSON 结果
- 公开健康和版本端点

### 保持范围外

- 认证和多租户
- 实时数据库元数据检查
- 分布式策略管理
- MCP 协议处理

## 提议的形状

- `internal/interfaces/http`
  - 请求绑定
  - 响应渲染
  - 错误映射
- `internal/application`
  - 复用 CLI 已调用的相同服务/用例层
- `cmd/deltascope-server`
  - 薄进程入口

## 预期结果

此里程碑后，`DeltaScope` 应作为一个小服务运行，通过 JSON API 公开相同的离线审计引擎。这应使 MCP 服务器里程碑成为一个适配器问题，而不是核心审计重写。
