# DeltaScope v0.200.0 发行说明

## 概要 — MySQL/TiDB DDL 静默规范化全覆盖里程碑

v0.200.0 通过 28 条 DDL notice 规则，将所有此前为 normalized_silent 的 MySQL/TiDB DDL 形态提升为 finding-covered。本版本**不**新增 parser 支持、不改变 PostgreSQL 审核行为、不声称 full DDL coverage。

## 变更内容

- 28 条新 DDL notice 规则，覆盖 MySQL/TiDB 生命周期 DDL 事件（CREATE/DROP/ALTER DATABASE、CREATE/DROP/ALTER TABLE、TRUNCATE、RENAME TABLE）、MySQL/TiDB ALTER TABLE 动作通知（ADD/DROP/MODIFY COLUMN、CHANGE COLUMN、ADD/DROP INDEX、RENAME INDEX/COLUMN、ADD/DROP FOREIGN KEY、ADD/DROP/SET DEFAULT、ORDER BY、CONVERT/MODIFY CHARSET）、TiDB 专属规则（CREATE/ALTER/DROP PLACEMENT POLICY、CREATE/ALTER/DROP SEQUENCE）以及 MySQL 专属 DROP RESOURCE GROUP。
- MySQL normalized_silent 从 25 降至 **0**；finding_covered 从 21 升至 **46**。
- TiDB normalized_silent 从 27 降至 **0**；finding_covered 从 18 升至 **45**。
- SQL corpus 扩展至 **582/582** 目标、**100.0%**、**245 YAML** fixture 文件。
- 决策记录：`docs/decisions/2026-05-27-v0.200.0-mysql-tidb-ddl-normalized-silent-coverage.md`。

## DDL 覆盖范围普查

### MySQL

| 状态 | 变更前 | 变更后 |
|------|-------:|------:|
| 总数 | 61 | 61 |
| Finding 覆盖 | 21 | **46** |
| 静默规范化 | 25 | **0** |
| 不支持边界 | 0 | 0 |
| Parser 错误 | 15 | 15 |
| 未分类 | 0 | 0 |

### TiDB

| 状态 | 变更前 | 变更后 |
|------|-------:|------:|
| 总数 | 54 | 54 |
| Finding 覆盖 | 18 | **45** |
| 静默规范化 | 27 | **0** |
| 不支持边界 | 0 | 0 |
| Parser 错误 | 9 | 9 |
| 未分类 | 0 | 0 |

### PostgreSQL（不变）

| 状态 | 数量 |
|------|-----:|
| 总数 | 285 |
| Finding 覆盖 | 274 |
| 静默规范化 | 6 |
| 不支持边界 | 0 |
| Parser 错误 | 5 |
| 未分类 | 0 |

PostgreSQL `285` 是 consolidated tracked-case total，包含各源 census 列表之间的重叠。它不是唯一 SQL grammar denominator。

### 剩余缺口

- MySQL parser 缺口（15 种形态）：triggers、events、functions、ALTER VIEW、ALTER PROCEDURE、tablespace、CREATE/ALTER RESOURCE GROUP。
- TiDB parser 缺口（9 种形态）：triggers、events、functions、ALTER VIEW、ALTER TABLE TTL、ALTER TABLE LOCALITY。

## 不变指标

- PostgreSQL ALTER TABLE 规则数：**32**（不变）。
- PostgreSQL consolidated DDL census：**285/274/6/0/5/0**（不变）。

## 非目标

- 不新增 parser 支持。
- 不声称 full MySQL DDL support。
- 不声称 full TiDB DDL support。
- 不声称 full PostgreSQL DDL support。
- 不声称 dialect parity。
- 不声称 runtime/catalog validation。
- 不声称修复 parser-error。
