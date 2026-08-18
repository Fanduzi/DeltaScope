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

## 覆盖目录查询（v0.280.0）

`deltascope ddl-coverage` CLI 命令（`v0.280.0` 起提供）查询生成的覆盖目录。当前二进制会把该目录编译进命令，因此可在空工作目录运行，不需要进程旁边存在 `docs/reference/ddl-coverage-catalog.json`。这是目录查询工具——它不执行审计、不解析 SQL、不调用审计服务。

### 概要

```bash
deltascope ddl-coverage [flags]
```

### 标志

| 标志 | 默认值 | 描述 |
|------|--------|------|
| `--dialect` | （无） | 按方言过滤：`mysql`、`tidb`、`postgresql` |
| `--classification` | （无） | 按分类过滤：`finding_covered`、`normalized_silent`、`unsupported_boundary`、`parser_error`、`unclassified` |
| `--guidance-code` | （无） | 按 guidance code 过滤：`parser_upgrade_candidate` |
| `--family` | （无） | 对目录 family 字段做大小写不敏感子串匹配 |
| `--form` | （无） | 对目录 form 字段做大小写不敏感子串匹配 |
| `--search` | （无） | 对 family、form、notes、guidance code 和 rule IDs 做大小写不敏感子串匹配 |
| `--format` | `text` | 输出格式：`text` 或 `json` |
| `--limit` | `0` | 限制返回条数；`0` 表示不限制 |

所有过滤条件均为可选。多个过滤条件同时提供时，以 AND 方式组合。

### 输出格式

#### 文本输出（默认）

```text
DIALECT  CLASSIFICATION  FAMILY          FORM                         GUIDANCE
mysql    parser_error    view_lifecycle  ALTER VIEW                   parser_upgrade_candidate
mysql    parser_error    view_lifecycle  CREATE VIEW                  parser_upgrade_candidate

2 entries
```

文本输出面向人工审查，稳定性足以满足日常使用，但 JSON 才是机器可读的稳定契约。

#### JSON 输出

```json
{
  "version": "v0.280.0",
  "summary": {
    "total": 2,
    "returned": 2,
    "filters": {
      "dialect": "mysql",
      "classification": "parser_error"
    }
  },
  "entries": [
    {
      "dialect": "mysql",
      "family": "view_lifecycle",
      "form": "ALTER VIEW",
      "classification": "parser_error",
      "finding_rule_ids": null,
      "guidance_code": "parser_upgrade_candidate",
      "evidence_ref": "...",
      "notes": ""
    }
  ]
}
```

JSON 输出包含 `version`、`summary`（含 `total`、`returned` 和当前 `filters`）以及 `entries`。这是面向自动化的稳定机器可读契约。

### 示例

MySQL parser-upgrade 候选：

```bash
deltascope ddl-coverage --dialect mysql --classification parser_error --guidance-code parser_upgrade_candidate
```

PostgreSQL DROP SUBSCRIPTION（JSON 格式）：

```bash
deltascope ddl-coverage --dialect postgresql --search "drop subscription" --format json
```

所有 TiDB 条目（JSON 格式）：

```bash
deltascope ddl-coverage --dialect tidb --format json
```

空查询——无目录匹配时返回成功，`entries` 为空数组：

```bash
deltascope ddl-coverage --search definitely-not-present --format json
```

### 重要说明

- 查询结果反映 DeltaScope 已验证的目录条目，不是数据库厂商语法的完整覆盖。
- 空结果表示搜索未匹配任何目录条目。它不表示数据库不支持该形态，也不是失败。
- 未出现在目录中表示 DeltaScope 尚未验证该形态——不代表数据库不支持。
- 此命令不审计 SQL、不增加 parser 支持、不引入 fallback parser 行为、不新增 SQL 审计规则，也不声称完整 DDL 支持或方言对等。

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
