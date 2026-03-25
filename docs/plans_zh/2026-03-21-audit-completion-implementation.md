# 审计能力补完实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**目标：** 完成 DeltaScope 的审计能力里程碑，包括能力验收矩阵、可选的元数据感知增强、更深层的 alter 兼容性、剩余审计缺口补齐，以及升级后的公共文档。

**架构：** 保持当前离线优先的审计流程，并将元数据作为可选事实加入。无论是否接入实时元数据，同一套 application/domain 路径都应生效。规则应继续消费规范化 spec 与快照，而不是直接依赖数据库客户端。

**技术栈：** Go、现有 domain/application/infrastructure 分层、TiDB parser、Cobra/Viper、标准 SQL 访问、Go testing

---

### 任务 1：建立能力矩阵基线

**Files:**
- Create: `docs/plans/2026-03-21-audit-capability-matrix.md`
- Modify: `docs/plans/2026-03-20-deltascope-v1-decisions.md`

**Step 1:** 枚举重要审计能力及其当前状态
**Step 2:** 将每一行标记为已覆盖、已替代、已延期或不在范围内
**Step 3:** 识别仍阻塞里程碑完成的精确缺口
**Step 4:** commit

### 任务 2：增加元数据领域抽象

**Files:**
- Modify: `internal/domain/spec/...`
- Create/Modify: 根据需要更新元数据相关 README 文件

**Step 1:** 为实例事实与表快照领域类型编写失败测试
**Step 2:** 增加实例事实与表快照的规范化元数据结构
**Step 3:** 重新运行定向测试
**Step 4:** commit

### 任务 3：增加元数据提供者接口与基础设施适配器

**Files:**
- Modify: `internal/application/audit/...`
- Create/Modify: `internal/infrastructure/...` 下元数据提供者文件
- Test: provider 与 orchestration 测试

**Step 1:** 为可选元数据感知编排编写失败测试
**Step 2:** 增加 provider 接口及基于 MySQL/TiDB 的实现
**Step 3:** 在未配置 provider 时保持离线模式行为不变
**Step 4:** 运行定向测试
**Step 5:** commit

### 任务 4：增加对象存在性与快照驱动规则

**Files:**
- Modify: `internal/domain/rule/ddl/...`
- Modify: `internal/domain/policy/defaults.go`
- Modify: `configs/deltascope.example.yaml`
- Test: 新增 DDL 规则测试

**Step 1:** 为表/列/索引/主键存在性检查编写失败测试
**Step 2:** 基于表快照与对象事实实现规则
**Step 3:** 接入默认配置与配置模板
**Step 4:** 重新运行定向测试
**Step 5:** commit

### 任务 5：深化 alter 兼容性检查

**Files:**
- Modify: `internal/domain/spec/...`
- Modify: `internal/domain/rule/ddl/...`
- Modify: `internal/application/audit/...`
- Test: 聚焦兼容性的 alter 测试

**Step 1:** 为源到目标类型兼容性与显式形状变更编写失败测试
**Step 2:** 实现更深入的兼容性事实与规则
**Step 3:** 对暂不支持的情况保持诚实，不要过度声明
**Step 4:** 运行定向测试
**Step 5:** commit

### 任务 6：根据矩阵补齐剩余重要 DDL/DML 缺口

**Files:**
- Modify: 由矩阵识别出的具体 rule/spec/policy 文件
- Test: 规则级测试

**Step 1:** 取出矩阵中仍标记为缺口的行
**Step 2:** 只实现价值最高的剩余缺口
**Step 3:** 重新运行定向测试并更新矩阵状态
**Step 4:** commit

### 任务 7：升级面向产品的文档与发布面

**Files:**
- Modify: `README.md`
- Create: `README_ZH.md`
- Create: `CHANGELOG.md`
- Create: `SECURITY.md`
- Modify: 相关模块 README 文件

**Step 1:** 以更成熟的产品定位与快捷链接重写英文 README
**Step 2:** 增加中文 README，并保持结构对齐
**Step 3:** 增加 shields、发布/版本引用、changelog 与安全指引
**Step 4:** 更新受版本化与元数据模式影响的模块文档
**Step 5:** commit

### 任务 8：最终验证与里程碑收口

**Files:**
- Modify: `docs/plans/2026-03-20-overnight-handoff.md`
- Modify: `docs/plans/2026-03-20-autonomous-progress.md`
- Modify: `docs/plans/2026-03-20-deltascope-v1-decisions.md`
- Modify: `docs/plans/2026-03-21-audit-capability-matrix.md`

**Step 1:** 运行完整验证，包括 CLI、HTTP、配置模板与 three-level-doc 检查
**Step 2:** 更新 handoff/progress/decision 文档，并最终确认矩阵状态
**Step 3:** commit
**Step 4:** push