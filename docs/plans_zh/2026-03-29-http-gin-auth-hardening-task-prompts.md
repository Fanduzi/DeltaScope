# HTTP Gin + Auth Hardening 任务提示词

> 用于 `HTTP Gin + Auth Hardening` 里程碑的任务式实现与评审。
> 默认工作目录：`/Users/fan/GolangProjects/DeltaScope`。

## 全局规则

- 除非显式版本化，不改变 `/v1/audit` 对外 JSON 契约。
- 业务逻辑不得进入 Gin handler，只做传输适配。
- 本里程碑认证头固定为 `X-API-Key`。
- 缺失 key 返回 `401`，无效 key 返回 `403`。
- `/healthz` 与 `/version` 的放行行为必须可配置且文档明确。
- 中间件顺序必须固定并由测试覆盖。
- 绝不记录原始 API key 到日志。
- 在认证关闭时保持对既有调用方兼容。
- 优先写 HTTP 黑盒测试，避免依赖框架内部细节。
- 每个任务都要回传：变更文件、测试命令、结果、commit hash。

## 里程碑焦点

- Gin 版 HTTP 适配层迁移
- API Key 认证与配置面
- request-id/recover/timeout/logging 基线中间件
- 路由与错误契约稳定性
- 用户接入文档与 key 轮换操作说明

## 任务意图

### 任务 1：规划文档

- 固化中英文 design/implementation/task-prompts。
- 明确本里程碑选择 Gin，不选择 Fiber。

### 任务 2：Gin 迁移

- 将 mux 路由改为 Gin 路由装配。
- 不改变对外端点契约。

### 任务 3：API Key 认证

- 增加基于配置的 key 校验。
- 明确 401/403 和 allow-path 语义。

### 任务 4：基础中间件链

- 增加 request-id、recovery、timeout、logging。
- timeout/panic 必须返回稳定 JSON 错误 envelope。

### 任务 5：配置面

- 引入并校验认证相关配置项。
- 保持老配置兼容。

### 任务 6：契约测试

- 用黑盒测试锁定路由与错误行为。
- 防止后续重构造成行为漂移。

### 任务 7：文档

- 增加 `X-API-Key` 调用示例。
- 解释 401/403 与 key 轮换流程。

### 任务 8：发布交接

- 准备迁移说明与 release 要点。
- 确认里程碑达到可发布状态。
