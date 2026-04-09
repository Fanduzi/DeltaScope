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

## DDL：PostgreSQL 迁移安全

这些规则用于防范常见的 PostgreSQL 迁移模式，避免引发全表重写、长时间持锁或生产事故。仅在设置 `--dialect postgresql` 时生效，MySQL/TiDB 方言下自动跳过。

| 规则 ID | 检查描述 | 离线 | 元数据 | 默认级别 |
|---------|---------|:----:|:------:|---------|
| `ddl.pg.create_index.concurrently.require` | 不带 `CONCURRENTLY` 的 `CREATE INDEX` 持有排他锁，阻塞读写 | ✓ | ✗ | warning |
| `ddl.pg.alter.add_column.non_null_default.rewrite.warn` | 添加带 volatile 默认值的 `NOT NULL` 列可能触发全表重写 | ✓ | ✗ | warning |
| `ddl.pg.alter.add_check.not_valid.require` | 不带 `NOT VALID` 的 `ADD CHECK` 需要持 `ACCESS EXCLUSIVE` 锁的全表扫描 | ✓ | ✗ | warning |
| `ddl.pg.alter.set_data_type.rewrite.warn` | 更改列类型可能需要全表重写（取决于类型转换） | ✓ | ✗ | warning |

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
