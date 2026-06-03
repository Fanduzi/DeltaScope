# DeltaScope v0.250.0 发行说明

## 概要 — Parser Upgrade Candidate Evidence Pack

v0.250.0 是一个 parser-upgrade 候选证据里程碑。它记录了 29 个剩余 `parser_error` 形态中哪些会从 parser/库升级中受益，哪些非候选边界不得跨越，以及面向用户的影响。本版本**不**新增 parser 支持、不改变 parser 行为、不新增 SQL 审计规则、不实现 fallback parser、不减少 `parser_error` 计数、不改变公开输出格式、不改变任何 SQL 审核行为。

## Parser-Upgrade 候选证据

MySQL、TiDB 和 PostgreSQL 全部 29 个剩余 `parser_error` 案例已分类至可行性桶：

| 分类 | MySQL | TiDB | PostgreSQL | 合计 |
|------|-------|------|------------|------|
| `parser_upgrade_candidate` | 5 | 0 | 5 | 10 |
| `bounded_fallback_candidate` | 1 | 3 | 0 | 4 |
| `product_unsupported_or_inapplicable` | 0 | 6 | 0 | 6 |
| `unsafe_fallback_defer` | 9 | 0 | 0 | 9 |
| `needs_research` | 0 | 0 | 0 | 0 |
| **合计** | **15** | **9** | **5** | **29** |

### Parser-Upgrade 候选形态 (10 个)

这些形态在上游 parser 库添加语法支持后将变为可解析：

**MySQL (5):**

| # | 家族 | 案例 |
|---|------|------|
| 1 | view | ALTER VIEW |
| 2 | routine | ALTER PROCEDURE |
| 3 | routine | CREATE FUNCTION |
| 4 | routine | ALTER FUNCTION |
| 5 | routine | DROP FUNCTION |

**PostgreSQL (5):**

| # | 家族 | 案例 |
|---|------|------|
| 1 | subscription | DROP SUBSCRIPTION ... WITH (drop_slot = true) |
| 2 | constraints | NOT NULL NOT VALID |
| 3 | constraints | ALTER CONSTRAINT NOT ENFORCED |
| 4 | constraints | ALTER CONSTRAINT INHERIT |
| 5 | constraints | ALTER CONSTRAINT NO INHERIT |

### 用户影响

- `parser_error` 语句不被审计。不会为不可解析的 SQL 生成 finding。
- 不从解析失败推断审计结论。
- 原始 SQL 文本和 parser `near ...` 错误片段不复制到面向用户的指引中。
- 用户应手动审查 `parser_error` 语句。
- 这些证据有助于未来的 parser 升级规划。

## 非目标

- 不新增 parser 支持。
- 不改变 parser 适配器。
- 不实现 fallback parser。
- 不新增 SQL 审计规则。
- 不改变策略默认值。
- 不减少 parser_error 计数。
- 不改变公开输出格式。
- 不声称完整 DDL 支持。
- 不声称方言对等。
- 不改变 SQL 审核行为。

## Parser-Error 计数（未变）

| 方言 | Parser Error |
|------|:------------:|
| MySQL | 15 |
| TiDB | 9 |
| PostgreSQL | 5 |
| **合计** | **29** |

## DDL 覆盖范围普查（未变）

| 方言 | Total | Finding | Silent | Unsupported | Parser Error |
|------|------:|--------:|-------:|:-----------:|:------------:|
| MySQL | 61 | 46 | 0 | 0 | 15 |
| TiDB | 54 | 45 | 0 | 0 | 9 |
| PostgreSQL (consolidated tracked-case) | 285 | 274 | 6 | 0 | 5 |

## 未变指标

- SQL 语料库: **582/582**, **100.0%**, **245 YAML** fixture 文件。
- PostgreSQL ALTER TABLE 规则计数: **32**（未变）。
- PostgreSQL 合并 DDL 普查: **285/274/6/0/5/0**（未变）。
- MySQL/TiDB DDL Notice 段: **27**（未变）。
- TiDB 专用子段: **7**（未变）。

## 决策记录

`docs/decisions/2026-06-02-v0.250.0-parser-upgrade-candidate-evidence-pack.md`
