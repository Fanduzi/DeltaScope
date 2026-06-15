# DeltaScope v0.310.0 发行说明

## 概要 — 规则配置状态

v0.310.0 新增一个聚焦的 CLI 检查命令 `deltascope config status <rule-id>`，报告某条内置规则在当前配置下是 ON 还是 OFF，以及触发时会使用哪个 `level`。它同时说明用户的配置相对内置默认策略改动了哪些字段，并指向 `deltascope rules explain <rule-id>` 查看规则含义。本版本**不**新增审计规则、不改变规则行为、不改变审计行为、不改变 finding JSON 结构、不引入 `severity` 字段、不新增 parser 支持，也不改变 SDK/HTTP/MCP 输出。DeltaScope 继续使用 `level`，而非 `severity`。

## `config status` 命令

```bash
deltascope config status <rule-id> [--format text|json]
```

示例：

```bash
deltascope config status dml.where.require
deltascope --config ./deltascope.yaml config status dml.where.require
deltascope --config ./deltascope.yaml config status dml.where.require --format json
```

该命令回答一个实用问题：*在当前配置文件下，这条规则是开还是关，触发时会用哪个级别？*

它报告：

- 该规则当前是 **ON** 还是 **OFF**。
- 规则开启时使用的有效 `level`（`blocker`、`warning` 或 `notice`）。
- 用户的配置是否相对默认策略改动了 `enabled`、`level` 或 params，并并排展示默认值与当前值。
- 一个直接的下一步：`deltascope rules explain <rule-id>` 查看规则含义。

### 配置来源

该命令复用既有的全局 `--config` 标志，与 `deltascope audit` 一致。未提供 `--config` 时，`config status` 报告内置默认策略，并说明当前没有配置覆盖处于生效状态。`config lint --file` 仍是仅做校验的命令，行为不变。

### 输出格式

- `--format text`（默认）面向人类：说明 `ON` 或 `OFF`、有效 `level`、简明的配置效果解释、默认值与当前值，以及指向 `deltascope rules explain <rule-id>` 的 `Rule details:` 链接。
- `--format json` 返回面向自动化的稳定包装结构。JSON 输出包含顶层 `version` 字段（DeltaScope 构建版本）、`rule_id`、`status`（`enabled`、`level`、`state`）、`default`、`current`、`config_effect` 与 `rule_details_command`。JSON 输出中不存在 `severity` 字段。

最小 JSON 结构（完整字段覆盖，仅 `level` 不同）：

```json
{
  "version": "v0.310.0",
  "rule_id": "dml.where.require",
  "status": { "enabled": true, "level": "warning", "state": "on" },
  "config_effect": { "has_config": true, "has_override": true, "changed_fields": ["level"] },
  "rule_details_command": "deltascope rules explain dml.where.require"
}
```

## 规则级替换语义

`config status` 报告的有效策略**与审计路径应用的方式完全一致**。它原样读取已加载的策略，绝不会把部分规则配置静默合并到默认值之上，因为审计本身也不会这样做。

在 YAML 中提及一条规则会替换该规则的整条策略：被省略的字段变为零值。这不是部分合并。

| 字段 | 省略时的有效值 |
|---|---|
| `enabled` | `false` |
| `level` | `""`（空） |
| `params` | `nil`（空） |

**后果：** 用户若只写

```yaml
rules:
  dml.where.require:
    level: warning
```

本意是放宽级别，实际却会**关闭该规则**，因为 `enabled` 被省略、从而被替换为 `false`。`config status` 会显式指出这一点，而非隐藏它。若只想改级别同时保持规则开启，需写全所有字段，让替换保留其余字段：

```yaml
rules:
  dml.where.require:
    enabled: true
    level: warning
    params:
      required: true
```

加载器是否应改为合并语义，是另一个更大的决策，不在 v0.310.0 范围内。v0.310.0 不改变加载器、审计行为或默认策略取值。

## 非目标

- 不新增审计规则。
- 不改变规则行为。
- 不改变审计行为。
- 不改变 finding JSON 结构。
- 不引入 `severity` 字段；`level` 仍是公开的优先级字段。
- 不改变 parser 支持。
- 不新增 SDK/HTTP/MCP 的 config-status 表面。
- 不新增批量 `config effective` 命令（仅支持单规则状态）。
- 该命令不运行审计。
- 该命令不解析 SQL。
- 该命令不连接数据库。

## 规则目录统计

规则目录与 v0.300.0 一致。`config status` 是检查命令，不是规则变更。

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

## 未变更指标

- SQL 语料库：**582/582**，**100.0%**，**245 YAML** 文件。
- PostgreSQL ALTER TABLE 配置条目：**53**（未变更）。
- PostgreSQL 合并 DDL 普查：**285/274/6/0/5/0**（未变更）。
- DDL 覆盖范围目录：**400** 条记录（MySQL 61 / TiDB 54 / PostgreSQL 285 / parser_upgrade_candidate 18）（未变更）。
- Parser-error 总计：跨方言 **29** 个用例（未变更）。

## 决策记录

`docs/decisions/2026-06-14-v0.310.0-rule-config-status.md`
