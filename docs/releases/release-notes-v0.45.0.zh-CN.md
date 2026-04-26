# DeltaScope v0.45.0 发行说明

## 概要

DeltaScope 现在可以直接输出 GitLab Code Quality 报告，合并请求流水线无需任何后处理即可将 SQL 审计发现展示为行内代码质量注解。

## 新功能

### GitLab Code Quality 输出格式

- `--format gitlab-codequality` CLI 标志生成符合 [GitLab Code Quality 报告](https://docs.gitlab.com/ee/ci/testing/code_quality.html) 契约的 JSON 数组。
- 每条 DeltaScope 发现映射为一条 Code Quality 条目，包含 `check_name`、`description`、`severity`、`fingerprint` 和 `location` 字段。
- 来自 `--file` 的文件路径传播到 `location.path`；内联 SQL 使用审计输入文件名。
- 将报告作为 CI 制品添加到 `artifacts:reports:codequality`，即可在合并请求差异中直接查看发现。

### 契约测试与发布关卡

- 契约特征测试锁定所需的 JSON 形状和语义字段保证。
- 单元测试覆盖零发现、单条发现和多条发现场景。
- `make release-gitlab-codequality-smoke` 关卡在发布流水线中验证构建的 CLI 二进制文件是否符合契约。
- `make release-contract-gates` 现已包含 GitLab Code Quality 冒烟测试。

### 文档

- 新增教程：[在 GitLab CI 中使用 DeltaScope](../recipe/use-deltascope-in-gitlab-ci.zh-CN.md)，包含分步 `.gitlab-ci.yml` 配置。
- CLI 参考文档更新，记录 `--format` 标志。
- 审计能力矩阵更新，列出 GitLab Code Quality 为受支持的输出格式。

## 升级说明

- CLI 标志、公共 API、规则行为和现有输出格式无破坏性变更。
- `--format json`（默认）行为不变。
- 新增 `--format gitlab-codequality` 标志为纯增量；现有 CI 配置无需修改即可继续使用。

## 范围确认

- 无 parser、spec、domain-rule 或 policy 变更。
- 无 HTTP、MCP 或 `pkg/deltascope` 生产代码变更。
- 无新增依赖。
