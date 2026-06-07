# DeltaScope v0.270.0 发行说明

## 概要 — DDL Coverage Catalog

v0.270.0 引入 DDL 覆盖范围目录，将 DeltaScope 在 MySQL、TiDB 和 PostgreSQL 上的已验证 DDL 覆盖事实整合为一个机器可读的 JSON 制品和双语文档。本版本**不**新增 parser 支持、不改变 parser 行为、不新增 SQL 审计规则、不实现 fallback parser、不减少 `parser_error` 计数、不改变审计判定或 finding 语义、不改变任何 SQL 审核行为。

## DDL 覆盖范围目录

位于 `docs/reference/ddl-coverage-catalog.json` 的机器可读目录，列出 DeltaScope 已验证的每一条 DDL 形态：

- **400 条记录**，覆盖 MySQL（61）、TiDB（54）和 PostgreSQL（285）
- 每条记录包含方言、家族、形态标签、分类、引导元数据、规则 ID 和人工可读说明
- 从驱动现有覆盖测试的同一普查数据生成——非手工维护

### 分类词汇表

| 分类 | 含义 |
|---|---|
| `finding_covered` | 解析并在当前规则下产出一条或多条审计发现 |
| `normalized_silent` | 解析但当前默认策略不产出发现 |
| `unsupported_boundary` | 已识别的产品边界；返回不支持诊断 |
| `parser_error` | 所选方言 parser 无法解析该形态 |
| `unclassified` | 保留；发布门禁要求为零 |

## Parser-Upgrade Candidate 元数据

18 条记录携带 `guidance_code=parser_upgrade_candidate` 和 `evidence_ref` 文档链接。全部 10 个必需候选均已包含：

| 方言 | DDL 形态 | guidance_code |
|------|----------|---------------|
| MySQL | `ALTER VIEW` | `parser_upgrade_candidate` |
| MySQL | `ALTER PROCEDURE` | `parser_upgrade_candidate` |
| MySQL | `CREATE FUNCTION` | `parser_upgrade_candidate` |
| MySQL | `ALTER FUNCTION` | `parser_upgrade_candidate` |
| MySQL | `DROP FUNCTION` | `parser_upgrade_candidate` |
| PostgreSQL | `DROP SUBSCRIPTION WITH drop_slot` | `parser_upgrade_candidate` |
| PostgreSQL | `NOT NULL NOT VALID` | `parser_upgrade_candidate` |
| PostgreSQL | `ALTER CONSTRAINT NOT ENFORCED` | `parser_upgrade_candidate` |
| PostgreSQL | `ALTER CONSTRAINT INHERIT` | `parser_upgrade_candidate` |
| PostgreSQL | `ALTER CONSTRAINT NO INHERIT` | `parser_upgrade_candidate` |

## 漂移门禁

`make ddl-coverage-catalog-test` 验证目录 JSON 与普查测试保持一致。如果 checked-in JSON 已过时，门禁将失败。

## 用户文档

- `docs/reference/ddl-coverage.md` — 英文参考
- `docs/reference/ddl-coverage.zh-CN.md` — 中文参考（完全对等）

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
- PostgreSQL ALTER TABLE 配置条目: **53**（v0.170.0 记载 32；发售后审计发现计数已漂移，予以纠正）。
- PostgreSQL 合并 DDL 普查: **285/274/6/0/5/0**（未变）。
- Parser-error 合计: **29** 例跨所有方言（未变）。
- MySQL/TiDB DDL Notice 段: **27**（未变）。
- TiDB 专用子段: **7**（未变）。

## 决策记录

`docs/decisions/2026-06-05-v0.270.0-ddl-coverage-catalog.md`
