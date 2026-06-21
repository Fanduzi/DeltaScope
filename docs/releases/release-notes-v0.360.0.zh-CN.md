# DeltaScope v0.360.0 发行说明

## 概要 — 配置与规则解释体验

v0.360.0 收紧了 `config lint`、`rules explain` 和 `config status` 三个命令之间可读文本的衔接。三者都用来回答同一个问题：一份 YAML 配置对某条规则到底意味着什么。本次只动文本，不改底层行为、原语或机器可读契约。

`config lint` 的替换风险告警改成了直接、可操作的写法。每条告警先说清规则策略是「整体替换」而非「与默认值合并」，再写明漏掉字段的具体后果，最后以 `Inspect effective rule status:` 收尾，指向 `deltascope config status <rule-id> --config <path>`。

`rules explain <rule-id>` 的文本输出新增三段：从真实默认策略生成的 `Default policy:`、保留默认 `enabled` 与 `params` 只改 `level` 的 `Safe override example:`，以及指向 `config status` 的 `Inspect effective rule status:`。

公开文档（中英双语）补上了 `config lint → rules explain → config status` 的工作流说明，与新文本契约一致。

这次只改文案。它不改变审核行为、默认策略、任何规则、parser 支持或任何机器可读输出结构。不存在 `severity` 字段，DeltaScope 继续使用 `level`。

## CLI 文本改了什么

- `config lint` 的告警现在渲染成多行块。每条替换风险告警直接点明后果：
  - 漏掉 `enabled`：`<rule> is OFF because "enabled" is omitted.`
  - 漏掉 `level`：`<rule> has no effective level because "level" is omitted.`
  - 整个 `params` 漏掉：`<rule> removes default params because "params" is omitted.`
  - 漏掉 `params.<key>`：`<rule> removes default "params.<key>" because that key is omitted.`
  - 框定语：`This config replaces the whole rule policy; it does not merge with defaults.`
  - 衔接：`Inspect effective rule status:`，后面跟 `deltascope config status <rule-id> --config <file path>`。
- `rules explain` 的文本现在会渲染 `Default policy:`、`Safe override example:` 和 `Inspect effective rule status:`。原来的 `Config Example:` 段由 `policy.Default()` 生成，和新 `Default policy:` 段逐字节相同，因此移除，避免把默认策略打印两次。`Default Params:` 作为只看参数的精简视图保留。
- 衔接标签是 `Inspect effective rule status:`。没有用 `Next:`，因为它描述的是具体的检查动作，而不是暗示一套通用向导流程。

## 哪些没变

- `config lint`、`config status`、`rules explain` 的 JSON 输出没有变化。`rules explain --format json` 仍带由 catalog 生成的 `config_example` 字段。
- 退出码没有变化。`config lint --strict` 在只有告警时输出与默认模式逐字节相同，只是退出码改为 `2`。校验错误仍然优先，并抑制告警块。
- `config status` 仍是单条规则的 ON/OFF 检查。
- 告警排序保持确定性。

## 非目标

- 不改 `LoadPolicy` 行为。
- 不引入规则策略的部分合并语义。
- 不改默认策略。
- 不改审核行为。
- 不改 parser 支持。
- 不新增审核规则。
- 不改 finding JSON 结构。
- 不改 SDK/HTTP/MCP 响应结构。
- 不改 JSON、SARIF、GitHub Actions 或 GitLab Code Quality 渲染器行为。
- 不做配置自动修复命令。
- 不做完整的生效策略导出。
- 不做 CLI 中文 / i18n 输出。中文指引只在文档里维护。
- 不引入 `severity` 字段，`level` 仍是公开的优先级字段。

## 规则目录事实

规则目录与 v0.340.0 相同。本次只动文本，不是规则变更。

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

`docs/decisions/2026-06-20-v0.360.0-config-rule-explain-ux.md`
