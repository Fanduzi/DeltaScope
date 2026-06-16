# DeltaScope v0.320.0 发行说明

## 概要 — 配置 lint 替换告警

v0.320.0 让 `deltascope config lint` 在规则配置看起来会静默关闭或掏空某条规则时给出告警。告警描述的是规则级替换语义，这套语义本就决定了审计如何应用策略，lint 现在把它在审计运行之前就指出来。默认情况下 lint 打印告警并仍然以 0 退出。新增的 `--strict` 标志打印完全相同的文本，但以 2 退出，供希望对任意告警失败的 CI 步骤使用。

本版本**不**改变审计行为、配置加载器（`LoadPolicy`）、默认策略、任何规则、parser 支持或任何输出结构。不存在 `severity` 字段，DeltaScope 继续使用 `level`，而非 `severity`。

## `config lint` 命令

```bash
deltascope config lint --file <path> [--strict]
```

示例：

```bash
deltascope config lint --file ./deltascope.yaml
deltascope config lint --file ./deltascope.yaml --strict
```

干净配置的输出：

```text
Config OK
```

存在替换隐患的配置：

```text
Config OK with warnings

Warnings:
- rule "dml.where.require" is mentioned without "enabled"; the rule policy is replaced, not partially merged, so omitted "enabled" becomes false and the rule is OFF
- rule "dml.where.require" is mentioned without "params"; the rule policy is replaced, not partially merged, so omitted "params" become empty, removing the default params
```

### 典型的坑

想放宽规则级别的用户会写成这样：

```yaml
rules:
  dml.where.require:
    level: warning
```

这看起来像只改了一个字段，其实不是。在 YAML 中提及一条规则会替换该规则的整条策略，被省略的字段变为零值：

| 字段 | 省略时的有效值 |
|---|---|
| `enabled` | `false` |
| `level` | `""`（空） |
| `params` | `nil`（空） |

所以上面的配置会**关闭该规则**，因为 `enabled` 被省略、从而被替换为 `false`。lint 现在指出的正是这种情况。若只想改级别同时保持规则开启，需写全所有字段，让替换保留其余字段：

```yaml
rules:
  dml.where.require:
    enabled: true
    level: warning
    params:
      required: true
```

### 面向 CI 的 `--strict`

默认 lint 即便有告警也以 0 退出，因为告警不是错误，而且只提及规则、不写 `enabled` 的配置可能是有意为之。`--strict` 保持完全相同的文本输出，但返回退出码 2，这样 CI 步骤可以在出现任意替换隐患时让构建失败：

```bash
deltascope config lint --file ./deltascope.yaml --strict
```

不加 `--strict` 时，错误仍是错误，仍会非零退出。告警仅以文本形式输出，没有单独的 JSON 或 SARIF 结构，也不引入 `severity` 字段。

### 用 `config status` 进一步确认

lint 告诉你某条规则可能被关闭，`config status` 负责确认。对上面的坑：

```bash
deltascope --config ./deltascope.yaml config status dml.where.require
```

会报告该规则为 `OFF`，展示有效 `level`，并列出你的配置相对默认值改动了哪些字段。完整的 `config status` 契约见 v0.310.0 发行说明。

## 文档清理

v0.320.0 同时整理了公开文档：`config lint`、`config status` 和 `rules explain` 的措辞更清晰，中英文保持一致；并说明 `docs/decisions/` 是维护者的决策记录，而非用户指南。这些只是文档变更，不涉及行为变化。

## 非目标

- 不改变审计行为。
- 不改变配置加载器（`LoadPolicy`）；部分规则配置仍不会合并到默认值之上。
- 不改变默认策略或规则。
- 不改变 parser 支持。
- 不新增审计规则。
- 不改变 finding JSON 结构。
- 不改变 SDK/HTTP/MCP 响应结构。
- 不改变 SARIF、GitHub Actions 或 GitLab CodeQuality 渲染器。
- 不引入 `severity` 字段；`level` 仍是公开的优先级字段。

## 规则目录统计

规则目录与 v0.310.0 一致。`config lint` 告警描述的是既有语义，不是规则变更。

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

`docs/decisions/2026-06-15-v0.320.0-config-lint-warnings-docs-cleanup.md`
