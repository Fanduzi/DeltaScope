# CLI 补完实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**目标：** 通过暴露元数据感知审计访问、交付规则/配置/能力工具，并补齐剩余帮助、错误与输出缺口，使 DeltaScope CLI 不再存在主要能力缺面。

**架构：** 保持一套共享审计引擎。扩展 CLI 适配层与稳定的公共请求绑定，使命令层能够按需构建元数据感知请求；然后新增一层规则目录元数据，用于支撑新的自解释规则命令，而不污染规则执行接口。

**技术栈：** Go、Cobra、Viper、现有 application/domain/infrastructure 分层、MySQL driver、Go testing

---

### 任务 1：增加 CLI 补完设计工件

**Files:**
- Create: `docs/plans/2026-03-21-cli-completion-design.md`
- Create: `docs/plans/2026-03-21-cli-completion-implementation.md`
- Create: `docs/plans/2026-03-21-cli-completion-task-prompts.md`

**Step 1:** 保存已确认的 CLI Completion 设计
**Step 2:** 保存实施计划与任务提示词
**Step 3:** commit

### 任务 2：为 CLI 元数据感知使用扩展公共与 application 审计请求

**Files:**
- Modify: `pkg/deltascope/audit.go`
- Modify: `pkg/deltascope/README.md`
- Modify: `internal/application/audit/service.go`
- Modify: `internal/application/audit/README.md`
- Test: `pkg/deltascope/audit_test.go`
- Test: `internal/application/audit/service_test.go`

**Step 1:** 为元数据感知请求链路编写失败测试
**Step 2:** 增加 CLI 侧元数据感知审计所需的稳定请求字段
**Step 3:** 在元数据字段缺失时保持离线路径不变
**Step 4:** 运行聚焦测试
**Step 5:** commit

### 任务 3：在 `audit` 中增加连接参数解析与密码提示

**Files:**
- Modify: `internal/interfaces/cli/root.go`
- Modify: `internal/interfaces/cli/audit.go`
- Modify: `internal/interfaces/cli/README.md`
- Test: `internal/interfaces/cli/cli_test.go`

**Step 1:** 为类 MySQL 连接参数与互斥规则编写失败测试
**Step 2:** 增加 `-h/-P/-u/-p/--ask-password/-D/-S` 参数解析
**Step 3:** 增加无回显密码提示支持
**Step 4:** 保持离线模式行为不变
**Step 5:** 运行聚焦测试
**Step 6:** commit

### 任务 4：构建 CLI 侧元数据感知接线与 schema 解析

**Files:**
- Modify: `internal/interfaces/cli/audit.go`
- Modify: `internal/infrastructure/metadata/mysql/provider.go`
- Create/Modify: CLI 连接建立与 schema 推断相关 helper 文件
- Test: CLI 与 provider 聚焦测试

**Step 1:** 为 TCP/socket 连接、schema 推断与歧义处理编写失败测试
**Step 2:** 基于 CLI 连接输入创建元数据 provider
**Step 3:** 实现 schema 解析规则，包括显式 schema、唯一推断、歧义失败，以及 create-table 部分元数据行为
**Step 4:** 接入方言自动检测与显式方言不匹配错误
**Step 5:** 运行聚焦测试
**Step 6:** commit

### 任务 5：增加规则目录元数据

**Files:**
- Create/Modify: `internal/domain/rule/...` 下的规则目录文件
- Modify: 受影响规则 README 文件
- Test: 新增聚焦目录的测试

**Step 1:** 为目录查询、列表与规则元数据完整性编写失败测试
**Step 2:** 增加基于 `rule_id` 键控的规则目录模型
**Step 3:** 为已发布规则附加摘要、示例、参数与 metadata-aware 标记
**Step 4:** 运行聚焦测试
**Step 5:** commit

### 任务 6：增加 `rules list/show/search` 命令

**Files:**
- Create: `internal/interfaces/cli/rules.go`
- Create/Modify: 命令相关 README 文件
- Test: `internal/interfaces/cli/cli_test.go`

**Step 1:** 为 `rules list`、`rules show` 与 `rules search` 编写失败 CLI 测试
**Step 2:** 实现命令接线与过滤输出
**Step 3:** 确保 `rules show` 输出示例、配置示例与修复建议
**Step 4:** 运行聚焦测试
**Step 5:** commit

### 任务 7：增加 `config lint` 与 `config show-default`

**Files:**
- Create: `internal/interfaces/cli/config.go` 或按需新增同级命令文件
- Modify: 如有需要，调整配置加载 helper
- Modify: `internal/interfaces/cli/README.md`
- Test: `internal/interfaces/cli/cli_test.go`

**Step 1:** 为配置校验与默认配置打印编写失败测试
**Step 2:** 实现 `config lint`
**Step 3:** 实现 `config show-default`
**Step 4:** 运行聚焦测试
**Step 5:** commit

### 任务 8：增加 `capabilities` 命令

**Files:**
- Create: `internal/interfaces/cli/capabilities.go`
- Modify: 能力来源文档或 helper package（如有需要）
- Test: `internal/interfaces/cli/cli_test.go`

**Step 1:** 为稳定的 capabilities 摘要编写失败测试
**Step 2:** 实现对方言、输入、输出、模式、元数据事实和产品界面的能力报告
**Step 3:** 保持输出对人类和 agent 都简洁且稳定
**Step 4:** 运行聚焦测试
**Step 5:** commit

### 任务 9：补齐 CLI UX 缺口

**Files:**
- Modify: `internal/interfaces/cli/...`
- Modify: `cmd/deltascope/README.md`
- Modify: `README.md`
- Modify: `README_ZH.md`
- Test: `internal/interfaces/cli/cli_test.go`

**Step 1:** 为更好的 help、清晰错误与元数据感知输出细节编写失败测试
**Step 2:** 优化 help 文本、示例、quiet 输出、JSON 细节，以及连接/schema/方言错误信息
**Step 3:** 用英文和中文文档补充离线与元数据感知示例
**Step 4:** 运行聚焦测试
**Step 5:** commit

### 任务 10：最终验证与里程碑收口

**Files:**
- Modify: `docs/plans/2026-03-20-overnight-handoff.md`
- Modify: `docs/plans/2026-03-20-autonomous-progress.md`
- Modify: `docs/plans/2026-03-20-deltascope-v1-decisions.md`
- Modify: 所有变更过的模块 `README.md`

**Step 1:** 运行完整验证，包括 CLI 测试、包测试、配置模板检查与 three-level-doc 校验
**Step 2:** 用最终 CLI 里程碑结果更新 handoff/progress/decision 文档
**Step 3:** commit
**Step 4:** push