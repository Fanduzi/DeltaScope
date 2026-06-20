# DeltaScope v0.340.0 发行说明

## 概要 — 文档漂移守卫

v0.340.0 为当前公开文档和 CI 示例新增了一个发布阻断级的文档漂移守卫。它拦截近几个版本在实现落地后才暴露的漂移：过时的规则查看命令（`rules show`、`rules search`）、审计输出格式清单漂移、不安全或过时的 CI 工作流示例，以及示例版本锁定指向一个不支持所描述行为的发布。

守卫是一个静态、人工维护的检查器。它不执行文档片段，不调用外部服务，不处理 token，不运行 Docker，也不连接数据库。它读取当前公开文档和示例，把它们和已知的高风险公开契约逐项比对。

`docs-example-gates` 现在并入 `release-surface-gates`，过时的公开文档会在到达用户之前就阻断发布。它刻意不进 `make test`，因为这是面向发布的公开文档检查，不该拖慢日常开发循环。

这是仓库的发布契约，不是面向用户的新 DeltaScope CLI 功能。它不改变 SQL 的审核方式。

本版本**不**改变审核行为、默认策略、任何规则、parser 支持或任何机器可读输出结构。不存在 `severity` 字段，DeltaScope 继续使用 `level`。

## 守卫拦截什么

第一版覆盖范围是当前公开文档和示例：`README.md`、`README_ZH.md`、`docs/reference/cli.md`、`docs/reference/cli.zh-CN.md`、`docs/reference/config.md`、`docs/reference/config.zh-CN.md`、`docs/recipe/*.md`、`docs/recipe/*.zh-CN.md`，以及 `docs/examples/` 下的三个文件。

人工维护的检查项：

- 过时的规则查看命令，例如 `deltascope rules show` 和 `deltascope rules search`，并给出指向受支持的 `deltascope rules explain` 与 `deltascope rules list --search` 的修复提示。
- 审计输出格式清单，在文档列出受支持格式时核对（`markdown`、`json`、`github-actions`、`github-summary`、`sarif`、`gitlab-codequality`）。
- GitHub Actions 示例形态：只读的 `contents: read` 权限、不发 PR 评论、不处理 token、`config lint --strict` 门禁、`github-actions` 注解、`github-summary` 作业摘要，以及在传入 `$VERSION` 时 `DELTASCOPE_VERSION` 锁定与之匹配。
- GitLab 示例形态，包括原生的 `gitlab-codequality` 输出。
- 把 `severity field` 当作 DeltaScope 公开优先级的肯定式措辞，同时放行外部 schema 上下文和否定式澄清（如「no `severity` field」）。

## GitLab CI 示例：原生 Code Quality

为了让守卫在当前文档上通过，`docs/examples/gitlab-ci.yml` 已修正：GitLab CI 示例现在输出 DeltaScope 原生的 `--format gitlab-codequality`，并通过 `artifacts:reports:codequality` 暴露。finding 会以 Code Quality 注解的形式内联渲染在合并请求 diff 里。这只是文档示例修正；`gitlab-codequality` 渲染器本身与 v0.330.0 相比没有变化。

## 静态，天然不泄漏

检查器只用 Python 标准库。它不执行任何 Markdown 或 YAML 片段，也不发起网络、GitHub API、npm、Homebrew、Docker、数据库或 token 调用。问题输出包含文件路径、可用时的行号，以及一条修复提示。

## 非目标

- 不改变审核行为。
- 不改变默认策略或规则。
- 不改变 parser 支持。
- 不新增审核规则。
- 不改变 finding JSON 结构。
- 不改变 SDK/HTTP/MCP 响应结构。
- 不改变 JSON、SARIF、GitHub Actions 或 GitLab Code Quality 渲染器行为。
- 不引入 `severity` 字段，`level` 仍是公开的优先级字段。

## 规则目录事实

规则目录与 v0.330.0 相同。文档漂移守卫只是呈现已有契约，不是规则变更。

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

`docs/decisions/2026-06-19-v0.340.0-docs-drift-guard.md`
