# DeltaScope v0.190.0 发行说明

## 概要 — 跨方言 DDL 覆盖范围普查整合

v0.190.0 是一个发布工程 / 覆盖范围清查里程碑，建立了跨 MySQL、TiDB、PostgreSQL 的统一 DDL 覆盖范围普查。本版本**不**新增 SQL 审计规则、不新增 parser 支持、不改变产品行为。

## 变更内容

- `make ddl-census-report` 现在打印 MySQL、TiDB、PostgreSQL 的 tracked DDL coverage census。这是 inventory/reporting gate —— 它展示每种方言有多少 tracked DDL 形态是 finding-covered、silently normalized、explicitly unsupported 或 parser-blocked。这不是 full SQL grammar coverage claim。
- MySQL/TiDB tracked DDL census 从 18 扩展到 61/54 种形态，各覆盖 10 个 DDL 族。
- PostgreSQL tracked DDL census 从 5 个独立源测试整合为单一 consolidated 测试，并包含机器可验证的算术不变式。
- 决策记录：`docs/decisions/2026-05-26-v0.190.0-cross-dialect-ddl-census-consolidation.md`。

## DDL 覆盖范围普查

| 方言 | 总数 | Finding 覆盖 | 静默规范化 | 不支持边界 | Parser 错误 | 未分类 |
|------|-----:|------------:|----------:|:---------:|:----------:|:-----:|
| MySQL | 61 | 21 | 25 | 0 | 15 | 0 |
| TiDB | 54 | 18 | 27 | 0 | 9 | 0 |
| PostgreSQL (consolidated tracked-case) | 285 | 274 | 6 | 0 | 5 | 0 |

PostgreSQL `285` 是 consolidated tracked-case total，包含各源 census 列表之间的重叠（representative、completion、deep、long-tail、ALTER TABLE residual）。它不是唯一 SQL grammar denominator。

### MySQL/TiDB 主要缺口

- MySQL normalized_silent 后续规则候选：RENAME TABLE、standalone CREATE INDEX、ALTER DATABASE、CREATE/DROP PROCEDURE、privilege/DCL。
- MySQL parser 缺口（15 种形态）：triggers、events、functions、ALTER VIEW、ALTER PROCEDURE、tablespace、CREATE/ALTER RESOURCE GROUP。
- TiDB normalized_silent 后续规则候选：RENAME TABLE、ALTER TABLE ADD/DROP INDEX、standalone CREATE INDEX、PLACEMENT POLICY、SEQUENCE、ALTER TABLE PLACEMENT POLICY、privilege/DCL。
- TiDB parser 缺口（9 种形态）：triggers、events、functions、ALTER VIEW、ALTER TABLE TTL、ALTER TABLE LOCALITY。

## 不变指标

v0.190.0 不改变 SQL parser、规则或产品行为：

- PostgreSQL ALTER TABLE 规则数：**32**（不变）。
- Residual census：**66/60/2/0/4/0**（不变）。
- SQL corpus：**535/535**、**100.0%**、**243 YAML 文件**（不变）。

## 非目标

- 不新增 SQL 审计规则。
- 不新增 parser 支持。
- 不声称 full MySQL DDL support。
- 不声称 full TiDB DDL support。
- 不声称 full PostgreSQL DDL support。
- 不声称 dialect parity。
- 不声称 runtime/catalog validation。
