# DeltaScope v0.510.2 发行说明

## 概要

v0.510.2 发布既有的 #60–#62 决策：含未审计 unsupported statement 的原本 `pass` 审查结果降到 `review`；活跃 MCP launcher 示例使用 `npx -y --prefer-online @fanduzi/deltascope-mcp@latest`；单独调用 `deltascope-mcp version` 和 `help` 会在 MCP stdio 启动前精确别名到 `-version` 和 `-help`。

DeltaScope 仍是静态分析：不执行提交的 SQL、不返回查询结果，也不做授权判定。MCP 没有 Query Access 工具。已注册规则目录仍为 373 条。支持的 rule-and-dialect fixture coverage 仍为 586/586（100.0%），共 286 个 YAML fixture；这不是 SQL syntax 或 grammar coverage。

## 修复

- #60：当 unsupported statement 未被审计时，原本为 `pass` 的审查结果变为 `review`；已有 `review` 和 `reject` 不降级。
- #61：规范 MCP launcher 示例刷新 npm metadata 并选择 latest dist-tag，不改变 launcher runtime 行为。
- #62：单独 positional `version` 和 `help` 保持既有 dashed 形式的 stdout、stderr 和退出码，并且不启动 server。

## 非目标

- 不引入新的 verdict 枚举、parser、命令框架、MCP 工具、SQL 执行或授权功能。
- 不改变规则目录，也不是 SQL syntax 或 grammar coverage 声明。
- 不引入 severity 字段。

## 规则目录事实

| 指标 | 数量 |
|------|------:|
| 规则总数 | **373** |
| blocker | 73 |
| warning | 142 |
| notice | 158 |

## 语料与目录事实

- 支持的 rule-and-dialect fixture coverage：**586/586**、**100.0%**、**286** 个 YAML fixture；这不是 SQL syntax 或 grammar coverage。
- PostgreSQL ALTER TABLE 配置项：**53**。
- DDL 覆盖目录：**407** 条（mysql 62，tidb 55，postgresql 290，parser_upgrade_candidate 18）。

## 决策记录

- `docs/decisions/2026-08-30-unsupported-statement-verdict-review-floor.md`（#60）
- `docs/decisions/2026-08-30-mcp-launcher-upgrade-safe-install.md`（#61）
- `docs/decisions/2026-08-30-mcp-positional-meta-invocation.md`（#62）
