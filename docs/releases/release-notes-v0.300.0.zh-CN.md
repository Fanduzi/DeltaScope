# DeltaScope v0.300.0 发行说明

## 概要 — 审计 Action Summary

v0.300.0 为默认 markdown 审计报告新增 **Action Summary**。当审计产生 finding 时，报告会在开头渲染 `## Action Summary` 段落，按规则分组、按修复优先级排序，并为每个分组给出 `deltascope rules explain <rule_id>` 下一步命令。本版本**不**新增审计规则、不改变规则评估行为、不改变审计行为、不改变 finding JSON 结构、不引入 `severity` 字段、不新增 parser 支持，也不改变 SDK/HTTP/MCP/SARIF/GitHub Actions/GitLab Code Quality 输出。

## Markdown 审计输出中的 Action Summary

默认 `deltascope audit` 路径和 `--format markdown` 在存在 finding 时，会在报告开头渲染 `## Action Summary` 段落：

```bash
deltascope audit --file ./migration.sql
deltascope audit --format markdown --file ./migration.sql
```

该段落由已有审计 finding 和规则目录元数据派生，**不**重新运行审计、不解析 SQL、不评估规则、不检查原始 SQL，也不会在既有按语句段落之外新增原始 SQL。摘要通过 1-based 语句序号引用 finding，而非 SQL 片段。

### 分组与排序

每个 action item 按 `rule_id` 分组。每个分组展示：

- `[level] \`rule_id\`: N finding(s)`，其中 `level` 是该规则观察到的最高优先级（`blocker`、`warning` 或 `notice`）。
- 目录支持的 `Summary:` 与 `Suggestion:` 文本；当规则不在内置目录中时回退到既有的 finding 消息/建议。
- `Explain: deltascope rules explain <rule_id>` 作为下一步。
- 去重后的 1-based `Statements:` 序号（仅全局 finding 时省略），以及可选的 `Scope: global` 标记。

分组按 `level` 优先级（`blocker` → `warning` → `notice`）、再按 finding 数量降序、最后按 `rule_id` 升序确定性地排序。

### 截断与干净审计

- Markdown 输出最多渲染 10 个规则分组。当分组更多时，末尾输出 `Showing 10 of N rule groups.`。
- 干净审计（无 finding）完全省略 `## Action Summary` 段落。

### 隐私

Action Summary 不携带原始 SQL、规范化 SQL、parser `near ...` 文本、仅从用户 SQL 派生的对象名、来自实时数据库的元数据值，也不携带原始 finding 元数据 map。目录外规则的回退文本仅限审计结果已产生的既有 finding 消息和建议。

## 未变更的输出（机器契约）

- 审计 JSON 输出未变更。审计 JSON 中**不存在** `action_summary` 字段。
- Finding JSON 结构未变更，不新增、不重命名任何 finding 字段。
- `level` 仍是公开的优先级字段（`blocker`、`warning`、`notice`）。公开输出中不存在 `severity` 字段；DeltaScope 继续使用 `level`，而非 `severity`。
- SDK、HTTP、MCP、SARIF、GitHub Actions 和 GitLab Code Quality 输出均未变更。

## 规则目录统计

规则目录与 v0.290.0 一致。Action Summary 是渲染变更，不是规则变更。

| 指标 | 数量 |
|------|-----:|
| 规则总数 | **371** |
| blocker | 72 |
| warning | 142 |
| notice | 157 |

| 方言范围 | 规则数 |
|----------|------:|
| common | 177 |
| postgresql | 191 |
| mysql | 1 |
| tidb | 2 |

| 语句类型 | 规则数 |
|----------|------:|
| ddl | 361 |
| dml | 10 |

## 非目标

- 不新增审计规则。
- 不改变规则评估行为。
- 不改变审计行为。
- 不改变 finding JSON 结构。
- 不引入 `severity` 字段。
- 不改变 parser 支持。
- 不改变 SDK/HTTP/MCP/SARIF/GitHub Actions/GitLab Code Quality 输出。
- 不新增 `report` 子命令。
- 不声称完整修复所有迁移风险；摘要只指向下一步，不会自动修复任何内容。

## 未变更指标

- SQL 语料库：**582/582**，**100.0%**，**245 YAML** 文件。
- PostgreSQL ALTER TABLE 配置条目：**53**（未变更）。
- PostgreSQL 合并 DDL 普查：**285/274/6/0/5/0**（未变更）。
- DDL 覆盖范围目录：**400** 条记录（MySQL 61 / TiDB 54 / PostgreSQL 285 / parser_upgrade_candidate 18）（未变更）。
- Parser-error 总计：跨方言 **29** 个用例（未变更）。

## 决策记录

`docs/decisions/2026-06-13-v0.300.0-audit-action-summary.md`
