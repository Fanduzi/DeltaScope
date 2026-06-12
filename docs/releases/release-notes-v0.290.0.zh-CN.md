# DeltaScope v0.290.0 发行说明

## 概要 — 规则可发现性与解释

v0.290.0 新增 CLI 命令，用于浏览和解释 DeltaScope 内置规则目录。`deltascope rules list` 按方言、级别、类型、类别和自由文本搜索筛选规则列表。`deltascope rules explain <rule_id>` 展示单条规则的完整元数据。两条命令均支持 JSON 输出用于自动化。本版本**不**新增审计规则、不改变规则评估行为、不改变 finding JSON 结构、不引入 `severity` 字段、不将 `level` 重命名为 `severity`、不改变 parser 支持、不新增 SDK/HTTP/MCP 规则发现接口。

## CLI 规则发现

两条新命令让用户无需运行审计即可浏览规则目录：

```bash
# 列出所有规则
deltascope rules list

# 按方言筛选
deltascope rules list --dialect postgresql

# 按级别筛选
deltascope rules list --level blocker

# 按类型筛选
deltascope rules list --kind dml

# 自由文本搜索
deltascope rules list --search "primary key"

# JSON 输出
deltascope rules list --format json

# 查看单条规则详情
deltascope rules explain dml.where.require

# 以 JSON 格式查看规则详情
deltascope rules explain ddl.table.comment.require --format json
```

### 筛选条件（`rules list`）

| 标志 | 说明 |
|------|------|
| `--dialect` | 按方言筛选（`mysql`、`tidb`、`postgresql`、`common`） |
| `--level` | 按默认级别筛选（`blocker`、`warning`、`notice`） |
| `--kind` | 按语句类型筛选（`ddl`、`dml`） |
| `--category` | 不区分大小写的类别/家族筛选 |
| `--search` | 跨 rule ID、摘要、风险、建议、配置键和标签的不区分大小写搜索 |
| `--format` | 输出格式：`text`（默认）或 `json` |
| `--limit` | 最大显示行数（`0` = 无限制） |

### `level` 词汇

规则使用 `level`（而非 `severity`）作为公开的 finding 字段。取值为 `blocker`、`warning` 和 `notice`。该词汇与之前版本一致，同时匹配 finding JSON 输出和 `deltascope.example.yaml` 中的规则配置。公开输出中不存在 `severity` 字段。

## 规则目录统计

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
- 不改变 finding JSON 结构。
- 不引入 `severity` 字段。
- 不改变 parser 支持。
- 本版本不新增 SDK/HTTP/MCP 规则发现接口。
- 不改变 v0.280.0 DDL 覆盖范围查询行为。
- 不改变配置文件结构。
- 不改变默认规则级别。

## DDL 覆盖范围普查（未变更）

| 方言 | 总计 | Finding | Silent | Unsupported | Parser Error |
|------|-----:|--------:|-------:|:-----------:|:------------:|
| MySQL | 61 | 46 | 0 | 0 | 15 |
| TiDB | 54 | 45 | 0 | 0 | 9 |
| PostgreSQL（合并追踪用例） | 285 | 274 | 6 | 0 | 5 |

### PostgreSQL ALTER TABLE 残差（未变更）

`66/60/2/0/4/0`

## 未变更指标

- SQL 语料库：**582/582**，**100.0%**，**245 YAML** 文件。
- PostgreSQL ALTER TABLE 配置条目：**53**。
- PostgreSQL 合并 DDL 普查：**285/274/6/0/5/0**（未变更）。
- DDL 覆盖范围目录：**400** 条记录（MySQL 61 / TiDB 54 / PostgreSQL 285 / parser_upgrade_candidate 18）（未变更）。
- Parser-error 总计：跨方言 **29** 个用例（未变更）。

## 决策记录

`docs/decisions/2026-06-11-v0.290.0-rule-discoverability.md`
