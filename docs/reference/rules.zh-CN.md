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

### deltascope rules list

列出所有已注册的规则，支持可选过滤：

```bash
# 所有规则
deltascope rules list

# 按类型过滤
deltascope rules list --kind ddl
deltascope rules list --kind dml

# 按严重级别过滤
deltascope rules list --level blocker
deltascope rules list --level warning

# 仅显示当前已加载策略中启用的规则
deltascope rules list --enabled-only
```

输出示例：

```md
# DeltaScope Rules

- `ddl.table.comment.require` [warning] (ddl) Require DDL table comment require
- `ddl.table.row_size.max_bytes.require` [blocker] (ddl) Require DDL table row size max bytes require
- `dml.limit.forbid` [warning] (dml) Forbid DML limit forbid
- `dml.where.require` [blocker] (dml) Require DML where require
```

### deltascope rules show

显示单条规则的完整详情：

```bash
deltascope rules show dml.where.require
```

输出示例：

```md
# dml.where.require

Require DML where require. Default level is blocker, enabled=true, scope=dml, and the shipped policy treats it as a offline-safe rule.

- Default Enabled: `true`
- Default Level: `blocker`
- Statement Kinds: `dml`
- Metadata Aware: `false`

## Default Params
- `required`: `true`

## Trigger Example
```sql
DELETE FROM users;
```

## Valid Example
```sql
DELETE FROM users WHERE id = 1;
```

## Config Example
```yaml
rules:
  dml.where.require:
    enabled: true
    level: blocker
    params:
      required: true
```

## Remediation
Add the required clause, option, or object explicitly so the rule no longer has to infer intent.
```

### deltascope rules search

按关键词搜索规则（匹配规则 ID 和描述文本）：

```bash
deltascope rules search "where clause"
deltascope rules search metadata
deltascope rules search "prefix"
```

---

## DDL：建表规则

### 表级规则（23 条）

这些规则对整个 `CREATE TABLE` 语句的属性进行评估。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.table.comment.require` | 表必须拥有非空的 COMMENT | warning | 否 |
| `ddl.table.comment.max_length` | 表的 COMMENT 不得超过字符数限制 | warning | 否 |
| `ddl.table.name.max_length` | 表名长度限制 | blocker | 否 |
| `ddl.table.name.pattern.require` | 表名必须符合命名规则（默认：字母数字加下划线） | blocker | 否 |
| `ddl.table.name.keyword.forbid` | 表名不得使用 SQL 保留关键字 | blocker | 否 |
| `ddl.table.engine.allowlist` | 存储引擎必须在允许列表中（默认：InnoDB） | blocker | 否 |
| `ddl.table.charset.allowlist` | 表的默认字符集必须在允许列表中 | blocker | 否 |
| `ddl.table.row_format.allowlist` | ROW_FORMAT 必须在允许列表中（默认：DYNAMIC） | blocker | 否 |
| `ddl.table.auto_increment.init_value.require` | AUTO_INCREMENT 初始值必须符合要求 | blocker | 否 |
| `ddl.table.columns.min_count` | 表至少需要 N 列 | blocker | 否 |
| `ddl.table.primary_key.require` | 表必须拥有 PRIMARY KEY | blocker | 否 |
| `ddl.table.primary_key.columns.max_count` | PRIMARY KEY 列数限制 | warning | 否 |
| `ddl.table.primary_key.bigint.require` | PRIMARY KEY 列必须为 BIGINT 类型 | blocker | 否 |
| `ddl.table.primary_key.unsigned.require` | PRIMARY KEY 列必须为 UNSIGNED | blocker | 否 |
| `ddl.table.primary_key.auto_increment.require` | PRIMARY KEY 列必须为 AUTO_INCREMENT | blocker | 否 |
| `ddl.table.primary_key.not_null.require` | PRIMARY KEY 列必须为 NOT NULL | blocker | 否 |
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

### 视图规则（1 条）

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.view.create.forbid` | 禁止 CREATE VIEW | blocker | 否 |

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

### 类型兼容性规则（11 条）

这些规则检查通过 `MODIFY COLUMN` 或 `CHANGE COLUMN` 进行的列类型变更是否安全且兼容。大多数规则需要实时元数据来将目标类型与当前列定义进行比较。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.alter.modify_column.target_type_family.allowlist` | MODIFY COLUMN 目标类型族必须在允许列表中 | blocker | 否 |
| `ddl.alter.change_column.target_type_family.allowlist` | CHANGE COLUMN 目标类型族必须在允许列表中 | blocker | 否 |
| `ddl.alter.modify_column.compatibility.require` | MODIFY COLUMN 必须与当前列类型兼容 | blocker | **是** |
| `ddl.alter.change_column.compatibility.require` | CHANGE COLUMN 必须与当前列类型兼容 | blocker | **是** |
| `ddl.alter.modify_column.explicit_nullability_change.forbid` | MODIFY COLUMN 不得显式更改可空性 | blocker | **是** |
| `ddl.alter.change_column.explicit_nullability_change.forbid` | CHANGE COLUMN 不得显式更改可空性 | blocker | **是** |
| `ddl.alter.modify_column.explicit_default_change.forbid` | MODIFY COLUMN 不得显式更改 DEFAULT 值 | blocker | **是** |
| `ddl.alter.change_column.explicit_default_change.forbid` | CHANGE COLUMN 不得显式更改 DEFAULT 值 | blocker | **是** |
| `ddl.alter.modify_column.explicit_auto_increment_change.forbid` | MODIFY COLUMN 不得添加或删除 AUTO_INCREMENT | blocker | **是** |
| `ddl.alter.change_column.explicit_auto_increment_change.forbid` | CHANGE COLUMN 不得添加或删除 AUTO_INCREMENT | blocker | **是** |
| `ddl.alter.table_option.compatibility.require` | 表选项变更必须与当前表选项兼容 | warning | **是** |

### 存在性检查规则（11 条 — 元数据支撑）

这些规则验证 `ALTER TABLE` 语句引用的对象在实时 schema 中确实存在（或确实不存在）。在离线审计期间，这些规则将静默跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.table.exists.alter.require` | ALTER TABLE 目标表必须存在 | blocker | **是** |
| `ddl.table.exists.create.forbid` | CREATE TABLE 目标表必须尚不存在 | blocker | **是** |
| `ddl.alter.add_column.exists.forbid` | ADD COLUMN 目标列必须尚不存在 | blocker | **是** |
| `ddl.alter.drop_column.exists.require` | DROP COLUMN 目标列必须存在 | blocker | **是** |
| `ddl.alter.modify_column.exists.require` | MODIFY COLUMN 目标列必须存在 | blocker | **是** |
| `ddl.alter.change_column.exists.require` | CHANGE COLUMN 源列必须存在 | blocker | **是** |
| `ddl.alter.rename_column.exists.require` | RENAME COLUMN 源列必须存在 | blocker | **是** |
| `ddl.alter.add_index.exists.forbid` | ADD INDEX 名称必须尚不存在 | blocker | **是** |
| `ddl.alter.drop_index.exists.require` | DROP INDEX 目标索引必须存在 | blocker | **是** |
| `ddl.alter.rename_index.exists.require` | RENAME INDEX 源索引必须存在 | blocker | **是** |
| `ddl.alter.drop_primary_key.exists.require` | DROP PRIMARY KEY 要求主键存在 | blocker | **是** |

---

## DDL：对象生命周期规则（8 条）

这些规则约束 `DROP TABLE` 和 `TRUNCATE TABLE` 操作。

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

## DDL：全局规则（2 条）

全局规则在所有语句级规则执行完毕后，对**批次中的全部语句**进行跨语句评估。单条语句无法单独触发这类规则。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.alter.merge.mysql.require` | 对同一张表的多条 ALTER TABLE 语句应合并（MySQL） | warning | 否 |
| `ddl.alter.merge.tidb.require` | 对同一张表的多条 ALTER TABLE 语句指导建议（TiDB，默认禁用） | warning | 否 |

> **说明：** `ddl.alter.merge.mysql.require` 在同一输入中存在两条或更多针对同一张表的 `ALTER TABLE` 语句时触发。在 MySQL 中，每条 `ALTER TABLE` 都会引发表重建；将它们合并为一条语句可大幅减少停机时间。在 TiDB 中，多条 ALTER 通常开销较小，因此 `ddl.alter.merge.tidb.require` 在默认策略中处于禁用状态。

---

## DML 规则（10 条）

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

---

## 离线规则与元数据支撑规则

规则分为两类，取决于是否需要实时数据库连接：

**离线规则**始终运行，即使未提供数据库连接。它们仅基于 SQL 文本和 AST 进行评估。所有未在下方列为元数据支撑的规则均属于离线规则。

**元数据支撑规则**在没有活跃 `MetadataProvider` 时静默跳过。在离线审计期间，这些规则不会产生任何发现——不报错、不产生误报。仅当向 `deltascope audit` 提供连接标志时，这些规则才会激活。

### 元数据支撑规则 ID 完整列表

| 规则 ID |
|---------|
| `ddl.table.row_size.max_bytes.require` |
| `ddl.index.key_length.max_bytes.require` |
| `ddl.alter.modify_column.compatibility.require` |
| `ddl.alter.change_column.compatibility.require` |
| `ddl.alter.modify_column.explicit_nullability_change.forbid` |
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
| `ddl.alter.rename_column.exists.require` |
| `ddl.alter.add_index.exists.forbid` |
| `ddl.alter.drop_index.exists.require` |
| `ddl.alter.rename_index.exists.require` |
| `ddl.alter.drop_primary_key.exists.require` |
| `ddl.table.drop.exists.require` |
| `ddl.table.drop.adaptive_hash.warn` |
| `ddl.table.drop.rows.max_count` |
| `ddl.table.truncate.exists.require` |
| `ddl.table.truncate.adaptive_hash.warn` |
| `ddl.table.truncate.rows.max_count` |
| `ddl.table.denylist.forbid` |
| `dml.table.denylist.forbid` |

---

## 参考链接

- **参数文档** — [config.zh-CN.md](config.zh-CN.md)
- **规则评估概念概述** — [../concept/core-concepts.zh-CN.md](../concept/core-concepts.zh-CN.md)
- **元数据感知模式** — [../concept/metadata-aware-mode.zh-CN.md](../concept/metadata-aware-mode.zh-CN.md)
- **CLI 用法** — [cli.zh-CN.md](cli.zh-CN.md)
- **能力矩阵** — [audit-capability-matrix.zh-CN.md](audit-capability-matrix.zh-CN.md)
