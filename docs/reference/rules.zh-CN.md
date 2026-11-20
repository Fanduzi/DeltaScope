# 规则参考手册

DeltaScope 将所有审计逻辑以可发现的稳定规则 ID 形式提供，而非隐藏的启发式逻辑。每条规则均可通过策略配置进行检查、过滤、启用、禁用和参数化。

## 规则命名约定

规则 ID 遵循结构化的点分隔格式：

```
<kind>.<area>.<check>
```

- **`ddl`** — DDL 语句（CREATE TABLE、ALTER TABLE、DROP TABLE、TRUNCATE、CREATE VIEW 等）
- **`dml`** — DML 语句（SELECT、INSERT、UPDATE、DELETE、REPLACE 等）

### 严重级别

| 级别 | 含义 |
|------|------|
| `blocker` | 必须在应用 SQL 之前修复。表示高风险或违反策略的变更。 |
| `warning` | 应在应用之前进行检查。表示潜在风险或不规范的写法。 |
| `notice` | 仅供参考。无需立即处理。 |

### 审计结论映射

审计批次的整体结论由所有语句中最严重的发现决定：

| 批次中的发现 | 结论 |
|-------------|------|
| 存在任意 `blocker` 发现 | `reject` |
| 无 blocker；但存在至少一个 `warning` | `review` |
| 无 blocker 且无 `warning`（包括仅有 `notice` 或完全无发现） | `pass` |

`--fail-on` 标志控制哪个结论阈值会导致 CLI 以退出码 `1` 退出。详情参见 [CLI 参考手册](cli.zh-CN.md)。

---

## 发现规则

DeltaScope 提供两个只读的规则元数据查询命令。这些命令查询内置 **Rule Catalog**——不执行审计、不解析 SQL、不调用审计服务。

Catalog、Default Policy、Loaded 和 suppressions 是不同的计数：

- `deltascope rules list` 报告 Rule Catalog，包含默认禁用的 `dml.impact.*`。
- Default Policy 是无调用方配置时的启用规则集。它不启用 `dml.impact.*`。
- 审计 `rule_summary.loaded` 是该次审计实际注册的语句规则集。不要把 Catalog 计数当成 Loaded 计数。
- 三条 `ddl.constraint.foreign_key.name.*` 仍在 Catalog 和 Default Policy 中。当 `ddl.table.foreign_key.forbid` 启用时它们不会 Loaded；`config status` 以原因 `fk_forbid` 报告，因此它们是被抑制，不是缺失。

DeltaScope 使用已有的 `level` 字段（取值为 `blocker`、`warning`、`notice`）来表示严重程度。该字段在发现 JSON、配置 YAML 和目录输出中均命名为 `level`，不存在单独的 `severity` 字段。

### deltascope rules list

从内置目录列出规则，支持可选过滤：

```bash
# 所有规则
deltascope rules list

# 按 level 过滤
deltascope rules list --level blocker
deltascope rules list --level warning
deltascope rules list --level notice

# 按方言和 level 过滤，JSON 输出
deltascope rules list --dialect postgresql --level warning --format json

# 按 kind 过滤
deltascope rules list --kind ddl
deltascope rules list --kind dml

# 关键词搜索
deltascope rules list --search drop_column

# 按 category 过滤
deltascope rules list --category alter_table

# 限制结果数量
deltascope rules list --level blocker --limit 5
```

过滤条件：

| 标志 | 取值 | 描述 |
|------|------|------|
| `--dialect` | `mysql`、`tidb`、`postgresql`、`common` | 按方言范围过滤 |
| `--level` | `blocker`、`warning`、`notice` | 按默认 level 过滤 |
| `--kind` | `ddl`、`dml` | 按 SQL 语句类型过滤 |
| `--category` | （自由文本） | 按类别/分组进行大小写不敏感的子串匹配 |
| `--search` | （自由文本） | 在规则 ID、摘要、标签、配置键中进行大小写不敏感的搜索 |
| `--format` | `text`、`json` | 输出格式（默认：`text`） |
| `--limit` | 整数 | 限制结果数量；`0` 表示不限制 |

所有过滤条件均为可选。多个过滤条件以 AND 方式组合。空结果返回成功（零条规则）。

文本输出示例：

```text
RULE ID                                        LEVEL    DIALECT  KIND  CATEGORY
---------------------------------------------  -------  -------  ----  -----------
ddl.alter.drop_column.forbid                   warning  common   ddl   alter_table
ddl.alter.drop_column.notice                   notice   common   ddl   alter_table
ddl.pg.alter.drop_column.advisory              warning  postgresql  ddl   alter_table
3 rules
```

JSON 输出示例：

```bash
deltascope rules list --dialect postgresql --level warning --format json
```

```json
{
  "version": "v0.290.0",
  "summary": {
    "total": 62,
    "returned": 62,
    "filters": {
      "dialect": "postgresql",
      "level": "warning"
    }
  },
  "rules": [
    {
      "rule_id": "ddl.pg.alter.add_check.not_valid.require",
      "level": "warning",
      "dialect": "postgresql",
      "kind": "ddl",
      "category": "alter_table",
      "summary": "Require DDL pg alter add check not valid require",
      "enabled": true,
      "tags": ["ddl", "postgresql", "alter_table", "require"]
    }
  ]
}
```

无效的枚举值会产生明确的校验错误并以退出码 2 退出：

```text
invalid level "severe": must be one of blocker, warning, notice
```

### deltascope rules explain

通过精确的规则 ID 显示单条规则的详细信息：

```bash
deltascope rules explain dml.where.require
deltascope rules explain dml.where.require --format json
```

`rules explain` 不运行审计，不解析 SQL。它返回内置目录中的静态规则元数据。

文本输出示例：

```text
Rule ID:    dml.where.require
Level:      blocker
Enabled:    true
Dialects:   common
Kind:       dml
Category:   dml_safety
Config Key: dml.where.require

Summary:
  Require DML where require

Why:
  The statement is missing a clause, option, or object that the shipped policy requires.

Risk:
  Ignoring this rule can allow high-impact data changes to proceed with less safety review.

Suggestion:
  Add the required clause, option, or object explicitly so the rule no longer has to infer intent.

Tags: dml, common, dml_safety, require
Trigger Example:
  DELETE FROM users;
Valid Example:
  DELETE FROM users WHERE id = 1;

Default Params:
  required: true

Default policy:
  rules:
    dml.where.require:
      enabled: true
      level: blocker
      params:
        required: true

Safe override example:
  rules:
    dml.where.require:
      enabled: true
      level: warning
      params:
        required: true

Inspect effective rule status:
  deltascope config status dml.where.require --config deltascope.yaml
```

JSON 输出示例：

```bash
deltascope rules explain dml.where.require --format json
```

```json
{
  "version": "v0.290.0",
  "rule": {
    "rule_id": "dml.where.require",
    "level": "blocker",
    "enabled": true,
    "dialects": ["common"],
    "kind": "dml",
    "category": "dml_safety",
    "summary": "Require DML where require",
    "why": "The statement is missing a clause, option, or object that the shipped policy requires.",
    "risk": "Ignoring this rule can allow high-impact data changes to proceed with less safety review.",
    "suggestion": "Add the required clause, option, or object explicitly so the rule no longer has to infer intent.",
    "config_key": "dml.where.require",
    "tags": ["dml", "common", "dml_safety", "require"],
    "description": "Require DML where require. Default level is blocker, enabled=true, scope=dml, and the shipped policy treats it as a offline-safe rule.",
    "statement_kinds": ["dml"],
    "default_params": { "required": true },
    "metadata_aware": false,
    "trigger_example": "DELETE FROM users;",
    "valid_example": "DELETE FROM users WHERE id = 1;",
    "config_example": "rules:\n  dml.where.require:\n    enabled: true\n    level: blocker\n    params:\n      required: true",
    "remediation": "Add the required clause, option, or object explicitly so the rule no longer has to infer intent.",
    "config_hints": [
      "rules.dml.where.require.enabled",
      "rules.dml.where.require.level",
      "rules.dml.where.require.params.required"
    ]
  }
}
```

未知的规则 ID 会产生明确的错误并以退出码 2 退出：

```text
rule "nonexistent_rule" not found
```

### 配置键关系

对于大多数规则，`config_key` 与 `rule_id` 相同。要配置一条规则，在策略 YAML 的 `rules` 节下使用其 `config_key`：

```yaml
rules:
  dml.where.require:
    enabled: true
    level: blocker
    params:
      required: true
```

### 非目标

规则发现命令不：

- 审计 SQL 语句
- 添加新的审计规则或更改规则行为
- 更改发现 JSON 的结构
- 引入 `severity` 字段
- 声称返回所有可能的数据库风险
- 提供 SDK、HTTP 或 MCP 接口

---

## DDL：建表规则

### 表级规则（23 条）

这些规则对整个 `CREATE TABLE` 语句的属性进行评估。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.table.comment.require` | 表必须拥有非空的 COMMENT | warning | 否 |
| `ddl.table.comment.max_length` | 表的 COMMENT 不得超过 Unicode 码点数限制 | warning | 否 |
| `ddl.table.name.max_length` | 表名长度限制 | blocker | 否 |
| `ddl.table.name.pattern.require` | 表名必须符合命名规则（默认：字母数字加下划线） | blocker | 否 |
| `ddl.table.name.keyword.forbid` | 表名不得使用 SQL 保留关键字 | blocker | 否 |
| `ddl.table.engine.allowlist` | 存储引擎必须在允许列表中（默认：InnoDB） | blocker | 否 |
| `ddl.table.charset.allowlist` | 表的默认字符集必须在允许列表中 | blocker | 否 |
| `ddl.table.collation.allowlist` | 表排序规则必须在允许列表中（默认禁用；默认列表：utf8mb4_bin） | blocker | 否 |
| `ddl.table.row_format.allowlist` | ROW_FORMAT 必须在允许列表中（默认：DYNAMIC） | blocker | 否 |
| `ddl.table.auto_increment.init_value.require` | AUTO_INCREMENT 初始值必须符合要求 | blocker | 否 |
| `ddl.table.columns.min_count` | 表至少需要 N 列 | blocker | 否 |
| `ddl.table.primary_key.require` | 表必须拥有 PRIMARY KEY | blocker | 否 |
| `ddl.table.primary_key.columns.max_count` | PRIMARY KEY 列数限制 | warning | 否 |
| `ddl.table.primary_key.bigint.require` | PRIMARY KEY 列必须为 BIGINT 类型 | blocker | 否 |
| `ddl.table.primary_key.unsigned.require` | PRIMARY KEY 列必须为 UNSIGNED | blocker | 否 |
| `ddl.table.primary_key.auto_increment.require` | PRIMARY KEY 列必须为 AUTO_INCREMENT | blocker | 否 |
| `ddl.table.primary_key.not_null.require` | PRIMARY KEY 列必须为 NOT NULL | blocker | 否 |

**PostgreSQL 主键可用性（v0.37.0、v0.39.0）：** `ddl.table.primary_key.bigint.require` 和 `ddl.table.primary_key.columns.max_count` 现在适用于 PostgreSQL `CREATE TABLE` 语句（v0.37.0）和 `ALTER TABLE ... ADD CONSTRAINT ... PRIMARY KEY` 语句（v0.39.0）。`ddl.table.primary_key.not_null.require` 对 PostgreSQL 不产生稳定负例，因为 PK 列被有效视为 NOT NULL。

**默认策略方言隔离（v0.43.0）：** 从 v0.43.0 开始，设置 `--dialect postgresql` 时自动跳过 MySQL-family 规则，包括 `ddl.table.primary_key.unsigned.require`、`ddl.table.primary_key.auto_increment.require`、`ddl.table.primary_key.not_null.require`、`ddl.table.engine.allowlist`、`ddl.table.charset.allowlist`、`ddl.table.collation.allowlist`、`ddl.table.row_format.allowlist`、`ddl.table.auto_increment.init_value.require`、`ddl.table.partition.forbid`、`ddl.table.create_as.forbid`、`ddl.table.create_like.forbid`、`ddl.column.charset.allowlist`、`ddl.column.collation.allowlist`、`ddl.column.charset_collation.match.require`、`ddl.alter.change_column.forbid`、`ddl.alter.modify_column.forbid`，以及审计列检查中的 `ON UPDATE CURRENT_TIMESTAMP` 建议。反过来，MySQL/TiDB 审核排除所有 `ddl.pg.*` 和 PostgreSQL-only 方言门控规则。隔离在规则 `AppliesTo` 门控层实现。
| `ddl.table.audit_columns.require` | 表必须包含审计时间戳列 | warning | 否 |
| `ddl.table.foreign_key.forbid` | 禁止使用 FOREIGN KEY 约束 | blocker | 否 |
| `ddl.table.partition.forbid` | 禁止使用分区表 | blocker | 否 |
| `ddl.table.create_like.forbid` | 禁止 CREATE TABLE … LIKE | blocker | 否 |
| `ddl.table.create_as.forbid` | 禁止 CREATE TABLE … AS SELECT | blocker | 否 |
| `ddl.table.row_size.max_bytes.require` | 估算行大小不得超过 InnoDB 限制 | blocker | **是** |
| `ddl.table.denylist.forbid` | 禁止对拒绝列表中的 schema/表执行 DDL | blocker | **是** |

### 列级规则（16 条）

这些规则对 `CREATE TABLE` 语句中每个列定义进行评估。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.column.name.max_length` | 列名长度限制 | blocker | 否 |
| `ddl.column.name.pattern.require` | 列名必须符合命名规则 | blocker | 否 |
| `ddl.column.name.keyword.forbid` | 列名不得使用保留关键字 | blocker | 否 |
| `ddl.column.comment.require` | 列必须拥有非空的 COMMENT | warning | 否 |
| `ddl.column.default.require` | 列必须设置 DEFAULT 值 | warning | 否 |
| `ddl.column.not_null.require` | 列必须为 NOT NULL | warning | 否 |
| `ddl.column.varchar.max_length` | VARCHAR 长度限制 | blocker | 否 |
| `ddl.column.char.max_length` | CHAR 长度建议 | warning | 否 |
| `ddl.column.float_double.forbid` | 不建议使用 FLOAT/DOUBLE 类型 | warning | 否 |
| `ddl.column.blob_text.forbid` | BLOB/TEXT 类型治理（默认禁用） | warning | 否 |
| `ddl.column.json.forbid` | JSON 类型治理（默认禁用） | warning | 否 |
| `ddl.column.bit.forbid` | BIT 类型治理（默认禁用） | warning | 否 |
| `ddl.column.timestamp.forbid` | 禁止使用 TIMESTAMP 类型（推荐使用 DATETIME） | warning | 否 |
| `ddl.column.charset.allowlist` | 列字符集必须在允许列表中 | blocker | 否 |
| `ddl.column.collation.allowlist` | 列排序规则必须在允许列表中 | blocker | 否 |
| `ddl.column.charset_collation.match.require` | 列字符集与排序规则必须兼容 | blocker | 否 |

### 索引级规则（11 条）

这些规则对 `CREATE TABLE` 语句中的索引定义进行评估。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.index.total.max_count` | 单表最大索引数量 | warning | 否 |
| `ddl.index.columns.max_count` | 单个索引最大列数 | warning | 否 |
| `ddl.index.name.pattern.require` | 索引名必须符合命名规则 | blocker | 否 |
| `ddl.index.name.keyword.forbid` | 索引名不得使用保留关键字 | blocker | 否 |
| `ddl.index.unique.prefix.require` | UNIQUE INDEX 名称必须以规定前缀开头（默认：`uniq_`） | warning | 否 |
| `ddl.index.secondary.prefix.require` | 普通 INDEX 名称必须以规定前缀开头（默认：`idx_`） | warning | 否 |
| `ddl.index.fulltext.prefix.require` | FULLTEXT INDEX 名称必须以规定前缀开头（默认：`full_`） | warning | 否 |
| `ddl.index.duplicate.forbid` | 禁止重复索引（列顺序相同） | warning | 否 |
| `ddl.index.redundant_left_prefix.forbid` | 禁止作为其他索引左前缀子集的冗余索引 | warning | 否 |
| `ddl.index.redundant_unique_overlap.forbid` | 禁止被其他 UNIQUE 索引覆盖的冗余 UNIQUE 索引 | warning | 否 |
| `ddl.index.key_length.max_bytes.require` | 索引键长度不得超过实例限制 | blocker | **是** |

**PostgreSQL 索引可用性（v0.38.0，v0.49.0 更新）：** `ddl.index.secondary.prefix.require`、`ddl.index.unique.prefix.require` 和 `ddl.index.columns.max_count` 现在也适用于独立的 PostgreSQL `CREATE INDEX`、`CREATE UNIQUE INDEX` 和 `CREATE INDEX CONCURRENTLY` 语句。自 v0.49.0 起，partial index、expression index、INCLUDE 覆盖索引和非 btree 访问方法（GIN、hash 等）走规范化路径而非返回 unsupported。Operator class 和 NULLS NOT DISTINCT 仍不在 scope 内。

### 视图规则

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.view.create.forbid` | 禁止 CREATE/REPLACE 以及 MySQL ALTER VIEW 定义 | blocker | 否 |
| `ddl.view.drop.forbid` | 禁止 DROP VIEW | blocker | 否 |
| `ddl.view.target.valid` | 检查视图目标的存在性和类型 | blocker | 有序状态 |
| `ddl.view.source.valid` | 检查依赖、列绑定和输出名称 | blocker | 有序状态 |
| `ddl.view.security.definer.warn` | 审查生效的 DEFINER 安全模式或显式具名 definer | warning | 继承时使用有序状态 |
| `ddl.view.algorithm.warn` | 审查请求的 TEMPTABLE 或已知 MERGE 回退（MySQL） | warning | 否 |
| `ddl.view.check_option.valid` | 拒绝已知不可更新视图上的 CHECK OPTION（MySQL） | blocker | 按需使用有序状态 |
| `ddl.view.options.ignored.warn` | 审查 TiDB 上缺少执行保证的显式选项 | warning | 否 |

六条语义规则没有参数。目标、来源与安全规则适用于 MySQL 和 TiDB；
算法与检查选项规则仅适用于 MySQL，忽略选项审查仅适用于 TiDB。
TiDB 的 ALTER VIEW 和裸 CHECK OPTION 仍是厂商边界。缺少必要事实时
报告证据缺口，不制造 finding；关闭规则或调整级别不会改变有序状态。
安全警告不证明账户存在或权限有效。

---

## DDL：修改表规则

### 结构性变更（15 条）

这些规则约束 `ALTER TABLE` 语句中允许的结构性操作。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.alter.drop_column.forbid` | DROP COLUMN 治理（默认不强制） | warning | 否 |
| `ddl.alter.drop_index.forbid` | DROP INDEX 治理（默认不强制） | warning | 否 |
| `ddl.alter.drop_primary_key.forbid` | 禁止 DROP PRIMARY KEY | blocker | 否 |
| `ddl.alter.rename_table.forbid` | 禁止 RENAME TABLE | blocker | 否 |
| `ddl.alter.rename_column.forbid` | 禁止 RENAME COLUMN | blocker | 否 |
| `ddl.alter.rename_index.forbid` | 禁止 RENAME INDEX | blocker | 否 |
| `ddl.alter.change_column.forbid` | 禁止 CHANGE COLUMN | blocker | 否 |
| `ddl.alter.modify_column.forbid` | MODIFY COLUMN 治理（默认不强制） | warning | 否 |
| `ddl.alter.add_index.columns.max_count` | ADD INDEX 列数限制 | warning | 否 |
| `ddl.alter.add_index.duplicate.forbid` | ADD INDEX 不得与已有索引重复 | warning | 否 |
| `ddl.alter.add_index.redundant_left_prefix.forbid` | ADD INDEX 不得成为已有索引的左前缀 | warning | 否 |
| `ddl.alter.add_index.redundant_unique_overlap.forbid` | ADD UNIQUE INDEX 不得被已有 UNIQUE 索引覆盖 | warning | 否 |
| `ddl.alter.add_index.unique.prefix.require` | ADD UNIQUE INDEX 名称前缀要求 | warning | 否 |
| `ddl.alter.add_index.secondary.prefix.require` | ADD INDEX 名称前缀要求 | warning | 否 |
| `ddl.alter.add_index.fulltext.prefix.require` | ADD FULLTEXT INDEX 名称前缀要求 | warning | 否 |

**PostgreSQL ALTER TABLE ADD CONSTRAINT（v0.39.0）：** `ddl.alter.add_index.unique.prefix.require` 现在也适用于 PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... UNIQUE` 形式。`ddl.table.primary_key.bigint.require` 和 `ddl.table.primary_key.columns.max_count` 现在也适用于 PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... PRIMARY KEY`。这些规则复用已有的共享 alter-table 索引和主键规则族——未新增规则 ID。

### 类型兼容性规则（12 条）

这些规则检查通过 `MODIFY COLUMN` 或 `CHANGE COLUMN` 进行的列类型和属性变更是否安全且兼容。大多数规则需要实时元数据来将目标值与当前列定义进行比较。unknown-prior-state advisory 是显式 MODIFY 可空性子句在离线模式下的收敛回退。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.alter.modify_column.target_type_family.allowlist` | MODIFY COLUMN 目标类型族必须在允许列表中 | blocker | 否 |
| `ddl.alter.change_column.target_type_family.allowlist` | CHANGE COLUMN 目标类型族必须在允许列表中 | blocker | 否 |
| `ddl.alter.modify_column.compatibility.require` | MODIFY COLUMN 必须与当前列类型兼容 | blocker | **是** |
| `ddl.alter.change_column.compatibility.require` | CHANGE COLUMN 必须与当前列类型兼容 | blocker | **是** |
| `ddl.alter.modify_column.explicit_nullability_change.forbid` | MODIFY COLUMN 不得显式更改可空性 | blocker | **是** |
| `ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory` | 显式 MODIFY NULL/NOT NULL 但前置状态未知时发出非阻断提示 | notice | 否 |
| `ddl.alter.change_column.explicit_nullability_change.forbid` | CHANGE COLUMN 不得显式更改可空性 | blocker | **是** |
| `ddl.alter.modify_column.explicit_default_change.forbid` | MODIFY COLUMN 不得显式更改 DEFAULT 值 | blocker | **是** |
| `ddl.alter.change_column.explicit_default_change.forbid` | CHANGE COLUMN 不得显式更改 DEFAULT 值 | blocker | **是** |
| `ddl.alter.modify_column.explicit_auto_increment_change.forbid` | MODIFY COLUMN 不得添加或删除 AUTO_INCREMENT | blocker | **是** |
| `ddl.alter.change_column.explicit_auto_increment_change.forbid` | CHANGE COLUMN 不得添加或删除 AUTO_INCREMENT | blocker | **是** |
| `ddl.alter.table_option.compatibility.require` | 表选项变更必须与当前表选项兼容 | warning | **是** |

### 存在性与列身份规则（20 条 — 元数据支撑）

这些规则验证 DDL 语句引用的对象在请求内有序 schema 视图中确实存在（或确实不存在）：配置了元数据时取实时表快照，否则对 MySQL 与 TiDB 使用同一审计批次内先前语句派生的状态。`ddl.table.exists.alter.require`、`ddl.table.exists.create.forbid`、全部列/索引成员存在性规则（`add`/`drop`/`modify`/`change`/`rename` 列、`add`/`drop`/`rename` 索引、`set_default`/`drop_default` 目标列以及 `AFTER` 相对列位置引用）、CHANGE 与 RENAME 的目的名规则、`ddl.alter.drop_primary_key.exists.require`、`ddl.create_index.columns.exists.require` 与 `ddl.table.drop.exists.require` 在 MySQL/TiDB 上状态无法确定时产生 `unknown_table_state` 证据缺口；其余规则在无可用状态时仍静默跳过。PostgreSQL 的相关检查使用下述独立证据缺口。

在 MySQL 和 TiDB 上，`ddl.alter.change_column.target.exists.forbid` 与 `ddl.alter.rename_column.target.exists.forbid` 在新名属于另一列时各报告一条 blocker。旧名与新名是同一身份时不是自冲突。列集合未知时仍是既有 `unknown_table_state` 缺口，不把新名说成空闲。关闭这两条规则也不会让有序状态发布已知重名。

对已观测的 PostgreSQL 17，既有 ADD、RENAME 列规则使用普通持久非类型表的精确、新鲜列目录。ADD 重复用户列时报告 blocker，带 `IF NOT EXISTS` 则静默，并抑制三条描述实际新增的提示。独立声明规则仍运行。已观测的系统列重名不受 guard 保护。RENAME 先检查源列，缺失或系统源列报告 blocker。目的名已存在时也报告 blocker，包括改成原名。引号中的大小写和空格按原样比较。范围不明、目录过期或存在多个子动作时报告 `unproven_column_identity`。独立 RENAME 禁止策略仍适用。

PostgreSQL `ALTER TABLE IF EXISTS` 的既有存在性规则在已证明限定名缺失或已观测到任意类型关系时静默。未限定名在指定 schema 内查无、关系观察过期或身份未绑定时报告 `unproven_alter_table_relation`。三段限定名缺少数据库绑定时同样保留证据缺口。已存在关系仍需满足子动作自身条件。本切片不验证类型转换、表达式、依赖或权限。

`ddl.alter.rename_column.version.require` 按已解析版本检查 `RENAME COLUMN`。MySQL 8.0 系列中的 8.0.3 及以上、MySQL 8.4、TiDB 8.5 通过。MySQL 5.7 与 MySQL 8.0.0–8.0.2 是一条确定的版本不兼容 blocker：coverage 保持 `complete`，结论为 `reject`。版本缺失是 `missing_target_version` 缺口，不额外增加 finding。合法但不在已验证系列内的版本是 `target_version_out_of_validated_range` 缺口，不发布后态。`CHANGE COLUMN` 不继承这项检查。黄金 profile `t05-a5-column-identity-isolated` 启用原先八条有序状态 blocker 加上这三条，并关闭其余目录规则。它不改变既有规则的默认含义。

在 MySQL 与 TiDB 上，一条普通的单动作 `DROP COLUMN` 会从同一请求的后续语句中去掉该列。前提是表已知存在、列集已知、删除后至少还剩一列，并且已加载的主键、索引、约束和同表其他列的表达式都能证明与该列无关。Unknown 为 false 的 nil 或空索引、空约束集合是已知空集。无关成员保留，包括 `PRIMARY(id)`。统计清理与一次普通 `MODIFY` 相同。之后同名 `ADD` 使用新定义。普通索引引用该列、成员集合未知、最后一列、`IF EXISTS`，或语句里还有其他动作时，命名表身份和已加载依赖一起失效。这一子集不删除或收缩索引，也不改写约束。PostgreSQL 仍使用原先的逐句快照。黄金 profile `t05-a6-drop-column-isolated` 启用六条既有 blocker，并关闭其余目录规则，不改变这些规则的默认含义。CLI 的 `--schema` 仍会进入元数据模式，所以离线 CLI 调用的请求 schema 为空。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.table.exists.alter.require` | ALTER TABLE 目标表必须存在 | blocker | **是** |
| `ddl.table.exists.create.forbid` | CREATE TABLE 目标表必须尚不存在 | blocker | **是** |
| `ddl.alter.add_column.exists.forbid` | ADD COLUMN 目标列必须尚不存在 | blocker | **是** |
| `ddl.alter.drop_column.exists.require` | 检查 DROP COLUMN 目标是否存在，以及 PostgreSQL RESTRICT 下的直接依赖 | blocker | **是** |
| `ddl.alter.modify_column.exists.require` | MODIFY COLUMN 目标列必须存在 | blocker | **是** |
| `ddl.alter.change_column.exists.require` | CHANGE COLUMN 源列必须存在 | blocker | **是** |
| `ddl.alter.change_column.target.exists.forbid` | CHANGE COLUMN 新名不得属于另一列 | blocker | **是** |
| `ddl.alter.rename_column.exists.require` | RENAME COLUMN 源列必须存在 | blocker | **是** |
| `ddl.alter.rename_column.target.exists.forbid` | RENAME COLUMN 新名不得属于另一列 | blocker | **是** |
| `ddl.alter.rename_column.version.require` | RENAME COLUMN 需要 MySQL 8.0.3+（8.0 系列）、MySQL 8.4 或 TiDB 8.5；已知更早的 MySQL 版本是一条 blocker | blocker | **是** |
| `ddl.alter.set_default.exists.require` | ALTER COLUMN SET DEFAULT 目标列必须存在 | blocker | **是** |
| `ddl.alter.drop_default.exists.require` | ALTER COLUMN DROP DEFAULT 目标列必须存在 | blocker | **是** |
| `ddl.alter.column_position.exists.require` | AFTER 位置子句引用的列必须存在（MySQL 按同语句累计工作状态评估兄弟列；TiDB 按语句前状态） | blocker | **是** |
| `ddl.alter.add_index.exists.forbid` | ADD INDEX 名称必须尚不存在 | blocker | **是** |
| `ddl.alter.drop_index.exists.require` | DROP INDEX 目标索引必须存在 | blocker | **是** |
| `ddl.alter.rename_index.exists.require` | RENAME INDEX 源索引必须存在 | blocker | **是** |
| `ddl.alter.drop_primary_key.exists.require` | DROP PRIMARY KEY 要求主键存在 | blocker | **是** |
| `ddl.create_index.columns.exists.require` | 独立 `CREATE INDEX` 引用的列必须在目标表上存在 | blocker | **是** |
| `ddl.alter.alter_check.exists.require` | `ALTER CHECK`/`ALTER CONSTRAINT` 目标必须是已证 CHECK 成员 | blocker | **是** |
| `ddl.alter.drop_check.exists.require` | `DROP CHECK`/`DROP CONSTRAINT` 目标必须是已证 CHECK 成员 | blocker | **是** |

### 高级列规则（2 条）

这组规则覆盖 MySQL/TiDB 高级列能力——生成列、表达式默认值（`DEFAULT (<expr>)` 与 `ALTER ... ALTER COLUMN ... SET/DROP DEFAULT`）以及列可见性——基于从解析 AST 与 MySQL catalog 收集的共享、不可求值的表达式事实。

`ddl.alter.column.dependency.require` 阻止源列仍被兄弟列的生成列或表达式默认定义引用的 `DROP COLUMN`、`RENAME COLUMN` 或改名的 `CHANGE COLUMN`，按声明顺序为每个依赖方产出一条 blocker。同名的 `MODIFY COLUMN` 保留列身份，不在本规则范围内。列集合被扣留或依赖图无法证明时产生 `unknown_table_state`/`incomplete_alter_dependency_effects` 证据缺口而非静默通过；有序状态也绝不把已被证伪的转移发布为可复用的成功后态。

`ddl.column.advanced.compatibility.require` 应用冻结的产品/版本责任表：生成列要求 MySQL 5.7.6+ 或 TiDB 8.5，表达式默认值要求 MySQL 8.0.13+，可见性要求 MySQL 8.0.23+。冻结的 TiDB 默认值区分是明确的：`CREATE` 上的 `JSON_OBJECT` 族默认值和针对 `VARCHAR(36)` 的 `ALTER ... SET DEFAULT (uuid())` 是已观测正例；通过 `SET DEFAULT` 的 JSON 表达式默认值和算术/本地列默认值是已观测负例。未观测族（包括不在已观测集合内的标准函数）报告为有界证据缺口，绝不当作已知的库内拒绝。子查询、聚合、非确定性生成函数、存储模式转换是已确定的负例。声明级检查在离线时同样执行；只有需要与旧列对比的检查才读取请求内有序 schema 视图，无法证明时报告 `incomplete_source_column`/`target_version_out_of_validated_range` 缺口而不是伪造事实。可选 `features` 列表参数可收窄执行范围（`generated`、`expression_default`、`visibility`；省略则三者全启用，`[]` 表示规则已加载但不适用，未知名称在加载时被拒绝）。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.alter.column.dependency.require` | 当兄弟生成列或表达式默认列仍引用源列时，阻止其删除/改名/变更身份（同名列的 MODIFY 不在范围内） | blocker | **是** |
| `ddl.column.advanced.compatibility.require` | 生成列、表达式默认值与可见性声明及转移必须满足目标产品/版本契约 | blocker | **是** |

### CHECK 约束生命周期规则（5 条 — #91）

这组规则覆盖 MySQL/TiDB CHECK 约束生命周期——`CREATE TABLE` 声明、列级 `CHECK` 选项、`ALTER TABLE ... ADD CONSTRAINT ... CHECK`、`DROP CHECK`/`DROP CONSTRAINT`、以及 `ALTER CHECK`/`ALTER CONSTRAINT ... [NOT] ENFORCED`——由一个共享的 arm 判定先回答目标是否真正存储并执行 CHECK 约束。`ALTER CONSTRAINT` 归一化为 `ALTER CHECK` 的别名，`DROP CHECK`/`DROP CONSTRAINT` 共享同一目标绑定。CHECK 原始表达式文本从不保留也不发出；只有归一化的结构/引用事实与约束名参与判定。

`ddl.constraint.check.not_enforced.notice` 在真实存储并执行约束的 arm 上（MySQL 8.0.16+；`tidb_enable_check_constraint=ON` 的 TiDB）对每个声明或翻转的 `NOT ENFORCED` CHECK 触发一次。`ENFORCED`、普通列变更、缺失或非 CHECK 的翻转目标不触发同一 finding——已证缺失目标改由存在性 blocker 覆盖，arm 无法判定时（版本缺失、TiDB 开关缺失）产生可归属证据缺口而非 finding。

`ddl.constraint.check.ignored.notice` 揭示接受但忽略的声明：MySQL 8.0.16 以下解析裸 CHECK 声明但既不存储也不执行，TiDB 在 `tidb_enable_check_constraint=OFF` 时解析但静默忽略。该 notice 代替"声明即有效执行"的假象——但只覆盖真正被接受的声明：MySQL 忽略臂上写了 `ENFORCED`/`NOT ENFORCED` 子句的声明是已证 1064 语法拒绝（5.7 探针），由版本规则处理而非此 notice。arm 未决时只产生有界缺口，绝不发此 notice。

`ddl.alter.alter_check.exists.require` 与 `ddl.alter.drop_check.exists.require` 把 `ALTER CHECK`/`ALTER CONSTRAINT`、`DROP CHECK`/`DROP CONSTRAINT` 的目标经共享的作者拼写分类器绑定——`CHECK` 关键字只绑定 CHECK 成员，通用 `CONSTRAINT` 关键字绑定唯一命名的 check/unique/PK/FK 成员——已证缺失、已证非 CHECK（普通索引）、或跨类型同名目标都是一条 blocker（原生 3821/3940/3941/3939 类拒绝）；相关命名空间未证是缺口而非猜测。`ddl.alter.alter_check.version.require` 按拼写分段报告 MySQL 语法拒绝——`CHECK` 拼写自 8.0.16 起执行、通用 `CONSTRAINT` 别名自 8.0.19 起（8.0.17 探针实证），5.7 家族两种拼写都拒绝；TiDB 与受支持的 MySQL 版本上保持静默，版本缺失或拼写不可恢复时仍是有界缺口，不伪造拒绝。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.constraint.check.not_enforced.notice` | 存储/执行 arm 上声明或翻转的 `NOT ENFORCED` CHECK 给出复核 notice | notice | **是** |
| `ddl.constraint.check.ignored.notice` | 接受但忽略 arm（MySQL <8.0.16、TiDB 开关 OFF）上的 CHECK 声明被揭示而非当作已执行 | notice | **是** |
| `ddl.alter.alter_check.exists.require` | `ALTER CHECK`/`ALTER CONSTRAINT` 目标必须是已证 CHECK 成员 | blocker | **是** |
| `ddl.alter.drop_check.exists.require` | `DROP CHECK`/`DROP CONSTRAINT` 目标必须是已证 CHECK 成员 | blocker | **是** |
| `ddl.alter.alter_check.version.require` | MySQL 上 `ALTER CHECK`/`DROP CHECK` 拼写要求 8.0.16+，通用 `CONSTRAINT` 别名要求 8.0.19+（5.7 家族两者皆拒绝） | blocker | **是** |

> **注：** 有序 schema 状态在 `ALTER CHECK`/`ALTER CONSTRAINT` 转移间携带每个 CHECK 成员的归一化执行位（`enforced`/`not_enforced`），同一请求内的 `ADD CHECK` → `ALTER`/`DROP` 序列保持绑定。MySQL 的元数据 provider 从 `TABLE_CONSTRAINTS` 读成员（存在 `ENFORCED` 列时读取），按 catalog/schema/name 关联 `CHECK_CONSTRAINTS`；TiDB 成员来自 `TIDB_CHECK_CONSTRAINTS`——执行状态经 catalog 不可观测（`SHOW CREATE TABLE` 是唯一出口），有序状态改为跟踪已声明的转移而非伪造 catalog 执行事实。引用列上的 `CHANGE COLUMN` 在 TiDB 上静默连带删除 CHECK（MySQL 拒绝），与探针实测一致。

---

## DDL：对象生命周期规则（8 条）

这些规则约束 `DROP TABLE` 和 `TRUNCATE TABLE` 操作。

在 MySQL/TiDB 上，启用的 DROP 存在性规则遇到未知目标状态时产生一个 `unknown_table_state` 缺口，所需事实为 `target_table.existence`。已确认不存在的目标仍产生原有 blocker，包括 `IF EXISTS`：这是严格政策，不是数据库执行失败预测。符合模板的单表 DROP 为后续 CREATE 保留确定缺席；多目标形式仍保守失效。PostgreSQL 与 TRUNCATE 的缺口行为不变。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.table.drop.forbid` | 禁止 DROP TABLE | blocker | 否 |
| `ddl.table.drop.exists.require` | DROP TABLE 目标表必须存在 | blocker | **是** |
| `ddl.table.drop.adaptive_hash.warn` | 当自适应哈希索引激活时，DROP TABLE 给出警告 | warning | **是** |
| `ddl.table.drop.rows.max_count` | 当表行数过多时，DROP TABLE 给出警告 | warning | **是** |
| `ddl.table.truncate.forbid` | 禁止 TRUNCATE TABLE | blocker | 否 |
| `ddl.table.truncate.exists.require` | TRUNCATE TABLE 目标表必须存在 | blocker | **是** |
| `ddl.table.truncate.adaptive_hash.warn` | 当自适应哈希索引激活时，TRUNCATE 给出警告 | warning | **是** |
| `ddl.table.truncate.rows.max_count` | 当表行数过多时，TRUNCATE 给出警告 | warning | **是** |

---

## DDL：全局规则（3 条）

全局规则在所有语句级规则执行完毕后，对**批次中的全部语句**进行跨语句评估。单条语句无法单独触发这类规则。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.alter.merge.mysql.require` | 对同一张表的多条 ALTER TABLE 语句应合并（MySQL） | warning | 否 |
| `ddl.alter.merge.tidb.require` | 对同一张表的多条 ALTER TABLE 语句指导建议（TiDB，默认禁用） | warning | 否 |
| `ddl.pg.alter.not_valid_constraint.validate.require` | 命名 PostgreSQL CHECK/FK `NOT VALID` 约束应在同一次审计 SQL 批次中被后续匹配的 `VALIDATE CONSTRAINT` 跟随 | warning | 否 |

> **说明：** `ddl.alter.merge.mysql.require` 在同一输入中存在两条或更多针对同一张表的 `ALTER TABLE` 语句时触发。在 MySQL 中，每条 `ALTER TABLE` 都会引发表重建；将它们合并为一条语句可大幅减少停机时间。在 TiDB 中，多条 ALTER 通常开销较小，因此 `ddl.alter.merge.tidb.require` 在默认策略中处于禁用状态。
>
> 从 `v0.42.0` 开始，`ddl.pg.alter.not_valid_constraint.validate.require` 作为 PostgreSQL-only GlobalRule 触发。它扫描同一次审计 SQL 批次中命名的 `ALTER TABLE ... ADD CONSTRAINT ... NOT VALID` CHECK 或 FOREIGN KEY 约束；只有后续存在 schema、table、constraint name 都匹配的 `ALTER TABLE ... VALIDATE CONSTRAINT ...` 时才会 suppress warning。这不是首次支持 `VALIDATE CONSTRAINT`，不查询 live validation state，跳过未命名约束，也不追踪跨文件部署窗口。

---

## DDL：MySQL/TiDB Database/Schema 生命周期规则（2 条）

这些规则用于防范 MySQL 和 TiDB database/schema 生命周期 DDL 操作——`CREATE DATABASE`、`CREATE SCHEMA`、`DROP DATABASE`、`DROP SCHEMA`。在 MySQL/TiDB 中，`SCHEMA` 是 `DATABASE` 的同义词。仅在设置 `--dialect mysql` 或 `--dialect tidb` 时生效，PostgreSQL 方言下自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.database.create.notice` | `CREATE DATABASE` / `CREATE SCHEMA` 创建新的逻辑命名空间——信息性通知 | notice | 否 |
| `ddl.database.drop.warn` | `DROP DATABASE` / `DROP SCHEMA` 移除数据库及其包含的所有对象——应当审查 | warning | 否 |

> **说明：** 这些规则是 MySQL/TiDB 专用的，审计 PostgreSQL SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。`CREATE DATABASE IF NOT EXISTS` 和 `CREATE SCHEMA IF NOT EXISTS` 仍然会发出 notice。`DROP DATABASE IF EXISTS` 和 `DROP SCHEMA IF EXISTS` 仍然会发出 warning。DeltaScope 不执行在线数据库存在性验证。`CREATE DATABASE ... CHARACTER SET` / `COLLATE` 选项作为解析器事实保留，但本里程碑中无策略规则对其进行治理。这不是完整的 DDL 支持——trigger、routine、event 和数据库权限生命周期均推迟支持。

---

## DDL：PostgreSQL 约束目标（3 条规则）

| 规则 ID | 限定检查 | 默认级别 | 需要元数据 |
|---------|----------|:--------:|:----------:|
| `ddl.pg.alter.validate_constraint.target.require` | VALIDATE 的目标存在且为 FK/CHECK | blocker | 是 |
| `ddl.pg.alter.constraint_deferrability.target.require` | ALTER CONSTRAINT deferrability 的目标存在且为 FK | blocker | 是 |
| `ddl.pg.alter.drop_constraint.target.require` | 检查精确 DROP 目标、RESTRICT 下主键或唯一约束的直接外键依赖、继承的 CHECK，以及对分区父表使用 ONLY 时的 CHECK。IF EXISTS 保护目标缺失的情况。 | blocker | 是 |

三条默认行为均已获明确批准。检查依赖精确绑定表/schema 的完整、新鲜约束目录，以及真实观测的稳定 PostgreSQL 17 版本。解码后的标识符区分大小写，重复或归属不明的记录不采用 last-wins。缺失、歧义、过期或超出作用域的事实保留 `unproven_constraint_target` 缺口（`target_table.pg_constraints`、`instance.version` 或 `constraint.if_exists`），不猜测目标不存在。事实缺口沿用 **unverified/review** 协议；语法和 parser 边界仍是 incomplete。调用者的 `target_version` 不能替代服务器观测，也不扩展仅面向 MySQL/TiDB 的公共版本身份协议。

已证明为 validated 的 FK/CHECK 不再收到原有扫描 notice；已证明目标缺失的 DROP IF EXISTS 不再收到移除完整性保证的 notice。事实未知时保留条件式建议。原有 FK/PK forbid 和 NOT VALID→VALIDATE 跟进仍独立生效。

已接受的阶段边界：前序相关 DDL、同一 ALTER 中的前序动作使旧目录事实失效，不推演 PG 后态。未知/parser 失败或未建模的跨身份影响会更保守地失效。本批不启用通用 PostgreSQL 顺序执行模型；目标检查不证明数据满足约束、依赖可移除、权限、继承/分区合法性或执行成功。原生 23514/23503 数据拒绝仍是独立边界。

公共 Go provider 可使用 `PGConstraintState` 别名提供名称、原生 `pg_constraint.contype` 与 validated 状态，目录字段不进入 JSON。新增 finding 仅输出限定身份/动作、`reason_code`、`target_exists` 和可选的单字符 `target_constraint_kind`，不输出表达式或原始 provider 错误。

维护者说明：[决策与冻结的 T37 用例角色](../decisions/2026-11-17-postgresql-constraint-target-evidence.md)。这仍是 F09 的部分切片。

---

## DDL：PostgreSQL ALTER 约束迁移风险（2 条规则）

| 规则 ID | 描述 | 默认级别 | 需要元数据 |
|---------|------|:--------:|:----------:|
| `ddl.pg.alter.add_foreign_key.not_valid.require` | ALTER ADD FOREIGN KEY 未写 NOT VALID 时，请求对既有行进行初始验证 | warning | 否 |
| `ddl.pg.alter.add_primary_key.concurrent_index.warn` | ALTER ADD PRIMARY KEY 未写 USING INDEX 时，请求在 ALTER 锁下构建索引 | warning | 否 |

两条规则默认启用，新增 review 影响已获明确批准。对应声明中写出 NOT VALID / USING INDEX 时，本规则静默；这不代表目标存在、索引合格或语句可成功执行。选项事实缺失或未建模时保留 `unproven_constraint_declaration` 证据缺口（`constraint.not_valid` / `constraint.using_index`），不从字段缺失猜测。CREATE、其他 ALTER 动作以及 MySQL/TiDB 不匹配。

FK 告警不放宽 `ddl.table.foreign_key.forbid`；原有 CHECK NOT VALID 和 NOT VALID 后续 VALIDATE 检查保持独立。建议有适用边界：PostgreSQL 17 的分区表外键不允许 NOT VALID。预先并发建立合格唯一索引可减少 PK 的索引构建步骤，但 USING INDEX 附着及 NOT NULL 验证仍可能需要锁或扫描，不承诺实际耗时或无锁执行。

既有 `ddl.pg.alter.add_unique_constraint.concurrent_index.advisory` 保留 ID 和默认 notice 级别；显式 UNIQUE USING INDEX 不再收到错误的“正在构建索引”提示。普通 UNIQUE 仍提示，并将锁说明修正为文档规定的 ACCESS EXCLUSIVE。新增 finding 仅携带表/schema/约束身份、类型、动作和相关声明布尔值，不输出 SQL 或索引表达式载荷。

维护者说明：[决策记录](../decisions/2026-11-17-postgresql-alter-constraint-build-risk.md)。这只是 F09 的一个切片，不代表约束状态或整族覆盖完成。

---

## DDL：PostgreSQL 列 STORAGE / COMPRESSION（2 条规则）

默认开启、无参数、blocker，要求真实观测 PG17：

| 规则 | 限定检查 |
| --- | --- |
| `ddl.pg.alter.column.storage.valid` | 非法大小写不敏感 STORAGE 模式、已证明用户列缺失/系统目标不适用、或不具备 TOAST 能力类型上的非 PLAIN 策略 |
| `ddl.pg.alter.column.compression.valid` | 非法大小写敏感压缩方法、已证明缺失/不适用目标、非 TOAST 类型上的非 default 压缩、或真实构建枚举证明 lz4 不可用 |

STORAGE 接受 PLAIN/EXTERNAL/EXTENDED/MAIN/DEFAULT，遵循原生大小写语义，不把 Unicode
小写仿形误当合法模式。实际 `pg_type.typstorage` 证明类型能力和 DEFAULT 策略，不能用
类型名/整数宽度或当前列 `attstorage` 替代。COMPRESSION 接受 pglz/lz4/default，quoted
大写名称不等价；固定长度类型允许 compression default；toastable text 即使当前 STORAGE
PLAIN 仍可设置 pglz/lz4。当前默认选 pglz 不代表 lz4 不可用；复用普通用户可读的
pg_settings 查询，从 `default_toast_compression` 的完整 enumvals 证明构建支持，
不新增受限 pg_config 读取；版本、设置来源及其比较，以及 D7 新增的属性过滤和真实
类型联接明确绑定 pg_catalog 来源/操作符，不能采用 search_path 的同名对象或操作符。
这不是对所有历史元数据查询的全面命名空间加固承诺。

目标结论要求精确、新鲜的普通持久非继承/非分区表事实；内在非法模式/方法只需真实版本，
不伪造目标。未知/过期/错绑定/歧义/未建模事实保留 `unproven_column_storage` 或
`unproven_column_compression`（unverified、至少 review），按需指明 `instance.version`、
`target_table.pg_columns`、`column.storage_strategy`、`column.compression_method`、
`column.type_storage`、`instance.pg_compression_lz4`。相关前序 DDL/parser failure/较早
ALTER 子动作撤销旧证明，不模拟后态、权限或依赖。

真实负 attnum 系统成员与用户 Columns 分开；用户目录为空不证明 ctid 缺失。相关既有
生成列、统计/选项和 DROP 检查对已证明不适用的系统目标准确拒绝，不再谎称缺失。
IF EXISTS 不能把真实系统列变成 no-op；保留旧 DROP ID，并抑制这类已证明不可执行目标的
删除 advisory。系统事实未给仍是 gap；quoted 大写用户名称不与系统成员混同。

设置 STORAGE/COMPRESSION 本身不重写/重压缩既有行，不证明未来压缩率、数据、依赖、权限
或执行成功。新 blocker finding 仅含有界动作/标识符/原因，不输出方法载荷、私有目录/构建
事实或 provider 错误。独立 forbid 和 MySQL/TiDB 不变；F09 其他维度继续开放。

[决策与精确证据](../decisions/2026-11-18-postgresql-column-storage-compression-targets.md)。

## DDL：PostgreSQL 列调优与 guarded DROP（3 条新规则）

默认开启、无参数，要求真实观测的 PG17：

| 规则 | 限定检查 | 默认级别 |
| --- | --- | --- |
| `ddl.pg.alter.column.statistics.target.valid` | SET STATISTICS 小于 -1，或已证明目标列缺失 | blocker |
| `ddl.pg.alter.column.statistics.clamp.warn` | 适用的显式目标大于 10000，会被原生降至 10000 | warning |
| `ddl.pg.alter.column.options.valid` | 已知内建 SET n_distinct/n_distinct_inherited 的 [-1, DBL_MAX] 范围、重复已知键及 SET/RESET 已证明缺失的列 | blocker |

DEFAULT、-1、0..10000 合法；大于 10000 不是非法值，原生成功执行并警告降至上限。
已证明非法目标不再发 clamp 提醒。SET 保留有序重复键，不采用最后值；RESET 是移除，
未知 RESET 键不能照搬 SET 的非法声明判断。

依赖目标的结论要求精确、新鲜的普通持久非继承/非分区表目录。已建模的内在数值错误
只需真实观测 PG17，不伪造目录。未知/扩展 SET 键、quoted/nonfinite/overflow/下溢/非正规数/
unmodeled 值及过期/歧义/错误绑定事实保留 `unproven_column_tuning` gap（unverified、
至少 review）；所需事实为 `instance.version`、`target_table.pg_columns`、
`column.statistics_target` 或 `column.options`。相关前序 DDL/parser failure、较早
ALTER 子动作后撤销旧目录，不模拟后态，不证明采样效果、数据、表达式、权限或更广表类型。
PG17 的 strtod 会拒绝下溢，Go 却可能无错误地产生有限非正规数，或把非零字面量舍入成
0；这些值及最小正规数的舍入边界保留 gap。真正的零只从 AST 数值系数确认，正常的小
正规数仍可建模。最小正规数字面量即便原生合法，本阶段也可能仍需 review。

既有 `ddl.alter.drop_column.exists.require` 保留 ID/默认值，但 PostgreSQL 分支改用
精确新鲜 PG17 列和 guard 证明。已证明缺失且有 parsed IF EXISTS 时是 no-op，不再误发
缺失 blocker 或删除 advisory；无 guard 的已证明缺失仍拒绝，真实存在列仍保留删除风险。
未知证明保留 `unproven_column_target`（`instance.version`、`target_table.pg_columns`
或 `column.if_exists`），不猜缺失/no-op。MySQL/TiDB 以及显式开启的独立 forbid 不变。
CASCADE 仅被保留，不证明依赖/权限合法或执行成功。

存储说明改为有条件描述：SET fillfactor 本身不立即重写既有行；设置当前 access method
不重写，分区表设置本身不重写分区数据。不另加默认谓词。新 finding metadata 仅含有界
动作/表/schema/列/参数名/原因，不含选项数值、quoted 载荷或 provider 错误。

[决策与剩余范围](../decisions/2026-11-18-postgresql-column-tuning-and-drop-guards.md)。
其余原始 F09 维度继续追踪，不因此升级为整行支持。

## DDL：PostgreSQL 既有列生成属性（4 条规则）

四条规则均默认开启、无参数，依赖 PG17 元数据。

| 规则 | 限定检查 | 默认级别 |
| --- | --- | --- |
| `ddl.pg.alter.column.generation.transition.valid` | ADD/SET/DROP IDENTITY、DROP EXPRESSION 及 DEFAULT/NOT NULL 的生成属性和主键前置条件，以及 DROP NOT NULL 的现行 replica identity 键列保护 | blocker |
| `ddl.pg.alter.identity.sequence_options.valid` | 重复 mode/options，以及非法 increment/cache/类型/边界/START/RESTART | blocker |
| `ddl.pg.alter.identity.restart.warn` | 显式 RESTART 的值重用及并发发号阻塞风险 | warning |
| `ddl.pg.alter.identity.cycle.warn` | ADD IDENTITY 或 SET options 中的显式 CYCLE | warning |

本阶段要求真实观测的 PG17 banner，以及精确、新鲜的普通持久非继承/非分区表目录。
ADD 要求目标是既有普通整数列，已 NOT NULL、无 DEFAULT 或生成属性；SET identity/
options/RESTART 要求 identity 目标。DROP IDENTITY/EXPRESSION 的 IF EXISTS 仅保护生成
类型缺失，不保护列缺失。identity/stored-generated 上的 DEFAULT，以及 identity/PK
上的 DROP NOT NULL 是已证明冲突。合法目标/选项不产生 blocker，已证明 guarded no-op
不产生新目标错误。旧 blanket forbid 和独立规则不取消，静默不保证整体 pass 或执行成功。

ADD 复用限定的新序列声明检查；SET 使用实际依赖绑定的 sequence 类型和稳定目录参数，
不猜名称或 CREATE 默认值。SET START 只修改记录起点，不改变当前值、不触发 RESTART
提醒。NO CYCLE、遗漏不触发 CYCLE 提醒；D2 CREATE/ADD COLUMN 谓词保持独立、不重复命中。

未知、歧义、unmodeled、过期或错误绑定事实保留 `unproven_column_generation_target` /
`unproven_identity_sequence_options` gap（unverified、至少 review）。缺少真实 PG17
版本时保留版本 gap，不猜 blocker；版本已观测时，内在选项错误可以不依赖列目录证明。
nullable SET NOT NULL 保留数据证明缺口，缩窄边界而未显式 RESTART 时可能仍需未读取的
当前值证明。相关前序 DDL/parser failure、较早 ALTER 子动作后撤销旧事实，不模拟 PG
后态。不扫描数据、不分析表达式/权限、不读取或推进当前计数；新 finding 不携带表达式、
provider 错误或选项数值载荷。

## DDL：PostgreSQL 生成列与标识列声明（2 条规则）

| 规则 ID | 描述 | 默认级别 | 需要元数据 |
|---------|------|:--------:|:----------:|
| `ddl.pg.column.generation.declaration.valid` | 检查 CREATE TABLE / ADD COLUMN 中的 DEFAULT/NULL 冲突、已证明的标识列整数标量类型及序列边界 | blocker | 否 |
| `ddl.pg.column.identity.cycle.warn` | 显式、无歧义的标识列 CYCLE 声明可能重复使用数值，需复核 | warning | 否 |

两条规则默认启用；新增 reject/review 影响已获明确批准。不带 CYCLE 的 ALWAYS / BY DEFAULT、NO CYCLE 和省略循环选项均不触发循环告警。声明已被证明无效时不再发循环告警。INCREMENT 0、非正 CACHE、超出类型范围的 MIN/MAX、MIN ≥ MAX 及范围外的 START 触发有效性 finding；省略的序列边界依据 PostgreSQL 文档规定的 CREATE 默认值和递增方向确定。未解析的自定义类型、重复声明或选项、未建模选项及超出 int64 的数值保留为 `unproven_generation_declaration` 证据缺口，所需事实为 `column.generation_declaration`、`column.identity_type` 或 `column.identity_options`，不采用最后一个值推断。

仅适用于 PostgreSQL CREATE TABLE 和 ALTER TABLE ADD COLUMN。既有列上的 ADD/SET/DROP IDENTITY 不在这两条声明规则范围内，限定前置条件见上一节；SET EXPRESSION 保持原有检查。不验证生成表达式的合法性、易变性和依赖，也不判断继承、LIKE、分区、权限、唯一性或序列运行状态。新增 finding 仅携带表、列、操作、生成类别、模式和原因，不复制表达式或序列数值选项；原有调用者 SQL 输出不变。原生证据使用 PostgreSQL 17.7，解析错误和其他未支持边界仍明确保留。

维护者说明：[决策记录](../decisions/2026-11-16-postgresql-generated-identity-governance.md)。

---

## DDL：MySQL 存储例程安全上下文（1 条规则）

| 规则 ID | 描述 | 默认级别 | 需要元数据 |
|---------|------|:--------:|:----------:|
| `ddl.routine.security.definer.warn` | `CREATE PROCEDURE` / `CREATE FUNCTION` 显式或默认使用 DEFINER，以及 `ALTER PROCEDURE` / `ALTER FUNCTION` 显式指定 `SQL SECURITY DEFINER` 时提示复核 | warning | 否 |

默认启用，仅适用于 MySQL。CREATE 未写安全模式时采用 MySQL 文档规定的 DEFINER 默认值；ALTER 未写该选项时不推断既有模式。显式 INVOKER 不触发本规则，但不代表代码安全。`DEFINER=account` 本身不是触发条件：配合 INVOKER 时静默；CREATE 省略模式时仍因默认模式告警。SONAME 可加载函数、DROP、TiDB 厂商边界和 PostgreSQL 不适用。

finding 仅携带例程名称、种类、操作、`security=definer`、`security_source=authored|create_default` 及可选 schema。不解析账号权限，不分析例程体；RETURN、动态 SQL 等既有边界仍与告警并存。未知或重复的安全特征产生 `unproven_routine_security` 证据缺口（`routine.sql_security`），不按最后一个值推断 INVOKER。经批准，默认告警会将原本 pass 的审计改为 review；仍可通过 enabled/level 配置覆盖。见[决策记录](../decisions/2026-11-16-mysql-routine-security-definer-warn.md)。

---

## DDL：MySQL/TiDB DDL Notice 规则（27 条）

这些规则将此前为 normalized_silent 的 MySQL 和 TiDB DDL 形态提升为 finding-covered。覆盖生命周期 DDL 事件（独立索引、重命名、数据库变更、用户/角色/权限管理、存储过程生命周期、资源组）、ALTER TABLE 动作通知（列添加/删除/修改、约束、索引、外键），以及 TiDB 专属的 placement policy 和 sequence 生命周期。仅在设置 `--dialect mysql` 或 `--dialect tidb` 时生效，PostgreSQL 方言下自动跳过。

### 独立 DDL 生命周期（20 条，共享）

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.rename_table.notice` | `RENAME TABLE` 重命名表 | notice | 否 |
| `ddl.create_index.notice` | 独立 `CREATE INDEX` 创建新索引 | notice | 否 |
| `ddl.drop_index.notice` | 独立 `DROP INDEX` 删除索引 | notice | 否 |
| `ddl.alter_database.notice` | `ALTER DATABASE` 修改数据库级别设置 | notice | 否 |
| `ddl.create_procedure.notice` | `CREATE PROCEDURE` 定义存储过程 | notice | 否 |
| `ddl.drop_procedure.notice` | `DROP PROCEDURE` 删除存储过程 | notice | 否 |
| `ddl.create_user.notice` | `CREATE USER` 创建数据库用户 | notice | 否 |
| `ddl.alter_user.notice` | `ALTER USER` 修改数据库用户 | notice | 否 |
| `ddl.drop_user.notice` | `DROP USER` 删除数据库用户 | notice | 否 |
| `ddl.create_role.notice` | `CREATE ROLE` 创建新角色 | notice | 否 |
| `ddl.drop_role.notice` | `DROP ROLE` 删除角色 | notice | 否 |
| `ddl.grant.notice` | `GRANT` 分配权限 | notice | 否 |
| `ddl.revoke.notice` | `REVOKE` 撤销权限 | notice | 否 |
| `ddl.drop_resource_group.notice` | `DROP RESOURCE GROUP` 删除资源组（仅 MySQL） | notice | 否 |
| `ddl.alter.add_column.notice` | `ALTER TABLE ... ADD COLUMN` 添加列 | notice | 否 |
| `ddl.alter.add_constraint.notice` | `ALTER TABLE ... ADD CONSTRAINT` 添加约束 | notice | 否 |
| `ddl.alter.drop_column.notice` | `ALTER TABLE ... DROP COLUMN` 删除列 | notice | 否 |
| `ddl.alter.modify_column.notice` | `ALTER TABLE ... MODIFY COLUMN` 修改列定义 | notice | 否 |
| `ddl.alter.drop_index.notice` | `ALTER TABLE ... DROP INDEX` 删除索引 | notice | 否 |
| `ddl.alter.drop_foreign_key.notice` | `ALTER TABLE ... DROP FOREIGN KEY` 删除外键 | notice | 否 |

### TiDB 专属（7 条）

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.create_placement_policy.notice` | `CREATE PLACEMENT POLICY` 定义放置策略 | notice | 否 |
| `ddl.alter_placement_policy.notice` | `ALTER PLACEMENT POLICY` 修改放置策略 | notice | 否 |
| `ddl.drop_placement_policy.notice` | `DROP PLACEMENT POLICY` 删除放置策略 | notice | 否 |
| `ddl.create_sequence.notice` | `CREATE SEQUENCE` 定义序列 | notice | 否 |
| `ddl.alter_sequence.notice` | `ALTER SEQUENCE` 修改序列 | notice | 否 |
| `ddl.drop_sequence.notice` | `DROP SEQUENCE` 删除序列 | notice | 否 |
| `ddl.tidb.alter_table.placement_policy.notice` | `ALTER TABLE ... PLACEMENT POLICY` 将放置策略关联到表 | notice | 否 |

> **说明：** 这些规则是 MySQL/TiDB 专用的，审计 PostgreSQL SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。这些仅是信息性 notice 规则——不会阻止、警告或执行在线验证。`ddl.drop_resource_group.notice` 仅限 MySQL，TiDB 下自动跳过。TiDB placement policy 和 sequence 规则在 MySQL 下自动跳过。这不是完整的 MySQL/TiDB DDL 支持——triggers、events、functions、TiDB ALTER VIEW、ALTER PROCEDURE、tablespace 和 parser-error 形态仍推迟支持。

---

## DDL：PostgreSQL 迁移安全规则（9 条）

这些规则用于防范常见的 PostgreSQL 迁移模式，避免引发全表重写、长时间持锁或生产事故。仅在设置 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.create_index.concurrently.require` | `CREATE INDEX` 必须使用 `CONCURRENTLY` 以避免阻塞读写。发现包含有界索引形态元数据：`index_kind`、`access_method`、`column_count`、`included_column_count`、`has_predicate`、`has_expression_keys`、`expression_count`。元数据为增量添加，不输出谓词或表达式 SQL 文本。 | warning | 否 |
| `ddl.pg.alter.add_column.non_null_default.rewrite.warn` | 添加带有 volatile 默认值的 `NOT NULL` 列可能触发全表重写。发现包含：`not_null`、`has_default`、`default_kind`（`literal`、`null`、`function_call`、`expression`、`unknown`）。元数据不输出默认表达式文本或函数名。 | warning | 否 |
| `ddl.pg.alter.constraint_using_index.valid` | 检查 PostgreSQL 17 普通表上 `PRIMARY KEY` 或 `UNIQUE USING INDEX` 的精确目录前提。目录证据足够时核对键列的默认操作符类。缺少适用事实时报告 `unproven_constraint_using_index`。 | blocker | 否 |
| `ddl.pg.alter.add_foreign_key.targets.valid` | 在 PostgreSQL 17 普通持久表上检查两侧身份、被引用键资格和列数。省略引用列时使用目录中实际观测到的主键。`ON DELETE SET NULL` 或 `SET DEFAULT` 中的列清单必须属于声明的外键。 | blocker | 否 |
| `ddl.pg.alter.add_check.name.available` | 对 PostgreSQL 17 上目录新鲜、精确绑定的普通持久表，检查单动作显式命名的 `ADD CHECK` 或 `ADD CHECK NOT VALID` 是否占用同表约束名。证据不足或过期时报告 `unproven_check_constraint_name`。 | blocker | 否 |
| `ddl.pg.alter.add_check.not_valid.require` | `ADD CHECK` 约束应使用 `NOT VALID` 以避免持 `ACCESS EXCLUSIVE` 锁的全表扫描 | warning | 否 |
| `ddl.pg.alter.set_data_type.rewrite.warn` | 更改列类型可能需要全表重写（取决于类型转换）。发现包含：`has_using`（是否存在 USING 子句）。元数据不输出 USING 表达式 SQL 文本。 | warning | 否 |
| `ddl.pg.alter.not_valid_constraint.validate.require` | 命名 CHECK/FK `NOT VALID` 约束在同一次审计 SQL 批次中缺少后续匹配的 `VALIDATE CONSTRAINT` | warning | 否 |
| `ddl.pg.drop_index.advisory` | `DROP INDEX` 移除索引，建议审查依赖查询 | notice | 否 |
| `ddl.pg.alter.add_column.non_null_no_default.warn` | 添加 `NOT NULL` 列但未指定 `DEFAULT`，可能导致大表全表重写。发现包含：`not_null`、`has_default`。 | warning | 否 |
| `ddl.pg.alter.add_unique_constraint.concurrent_index.advisory` | `ADD UNIQUE CONSTRAINT` 不含 `NOT VALID` 且后续没有 `CREATE UNIQUE INDEX CONCURRENTLY`，建议使用并发索引创建以实现零停机部署 | notice | 否 |
| `ddl.pg.alter.drop_constraint.advisory` | `DROP CONSTRAINT` 移除 CHECK、UNIQUE 或 FOREIGN KEY 约束，建议审查依赖查询和数据完整性影响 | notice | 否 |

---

## DDL：PostgreSQL 对象生命周期规则（10 条）

这些规则用于防范 PostgreSQL 对象生命周期 DDL 操作中的风险——schema、sequence 和 materialized view 的 CREATE/DROP/ALTER。仅在设置 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.create_schema.notice` | `CREATE SCHEMA` 创建新的命名空间——信息性通知 | notice | 否 |
| `ddl.pg.drop_schema.advisory` | `DROP SCHEMA` 移除 schema，建议审查依赖对象 | notice | 否 |
| `ddl.pg.drop_schema.cascade.warn` | `DROP SCHEMA ... CASCADE` 使用级联删除，可能静默移除依赖对象 | warning | 否 |
| `ddl.pg.create_sequence.cycle.warn` | `CREATE SEQUENCE ... CYCLE` 可能导致序列值回绕 | warning | 否 |
| `ddl.pg.alter_sequence.restart.warn` | `ALTER SEQUENCE ... RESTART` 重置序列计数器，可能与已有行冲突 | warning | 否 |
| `ddl.pg.alter_sequence.cycle.warn` | `ALTER SEQUENCE ... CYCLE` 在已有序列上启用值回绕 | warning | 否 |
| `ddl.pg.drop_sequence.advisory` | `DROP SEQUENCE` 移除序列，建议审查依赖列 | notice | 否 |
| `ddl.pg.drop_sequence.cascade.warn` | `DROP SEQUENCE ... CASCADE` 使用级联删除，可能静默移除依赖对象 | warning | 否 |
| `ddl.pg.drop_materialized_view.advisory` | `DROP MATERIALIZED VIEW` 移除物化视图，建议审查依赖查询 | notice | 否 |
| `ddl.pg.drop_materialized_view.cascade.warn` | `DROP MATERIALIZED VIEW ... CASCADE` 使用级联删除，可能静默移除依赖对象 | warning | 否 |
| `ddl.pg.refresh_materialized_view.concurrently.warn` | 非并发 `REFRESH MATERIALIZED VIEW`（默认或显式 `WITH DATA`）持有排他锁 | warning | 否 |
| `ddl.pg.refresh_materialized_view.no_data.notice` | `REFRESH MATERIALIZED VIEW ... WITH NO DATA` 清空物化视图——下游读取方可能看到空结果 | notice | 否 |

> **说明：** 这些规则是 PostgreSQL 专用的，审计 MySQL 或 TiDB SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。`CREATE SCHEMA IF NOT EXISTS` 仍然会发出 `ddl.pg.create_schema.notice` 的 notice。已有 `DROP SCHEMA` 行为（`ddl.pg.drop_schema.advisory`、`ddl.pg.drop_schema.cascade.warn`）不变。`CREATE SCHEMA AUTHORIZATION` 和嵌套 `CREATE SCHEMA ... CREATE TABLE ...` 仍不支持/推迟支持。DeltaScope 不执行在线 schema 存在性验证。`CONCURRENTLY` 刷新通过两条物化视图规则均不产生 finding。`WITH NO DATA` 同时触发两条规则，因为它也是非并发的。这不是 `CONCURRENTLY` 所需的唯一索引在线验证——DeltaScope 不会检查物化视图上是否存在唯一索引。

> **说明：** 这些规则是 PostgreSQL 专用的，审计 MySQL 或 TiDB SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。从 `v0.41.0` 开始，`ddl.pg.alter.add_check.not_valid.require` 也对 `ALTER TABLE ... ADD CONSTRAINT ... CHECK` 语句触发。从 `v0.42.0` 开始，`ddl.pg.alter.not_valid_constraint.validate.require` 对命名 CHECK 和 FOREIGN KEY `NOT VALID` 约束执行同批次校验配对检查。CHECK 命名规则（`ddl.constraint.check.name.prefix.require`、`ddl.constraint.check.name.suffix.require`、`ddl.constraint.check.name.contains.require`）在配置后同样覆盖 ALTER TABLE CHECK 路径。

---

## DDL：PostgreSQL 类型生命周期规则（5 条）

这些规则用于防范 PostgreSQL 类型生命周期 DDL 操作中的风险——enum 类型创建、加值和类型删除。仅在设置 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.create_type.enum.notice` | `CREATE TYPE ... AS ENUM` 引入新的 enum 类型——信息性提示 | notice | 否 |
| `ddl.pg.alter_type.add_value.advisory` | `ALTER TYPE ... ADD VALUE` 向已有 enum 追加值——建议审查应用使用情况 | warning | 否 |
| `ddl.pg.alter_type.add_value.position.notice` | `ALTER TYPE ... ADD VALUE ... BEFORE/AFTER` 定位新 enum 值——信息性提示 | notice | 否 |
| `ddl.pg.drop_type.advisory` | `DROP TYPE` 移除用户定义类型——建议审查依赖列和函数 | warning | 否 |
| `ddl.pg.drop_type.cascade.warn` | `DROP TYPE ... CASCADE` 使用级联删除，可能静默移除依赖对象 | warning | 否 |

> **说明：** 这些规则是 PostgreSQL 专用的，审计 MySQL 或 TiDB SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。DeltaScope 不会检查在线依赖对象、验证 enum 值是否已被数据或应用代码使用，也不会建模完整的 PostgreSQL 类型系统语义。复合类型现已支持——参见下方 Composite Type Lifecycle 规则。域（`CREATE DOMAIN ...`）已支持——参见下方域生命周期规则。

---

## DDL：PostgreSQL Composite Type Lifecycle 规则（3 条）

这些规则用于覆盖 PostgreSQL composite type 生命周期 DDL 操作——composite type 创建、重命名和 schema 移动。仅在设置 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.create_type.composite.notice` | `CREATE TYPE ... AS (...)` 引入新的 composite type——信息性提示 | notice | 否 |
| `ddl.pg.alter_type.composite_rename.notice` | `ALTER TYPE ... RENAME TO` 变更 composite type 名称——信息性提示 | notice | 否 |
| `ddl.pg.alter_type.composite_set_schema.notice` | `ALTER TYPE ... SET SCHEMA` 将 composite type 移至不同 schema——信息性提示 | notice | 否 |

> **说明：** 这些规则是 PostgreSQL 专用的，审计 MySQL 或 TiDB SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。`DROP TYPE` 不由 composite-specific 规则覆盖——它复用 Type Lifecycle 规则族中已有的 `ddl.pg.drop_type.advisory` 和 `ddl.pg.drop_type.cascade.warn`。属性级操作（`ADD ATTRIBUTE`、`DROP ATTRIBUTE`、`ALTER ATTRIBUTE ... TYPE`、`RENAME ATTRIBUTE`）明确不支持/延迟。DeltaScope 在结构层级上可以识别 composite type 属性定义中的 `COLLATE` 注解，但不渲染、解释或校验 collation 语义。

---

## DDL：PostgreSQL 域生命周期规则（7 条）

这些规则用于防范 PostgreSQL 域生命周期 DDL 操作中的风险——域创建、约束/默认值/可空性变更、重命名和删除。仅在设置 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.create_domain.notice` | `CREATE DOMAIN` 引入可复用的类型约束——信息性提示 | notice | 否 |
| `ddl.pg.alter_domain.constraint.notice` | `ALTER DOMAIN ... ADD/DROP/VALIDATE CONSTRAINT` 修改类型合约——信息性提示 | notice | 否 |
| `ddl.pg.alter_domain.default.notice` | `ALTER DOMAIN ... SET/DROP DEFAULT` 变更隐式值——信息性提示 | notice | 否 |
| `ddl.pg.alter_domain.not_null.notice` | `ALTER DOMAIN ... SET/DROP NOT NULL` 变更可空性——信息性提示 | notice | 否 |
| `ddl.pg.alter_domain.rename.notice` | `ALTER DOMAIN ... RENAME TO` 变更域名称——信息性提示 | notice | 否 |
| `ddl.pg.drop_domain.advisory` | `DROP DOMAIN` 移除域——建议审查依赖列 | warning | 否 |
| `ddl.pg.drop_domain.cascade.warn` | `DROP DOMAIN ... CASCADE` 使用级联删除，可能静默移除依赖对象 | warning | 否 |

> **说明：** 这些规则是 PostgreSQL 专用的，审计 MySQL 或 TiDB SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。DeltaScope 不渲染 `CHECK` 或 `DEFAULT` 表达式文本——规则只暴露布尔事实（`has_check`、`has_default`、`not_null`）和约束名称，不包含表达式正文。DeltaScope 不对域执行在线依赖验证。`DROP DOMAIN IF EXISTS ... CASCADE` 会同时触发 `ddl.pg.drop_domain.advisory` 和 `ddl.pg.drop_domain.cascade.warn`，属于有意设计。复合类型现已支持——参见上方 Composite Type Lifecycle 规则。扩展现已支持——参见下方 Extension Lifecycle 规则。

---

## DDL：PostgreSQL Extension 生命周期规则（6 条）

这些规则用于防范 PostgreSQL extension 生命周期 DDL 操作中的风险——扩展安装、升级、schema 迁移和删除。仅在设置 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.create_extension.notice` | `CREATE EXTENSION` 安装扩展到数据库——信息性提示 | notice | 否 |
| `ddl.pg.create_extension.cascade.warn` | `CREATE EXTENSION ... CASCADE` 自动安装依赖——可能引入非预期扩展 | warning | 否 |
| `ddl.pg.alter_extension.update.notice` | `ALTER EXTENSION ... UPDATE` / `UPDATE TO` 升级扩展——信息性提示 | notice | 否 |
| `ddl.pg.alter_extension.set_schema.notice` | `ALTER EXTENSION ... SET SCHEMA` 将扩展移至不同 schema——信息性提示 | notice | 否 |
| `ddl.pg.drop_extension.advisory` | `DROP EXTENSION` 移除扩展——建议审查依赖对象 | warning | 否 |
| `ddl.pg.drop_extension.cascade.warn` | `DROP EXTENSION ... CASCADE` 使用级联删除，可能静默移除依赖对象 | warning | 否 |

> **说明：** 这些规则是 PostgreSQL 专用的，审计 MySQL 或 TiDB SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。`CREATE EXTENSION ... CASCADE` 会同时触发 `ddl.pg.create_extension.notice` 和 `ddl.pg.create_extension.cascade.warn`。`DROP EXTENSION ... CASCADE` 会同时触发 `ddl.pg.drop_extension.advisory` 和 `ddl.pg.drop_extension.cascade.warn`，属于有意设计。DeltaScope 不对 extension 做可用性、已安装包、版本兼容性或依赖图的实时校验。Extension 成员变更（`ALTER EXTENSION ... ADD/DROP TABLE`）明确不支持/延迟。表级权限 DCL 现已支持——见下方表级权限 DCL 规则。

---

## DDL：PostgreSQL 表级权限 DCL 规则（4 条）

这些规则覆盖 PostgreSQL 表级权限 DCL 操作——`GRANT ... ON TABLE` 和 `REVOKE ... ON TABLE`。仅在 `--dialect postgresql` 时生效，MySQL/TiDB 方言自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.grant.table_privilege.notice` | `GRANT ... ON TABLE` 授予表级权限——信息性通知 | notice | No |
| `ddl.pg.grant.table_privilege.all.warn` | `GRANT ALL PRIVILEGES ON TABLE` 授予所有权限——警告过度授权 | warning | No |
| `ddl.pg.revoke.table_privilege.notice` | `REVOKE ... ON TABLE` 撤销表级权限——信息性通知 | notice | No |
| `ddl.pg.revoke.table_privilege.cascade.warn` | `REVOKE ... ON TABLE ... CASCADE` 级联撤销依赖权限——警告级联副作用 | warning | No |

> **说明：** 这些规则是 PostgreSQL 专用的，审计 MySQL 或 TiDB SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。`GRANT ALL PRIVILEGES ON TABLE` 会同时触发 `ddl.pg.grant.table_privilege.notice` 和 `ddl.pg.grant.table_privilege.all.warn`。`REVOKE ... ON TABLE ... CASCADE` 会同时触发 `ddl.pg.revoke.table_privilege.notice` 和 `ddl.pg.revoke.table_privilege.cascade.warn`，属于有意设计。支持多个 privileges（如 `SELECT, INSERT`）、多个 grantees、schema-qualified 表名（如 `public.users`）。DeltaScope 不做任何形式的实时校验——不验证 grantee/role 是否存在、不验证 table/object 是否存在、不验证当前用户是否有授权权限、不计算 effective privileges、不解析 role inheritance、不验证 ownership、不评估 RLS/policies。`ALL TABLES IN SCHEMA`、sequence privileges、role membership GRANT/REVOKE 和 `ALTER DEFAULT PRIVILEGES` 不支持。这是窄表级权限 DCL 支持，不是广泛的治理或 admin DCL 支持。

---

## DDL：PostgreSQL ALTER TABLE 覆盖规则（53 条配置）

这些规则在 PostgreSQL Migration-Safety 和 Object Lifecycle 规则族之外扩展 ALTER TABLE 审核覆盖。仅在 `--dialect postgresql` 时生效，MySQL/TiDB 方言自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.alter.drop_column.advisory` | `ALTER TABLE ... DROP COLUMN` 移除列，建议审查依赖查询和应用逻辑 | warning | 否 |
| `ddl.pg.alter.validate_constraint.advisory` | `ALTER TABLE ... VALIDATE CONSTRAINT` 执行验证扫描，提醒注意大表上的表级锁持有时长 | notice | 否 |
| `ddl.pg.alter.add_column.nullable.notice` | `ALTER TABLE ... ADD COLUMN` 添加不带 DEFAULT 的可空列，注意下游代码可能遇到意外 NULL 值 | notice | 否 |
| `ddl.pg.alter.set_schema.advisory` | `ALTER TABLE ... SET SCHEMA` 将表移至不同 schema，建议审查依赖查询和应用连接 | notice | 否 |
| `ddl.pg.alter.owner.advisory` | `ALTER TABLE ... OWNER TO` 更改表所有者，建议审查权限影响 | notice | 否 |
| `ddl.pg.alter.enable_trigger.notice` | `ALTER TABLE ... ENABLE TRIGGER name` 重新启用指定触发器，信息性提示 | notice | 否 |
| `ddl.pg.alter.disable_trigger.warn` | `ALTER TABLE ... DISABLE TRIGGER name` 禁用指定触发器，警告该表上的触发器将不再执行 | warning | 否 |
| `ddl.pg.alter.attach_partition.advisory` | `ALTER TABLE ... ATTACH PARTITION` 将分区挂载到分区表，建议审查分区边界和数据路由 | notice | 否 |
| `ddl.pg.alter.detach_partition.warn` | `ALTER TABLE ... DETACH PARTITION` 分离分区，警告针对该分区的查询可能失败 | warning | 否 |
| `ddl.pg.alter.set_logged.notice` | `ALTER TABLE ... SET LOGGED` 将 unlogged 表转为 logged——信息性提示 | notice | 否 |
| `ddl.pg.alter.set_unlogged.notice` | `ALTER TABLE ... SET UNLOGGED` 将 logged 表转为 unlogged——信息性提示 | notice | 否 |

> **说明：** 这些规则是 PostgreSQL 专用的，审计 MySQL 或 TiDB SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。触发器范围形式（`ENABLE/DISABLE TRIGGER ALL/USER`）已规范化，复用这些相同规则。这不是完整的 PostgreSQL ALTER TABLE 覆盖。DeltaScope 不会验证目标表当前是否为 logged 或 unlogged 状态。

### 存储 / 布局（v0.130.0）

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.alter.set_tablespace.notice` | `ALTER TABLE ... SET TABLESPACE` 描述移动请求，并识别已证明的同表空间选择 | notice | 否 |
| `ddl.pg.alter.set_access_method.warn` | `ALTER TABLE ... SET ACCESS METHOD` 更改表访问方法——警告 rewrite 和兼容性影响 | warning | 否 |

### Trigger / Rule 残留（v0.130.0）

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.alter.enable_replica_trigger.notice` | `ALTER TABLE ... ENABLE REPLICA TRIGGER` 以 replica 模式启用触发器——信息性提示 | notice | 否 |
| `ddl.pg.alter.enable_always_trigger.notice` | `ALTER TABLE ... ENABLE ALWAYS TRIGGER` 以 always 模式启用触发器——信息性提示 | notice | 否 |
| `ddl.pg.alter.enable_rule.notice` | `ALTER TABLE ... ENABLE RULE` 启用重写规则——信息性提示 | notice | 否 |
| `ddl.pg.alter.disable_rule.warn` | `ALTER TABLE ... DISABLE RULE` 禁用重写规则——警告该规则将不再触发 | warning | 否 |
| `ddl.pg.alter.enable_replica_rule.notice` | `ALTER TABLE ... ENABLE REPLICA RULE` 以 replica 模式启用规则——信息性提示 | notice | 否 |
| `ddl.pg.alter.enable_always_rule.notice` | `ALTER TABLE ... ENABLE ALWAYS RULE` 以 always 模式启用规则——信息性提示 | notice | 否 |

### Reloptions（v0.130.0）

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.alter.set_reloptions.warn` | `ALTER TABLE ... SET (...)` 设置存储参数——警告潜在的 rewrite 或行为变化 | warning | 否 |
| `ddl.pg.alter.reset_reloptions.notice` | `ALTER TABLE ... RESET (...)` 将存储参数重置为默认值——信息性提示 | notice | 否 |

### 列属性（v0.140.0）

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.alter.set_column_statistics.notice` | `ALTER TABLE ... ALTER COLUMN ... SET STATISTICS` 设置列统计信息目标——信息性提示 | notice | 否 |
| `ddl.pg.alter.set_column_options.notice` | `ALTER TABLE ... ALTER COLUMN ... SET (...)` 设置列属性选项——信息性提示 | notice | 否 |
| `ddl.pg.alter.reset_column_options.notice` | `ALTER TABLE ... ALTER COLUMN ... RESET (...)` 重置列属性选项——信息性提示 | notice | 否 |
| `ddl.pg.alter.set_column_storage.notice` | `ALTER TABLE ... ALTER COLUMN ... SET STORAGE` 设置列存储策略——信息性提示 | notice | 否 |
| `ddl.pg.alter.set_column_compression.notice` | `ALTER TABLE ... ALTER COLUMN ... SET COMPRESSION` 设置列压缩方法——信息性提示 | notice | 否 |

### CLUSTER / DETACH FINALIZE（v0.140.0）

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.alter.cluster_on.notice` | `ALTER TABLE ... CLUSTER ON` 按指定索引聚簇表——信息性提示 | notice | 否 |
| `ddl.pg.alter.set_without_cluster.notice` | `ALTER TABLE ... SET WITHOUT CLUSTER` 移除表聚簇——信息性提示 | notice | 否 |
| `ddl.pg.alter.detach_partition_finalize.notice` | `ALTER TABLE ... DETACH PARTITION ... FINALIZE` 完成分区拆离——信息性提示 | notice | 否 |

### 表关系（v0.150.0）

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.alter.add_inherit.notice` | `ALTER TABLE ... INHERIT` 添加父表继承关系——信息性提示 | notice | 否 |
| `ddl.pg.alter.drop_inherit.notice` | `ALTER TABLE ... NO INHERIT` 移除父表继承关系——信息性提示 | notice | 否 |
| `ddl.pg.alter.add_of_type.notice` | `ALTER TABLE ... OF type_name` 将表绑定到 typed table 类型——信息性提示 | notice | 否 |
| `ddl.pg.alter.drop_of_type.notice` | `ALTER TABLE ... NOT OF` 移除 typed table 类型绑定——信息性提示 | notice | 否 |

> **说明（v0.130.0）：** 这些规则是 PostgreSQL 专用的离线规则。发现不输出 trigger 函数名、trigger 函数体、rule 查询文本、rule 命令文本、tablespace 名称、access method 名称或 reloption 键/值（如 `fillfactor`、`autovacuum_enabled`）。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
### 约束可延迟性（v0.160.0）

| 规则 ID | 默认级别 | 动作 | 描述 |
|---------|---------|------|------|
| `ddl.pg.alter.constraint_deferrable.notice` | notice | `alter_constraint_deferrable` | 外键约束被标记为 DEFERRABLE |
| `ddl.pg.alter.constraint_initially_deferred.notice` | notice | `alter_constraint_initially_deferred` | 外键约束被标记为 INITIALLY DEFERRED |

> **注（v0.160.0）：** 这些规则是 PostgreSQL 专用且离线的。约束可延迟性 finding 仅输出有限元数据：operation、action、table、constraint name、constraint type（`foreign_key`）以及 deferrable/initially_deferred 布尔标志。不输出原始 SQL、表达式文本、谓词文本、操作符类名、排除操作符、序列选项、目录状态、验证结果或依赖图。`SET WITHOUT OIDS` 以动作 `set_without_oids` 静默规范化，不产生 finding（自 PG12 起已废弃）。

### 最终可解析边界（v0.170.0）

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.alter.set_expression.notice` | `ALTER TABLE ... ALTER COLUMN ... SET EXPRESSION` 设置生成列表达式——信息性提示 | notice | 否 |
| `ddl.pg.alter.add_identity.notice` | `ALTER TABLE ... ALTER COLUMN ... ADD GENERATED ... AS IDENTITY` 为已有列添加身份——信息性提示 | notice | 否 |
| `ddl.pg.alter.add_exclusion_constraint.notice` | `ALTER TABLE ... ADD CONSTRAINT ... EXCLUDE USING` 添加排除约束——信息性提示 | notice | 否 |
| `ddl.pg.alter.move_all_tablespace.notice` | `ALTER TABLE ALL IN TABLESPACE ... SET TABLESPACE ...` 请求在事务内移动符合条件的表 | notice | 否 |

> **注（v0.170.0）：** 这些规则是 PostgreSQL 专用且离线的。最终可解析边界 finding 仅输出有限元数据：operation、action、table、column（仅 SET EXPRESSION 和 ADD IDENTITY）以及 constraint name（仅 ADD EXCLUSION CONSTRAINT）。不输出表达式主体、序列选项、排除操作符/谓词、操作符类名、目录状态或原始 SQL。

| `ddl.pg.alter.replica_identity_full.warn` | `ALTER TABLE ... REPLICA IDENTITY FULL` 写入完整旧行镜像到 WAL——警告复制开销 | warning | 否 |
| `ddl.pg.alter.replica_identity_nothing.warn` | `ALTER TABLE ... REPLICA IDENTITY NOTHING` 不写入旧行镜像到 WAL——警告逻辑复制将不可用 | warning | 否 |
| `ddl.pg.alter.replica_identity_using_index.notice` | `ALTER TABLE ... REPLICA IDENTITY USING INDEX ...` 使用指定索引用于 WAL 旧行镜像——信息性提示 | notice | 否 |

> **说明：** 这些规则是 PostgreSQL 专用的，审计 MySQL 或 TiDB SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。`REPLICA IDENTITY DEFAULT` 已规范化且故意静默。DeltaScope 不会验证 `REPLICA IDENTITY USING INDEX` 所引用的索引是否有效、唯一或非部分索引。

---

## DDL：PostgreSQL RLS/Policy 生命周期规则（7 条）

这些规则覆盖 PostgreSQL 行级安全策略和 RLS 开关操作。仅在 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.create_policy.notice` | `CREATE POLICY` 引入新的 RLS 策略——信息性提示 | notice | 否 |
| `ddl.pg.alter_policy.notice` | `ALTER POLICY` 修改已有 RLS 策略——信息性提示 | notice | 否 |
| `ddl.pg.drop_policy.warn` | `DROP POLICY` 移除 RLS 策略——警告行级保护被移除 | warning | 否 |
| `ddl.pg.alter.enable_rls.notice` | `ALTER TABLE ... ENABLE ROW LEVEL SECURITY` 启用 RLS——信息性提示 | notice | 否 |
| `ddl.pg.alter.disable_rls.warn` | `ALTER TABLE ... DISABLE ROW LEVEL SECURITY` 禁用 RLS——警告行级保护被关闭 | warning | 否 |
| `ddl.pg.alter.force_rls.notice` | `ALTER TABLE ... FORCE ROW LEVEL SECURITY` 对表 owner 强制 RLS——信息性提示 | notice | 否 |
| `ddl.pg.alter.no_force_rls.notice` | `ALTER TABLE ... NO FORCE ROW LEVEL SECURITY` 取消对表 owner 的 RLS 强制——信息性提示 | notice | 否 |

> **说明：** 这些规则是 PostgreSQL 专用的，审计 MySQL 或 TiDB SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。DeltaScope 不评估策略表达式、不验证策略对特定角色的适用性、不检查在线 RLS 状态。`CREATE POLICY ... AS PERMISSIVE` 和 `CREATE POLICY ... AS RESTRICTIVE` 均由 `ddl.pg.create_policy.notice` 覆盖。策略 `WITH CHECK` 和 `USING` 表达式文本不被渲染。这不是完整的 PostgreSQL RLS 治理。

---

## DDL：PostgreSQL Trigger 生命周期规则（3 条）

这些规则覆盖 PostgreSQL 触发器生命周期操作。仅在 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.create_trigger.notice` | `CREATE TRIGGER` 引入新触发器——信息性提示 | notice | 否 |
| `ddl.pg.create_constraint_trigger.warn` | `CREATE CONSTRAINT TRIGGER` 创建约束触发器——警告约束触发器语义 | warning | 否 |
| `ddl.pg.drop_trigger.advisory` | `DROP TRIGGER` 移除触发器——建议审查依赖逻辑 | notice | 否 |

> **说明：** 这些规则是 PostgreSQL 专用的，审计 MySQL 或 TiDB SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。DeltaScope 不评估触发器体、不验证触发器函数是否存在、不检查在线触发器状态。`INSTEAD OF` 触发器和转换表（`REFERENCING OLD TABLE / NEW TABLE`）已规范化但不产生额外 finding。这不是完整的 PostgreSQL 触发器治理。

---

## DDL：PostgreSQL Function/Procedure 生命周期规则（6 条）

这些规则覆盖 PostgreSQL 函数和存储过程生命周期操作。仅在 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.create_function.notice` | `CREATE FUNCTION` 引入新函数——信息性提示 | notice | 否 |
| `ddl.pg.create_function.security_definer.warn` | `CREATE FUNCTION ... SECURITY DEFINER` 以 owner 权限执行——警告权限提升风险 | warning | 否 |
| `ddl.pg.create_or_replace_function.advisory` | `CREATE OR REPLACE FUNCTION` 替换已有函数——建议审查下游依赖 | notice | 否 |
| `ddl.pg.drop_function.advisory` | `DROP FUNCTION` 移除函数——建议审查依赖对象 | notice | 否 |
| `ddl.pg.create_procedure.notice` | `CREATE PROCEDURE` 引入新存储过程——信息性提示 | notice | 否 |
| `ddl.pg.drop_procedure.advisory` | `DROP PROCEDURE` 移除存储过程——建议审查依赖对象 | notice | 否 |

> **说明：** 这些规则是 PostgreSQL 专用的，审计 MySQL 或 TiDB SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。`CREATE FUNCTION ... SECURITY DEFINER` 会同时触发 `ddl.pg.create_function.notice` 和 `ddl.pg.create_function.security_definer.warn`，属于有意设计。DeltaScope 不评估函数体、不验证参数类型、不检查在线函数状态、不解析 `LANGUAGE` / `VOLATILITY` / `PARALLEL` 安全性。函数参数签名不被建模。这不是完整的 PostgreSQL 函数/存储过程治理。

---

## DDL：PostgreSQL 高级视图生命周期规则（6 条）

这些规则覆盖 PostgreSQL 视图生命周期操作，覆盖范围超出基础 `CREATE VIEW` / `DROP VIEW` 形态。仅在 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.create_or_replace_view.advisory` | `CREATE OR REPLACE VIEW` 替换已有视图——建议审查下游依赖 | notice | 否 |
| `ddl.pg.create_temp_view.notice` | `CREATE TEMP VIEW` / `CREATE TEMPORARY VIEW` 创建会话级临时视图——信息性提示 | notice | 否 |
| `ddl.pg.create_view.check_option.notice` | `CREATE VIEW ... WITH CHECK OPTION` 对通过视图的插入/更新强制检查选项——信息性提示 | notice | 否 |
| `ddl.pg.drop_view.cascade.warn` | `DROP VIEW ... CASCADE` 使用级联删除，可能静默移除依赖对象 | warning | 否 |
| `ddl.pg.alter_view.rename.notice` | `ALTER VIEW ... RENAME TO` 变更视图名称——信息性提示 | notice | 否 |
| `ddl.pg.alter_view.set_schema.notice` | `ALTER VIEW ... SET SCHEMA` 将视图移至不同 schema——信息性提示 | notice | 否 |

> **说明：** 这些规则是 PostgreSQL 专用的，审计 MySQL 或 TiDB SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。`CREATE OR REPLACE VIEW` 在基础 `ddl.view.create.forbid`（启用时）之外还会触发 `ddl.pg.create_or_replace_view.advisory`。`CASCADED` 与 `LOCAL` 检查选项语义不被建模。DeltaScope 不评估视图查询体、不检查在线视图状态。这不是完整的 PostgreSQL 视图治理。

---

## DDL：PostgreSQL 已选 ALTER 对象生命周期规则（6 条）

这些规则覆盖 PostgreSQL 对 schema、index 和 materialized view 的已选 ALTER 操作。仅在 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.alter_schema.rename.notice` | `ALTER SCHEMA ... RENAME TO` 变更 schema 名称——信息性提示 | notice | 否 |
| `ddl.pg.alter_schema.owner.notice` | `ALTER SCHEMA ... OWNER TO` 变更 schema owner——信息性提示 | notice | 否 |
| `ddl.pg.alter_index.rename.notice` | `ALTER INDEX ... RENAME TO` 变更索引名称——信息性提示 | notice | 否 |
| `ddl.pg.alter_index.set_tablespace.notice` | `ALTER INDEX ... SET TABLESPACE` 将索引移至不同表空间——信息性提示 | notice | 否 |
| `ddl.pg.alter_materialized_view.rename.notice` | `ALTER MATERIALIZED VIEW ... RENAME TO` 变更物化视图名称——信息性提示 | notice | 否 |
| `ddl.pg.alter_materialized_view.set_schema.notice` | `ALTER MATERIALIZED VIEW ... SET SCHEMA` 将物化视图移至不同 schema——信息性提示 | notice | 否 |

> **说明：** 这些规则是 PostgreSQL 专用的，审计 MySQL 或 TiDB SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。DeltaScope 不验证在线 schema/index/物化视图的存在性、所有权或表空间可用性。这不是完整的 PostgreSQL ALTER 对象生命周期覆盖——这些对象类型的其余 ALTER 形式（如 `ALTER INDEX ... SET (...)`、`ALTER MATERIALIZED VIEW ... OWNER TO`）已推迟。

---

## DDL：PostgreSQL Composite Type Attribute Lifecycle 规则（4 条）

`v0.80.0` 新增 selected PostgreSQL non-permission DDL deep coverage，覆盖 composite type 属性变更操作。这 4 条规则覆盖 `ALTER TYPE ... ADD ATTRIBUTE`、`DROP ATTRIBUTE`、`ALTER ATTRIBUTE ... TYPE` 和 `RENAME ATTRIBUTE`，这些操作此前在 Composite Type Lifecycle 部分列为不支持/延迟。仅在 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.alter_type.add_attribute.notice` | `ALTER TYPE ... ADD ATTRIBUTE` 向 composite type 添加新属性——信息性提示 | notice | 否 |
| `ddl.pg.alter_type.drop_attribute.warn` | `ALTER TYPE ... DROP ATTRIBUTE` 从 composite type 移除属性——警告依赖列和函数 | warning | 否 |
| `ddl.pg.alter_type.alter_attribute_type.warn` | `ALTER TYPE ... ALTER ATTRIBUTE ... TYPE` 变更属性类型——警告潜在数据转换问题 | warning | 否 |
| `ddl.pg.alter_type.rename_attribute.notice` | `ALTER TYPE ... RENAME ATTRIBUTE` 重命名属性——信息性提示 | notice | 否 |

> **说明：** 这些规则是 PostgreSQL 专用的，审计 MySQL 或 TiDB SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。这些规则取代了此前在 Composite Type Lifecycle 部分中对 `ADD ATTRIBUTE`、`DROP ATTRIBUTE`、`ALTER ATTRIBUTE ... TYPE` 和 `RENAME ATTRIBUTE` 列出的不支持/延迟条目。DeltaScope 不检查在线依赖对象、不验证数据转换安全性、不建模完整的 PostgreSQL 类型系统语义。`DROP TYPE` 复用 Type Lifecycle 规则族中的已有规则。不影响 MySQL/TiDB 行为。

---

## DDL：PostgreSQL Extension Member Lifecycle 规则（2 条）

`v0.80.0` 新增 selected PostgreSQL non-permission DDL deep coverage，覆盖 extension 成员变更操作。这 2 条规则覆盖 `ALTER EXTENSION ... ADD TABLE` 和 `ALTER EXTENSION ... DROP TABLE`，这些操作此前在 Extension Lifecycle 部分列为不支持/延迟。仅在 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.alter_extension.add_member.notice` | `ALTER EXTENSION ... ADD TABLE` 将对象添加到扩展——信息性提示 | notice | 否 |
| `ddl.pg.alter_extension.drop_member.warn` | `ALTER EXTENSION ... DROP TABLE` 从扩展中移除对象——警告该对象可能在扩展删除时被一并移除 | warning | 否 |

> **说明：** 这些规则是 PostgreSQL 专用的，审计 MySQL 或 TiDB SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。这些规则取代了此前在 Extension Lifecycle 部分中对 extension 成员变更（`ALTER EXTENSION ... ADD/DROP TABLE`）列出的不支持/延迟条目。DeltaScope 不验证被引用对象是否存在、不验证 extension 成员状态、不检查在线依赖图。不影响 MySQL/TiDB 行为。

---

## DDL：PostgreSQL Publication/Subscription Lifecycle 规则（7 条）

`v0.80.0` 新增 selected PostgreSQL non-permission DDL deep coverage，覆盖逻辑复制的发布和订阅生命周期。这 7 条规则覆盖 `CREATE PUBLICATION`、`ALTER PUBLICATION`、`DROP PUBLICATION`、`CREATE SUBSCRIPTION`、`ALTER SUBSCRIPTION`、`ALTER SUBSCRIPTION ... DISABLE` 和 `DROP SUBSCRIPTION`。仅在 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.create_publication.notice` | `CREATE PUBLICATION` 引入新的逻辑复制发布——信息性提示 | notice | 否 |
| `ddl.pg.alter_publication.notice` | `ALTER PUBLICATION` 修改已有发布——信息性提示 | notice | 否 |
| `ddl.pg.drop_publication.warn` | `DROP PUBLICATION` 移除发布——警告订阅者将停止接收变更 | warning | 否 |
| `ddl.pg.create_subscription.notice` | `CREATE SUBSCRIPTION` 建立新的订阅连接——信息性提示 | notice | 否 |
| `ddl.pg.alter_subscription.notice` | `ALTER SUBSCRIPTION` 修改已有订阅——信息性提示 | notice | 否 |
| `ddl.pg.alter_subscription.disable.warn` | `ALTER SUBSCRIPTION ... DISABLE` 禁用订阅——警告复制将停止 | warning | 否 |
| `ddl.pg.drop_subscription.warn` | `DROP SUBSCRIPTION` 移除订阅——警告关于复制槽清理 | warning | 否 |

> **说明：** 这些规则是 PostgreSQL 专用的，审计 MySQL 或 TiDB SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。`DROP SUBSCRIPTION ... WITH (drop_slot = true)` 仍被延迟（parser_error）——DeltaScope 不解析 `DROP SUBSCRIPTION` 上的 `WITH` 选项子句。DeltaScope 不验证在线发布/订阅状态、复制槽状态或连接参数。发布列列表和行过滤器作为解析器事实保留，但无策略规则对其进行治理。这不是完整的 PostgreSQL 逻辑复制治理。不影响 MySQL/TiDB 行为。

---

## DDL：PostgreSQL Foreign Object Lifecycle 规则（12 条）

`v0.80.0` 新增 selected PostgreSQL non-permission DDL deep coverage，覆盖外部数据包装器、外部服务器、用户映射和外部表的生命周期。这 12 条规则覆盖全部四种外部数据对象类型的 CREATE/ALTER/DROP 操作。仅在 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.create_foreign_data_wrapper.notice` | `CREATE FOREIGN DATA WRAPPER` 引入新的 FDW——信息性提示 | notice | 否 |
| `ddl.pg.alter_foreign_data_wrapper.notice` | `ALTER FOREIGN DATA WRAPPER` 修改已有 FDW——信息性提示 | notice | 否 |
| `ddl.pg.drop_foreign_data_wrapper.warn` | `DROP FOREIGN DATA WRAPPER` 移除 FDW——警告依赖的外部服务器和外部表 | warning | 否 |
| `ddl.pg.create_foreign_server.notice` | `CREATE SERVER` 注册新的外部服务器——信息性提示 | notice | 否 |
| `ddl.pg.alter_foreign_server.notice` | `ALTER SERVER` 修改已有外部服务器——信息性提示 | notice | 否 |
| `ddl.pg.drop_foreign_server.warn` | `DROP SERVER` 移除外部服务器——警告依赖的用户映射和外部表 | warning | 否 |
| `ddl.pg.create_user_mapping.notice` | `CREATE USER MAPPING` 注册外部服务器的用户映射——信息性提示 | notice | 否 |
| `ddl.pg.alter_user_mapping.notice` | `ALTER USER MAPPING` 修改已有用户映射——信息性提示 | notice | 否 |
| `ddl.pg.drop_user_mapping.warn` | `DROP USER MAPPING` 移除用户映射——警告依赖的外部表连接 | warning | 否 |
| `ddl.pg.create_foreign_table.notice` | `CREATE FOREIGN TABLE` 引入新的外部表——信息性提示 | notice | 否 |
| `ddl.pg.alter_foreign_table.notice` | `ALTER FOREIGN TABLE` 修改已有外部表——信息性提示 | notice | 否 |
| `ddl.pg.drop_foreign_table.warn` | `DROP FOREIGN TABLE` 移除外部表——警告依赖查询 | warning | 否 |

> **说明：** 这些规则是 PostgreSQL 专用的，审计 MySQL 或 TiDB SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。DeltaScope 不验证在线外部数据对象的存在性、连接参数、FDW handler/validator 函数或外部表列兼容性。FDW 选项（`OPTIONS (...)`）作为解析器事实保留，但无策略规则对其进行治理。`IMPORT FOREIGN SCHEMA` 仍不支持/延迟。这不是完整的 PostgreSQL 外部数据治理。不影响 MySQL/TiDB 行为。

---

## DDL：PostgreSQL Annotation Lifecycle 规则（4 条）

`v0.80.0` 新增 selected PostgreSQL non-permission DDL deep coverage，覆盖对象注解操作。这 4 条规则覆盖 `COMMENT ON` 和 `SECURITY LABEL` 的设置和移除操作。仅在 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.comment_on.notice` | `COMMENT ON ... IS 'text'` 为数据库对象附加注释——信息性提示 | notice | 否 |
| `ddl.pg.comment_on.remove.notice` | `COMMENT ON ... IS NULL` 移除数据库对象的注释——信息性提示 | notice | 否 |
| `ddl.pg.security_label.notice` | `SECURITY LABEL ... IS 'label'` 为数据库对象附加安全标签——信息性提示 | notice | 否 |
| `ddl.pg.security_label.remove.notice` | `SECURITY LABEL ... IS NULL` 移除数据库对象的安全标签——信息性提示 | notice | 否 |

> **说明：** 这些规则是 PostgreSQL 专用的，审计 MySQL 或 TiDB SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。DeltaScope 不验证目标对象是否存在、不验证注释/标签内容是否符合策略、不检查在线注解状态。`SECURITY LABEL ... FOR provider ...` 的 provider 名称作为解析器事实保留，但无策略规则对其进行治理。这不是完整的 PostgreSQL 注解治理。不影响 MySQL/TiDB 行为。

---

## DDL：PostgreSQL Event Trigger / Rewrite Rule Lifecycle 规则（7 条）

`v0.80.0` 新增 selected PostgreSQL non-permission DDL deep coverage，覆盖事件触发器和重写规则。这 7 条规则覆盖 `CREATE EVENT TRIGGER`、`ALTER EVENT TRIGGER`、`ALTER EVENT TRIGGER ... DISABLE`、`DROP EVENT TRIGGER`、`CREATE RULE`、`ALTER RULE` 和 `DROP RULE`。仅在 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.create_event_trigger.notice` | `CREATE EVENT TRIGGER` 引入新的事件触发器——信息性提示 | notice | 否 |
| `ddl.pg.alter_event_trigger.notice` | `ALTER EVENT TRIGGER` 修改已有事件触发器——信息性提示 | notice | 否 |
| `ddl.pg.alter_event_trigger.disable.warn` | `ALTER EVENT TRIGGER ... DISABLE` 禁用事件触发器——警告 DDL 事件处理将停止 | warning | 否 |
| `ddl.pg.drop_event_trigger.warn` | `DROP EVENT TRIGGER` 移除事件触发器——警告 DDL 事件处理影响 | warning | 否 |
| `ddl.pg.create_rule.notice` | `CREATE RULE` 引入新的重写规则——信息性提示 | notice | 否 |
| `ddl.pg.alter_rule.notice` | `ALTER RULE` 修改已有重写规则——信息性提示 | notice | 否 |
| `ddl.pg.drop_rule.warn` | `DROP RULE` 移除重写规则——警告依赖查询行为 | warning | 否 |

> **说明：** 这些规则是 PostgreSQL 专用的，审计 MySQL 或 TiDB SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。DeltaScope 不评估事件触发器体或重写规则动作、不验证触发器函数是否存在、不检查在线事件触发器/规则状态。事件触发器 `WHEN` 条件和规则 `INSTEAD` / `ALSO` 语义作为解析器事实保留，但无策略规则对其进行治理。这不是完整的 PostgreSQL 事件触发器/重写规则治理。不影响 MySQL/TiDB 行为。

---

## DML 规则（11 条）

这些规则对 DML 语句进行评估：`SELECT`、`INSERT`、`UPDATE`、`DELETE`、`REPLACE`。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `dml.where.require` | UPDATE/DELETE 必须包含 WHERE 子句 | blocker | 否 |
| `dml.limit.forbid` | 不建议在 UPDATE/DELETE 中使用 LIMIT | warning | 否 |
| `dml.order_by.forbid` | 不建议在 UPDATE/DELETE 中使用 ORDER BY | warning | 否 |
| `dml.subquery.forbid` | 禁止在 DML 中使用子查询 | blocker | 否 |
| `dml.join.on.require` | JOIN 必须包含 ON 条件 | blocker | 否 |
| `dml.insert.rows.max_count` | INSERT VALUES 行数限制 | warning | 否 |
| `dml.replace.forbid` | 禁止 REPLACE INTO | blocker | 否 |
| `dml.insert.select.forbid` | 禁止 INSERT INTO … SELECT | blocker | 否 |
| `dml.insert.on_duplicate.forbid` | 禁止 INSERT … ON DUPLICATE KEY UPDATE | blocker | 否 |
| `dml.table.denylist.forbid` | 禁止对拒绝列表中的 schema/表执行 DML | blocker | **是** |
| `dml.table.exists.require` | MySQL/TiDB/PostgreSQL 的 DML 目标表必须存在于实时元数据中 | blocker | **是** |

---

## 离线规则与元数据支撑规则

规则分为两类，取决于是否需要实时数据库连接：

**离线规则**始终运行，即使未提供数据库连接。它们仅基于 SQL 文本和 AST 进行评估。所有未在下方列为元数据支撑的规则均属于离线规则。

**元数据支撑规则**在没有活跃 `MetadataProvider` 时通常静默跳过。离线审计不会对无法取得的状态作出断言。有两个明确的例外：MODIFY unknown-prior-state advisory 以 notice 级别离线运行，而已有的 MODIFY blocker 只有在确认源列并完成比较后才触发；`ddl.table.row_size.max_bytes.require` 按字段级解析其估算所消费的事实——声明齐备的语句无需实例对象即可评估，而真正缺失的 `instance.default_charset`/`instance.innodb_default_row_format` 会报告 `missing_instance_fact` 证据缺口（将覆盖率降为 unverified），而非静默通过（issue #85 T06-A15）。

### 元数据支撑规则 ID 完整列表

unknown-prior-state advisory 会在元数据可用时消费实时状态，因此列在此处；但它明确不要求元数据，离线审计也会发出有界 notice。`ddl.table.row_size.max_bytes.require` 同理被列出：实例事实是它的可选缺省来源，而非连接性前提。

| 规则 ID |
|---------|
| `ddl.table.row_size.max_bytes.require` |
| `ddl.index.key_length.max_bytes.require` |
| `ddl.alter.modify_column.compatibility.require` |
| `ddl.alter.change_column.compatibility.require` |
| `ddl.alter.modify_column.explicit_nullability_change.forbid` |
| `ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory` |
| `ddl.alter.change_column.explicit_nullability_change.forbid` |
| `ddl.alter.modify_column.explicit_default_change.forbid` |
| `ddl.alter.change_column.explicit_default_change.forbid` |
| `ddl.alter.modify_column.explicit_auto_increment_change.forbid` |
| `ddl.alter.change_column.explicit_auto_increment_change.forbid` |
| `ddl.alter.table_option.compatibility.require` |
| `ddl.table.exists.alter.require` |
| `ddl.table.exists.create.forbid` |
| `ddl.alter.add_column.exists.forbid` |
| `ddl.alter.drop_column.exists.require` |
| `ddl.alter.modify_column.exists.require` |
| `ddl.alter.change_column.exists.require` |
| `ddl.alter.change_column.target.exists.forbid` |
| `ddl.alter.rename_column.exists.require` |
| `ddl.alter.rename_column.target.exists.forbid` |
| `ddl.alter.rename_column.version.require` |
| `ddl.alter.add_index.exists.forbid` |
| `ddl.alter.drop_index.exists.require` |
| `ddl.alter.rename_index.exists.require` |
| `ddl.alter.drop_primary_key.exists.require` |
| `ddl.create_index.columns.exists.require` |
| `ddl.alter.alter_check.exists.require` |
| `ddl.alter.drop_check.exists.require` |
| `ddl.alter.alter_check.version.require` |
| `ddl.constraint.check.not_enforced.notice` |
| `ddl.constraint.check.ignored.notice` |
| `ddl.alter.column.dependency.require` |
| `ddl.column.advanced.compatibility.require` |
| `ddl.table.drop.exists.require` |
| `ddl.table.drop.adaptive_hash.warn` |
| `ddl.table.drop.rows.max_count` |
| `ddl.table.truncate.exists.require` |
| `ddl.table.truncate.adaptive_hash.warn` |
| `ddl.table.truncate.rows.max_count` |
| `ddl.table.denylist.forbid` |
| `dml.table.denylist.forbid` |
| `dml.table.exists.require` |

---

## 信任与误配防护（非规则行为）

v0.20.0 引入了增量行为，帮助识别方言误配和未支持的功能面。这些**不是**可配置的规则，在策略 YAML 中没有条目，不能被禁用或调整级别。

| 行为 | 类规则 ID | 说明 |
|------|----------|------|
| PostgreSQL 语法启发式通知 | `dialect.postgresql.syntax.detected.notice` | 在 MySQL/TiDB 路径审计时，检测到常见 PG 专属语法标记（`ON CONFLICT`、`::`、`ALTER COLUMN TYPE USING`、`GENERATED AS IDENTITY`）后作为全局建议性告警发出。`RETURNING` 已不在该列表中，它是合法的 TiDB 语法。DeltaScope 不会自动切换方言。 |
| MySQL DML RETURNING 不支持通知 | `dialect.mysql.returning.unsupported.notice` | 在 MySQL 方言下，当解析出的 DML 语句带有 `RETURNING` 子句时作为全局建议性告警发出（TiDB parser 可识别 `INSERT`、`UPDATE` 和单表 `DELETE` 的 `RETURNING`）。MySQL Server 不支持 DML `RETURNING`；如果 SQL 目标是 TiDB，用 `--dialect tidb` 重跑。TiDB 方言接受 `RETURNING`，不会触发该通知。 |
| PostgreSQL 能力边界错误 | — | 未支持的 PG 功能面返回类型化的 `PostgreSQLCapabilityBoundaryError`，取代启发式字符串匹配。 |
| 启发式误报排除 | — | PostgreSQL 语法启发式不对字符串字面量、双引号标识符、反引号标识符、行注释或块注释中的标记触发。 |
| 信任上下文可见性 | — | CLI 输出格式（json、markdown、quiet）包含审计上下文及方言来源和信任提示。`github-actions` 和 `sarif` 格式不包含。 |
| 规则摘要可见性 | — | CLI 输出格式（json、markdown、quiet）包含已加载、适用和跳过的规则计数。`github-actions` 和 `sarif` 格式不包含。 |

各项能力的权威状态请参见[能力矩阵](audit-capability-matrix.zh-CN.md)。

---

## PostgreSQL DDL 覆盖范围（v0.21.0 / v0.23.0 / v0.24.0）

`v0.21.0` 扩展了 PostgreSQL DDL 标准化范围，使常见迁移后续语句通过共享审核管线处理，不再返回能力边界错误。`v0.23.0` 则扩展了更多常见 PostgreSQL `CREATE TABLE` 约束形态的覆盖范围。`v0.24.0` 深化了这些建表形态的语义信息，通过共享 `spec.Constraint` 模型保留解析器拥有的被引用表和被引用列事实。这些版本均不新增规则 ID；新标准化动作和建表结构在适用时继续复用已有的共享规则族。

### PostgreSQL 边界支持就绪门控（v0.32.0）

`v0.32.0` 是 **PostgreSQL 边界支持就绪门控**。这是一个决策里程碑，不是功能发布。未新增规则 ID。Characterization 测试记录了 generated 和 identity 列的稳定 AST 事实；就绪报告推荐 `v0.33.0` 作为窄事实保留包。未变更任何生产 extractor、spec、rule 或 policy 代码。

### PostgreSQL 列类型目标

`ddl.pg.alter.column_type.destination.valid` 默认级别为 blocker。它在已观测的 PostgreSQL 17 上先核对普通表和用户源列，再检查 `TYPE` 或 `TYPE USING` 的精确两段限定目标。缺失 schema、类型、关联数组、未定义 shell type 或不适用的伪类型可以触发拒绝。规则还检查 varchar、bpchar 和 numeric 已建模的字面参数、精确限定的排序规则，以及普通 TYPE 在已观测标量类型之间的赋值转换。缺少适用证据时报告 `unproven_column_type_destination`。已有值的转换、USING 或默认表达式的求值、权限及实际重写需要其他证据。

### PostgreSQL 批量表空间目标

`ddl.pg.alter.move_all_tablespace.target.valid` 默认级别为 blocker，适用于已观测的 PostgreSQL 17。它按声明顺序检查 `OWNED BY` 角色，随后检查源和目标空间是否存在，以及两侧是否涉及 `pg_global`。裸会话别名按观测连接解析。观察缺失或过期时报告 `unproven_bulk_tablespace_targets`。既有 notice 描述事务内的移动请求，或有证据的同空间选择。权限、表筛选、存储和当前锁状态需分别判断。发现省略角色名和目录标识。

### PostgreSQL 普通表所有者目标

`ddl.pg.alter.owner.target.valid` 默认级别为 blocker，适用于已观测 PostgreSQL 17 上限定名精确、目录新鲜的普通持久非类型表。已证明字面角色缺失或目标为 `PUBLIC` 时报告拒绝。引号角色名按原样匹配。裸 `CURRENT_USER`、`CURRENT_ROLE`、`SESSION_USER` 以实际观测的审计会话为准。缺少观察时报告 `unproven_table_owner_target`。既有 notice 描述请求或本会话中已证明的同所有者选择。发现不输出角色名，也不判断变更权限或 schema 授权。

### PostgreSQL 继承列和分区列

既有 DROP 和 RENAME 源列规则使用 PostgreSQL 17 的继承和分区目录。对精确目标执行单个动作时，DROP 拒绝继承列、直接分区键，以及仅删除仍有附着子分区的父表列。RENAME 拒绝继承源列，以及需要遍历子表时使用的 ONLY。递归后代与依赖结果仍需各自的证据。

### PostgreSQL SET WITHOUT OIDS

对于观测到的 PostgreSQL 17 上的单个 SET WITHOUT OIDS 动作，`ddl.table.exists.alter.require` 检查精确目标的关系种类。IF EXISTS 保护已证缺失的目标。普通表、分区表和外部表满足种类要求，视图和序列会被拒绝。PostgreSQL 保留这一兼容语法，执行时不改变列数据。

### PostgreSQL 普通表改名与目标 schema

`ddl.pg.alter.rename_table.target.valid` 和 `ddl.pg.alter.set_schema.target.valid` 默认级别为 blocker，适用于已观测 PostgreSQL 17 上精确限定名、目录新鲜的普通持久非类型表。RENAME 检查目标关系名与行类型名，识别已观测到可让位的自动生成数组类型。既有改名禁止规则独立生效。SET SCHEMA 检查目标 schema，以及主表、行类型、关联数组类型、附属索引和列自有序列的精确名称。既有 notice 会识别同 schema 请求。缺少适用的目录事实时报告 `unproven_table_namespace_destination`。权限和锁另需证据。

### PostgreSQL ALTER COLUMN TYPE 源列检查

`ddl.pg.alter.column_type.source.valid` 默认级别为 blocker，适用于已观测 PostgreSQL 17 上精确限定名、目录新鲜的普通持久非类型表。源列已证缺失、为实际系统列，或为指定 `USING` 的存储生成列时报告拒绝。证据不足时报告 `unproven_column_type_source`。目标类型、依赖、转换表达式和行数据仍是独立维度。既有迁移提示继续说明可能的重写与锁影响。

### PostgreSQL 普通表持久性转换

`ddl.pg.alter.persistence.transition.valid` 默认级别为 blocker，适用于已观测 PostgreSQL 17 上精确限定名、目录新鲜的普通非类型表。从永久表切到 UNLOGGED 时检查直接 publication 与来自其他永久表的入站外键。从 unlogged 切到 LOGGED 时检查指向非永久表的出站外键。证据不足时报告 `unproven_table_persistence`。已证明持久性相同时，既有 notice 说明本次请求不重写文件。

### PostgreSQL 普通表访问方法目标

`ddl.pg.alter.access_method.target.valid` 默认级别为 blocker。适用于已观测 PostgreSQL 17、限定名精确且目录新鲜的普通持久非类型表，排除共享及 mapped 关系。已证明目标访问方法缺失或属于索引方法时报告拒绝。裸 `DEFAULT` 读取当前有效设置，引号名称按原样匹配。证据不足时报告 `unproven_table_access_method`。既有 warning 会注明已证明的同方法选择保留原表文件。

### PostgreSQL 单表表空间目标

`ddl.pg.alter.tablespace.target.valid` 默认级别为 blocker。适用于已观测 PostgreSQL 17、限定名精确且目录新鲜的普通持久非类型表，排除共享及 mapped 关系。已证目标缺失或为 `pg_global` 时报告拒绝。证据缺失或过期时报告 `unproven_table_tablespace`。零值 `reltablespace` 对应实际数据库默认空间。既有提示在已证明同空间时说明文件不会被复制，其余情况以条件措辞描述文件移动。

### PostgreSQL heap 表存储选项

`ddl.pg.alter.reloptions.valid` 默认级别为 blocker。适用于已观测 PostgreSQL 17、限定名精确且目录新鲜的普通持久非类型 heap 表。检查整数 `fillfactor` 的 10–100 范围、已验证的 `autovacuum_enabled` 布尔输入、重复的已知 SET 键，以及带值 RESET。无值 RESET 可含未知或重复键。未知 SET 键、TOAST 或扩展参数、未经验证的数值形式和不明范围报告 `unproven_table_reloptions`。原有提示仍描述请求及影响。

### PostgreSQL 复制标识索引附着

`ddl.pg.alter.replica_identity_using_index.valid` 默认级别为 blocker。对已观测的 PG17，检查精确限定名绑定、目录新鲜的普通持久非类型表。索引缺失、对象类型或归属不符、非唯一、延迟约束、表达式、谓词或键列不适用时报告原生前提问题。可空的 INCLUDE 列不参与键列检查。绑定、范围或目录事实不足时报告 `unproven_replica_identity_index`。既有 notice 描述请求。PG 可以选中无效索引，选中状态不能证明逻辑复制可用。

### PostgreSQL ALTER TABLE GENERATED 后续边界包（v0.31.0）

`v0.31.0` 将额外的 PostgreSQL generated/identity `ALTER TABLE` 形态映射到显式 unsupported feature 标签，收口了 `v0.30.0` 留下的相邻间隙。这些结果**不是**规则 finding，**没有新增规则 ID**。它们是提取器层契约，返回带特性标签和原因字符串的 `UnsupportedDetail` 条目。

- `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION` → `generated_column`
- `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ...` → `generated_as_identity`
- `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY` → `generated_as_identity`
- 语料、service 以及 CLI / HTTP / MCP / `pkg/deltascope` 的表面对等一起锁定这一契约。
- 这是边界收紧，不是 generated-column 支持、identity-column 支持，也不是完整的 PostgreSQL `ALTER TABLE` 支持。

### PostgreSQL ALTER TABLE GENERATED Boundary Pack（v0.30.0）

`v0.30.0` 收紧了 PostgreSQL `ALTER TABLE ... ADD COLUMN` 在 generated stored / identity 形态下的不支持边界契约。这些结果**不是**规则 finding，**没有新增规则 ID**。它们是提取器层契约，返回带特性标签和原因字符串的 `UnsupportedDetail` 条目。

- `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS (...) STORED` → `generated_column`
- `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS IDENTITY` → `generated_as_identity`
- 语料、service 以及 CLI / HTTP / MCP / `pkg/deltascope` 的表面对等一起锁定这一契约。
- 相邻的 `DROP EXPRESSION`、`SET GENERATED`、`DROP IDENTITY` 现已在 `v0.31.0` 中获得显式 unsupported 映射。
- 这是边界收紧，不是 generated-column 支持、identity-column 支持，也不是广义的 PostgreSQL `ALTER TABLE` 支持。

### PostgreSQL CREATE TABLE 不支持边界（v0.26.0）

`v0.26.0` 在提取器层收口了 PostgreSQL `CREATE TABLE` 的不支持边界契约。以下语法被显式标记为 unsupported——它们**不是**规则 finding，**没有新增规则 ID**。它们是提取器层契约，返回包含特性标签和原因字符串的 `UnsupportedDetail` 条目。

### Schema-Qualified Reference 语义（v0.27.0）

`v0.27.0` 通过 additive `ReferencedSchema` 字段在共享 `spec.Constraint` 契约中保留了 PostgreSQL schema-qualified 被引用对象事实。这**不是**新规则 ID——它属于提取器/共享语义事实。从 `v0.28.0` 开始，FK forbid finding metadata 已暴露 `referenced_schema`、`referenced_table` 和 `referenced_columns`。

### Referenced-Object Metadata Surface（v0.28.0）

`v0.28.0` 将 `ddl.table.foreign_key.forbid` finding metadata 向外扩展，暴露 PostgreSQL 被引用对象字段（`referenced_schema`、`referenced_table`、`referenced_columns`）。这些字段在 `v0.27.0` 已存在于共享语义契约中；`v0.28.0` 使其在 CLI JSON、HTTP 响应、MCP StructuredContent 和 `pkg/deltascope` finding metadata 中可见。

- **没有新 rule ID**——`ddl.table.foreign_key.forbid` 规则不变，仅 finding metadata 更宽。
- **条件发射**——`referenced_schema` 在无 schema 限定符时省略；`referenced_table` 和 `referenced_columns` 在所有携带这些事实的 FK 约束中出现。
- **规范化表示**——`referenced_table` 不会拼接成 `"public.users"`。
- 这**不是** schema-aware FK 策略支持，不是完整的 PostgreSQL 外键支持，也不是新规则族。

### Schema-Aware FK Policy Pack（v0.29.0）

`v0.29.0` 是第一个 schema-aware FK policy 步骤。DeltaScope 新增了 PostgreSQL-only notice 规则 `ddl.pg.table.foreign_key.cross_schema.advisory`，用于显式 cross-schema 外键。

- **规则契约**——仅当审计方言为 PostgreSQL、owning table schema 显式存在、referenced schema 显式存在且两者不同时触发。
- **不触发的情况**——same-schema 外键不触发；裸引用如 `REFERENCES users(id)` 也不触发，因为 referenced schema 仍然 unknown。
- **不做推断/建模**——DeltaScope 不推断 `public`，也不建模 PostgreSQL `search_path` 语义。
- **Metadata surface**——finding 可包含 `table_schema`、`referenced_schema`、`referenced_table`、`referenced_columns`；`referenced_table` 始终规范化为 `"users"`，不会写成 `"auth.users"`。
- **边界**——这不是完整的 PostgreSQL 外键支持，也不是跨 schema 校验引擎。

### PostgreSQL ALTER TABLE FK 事实支持（v0.40.0）

`v0.40.0` 将 FK 规则覆盖扩展到 PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` 形态。已有 FK 规则现在可以对 ALTER TABLE FK 添加产生 findings，与已有的 `CREATE TABLE` FK 路径并列覆盖。

| Rule ID | 触发条件 | 覆盖路径 |
|---------|---------|---------|
| `ddl.table.foreign_key.forbid` | 默认策略下外键约束被禁止 | `CREATE TABLE` + `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` |
| `ddl.pg.table.foreign_key.cross_schema.advisory` | cross-schema FK 引用（owning 与 referenced schema 不同） | `CREATE TABLE` + `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` |

- **不新增 rule ID**——已有规则通过 `DDL.Constraints` 投影覆盖 ALTER TABLE FK 添加。
- **保留事实**——本地列、被引用表、被引用列、referenced schema（schema-qualified 引用时）。
- **不做在线 schema FK 存在性验证**——仅语句级事实。
- **不做可延迟/MATCH FULL 策略扩展**。
- **无 MySQL/TiDB 行为变更**。

| 特性 | 提取器标签 |
|------|-----------|
| Identity 列（`GENERATED ... AS IDENTITY`） | `generated_as_identity` |
| Generated stored 列（`GENERATED ALWAYS AS ... STORED`） | `generated_column` |
| Exclusion 约束（`EXCLUDE USING`） | `exclusion_constraint` |
| 分区表（`PARTITION BY`） | `partitioning` |

每条边界由语料用例和 CLI、HTTP、MCP、`pkg/deltascope` 的表面对等测试锁定。surface 契约详情见[审计能力矩阵](audit-capability-matrix.zh-CN.md)。

### 已支持的 PostgreSQL DDL 动作

| 动作 | 标准化为 | 规则行为 |
|------|---------|---------|
| `ALTER COLUMN ... SET DEFAULT` | `set_default` | 作为标准 alter 动作通过已有的 alter 语义规则处理 |
| `ALTER COLUMN ... DROP DEFAULT` | `drop_default` | 作为标准 alter 动作通过已有的 alter 语义规则处理 |
| `ALTER COLUMN ... SET NOT NULL` | `set_not_null` | 作为标准 alter 动作通过已有的 alter 语义规则处理 |
| `ALTER COLUMN ... DROP NOT NULL` | `drop_not_null` | 作为标准 alter 动作通过已有的 alter 语义规则处理 |
| `VALIDATE CONSTRAINT` | `validate_constraint` | supported 且 auditable。无专用规则；除非其他 finding 适用，否则产生干净的审计结果 |
| `DROP CONSTRAINT` | `drop_constraint` | 约束移除。当目标是主键且 metadata 可用时，已有的 `ddl.alter.drop_primary_key` 规则适用。否则作为标准 alter 动作处理 |
| 表级命名 `CHECK` | `create_table` 共享事实 | supported 且 auditable。配置后可复用既有约束命名治理 |
| 列级内联 `CHECK` | `create_table` 共享事实 | supported 且 auditable。不新增专用规则；仅在现有共享语义适用时产生 finding |
| 表级命名 `UNIQUE` | `create_table` 共享事实 | supported 且 auditable。配置后可复用既有约束命名治理 |
| 列级内联 `UNIQUE` | `create_table` 共享事实 | supported 且 auditable。现有共享索引规则可以消费标准化后的索引事实 |
| 表级命名 `FOREIGN KEY` | `create_table` 共享事实 | supported 且 auditable。仅在策略允许外键时，现有外键命名治理才有意义。`v0.24.0`：保留 `ReferencedTable` 和 `ReferencedColumns` 作为 parser-owned 共享契约事实 |
| 列级内联 `REFERENCES` | `create_table` 共享事实 | supported 且 auditable。仅暴露 parser-owned 共享事实；不发明新的 metadata 语义，也不新增专用规则。`v0.24.0`：保留 `ReferencedTable` 和 `ReferencedColumns` 作为 parser-owned 共享契约事实 |

### 关键说明

- 不需要新的规则配置项。这些版本的价值在于已有的共享规则和 metadata-aware 语义现在可以覆盖更多 PostgreSQL DDL 动作与更丰富的建表语义。
- `DROP CONSTRAINT` 针对主键（如 `DROP CONSTRAINT users_pkey`）仅在 metadata-aware 模式下映射到已有的主键规则。在离线模式下，它作为普通 alter 动作通过，不产生专用 finding。
- `VALIDATE CONSTRAINT` 是 supported 且 auditable 的，但没有专用规则。除非同一语句上适用其他 finding，否则产生干净的审计结果。
- 对内联 `REFERENCES` 的描述应保持收敛：DeltaScope 现在保留 parser-owned 的共享关系事实而不是直接落入能力边界错误，但这并不代表新增了 metadata-aware 外键语义。
- `v0.24.0` 深化了 `v0.23.0` 的外键语义：`ReferencedTable` 和 `ReferencedColumns` 是解析器拥有的结构事实，不是元数据真相。它们代表 SQL 语句所声明的内容，而非数据库 schema 的当前状态。
- `v0.23.0`/`v0.24.0` 的建表工作不应被表述为”完整 PostgreSQL CREATE TABLE 支持”；它只是面向常见、可复用共享规则的结构化覆盖与语义深化。

---

## 参考链接

- **参数文档** — [config.zh-CN.md](config.zh-CN.md)
- **规则评估概念概述** — [../concept/core-concepts.zh-CN.md](../concept/core-concepts.zh-CN.md)
- **元数据感知模式** — [../concept/metadata-aware-mode.zh-CN.md](../concept/metadata-aware-mode.zh-CN.md)
- **CLI 用法** — [cli.zh-CN.md](cli.zh-CN.md)
- **能力矩阵** — [audit-capability-matrix.zh-CN.md](audit-capability-matrix.zh-CN.md)

从 `v0.44.0` 开始，`make release-contract-gates VERSION=vX.Y.Z` 将版本面校验、二进制版本输出、默认策略方言隔离和 archive 完整性合并为统一的 pre-publish gate。未新增规则 ID、解析器功能或公共 API 契约。

### PostgreSQL 排序规则生命周期（v0.100.0）

v0.100.0 新增 PostgreSQL 排序规则对象生命周期审核规则。这些规则仅在设置 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。

| 规则 ID | 检查描述 | 默认级别 | 是否需要元数据 |
|---------|---------|:--------:|:--------------:|
| `ddl.pg.create_collation.notice` | CREATE COLLATION 触发信息通知 | notice | 否 |
| `ddl.pg.alter_collation.notice` | ALTER COLLATION（重命名/属主/模式）触发信息通知 | notice | 否 |
| `ddl.pg.drop_collation.warn` | DROP COLLATION 触发结构销毁警告 | warning | 否 |

> **注意：** 这些规则仅适用于 PostgreSQL，审核 MySQL 或 TiDB SQL 时自动跳过。均为离线规则，不需要数据库连接。

### PostgreSQL 扩展统计生命周期（v0.100.0）

v0.100.0 新增 PostgreSQL 扩展统计对象生命周期审核规则。这些规则仅在设置 `--dialect postgresql` 时生效。

| 规则 ID | 检查描述 | 默认级别 | 是否需要元数据 |
|---------|---------|:--------:|:--------------:|
| `ddl.pg.create_statistics.notice` | CREATE STATISTICS 触发信息通知 | notice | 否 |
| `ddl.pg.alter_statistics.notice` | ALTER STATISTICS（重命名/属主/模式）触发信息通知 | notice | 否 |
| `ddl.pg.drop_statistics.warn` | DROP STATISTICS 触发结构销毁警告 | warning | 否 |

> **注意：** 这些规则仅适用于 PostgreSQL，审核 MySQL 或 TiDB SQL 时自动跳过。均为离线规则，不需要数据库连接。

### PostgreSQL 聚合/操作符/转换生命周期（v0.100.0）

v0.100.0 新增 PostgreSQL 聚合、操作符和转换对象生命周期审核规则。这些规则仅在设置 `--dialect postgresql` 时生效。

| 规则 ID | 检查描述 | 默认级别 | 是否需要元数据 |
|---------|---------|:--------:|:--------------:|
| `ddl.pg.create_aggregate.notice` | CREATE AGGREGATE 触发信息通知 | notice | 否 |
| `ddl.pg.alter_aggregate.notice` | ALTER AGGREGATE（重命名/属主/模式）触发信息通知 | notice | 否 |
| `ddl.pg.drop_aggregate.warn` | DROP AGGREGATE 触发结构销毁警告 | warning | 否 |
| `ddl.pg.create_operator.notice` | CREATE OPERATOR 触发信息通知 | notice | 否 |
| `ddl.pg.alter_operator.notice` | ALTER OPERATOR（属主/模式）触发信息通知 | notice | 否 |
| `ddl.pg.drop_operator.warn` | DROP OPERATOR 触发结构销毁警告 | warning | 否 |
| `ddl.pg.create_conversion.notice` | CREATE CONVERSION 触发信息通知 | notice | 否 |
| `ddl.pg.alter_conversion.notice` | ALTER CONVERSION（重命名/属主/模式）触发信息通知 | notice | 否 |
| `ddl.pg.drop_conversion.warn` | DROP CONVERSION 触发结构销毁警告 | warning | 否 |

> **注意：** 这些规则仅适用于 PostgreSQL，审核 MySQL 或 TiDB SQL 时自动跳过。均为离线规则，不需要数据库连接。规范化发现避免将聚合函数、操作符过程或转换函数名称投射到输出中。

### PostgreSQL 操作符族/类生命周期（v0.100.0）

v0.100.0 新增 PostgreSQL 操作符族和操作符类对象生命周期审核规则。这些规则仅在设置 `--dialect postgresql` 时生效。

| 规则 ID | 检查描述 | 默认级别 | 是否需要元数据 |
|---------|---------|:--------:|:--------------:|
| `ddl.pg.create_operator_family.notice` | CREATE OPERATOR FAMILY 触发信息通知 | notice | 否 |
| `ddl.pg.alter_operator_family.notice` | ALTER OPERATOR FAMILY（重命名/属主/模式）触发信息通知 | notice | 否 |
| `ddl.pg.drop_operator_family.warn` | DROP OPERATOR FAMILY 触发结构销毁警告 | warning | 否 |
| `ddl.pg.create_operator_class.notice` | CREATE OPERATOR CLASS 触发信息通知 | notice | 否 |
| `ddl.pg.alter_operator_class.notice` | ALTER OPERATOR CLASS（重命名/属主/模式）触发信息通知 | notice | 否 |
| `ddl.pg.drop_operator_class.warn` | DROP OPERATOR CLASS 触发结构销毁警告 | warning | 否 |

> **注意：** 这些规则仅适用于 PostgreSQL，审核 MySQL 或 TiDB SQL 时自动跳过。均为离线规则，不需要数据库连接。

### PostgreSQL 全文搜索对象生命周期（v0.100.0）

v0.100.0 新增 PostgreSQL 全文搜索配置、字典、解析器和模板对象生命周期审核规则。这些规则仅在设置 `--dialect postgresql` 时生效。

| 规则 ID | 检查描述 | 默认级别 | 是否需要元数据 |
|---------|---------|:--------:|:--------------:|
| `ddl.pg.create_text_search_configuration.notice` | CREATE TEXT SEARCH CONFIGURATION 触发信息通知 | notice | 否 |
| `ddl.pg.alter_text_search_configuration.notice` | ALTER TEXT SEARCH CONFIGURATION（重命名/属主/模式）触发信息通知 | notice | 否 |
| `ddl.pg.drop_text_search_configuration.warn` | DROP TEXT SEARCH CONFIGURATION 触发结构销毁警告 | warning | 否 |
| `ddl.pg.create_text_search_dictionary.notice` | CREATE TEXT SEARCH DICTIONARY 触发信息通知 | notice | 否 |
| `ddl.pg.alter_text_search_dictionary.notice` | ALTER TEXT SEARCH DICTIONARY（重命名/属主/模式）触发信息通知 | notice | 否 |
| `ddl.pg.drop_text_search_dictionary.warn` | DROP TEXT SEARCH DICTIONARY 触发结构销毁警告 | warning | 否 |
| `ddl.pg.create_text_search_parser.notice` | CREATE TEXT SEARCH PARSER 触发信息通知 | notice | 否 |
| `ddl.pg.alter_text_search_parser.notice` | ALTER TEXT SEARCH PARSER（重命名/模式）触发信息通知 | notice | 否 |
| `ddl.pg.drop_text_search_parser.warn` | DROP TEXT SEARCH PARSER 触发结构销毁警告 | warning | 否 |
| `ddl.pg.create_text_search_template.notice` | CREATE TEXT SEARCH TEMPLATE 触发信息通知 | notice | 否 |
| `ddl.pg.alter_text_search_template.notice` | ALTER TEXT SEARCH TEMPLATE（重命名/模式）触发信息通知 | notice | 否 |
| `ddl.pg.drop_text_search_template.warn` | DROP TEXT SEARCH TEMPLATE 触发结构销毁警告 | warning | 否 |

> **注意：** 这些规则仅适用于 PostgreSQL，审核 MySQL 或 TiDB SQL 时自动跳过。均为离线规则，不需要数据库连接。规范化发现避免将全文搜索函数名称（start/end/lextype/lexize）投射到输出中。

### PostgreSQL 边界闭合（v0.100.0）

v0.100.0 新增选定 PostgreSQL 边界对象生命周期审核规则：DROP TRANSFORM、DROP ACCESS METHOD 和 ALTER LARGE OBJECT 属主变更。这些规则仅在设置 `--dialect postgresql` 时生效。

| 规则 ID | 检查描述 | 默认级别 | 是否需要元数据 |
|---------|---------|:--------:|:--------------:|
| `ddl.pg.create_transform.notice` | CREATE TRANSFORM 触发信息通知 | notice | 否 |
| `ddl.pg.create_access_method.notice` | CREATE ACCESS METHOD 触发信息通知 | notice | 否 |
| `ddl.pg.drop_transform.warn` | DROP TRANSFORM 触发结构销毁警告 | warning | 否 |
| `ddl.pg.drop_access_method.warn` | DROP ACCESS METHOD 触发结构销毁警告 | warning | 否 |
| `ddl.pg.alter_large_object.owner.notice` | ALTER LARGE OBJECT ... OWNER TO 触发信息通知 | notice | 否 |

> **注意：** 这些规则仅适用于 PostgreSQL，审核 MySQL 或 TiDB SQL 时自动跳过。均为离线规则，不需要数据库连接。
>
> **延迟的边界情形：** CREATE TRANSFORM 和 CREATE ACCESS METHOD 被有意不覆盖，因为其处理函数名称即对象身份，安全规范化与载荷安全约束不兼容。

### PostgreSQL 元数据感知对象验证（v0.90.0）

v0.90.0 新增已选 PostgreSQL 生命周期规则发现的元数据感知对象验证。当配置了 PostgreSQL 元数据连接时，DeltaScope 通过 `pg_catalog` 查询解析非表对象，并将对象存在性和安全属性信息注入生命周期发现。**未新增规则 ID。** 现有生命周期规则发现在元数据可用时被元数据字段增强。

#### 元数据投射字段

当对象元数据被解析后，以下字段出现在发现的 `metadata` 对象上：

| 字段 | 类型 | 描述 |
|------|------|------|
| `metadata_status` | string | `confirmed`、`not_found` 或 `unavailable` |
| `metadata_exists` | boolean | 对象是否存在于数据库中 |
| `metadata_object_type` | string | 解析的对象类型（如 `domain`、`type`、`extension`） |
| `metadata_object_name` | string | 解析的对象名称 |
| `metadata_schema` | string | 包含对象的模式（当存在歧义时） |

#### 安全可投射属性

仅以下属性键从对象快照投射到发现中：

`type_kind`、`extension_version`、`enabled`、`server`、`foreign_data_wrapper`、`target_type`、`has_options`、`table`

这些以 `metadata_<key>` 形式出现在发现上（例如 `metadata_type_kind`、`metadata_extension_version`）。

#### 被屏蔽属性

以下属性类别**绝不会**投射到发现中，即使在对象快照中存在：password、secret、token、api_key、connection、dsn、connstr、body、definition、comment、label、query、action_sql、options。

#### 支持的生命周期规则

对象元数据增强适用于以下 PostgreSQL 生命周期规则：

`ddl.pg.drop_schema.advisory`、`ddl.pg.drop_type.advisory`、`ddl.pg.drop_domain.advisory`、`ddl.pg.drop_extension.advisory`、`ddl.pg.drop_sequence.advisory`、`ddl.pg.drop_materialized_view.advisory`、`ddl.pg.drop_publication.warn`、`ddl.pg.drop_foreign_server.warn`、`ddl.pg.drop_user_mapping.warn`、`ddl.pg.comment_on.notice`

无元数据连接时，这些规则照常产生发现 — 不出现元数据字段。

---

### PostgreSQL Generated/Identity Rule Coverage（v0.36.0）

v0.36.0 是 **PostgreSQL Generated/Identity Rule Coverage Pack**。三条新的 PostgreSQL-only forbid 规则覆盖了 v0.35.0 已支持的 generated/identity 状态转换形态。这些规则使用现有的 `newForbiddenAlterActionRule` 构造器加 PostgreSQL-only 方言白名单。

| Rule ID | Action | 覆盖形态 | 方言 |
|---------|--------|---------|------|
| `ddl.alter.drop_expression.forbid` | `drop_expression` | `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION` | 仅 PostgreSQL |
| `ddl.alter.set_generated.forbid` | `set_generated` | `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ...` | 仅 PostgreSQL |
| `ddl.alter.drop_identity.forbid` | `drop_identity` | `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY` | 仅 PostgreSQL |

这些规则在 CLI、HTTP、MCP 和 `pkg/deltascope` 四条产品面上产生明确的 `rule_id` findings。这是规则覆盖——不是 parser 支持范围扩展、不是 spec 契约扩展、不是 generated expression 求值、不是完整的 PostgreSQL 序列语义。无 MySQL/TiDB 行为变更。

### PostgreSQL Generated/Identity State-Transition Support（v0.35.0）

v0.35.0 是 **PostgreSQL Generated/Identity State-Transition Pack**。PostgreSQL generated 和 identity 列的状态转换形态现在通过正常审核路径支持。它是提取器层的支持范围扩展——不是规则行为变更。未新增规则 ID。已有规则对这些新支持形态的适用方式与其它 PostgreSQL DDL 语句一致。

- 已支持形态：`DROP EXPRESSION`、`SET GENERATED ALWAYS`、`SET GENERATED BY DEFAULT`、`DROP IDENTITY`。
- 标准化契约：`drop_expression`、`set_generated` 含 `generated_when`（`"a"` / `"d"`）、`drop_identity`。
- 这不是完整的 generated-column 生命周期支持、不是 generated expression 求值、不是完整的 PostgreSQL 序列语义。
- 无新增规则 ID、CLI 标志或规则行为变更。

### PostgreSQL Generated/Identity Narrow Support（v0.34.0）

v0.34.0 新增了窄范围 generated/identity 定义形态的支持。完整已支持形态表参见 [audit-capability-matrix.zh-CN.md](audit-capability-matrix.zh-CN.md)。保留事实：`generated_when`、`is_identity`、`identity_options`（来自 v0.33.0）继续流转。

## v0.33.0 — 共享契约变更与 Unsupported Metadata

v0.33.0 未新增 rule ID 或规则行为变更。变更限于：

- `spec.Column` 新增 `GeneratedWhen`、`IsIdentity`、`IdentityOptions` 字段（shared contract widening）
- `spec.UnsupportedDetail` 新增 `Metadata map[string]any` 字段（unsupported metadata surfacing）
- PostgreSQL extractor 填充新字段和 metadata
- Surface parity 测试验证四条传输通道的 metadata 流转

Unsupported feature 名称不变：`generated_column`、`generated_as_identity`。
