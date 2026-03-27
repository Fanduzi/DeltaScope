# 官方 MCP Server 实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**目标：** 交付官方 `deltascope-mcp` stdio server，通过 MCP 暴露 DeltaScope 的审计与规则发现能力，并同时支持离线与 metadata-aware 审计流程。

**架构：** 在现有共享审计路径与规则目录之上新增一层 MCP 接口层。复用 `pkg/deltascope` 的结果语义，在需要时抽取共享 metadata 连接准备逻辑，保持 MCP 传输层足够薄，并且只在 `audit_sql` 成功结果上新增顶层 `context` 字段。

**技术栈：** Go、现有 DeltaScope application/domain/infrastructure 分层、`pkg/deltascope`、规则目录、metadata provider 基础设施、MCP server 运行时/库、Markdown 文档、Go 测试

---

### Task 1: 保存里程碑规划文档

**Files:**
- Create: `docs/plans/2026-03-26-official-mcp-server-design.md`
- Create: `docs/plans/2026-03-26-official-mcp-server-implementation.md`
- Create: `docs/plans/2026-03-26-official-mcp-server-task-prompts.md`
- Create: `docs/plans_zh/2026-03-26-official-mcp-server-design.md`
- Create: `docs/plans_zh/2026-03-26-official-mcp-server-implementation.md`
- Create: `docs/plans_zh/2026-03-26-official-mcp-server-task-prompts.md`

**Step 1:** 以中英双语保存已确认的设计、实施计划与任务提示词
**Step 2:** 检查六份文档在命名、范围与术语上的一致性
**Step 3:** commit

### Task 2: 选择并接入 MCP runtime

**Files:**
- Modify/Create: `go.mod`
- Create: `internal/interfaces/mcp/...`
- Create: `cmd/deltascope-mcp/main.go`
- Modify: 受影响的模块 `README.md`
- Test: 新增 MCP interface 测试

**Step 1:** 选择最适合本地 stdio server 的最小 Go MCP runtime/library
**Step 2:** 添加依赖并搭建薄的 MCP server 启动骨架
**Step 3:** 首版只暴露 stdio 启动方式
**Step 4:** 为 server 启动与工具注册增加聚焦 smoke tests
**Step 5:** commit

### Task 3: 定义共享 MCP 请求、成功返回与错误契约

**Files:**
- Create/Modify: `internal/interfaces/mcp/...`
- Modify: 因公开契约变化而受影响的 reference 文档与模块 README
- Test: MCP 包内的契约测试

**Step 1:** 先写失败测试，约束 `audit_sql` 成功返回必须保留 `v0.6.2` 结果主体并仅新增顶层 `context`
**Step 2:** 定义 `audit_sql`、`describe_rule` 与 `list_rules` 的 MCP 请求/响应类型
**Step 3:** 定义请求、连接、配置与内部错误的稳定结构化错误码
**Step 4:** 确保成功与错误 payload 都不泄露密码、完整 DSN 或原始连接结构
**Step 5:** 运行聚焦测试
**Step 6:** commit

### Task 4: 抽取共享 metadata 连接准备逻辑

**Files:**
- Modify: 必要的共享 audit/helper 包
- Modify: `internal/interfaces/cli/...`
- Create/Modify: `internal/interfaces/mcp/...` 下的连接 helper 文件
- Modify: 受影响模块的 `README.md`
- Test: 聚焦 helper 测试

**Step 1:** 识别 CLI 中应抽共享而不是复制的 metadata-preparation 逻辑
**Step 2:** 抽取可复用的连接校验、方言探测与 schema 解析 helper
**Step 3:** 抽取后保持现有 CLI 行为稳定
**Step 4:** 同时运行 CLI 与共享 helper 的聚焦测试
**Step 5:** commit

### Task 5: 实现 `connection_ref` 与直接连接解析

**Files:**
- Create/Modify: MCP 连接配置与解析文件
- Possibly Create: 如复用价值足够，新增一个小型连接配置读取包
- Modify: 连接配置格式相关文档
- Test: 连接配置与校验测试

**Step 1:** 先写失败测试，覆盖 `connection_ref` 查找、直接 `connection` 输入与互斥校验
**Step 2:** 实现从 `~/.config/deltascope/connections.yaml` 读取命名连接
**Step 3:** 支持直接 `connection` 输入，包括 `host`、`port`、`socket`、`user`、`schema`、`dialect` 与唯一密码来源
**Step 4:** 明确校验 `password`、`password_env`、`password_file` 只能三选一
**Step 5:** 让直接连接与命名连接都归一化为同一个内部连接结构
**Step 6:** 运行聚焦测试
**Step 7:** commit

### Task 6: 实现 `audit_sql`

**Files:**
- Modify/Create: `internal/interfaces/mcp/...`
- Modify: 为整洁集成所需的共享请求整形 helper
- Test: `audit_sql` 聚焦测试

**Step 1:** 先写离线 `audit_sql` 成功返回的失败测试
**Step 2:** 再写 metadata-aware `audit_sql` 的失败测试，覆盖 `connection_ref` 与直接 `connection`
**Step 3:** 直接调用共享 DeltaScope 审计路径，而不是 shell 到 CLI
**Step 4:** 新增顶层 `context` 字段，稳定返回 mode、dialect、schema 与 metadata source 信息
**Step 5:** 确保 metadata-aware 连接失败返回结构化错误，而不是回退到 offline
**Step 6:** 运行聚焦测试
**Step 7:** commit

### Task 7: 实现 `describe_rule` 与 `list_rules`

**Files:**
- Modify/Create: `internal/interfaces/mcp/...`
- Modify: 如接口或发现语义需要说明，更新规则目录相关 README
- Test: MCP 规则工具测试

**Step 1:** 先写按 `rule_id` 查询规则的失败测试
**Step 2:** 先写带实用过滤条件的规则列表失败测试
**Step 3:** 基于已发布规则目录实现 `describe_rule`
**Step 4:** 用小而稳定的过滤集合实现 `list_rules`，避免引入宽泛查询语言
**Step 5:** 确保返回元数据与现有规则目录和公开文档保持一致
**Step 6:** 运行聚焦测试
**Step 7:** commit

### Task 8: 增加 secret-redaction 与错误加固覆盖

**Files:**
- Modify: MCP 错误与日志 helper
- Modify: 必要时调整连接解析代码
- Test: redaction、配置与失败路径测试

**Step 1:** 先写失败测试，证明返回错误中不会出现密码与 DSN
**Step 2:** 在连接与配置错误路径中做敏感信息脱敏
**Step 3:** 增加 missing env、密码文件不可读、配置文件非法与连接失败等覆盖
**Step 4:** 保持错误 `code` 稳定、`message` 对用户可读
**Step 5:** 运行聚焦测试
**Step 6:** commit

### Task 9: 文档化官方 MCP 界面

**Files:**
- Modify: `README.md`
- Modify: `README_ZH.md`
- Create/Modify: `docs/reference/...`
- Create/Modify: `docs/recipe/...`
- Modify: `cmd/deltascope/README.md`
- Create/Modify: `cmd/deltascope-mcp/README.md`
- Modify: 受影响模块 `README.md`

**Step 1:** 增加 `deltascope-mcp` 的产品级文档
**Step 2:** 文档化 tool 输入、结果结构与错误码
**Step 3:** 文档化 `connection_ref`、直接 `connection` 与 secret-handling 指引
**Step 4:** 增加 offline 与 metadata-aware 的 MCP 调用示例
**Step 5:** 在适用范围内保持中英文文档对齐
**Step 6:** 运行链接与内容 sanity check
**Step 7:** commit

### Task 10: 最终验证与里程碑收口

**Files:**
- Modify: `docs/plans/2026-03-20-overnight-handoff.md`
- Modify: `docs/plans/2026-03-20-autonomous-progress.md`
- Modify: `docs/plans/2026-03-20-deltascope-v1-decisions.md`
- Modify: 任意被修改的模块 `README.md`

**Step 1:** 运行 MCP 包测试、规则工具测试与 metadata-aware 审计测试
**Step 2:** 再运行更广泛的验证，例如 `go test ./...`
**Step 3:** 验证 `audit_sql` 成功返回保留 `v0.6.2` 审计主体，并且只新增顶层 `context`
**Step 4:** 若当前仓库流程要求，运行 three-level-doc 校验
**Step 5:** 更新 handoff、progress 与 decisions，记录 MCP 里程碑结果
**Step 6:** commit
**Step 7:** push
