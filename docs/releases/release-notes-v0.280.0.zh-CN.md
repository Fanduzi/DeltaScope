# DeltaScope v0.280.0 发行说明

## 概要 — DDL Coverage Catalog Query

v0.280.0 新增 `deltascope ddl-coverage` CLI 命令，用于查询 v0.270.0 引入的 DDL 覆盖范围目录。用户可按方言、分类、引导代码、家族、形态或自由文本搜索进行筛选，并以人工可读表格或 JSON 格式输出结果。本版本**不**新增 parser 支持、不改变 parser 行为、不新增 SQL 审计规则、不实现 fallback parser、不减少 `parser_error` 计数、不改变审计判定或 finding 语义、不改变任何 SQL 审核行为。

## CLI 目录查询

新增 `deltascope ddl-coverage` 命令，提供交互式目录访问：

```bash
# 列出所有条目（默认表格格式，最多 50 行）
deltascope ddl-coverage

# 按方言筛选
deltascope ddl-coverage --dialect mysql

# 按分类筛选
deltascope ddl-coverage --classification finding_covered

# 按引导代码筛选
deltascope ddl-coverage --guidance-code parser_upgrade_candidate

# 在家族和形态标签中自由文本搜索
deltascope ddl-coverage --search "ALTER TABLE"

# JSON 输出用于脚本集成
deltascope ddl-coverage --format json --limit 0
```

### 筛选器

| 标志 | 说明 |
|------|------|
| `--dialect` | 按方言筛选（`mysql`、`tidb`、`postgresql`） |
| `--classification` | 按分类筛选（`finding_covered`、`normalized_silent`、`unsupported_boundary`、`parser_error`） |
| `--guidance-code` | 按引导代码筛选（如 `parser_upgrade_candidate`） |
| `--family` | 按 DDL 家族筛选（如 `ALTER TABLE`） |
| `--form` | 按形态标签筛选 |
| `--search` | 跨家族和形态的大小写不敏感子串搜索 |
| `--format` | 输出格式：`table`（默认）或 `json` |
| `--limit` | 最大显示行数（`0` = 不限） |

## 非目标

- 不新增 parser 支持。
- 不实现 fallback parser。
- 不新增 SQL 审计规则。
- 不减少 parser_error 计数。
- 不改变审计判定或 finding 语义。
- 不改变 SQL 审核行为。
- 不改变运行时行为。
- 不改变 npm/Homebrew 行为。

## DDL 覆盖范围普查（未变）

| 方言 | Total | Finding | Silent | Unsupported | Parser Error |
|------|------:|--------:|-------:|:-----------:|:------------:|
| MySQL | 61 | 46 | 0 | 0 | 15 |
| TiDB | 54 | 45 | 0 | 0 | 9 |
| PostgreSQL (consolidated tracked-case) | 285 | 274 | 6 | 0 | 5 |

### PostgreSQL ALTER TABLE Residual（未变）

`66/60/2/0/4/0`

## 未变指标

- SQL 语料库: **582/582**, **100.0%**, **245 YAML** fixture 文件。
- PostgreSQL ALTER TABLE 配置条目: **53**。
- PostgreSQL 合并 DDL 普查: **285/274/6/0/5/0**（未变）。
- DDL 覆盖范围目录: **400** 条记录（MySQL 61 / TiDB 54 / PostgreSQL 285 / parser_upgrade_candidate 18）（未变）。
- Parser-error 合计: **29** 例跨所有方言（未变）。
- MySQL/TiDB DDL Notice 段: **27**（未变）。
- TiDB 专用子段: **7**（未变）。

## 决策记录

`docs/decisions/2026-06-07-v0.280.0-ddl-coverage-catalog-query.md`
