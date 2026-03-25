# CLI 元数据 E2E 实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**目标：** 为 DeltaScope 的元数据感知 CLI 路径增加可重复执行、基于 Docker 的端到端覆盖，目标是真实 MySQL 与 TiDB 实例。

**架构：** 保持默认单元/集成测试不变，另外增加一层独立的 Docker Compose + shell 型 E2E。CLI 仍是唯一公开测试界面；脚本负责准备数据库 fixture、等待就绪、执行 `deltascope audit` 命令，并基于 JSON 输出和退出码断言结果。

**技术栈：** Go、Docker Compose、MySQL 镜像、PingCAP TiDB 镜像、shell scripts、现有 CLI

---

### 任务 1：增加规划工件

**Files:**
- Create: `docs/plans/2026-03-22-cli-metadata-e2e-design.md`
- Create: `docs/plans/2026-03-22-cli-metadata-e2e-implementation.md`
- Create: `docs/plans/2026-03-22-cli-metadata-e2e-task-prompts.md`

**Step 1:** 保存已确认设计
**Step 2:** 保存实施计划与提示词
**Step 3:** commit

### 任务 2：增加 Docker Compose 与 fixture SQL

**Files:**
- Create: `docker/cli-e2e-compose.yaml`
- Create: `docker/mysql/init.sql`
- Create: `docker/tidb/init.sql`
- Modify: 如目录级文档需要感知新 fixture，则更新相关 README

**Step 1:** 定义 MySQL 与 TiDB 服务，并使用可预测端口与就绪假设
**Step 2:** 为唯一 schema、歧义 schema 与兼容性/存在性场景创建 fixture SQL
**Step 3:** 验证 fixture 意图与计划中的断言矩阵一致
**Step 4:** commit

### 任务 3：增加 E2E 执行脚本

**Files:**
- Create: `scripts/test_cli_metadata_e2e.sh`
- Modify: 如有需要，更新 `scripts/` 文档或 README 引用

**Step 1:** 编写可运行 `mysql`、`tidb` 或 `all` 的脚本
**Step 2:** 增加容器启动、就绪等待与清理逻辑
**Step 3:** 增加针对 CLI 输出与退出码的 JSON 断言 helper
**Step 4:** commit

### 任务 4：增加 MySQL 元数据感知 CLI E2E 覆盖

**Files:**
- Modify: `scripts/test_cli_metadata_e2e.sh`
- Modify: 如有需要，调整 fixture SQL

**Step 1:** 增加 MySQL 断言，覆盖方言检测、schema 推断、歧义、显式 schema、限定 SQL、基于元数据的 findings，以及 create-table 部分元数据行为
**Step 2:** 验证当预期错误时脚本会失败，预期正确时会通过
**Step 3:** commit

### 任务 5：增加 TiDB 元数据感知 CLI E2E 覆盖

**Files:**
- Modify: `scripts/test_cli_metadata_e2e.sh`
- Modify: 如有需要，调整 fixture SQL

**Step 1:** 增加 TiDB 断言，覆盖方言检测、schema 推断、歧义、限定 SQL、存在性检查，以及至少一个基于实例事实的规则
**Step 2:** 如果某些预期只适用于 MySQL，则在 TiDB 分支中保持诚实
**Step 3:** commit

### 任务 6：增加 Makefile 目标与使用文档

**Files:**
- Create/Modify: `Makefile`
- Modify: `README.md`
- Modify: `README_ZH.md`
- Modify: 受影响的模块 README 文件

**Step 1:** 增加 `make test-e2e-cli`、`make test-e2e-cli-mysql` 与 `make test-e2e-cli-tidb`
**Step 2:** 记录前置条件、本地使用方式，以及 E2E 与 `go test ./...` 分离这一事实
**Step 3:** commit

### 任务 7：最终验证与风险关闭

**Files:**
- Modify: `docs/plans/2026-03-20-overnight-handoff.md`
- Modify: `docs/plans/2026-03-20-autonomous-progress.md`
- Modify: `docs/plans/2026-03-20-deltascope-v1-decisions.md`
- Modify: 所有提及缺少 live smoke 的 handoff/progress 文档

**Step 1:** 运行完整验证：
- `go test ./...`
- `make test-e2e-cli-mysql`
- `make test-e2e-cli-tidb`
- 任意受影响代码/文档对应的 three-level-doc 检查
**Step 2:** 移除旧的“尚未进行 live smoke”风险表述，并替换为新的 E2E 证据
**Step 3:** commit
**Step 4:** push