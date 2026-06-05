# DDL 覆盖目录

DDL 覆盖目录回答一个核心问题：对于给定的 DDL 形态，DeltaScope 是审计它、静默标准化它、明确标记为不支持边界，还是无法解析它？

机器可读 JSON 版本位于 [`ddl-coverage-catalog.json`](ddl-coverage-catalog.json)。

---

## 这是什么

这是 DeltaScope 的**已验证代表性 DDL 覆盖目录**。它列出了 DeltaScope census 测试已分类的每一个 DDL 形态，以及分类结果和（适用的）触发规则 ID。

- 从 DeltaScope 自身的 census 测试数据生成。
- 覆盖 MySQL、TiDB 和 PostgreSQL 方言。
- 每条记录包含分类、方言、形态标签和关联的规则 ID。

## 这不是什么

本目录有明确边界。它**不是**：

- MySQL、TiDB 或 PostgreSQL 的官方 DDL 语法全集。许多数据库厂商语法的形态未收录于此。
- 完整 DDL 支持声明。DeltaScope 不审计数据库厂商接受的每一条 DDL 语句。
- 方言对等声明。MySQL、TiDB 和 PostgreSQL 的覆盖计数各不相同。
- 对所有数据库厂商语法已覆盖的保证。
- 新增的 parser、fallback parser 或新 SQL 审计规则。本目录仅描述已有行为。

---

## 分类说明

| 分类 | 含义 |
|---|---|
| `finding_covered` | DeltaScope 能解析并抽取该形态，当前规则可能产生审计发现。 |
| `normalized_silent` | DeltaScope 能解析并抽取该形态，但当前默认策略不产生发现。这是已知的静默路径，不是未追踪的缺口。 |
| `unsupported_boundary` | DeltaScope 明确识别为产品边界，返回不支持诊断而非假装审计。 |
| `parser_error` | 所选方言 parser 无法解析该形态。DeltaScope 不审计它。 |
| `unclassified` | 预留给目录生成失败的情况。release gate 要求此计数为零。 |

---

## 摘要

| 方言 | 总计 | finding_covered | normalized_silent | unsupported_boundary | parser_error | unclassified |
|------|-----:|----------------:|------------------:|--------------------:|-------------:|-------------:|
| MySQL | 61 | 46 | 0 | 0 | 15 | 0 |
| TiDB | 54 | 45 | 0 | 0 | 9 | 0 |
| PostgreSQL | 285 | 274 | 6 | 0 | 5 | 0 |
| PG ALTER TABLE residual | 66 | 60 | 2 | 0 | 4 | 0 |

---

## 用户示例

### MySQL `CREATE TABLE` — `finding_covered`

```bash
deltascope audit --dialect mysql --sql "CREATE TABLE t (id INT PRIMARY KEY)"
```

分类：`finding_covered`。可能触发的规则包括 `ddl.table.primary_key.require`、`ddl.column.not_null.require` 等，具体取决于表定义。

### MySQL `ALTER VIEW` — `parser_error`（含 guidance）

```bash
deltascope audit --dialect mysql --sql "ALTER VIEW v AS SELECT 1"
```

分类：`parser_error`。MySQL 方言 parser 无法解析 `ALTER VIEW`。此记录包含 `guidance_code=parser_upgrade_candidate` 和指向 parser-upgrade 候选证据的 `evidence_ref` 链接。

### TiDB `ALTER TABLE TTL` — `parser_error`

```bash
deltascope audit --dialect tidb --sql "ALTER TABLE t TTL = INTERVAL 7 DAY"
```

分类：`parser_error`。TiDB 方言 parser 无法解析 TTL 子句。

### PostgreSQL `ALTER TABLE ... VALIDATE CONSTRAINT` — `finding_covered`

```bash
deltascope audit --dialect postgresql --sql "ALTER TABLE t VALIDATE CONSTRAINT ck"
```

分类：`finding_covered`。规则：`ddl.pg.alter.validate_constraint.advisory`。

### PostgreSQL `DROP SUBSCRIPTION ... WITH` — `parser_error`（含 guidance）

```bash
deltascope audit --dialect postgresql --sql "DROP SUBSCRIPTION sub WITH (DROP SLOT)"
```

分类：`parser_error`。PostgreSQL 方言 parser 无法解析此形态。此记录包含 `guidance_code=parser_upgrade_candidate` 和 `evidence_ref`。

---

## Guidance 元数据

部分 `parser_error` 记录包含额外字段，帮助解释 DeltaScope 为何未审计该语句。

### `guidance_code`

对 parser-error 用例进行分类的标签。当前值为 `parser_upgrade_candidate`，表示该形态在 parser 或库升级后可能变为可解析。

`parser_upgrade_candidate` 是文档化分类。它不是当前的 parser 支持、不是 fallback parser、不是新 SQL 审计规则。DeltaScope 不从解析失败文本推断 findings。

### `evidence_ref`

指向该分类公开文档的 URL。对于 parser-upgrade 候选，链接到 v0.250.0 证据章节：

[`parser-upgrade-candidate-evidence-v02500`](cli.zh-CN.md#parser-upgrade-candidate-evidence-v02500)

---

## 如何在本地验证

运行以下命令验证目录完整性和 census 一致性：

```bash
# 验证目录 JSON 与 census 基线一致
make ddl-coverage-catalog-test

# 查看 DDL census 报告
make ddl-census-report

# 查看 SQL corpus 报告
make sql-corpus-report
```

当 census 数据变更时，重新生成目录 JSON：

```bash
UPDATE_DDL_COVERAGE_CATALOG=1 go test ./internal/application/audit -tags postgresql -run TestDDLCoverageCatalog
```
