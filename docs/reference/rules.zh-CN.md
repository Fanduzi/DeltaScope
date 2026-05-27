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

**PostgreSQL 索引可用性（v0.38.0，v0.49.0 更新）：** `ddl.index.secondary.prefix.require`、`ddl.index.unique.prefix.require` 和 `ddl.index.columns.max_count` 现在也适用于独立的 PostgreSQL `CREATE INDEX`、`CREATE UNIQUE INDEX` 和 `CREATE INDEX CONCURRENTLY` 语句。自 v0.49.0 起，partial index、expression index、INCLUDE 覆盖索引和非 btree 访问方法（GIN、hash 等）走规范化路径而非返回 unsupported。Operator class 和 NULLS NOT DISTINCT 仍不在 scope 内。

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

## DDL：MySQL/TiDB Database/Schema 生命周期规则（2 条）

这些规则用于防范 MySQL 和 TiDB database/schema 生命周期 DDL 操作——`CREATE DATABASE`、`CREATE SCHEMA`、`DROP DATABASE`、`DROP SCHEMA`。在 MySQL/TiDB 中，`SCHEMA` 是 `DATABASE` 的同义词。仅在设置 `--dialect mysql` 或 `--dialect tidb` 时生效，PostgreSQL 方言下自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.database.create.notice` | `CREATE DATABASE` / `CREATE SCHEMA` 创建新的逻辑命名空间——信息性通知 | notice | 否 |
| `ddl.database.drop.warn` | `DROP DATABASE` / `DROP SCHEMA` 移除数据库及其包含的所有对象——应当审查 | warning | 否 |

> **说明：** 这些规则是 MySQL/TiDB 专用的，审计 PostgreSQL SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。`CREATE DATABASE IF NOT EXISTS` 和 `CREATE SCHEMA IF NOT EXISTS` 仍然会发出 notice。`DROP DATABASE IF EXISTS` 和 `DROP SCHEMA IF EXISTS` 仍然会发出 warning。DeltaScope 不执行在线数据库存在性验证。`CREATE DATABASE ... CHARACTER SET` / `COLLATE` 选项作为解析器事实保留，但本里程碑中无策略规则对其进行治理。这不是完整的 DDL 支持——trigger、routine、event 和数据库权限生命周期均推迟支持。

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

> **说明：** 这些规则是 MySQL/TiDB 专用的，审计 PostgreSQL SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。这些仅是信息性 notice 规则——不会阻止、警告或执行在线验证。`ddl.drop_resource_group.notice` 仅限 MySQL，TiDB 下自动跳过。TiDB placement policy 和 sequence 规则在 MySQL 下自动跳过。这不是完整的 MySQL/TiDB DDL 支持——triggers、events、functions、ALTER VIEW、ALTER PROCEDURE、tablespace 和 parser-error 形态仍推迟支持。

---

## DDL：PostgreSQL 迁移安全规则（9 条）

这些规则用于防范常见的 PostgreSQL 迁移模式，避免引发全表重写、长时间持锁或生产事故。仅在设置 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。

| 规则 ID | 描述 | 默认级别 | 是否需要元数据 |
|---------|------|:--------:|:--------------:|
| `ddl.pg.create_index.concurrently.require` | `CREATE INDEX` 必须使用 `CONCURRENTLY` 以避免阻塞读写。发现包含有界索引形态元数据：`index_kind`、`access_method`、`column_count`、`included_column_count`、`has_predicate`、`has_expression_keys`、`expression_count`。元数据为增量添加，不输出谓词或表达式 SQL 文本。 | warning | 否 |
| `ddl.pg.alter.add_column.non_null_default.rewrite.warn` | 添加带有 volatile 默认值的 `NOT NULL` 列可能触发全表重写。发现包含：`not_null`、`has_default`、`default_kind`（`literal`、`null`、`function_call`、`expression`、`unknown`）。元数据不输出默认表达式文本或函数名。 | warning | 否 |
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

## DDL：PostgreSQL ALTER TABLE 覆盖规则（32 条）

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
| `ddl.pg.alter.set_tablespace.notice` | `ALTER TABLE ... SET TABLESPACE` 将表移至不同表空间——信息性提示 | notice | 否 |
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
| `ddl.pg.alter.move_all_tablespace.notice` | `ALTER TABLE ALL IN TABLESPACE ... SET TABLESPACE ...` 移动表空间中的所有表——信息性提示 | notice | 否 |

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
