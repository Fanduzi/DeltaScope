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

```text
# DeltaScope Rules

RULE ID                              LEVEL    KIND  SUMMARY
-----------------------------------  -------  ----  ----------------------------------------------
ddl.table.comment.require           warning  ddl   Require DDL table comment require
ddl.table.row_size.max_bytes.require  blocker  ddl   Require DDL table row size max bytes require
dml.limit.forbid                    warning  dml   Forbid DML limit forbid
dml.where.require                   blocker  dml   Require DML where require
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

**PostgreSQL 主键可用性（v0.37.0、v0.39.0）：** `ddl.table.primary_key.bigint.require` 和 `ddl.table.primary_key.columns.max_count` 现在适用于 PostgreSQL `CREATE TABLE` 语句（v0.37.0）和 `ALTER TABLE ... ADD CONSTRAINT ... PRIMARY KEY` 语句（v0.39.0）。`ddl.table.primary_key.not_null.require` 对 PostgreSQL 不产生稳定负例，因为 PK 列被有效视为 NOT NULL。

**默认策略方言隔离（v0.43.0）：** 从 v0.43.0 开始，设置 `--dialect postgresql` 时自动跳过 MySQL-family 规则，包括 `ddl.table.primary_key.unsigned.require`、`ddl.table.primary_key.auto_increment.require`、`ddl.table.primary_key.not_null.require`、`ddl.table.engine.allowlist`、`ddl.table.charset.allowlist`、`ddl.table.row_format.allowlist`、`ddl.table.auto_increment.init_value.require`、`ddl.table.partition.forbid`、`ddl.table.create_as.forbid`、`ddl.table.create_like.forbid`、`ddl.column.charset.allowlist`、`ddl.column.collation.allowlist`、`ddl.column.charset_collation.match.require`、`ddl.alter.change_column.forbid`、`ddl.alter.modify_column.forbid`，以及审计列检查中的 `ON UPDATE CURRENT_TIMESTAMP` 建议。反过来，MySQL/TiDB 审核排除所有 `ddl.pg.*` 和 PostgreSQL-only 方言门控规则。隔离在规则 `AppliesTo` 门控层实现。
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

**PostgreSQL 索引可用性（v0.38.0）：** `ddl.index.secondary.prefix.require`、`ddl.index.unique.prefix.require` 和 `ddl.index.columns.max_count` 现在也适用于独立的 PostgreSQL `CREATE INDEX`、`CREATE UNIQUE INDEX` 和 `CREATE INDEX CONCURRENTLY` 语句（仅限 btree）。Partial index、expression index、INCLUDE、operator class、非 btree 访问方法和 NULLS NOT DISTINCT 仍不在 scope 内。

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

**PostgreSQL ALTER TABLE ADD CONSTRAINT（v0.39.0）：** `ddl.alter.add_index.unique.prefix.require` 现在也适用于 PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... UNIQUE` 形式。`ddl.table.primary_key.bigint.require` 和 `ddl.table.primary_key.columns.max_count` 现在也适用于 PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... PRIMARY KEY`。这些规则复用已有的共享 alter-table 索引和主键规则族——未新增规则 ID。

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

## DDL：PostgreSQL 迁移安全规则（9 条）

这些规则用于防范常见的 PostgreSQL 迁移模式，避免引发全表重写、长时间持锁或生产事故。仅在设置 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.create_index.concurrently.require` | `CREATE INDEX` 必须使用 `CONCURRENTLY` 以避免阻塞读写 | warning | 否 |
| `ddl.pg.alter.add_column.non_null_default.rewrite.warn` | 添加带有 volatile 默认值的 `NOT NULL` 列可能触发全表重写 | warning | 否 |
| `ddl.pg.alter.add_check.not_valid.require` | `ADD CHECK` 约束应使用 `NOT VALID` 以避免持 `ACCESS EXCLUSIVE` 锁的全表扫描 | warning | 否 |
| `ddl.pg.alter.set_data_type.rewrite.warn` | 更改列类型可能需要全表重写（取决于类型转换） | warning | 否 |
| `ddl.pg.alter.not_valid_constraint.validate.require` | 命名 CHECK/FK `NOT VALID` 约束在同一次审计 SQL 批次中缺少后续匹配的 `VALIDATE CONSTRAINT` | warning | 否 |
| `ddl.pg.drop_index.advisory` | `DROP INDEX` 移除索引，建议审查依赖查询 | notice | 否 |
| `ddl.pg.alter.add_column.non_null_no_default.warn` | 添加 `NOT NULL` 列但未指定 `DEFAULT`，可能导致大表全表重写 | warning | 否 |
| `ddl.pg.alter.add_unique_constraint.concurrent_index.advisory` | `ADD UNIQUE CONSTRAINT` 不含 `NOT VALID` 且后续没有 `CREATE UNIQUE INDEX CONCURRENTLY`，建议使用并发索引创建以实现零停机部署 | notice | 否 |
| `ddl.pg.alter.drop_constraint.advisory` | `DROP CONSTRAINT` 移除 CHECK、UNIQUE 或 FOREIGN KEY 约束，建议审查依赖查询和数据完整性影响 | notice | 否 |

> **说明：** 这些规则是 PostgreSQL 专用的，审计 MySQL 或 TiDB SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。从 `v0.41.0` 开始，`ddl.pg.alter.add_check.not_valid.require` 也对 `ALTER TABLE ... ADD CONSTRAINT ... CHECK` 语句触发。从 `v0.42.0` 开始，`ddl.pg.alter.not_valid_constraint.validate.require` 对命名 CHECK 和 FOREIGN KEY `NOT VALID` 约束执行同批次校验配对检查。CHECK 命名规则（`ddl.constraint.check.name.prefix.require`、`ddl.constraint.check.name.suffix.require`、`ddl.constraint.check.name.contains.require`）在配置后同样覆盖 ALTER TABLE CHECK 路径。

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

## 信任与误配防护（非规则行为）

v0.20.0 引入了增量行为，帮助识别方言误配和未支持的功能面。这些**不是**可配置的规则，在策略 YAML 中没有条目，不能被禁用或调整级别。

| 行为 | 类规则 ID | 说明 |
|------|----------|------|
| PostgreSQL 语法启发式通知 | `dialect.postgresql.syntax.detected.notice` | 在 MySQL/TiDB 路径审计时，检测到常见 PG 专属语法标记（`RETURNING`、`ON CONFLICT`、`::`、`ALTER COLUMN TYPE USING`、`GENERATED AS IDENTITY`）后作为全局建议性告警发出。DeltaScope 不会自动切换方言。 |
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
