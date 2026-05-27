# DeltaScope v0.210.0 发行说明

## 概要 — 跨方言 Parser-Error 可行性普查

v0.210.0 将 MySQL（15）、TiDB（9）、PostgreSQL（5）共计 29 个 parser-error 案例分类至可行性桶，并新增 `make ddl-parser-error-feasibility-report` 报告门。本版本**不**新增 parser 支持、不新增 SQL 审核规则、不实现 fallback parser、不降低 parser_error 数量。

## Parser-Error 可行性桶

| 桶 | MySQL | TiDB | PostgreSQL | 合计 |
|----|-------|------|------------|------|
| `parser_upgrade_candidate` | 5 | 0 | 5 | 10 |
| `bounded_fallback_candidate` | 1 | 3 | 0 | 4 |
| `product_unsupported_or_inapplicable` | 0 | 6 | 0 | 6 |
| `unsafe_fallback_defer` | 9 | 0 | 0 | 9 |
| `needs_research` | 0 | 0 | 0 | 0 |
| **合计** | **15** | **9** | **5** | **29** |

### MySQL Parser-Error 案例（15）

| 家族 | 形态 | 桶 |
|------|------|----|
| view | ALTER VIEW | unsafe_fallback_defer |
| routine | ALTER PROCEDURE | unsafe_fallback_defer |
| routine | CREATE FUNCTION | parser_upgrade_candidate |
| routine | ALTER FUNCTION | unsafe_fallback_defer |
| routine | DROP FUNCTION | unsafe_fallback_defer |
| trigger | CREATE TRIGGER | unsafe_fallback_defer |
| trigger | DROP TRIGGER | unsafe_fallback_defer |
| event | CREATE EVENT | unsafe_fallback_defer |
| event | ALTER EVENT | unsafe_fallback_defer |
| event | DROP EVENT | unsafe_fallback_defer |
| tablespace | CREATE TABLESPACE | parser_upgrade_candidate |
| tablespace | ALTER TABLESPACE | parser_upgrade_candidate |
| tablespace | DROP TABLESPACE | parser_upgrade_candidate |
| resource_group | CREATE RESOURCE GROUP | parser_upgrade_candidate |
| resource_group | ALTER RESOURCE GROUP | bounded_fallback_candidate |

### TiDB Parser-Error 案例（9）

| 家族 | 形态 | 桶 |
|------|------|----|
| view | ALTER VIEW | bounded_fallback_candidate |
| table_option | ALTER TABLE TTL | bounded_fallback_candidate |
| table_option | ALTER TABLE LOCALITY | bounded_fallback_candidate |
| trigger | CREATE TRIGGER | product_unsupported_or_inapplicable |
| trigger | DROP TRIGGER | product_unsupported_or_inapplicable |
| routine | CREATE FUNCTION | product_unsupported_or_inapplicable |
| routine | DROP FUNCTION | product_unsupported_or_inapplicable |
| event | CREATE EVENT | product_unsupported_or_inapplicable |
| event | DROP EVENT | product_unsupported_or_inapplicable |

### PostgreSQL Parser-Error 案例（5）

| 家族 | 形态 | 桶 |
|------|------|----|
| subscription | DROP SUBSCRIPTION WITH drop_slot | parser_upgrade_candidate |
| constraints | NOT NULL NOT VALID | parser_upgrade_candidate |
| constraints | ALTER CONSTRAINT NOT ENFORCED | parser_upgrade_candidate |
| constraints | ALTER CONSTRAINT INHERIT | parser_upgrade_candidate |
| constraints | ALTER CONSTRAINT NO INHERIT | parser_upgrade_candidate |

## DDL 覆盖范围普查（不变）

| 方言 | 总数 | Finding | 静默 | 不支持 | Parser 错误 |
|------|-----:|--------:|-----:|:------:|:----------:|
| MySQL | 61 | 46 | 0 | 0 | 15 |
| TiDB | 54 | 45 | 0 | 0 | 9 |
| PostgreSQL（consolidated tracked-case） | 285 | 274 | 6 | 0 | 5 |

## 不变指标

- SQL corpus：**582/582**，**100.0%**，**245 YAML** fixture 文件。
- PostgreSQL ALTER TABLE 规则数：**32**（不变）。
- PostgreSQL consolidated DDL census：**285/274/6/0/5/0**（不变）。

## 报告门

- `make ddl-parser-error-feasibility-report` — 打印 MySQL、TiDB、PostgreSQL 分类 parser-error 可行性普查。

## 非目标

- 不新增 parser 支持。
- 不新增 SQL 审核规则。
- 不实现 fallback parser。
- 不降低 parser_error 数量。
- 不声称 full MySQL/TiDB/PostgreSQL DDL support。
- 不声称 dialect parity。
- 不声称 runtime/catalog validation。

## 决策记录

`docs/decisions/2026-05-27-v0.210.0-cross-dialect-parser-error-feasibility-census.md`
