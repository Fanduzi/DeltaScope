# HTTP API 服务任务提示

> 用于 `HTTP API Service` 里程碑的逐任务实施和审查。
> 每个提示假设工作在 `/Users/fan/GolangProjects/deltascope` 内进行。

## 全局规则

- 保持 HTTP 层薄且仅适配器。
- 复用现有审计核心而不是分叉 CLI 逻辑。
- 保持 `three-level-doc` 作为硬关卡。
- 每个任务返回更改的文件、运行的测试、状态和提交哈希。

## 里程碑焦点

- JSON 审计端点
- 健康/版本端点
- 配置支持的长期运行服务器接线
- 文档和验证关闭
