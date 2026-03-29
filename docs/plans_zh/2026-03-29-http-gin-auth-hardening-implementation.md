# HTTP Gin + Auth Hardening 实施计划

> **给 agent 执行者：** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans。所有步骤使用 `- [ ]` 勾选形式。

**目标：** 将 DeltaScope HTTP 适配层迁移到 Gin，补齐 API Key 认证和基础生产中间件，同时保持当前 API 契约稳定。

**架构原则：** 框架相关逻辑仅放在 `internal/interfaces/http`，`application/domain` 保持框架无关。

**技术栈：** Go、Gin、现有 DeltaScope 审核应用服务、HTTP 黑盒测试、Markdown 文档

---

### 任务 1：落地计划文档

**文件：**
- Create: `docs/plans/2026-03-29-http-gin-auth-hardening-design.md`
- Create: `docs/plans/2026-03-29-http-gin-auth-hardening-implementation.md`
- Create: `docs/plans/2026-03-29-http-gin-auth-hardening-task-prompts.md`
- Create: `docs/plans_zh/2026-03-29-http-gin-auth-hardening-design.md`
- Create: `docs/plans_zh/2026-03-29-http-gin-auth-hardening-implementation.md`
- Create: `docs/plans_zh/2026-03-29-http-gin-auth-hardening-task-prompts.md`

- [ ] Step 1: 写入中英文六份规划文档并保持范围一致
- [ ] Step 2: 检查中英文是否出现范围漂移
- [ ] Step 3: 仅提交规划文档

### 任务 2：引入 Gin 服务骨架

**文件：**
- Modify: `internal/interfaces/http/server.go`
- Modify: `internal/interfaces/http/handler.go`
- Modify: `cmd/deltascope-server/main.go`（仅在启动装配需要时）
- Test: HTTP 路由契约测试

- [ ] Step 1: 先写失败测试，锁定迁移后路由行为
- [ ] Step 2: 将 mux 路由迁移到 Gin，路径保持不变
- [ ] Step 3: 保持当前响应结构与状态码语义
- [ ] Step 4: 运行聚焦测试
- [ ] Step 5: 提交 Gin 迁移改动

### 任务 3：实现 API Key 认证中间件

**文件：**
- Create/Modify: `internal/interfaces/http/middleware/auth*.go`
- Modify: HTTP 配置加载路径（如需要）
- Test: 认证中间件测试

- [ ] Step 1: 先写失败测试（缺失 401、错误 403、正确 200）
- [ ] Step 2: 实现 `X-API-Key` 校验，key 集合来自配置
- [ ] Step 3: 支持 allow-path 放行（默认健康检查/版本）
- [ ] Step 4: 确保日志不泄露原始 key
- [ ] Step 5: 运行测试并提交

### 任务 4：补齐基础中间件链

**文件：**
- Create/Modify: `internal/interfaces/http/middleware/requestid*.go`
- Create/Modify: `internal/interfaces/http/middleware/recovery*.go`
- Create/Modify: `internal/interfaces/http/middleware/timeout*.go`
- Create/Modify: `internal/interfaces/http/middleware/logging*.go`
- Test: 中间件行为测试

- [ ] Step 1: 先写失败测试（timeout envelope、panic recovery）
- [ ] Step 2: 实现 request-id/recovery/timeout/logging
- [ ] Step 3: 固化中间件顺序
- [ ] Step 4: 运行聚焦测试
- [ ] Step 5: 提交中间件改动

### 任务 5：新增认证配置面

**文件：**
- Modify: 配置结构和解析路径
- Modify: `configs/deltascope.example.yaml`（如适用）
- Test: 配置解析测试

- [ ] Step 1: 定义 `http.auth.enabled`、`http.auth.keys`、`http.auth.allow_paths`
- [ ] Step 2: 在启用认证时增加无效配置校验
- [ ] Step 3: 保持老配置兼容
- [ ] Step 4: 运行配置和 HTTP 测试
- [ ] Step 5: 提交配置改动

### 任务 6：用黑盒测试锁定契约

**文件：**
- Create/Modify: HTTP 集成测试

- [ ] Step 1: 校验 `/v1/audit` 返回结构保持稳定
- [ ] Step 2: 校验非法 JSON 与输入错误的 envelope 稳定
- [ ] Step 3: 校验 auth + timeout 不影响正常路径
- [ ] Step 4: 运行 `go test ./...` 并提交

### 任务 7：更新文档与接入说明

**文件：**
- Modify: `README.md`
- Modify: `README_ZH.md`
- Modify: `docs/reference/http-api.md`
- Modify: `docs/reference/http-api.zh-CN.md`
- Modify: `cmd/deltascope-server/README.md`

- [ ] Step 1: 增加 `X-API-Key` 的调用示例
- [ ] Step 2: 说明 401/403 语义及健康检查放行策略
- [ ] Step 3: 增加双 key 轮换操作说明
- [ ] Step 4: 文档一致性检查后提交

### 任务 8：发布前收口

**文件：**
- Modify/Create: release note 草稿与 checklist（如需要）

- [ ] Step 1: 汇总行为变化与兼容性说明
- [ ] Step 2: 确认测试通过和迁移文档完整
- [ ] Step 3: 准备下一标签发布要点
- [ ] Step 4: 最终提交并交接
