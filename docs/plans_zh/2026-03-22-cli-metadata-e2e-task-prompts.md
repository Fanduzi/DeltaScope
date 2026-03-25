# CLI 元数据 E2E 任务提示词

> 用于 `CLI Metadata E2E` 里程碑的逐任务实施与评审。
> 每个提示词都假设工作发生在 `/Users/fan/GolangProjects/deltascope` 中。

## 全局规则

- 保持现有 CLI 契约不变；这个里程碑只是通过真实目标验证它。
- 只测试公共 CLI，不要通过内部 helper 绕过它。
- 让容器化 E2E 与常规 `go test` 工作流分离。
- 使用稳定的 fixture schema 与表，使歧义与推断行为具备确定性。
- 优先用 JSON 断言，不要依赖 Markdown 字符串匹配。
- 将 `three-level-doc` 视为对任何代码或文档变更的硬门槛。
- 每个任务都返回变更文件、运行命令、状态与 commit hash。

## 里程碑重点

- 真实 MySQL 元数据感知 CLI smoke
- 真实 TiDB 元数据感知 CLI smoke
- schema 推断与歧义覆盖
- 带 schema 限定 SQL 覆盖
- 基于元数据的存在性与兼容性覆盖
- 通过 Make 目标提升本地开发体验

## 任务意图

### 任务 1：规划工件

- 保存已确认的设计、实施计划与提示词。

### 任务 2：Docker 与 Fixtures

- 增加可重复的 MySQL 与 TiDB 服务，以及能同时制造唯一目标和歧义目标的 fixture SQL。

### 任务 3：E2E 脚本

- 增加一份脚本，可以对 MySQL、TiDB 或两者运行元数据感知 CLI 测试套件。
- 保持清理与就绪处理具备确定性。

### 任务 4：MySQL 覆盖

- 通过真实 CLI 调用与 JSON 断言证明 MySQL 路径可用。

### 任务 5：TiDB 覆盖

- 通过真实 CLI 调用与 JSON 断言证明 TiDB 路径可用。

### 任务 6：Makefile 与文档

- 增加简单的本地入口，并记录如何运行 E2E 套件。

### 任务 7：收口

- 重新运行验证。
- 从 handoff/progress 文档中去掉剩余的“尚未 live smoke”风险。
- 让该里程碑处于可发布状态。