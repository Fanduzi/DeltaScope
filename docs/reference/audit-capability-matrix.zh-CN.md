# 审计能力矩阵

本矩阵列出了 DeltaScope 内置的所有规则，包括规则 ID、是否支持离线运行、是否需要元数据，以及默认的发现级别。通过本文档，您可以了解 DeltaScope 在特定 SQL 语句和审计配置下会检查哪些内容。

**离线（Offline）** 规则仅依赖 SQL 文本即可触发，无需数据库连接。**元数据感知（Metadata）** 规则在配置了元数据提供者时，会额外读取实时的 Schema 或实例信息；未配置元数据提供者时，这些规则将被静默跳过。

**Pattern legality checks**（例如 `*.name.pattern.require`、`*.name.keyword.forbid`）用于约束词法合法性。**Structured naming governance**（例如 `prefix`、`suffix`、`contains`）用于约束团队命名约定。这两层能力互补，不互相替代。

---

## DDL：CREATE TABLE

### 表级检查

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.table.name.max_length` | 表名超过允许的最大长度 | ✓ | ✗ | warning |
| `ddl.table.name.prefix.require` | 表名未以要求的 structured naming 前缀开头 | ✓ | ✗ | warning |
| `ddl.table.name.suffix.require` | 表名未以要求的 structured naming 后缀结尾 | ✓ | ✗ | warning |
| `ddl.table.name.contains.require` | 表名未包含任一已配置的 structured naming token（OR 语义） | ✓ | ✗ | warning |
| `ddl.table.name.pattern.require` | 表名不符合要求的命名规范 | ✓ | ✗ | warning |
| `ddl.table.name.keyword.forbid` | 表名是 SQL 保留关键字 | ✓ | ✗ | blocker |
| `ddl.table.comment.require` | 表缺少 COMMENT 子句 | ✓ | ✗ | warning |
| `ddl.table.comment.max_length` | 表注释超过允许的最大长度 | ✓ | ✗ | warning |
| `ddl.table.engine.allowlist` | 存储引擎不在允许列表中 | ✓ | ✗ | blocker |
| `ddl.table.charset.allowlist` | 表字符集不在允许列表中 | ✓ | ✗ | blocker |
| `ddl.table.row_format.allowlist` | ROW_FORMAT 值不在允许列表中 | ✓ | ✗ | warning |
| `ddl.table.auto_increment.init_value.require` | AUTO_INCREMENT 初始值不满足要求的最小值 | ✓ | ✗ | warning |
| `ddl.table.columns.min_count` | 表的列数少于要求的最小列数 | ✓ | ✗ | blocker |
| `ddl.table.primary_key.require` | 表未定义 PRIMARY KEY | ✓ | ✗ | blocker |
| `ddl.table.primary_key.columns.max_count` | 主键包含的列数超过允许的最大值 | ✓ | ✗ | warning |
| `ddl.table.primary_key.bigint.require` | 主键列类型不是 BIGINT | ✓ | ✗ | warning |
| `ddl.table.primary_key.unsigned.require` | 主键列不是 UNSIGNED | ✓ | ✗ | warning |
| `ddl.table.primary_key.auto_increment.require` | 主键列不是 AUTO_INCREMENT | ✓ | ✗ | warning |
| `ddl.table.primary_key.not_null.require` | 主键列允许为 NULL | ✓ | ✗ | blocker |
| `ddl.table.audit_columns.require` | 缺少必要的审计时间戳列（如 `created_at`、`updated_at`） | ✓ | ✗ | warning |
| `ddl.table.create_as.forbid` | 不允许使用 CREATE TABLE … AS SELECT | ✓ | ✗ | blocker |
| `ddl.table.create_like.forbid` | 不允许使用 CREATE TABLE … LIKE | ✓ | ✗ | blocker |
| `ddl.table.foreign_key.forbid` | 不允许使用外键约束 | ✓ | ✗ | blocker |
| `ddl.table.partition.forbid` | 不允许使用分区表 | ✓ | ✗ | blocker |
| `ddl.table.row_size.max_bytes.require` | 根据实例行格式估算，行大小超过 InnoDB 行大小限制 | ✗ | ✓ | warning |
| `ddl.table.denylist.forbid` | 表名匹配 Schema 级或表级拒绝列表中的条目 | ✓ | ✗ | blocker |

### 列级检查

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.column.name.max_length` | 列名超过允许的最大长度 | ✓ | ✗ | warning |
| `ddl.column.name.prefix.require` | 列名未以要求的 structured naming 前缀开头 | ✓ | ✗ | warning |
| `ddl.column.name.suffix.require` | 列名未以要求的 structured naming 后缀结尾 | ✓ | ✗ | warning |
| `ddl.column.name.contains.require` | 列名未包含任一已配置的 structured naming token（OR 语义） | ✓ | ✗ | warning |
| `ddl.column.name.pattern.require` | 列名不符合要求的命名规范 | ✓ | ✗ | warning |
| `ddl.column.name.keyword.forbid` | 列名是 SQL 保留关键字 | ✓ | ✗ | blocker |
| `ddl.column.comment.require` | 列缺少 COMMENT 子句 | ✓ | ✗ | warning |
| `ddl.column.default.require` | 列未定义 DEFAULT 值 | ✓ | ✗ | warning |
| `ddl.column.not_null.require` | 列允许为 NULL（缺少 NOT NULL） | ✓ | ✗ | warning |
| `ddl.column.varchar.max_length` | VARCHAR 长度超过允许的最大值 | ✓ | ✗ | warning |
| `ddl.column.char.max_length` | CHAR 长度超过建议的最大值 | ✓ | ✗ | notice |
| `ddl.column.float_double.forbid` | 不允许使用 FLOAT 或 DOUBLE 类型，建议使用 DECIMAL | ✓ | ✗ | blocker |
| `ddl.column.blob_text.forbid` | 不允许使用 BLOB 或 TEXT 类型 | ✓ | ✗ | blocker |
| `ddl.column.json.forbid` | 不允许使用 JSON 类型 | ✓ | ✗ | blocker |
| `ddl.column.bit.forbid` | 不允许使用 BIT 类型 | ✓ | ✗ | blocker |
| `ddl.column.timestamp.forbid` | 不允许使用 TIMESTAMP 类型，建议使用 DATETIME | ✓ | ✗ | warning |
| `ddl.column.charset.allowlist` | 列字符集不在允许列表中 | ✓ | ✗ | blocker |
| `ddl.column.collation.allowlist` | 列排序规则不在允许列表中 | ✓ | ✗ | blocker |
| `ddl.column.charset_collation.match.require` | 列的字符集与排序规则不兼容 | ✓ | ✗ | blocker |

### 索引级检查

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.index.total.max_count` | 表的索引数量超过允许的最大值 | ✓ | ✗ | warning |
| `ddl.index.columns.max_count` | 某个索引包含的列数超过允许的最大值 | ✓ | ✗ | warning |
| `ddl.index.name.pattern.require` | 索引名不符合要求的词法命名模式 | ✓ | ✗ | warning |
| `ddl.index.name.keyword.forbid` | 索引名是 SQL 保留关键字 | ✓ | ✗ | blocker |
| `ddl.index.unique.prefix.require` | 唯一索引名未以要求的前缀开头 | ✓ | ✗ | warning |
| `ddl.index.unique.suffix.require` | 唯一索引名未以要求的 structured naming 后缀结尾 | ✓ | ✗ | warning |
| `ddl.index.unique.contains.require` | 唯一索引名未包含任一已配置的 structured naming token（OR 语义） | ✓ | ✗ | warning |
| `ddl.index.secondary.prefix.require` | 普通（非唯一）索引名未以要求的前缀开头 | ✓ | ✗ | warning |
| `ddl.index.secondary.suffix.require` | 普通（非唯一）索引名未以要求的 structured naming 后缀结尾 | ✓ | ✗ | warning |
| `ddl.index.secondary.contains.require` | 普通（非唯一）索引名未包含任一已配置的 structured naming token（OR 语义） | ✓ | ✗ | warning |
| `ddl.index.fulltext.prefix.require` | 全文索引名未以要求的前缀开头 | ✓ | ✗ | warning |
| `ddl.index.fulltext.suffix.require` | 全文索引名未以要求的 structured naming 后缀结尾 | ✓ | ✗ | warning |
| `ddl.index.fulltext.contains.require` | 全文索引名未包含任一已配置的 structured naming token（OR 语义） | ✓ | ✗ | warning |
| `ddl.index.duplicate.forbid` | 两个或多个索引覆盖完全相同的列集合 | ✓ | ✗ | warning |
| `ddl.index.redundant_left_prefix.forbid` | 某个索引是另一个索引的左前缀子集，因此冗余 | ✓ | ✗ | warning |
| `ddl.index.redundant_unique_overlap.forbid` | 某个非唯一索引被重叠的唯一索引覆盖，因此冗余 | ✓ | ✗ | warning |
| `ddl.index.key_length.max_bytes.require` | 根据实例的 `innodb_large_prefix` 设置，索引键长度超过 InnoDB 限制 | ✗ | ✓ | warning |

**PostgreSQL 索引可用性（v0.38.0，v0.49.0 更新）：** `ddl.index.secondary.prefix.require`、`ddl.index.unique.prefix.require` 和 `ddl.index.columns.max_count` 现在也适用于独立的 PostgreSQL `CREATE INDEX`、`CREATE UNIQUE INDEX` 和 `CREATE INDEX CONCURRENTLY` 语句。自 v0.49.0 起，partial index、expression index、INCLUDE 覆盖索引和非 btree 访问方法（GIN、hash 等）走规范化路径而非返回 unsupported。Operator class 和 NULLS NOT DISTINCT 仍不在 scope 内。

**PostgreSQL ALTER TABLE 约束可用性 (v0.39.0)：** `ALTER TABLE ... ADD PRIMARY KEY`、`ADD CONSTRAINT ... PRIMARY KEY`、`ADD UNIQUE` 和 `ADD CONSTRAINT ... UNIQUE` 形态现在保留语句级约束元数据。已有的主键规则（`ddl.table.primary_key.bigint.require`、`ddl.table.primary_key.columns.max_count`）和唯一前缀规则（`ddl.alter.add_index.unique.prefix.require`）可以对已批准形态产生 findings。外键、CHECK 约束、排他约束、可延迟性、验证生命周期、partial/expression index 语义、operator class 和在线 schema 重建不在范围内。

**PostgreSQL ALTER TABLE FK 可用性 (v0.40.0)：** `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` 形态现在保留语句级 FK 事实（本地列、被引用表、被引用列、schema-qualified 引用时保留 referenced_schema）。已有的 FK 规则（`ddl.table.foreign_key.forbid`、`ddl.pg.table.foreign_key.cross_schema.advisory`）可以对 ALTER TABLE FK 添加产生 findings。CHECK 约束、排他约束、可延迟性、MATCH FULL 策略、在线 schema FK 存在性验证和完整约束/索引对等不在范围内。

**PostgreSQL ALTER TABLE CHECK 可用性 (v0.41.0)：** `ALTER TABLE ... ADD CONSTRAINT ... CHECK` 形态现在保留语句级 CHECK 约束元数据（约束名称、CHECK 表达式）。已有的 CHECK 命名规则（`ddl.constraint.check.name.prefix.require`、`ddl.constraint.check.name.suffix.require`、`ddl.constraint.check.name.contains.require`）和 PostgreSQL `NOT VALID` 建议规则（`ddl.pg.alter.add_check.not_valid.require`）可以对 ALTER TABLE CHECK 添加产生 findings。排他约束、可延迟性、`NOT VALID` 校验强制、在线 schema CHECK 存在性验证和完整约束/索引对等不在范围内。

**PostgreSQL NOT VALID 校验配对 (v0.42.0)：** 命名的 PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... NOT VALID` CHECK 和 FOREIGN KEY 添加现在会参与批次级 GlobalRule（`ddl.pg.alter.not_valid_constraint.validate.require`）。当同一次审计 SQL 批次中没有后续匹配的 `ALTER TABLE ... VALIDATE CONSTRAINT ...`（匹配键为相同 schema、table 和 constraint name）时，DeltaScope 会发出 warning。这不是首次支持 `VALIDATE CONSTRAINT` 解析，不查询 live validation state，不追踪跨文件/跨发布窗口，跳过未命名约束，也不改变 MySQL/TiDB 行为。

**默认策略方言隔离（v0.43.0）：** 从 v0.43.0 开始，默认策略按 `--dialect` 隔离规则。设置 `--dialect postgresql` 时，MySQL/TiDB-only 规则（engine、charset、row format、unsigned/auto_increment 主键要求、partition、create_as/create_like、column charset/collation、change/modify column）和 MySQL-only 修复建议文本（`UNSIGNED`、`AUTO_INCREMENT`、`ON UPDATE CURRENT_TIMESTAMP`）被排除。设置 `--dialect mysql` 或 `--dialect tidb` 时，`ddl.pg.*` 和 PostgreSQL-only 方言门控规则被排除。隔离在规则 `AppliesTo` 门控层实现，不是后期过滤。

### 约束级检查

约束的 structured naming governance 只针对显式命名对象生效。未命名约束和隐式名称会被跳过。外键命名规则只在策略允许外键存在时才有意义；在内置默认 baseline 下，`ddl.table.foreign_key.forbid` 会抑制外键命名检查。

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.constraint.primary_key.name.prefix.require` | 显式命名的主键约束未以要求的 structured naming 前缀开头 | ✓ | ✗ | warning |
| `ddl.constraint.primary_key.name.suffix.require` | 显式命名的主键约束未以要求的 structured naming 后缀结尾 | ✓ | ✗ | warning |
| `ddl.constraint.primary_key.name.contains.require` | 显式命名的主键约束未包含任一已配置的 structured naming token（OR 语义） | ✓ | ✗ | warning |
| `ddl.constraint.unique_key.name.prefix.require` | 显式命名的唯一键约束未以要求的 structured naming 前缀开头 | ✓ | ✗ | warning |
| `ddl.constraint.unique_key.name.suffix.require` | 显式命名的唯一键约束未以要求的 structured naming 后缀结尾 | ✓ | ✗ | warning |
| `ddl.constraint.unique_key.name.contains.require` | 显式命名的唯一键约束未包含任一已配置的 structured naming token（OR 语义） | ✓ | ✗ | warning |
| `ddl.constraint.foreign_key.name.prefix.require` | 显式命名的外键约束未以要求的 structured naming 前缀开头 | ✓ | ✗ | warning |
| `ddl.constraint.foreign_key.name.suffix.require` | 显式命名的外键约束未以要求的 structured naming 后缀结尾 | ✓ | ✗ | warning |
| `ddl.constraint.foreign_key.name.contains.require` | 显式命名的外键约束未包含任一已配置的 structured naming token（OR 语义） | ✓ | ✗ | warning |
| `ddl.constraint.check.name.prefix.require` | 显式命名的 CHECK 约束未以要求的 structured naming 前缀开头 | ✓ | ✗ | warning |
| `ddl.constraint.check.name.suffix.require` | 显式命名的 CHECK 约束未以要求的 structured naming 后缀结尾 | ✓ | ✗ | warning |
| `ddl.constraint.check.name.contains.require` | 显式命名的 CHECK 约束未包含任一已配置的 structured naming token（OR 语义） | ✓ | ✗ | warning |

### 其他 CREATE TABLE 检查

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.view.create.forbid` | 不允许使用 CREATE VIEW | ✓ | ✗ | blocker |

---

## DDL：ALTER TABLE

### 结构性检查

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.alter.drop_column.forbid` | 不允许使用 ALTER TABLE … DROP COLUMN | ✓ | ✗ | blocker |
| `ddl.alter.drop_index.forbid` | 不允许使用 ALTER TABLE … DROP INDEX | ✓ | ✗ | blocker |
| `ddl.alter.drop_primary_key.forbid` | 不允许使用 ALTER TABLE … DROP PRIMARY KEY | ✓ | ✗ | blocker |
| `ddl.alter.rename_table.forbid` | 不允许通过 ALTER TABLE 重命名表 | ✓ | ✗ | blocker |
| `ddl.alter.rename_column.forbid` | 不允许使用 ALTER TABLE … RENAME COLUMN | ✓ | ✗ | blocker |
| `ddl.alter.rename_index.forbid` | 不允许使用 ALTER TABLE … RENAME INDEX | ✓ | ✗ | blocker |
| `ddl.alter.change_column.forbid` | 不允许使用 ALTER TABLE … CHANGE COLUMN | ✓ | ✗ | blocker |
| `ddl.alter.modify_column.forbid` | 不允许使用 ALTER TABLE … MODIFY COLUMN | ✓ | ✗ | blocker |

### 类型兼容性检查

以下规则在 CHANGE COLUMN 或 MODIFY COLUMN 操作未被全局禁止、且存在当前列类型的元数据快照时触发。

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.alter.table_option.compatibility.require` | 表选项变更（如字符集）与当前表状态不兼容 | ✗ | ✓ | blocker |

### ALTER 路径上的索引检查

ALTER 路径的索引检查复用 CREATE TABLE 中的相同逻辑。

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.index.duplicate.forbid` | 新增索引与现有索引或同时新增的索引重复 | ✓ / ✓ | ✓ | warning |
| `ddl.index.redundant_left_prefix.forbid` | 新增索引是现有索引或同时新增索引的左前缀子集 | ✓ / ✓ | ✓ | warning |
| `ddl.index.redundant_unique_overlap.forbid` | 新增索引被重叠的唯一索引覆盖而冗余 | ✓ / ✓ | ✓ | warning |

### 存在性检查（仅元数据感知模式）

以下规则需要实时表快照，离线模式下将被跳过。

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.alter.column.add.exists` | 待添加的列在当前 Schema 中已存在 | ✗ | ✓ | blocker |
| `ddl.alter.column.drop.exists` | 待删除的列在当前 Schema 中不存在 | ✗ | ✓ | blocker |
| `ddl.alter.column.modify.exists` | 待修改的列在当前 Schema 中不存在 | ✗ | ✓ | blocker |
| `ddl.alter.column.change.exists` | 待变更的列在当前 Schema 中不存在 | ✗ | ✓ | blocker |
| `ddl.alter.column.rename.exists` | 待重命名的列在当前 Schema 中不存在 | ✗ | ✓ | blocker |
| `ddl.alter.index.add.exists` | 待添加的索引在当前 Schema 中已存在 | ✗ | ✓ | blocker |
| `ddl.alter.index.drop.exists` | 待删除的索引在当前 Schema 中不存在 | ✗ | ✓ | blocker |
| `ddl.alter.index.rename.exists` | 待重命名的索引在当前 Schema 中不存在 | ✗ | ✓ | blocker |
| `ddl.alter.primary_key.drop.exists` | 待删除的主键在表上不存在 | ✗ | ✓ | blocker |

### 全局规则：合并 ALTER

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.alter.merge.mysql` | 针对同一张表的多条 ALTER TABLE 语句应合并为一条（MySQL） | ✓ | ✗ | warning |
| `ddl.alter.merge.tidb` | 针对同一张表的多条 ALTER TABLE 语句应合并为一条（TiDB） | ✓ | ✗ | notice |

---

## DDL：对象生命周期

### DROP TABLE

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.table.drop.forbid` | 不允许使用 DROP TABLE | ✓ | ✗ | blocker |
| `ddl.table.drop.exists` | 待删除的表在当前 Schema 中不存在 | ✗ | ✓ | warning |
| `ddl.table.drop.row_count` | 表的行数超过配置的安全阈值 | ✗ | ✓ | warning |
| `ddl.table.drop.adaptive_hash` | 表启用了 `innodb_adaptive_hash_index`，删除可能导致延迟抖动 | ✗ | ✓ | notice |

### TRUNCATE TABLE

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.table.truncate.forbid` | 不允许使用 TRUNCATE TABLE | ✓ | ✗ | blocker |
| `ddl.table.truncate.exists` | 待截断的表在当前 Schema 中不存在 | ✗ | ✓ | warning |
| `ddl.table.truncate.row_count` | 表的行数超过配置的安全阈值 | ✗ | ✓ | warning |
| `ddl.table.truncate.adaptive_hash` | 表启用了 `innodb_adaptive_hash_index`，截断可能导致延迟抖动 | ✗ | ✓ | notice |

---

## DDL：MySQL/TiDB Database/Schema 生命周期（v0.64.0）

`v0.64.0` 将 MySQL/TiDB database 和 schema 生命周期 DDL——`CREATE DATABASE`、`CREATE SCHEMA`、`DROP DATABASE`、`DROP SCHEMA`——通过审核管线进行标准化处理，不再静默通过。在 MySQL/TiDB 中，`SCHEMA` 是 `DATABASE` 的同义词。两条新规则提供 notice 和 warning 覆盖。仅在设置 `--dialect mysql` 或 `--dialect tidb` 时生效。

### 标准化操作

| MySQL/TiDB DDL 动作 | 标准化为 | 已支持 | 可审计 | 规则映射 |
|---------------------|---------|:------:|:------:|:--------:|
| `CREATE DATABASE name` | `create_schema` (object_type=database) | ✓ | ✓ | ✓ |
| `CREATE DATABASE IF NOT EXISTS name` | `create_schema` (object_type=database, if_not_exists=true) | ✓ | ✓ | ✓ |
| `CREATE SCHEMA name` | `create_schema` (object_type=database) | ✓ | ✓ | ✓ |
| `DROP DATABASE name` | `drop_schema` (object_type=database) | ✓ | ✓ | ✓ |
| `DROP DATABASE IF EXISTS name` | `drop_schema` (object_type=database, if_exists=true) | ✓ | ✓ | ✓ |
| `DROP SCHEMA name` | `drop_schema` (object_type=database) | ✓ | ✓ | ✓ |

### Database/Schema 生命周期规则

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.database.create.notice` | `CREATE DATABASE` / `CREATE SCHEMA` 创建新的逻辑命名空间——信息性通知 | ✓ | ✗ | notice |
| `ddl.database.drop.warn` | `DROP DATABASE` / `DROP SCHEMA` 移除数据库及其包含的所有对象——应当审查 | ✓ | ✗ | warning |

> **说明：** 这些规则是 MySQL/TiDB 专用的，审计 PostgreSQL SQL 时会自动跳过。它们属于离线规则，不需要数据库连接。DeltaScope 不执行在线数据库存在性验证。`CREATE DATABASE ... CHARACTER SET` / `COLLATE` 选项作为解析器事实保留，但无策略规则对其进行治理。这不是完整的 DDL 支持——trigger、routine、event 和数据库权限生命周期均推迟支持。

---

## DDL：PostgreSQL 迁移安全

这些规则用于防范常见的 PostgreSQL 迁移模式，避免引发全表重写、长时间持锁或生产事故。仅在设置 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。v0.120.0 为选定规则增加有界语义元数据（索引形态、默认值分类、USING 是否存在）；元数据为增量添加，不输出原始 SQL 文本。

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.pg.create_index.concurrently.require` | 不带 `CONCURRENTLY` 的 `CREATE INDEX` 持有排他锁，阻塞读写。发现元数据：`index_kind`、`access_method`、`column_count`、`included_column_count`、`has_predicate`、`has_expression_keys`、`expression_count`（有界，不含 SQL 文本）。 | ✓ | ✗ | warning |
| `ddl.pg.alter.add_column.non_null_default.rewrite.warn` | 添加带 volatile 默认值的 `NOT NULL` 列可能触发全表重写。发现元数据：`not_null`、`has_default`、`default_kind`（有界分类，不含表达式文本）。 | ✓ | ✗ | warning |
| `ddl.pg.alter.add_check.not_valid.require` | 不带 `NOT VALID` 的 `ADD CHECK` 需要持 `ACCESS EXCLUSIVE` 锁的全表扫描 | ✓ | ✗ | warning |
| `ddl.pg.alter.set_data_type.rewrite.warn` | 更改列类型可能需要全表重写（取决于类型转换）。发现元数据：`has_using`（布尔值，不含 USING 表达式文本）。 | ✓ | ✗ | warning |
| `ddl.pg.alter.not_valid_constraint.validate.require` | 命名 CHECK/FK `NOT VALID` 约束在同一次审计 SQL 批次中缺少后续匹配的 `VALIDATE CONSTRAINT` | ✓ | ✗ | warning |
| `ddl.pg.drop_index.advisory` | `DROP INDEX` 移除索引，建议审查依赖查询 | ✓ | ✗ | notice |
| `ddl.pg.alter.add_column.non_null_no_default.warn` | 添加 `NOT NULL` 列但未指定 `DEFAULT`，可能导致大表全表重写。发现元数据：`not_null`、`has_default`。 | ✓ | ✗ | warning |
| `ddl.pg.alter.add_unique_constraint.concurrent_index.advisory` | `ADD UNIQUE CONSTRAINT` 不含 `NOT VALID` 且后续没有并发索引创建，建议使用并发索引 | ✓ | ✗ | notice |
| `ddl.pg.alter.drop_constraint.advisory` | `DROP CONSTRAINT` 移除 CHECK/UNIQUE/FK 约束，建议审查数据完整性 | ✓ | ✗ | notice |

---

## DDL：PostgreSQL 对象生命周期（v0.50.0，v0.64.0 更新）

`v0.50.0` 将 PostgreSQL 对象生命周期 DDL——schema、sequence 和 materialized view 的 CREATE/DROP/ALTER 操作——通过审核管线进行标准化处理，不再返回 unsupported。九条 PostgreSQL-only 规则防护级联删除、序列值回绕和序列计数器重置。`v0.64.0` 新增 `ddl.pg.create_schema.notice` 用于 `CREATE SCHEMA` 覆盖。仅在设置 `--dialect postgresql` 时生效。

### 标准化操作

| PostgreSQL DDL 动作 | 标准化为 | 已支持 | 可审计 | 规则映射 |
|---------------------|---------|:------:|:------:|:--------:|
| `CREATE SCHEMA` | `create_schema` | ✓ | ✓ | ✓ |
| `DROP SCHEMA` | `drop_schema` | ✓ | ✓ | ✓ |
| `CREATE SEQUENCE` | `create_sequence` | ✓ | ✓ | ✓ |
| `ALTER SEQUENCE` | `alter_sequence` | ✓ | ✓ | ✓ |
| `DROP SEQUENCE` | `drop_sequence` | ✓ | ✓ | ✓ |
| `CREATE MATERIALIZED VIEW` | `create_materialized_view` | ✓ | ✓ | — |
| `DROP MATERIALIZED VIEW` | `drop_materialized_view` | ✓ | ✓ | ✓ |
| `REFRESH MATERIALIZED VIEW` | `refresh_materialized_view` | ✓ | ✓ | ✓ |

### 对象生命周期规则

| 规则 ID | 检查描述 | 离线 | Metadata | 默认级别 |
|---------|---------|:----:|:--------:|---------|
| `ddl.pg.create_schema.notice` | `CREATE SCHEMA` 创建新的命名空间——信息性通知 | ✓ | ✗ | notice |
| `ddl.pg.drop_schema.advisory` | `DROP SCHEMA` 移除 schema，建议审查依赖对象 | ✓ | ✗ | notice |
| `ddl.pg.drop_schema.cascade.warn` | `DROP SCHEMA ... CASCADE` 使用级联删除，可能静默移除依赖对象 | ✓ | ✗ | warning |
| `ddl.pg.create_sequence.cycle.warn` | `CREATE SEQUENCE ... CYCLE` 可能导致序列值回绕 | ✓ | ✗ | warning |
| `ddl.pg.alter_sequence.restart.warn` | `ALTER SEQUENCE ... RESTART` 重置序列计数器，可能与已有行冲突 | ✓ | ✗ | warning |
| `ddl.pg.alter_sequence.cycle.warn` | `ALTER SEQUENCE ... CYCLE` 在已有序列上启用值回绕 | ✓ | ✗ | warning |
| `ddl.pg.drop_sequence.advisory` | `DROP SEQUENCE` 移除序列，建议审查依赖列 | ✓ | ✗ | notice |
| `ddl.pg.drop_sequence.cascade.warn` | `DROP SEQUENCE ... CASCADE` 使用级联删除，可能静默移除依赖对象 | ✓ | ✗ | warning |
| `ddl.pg.drop_materialized_view.advisory` | `DROP MATERIALIZED VIEW` 移除物化视图，建议审查依赖查询 | ✓ | ✗ | notice |
| `ddl.pg.drop_materialized_view.cascade.warn` | `DROP MATERIALIZED VIEW ... CASCADE` 使用级联删除，可能静默移除依赖对象 | ✓ | ✗ | warning |
| `ddl.pg.refresh_materialized_view.concurrently.warn` | 非并发 `REFRESH MATERIALIZED VIEW` 持有排他锁——对默认或显式 `WITH DATA` 刷新发出警告 | ✓ | ✗ | warning |
| `ddl.pg.refresh_materialized_view.no_data.notice` | `REFRESH MATERIALIZED VIEW ... WITH NO DATA` 清空物化视图——下游读取方可能看到空结果 | ✓ | ✗ | notice |

> **说明：** `CONCURRENTLY` 刷新通过两条规则均不产生 finding。`WITH NO DATA` 同时触发两条规则，因为它也是非并发的。这不是 `CONCURRENTLY` 所需的唯一索引在线验证——DeltaScope 不会检查物化视图上是否存在唯一索引。这不是完整的 PostgreSQL 对象生命周期覆盖——剩余 unsupported DDL 形式（trigger、function 等）仍为显式边界。

---

## DDL：PostgreSQL 类型生命周期（v0.55.0）

`v0.55.0` 新增 PostgreSQL 类型生命周期覆盖，支持 enum 类型创建、加值和类型删除。DeltaScope 规范化 `CREATE TYPE ... AS ENUM`、`ALTER TYPE ... ADD VALUE` 和 `DROP TYPE`，新增五条 PostgreSQL-only 发现，并将复合类型和域作为显式不支持边界。这些规则仅在设置 `--dialect postgresql` 时生效。

### 标准化操作

| SQL | 标准化操作 |
|-----|-----------|
| `CREATE TYPE color AS ENUM ('red', 'green', 'blue')` | `create_type` (type_kind=enum, labels=red,green,blue) |
| `ALTER TYPE color ADD VALUE 'yellow'` | `alter_type` (type_kind=enum, action=add_value, value=yellow) |
| `ALTER TYPE color ADD VALUE IF NOT EXISTS 'yellow'` | `alter_type` (type_kind=enum, action=add_value, value=yellow, if_not_exists=true) |
| `ALTER TYPE color ADD VALUE 'yellow' BEFORE 'green'` | `alter_type` (type_kind=enum, action=add_value, value=yellow, placement=before, neighbor=green) |
| `ALTER TYPE color ADD VALUE 'yellow' AFTER 'green'` | `alter_type` (type_kind=enum, action=add_value, value=yellow, placement=after, neighbor=green) |
| `DROP TYPE color` | `drop_type` |
| `DROP TYPE IF EXISTS color CASCADE` | `drop_type` (if_exists=true, cascade=true) |

### 类型生命周期规则

| 规则 ID | 检查描述 | 离线 | Metadata | 默认级别 |
|---------|---------|:----:|:--------:|---------|
| `ddl.pg.create_type.enum.notice` | `CREATE TYPE ... AS ENUM` 引入新的 enum 类型——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter_type.add_value.advisory` | `ALTER TYPE ... ADD VALUE` 向已有 enum 追加值——建议审查应用使用情况 | ✓ | ✗ | warning |
| `ddl.pg.alter_type.add_value.position.notice` | `ALTER TYPE ... ADD VALUE ... BEFORE/AFTER` 定位新 enum 值——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.drop_type.advisory` | `DROP TYPE` 移除用户定义类型——建议审查依赖列和函数 | ✓ | ✗ | warning |
| `ddl.pg.drop_type.cascade.warn` | `DROP TYPE ... CASCADE` 使用级联删除，可能静默移除依赖对象 | ✓ | ✗ | warning |

> **说明：** 这些规则均为离线规则，不需要数据库连接。DeltaScope 不会检查在线依赖对象、验证 enum 值是否已被数据或应用代码使用，也不会建模完整的 PostgreSQL 类型系统语义。这不是完整的 PostgreSQL 类型生命周期覆盖。复合类型现已支持——参见下方 Composite Type Lifecycle。域（`CREATE DOMAIN ...`）已支持——参见下方域生命周期。不影响 MySQL/TiDB 行为。

---

## DDL：PostgreSQL Composite Type Lifecycle（v0.58.0）

`v0.58.0` 新增 PostgreSQL composite type lifecycle 窄支持。DeltaScope 规范化 `CREATE TYPE ... AS (...)`、`ALTER TYPE ... RENAME TO` 和 `ALTER TYPE ... SET SCHEMA`，新增三条 PostgreSQL-only 发现，并将属性级操作（`ADD ATTRIBUTE`、`DROP ATTRIBUTE`、`ALTER ATTRIBUTE ... TYPE`、`RENAME ATTRIBUTE`）作为显式不支持/延迟边界。`DROP TYPE` 复用 v0.55.0 已有规则。这些规则仅在设置 `--dialect postgresql` 时生效。

### 标准化操作

| SQL | 标准化操作 |
|-----|-----------|
| `CREATE TYPE address AS (street text, city text)` | `create_type_composite` |
| `CREATE TYPE qualified.address AS (street text, city text)` | `create_type_composite` |
| `CREATE TYPE address AS (street text COLLATE "C", city text)` | `create_type_composite`（collation 被记录但不做解释） |
| `ALTER TYPE address RENAME TO mailing_address` | `alter_type` (action=rename) |
| `ALTER TYPE address SET SCHEMA archive` | `alter_type` (action=set_schema) |

### Composite Type Lifecycle 规则

| 规则 ID | 检查描述 | 离线 | Metadata | 默认级别 |
|---------|---------|:----:|:--------:|---------|
| `ddl.pg.create_type.composite.notice` | `CREATE TYPE ... AS (...)` 引入新的 composite type——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter_type.composite_rename.notice` | `ALTER TYPE ... RENAME TO` 变更 composite type 名称——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter_type.composite_set_schema.notice` | `ALTER TYPE ... SET SCHEMA` 将 composite type 移至不同 schema——信息性提示 | ✓ | ✗ | notice |

### 不支持/延迟的操作

| SQL | 不支持的特性 |
|-----|------------|
| `ALTER TYPE ... ADD ATTRIBUTE` | `alter_type_add_attribute` |
| `ALTER TYPE ... DROP ATTRIBUTE` | `alter_type_drop_attribute` |
| `ALTER TYPE ... ALTER ATTRIBUTE ... TYPE` | `alter_type_alter_attribute_type` |
| `ALTER TYPE ... RENAME ATTRIBUTE ... TO ...` | `alter_type_rename_attribute` |

> **说明：** 这些规则均为离线规则，不需要数据库连接。`DROP TYPE` 不由 composite-specific 规则覆盖——它复用 Type Lifecycle 规则族中已有的 `ddl.pg.drop_type.advisory` 和 `ddl.pg.drop_type.cascade.warn`。属性级操作明确延迟。DeltaScope 在结构层级上可以识别 composite type 属性定义中的 `COLLATE` 注解，但不渲染、解释或校验 collation 语义。不影响 MySQL/TiDB 行为。

---

## DDL：PostgreSQL 域生命周期（v0.57.0）

`v0.57.0` 新增 PostgreSQL 域生命周期覆盖。DeltaScope 规范化 `CREATE DOMAIN`、`ALTER DOMAIN`（constraint、default、not null、rename）和 `DROP DOMAIN`，新增七条 PostgreSQL-only 发现，并将复合类型作为显式不支持边界。`CHECK` 和 `DEFAULT` 表达式文本明确不渲染——规则只暴露布尔事实和约束名称。这些规则仅在设置 `--dialect postgresql` 时生效。

### 标准化操作

| SQL | 标准化操作 |
|-----|-----------|
| `CREATE DOMAIN email AS text CHECK (VALUE <> '')` | `create_domain` (type_kind=domain, base_type=text, has_check=true) |
| `CREATE DOMAIN email AS text NOT NULL DEFAULT 'n/a' CONSTRAINT chk CHECK (...)` | `create_domain` (type_kind=domain, base_type=text, not_null=true, has_default=true, has_check=true, constraint=chk) |
| `ALTER DOMAIN email SET DEFAULT 'x'` | `alter_domain` (action=set_default, has_default=true) |
| `ALTER DOMAIN email DROP DEFAULT` | `alter_domain` (action=drop_default) |
| `ALTER DOMAIN email SET NOT NULL` | `alter_domain` (action=set_not_null, not_null=true) |
| `ALTER DOMAIN email DROP NOT NULL` | `alter_domain` (action=drop_not_null) |
| `ALTER DOMAIN email ADD CONSTRAINT chk CHECK (...)` | `alter_domain` (action=add_constraint, has_check=true, constraint=chk) |
| `ALTER DOMAIN email DROP CONSTRAINT chk` | `alter_domain` (action=drop_constraint, constraint=chk) |
| `ALTER DOMAIN email VALIDATE CONSTRAINT chk` | `alter_domain` (action=validate_constraint, constraint=chk) |
| `ALTER DOMAIN email RENAME TO contact_email` | `alter_domain` (action=rename, new_name=contact_email) |
| `DROP DOMAIN email` | `drop_domain` |
| `DROP DOMAIN IF EXISTS email CASCADE` | `drop_domain` (if_exists=true, cascade=true) |

### 域生命周期规则

| 规则 ID | 检查描述 | 离线 | Metadata | 默认级别 |
|---------|---------|:----:|:--------:|---------|
| `ddl.pg.create_domain.notice` | `CREATE DOMAIN` 引入可复用类型约束——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter_domain.constraint.notice` | `ALTER DOMAIN ... ADD/DROP/VALIDATE CONSTRAINT` 修改类型合约——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter_domain.default.notice` | `ALTER DOMAIN ... SET/DROP DEFAULT` 变更隐式值——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter_domain.not_null.notice` | `ALTER DOMAIN ... SET/DROP NOT NULL` 变更可空性——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter_domain.rename.notice` | `ALTER DOMAIN ... RENAME TO` 变更域名称——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.drop_domain.advisory` | `DROP DOMAIN` 移除域——建议审查依赖列 | ✓ | ✗ | warning |
| `ddl.pg.drop_domain.cascade.warn` | `DROP DOMAIN ... CASCADE` 使用级联删除，可能静默移除依赖对象 | ✓ | ✗ | warning |

> **说明：** 这些规则均为离线规则，不需要数据库连接。DeltaScope 不渲染 `CHECK` 或 `DEFAULT` 表达式文本——规则只暴露布尔事实（`has_check`、`has_default`、`not_null`）和约束名称，不包含表达式正文。DeltaScope 不对域执行在线依赖验证。`DROP DOMAIN IF EXISTS ... CASCADE` 会同时触发 `ddl.pg.drop_domain.advisory` 和 `ddl.pg.drop_domain.cascade.warn`，属于有意设计。复合类型现已支持——参见上方 Composite Type Lifecycle。扩展现已支持——参见下方 Extension Lifecycle。不影响 MySQL/TiDB 行为。

---

## DDL：PostgreSQL Extension 生命周期（v0.59.0）

`v0.59.0` 新增 PostgreSQL extension 生命周期窄支持。DeltaScope 规范化 `CREATE EXTENSION`、`ALTER EXTENSION`（`UPDATE`、`UPDATE TO`、`SET SCHEMA`）和 `DROP EXTENSION`，新增六条 PostgreSQL-only 发现，并将 extension 成员变更（`ALTER EXTENSION ... ADD/DROP TABLE`）作为显式不支持/延迟边界。DeltaScope 不对 extension 做可用性、已安装包、版本兼容性或依赖图的实时校验。这些规则仅在设置 `--dialect postgresql` 时生效。

### 规范化操作

| SQL | 规范化操作 |
|-----|-----------|
| `CREATE EXTENSION pg_trgm` | `create_extension` |
| `CREATE EXTENSION IF NOT EXISTS pg_trgm` | `create_extension`（if_not_exists=true） |
| `CREATE EXTENSION pg_trgm WITH SCHEMA utils` | `create_extension`（schema=utils） |
| `CREATE EXTENSION pg_trgm WITH VERSION '1.5'` | `create_extension`（version=1.5） |
| `CREATE EXTENSION pg_trgm CASCADE` | `create_extension`（cascade=true） |
| `ALTER EXTENSION pg_trgm UPDATE` | `alter_extension`（action=update） |
| `ALTER EXTENSION pg_trgm UPDATE TO '1.6'` | `alter_extension`（action=update_to） |
| `ALTER EXTENSION pg_trgm SET SCHEMA utils` | `alter_extension`（action=set_schema） |
| `DROP EXTENSION pg_trgm` | `drop_extension` |
| `DROP EXTENSION IF EXISTS pg_trgm` | `drop_extension`（if_exists=true） |
| `DROP EXTENSION pg_trgm CASCADE` | `drop_extension`（cascade=true） |

### Extension 生命周期规则

| 规则 ID | 检查描述 | 离线 | Metadata | 默认级别 |
|---------|---------|:----:|:--------:|---------|
| `ddl.pg.create_extension.notice` | `CREATE EXTENSION` 安装扩展到数据库——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.create_extension.cascade.warn` | `CREATE EXTENSION ... CASCADE` 自动安装依赖——可能引入非预期扩展 | ✓ | ✗ | warning |
| `ddl.pg.alter_extension.update.notice` | `ALTER EXTENSION ... UPDATE` / `UPDATE TO` 升级扩展——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter_extension.set_schema.notice` | `ALTER EXTENSION ... SET SCHEMA` 将扩展移至不同 schema——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.drop_extension.advisory` | `DROP EXTENSION` 移除扩展——建议审查依赖对象 | ✓ | ✗ | warning |
| `ddl.pg.drop_extension.cascade.warn` | `DROP EXTENSION ... CASCADE` 使用级联删除，可能静默移除依赖对象 | ✓ | ✗ | warning |

### 不支持 / 延迟的操作

| SQL | 不支持的特性 |
|-----|------------|
| `ALTER EXTENSION ... ADD TABLE` | `alter_extension_add_member` |
| `ALTER EXTENSION ... DROP TABLE` | `alter_extension_drop_member` |

> **说明：** 这些规则均为离线规则，不需要数据库连接。`CREATE EXTENSION ... CASCADE` 会同时触发 `ddl.pg.create_extension.notice` 和 `ddl.pg.create_extension.cascade.warn`。`DROP EXTENSION ... CASCADE` 会同时触发 `ddl.pg.drop_extension.advisory` 和 `ddl.pg.drop_extension.cascade.warn`，属于有意设计。DeltaScope 不对 extension 做可用性、已安装包、版本兼容性或依赖图的实时校验。Extension 成员变更（`ALTER EXTENSION ... ADD/DROP TABLE`）明确不支持/延迟。不影响 MySQL/TiDB 行为。

---

## DDL：PostgreSQL 表级权限 DCL（v0.60.0）

`v0.60.0` 新增 PostgreSQL 表级权限 DCL 窄支持。DeltaScope 规范化 `GRANT ... ON TABLE` 和 `REVOKE ... ON TABLE`，新增四条 PostgreSQL-only 发现，并将 `ALL TABLES IN SCHEMA`、sequence privileges、role membership GRANT/REVOKE 和 `ALTER DEFAULT PRIVILEGES` 作为显式不支持/延迟边界。DeltaScope 不对表级权限做任何形式的实时校验。这些规则仅在设置 `--dialect postgresql` 时生效。

### 规范化操作

| SQL | 规范化操作 |
|-----|-----------|
| `GRANT SELECT ON users TO reader` | `grant_table_privilege` |
| `GRANT SELECT, INSERT ON users TO reader, writer` | `grant_table_privilege`（privileges=[SELECT, INSERT], grantees=[reader, writer]） |
| `GRANT ALL PRIVILEGES ON users TO admin` | `grant_table_privilege`（all_privileges=true） |
| `GRANT SELECT ON public.users TO reader` | `grant_table_privilege`（schema=public） |
| `REVOKE SELECT ON users FROM reader` | `revoke_table_privilege` |
| `REVOKE INSERT, UPDATE ON users FROM writer, editor` | `revoke_table_privilege`（privileges=[INSERT, UPDATE], grantees=[writer, editor]） |
| `REVOKE ALL PRIVILEGES ON users FROM admin` | `revoke_table_privilege`（all_privileges=true） |
| `REVOKE SELECT ON users FROM reader CASCADE` | `revoke_table_privilege`（cascade=true） |

### 表级权限 DCL 规则

| 规则 ID | 检查描述 | 离线 | Metadata | 默认级别 |
|---------|---------|:----:|:--------:|---------|
| `ddl.pg.grant.table_privilege.notice` | `GRANT ... ON TABLE` 授予表级权限——信息性通知 | ✓ | ✗ | notice |
| `ddl.pg.grant.table_privilege.all.warn` | `GRANT ALL PRIVILEGES ON TABLE` 授予所有权限——警告过度授权 | ✓ | ✗ | warning |
| `ddl.pg.revoke.table_privilege.notice` | `REVOKE ... ON TABLE` 撤销表级权限——信息性通知 | ✓ | ✗ | notice |
| `ddl.pg.revoke.table_privilege.cascade.warn` | `REVOKE ... ON TABLE ... CASCADE` 级联撤销——警告级联副作用 | ✓ | ✗ | warning |

### 不支持/延迟的操作

| SQL | 状态 |
|-----|------|
| `GRANT ... ON ALL TABLES IN SCHEMA` | 不支持 |
| Sequence privileges（`GRANT ... ON SEQUENCE`） | 不支持 |
| Role membership（`GRANT role TO role`） | 不支持 |
| `ALTER DEFAULT PRIVILEGES` | 不支持 |

> **说明：** 这些规则均为离线规则，不需要数据库连接。`GRANT ALL PRIVILEGES ON TABLE` 会同时触发 `ddl.pg.grant.table_privilege.notice` 和 `ddl.pg.grant.table_privilege.all.warn`。`REVOKE ... ON TABLE ... CASCADE` 会同时触发 `ddl.pg.revoke.table_privilege.notice` 和 `ddl.pg.revoke.table_privilege.cascade.warn`，属于有意设计。DeltaScope 不做任何形式的实时校验——不验证 grantee/role 是否存在、不验证 table/object 是否存在、不验证当前用户是否有授权权限、不计算 effective privileges、不解析 role inheritance、不验证 ownership、不评估 RLS/policies。这是窄表级权限 DCL 支持，不是广泛的治理或 admin DCL 支持。不影响 MySQL/TiDB 行为。

---

## DDL：PostgreSQL ALTER TABLE 覆盖（v0.51.0 / v0.52.0 / v0.54.0 / v0.56.0 / v0.130.0）

`v0.51.0` 扩展了 PostgreSQL ALTER TABLE 审核覆盖，新增三条补位规则。`v0.52.0` 新增六条规则覆盖此前 unsupported 的 ALTER TABLE 动作。`v0.54.0` 将触发器范围形式（`ENABLE/DISABLE TRIGGER ALL/USER`）规范化，复用既有触发器规则，并新增三条副本标识规则。`v0.56.0` 新增两条 logged-state 规则覆盖 `SET LOGGED` 和 `SET UNLOGGED`。`v0.130.0` 新增 10 条规则覆盖存储/布局、trigger/rule 残留和 reloptions。这些规则覆盖了既有 migration-safety 和 object lifecycle 规则族之外最常见的 ALTER TABLE 安全模式。仅在设置 `--dialect postgresql` 时生效。

### ALTER TABLE 覆盖规则

| 规则 ID | 检查描述 | 离线 | Metadata | 默认级别 |
|---------|---------|:----:|:--------:|---------|
| `ddl.pg.alter.drop_column.advisory` | `ALTER TABLE ... DROP COLUMN` 移除列，建议审查依赖查询和应用逻辑 | ✓ | ✗ | warning |
| `ddl.pg.alter.validate_constraint.advisory` | `ALTER TABLE ... VALIDATE CONSTRAINT` 执行验证扫描，提醒注意大表上的表级锁持有时长 | ✓ | ✗ | notice |
| `ddl.pg.alter.add_column.nullable.notice` | `ALTER TABLE ... ADD COLUMN` 添加不带 DEFAULT 的可空列，注意下游代码可能遇到意外 NULL 值 | ✓ | ✗ | notice |
| `ddl.pg.alter.set_schema.advisory` | `ALTER TABLE ... SET SCHEMA` 将表移至不同 schema，建议审查依赖查询和应用连接 | ✓ | ✗ | notice |
| `ddl.pg.alter.owner.advisory` | `ALTER TABLE ... OWNER TO` 更改表所有者，建议审查权限影响 | ✓ | ✗ | notice |
| `ddl.pg.alter.enable_trigger.notice` | `ALTER TABLE ... ENABLE TRIGGER name` 重新启用指定触发器，信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter.disable_trigger.warn` | `ALTER TABLE ... DISABLE TRIGGER name` 禁用指定触发器，警告该表上的触发器将不再执行 | ✓ | ✗ | warning |
| `ddl.pg.alter.attach_partition.advisory` | `ALTER TABLE ... ATTACH PARTITION` 将分区挂载到分区表，建议审查分区边界和数据路由 | ✓ | ✗ | notice |
| `ddl.pg.alter.detach_partition.warn` | `ALTER TABLE ... DETACH PARTITION` 分离分区，警告针对该分区的查询可能失败 | ✓ | ✗ | warning |
| `ddl.pg.alter.replica_identity_full.warn` | `ALTER TABLE ... REPLICA IDENTITY FULL` 写入完整旧行镜像到 WAL——警告复制开销 | ✓ | ✗ | warning |
| `ddl.pg.alter.replica_identity_nothing.warn` | `ALTER TABLE ... REPLICA IDENTITY NOTHING` 不写入旧行镜像到 WAL——警告逻辑复制将不可用 | ✓ | ✗ | warning |
| `ddl.pg.alter.replica_identity_using_index.notice` | `ALTER TABLE ... REPLICA IDENTITY USING INDEX ...` 使用指定索引用于 WAL 旧行镜像——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter.set_logged.notice` | `ALTER TABLE ... SET LOGGED` 将 unlogged 表转为 logged——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter.set_unlogged.notice` | `ALTER TABLE ... SET UNLOGGED` 将 logged 表转为 unlogged——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter.set_tablespace.notice` | `ALTER TABLE ... SET TABLESPACE` 将表移至不同表空间——信息性提示 (v0.130.0) | ✓ | ✗ | notice |
| `ddl.pg.alter.set_access_method.warn` | `ALTER TABLE ... SET ACCESS METHOD` 更改表访问方法——警告 rewrite 和兼容性影响 (v0.130.0) | ✓ | ✗ | warning |
| `ddl.pg.alter.enable_replica_trigger.notice` | `ALTER TABLE ... ENABLE REPLICA TRIGGER` 以 replica 模式启用触发器——信息性提示 (v0.130.0) | ✓ | ✗ | notice |
| `ddl.pg.alter.enable_always_trigger.notice` | `ALTER TABLE ... ENABLE ALWAYS TRIGGER` 以 always 模式启用触发器——信息性提示 (v0.130.0) | ✓ | ✗ | notice |
| `ddl.pg.alter.enable_rule.notice` | `ALTER TABLE ... ENABLE RULE` 启用重写规则——信息性提示 (v0.130.0) | ✓ | ✗ | notice |
| `ddl.pg.alter.disable_rule.warn` | `ALTER TABLE ... DISABLE RULE` 禁用重写规则——警告该规则将不再触发 (v0.130.0) | ✓ | ✗ | warning |
| `ddl.pg.alter.enable_replica_rule.notice` | `ALTER TABLE ... ENABLE REPLICA RULE` 以 replica 模式启用规则——信息性提示 (v0.130.0) | ✓ | ✗ | notice |
| `ddl.pg.alter.enable_always_rule.notice` | `ALTER TABLE ... ENABLE ALWAYS RULE` 以 always 模式启用规则——信息性提示 (v0.130.0) | ✓ | ✗ | notice |
| `ddl.pg.alter.set_reloptions.warn` | `ALTER TABLE ... SET (...)` 设置存储参数——警告潜在的 rewrite 或行为变化 (v0.130.0) | ✓ | ✗ | warning |
| `ddl.pg.alter.reset_reloptions.notice` | `ALTER TABLE ... RESET (...)` 将存储参数重置为默认值——信息性提示 (v0.130.0) | ✓ | ✗ | notice |

> **说明：** 触发器范围形式（`ENABLE/DISABLE TRIGGER ALL/USER`）已规范化，复用上方的 `enable_trigger` 和 `disable_trigger` 规则。`REPLICA IDENTITY DEFAULT` 已规范化且故意静默。这不是完整的 PostgreSQL ALTER TABLE 覆盖。这些规则均为离线规则，不需要数据库连接。DeltaScope 不会验证 `REPLICA IDENTITY USING INDEX` 所引用的索引是否有效、唯一或非部分索引。DeltaScope 不会验证目标表当前是否为 logged 或 unlogged 状态。v0.130.0 发现不输出 trigger 函数名、trigger 函数体、rule 查询/命令文本、tablespace 名称、access method 名称或 reloption 键/值。

---

## DDL：PostgreSQL RLS/Policy 生命周期（v0.70.0）

`v0.70.0` 新增 PostgreSQL 行级安全策略和 RLS 开关生命周期覆盖。DeltaScope 规范化 `CREATE POLICY`、`ALTER POLICY`、`DROP POLICY` 以及 `ALTER TABLE ... ENABLE/DISABLE/FORCE/NO FORCE ROW LEVEL SECURITY`，新增七条 PostgreSQL-only 规则。这些规则仅在设置 `--dialect postgresql` 时生效。

### 规范化操作

| SQL | 规范化操作 |
|-----|-----------|
| `CREATE POLICY p1 ON users USING (true)` | `create_policy` |
| `CREATE POLICY p1 ON users AS RESTRICTIVE ...` | `create_policy` (policy_type=restrictive) |
| `ALTER POLICY p1 ON users USING (true)` | `alter_policy` |
| `DROP POLICY p1 ON users` | `drop_policy` |
| `ALTER TABLE users ENABLE ROW LEVEL SECURITY` | `alter_table` (action=enable_rls) |
| `ALTER TABLE users DISABLE ROW LEVEL SECURITY` | `alter_table` (action=disable_rls) |
| `ALTER TABLE users FORCE ROW LEVEL SECURITY` | `alter_table` (action=force_rls) |
| `ALTER TABLE users NO FORCE ROW LEVEL SECURITY` | `alter_table` (action=no_force_rls) |

### RLS/Policy 生命周期规则

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.pg.create_policy.notice` | `CREATE POLICY` 引入新的 RLS 策略——信息性通知 | ✓ | ✗ | notice |
| `ddl.pg.alter_policy.notice` | `ALTER POLICY` 修改已有 RLS 策略——信息性通知 | ✓ | ✗ | notice |
| `ddl.pg.drop_policy.warn` | `DROP POLICY` 移除 RLS 策略——警告行级保护被移除 | ✓ | ✗ | warning |
| `ddl.pg.alter.enable_rls.notice` | `ALTER TABLE ... ENABLE ROW LEVEL SECURITY` 启用 RLS——信息性通知 | ✓ | ✗ | notice |
| `ddl.pg.alter.disable_rls.warn` | `ALTER TABLE ... DISABLE ROW LEVEL SECURITY` 禁用 RLS——警告保护被关闭 | ✓ | ✗ | warning |
| `ddl.pg.alter.force_rls.notice` | `ALTER TABLE ... FORCE ROW LEVEL SECURITY` 对表 owner 强制 RLS——信息性通知 | ✓ | ✗ | notice |
| `ddl.pg.alter.no_force_rls.notice` | `ALTER TABLE ... NO FORCE ROW LEVEL SECURITY` 取消对表 owner 的 RLS 强制——信息性通知 | ✓ | ✗ | notice |

> **说明：** 这些规则均为离线规则，不需要数据库连接。DeltaScope 不评估策略表达式、不验证策略对特定角色的适用性、不检查在线 RLS 状态。这不是完整的 PostgreSQL RLS 治理。不影响 MySQL/TiDB 行为。

---

## DDL：PostgreSQL Trigger 生命周期（v0.70.0）

`v0.70.0` 新增 PostgreSQL 触发器生命周期覆盖，支持 `CREATE TRIGGER`、`CREATE CONSTRAINT TRIGGER` 和 `DROP TRIGGER`。这些规则仅在设置 `--dialect postgresql` 时生效。

### 规范化操作

| SQL | 规范化操作 |
|-----|-----------|
| `CREATE TRIGGER t1 AFTER INSERT ON users FOR EACH ROW EXECUTE FUNCTION f()` | `create_trigger` |
| `CREATE CONSTRAINT TRIGGER t1 AFTER INSERT ON users DEFERRABLE INITIALLY DEFERRED ...` | `create_constraint_trigger` |
| `DROP TRIGGER t1 ON users` | `drop_trigger` |

### Trigger 生命周期规则

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.pg.create_trigger.notice` | `CREATE TRIGGER` 引入新触发器——信息性通知 | ✓ | ✗ | notice |
| `ddl.pg.create_constraint_trigger.warn` | `CREATE CONSTRAINT TRIGGER` 创建约束触发器——警告约束触发器语义 | ✓ | ✗ | warning |
| `ddl.pg.drop_trigger.advisory` | `DROP TRIGGER` 移除触发器——建议审查依赖逻辑 | ✓ | ✗ | notice |

> **说明：** 这些规则均为离线规则，不需要数据库连接。DeltaScope 不评估触发器体、不验证触发器函数是否存在。这不是完整的 PostgreSQL 触发器治理。不影响 MySQL/TiDB 行为。

---

## DDL：PostgreSQL Function/Procedure 生命周期（v0.70.0）

`v0.70.0` 新增 PostgreSQL 函数和存储过程生命周期覆盖，支持 `CREATE FUNCTION`、`CREATE OR REPLACE FUNCTION`、`DROP FUNCTION`、`CREATE PROCEDURE` 和 `DROP PROCEDURE`。这些规则仅在设置 `--dialect postgresql` 时生效。

### 规范化操作

| SQL | 规范化操作 |
|-----|-----------|
| `CREATE FUNCTION f() RETURNS void LANGUAGE plpgsql AS $$ ... $$` | `create_function` |
| `CREATE FUNCTION f() ... SECURITY DEFINER` | `create_function` (security_definer=true) |
| `CREATE OR REPLACE FUNCTION f() ...` | `create_or_replace_function` |
| `DROP FUNCTION f()` | `drop_function` |
| `CREATE PROCEDURE p() LANGUAGE plpgsql AS $$ ... $$` | `create_procedure` |
| `DROP PROCEDURE p()` | `drop_procedure` |

### Function/Procedure 生命周期规则

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.pg.create_function.notice` | `CREATE FUNCTION` 引入新函数——信息性通知 | ✓ | ✗ | notice |
| `ddl.pg.create_function.security_definer.warn` | `CREATE FUNCTION ... SECURITY DEFINER` 以 owner 权限执行——警告权限提升风险 | ✓ | ✗ | warning |
| `ddl.pg.create_or_replace_function.advisory` | `CREATE OR REPLACE FUNCTION` 替换已有函数——建议审查下游依赖 | ✓ | ✗ | notice |
| `ddl.pg.drop_function.advisory` | `DROP FUNCTION` 移除函数——建议审查依赖对象 | ✓ | ✗ | notice |
| `ddl.pg.create_procedure.notice` | `CREATE PROCEDURE` 引入新存储过程——信息性通知 | ✓ | ✗ | notice |
| `ddl.pg.drop_procedure.advisory` | `DROP PROCEDURE` 移除存储过程——建议审查依赖对象 | ✓ | ✗ | notice |

> **说明：** 这些规则均为离线规则，不需要数据库连接。`CREATE FUNCTION ... SECURITY DEFINER` 会同时触发 `ddl.pg.create_function.notice` 和 `ddl.pg.create_function.security_definer.warn`，属于有意设计。DeltaScope 不评估函数体、不验证参数类型、不检查在线函数状态。这不是完整的 PostgreSQL 函数/存储过程治理。不影响 MySQL/TiDB 行为。

---

## DDL：PostgreSQL 高级视图生命周期（v0.70.0）

`v0.70.0` 新增 PostgreSQL 高级视图生命周期覆盖，超出基础 `CREATE VIEW` / `DROP VIEW` 形态。DeltaScope 规范化 `CREATE OR REPLACE VIEW`、`CREATE TEMP VIEW`、`CREATE VIEW ... WITH CHECK OPTION`、`DROP VIEW ... CASCADE`、`ALTER VIEW ... RENAME TO` 和 `ALTER VIEW ... SET SCHEMA`，新增六条 PostgreSQL-only 规则。这些规则仅在设置 `--dialect postgresql` 时生效。

### 规范化操作

| SQL | 规范化操作 |
|-----|-----------|
| `CREATE OR REPLACE VIEW v1 AS SELECT 1` | `create_or_replace_view` |
| `CREATE TEMP VIEW tv1 AS SELECT 1` | `create_temp_view` |
| `CREATE TEMPORARY VIEW tv1 AS SELECT 1` | `create_temp_view` |
| `CREATE VIEW v1 AS SELECT 1 WITH CHECK OPTION` | `create_view` (check_option=cascaded) |
| `CREATE VIEW v1 AS SELECT 1 WITH CASCADED CHECK OPTION` | `create_view` (check_option=cascaded) |
| `CREATE VIEW v1 AS SELECT 1 WITH LOCAL CHECK OPTION` | `create_view` (check_option=local) |
| `DROP VIEW v1 CASCADE` | `drop_view` (cascade=true) |
| `ALTER VIEW v1 RENAME TO v2` | `alter_view` (action=rename) |
| `ALTER VIEW v1 SET SCHEMA schema2` | `alter_view` (action=set_schema) |

### 高级视图生命周期规则

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.pg.create_or_replace_view.advisory` | `CREATE OR REPLACE VIEW` 替换已有视图——建议审查下游依赖 | ✓ | ✗ | notice |
| `ddl.pg.create_temp_view.notice` | `CREATE TEMP VIEW` / `CREATE TEMPORARY VIEW` 创建会话级临时视图——信息性通知 | ✓ | ✗ | notice |
| `ddl.pg.create_view.check_option.notice` | `CREATE VIEW ... WITH CHECK OPTION` 强制检查选项——信息性通知 | ✓ | ✗ | notice |
| `ddl.pg.drop_view.cascade.warn` | `DROP VIEW ... CASCADE` 使用级联删除——可能静默移除依赖对象 | ✓ | ✗ | warning |
| `ddl.pg.alter_view.rename.notice` | `ALTER VIEW ... RENAME TO` 变更视图名称——信息性通知 | ✓ | ✗ | notice |
| `ddl.pg.alter_view.set_schema.notice` | `ALTER VIEW ... SET SCHEMA` 将视图移至不同 schema——信息性通知 | ✓ | ✗ | notice |

> **说明：** 这些规则均为离线规则，不需要数据库连接。DeltaScope 不评估视图查询体、不检查在线视图状态。这不是完整的 PostgreSQL 视图治理。不影响 MySQL/TiDB 行为。

---

## DDL：PostgreSQL 已选 ALTER 对象生命周期（v0.70.0）

`v0.70.0` 新增 PostgreSQL 对 schema、index 和 materialized view 的已选 ALTER 对象生命周期覆盖。DeltaScope 规范化 `ALTER SCHEMA ... RENAME TO`、`ALTER SCHEMA ... OWNER TO`、`ALTER INDEX ... RENAME TO`、`ALTER INDEX ... SET TABLESPACE`、`ALTER MATERIALIZED VIEW ... RENAME TO` 和 `ALTER MATERIALIZED VIEW ... SET SCHEMA`，新增六条 PostgreSQL-only 规则。这些规则仅在设置 `--dialect postgresql` 时生效。

### 规范化操作

| SQL | 规范化操作 |
|-----|-----------|
| `ALTER SCHEMA s1 RENAME TO s2` | `alter_schema` (action=rename) |
| `ALTER SCHEMA s1 OWNER TO new_owner` | `alter_schema` (action=owner) |
| `ALTER INDEX idx1 RENAME TO idx2` | `alter_index` (action=rename) |
| `ALTER INDEX idx1 SET TABLESPACE ts2` | `alter_index` (action=set_tablespace) |
| `ALTER MATERIALIZED VIEW mv1 RENAME TO mv2` | `alter_materialized_view` (action=rename) |
| `ALTER MATERIALIZED VIEW mv1 SET SCHEMA schema2` | `alter_materialized_view` (action=set_schema) |

### 已选 ALTER 对象生命周期规则

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.pg.alter_schema.rename.notice` | `ALTER SCHEMA ... RENAME TO` 变更 schema 名称——信息性通知 | ✓ | ✗ | notice |
| `ddl.pg.alter_schema.owner.notice` | `ALTER SCHEMA ... OWNER TO` 变更 schema owner——信息性通知 | ✓ | ✗ | notice |
| `ddl.pg.alter_index.rename.notice` | `ALTER INDEX ... RENAME TO` 变更索引名称——信息性通知 | ✓ | ✗ | notice |
| `ddl.pg.alter_index.set_tablespace.notice` | `ALTER INDEX ... SET TABLESPACE` 将索引移至不同表空间——信息性通知 | ✓ | ✗ | notice |
| `ddl.pg.alter_materialized_view.rename.notice` | `ALTER MATERIALIZED VIEW ... RENAME TO` 变更物化视图名称——信息性通知 | ✓ | ✗ | notice |
| `ddl.pg.alter_materialized_view.set_schema.notice` | `ALTER MATERIALIZED VIEW ... SET SCHEMA` 将物化视图移至不同 schema——信息性通知 | ✓ | ✗ | notice |

> **说明：** 这些规则均为离线规则，不需要数据库连接。DeltaScope 不验证在线 schema/index/物化视图的存在性、所有权或表空间可用性。这不是完整的 PostgreSQL ALTER 对象生命周期覆盖——其余 ALTER 形式（如 `ALTER INDEX ... SET (...)`、`ALTER MATERIALIZED VIEW ... OWNER TO`）已推迟。不影响 MySQL/TiDB 行为。

---

## DDL：PostgreSQL Selected Non-Permission DDL Deep Coverage（v0.80.0）

`v0.80.0` 新增 36 条 PostgreSQL-only DDL 生命周期规则，提供 selected PostgreSQL non-permission DDL deep coverage，涵盖六个规则族：composite type 属性变更、extension 成员变更、publication/subscription 生命周期、foreign object 生命周期（外部数据包装器、外部服务器、用户映射、外部表）、注解操作（`COMMENT ON`、`SECURITY LABEL`）以及 event trigger/rewrite rule 生命周期。这些规则仅在设置 `--dialect postgresql` 时生效。

### Composite Type Attribute Lifecycle 规则

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.pg.alter_type.add_attribute.notice` | `ALTER TYPE ... ADD ATTRIBUTE` 向 composite type 添加新属性——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter_type.drop_attribute.warn` | `ALTER TYPE ... DROP ATTRIBUTE` 移除属性——警告依赖列和函数 | ✓ | ✗ | warning |
| `ddl.pg.alter_type.alter_attribute_type.warn` | `ALTER TYPE ... ALTER ATTRIBUTE ... TYPE` 变更属性类型——警告潜在数据转换问题 | ✓ | ✗ | warning |
| `ddl.pg.alter_type.rename_attribute.notice` | `ALTER TYPE ... RENAME ATTRIBUTE` 重命名属性——信息性提示 | ✓ | ✗ | notice |

### Extension Member Lifecycle 规则

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.pg.alter_extension.add_member.notice` | `ALTER EXTENSION ... ADD TABLE` 将对象添加到扩展——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter_extension.drop_member.warn` | `ALTER EXTENSION ... DROP TABLE` 从扩展中移除对象——警告 extension-drop 级联影响 | ✓ | ✗ | warning |

### Publication/Subscription Lifecycle 规则

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.pg.create_publication.notice` | `CREATE PUBLICATION` 引入新的发布——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter_publication.notice` | `ALTER PUBLICATION` 修改已有发布——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.drop_publication.warn` | `DROP PUBLICATION` 移除发布——警告订阅者将停止接收变更 | ✓ | ✗ | warning |
| `ddl.pg.create_subscription.notice` | `CREATE SUBSCRIPTION` 建立新的订阅——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter_subscription.notice` | `ALTER SUBSCRIPTION` 修改已有订阅——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter_subscription.disable.warn` | `ALTER SUBSCRIPTION ... DISABLE` 禁用订阅——警告复制将停止 | ✓ | ✗ | warning |
| `ddl.pg.drop_subscription.warn` | `DROP SUBSCRIPTION` 移除订阅——警告关于复制槽清理 | ✓ | ✗ | warning |

### Foreign Object Lifecycle 规则

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.pg.create_foreign_data_wrapper.notice` | `CREATE FOREIGN DATA WRAPPER` 引入新的 FDW——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter_foreign_data_wrapper.notice` | `ALTER FOREIGN DATA WRAPPER` 修改已有 FDW——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.drop_foreign_data_wrapper.warn` | `DROP FOREIGN DATA WRAPPER` 移除 FDW——警告依赖的外部服务器和外部表 | ✓ | ✗ | warning |
| `ddl.pg.create_foreign_server.notice` | `CREATE SERVER` 注册新的外部服务器——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter_foreign_server.notice` | `ALTER SERVER` 修改已有外部服务器——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.drop_foreign_server.warn` | `DROP SERVER` 移除外部服务器——警告依赖的用户映射和外部表 | ✓ | ✗ | warning |
| `ddl.pg.create_user_mapping.notice` | `CREATE USER MAPPING` 注册用户映射——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter_user_mapping.notice` | `ALTER USER MAPPING` 修改已有用户映射——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.drop_user_mapping.warn` | `DROP USER MAPPING` 移除用户映射——警告依赖的外部表连接 | ✓ | ✗ | warning |
| `ddl.pg.create_foreign_table.notice` | `CREATE FOREIGN TABLE` 引入新的外部表——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter_foreign_table.notice` | `ALTER FOREIGN TABLE` 修改已有外部表——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.drop_foreign_table.warn` | `DROP FOREIGN TABLE` 移除外部表——警告依赖查询 | ✓ | ✗ | warning |

### Annotation Lifecycle 规则

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.pg.comment_on.notice` | `COMMENT ON ... IS 'text'` 为数据库对象附加注释——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.comment_on.remove.notice` | `COMMENT ON ... IS NULL` 移除注释——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.security_label.notice` | `SECURITY LABEL ... IS 'label'` 附加安全标签——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.security_label.remove.notice` | `SECURITY LABEL ... IS NULL` 移除安全标签——信息性提示 | ✓ | ✗ | notice |

### Event Trigger / Rewrite Rule Lifecycle 规则

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.pg.create_event_trigger.notice` | `CREATE EVENT TRIGGER` 引入新的事件触发器——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter_event_trigger.notice` | `ALTER EVENT TRIGGER` 修改已有事件触发器——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter_event_trigger.disable.warn` | `ALTER EVENT TRIGGER ... DISABLE` 禁用事件触发器——警告 DDL 事件处理将停止 | ✓ | ✗ | warning |
| `ddl.pg.drop_event_trigger.warn` | `DROP EVENT TRIGGER` 移除事件触发器——警告 DDL 事件处理影响 | ✓ | ✗ | warning |
| `ddl.pg.create_rule.notice` | `CREATE RULE` 引入新的重写规则——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.alter_rule.notice` | `ALTER RULE` 修改已有重写规则——信息性提示 | ✓ | ✗ | notice |
| `ddl.pg.drop_rule.warn` | `DROP RULE` 移除重写规则——警告依赖查询行为 | ✓ | ✗ | warning |

> **说明：** 这是 36 条新的 PostgreSQL-only DDL 生命周期规则。所有规则均为离线规则，不需要数据库连接。Composite type 属性操作取代了此前在 Composite Type Lifecycle 部分中列出的不支持/延迟条目。Extension 成员操作取代了此前在 Extension Lifecycle 部分中列出的不支持/延迟条目。`DROP SUBSCRIPTION ... WITH (drop_slot = true)` 仍被延迟（parser_error）。DeltaScope 不验证在线对象状态、不验证数据转换安全性、不检查复制槽状态、不验证 FDW handler/validator 函数、不评估触发器/规则体。这是 selected PostgreSQL non-permission DDL deep coverage——不是 full PostgreSQL DDL support，也不是 complete PostgreSQL grammar coverage。不影响 MySQL/TiDB 行为。

---

## DDL：PostgreSQL 长尾对象生命周期（v0.100.0）

`v0.100.0` 扩展 PostgreSQL DDL 审核覆盖范围至选定的长尾对象生命周期族：排序规则、扩展统计、聚合/操作符/转换、操作符族/类、全文搜索对象以及边界闭合（DROP TRANSFORM、DROP ACCESS METHOD、ALTER LARGE OBJECT）。DeltaScope 规范化这些 DDL 形态，新增 36 条 PostgreSQL-only 生命周期发现/规则。这些规则仅在设置 `--dialect postgresql` 时生效。

### 规范化操作

| SQL | 规范化操作 |
|-----|-----------|
| `CREATE COLLATION ...` | `create_collation` |
| `ALTER COLLATION ... RENAME TO` | `alter_collation` |
| `ALTER COLLATION ... OWNER TO` | `alter_collation` |
| `ALTER COLLATION ... SET SCHEMA` | `alter_collation` |
| `DROP COLLATION ...` | `drop_collation` |
| `CREATE STATISTICS ...` | `create_statistics` |
| `ALTER STATISTICS ... RENAME TO` | `alter_statistics` |
| `ALTER STATISTICS ... OWNER TO` | `alter_statistics` |
| `ALTER STATISTICS ... SET SCHEMA` | `alter_statistics` |
| `DROP STATISTICS ...` | `drop_statistics` |
| `CREATE AGGREGATE ...` | `create_aggregate` |
| `ALTER AGGREGATE ... RENAME TO` | `alter_aggregate` |
| `ALTER AGGREGATE ... OWNER TO` | `alter_aggregate` |
| `ALTER AGGREGATE ... SET SCHEMA` | `alter_aggregate` |
| `DROP AGGREGATE ...` | `drop_aggregate` |
| `CREATE OPERATOR ...` | `create_operator` |
| `ALTER OPERATOR ... OWNER TO` | `alter_operator` |
| `ALTER OPERATOR ... SET SCHEMA` | `alter_operator` |
| `DROP OPERATOR ...` | `drop_operator` |
| `CREATE CONVERSION ...` | `create_conversion` |
| `ALTER CONVERSION ... RENAME TO` | `alter_conversion` |
| `ALTER CONVERSION ... OWNER TO` | `alter_conversion` |
| `ALTER CONVERSION ... SET SCHEMA` | `alter_conversion` |
| `DROP CONVERSION ...` | `drop_conversion` |
| `CREATE OPERATOR FAMILY ...` | `create_operator_family` |
| `ALTER OPERATOR FAMILY ... RENAME TO` | `alter_operator_family` |
| `ALTER OPERATOR FAMILY ... OWNER TO` | `alter_operator_family` |
| `ALTER OPERATOR FAMILY ... SET SCHEMA` | `alter_operator_family` |
| `DROP OPERATOR FAMILY ...` | `drop_operator_family` |
| `CREATE OPERATOR CLASS ...` | `create_operator_class` |
| `ALTER OPERATOR CLASS ... RENAME TO` | `alter_operator_class` |
| `ALTER OPERATOR CLASS ... OWNER TO` | `alter_operator_class` |
| `ALTER OPERATOR CLASS ... SET SCHEMA` | `alter_operator_class` |
| `DROP OPERATOR CLASS ...` | `drop_operator_class` |
| `CREATE TEXT SEARCH CONFIGURATION ...` | `create_text_search_configuration` |
| `ALTER TEXT SEARCH CONFIGURATION ...` | `alter_text_search_configuration` |
| `DROP TEXT SEARCH CONFIGURATION ...` | `drop_text_search_configuration` |
| `CREATE TEXT SEARCH DICTIONARY ...` | `create_text_search_dictionary` |
| `ALTER TEXT SEARCH DICTIONARY ...` | `alter_text_search_dictionary` |
| `DROP TEXT SEARCH DICTIONARY ...` | `drop_text_search_dictionary` |
| `CREATE TEXT SEARCH PARSER ...` | `create_text_search_parser` |
| `ALTER TEXT SEARCH PARSER ...` | `alter_text_search_parser` |
| `DROP TEXT SEARCH PARSER ...` | `drop_text_search_parser` |
| `CREATE TEXT SEARCH TEMPLATE ...` | `create_text_search_template` |
| `ALTER TEXT SEARCH TEMPLATE ...` | `alter_text_search_template` |
| `DROP TEXT SEARCH TEMPLATE ...` | `drop_text_search_template` |
| `CREATE TRANSFORM FOR ... LANGUAGE ...` | `create_transform` |
| `DROP TRANSFORM FOR ... LANGUAGE ...` | `drop_transform` |
| `CREATE ACCESS METHOD ... TYPE ... HANDLER ...` | `create_access_method` |
| `DROP ACCESS METHOD ...` | `drop_access_method` |
| `ALTER LARGE OBJECT ... OWNER TO` | `alter_large_object` |

### 长尾生命周期规则

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.pg.create_collation.notice` | CREATE COLLATION 通知 | ✓ | ✗ | notice |
| `ddl.pg.alter_collation.notice` | ALTER COLLATION 通知 | ✓ | ✗ | notice |
| `ddl.pg.drop_collation.warn` | DROP COLLATION 警告 | ✓ | ✗ | warning |
| `ddl.pg.create_statistics.notice` | CREATE STATISTICS 通知 | ✓ | ✗ | notice |
| `ddl.pg.alter_statistics.notice` | ALTER STATISTICS 通知 | ✓ | ✗ | notice |
| `ddl.pg.drop_statistics.warn` | DROP STATISTICS 警告 | ✓ | ✗ | warning |
| `ddl.pg.create_aggregate.notice` | CREATE AGGREGATE 通知 | ✓ | ✗ | notice |
| `ddl.pg.alter_aggregate.notice` | ALTER AGGREGATE 通知 | ✓ | ✗ | notice |
| `ddl.pg.drop_aggregate.warn` | DROP AGGREGATE 警告 | ✓ | ✗ | warning |
| `ddl.pg.create_operator.notice` | CREATE OPERATOR 通知 | ✓ | ✗ | notice |
| `ddl.pg.alter_operator.notice` | ALTER OPERATOR 通知 | ✓ | ✗ | notice |
| `ddl.pg.drop_operator.warn` | DROP OPERATOR 警告 | ✓ | ✗ | warning |
| `ddl.pg.create_conversion.notice` | CREATE CONVERSION 通知 | ✓ | ✗ | notice |
| `ddl.pg.alter_conversion.notice` | ALTER CONVERSION 通知 | ✓ | ✗ | notice |
| `ddl.pg.drop_conversion.warn` | DROP CONVERSION 警告 | ✓ | ✗ | warning |
| `ddl.pg.create_operator_family.notice` | CREATE OPERATOR FAMILY 通知 | ✓ | ✗ | notice |
| `ddl.pg.alter_operator_family.notice` | ALTER OPERATOR FAMILY 通知 | ✓ | ✗ | notice |
| `ddl.pg.drop_operator_family.warn` | DROP OPERATOR FAMILY 警告 | ✓ | ✗ | warning |
| `ddl.pg.create_operator_class.notice` | CREATE OPERATOR CLASS 通知 | ✓ | ✗ | notice |
| `ddl.pg.alter_operator_class.notice` | ALTER OPERATOR CLASS 通知 | ✓ | ✗ | notice |
| `ddl.pg.drop_operator_class.warn` | DROP OPERATOR CLASS 警告 | ✓ | ✗ | warning |
| `ddl.pg.create_text_search_configuration.notice` | CREATE TEXT SEARCH CONFIGURATION 通知 | ✓ | ✗ | notice |
| `ddl.pg.alter_text_search_configuration.notice` | ALTER TEXT SEARCH CONFIGURATION 通知 | ✓ | ✗ | notice |
| `ddl.pg.drop_text_search_configuration.warn` | DROP TEXT SEARCH CONFIGURATION 警告 | ✓ | ✗ | warning |
| `ddl.pg.create_text_search_dictionary.notice` | CREATE TEXT SEARCH DICTIONARY 通知 | ✓ | ✗ | notice |
| `ddl.pg.alter_text_search_dictionary.notice` | ALTER TEXT SEARCH DICTIONARY 通知 | ✓ | ✗ | notice |
| `ddl.pg.drop_text_search_dictionary.warn` | DROP TEXT SEARCH DICTIONARY 警告 | ✓ | ✗ | warning |
| `ddl.pg.create_text_search_parser.notice` | CREATE TEXT SEARCH PARSER 通知 | ✓ | ✗ | notice |
| `ddl.pg.alter_text_search_parser.notice` | ALTER TEXT SEARCH PARSER 通知 | ✓ | ✗ | notice |
| `ddl.pg.drop_text_search_parser.warn` | DROP TEXT SEARCH PARSER 警告 | ✓ | ✗ | warning |
| `ddl.pg.create_text_search_template.notice` | CREATE TEXT SEARCH TEMPLATE 通知 | ✓ | ✗ | notice |
| `ddl.pg.alter_text_search_template.notice` | ALTER TEXT SEARCH TEMPLATE 通知 | ✓ | ✗ | notice |
| `ddl.pg.drop_text_search_template.warn` | DROP TEXT SEARCH TEMPLATE 警告 | ✓ | ✗ | warning |
| `ddl.pg.create_transform.notice` | CREATE TRANSFORM 通知 | ✓ | ✗ | notice |
| `ddl.pg.create_access_method.notice` | CREATE ACCESS METHOD 通知 | ✓ | ✗ | notice |
| `ddl.pg.drop_transform.warn` | DROP TRANSFORM 警告 | ✓ | ✗ | warning |
| `ddl.pg.drop_access_method.warn` | DROP ACCESS METHOD 警告 | ✓ | ✗ | warning |
| `ddl.pg.alter_large_object.owner.notice` | ALTER LARGE OBJECT 属主变更通知 | ✓ | ✗ | notice |

---

## DDL：PostgreSQL 覆盖范围扩展（v0.21.0 / v0.23.0 / v0.24.0）

`v0.21.0` 将常见 PostgreSQL 迁移后续 DDL 通过共享审核管线进行标准化处理。`v0.23.0` 进一步扩展了 PostgreSQL `CREATE TABLE` 常见约束形态的覆盖范围。`v0.24.0` 深化了这些建表形态的语义信息，通过共享 `spec.Constraint` 模型保留解析器拥有的 `ReferencedTable` 和 `ReferencedColumns`。这些功能面此前会落入能力边界错误或结构不完整；现在会产生带有渐进丰富语义的正常审计结果。不引入新规则——已有的共享规则族在适用时自动生效。

| PostgreSQL DDL 动作 | 标准化为 | 已支持 | 可审计 | 规则映射 | 依赖 Metadata | 说明 |
|---------------------|---------|:------:|:------:|:--------:|:------------:|------|
| `ALTER COLUMN ... SET DEFAULT` | `set_default` | ✓ | ✓ | ✓（共享 alter） | — | 标准 alter 动作 |
| `ALTER COLUMN ... DROP DEFAULT` | `drop_default` | ✓ | ✓ | ✓（共享 alter） | — | 标准 alter 动作 |
| `ALTER COLUMN ... SET NOT NULL` | `set_not_null` | ✓ | ✓ | ✓（共享 alter） | — | 标准 alter 动作 |
| `ALTER COLUMN ... DROP NOT NULL` | `drop_not_null` | ✓ | ✓ | ✓（共享 alter） | — | 标准 alter 动作 |
| `VALIDATE CONSTRAINT` | `validate_constraint` | ✓ | ✓ | — | — | 无专用规则；除非其他 finding 适用，否则产生干净审计 |
| `DROP CONSTRAINT`（一般） | `drop_constraint` | ✓ | ✓ | — | — | 离线模式下为标准 alter 动作 |
| `DROP CONSTRAINT`（主键） | `drop_constraint` | ✓ | ✓ | ✓（`ddl.alter.drop_primary_key`） | ✓ | 通过 metadata-aware 规则的主键映射 |
| 表级命名 `CHECK` | `create_table` 共享事实 | ✓ | ✓ | ✓（配置后可复用共享约束命名治理） | — | 覆盖扩展；不新增规则族 |
| 列级内联 `CHECK` | `create_table` 共享事实 | ✓ | ✓ | — | — | 结构已支持；无专用规则族 |
| 表级命名 `UNIQUE` | `create_table` 共享事实 | ✓ | ✓ | ✓（配置后可复用共享约束命名治理） | — | 命名约束事实可复用既有命名治理 |
| 列级内联 `UNIQUE` | `create_table` 共享事实 | ✓ | ✓ | ✓（共享索引事实） | — | 现有共享索引规则可消费标准化索引事实 |
| 表级命名 `FOREIGN KEY` | `create_table` 共享事实 | ✓ | ✓ | ✓（配置后可复用共享约束命名治理） | — | 仅当策略允许外键时，命名规则才有意义。`v0.24.0`：保留 `ReferencedTable`/`ReferencedColumns` |
| 列级内联 `REFERENCES` | `create_table` 共享事实 | ✓ | ✓ | — | — | 仅作为 parser-owned 共享事实；不发明 metadata 语义。`v0.24.0`：保留 `ReferencedTable`/`ReferencedColumns` |

### 接口一致性

所有新标准化的 PostgreSQL DDL 动作以及 `v0.23.0`/`v0.24.0` 的建表覆盖形态，均已在 CLI、HTTP（`POST /v1/audit`）、MCP（`audit_sql`）和公共 Go API（`pkg/deltascope`）上确认一致。

## Confidence 入口（`v0.22.0`）

`v0.22.0` 是 **E2E & Release Confidence Pack**。它不引入新的 SQL 规则语义，而是通过规范化的仓库目标来记录并验证既有的 CLI、HTTP、MCP 与 release surface。

- `make pg-unit-test-gates` —— 无需 Docker 的 PostgreSQL tag 单元测试 confidence
- `make pg-e2e-gates` —— 基于 Docker 的 PostgreSQL CLI、HTTP、MCP transport confidence
- `make pg-confidence-gates` —— 规范化的 PostgreSQL confidence 总入口
- `make release-surface-gates VERSION=vX.Y.Z` —— package/release 合同校验
- `make release-version-surface-gates VERSION=vX.Y.Z` —— 带版本的文档/安装/release-notes 校验

## Release Contract Gates（`v0.44.0`）

`v0.44.0` 新增 **Release Contract Hardening Pack** — 统一的 `make release-contract-gates VERSION=vX.Y.Z` 在每次 tagged release 前校验版本面、二进制版本输出、默认策略方言隔离和 archive 完整性。未新增规则 ID、解析器功能或公共 API 契约。

## 语料库支撑的置信度（`v0.25.0`）

`v0.25.0` 是 **SQL 语料库与边界置信度包**。它新增了跨方言的 SQL 语料库（`testdata/sql-corpus/`），通过现有审计应用层运行代表性的 MySQL、TiDB 和 PostgreSQL 用例，并在两个层面断言预期结果：

1. **报告层**——不支持计数、语句类型、findings 包含/排除。
2. **语义层**——操作名和约束事实（类型、名称、列、被引用表/列）。

语料库不新增规则、不改变审计行为、不影响终端用户工作流。它是发布信心资产：回答哪些 SQL 模式已被验证、预期结果是什么。

## PostgreSQL Primary Key 事实支持（`v0.37.0`）

`v0.37.0` 是 **PostgreSQL Primary Key Fact Support Pack**。PostgreSQL `CREATE TABLE` 的内联、表级、命名和复合主键声明现在会填充 DeltaScope 标准化主键契约，使已有的主键规则可以审计 PostgreSQL `CREATE TABLE` 语句。

| 方面 | 范围 |
|------|------|
| 已支持形态 | 内联（`id bigint PRIMARY KEY`）、表级（`PRIMARY KEY (id)`）、命名（`CONSTRAINT t_pkey PRIMARY KEY (id)`）、复合（`PRIMARY KEY (a, b)`） |
| NOT NULL 推导 | PK 列被有效视为 NOT NULL |
| 解锁规则 | `ddl.table.primary_key.bigint.require`、`ddl.table.primary_key.columns.max_count` |
| `ddl.table.primary_key.not_null.require` | 对 PostgreSQL 无稳定负例——PK 列有效 NOT NULL |
| Parser/spec 变更 | PostgreSQL 提取器填充共享 `DDL.PrimaryKey` 契约 |
| 新 rule ID | 无 |
| 新 CLI / API 标志 | 无 |

### 不包含的内容

- 完整 PostgreSQL 索引支持。
- `ALTER TABLE ADD PRIMARY KEY` 支持。
- 在线 schema 主键内省。
- 完整 PostgreSQL 约束/索引对等。

## PostgreSQL Generated/Identity Rule Coverage（`v0.36.0`）

`v0.36.0` 是 **PostgreSQL Generated/Identity Rule Coverage Pack**。三条新的 PostgreSQL-only forbid 规则覆盖了 v0.35.0 已支持的 generated/identity 状态转换形态。这是规则覆盖——不是 parser 支持范围扩展、不是 spec 契约扩展、不是 generated expression 求值、不是完整的 PostgreSQL 序列语义。

| 方面 | 范围 |
|------|------|
| 新 rule ID | `ddl.alter.drop_expression.forbid`、`ddl.alter.set_generated.forbid`、`ddl.alter.drop_identity.forbid` |
| 规则类型 | PostgreSQL-only forbid alter-action 规则 |
| 覆盖形态 | `DROP EXPRESSION`、`SET GENERATED ALWAYS`、`SET GENERATED BY DEFAULT`、`DROP IDENTITY` |
| Parser/spec 变更 | 无——状态转换形态已在 v0.35.0 支持 |
| 新 spec 字段 | 无 |
| 新 CLI / API 标志 | 无 |

### 规则覆盖矩阵

| Rule ID | Action | 覆盖形态 |
|---------|--------|---------|
| `ddl.alter.drop_expression.forbid` | `drop_expression` | `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION` |
| `ddl.alter.set_generated.forbid` | `set_generated` | `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ALWAYS/BY DEFAULT` |
| `ddl.alter.drop_identity.forbid` | `drop_identity` | `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY` |

### Surface 契约

| Surface | 行为 |
|---------|------|
| CLI | 正常审核输出，包含带 `rule_id` 的 findings |
| pkg/deltascope | `Audit()` 返回带 findings 的 result，含明确 `rule_id` |
| HTTP | 正常审核响应，findings 含明确 `rule_id` |
| MCP | 正常 tool result，findings 含明确 `rule_id` |

### 未变更的部分

- 无 parser 支持范围扩展。
- 无 spec 契约扩展。
- 无 generated expression 求值。
- 无完整 PostgreSQL 序列语义。
- 无 MySQL/TiDB 行为变更。

## PostgreSQL Generated/Identity State-Transition Support（`v0.35.0`）

`v0.35.0` 是 **PostgreSQL Generated/Identity State-Transition Pack**。PostgreSQL generated 和 identity 列的状态转换形态现在通过正常审核路径支持。这是状态转换支持——不是完整的 generated-column 生命周期支持、不是 generated expression 求值、不是完整的 PostgreSQL 序列语义，也没有新增 rule ID。

| 方面 | 范围 |
|------|------|
| 已支持形态 | `DROP EXPRESSION`、`SET GENERATED ALWAYS`、`SET GENERATED BY DEFAULT`、`DROP IDENTITY` |
| 标准化契约 | `drop_expression`、`set_generated` 含 `generated_when`（`"a"` / `"d"`）、`drop_identity` |
| GeneratedExpression | 不在契约中——不保留表达式文本 |
| 新 rule ID | 无 |
| 新 CLI / API 标志 | 无 |

### 已支持状态转换形态

| 形态 | 状态 | 审核行为 |
|------|------|---------|
| `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION` | 已支持 | 正常审核结果，适用时产生 findings |
| `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ALWAYS` | 已支持 | 正常审核结果，适用时产生 findings |
| `ALTER TABLE ... ALTER COLUMN ... SET GENERATED BY DEFAULT` | 已支持 | 正常审核结果，适用时产生 findings |
| `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY` | 已支持 | 正常审核结果，适用时产生 findings |

### 已支持形态的 Surface 契约

| Surface | 行为 |
|---------|------|
| CLI | 正常审核输出（退出码 0 或 1 取决于 findings），无 `unsupported` 数组 |
| pkg/deltascope | `Audit()` 返回带 statements 的 result，无 `ErrUnsupportedStatement` |
| HTTP | 正常审核响应，无 unsupported 错误 |
| MCP | 正常 tool result 带 statements，无 `IsError` |

### 前序：窄范围定义形态支持（`v0.34.0`）

`v0.34.0` 新增了窄范围 generated/identity 定义形态（`CREATE TABLE` 和 `ALTER TABLE ADD COLUMN`）的支持。这些形态继续通过正常审核路径流转。保留事实：`generated_when`、`is_identity`、`identity_options`（来自 v0.33.0）。

### v0.33.0 — PostgreSQL Generated/Identity 事实保留 + Unsupported Metadata 展示包

| 方面 | 范围 |
|------|------|
| 共享契约字段 | `spec.Column` 新增 `GeneratedWhen`、`IsIdentity`、`IdentityOptions`（`CREATE TABLE` + `ALTER TABLE ADD COLUMN`） |
| Unsupported metadata | `UnsupportedDetail.Metadata` 携带 `column`、`generated_when`、`is_identity`、`identity_options` |
| Surface 契约 | CLI / pkg / HTTP 暴露 metadata；MCP 限制（metadata 未直接展示） |
| 延迟领域 | `GeneratedExpression`、ALTER TABLE 状态转换 metadata |

## PostgreSQL 边界支持就绪门控（`v0.32.0`）

`v0.32.0` 是 **PostgreSQL 边界支持就绪门控**。这是一个决策里程碑——不是功能发布。Characterization 测试记录了 generated 和 identity 列的稳定 AST 事实；就绪报告推荐 `v0.33.0` 作为窄事实保留包。

| 方面 | 说明 |
|------|------|
| Characterization 测试 | `parser_test.go` 中 7 个测试记录 `GeneratedWhen` 编码、约束类型、序列选项结构 |
| 就绪报告 | 完整边界清单、AST 事实覆盖、v0.33.0 推荐 |
| 新 rule ID | 无 |
| 新 CLI / API 标志 | 无 |
| 生产代码变更 | 无 |

未新增审核能力、规则或 surface 契约。

## PostgreSQL ALTER TABLE GENERATED 后续边界包（`v0.31.0`）

`v0.31.0` 是 **PostgreSQL ALTER TABLE GENERATED 后续边界包**。它将额外的 PostgreSQL generated/identity `ALTER TABLE` 形态映射到显式 unsupported feature 标签，收口了 `v0.30.0` 留下的相邻间隙。这些结果是显式 unsupported 契约，不是新的规则 finding。

| 方面 | 说明 |
|------|------|
| Drop expression | `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION` → `generated_column` |
| Set generated | `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ...` → `generated_as_identity` |
| Drop identity | `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY` → `generated_as_identity` |
| 锁定方式 | 语料用例、service 检查，以及 CLI、HTTP、MCP、`pkg/deltascope` 的表面对等 |
| 新 rule ID | 无 |
| 新 CLI / API 标志 | 无 |

这是边界收紧，不是 generated-column 支持、identity-column 支持，也不是完整的 PostgreSQL `ALTER TABLE` 支持。

## PostgreSQL ALTER TABLE GENERATED Boundary Pack（`v0.30.0`）

`v0.30.0` 是 **PostgreSQL ALTER TABLE GENERATED Boundary Pack**。它收紧了 PostgreSQL `ALTER TABLE ... ADD COLUMN` 在 generated stored / identity 语义下的不支持边界契约。这些结果是显式 unsupported 契约，不是新的规则 finding。

| 方面 | 说明 |
|------|------|
| Generated stored add-column | `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS (...) STORED` → `generated_column` |
| Identity add-column | `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS IDENTITY` → `generated_as_identity` |
| 锁定方式 | 语料用例、service 检查，以及 CLI、HTTP、MCP、`pkg/deltascope` 的表面对等 |
| 相邻形态 | `DROP EXPRESSION`、`SET GENERATED`、`DROP IDENTITY` 现已在 `v0.31.0` 中获得显式 unsupported 映射 |
| 新 rule ID | 无 |
| 新 CLI / API 标志 | 无 |

这是边界收紧，不是 generated-column 支持、identity-column 支持，也不是广义的 PostgreSQL `ALTER TABLE` 支持。

## Schema-Aware FK Policy Pack（`v0.29.0`）

`v0.29.0` 是 **Schema-Aware FK Policy Pack**。这是第一个 schema-aware FK policy 步骤：当 owning table schema 与 referenced schema 都显式存在且两者不同，DeltaScope 会发出 PostgreSQL-only notice 规则 `ddl.pg.table.foreign_key.cross_schema.advisory`。

| 方面 | 说明 |
|------|------|
| 新 rule ID | `ddl.pg.table.foreign_key.cross_schema.advisory` |
| 默认级别 | `notice` |
| 触发条件 | 仅 PostgreSQL；owning table schema 显式存在；referenced schema 显式存在；两者不同 |
| Same-schema FK | 不发出 advisory |
| 裸 `REFERENCES users(id)` | 不发出 advisory；referenced schema 保持 unknown |
| Metadata | finding 可包含 `table_schema`、`referenced_schema`、`referenced_table`、`referenced_columns` |
| 规范化表示 | `referenced_table` 始终是 `"users"`，不会写成 `"auth.users"` |

这不是完整的 PostgreSQL 外键支持，不是跨 schema 校验引擎，也不是 `search_path`-aware 行为。

## Referenced-Object Metadata Surface（`v0.28.0`）

`v0.28.0` 是 **Referenced-Object Metadata Surface Pack**。它将 PostgreSQL 被引用对象事实（`referenced_schema`、`referenced_table`、`referenced_columns`）从共享语义契约中以 additive 方式暴露到 FK forbid 规则的 finding metadata，覆盖 CLI、HTTP、MCP 和 `pkg/deltascope` 四条传输面。这是 additive metadata widening，不是新规则族。

| 方面 | 说明 |
|------|------|
| Widened metadata | `ddl.table.foreign_key.forbid` finding metadata 现在在底层约束携带被引用对象事实时，会包含 `referenced_schema`、`referenced_table`、`referenced_columns` |
| 条件发射 | `referenced_schema` 在无 schema 限定符时省略；`referenced_table` 和 `referenced_columns` 在所有携带这些事实的 FK 约束中出现 |
| 规范化表示 | `referenced_table` 不会拼接成 `"public.users"`——schema 和 table 始终是独立字段 |
| 锁定方式 | 覆盖 CLI、HTTP、MCP 和 `pkg/deltascope` 的 surface 测试 |
| 没有新 rule ID | `ddl.table.foreign_key.forbid` 规则不变，仅 finding metadata 更宽 |

这不是 schema-aware FK 策略支持，不是完整的 PostgreSQL 外键支持，也不是新规则族。

## Schema-Qualified Reference 语义（`v0.27.0`）

`v0.27.0` 是 **Schema-Qualified Reference Semantics Pack**。它在共享 `spec.Constraint` 契约中保留了 PostgreSQL schema-qualified 被引用对象事实。这是语义契约保留，不是新规则族。

| 方面 | 说明 |
|------|------|
| 新增字段 | `spec.Constraint` 上的 `ReferencedSchema` |
| 规范化表示 | `ReferencedSchema = "public"`，`ReferencedTable = "users"`（从不拼接） |
| 锁定方式 | 语料用例 + 服务层语义测试 |
| 公共 finding 元数据 | `v0.28.0` 已将 `referenced_schema`、`referenced_table`、`referenced_columns` 暴露到 FK forbid finding 输出 |

当前公共传输面已在 FK forbid finding metadata 中暴露被引用对象字段（`v0.28.0`）。这不是完整的 PostgreSQL 外键支持，也不是 schema-aware 规则支持。

## PostgreSQL 不支持边界（`v0.26.0`）

`v0.26.0` 是 **PostgreSQL CREATE TABLE 不支持边界收口包**。它在提取器层收口了 PostgreSQL `CREATE TABLE` 中明确不在支持范围内的语法边界：

| 特性 | 提取器标签 | Surface 契约 |
|------|-----------|-------------|
| Identity 列（`GENERATED ... AS IDENTITY`） | `generated_as_identity` | Unsupported |
| Generated stored 列（`GENERATED ALWAYS AS ... STORED`） | `generated_column` | Unsupported |
| Exclusion 约束（`EXCLUDE USING`） | `exclusion_constraint` | Unsupported |
| 分区表（`PARTITION BY`） | `partitioning` | Unsupported |

每条边界由语料用例和表面对等测试锁定。没有新增规则 ID——这些是提取器层的 unsupported 契约，不是规则 finding。

不支持语句的 surface 契约：

- **CLI** 和 **`pkg/deltascope`**：返回带 `unsupported` 数组的部分结果，以及 `ErrUnsupportedStatement`。
- **HTTP** 和 **MCP**：作为传输层错误暴露（HTTP 错误响应、MCP tool error）。
- **`v0.30.0` 说明**：PostgreSQL `ALTER TABLE ... ADD COLUMN` 的 generated / identity 形态现在也通过 `generated_column` 与 `generated_as_identity` 走相同的显式 unsupported 契约；相邻的 `DROP EXPRESSION`、`SET GENERATED`、`DROP IDENTITY` 现已在 `v0.31.0` 中获得显式 unsupported 映射。
- **`v0.33.0` 说明**：Unsupported 结果现在通过 `UnsupportedDetail.Metadata` 暴露结构化 metadata（`column`、`generated_when`、`is_identity`、`identity_options`）。CLI / pkg / HTTP 通道直接展示 metadata；MCP 通道受限于当前 transport 未直接展示 metadata 字段。

---

## DML

### WHERE 条件 / 安全防护

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `dml.where.require` | UPDATE 或 DELETE 语句缺少 WHERE 子句 | ✓ | ✗ | blocker |
| `dml.subquery.forbid` | 语句包含子查询 | ✓ | ✗ | warning |
| `dml.order_by.forbid` | 语句包含 ORDER BY 子句 | ✓ | ✗ | warning |
| `dml.limit.forbid` | 语句包含 LIMIT 子句 | ✓ | ✗ | warning |
| `dml.join.on.require` | JOIN 缺少 ON 条件 | ✓ | ✗ | blocker |
| `dml.replace.forbid` | 不允许使用 REPLACE INTO 语句 | ✓ | ✗ | blocker |

### INSERT 限制

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `dml.insert.select.forbid` | 不允许使用 INSERT … SELECT | ✓ | ✗ | blocker |
| `dml.insert.on_duplicate.forbid` | 不允许使用 INSERT … ON DUPLICATE KEY UPDATE | ✓ | ✗ | blocker |
| `dml.insert.rows.max_count` | INSERT 语句插入的行数超过允许的最大值 | ✓ | ✗ | warning |

### 表拒绝列表

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `dml.table.denylist.forbid` | DML 操作的目标表在配置的 Schema 级或表级拒绝列表中 | ✓ | ✗ | blocker |

---

## 元数据感知能力

配置元数据提供者后，DeltaScope 在评估前会从目标数据库加载实时信息，以启用上述元数据感知规则。未配置提供者时，所有元数据感知规则将被静默跳过，审计始终可以安全地在离线模式下运行。

### 实例信息（Instance Facts）

| 信息项 | 启用的能力 |
|--------|-----------|
| `version` | 版本条件性规则行为（如方言特定的语法检查） |
| `character_set_database` | 为未指定字符集的表解析默认字符集 |
| `innodb_large_prefix` | 计算索引键长度限制（`ddl.index.key_length.max_bytes.require`） |
| `innodb_default_row_format` | 对使用实例默认行格式的表进行行大小估算 |
| `innodb_adaptive_hash_index` | 在 DROP 和 TRUNCATE 时触发自适应哈希索引警告 |

### 表快照（Table Snapshot）

| 快照字段 | 启用的能力 |
|---------|-----------|
| 列定义 | ADD/DROP/MODIFY/CHANGE/RENAME 时的列存在性检查；类型兼容性验证 |
| 索引定义 | ADD/DROP/RENAME 时的索引存在性检查；与现有索引的冗余性检查 |
| 主键形态 | DROP PRIMARY KEY 前的主键存在性检查 |
| 表选项（引擎、字符集、行格式） | ALTER TABLE 时的表选项兼容性检查 |
| `table_rows` | DROP TABLE 和 TRUNCATE TABLE 的行数安全阈值检查 |

### 对象元数据（Object Metadata，v0.90.0）

PostgreSQL 元数据感知审核现在解析选定的非表对象元数据，并将安全属性注入生命周期规则发现：

| 对象类型 | 示例 SQL | 投射属性 |
|---------|---------|---------|
| `schema` | `DROP SCHEMA old_schema` | status/name/exists |
| `type` | `DROP TYPE app.color` | `type_kind` |
| `domain` | `DROP DOMAIN app.email_address` | `has_check` |
| `extension` | `DROP EXTENSION pgcrypto` | `extension_version`, `enabled` |
| `sequence` | `DROP SEQUENCE ticket_seq` | status/name/exists |
| `materialized_view` | `DROP MATERIALIZED VIEW user_summary` | status/name/exists |
| `publication` | `DROP PUBLICATION pub_users` | status/name/exists |
| `foreign_server` | `DROP SERVER fs_test` | `foreign_data_wrapper`, `has_options` |
| `user_mapping` | `DROP USER MAPPING FOR ... SERVER fs_test` | `server` |
| `comment` | `COMMENT ON TABLE users IS '...'` | `target_type` |

发现中出现 `metadata_status`（`confirmed`/`not_found`/`unavailable`）、`metadata_exists`、`metadata_object_type`、`metadata_object_name`、`metadata_schema` 等字段。仅 8 个安全属性键可投射到发现中；password、secret、connection、body、definition、comment、label、query、action_sql、options 等敏感属性被双重黑名单/白名单过滤。MySQL/TiDB 对象元数据解析返回 `unavailable` — 无行为变更。

---

## 信任与误配防护

这些是增量行为（不是规则），帮助识别方言误配和未支持的功能面。不改变规则评估或触发条件。

| 能力 | 状态 | 说明 |
|------|------|------|
| PostgreSQL 语法启发式通知 | 已覆盖 | 在 MySQL/TiDB 路径审计时，检测常见 PG 专属语法标记（`RETURNING`、`ON CONFLICT`、`::`、`ALTER COLUMN TYPE USING`、`GENERATED AS IDENTITY`），发出 `dialect.postgresql.syntax.detected.notice` 全局建议性告警。DeltaScope 不会自动切换方言。 |
| PostgreSQL 能力边界错误 | 已覆盖 | 未支持的 PG 功能面返回类型化的 `PostgreSQLCapabilityBoundaryError`，取代启发式字符串匹配，使 CI 和工具能区分已知限制和真正的失败。 |
| 离线信任上下文可见性 | 已覆盖 | CLI 输出格式（json、markdown、quiet）报告审计上下文：markdown 包含 `## Audit Context` 区段和信任提示；JSON 包含 `context` 对象；quiet 包含 `[context]` 行。`github-actions` 和 `sarif` 格式仅输出告警结果，不包含上下文元数据。 |
| 规则摘要 / 跳过规则可见性 | 已覆盖 | CLI 输出格式（json、markdown、quiet）报告已加载、适用和跳过的规则计数，方便确认当前方言下哪些规则运行了。`github-actions` 和 `sarif` 格式仅输出告警结果，不包含规则摘要元数据。 |
| 启发式误报排除 | 已覆盖 | PostgreSQL 语法启发式不对字符串字面量、双引号标识符、反引号标识符、行注释或块注释中的标记触发。 |
| GitLab Code Quality 输出 | 已覆盖 | `--format gitlab-codequality` 生成 GitLab Code Quality 报告（`gl-code-quality-report.json`），用于合并请求小组件和差异标注。所有套餐（Free+）。见 [use-deltascope-in-gitlab-ci.zh-CN.md](../recipe/use-deltascope-in-gitlab-ci.zh-CN.md)。 |
| 源码位置保真 | 已覆盖 | CI 渲染器（GitHub Actions、SARIF、GitLab Code Quality）通过渐进式源码映射器为每条发现携带原始文件路径和语句起始行号。支持 MySQL、TiDB 和 PostgreSQL 方言的多语句迁移文件。 |
