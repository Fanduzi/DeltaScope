# Audit Capability Matrix

This matrix lists every rule shipped with DeltaScope, its rule ID, whether it runs in offline mode, whether it requires live metadata, and its default finding level. Use this reference to understand what DeltaScope will and will not flag for a given SQL statement and audit configuration.

**Offline** rules fire on SQL text alone — no database connection required. **Metadata-aware** rules additionally consume live schema or instance facts when a metadata provider is configured; without metadata they are silently skipped.

---

## DDL: Create Table

### Table-Level Checks

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.table.name.max_length` | Table name exceeds the maximum allowed length | ✓ | ✗ | warning |
| `ddl.table.name.pattern.require` | Table name does not match the required naming pattern | ✓ | ✗ | warning |
| `ddl.table.name.keyword.forbid` | Table name is a reserved SQL keyword | ✓ | ✗ | blocker |
| `ddl.table.comment.require` | Table is missing a COMMENT clause | ✓ | ✗ | warning |
| `ddl.table.comment.max_length` | Table comment exceeds the maximum allowed length | ✓ | ✗ | warning |
| `ddl.table.engine.allowlist` | Storage engine is not on the permitted list | ✓ | ✗ | blocker |
| `ddl.table.charset.allowlist` | Table character set is not on the permitted list | ✓ | ✗ | blocker |
| `ddl.table.row_format.allowlist` | ROW_FORMAT value is not on the permitted list | ✓ | ✗ | warning |
| `ddl.table.auto_increment.init_value.require` | AUTO_INCREMENT initial value does not meet the required minimum | ✓ | ✗ | warning |
| `ddl.table.columns.min_count` | Table has fewer columns than the required minimum | ✓ | ✗ | blocker |
| `ddl.table.primary_key.require` | Table has no PRIMARY KEY defined | ✓ | ✗ | blocker |
| `ddl.table.primary_key.columns.max_count` | Primary key spans more columns than the allowed maximum | ✓ | ✗ | warning |
| `ddl.table.primary_key.bigint.require` | Primary key column is not BIGINT | ✓ | ✗ | warning |
| `ddl.table.primary_key.unsigned.require` | Primary key column is not UNSIGNED | ✓ | ✗ | warning |
| `ddl.table.primary_key.auto_increment.require` | Primary key column is not AUTO_INCREMENT | ✓ | ✗ | warning |
| `ddl.table.primary_key.not_null.require` | Primary key column is nullable | ✓ | ✗ | blocker |
| `ddl.table.audit_columns.require` | Required audit timestamp columns (e.g. `created_at`, `updated_at`) are missing | ✓ | ✗ | warning |
| `ddl.table.create_as.forbid` | CREATE TABLE … AS SELECT is not permitted | ✓ | ✗ | blocker |
| `ddl.table.create_like.forbid` | CREATE TABLE … LIKE is not permitted | ✓ | ✗ | blocker |
| `ddl.table.foreign_key.forbid` | Foreign key constraints are not permitted | ✓ | ✗ | blocker |
| `ddl.table.partition.forbid` | Partitioned tables are not permitted | ✓ | ✗ | blocker |
| `ddl.table.row_size.max_bytes.require` | Estimated row size exceeds the InnoDB row-size limit given the instance's row format | ✗ | ✓ | warning |
| `ddl.table.denylist.forbid` | Table name matches a schema- or table-level denylist entry | ✓ | ✗ | blocker |

### Column-Level Checks

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.column.name.max_length` | Column name exceeds the maximum allowed length | ✓ | ✗ | warning |
| `ddl.column.name.pattern.require` | Column name does not match the required naming pattern | ✓ | ✗ | warning |
| `ddl.column.name.keyword.forbid` | Column name is a reserved SQL keyword | ✓ | ✗ | blocker |
| `ddl.column.comment.require` | Column is missing a COMMENT clause | ✓ | ✗ | warning |
| `ddl.column.default.require` | Column has no DEFAULT value defined | ✓ | ✗ | warning |
| `ddl.column.not_null.require` | Column is nullable (missing NOT NULL) | ✓ | ✗ | warning |
| `ddl.column.varchar.max_length` | VARCHAR length exceeds the permitted maximum | ✓ | ✗ | warning |
| `ddl.column.char.max_length` | CHAR length exceeds the recommended maximum | ✓ | ✗ | notice |
| `ddl.column.float_double.forbid` | FLOAT or DOUBLE type is not permitted; use DECIMAL instead | ✓ | ✗ | blocker |
| `ddl.column.blob_text.forbid` | BLOB or TEXT type is not permitted | ✓ | ✗ | blocker |
| `ddl.column.json.forbid` | JSON type is not permitted | ✓ | ✗ | blocker |
| `ddl.column.bit.forbid` | BIT type is not permitted | ✓ | ✗ | blocker |
| `ddl.column.timestamp.forbid` | TIMESTAMP type is not permitted; use DATETIME instead | ✓ | ✗ | warning |
| `ddl.column.charset.allowlist` | Column character set is not on the permitted list | ✓ | ✗ | blocker |
| `ddl.column.collation.allowlist` | Column collation is not on the permitted list | ✓ | ✗ | blocker |
| `ddl.column.charset_collation.match.require` | Column character set and collation are incompatible | ✓ | ✗ | blocker |

### Index-Level Checks

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.index.total.max_count` | Table has more indexes than the allowed maximum | ✓ | ✗ | warning |
| `ddl.index.columns.max_count` | An index spans more columns than the allowed maximum | ✓ | ✗ | warning |
| `ddl.index.unique.prefix.require` | Unique index name does not start with the required prefix | ✓ | ✗ | warning |
| `ddl.index.secondary.prefix.require` | Secondary (non-unique) index name does not start with the required prefix | ✓ | ✗ | warning |
| `ddl.index.fulltext.prefix.require` | Fulltext index name does not start with the required prefix | ✓ | ✗ | warning |
| `ddl.index.duplicate.forbid` | Two or more indexes cover the exact same set of columns | ✓ | ✗ | warning |
| `ddl.index.redundant_left_prefix.forbid` | An index is a left-prefix subset of another index, making it redundant | ✓ | ✗ | warning |
| `ddl.index.redundant_unique_overlap.forbid` | A non-unique index is made redundant by an overlapping unique index | ✓ | ✗ | warning |
| `ddl.index.key_length.max_bytes.require` | Index key length exceeds the InnoDB limit given the instance's `innodb_large_prefix` setting | ✗ | ✓ | warning |

### Other Create Table Checks

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.view.create.forbid` | CREATE VIEW is not permitted | ✓ | ✗ | blocker |

---

## DDL: Alter Table

### Structural Checks

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.alter.drop_column.forbid` | ALTER TABLE … DROP COLUMN is not permitted | ✓ | ✗ | blocker |
| `ddl.alter.drop_index.forbid` | ALTER TABLE … DROP INDEX is not permitted | ✓ | ✗ | blocker |
| `ddl.alter.drop_primary_key.forbid` | ALTER TABLE … DROP PRIMARY KEY is not permitted | ✓ | ✗ | blocker |
| `ddl.alter.rename_table.forbid` | Renaming the table via ALTER TABLE is not permitted | ✓ | ✗ | blocker |
| `ddl.alter.rename_column.forbid` | ALTER TABLE … RENAME COLUMN is not permitted | ✓ | ✗ | blocker |
| `ddl.alter.rename_index.forbid` | ALTER TABLE … RENAME INDEX is not permitted | ✓ | ✗ | blocker |
| `ddl.alter.change_column.forbid` | ALTER TABLE … CHANGE COLUMN is not permitted | ✓ | ✗ | blocker |
| `ddl.alter.modify_column.forbid` | ALTER TABLE … MODIFY COLUMN is not permitted | ✓ | ✗ | blocker |

### Type Compatibility Checks

These rules fire when a CHANGE COLUMN or MODIFY COLUMN operation is not globally forbidden and a metadata snapshot of the current column type is available.

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.alter.table_option.compatibility.require` | A table option change (e.g. charset) is incompatible with the current table state | ✗ | ✓ | blocker |

### Index Checks on Alter

Alter-path index checks reuse the same logic as CREATE TABLE.

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.index.duplicate.forbid` | Newly added index duplicates an existing or simultaneously added index | ✓ / ✓ | ✓ | warning |
| `ddl.index.redundant_left_prefix.forbid` | Newly added index is a left-prefix subset of an existing or simultaneously added index | ✓ / ✓ | ✓ | warning |
| `ddl.index.redundant_unique_overlap.forbid` | Newly added index is made redundant by an overlapping unique index | ✓ / ✓ | ✓ | warning |

### Existence Checks (Metadata-Aware Only)

These rules require a live table snapshot and are skipped in offline mode.

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.alter.column.add.exists` | Column being added already exists in the current schema | ✗ | ✓ | blocker |
| `ddl.alter.column.drop.exists` | Column being dropped does not exist in the current schema | ✗ | ✓ | blocker |
| `ddl.alter.column.modify.exists` | Column being modified does not exist in the current schema | ✗ | ✓ | blocker |
| `ddl.alter.column.change.exists` | Column being changed does not exist in the current schema | ✗ | ✓ | blocker |
| `ddl.alter.column.rename.exists` | Column being renamed does not exist in the current schema | ✗ | ✓ | blocker |
| `ddl.alter.index.add.exists` | Index being added already exists in the current schema | ✗ | ✓ | blocker |
| `ddl.alter.index.drop.exists` | Index being dropped does not exist in the current schema | ✗ | ✓ | blocker |
| `ddl.alter.index.rename.exists` | Index being renamed does not exist in the current schema | ✗ | ✓ | blocker |
| `ddl.alter.primary_key.drop.exists` | Primary key being dropped does not exist on the table | ✗ | ✓ | blocker |

### Global: Merge-Alter

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.alter.merge.mysql` | Multiple ALTER TABLE statements against the same table should be merged into one (MySQL) | ✓ | ✗ | warning |
| `ddl.alter.merge.tidb` | Multiple ALTER TABLE statements against the same table should be merged into one (TiDB) | ✓ | ✗ | notice |

---

## DDL: Object Lifecycle

### Drop Table

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.table.drop.forbid` | DROP TABLE is not permitted | ✓ | ✗ | blocker |
| `ddl.table.drop.exists` | Table being dropped does not exist in the current schema | ✗ | ✓ | warning |
| `ddl.table.drop.row_count` | Table has more rows than the configured safety threshold | ✗ | ✓ | warning |
| `ddl.table.drop.adaptive_hash` | Table has `innodb_adaptive_hash_index` enabled; dropping it may cause latency spikes | ✗ | ✓ | notice |

### Truncate Table

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.table.truncate.forbid` | TRUNCATE TABLE is not permitted | ✓ | ✗ | blocker |
| `ddl.table.truncate.exists` | Table being truncated does not exist in the current schema | ✗ | ✓ | warning |
| `ddl.table.truncate.row_count` | Table has more rows than the configured safety threshold | ✗ | ✓ | warning |
| `ddl.table.truncate.adaptive_hash` | Table has `innodb_adaptive_hash_index` enabled; truncating it may cause latency spikes | ✗ | ✓ | notice |

---

## DML

### WHERE / Safety Guards

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `dml.where.require` | UPDATE or DELETE statement is missing a WHERE clause | ✓ | ✗ | blocker |
| `dml.subquery.forbid` | Statement contains a subquery | ✓ | ✗ | warning |
| `dml.order_by.forbid` | Statement contains an ORDER BY clause | ✓ | ✗ | warning |
| `dml.limit.forbid` | Statement contains a LIMIT clause | ✓ | ✗ | warning |
| `dml.join.on.require` | JOIN does not have an ON condition | ✓ | ✗ | blocker |
| `dml.replace.forbid` | REPLACE INTO statement is not permitted | ✓ | ✗ | blocker |

### INSERT Restrictions

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `dml.insert.select.forbid` | INSERT … SELECT is not permitted | ✓ | ✗ | blocker |
| `dml.insert.on_duplicate.forbid` | INSERT … ON DUPLICATE KEY UPDATE is not permitted | ✓ | ✗ | blocker |
| `dml.insert.rows.max_count` | INSERT statement inserts more rows than the permitted maximum | ✓ | ✗ | warning |

### Table Denylists

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `dml.table.denylist.forbid` | DML targets a table on the configured schema- or table-level denylist | ✓ | ✗ | blocker |

---

## Metadata-Aware Capabilities

When a metadata provider is configured, DeltaScope loads live facts from the target database before evaluation. These facts unlock the metadata-gated rules listed above. Without a provider, all metadata-aware rules are silently skipped and the audit remains safe to run offline.

### Instance Facts

| Fact | What It Enables |
|------|-----------------|
| `version` | Version-conditioned rule behavior (e.g. dialect-specific syntax guards) |
| `character_set_database` | Default charset resolution for tables that omit an explicit charset |
| `innodb_large_prefix` | Index key length limit calculation (`ddl.index.key_length.max_bytes.require`) |
| `innodb_default_row_format` | Row size estimation for tables using the instance default row format |
| `innodb_adaptive_hash_index` | Adaptive hash cautions on DROP and TRUNCATE |

### Table Snapshot

| Snapshot Field | What It Enables |
|----------------|-----------------|
| Column definitions | Column existence checks on ADD/DROP/MODIFY/CHANGE/RENAME; type-compatibility guards |
| Index definitions | Index existence checks on ADD/DROP/RENAME; redundancy checks against existing indexes |
| Primary key shape | Primary key existence check before DROP PRIMARY KEY |
| Table options (engine, charset, row format) | Table option compatibility check on ALTER TABLE |
| `table_rows` | Row count safety threshold on DROP TABLE and TRUNCATE TABLE |
