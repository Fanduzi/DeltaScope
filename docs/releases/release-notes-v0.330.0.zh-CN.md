# DeltaScope v0.330.0 发行说明

## 概要 — CI 与 PR 审查体验

v0.330.0 为在 pull request 上审查 SQL 变更补了一层薄展示：新增 `--format github-summary` 输出，用于 GitHub Actions 作业摘要；`--format github-actions` 的行内注解改得更清楚；并重写了 GitHub Actions 示例工作流，用仅 `contents: read` 的权限把 `config lint --strict` 门禁、行内注解和作业摘要串起来。

这只是展示与文档，不是机器人。作业摘要和注解全部由本地 CLI 生成，不发 PR 评论，不引入 GitHub App，不发起 GitHub API 或网络调用，不处理 token，也没有 `workflow_dispatch`。这些都不改变 SQL 的审核方式。

本版本**不**改变审核行为、默认策略、任何规则、parser 支持或任何机器可读输出结构。不存在 `severity` 字段，DeltaScope 继续使用 `level`，而非 `severity`。

## `--format github-summary` 输出

`deltascope audit --format github-summary` 输出 GitHub 风格 Markdown，用于追加到 `$GITHUB_STEP_SUMMARY`，也就是 GitHub 在失败检查顶部展示的那段内容：

```bash
deltascope audit --dialect postgresql --file ./migrations.sql \
  --format github-summary --fail-on none >> "$GITHUB_STEP_SUMMARY"
```

摘要渲染固定标题、判定结果、一个小的计数表，以及 markdown 报告本来就在本地生成的 Action Summary。只有 blocker 的审核显示 `Verdict: REJECT`；只有 warning 的显示 `Verdict: REVIEW`；干净审核显示 `Verdict: PASS`。判定与 DeltaScope 三值模型（`pass` / `review` / `reject`）一致，没有生造 `PASS`/`FAIL` 二值。

它是面向人的界面，**不是**稳定的机器 schema。在自动化里解析它不受支持；机器消费者应使用 `--format json`、`--format sarif` 或 `--format gitlab-codequality`。

## 更清晰的 GitHub Actions 注解

`--format github-actions` 本就输出行内 workflow-command 注解。v0.330.0 只改了**文案**，让每条注解自带上下文，不必翻日志：

- 注解**标题**改为 `[<level>] <rule_id>`（原来只有规则 id），例如 `title=[blocker] dml.where.require`。
- **消息**保留 finding 消息，把可选的 `Suggestion:` 单独放一行，并追加 `Explain: deltascope rules explain <rule_id>`。
- 不支持语句的 notice 不变，不带 `rules explain` 链接，因为不支持语句没有规则 id。

级别到命令的映射不变：`blocker` → `::error`、`warning` → `::warning`、`notice` → `::notice`。文件、行、列行为不变，`%` / 换行 / 回车的 workflow-command 转义也不变。

## 重写 GitHub Actions 示例

`docs/examples/github-actions.yml` 重写为 DeltaScope 今天在 CI 中该有的跑法：

- 只用 `permissions: contents: read`。
- `config lint --strict` 门禁：当 `deltascope.yaml` 存在规则级替换隐患时让作业失败（退出码 2）；没有配置文件时跳过。
- `--format github-actions` 用于行内注解。
- `--format github-summary --fail-on none` 放在 `if: always()` 下，即使注解步骤因 blocker 非零退出，摘要仍会出现。

不发 PR 评论，不需要 `pull-requests: write`，不引入 GitHub App，不发起 GitHub API 或网络调用，不处理 token，也没有 `workflow_dispatch`。安装脚本的版本锁定指向 `v0.330.0`，即首个同时提供 `github-summary` 和新注解文案的版本。

## 隐私 / 不泄漏

作业摘要和注解文案都不包含原始 SQL、规范化 SQL、parser 的 `near ...` 片段、密钥、连接串或实时元数据载荷。它们只呈现规则 id 与级别、finding 计数、目录提供的摘要与建议文案、`rules explain <rule_id>` 命令、从 1 开始的语句序号和全局范围标记，也就是 markdown 报告本来就在输出的、范围受限的字段。GitHub Actions 注解保留原有的 `%`、换行、回车转义。

## 非目标

- 不改变审核行为。
- 不改变默认策略或规则。
- 不改变 parser 支持。
- 不新增审核规则。
- 不改变 finding JSON 结构。
- 不改变 SDK/HTTP/MCP 响应结构。
- 不改变 JSON、SARIF 或 GitLab Code Quality 渲染器。
- 不引入 `severity` 字段，`level` 仍是公开的优先级字段。

## 规则目录事实

规则目录与 v0.320.0 相同。`github-summary` 和注解文案只是呈现已有 finding，不是规则变更。

| 指标 | 数量 |
|------|----:|
| 规则总数 | **371** |
| blocker | 72 |
| warning | 142 |
| notice | 157 |

| 方言范围 | 规则 |
|----------|----:|
| common | 177 |
| postgresql | 191 |
| mysql | 1 |
| tidb | 2 |

| 语句类型 | 规则 |
|----------|----:|
| ddl | 361 |
| dml | 10 |

## 未变指标

- SQL 语料：**582/582**，**100.0%**，**245 YAML** 夹具文件。
- PostgreSQL ALTER TABLE 配置条目：**53**（未变）。
- DDL 覆盖目录：**400** 条目（未变）。
- parser-error 总数：跨方言 29 例（未变）。

## 决策记录

`docs/decisions/2026-06-18-v0.330.0-ci-pr-review-ux.md`
