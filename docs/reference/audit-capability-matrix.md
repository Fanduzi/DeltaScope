# Audit Capability Matrix

This matrix lists every rule shipped with DeltaScope, its rule ID, whether it runs in offline mode, whether it requires live metadata, and its default finding level. Use this reference to understand what DeltaScope will and will not flag for a given SQL statement and audit configuration.

**Offline** rules fire on SQL text alone — no database connection required. **Metadata-aware** rules additionally consume live schema or instance facts when a metadata provider is configured; without metadata they are silently skipped.

**Pattern legality checks** such as `*.name.pattern.require` and `*.name.keyword.forbid` enforce lexical validity. **Structured naming governance** such as `prefix`, `suffix`, and `contains` enforces team naming conventions. These are complementary layers, not replacements for one another.

## Supported Dialects for Metadata-Aware Audit

| Dialect | Metadata Sources | Notes |
|---------|-----------------|-------|
| MySQL | `information_schema`, `performance_schema.global_variables`, `InnoDB` stats | Full support. Engine, row-format, adaptive-hash, and InnoDB-specific rules apply. |
| TiDB | `information_schema`, `performance_schema` (optional) | Same sources as MySQL. `performance_schema` is optional — DeltaScope falls back gracefully. |
| PostgreSQL | `pg_catalog`, `pg_constraint`, `pg_indexes`, `EXPLAIN` (read-only) | Supported for metadata-aware audit. MySQL-specific features (InnoDB, adaptive hash, row format) are not applicable. PG-specific: `ALTER TABLE … DROP CONSTRAINT` maps to primary-key detection; DML impact estimation uses the PostgreSQL planner via `EXPLAIN` for `UPDATE`/`DELETE`; object metadata resolution for selected non-table objects (types, domains, extensions, sequences, materialized views, schemas, foreign servers, user mappings, publications, comments) enriches lifecycle rule findings with `metadata_status`, `metadata_object_type`, `metadata_object_name`, `metadata_exists`, and safe projectable attributes. |

## Capability Status

| Capability | Status | Notes |
|------------|--------|-------|
| affected-row threshold | covered | the audit flow computes conservative statement-level impact estimates through shape and metadata-aware facts, and `dml.impact.estimate` / `dml.impact.rows.max_count` / `dml.impact.ratio.max_percent` consume or report that shared payload |

---

## DDL: Create Table

### Table-Level Checks

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.table.name.max_length` | Table name exceeds the maximum allowed length | ✓ | ✗ | warning |
| `ddl.table.name.prefix.require` | Table name does not start with the required structured naming prefix | ✓ | ✗ | warning |
| `ddl.table.name.suffix.require` | Table name does not end with the required structured naming suffix | ✓ | ✗ | warning |
| `ddl.table.name.contains.require` | Table name does not contain any configured structured naming token (OR semantics) | ✓ | ✗ | warning |
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
| `ddl.column.name.prefix.require` | Column name does not start with the required structured naming prefix | ✓ | ✗ | warning |
| `ddl.column.name.suffix.require` | Column name does not end with the required structured naming suffix | ✓ | ✗ | warning |
| `ddl.column.name.contains.require` | Column name does not contain any configured structured naming token (OR semantics) | ✓ | ✗ | warning |
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
| `ddl.index.name.pattern.require` | Index name does not match the required lexical naming pattern | ✓ | ✗ | warning |
| `ddl.index.name.keyword.forbid` | Index name is a reserved SQL keyword | ✓ | ✗ | blocker |
| `ddl.index.unique.prefix.require` | Unique index name does not start with the required prefix | ✓ | ✗ | warning |
| `ddl.index.unique.suffix.require` | Unique index name does not end with the required structured naming suffix | ✓ | ✗ | warning |
| `ddl.index.unique.contains.require` | Unique index name does not contain any configured structured naming token (OR semantics) | ✓ | ✗ | warning |
| `ddl.index.secondary.prefix.require` | Secondary (non-unique) index name does not start with the required prefix | ✓ | ✗ | warning |
| `ddl.index.secondary.suffix.require` | Secondary (non-unique) index name does not end with the required structured naming suffix | ✓ | ✗ | warning |
| `ddl.index.secondary.contains.require` | Secondary (non-unique) index name does not contain any configured structured naming token (OR semantics) | ✓ | ✗ | warning |
| `ddl.index.fulltext.prefix.require` | Fulltext index name does not start with the required prefix | ✓ | ✗ | warning |
| `ddl.index.fulltext.suffix.require` | Fulltext index name does not end with the required structured naming suffix | ✓ | ✗ | warning |
| `ddl.index.fulltext.contains.require` | Fulltext index name does not contain any configured structured naming token (OR semantics) | ✓ | ✗ | warning |
| `ddl.index.duplicate.forbid` | Two or more indexes cover the exact same set of columns | ✓ | ✗ | warning |
| `ddl.index.redundant_left_prefix.forbid` | An index is a left-prefix subset of another index, making it redundant | ✓ | ✗ | warning |
| `ddl.index.redundant_unique_overlap.forbid` | A non-unique index is made redundant by an overlapping unique index | ✓ | ✗ | warning |
| `ddl.index.key_length.max_bytes.require` | Index key length exceeds the InnoDB limit given the instance's `innodb_large_prefix` setting | ✗ | ✓ | warning |

**PostgreSQL index availability (v0.38.0, updated v0.49.0):** `ddl.index.secondary.prefix.require`, `ddl.index.unique.prefix.require`, and `ddl.index.columns.max_count` now also apply to standalone PostgreSQL `CREATE INDEX`, `CREATE UNIQUE INDEX`, and `CREATE INDEX CONCURRENTLY` statements. Since v0.49.0, partial indexes, expression indexes, INCLUDE covering indexes, and non-btree access methods (GIN, hash, etc.) normalize through the audit pipeline instead of returning unsupported. Operator classes and NULLS NOT DISTINCT remain out of scope.

**PostgreSQL ALTER TABLE constraint availability (v0.39.0):** `ALTER TABLE ... ADD PRIMARY KEY`, `ADD CONSTRAINT ... PRIMARY KEY`, `ADD UNIQUE`, and `ADD CONSTRAINT ... UNIQUE` forms now preserve statement-local constraint metadata. Existing primary-key rules (`ddl.table.primary_key.bigint.require`, `ddl.table.primary_key.columns.max_count`) and unique prefix rule (`ddl.alter.add_index.unique.prefix.require`) produce findings for approved forms. Foreign keys, check constraints, exclusion constraints, deferrability, validation lifecycle, partial/expression index semantics, operator classes, and live schema reconstruction remain out of scope.

**PostgreSQL ALTER TABLE FK availability (v0.40.0):** `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` forms now preserve statement-local FK facts (local columns, referenced table, referenced columns, referenced schema for schema-qualified references). Existing FK rules (`ddl.table.foreign_key.forbid`, `ddl.pg.table.foreign_key.cross_schema.advisory`) produce findings for ALTER TABLE FK additions. Check constraints, exclusion constraints, deferrability, MATCH FULL policy, live schema FK existence validation, and full constraint/index parity remain out of scope.

**PostgreSQL ALTER TABLE CHECK availability (v0.41.0):** `ALTER TABLE ... ADD CONSTRAINT ... CHECK` forms now preserve statement-local check constraint metadata (constraint name, check expression). Existing check naming rules (`ddl.constraint.check.name.prefix.require`, `ddl.constraint.check.name.suffix.require`, `ddl.constraint.check.name.contains.require`) and the PostgreSQL `NOT VALID` advisory (`ddl.pg.alter.add_check.not_valid.require`) produce findings for ALTER TABLE CHECK additions. Exclusion constraints, deferrability, `NOT VALID` validation enforcement, live schema CHECK existence validation, and full constraint/index parity remain out of scope.

**PostgreSQL NOT VALID validation pairing (v0.42.0):** named PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... NOT VALID` CHECK and FOREIGN KEY additions now participate in a batch-level GlobalRule (`ddl.pg.alter.not_valid_constraint.validate.require`). DeltaScope warns when the same audited SQL batch does not contain a later matching `ALTER TABLE ... VALIDATE CONSTRAINT ...` statement using the same schema, table, and constraint name. This is not first-time `VALIDATE CONSTRAINT` parser support, does not query live validation state, does not track cross-file deployment windows, skips unnamed constraints, and does not change MySQL/TiDB behavior.

**Default policy dialect isolation (v0.43.0):** The shipped default policy now isolates rules by `--dialect`. When `--dialect postgresql` is set, MySQL/TiDB-only rules (engine, charset, row format, unsigned/auto_increment primary-key requirements, partition, create_as/create_like, column charset/collation, change/modify column) and MySQL-only remediation text (`UNSIGNED`, `AUTO_INCREMENT`, `ON UPDATE CURRENT_TIMESTAMP`) are excluded. When `--dialect mysql` or `--dialect tidb` is set, `ddl.pg.*` and PostgreSQL-only dialect-gated rules are excluded. Isolation is enforced at the rule `AppliesTo` gate level, not by post-filtering.

### Constraint-Level Checks

Structured naming governance for constraints only evaluates explicitly named objects. Unnamed or implicit names are skipped. Foreign key naming rules are only relevant when foreign keys are allowed by policy; under the shipped default baseline, `ddl.table.foreign_key.forbid` suppresses foreign key naming checks.

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.constraint.primary_key.name.prefix.require` | Explicitly named primary key constraint does not start with the required structured naming prefix | ✓ | ✗ | warning |
| `ddl.constraint.primary_key.name.suffix.require` | Explicitly named primary key constraint does not end with the required structured naming suffix | ✓ | ✗ | warning |
| `ddl.constraint.primary_key.name.contains.require` | Explicitly named primary key constraint does not contain any configured structured naming token (OR semantics) | ✓ | ✗ | warning |
| `ddl.constraint.unique_key.name.prefix.require` | Explicitly named unique key constraint does not start with the required structured naming prefix | ✓ | ✗ | warning |
| `ddl.constraint.unique_key.name.suffix.require` | Explicitly named unique key constraint does not end with the required structured naming suffix | ✓ | ✗ | warning |
| `ddl.constraint.unique_key.name.contains.require` | Explicitly named unique key constraint does not contain any configured structured naming token (OR semantics) | ✓ | ✗ | warning |
| `ddl.constraint.foreign_key.name.prefix.require` | Explicitly named foreign key constraint does not start with the required structured naming prefix | ✓ | ✗ | warning |
| `ddl.constraint.foreign_key.name.suffix.require` | Explicitly named foreign key constraint does not end with the required structured naming suffix | ✓ | ✗ | warning |
| `ddl.constraint.foreign_key.name.contains.require` | Explicitly named foreign key constraint does not contain any configured structured naming token (OR semantics) | ✓ | ✗ | warning |
| `ddl.constraint.check.name.prefix.require` | Explicitly named check constraint does not start with the required structured naming prefix | ✓ | ✗ | warning |
| `ddl.constraint.check.name.suffix.require` | Explicitly named check constraint does not end with the required structured naming suffix | ✓ | ✗ | warning |
| `ddl.constraint.check.name.contains.require` | Explicitly named check constraint does not contain any configured structured naming token (OR semantics) | ✓ | ✗ | warning |

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

## DDL: MySQL/TiDB Database/Schema Lifecycle (v0.64.0)

`v0.64.0` normalizes MySQL/TiDB database and schema lifecycle DDL — `CREATE DATABASE`, `CREATE SCHEMA`, `DROP DATABASE`, `DROP SCHEMA` — through the audit pipeline instead of passing silently. In MySQL/TiDB, `SCHEMA` is a synonym for `DATABASE`. Two new rules provide notice and warning coverage. These rules only apply when `--dialect mysql` or `--dialect tidb` is set.

### Normalized Operations

| MySQL/TiDB DDL Action | Normalized As | Supported | Auditable | Rule-Mapped |
|------------------------|---------------|:---------:|:---------:|:-----------:|
| `CREATE DATABASE name` | `create_schema` (object_type=database) | ✓ | ✓ | ✓ |
| `CREATE DATABASE IF NOT EXISTS name` | `create_schema` (object_type=database, if_not_exists=true) | ✓ | ✓ | ✓ |
| `CREATE SCHEMA name` | `create_schema` (object_type=database) | ✓ | ✓ | ✓ |
| `DROP DATABASE name` | `drop_schema` (object_type=database) | ✓ | ✓ | ✓ |
| `DROP DATABASE IF EXISTS name` | `drop_schema` (object_type=database, if_exists=true) | ✓ | ✓ | ✓ |
| `DROP SCHEMA name` | `drop_schema` (object_type=database) | ✓ | ✓ | ✓ |

### Database/Schema Lifecycle Rules

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.database.create.notice` | `CREATE DATABASE` / `CREATE SCHEMA` creates a new logical namespace — informational notice | ✓ | ✗ | notice |
| `ddl.database.drop.warn` | `DROP DATABASE` / `DROP SCHEMA` removes a database and all contained objects — should be reviewed | ✓ | ✗ | warning |

> **Note:** These rules are MySQL/TiDB-specific and are automatically skipped when auditing PostgreSQL SQL. They are offline rules and do not require a database connection. DeltaScope does not perform live database existence validation. `CREATE DATABASE ... CHARACTER SET` / `COLLATE` options are preserved as parser facts but no policy rule governs them. This is not full DDL support — trigger, routine, event, and database privilege lifecycle remain deferred.

---

## DDL: PostgreSQL Migration-Safety

These rules guard against common PostgreSQL migration patterns that can cause table rewrites, long-held locks, or production incidents. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects. v0.120.0 enriches selected rules with bounded semantic metadata (index shape, default classification, USING presence); metadata is additive and does not emit raw SQL text.

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.pg.create_index.concurrently.require` | `CREATE INDEX` without `CONCURRENTLY` holds an exclusive lock, blocking reads and writes. Finding metadata: `index_kind`, `access_method`, `column_count`, `included_column_count`, `has_predicate`, `has_expression_keys`, `expression_count` (bounded, no SQL text). | ✓ | ✗ | warning |
| `ddl.pg.alter.add_column.non_null_default.rewrite.warn` | Adding a `NOT NULL` column with a volatile default may trigger a full table rewrite. Finding metadata: `not_null`, `has_default`, `default_kind` (bounded classification, no expression text). | ✓ | ✗ | warning |
| `ddl.pg.alter.add_check.not_valid.require` | `ADD CHECK` constraint without `NOT VALID` requires a full table scan with `ACCESS EXCLUSIVE` lock | ✓ | ✗ | warning |
| `ddl.pg.alter.set_data_type.rewrite.warn` | Changing a column type may require a full table rewrite depending on the conversion. Finding metadata: `has_using` (boolean, no USING expression text). | ✓ | ✗ | warning |
| `ddl.pg.alter.not_valid_constraint.validate.require` | Named CHECK/FK `NOT VALID` constraint lacks a later matching `VALIDATE CONSTRAINT` in the same audited SQL batch | ✓ | ✗ | warning |
| `ddl.pg.drop_index.advisory` | `DROP INDEX` removes an index — advises review of dependent queries | ✓ | ✗ | notice |
| `ddl.pg.alter.add_column.non_null_no_default.warn` | Adding a `NOT NULL` column without `DEFAULT` can cause a full table rewrite on large tables. Finding metadata: `not_null`, `has_default`. | ✓ | ✗ | warning |
| `ddl.pg.alter.add_unique_constraint.concurrent_index.advisory` | `ADD UNIQUE CONSTRAINT` without `NOT VALID` and no subsequent `CREATE UNIQUE INDEX CONCURRENTLY` — advises concurrent index creation | ✓ | ✗ | notice |
| `ddl.pg.alter.drop_constraint.advisory` | `DROP CONSTRAINT` removes a CHECK, UNIQUE, or FK constraint — advises review of data integrity | ✓ | ✗ | notice |

---

## DDL: PostgreSQL Object Lifecycle (v0.50.0, updated v0.64.0)

`v0.50.0` normalizes PostgreSQL object lifecycle DDL — schema, sequence, and materialized view CREATE/DROP/ALTER operations — through the audit pipeline instead of returning unsupported. Nine PostgreSQL-only rules guard against cascade drops, sequence wraparound, and sequence counter resets. `v0.64.0` adds `ddl.pg.create_schema.notice` for `CREATE SCHEMA` coverage. These rules only apply when `--dialect postgresql` is set.

### Normalized Operations

| PostgreSQL DDL Action | Normalized As | Supported | Auditable | Rule-Mapped |
|-----------------------|---------------|:---------:|:---------:|:-----------:|
| `CREATE SCHEMA` | `create_schema` | ✓ | ✓ | ✓ |
| `DROP SCHEMA` | `drop_schema` | ✓ | ✓ | ✓ |
| `CREATE SEQUENCE` | `create_sequence` | ✓ | ✓ | ✓ |
| `ALTER SEQUENCE` | `alter_sequence` | ✓ | ✓ | ✓ |
| `DROP SEQUENCE` | `drop_sequence` | ✓ | ✓ | ✓ |
| `CREATE MATERIALIZED VIEW` | `create_materialized_view` | ✓ | ✓ | — |
| `DROP MATERIALIZED VIEW` | `drop_materialized_view` | ✓ | ✓ | ✓ |
| `REFRESH MATERIALIZED VIEW` | `refresh_materialized_view` | ✓ | ✓ | ✓ |

### Object Lifecycle Rules

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.pg.create_schema.notice` | `CREATE SCHEMA` creates a new namespace — informational notice | ✓ | ✗ | notice |
| `ddl.pg.drop_schema.advisory` | `DROP SCHEMA` removes a schema — advises review of dependent objects | ✓ | ✗ | notice |
| `ddl.pg.drop_schema.cascade.warn` | `DROP SCHEMA ... CASCADE` uses cascading deletion — may silently drop dependent objects | ✓ | ✗ | warning |
| `ddl.pg.create_sequence.cycle.warn` | `CREATE SEQUENCE ... CYCLE` may cause sequence value wraparound | ✓ | ✗ | warning |
| `ddl.pg.alter_sequence.restart.warn` | `ALTER SEQUENCE ... RESTART` resets the sequence counter — may conflict with existing rows | ✓ | ✗ | warning |
| `ddl.pg.alter_sequence.cycle.warn` | `ALTER SEQUENCE ... CYCLE` enables value wraparound on an existing sequence | ✓ | ✗ | warning |
| `ddl.pg.drop_sequence.advisory` | `DROP SEQUENCE` removes a sequence — advises review of dependent columns | ✓ | ✗ | notice |
| `ddl.pg.drop_sequence.cascade.warn` | `DROP SEQUENCE ... CASCADE` uses cascading deletion — may silently drop dependent objects | ✓ | ✗ | warning |
| `ddl.pg.drop_materialized_view.advisory` | `DROP MATERIALIZED VIEW` removes a materialized view — advises review of dependent queries | ✓ | ✗ | notice |
| `ddl.pg.drop_materialized_view.cascade.warn` | `DROP MATERIALIZED VIEW ... CASCADE` uses cascading deletion — may silently drop dependent objects | ✓ | ✗ | warning |
| `ddl.pg.refresh_materialized_view.concurrently.warn` | Non-concurrent `REFRESH MATERIALIZED VIEW` holds an exclusive lock — warns on default or explicit `WITH DATA` refreshes | ✓ | ✗ | warning |
| `ddl.pg.refresh_materialized_view.no_data.notice` | `REFRESH MATERIALIZED VIEW ... WITH NO DATA` empties the view — downstream readers may see empty results | ✓ | ✗ | notice |

> **Note:** `CONCURRENTLY` refreshes pass both rules without findings. `WITH NO DATA` triggers both rules because it is also non-concurrent. This is not live unique-index validation for `CONCURRENTLY` — DeltaScope does not verify whether a unique index exists on the materialized view. This is not complete PostgreSQL object lifecycle coverage — remaining unsupported DDL forms (trigger, function, etc.) are still explicit boundaries.

---

## DDL: PostgreSQL Type Lifecycle (v0.55.0)

`v0.55.0` adds PostgreSQL type lifecycle coverage for enum types and type drops. DeltaScope normalizes `CREATE TYPE ... AS ENUM`, `ALTER TYPE ... ADD VALUE`, and `DROP TYPE`, adds five PostgreSQL-only findings, and keeps composite types and domains as explicit unsupported boundaries. These rules only apply when `--dialect postgresql` is set.

### Normalized Operations

| SQL | Normalized Operation |
|-----|---------------------|
| `CREATE TYPE color AS ENUM ('red', 'green', 'blue')` | `create_type` (type_kind=enum, labels=red,green,blue) |
| `ALTER TYPE color ADD VALUE 'yellow'` | `alter_type` (type_kind=enum, action=add_value, value=yellow) |
| `ALTER TYPE color ADD VALUE IF NOT EXISTS 'yellow'` | `alter_type` (type_kind=enum, action=add_value, value=yellow, if_not_exists=true) |
| `ALTER TYPE color ADD VALUE 'yellow' BEFORE 'green'` | `alter_type` (type_kind=enum, action=add_value, value=yellow, placement=before, neighbor=green) |
| `ALTER TYPE color ADD VALUE 'yellow' AFTER 'green'` | `alter_type` (type_kind=enum, action=add_value, value=yellow, placement=after, neighbor=green) |
| `DROP TYPE color` | `drop_type` |
| `DROP TYPE IF EXISTS color CASCADE` | `drop_type` (if_exists=true, cascade=true) |

### Type Lifecycle Rules

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.pg.create_type.enum.notice` | `CREATE TYPE ... AS ENUM` introduces a new enum type — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_type.add_value.advisory` | `ALTER TYPE ... ADD VALUE` appends a value to an existing enum — advises review of application usage | ✓ | ✗ | warning |
| `ddl.pg.alter_type.add_value.position.notice` | `ALTER TYPE ... ADD VALUE ... BEFORE/AFTER` positions a new enum value — informational notice | ✓ | ✗ | notice |
| `ddl.pg.drop_type.advisory` | `DROP TYPE` removes a user-defined type — advises review of dependent columns and functions | ✓ | ✗ | warning |
| `ddl.pg.drop_type.cascade.warn` | `DROP TYPE ... CASCADE` uses cascading deletion — may silently drop dependent objects | ✓ | ✗ | warning |

> **Note:** These rules are offline-only and do not require a database connection. DeltaScope does not inspect live dependent objects, validate whether enum values are already used by data or application code, or model full PostgreSQL type system semantics. This is not full PostgreSQL type lifecycle coverage. Composite types are now supported — see Composite Type Lifecycle below. Domains are supported — see Domain Lifecycle below. No MySQL/TiDB behavior changes.

---

## DDL: PostgreSQL Composite Type Lifecycle (v0.58.0)

`v0.58.0` adds PostgreSQL composite type lifecycle narrow support. DeltaScope normalizes `CREATE TYPE ... AS (...)`, `ALTER TYPE ... RENAME TO`, and `ALTER TYPE ... SET SCHEMA`, adds three PostgreSQL-only findings, and keeps attribute-level operations (`ADD ATTRIBUTE`, `DROP ATTRIBUTE`, `ALTER ATTRIBUTE ... TYPE`, `RENAME ATTRIBUTE`) as explicit unsupported/deferred boundaries. `DROP TYPE` reuses existing v0.55.0 rules. These rules only apply when `--dialect postgresql` is set.

### Normalized Operations

| SQL | Normalized Operation |
|-----|---------------------|
| `CREATE TYPE address AS (street text, city text)` | `create_type_composite` |
| `CREATE TYPE qualified.address AS (street text, city text)` | `create_type_composite` |
| `CREATE TYPE address AS (street text COLLATE "C", city text)` | `create_type_composite` (collation recorded but not interpreted) |
| `ALTER TYPE address RENAME TO mailing_address` | `alter_type` (action=rename) |
| `ALTER TYPE address SET SCHEMA archive` | `alter_type` (action=set_schema) |

### Composite Type Lifecycle Rules

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.pg.create_type.composite.notice` | `CREATE TYPE ... AS (...)` introduces a new composite type — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_type.composite_rename.notice` | `ALTER TYPE ... RENAME TO` changes the composite type name — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_type.composite_set_schema.notice` | `ALTER TYPE ... SET SCHEMA` moves the composite type to a different schema — informational notice | ✓ | ✗ | notice |

### Unsupported / Deferred Operations

| SQL | Unsupported Feature |
|-----|-------------------|
| `ALTER TYPE ... ADD ATTRIBUTE` | `alter_type_add_attribute` |
| `ALTER TYPE ... DROP ATTRIBUTE` | `alter_type_drop_attribute` |
| `ALTER TYPE ... ALTER ATTRIBUTE ... TYPE` | `alter_type_alter_attribute_type` |
| `ALTER TYPE ... RENAME ATTRIBUTE ... TO ...` | `alter_type_rename_attribute` |

> **Note:** These rules are offline-only and do not require a database connection. `DROP TYPE` is not covered by composite-specific rules — it reuses the existing `ddl.pg.drop_type.advisory` and `ddl.pg.drop_type.cascade.warn` from the Type Lifecycle family. Attribute-level operations are explicitly deferred. DeltaScope recognizes `COLLATE` annotations on composite type attributes structurally but does not render, interpret, or validate collation semantics. No MySQL/TiDB behavior changes.

---

## DDL: PostgreSQL Domain Lifecycle (v0.57.0)

`v0.57.0` adds PostgreSQL domain lifecycle coverage. DeltaScope normalizes `CREATE DOMAIN`, `ALTER DOMAIN` (constraint, default, not null, rename), and `DROP DOMAIN`, adds seven PostgreSQL-only findings, and keeps composite types as an explicit unsupported boundary. `CHECK` and `DEFAULT` expression text is intentionally not rendered — rules emit boolean facts and constraint names where available. These rules only apply when `--dialect postgresql` is set.

### Normalized Operations

| SQL | Normalized Operation |
|-----|---------------------|
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

### Domain Lifecycle Rules

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.pg.create_domain.notice` | `CREATE DOMAIN` introduces a reusable type constraint — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_domain.constraint.notice` | `ALTER DOMAIN ... ADD/DROP/VALIDATE CONSTRAINT` modifies the type contract — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_domain.default.notice` | `ALTER DOMAIN ... SET/DROP DEFAULT` changes the implicit value — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_domain.not_null.notice` | `ALTER DOMAIN ... SET/DROP NOT NULL` changes nullability — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_domain.rename.notice` | `ALTER DOMAIN ... RENAME TO` changes the domain name — informational notice | ✓ | ✗ | notice |
| `ddl.pg.drop_domain.advisory` | `DROP DOMAIN` removes a domain — advises review of dependent columns | ✓ | ✗ | warning |
| `ddl.pg.drop_domain.cascade.warn` | `DROP DOMAIN ... CASCADE` uses cascading deletion — may silently drop dependent objects | ✓ | ✗ | warning |

> **Note:** These rules are offline-only and do not require a database connection. DeltaScope does not render `CHECK` or `DEFAULT` expression text — rules emit boolean facts (`has_check`, `has_default`, `not_null`) and constraint names, but never the expression body. DeltaScope does not perform live dependency validation on domains. `DROP DOMAIN IF EXISTS ... CASCADE` intentionally emits both `ddl.pg.drop_domain.advisory` and `ddl.pg.drop_domain.cascade.warn`. Composite types are now supported — see Composite Type Lifecycle above. Extensions are now supported — see Extension Lifecycle below. No MySQL/TiDB behavior changes.

---

## DDL: PostgreSQL Extension Lifecycle (v0.59.0)

`v0.59.0` adds PostgreSQL extension lifecycle narrow support. DeltaScope normalizes `CREATE EXTENSION`, `ALTER EXTENSION` (`UPDATE`, `UPDATE TO`, `SET SCHEMA`), and `DROP EXTENSION`, adds six PostgreSQL-only findings, and keeps extension member mutation (`ALTER EXTENSION ... ADD/DROP TABLE`) as explicit unsupported/deferred boundaries. DeltaScope does not perform live validation of extension availability, installed packages, version compatibility, or dependency graphs. These rules only apply when `--dialect postgresql` is set.

### Normalized Operations

| SQL | Normalized Operation |
|-----|---------------------|
| `CREATE EXTENSION pg_trgm` | `create_extension` |
| `CREATE EXTENSION IF NOT EXISTS pg_trgm` | `create_extension` (if_not_exists=true) |
| `CREATE EXTENSION pg_trgm WITH SCHEMA utils` | `create_extension` (schema=utils) |
| `CREATE EXTENSION pg_trgm WITH VERSION '1.5'` | `create_extension` (version=1.5) |
| `CREATE EXTENSION pg_trgm CASCADE` | `create_extension` (cascade=true) |
| `ALTER EXTENSION pg_trgm UPDATE` | `alter_extension` (action=update) |
| `ALTER EXTENSION pg_trgm UPDATE TO '1.6'` | `alter_extension` (action=update_to) |
| `ALTER EXTENSION pg_trgm SET SCHEMA utils` | `alter_extension` (action=set_schema) |
| `DROP EXTENSION pg_trgm` | `drop_extension` |
| `DROP EXTENSION IF EXISTS pg_trgm` | `drop_extension` (if_exists=true) |
| `DROP EXTENSION pg_trgm CASCADE` | `drop_extension` (cascade=true) |

### Extension Lifecycle Rules

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.pg.create_extension.notice` | `CREATE EXTENSION` installs an extension into the database — informational notice | ✓ | ✗ | notice |
| `ddl.pg.create_extension.cascade.warn` | `CREATE EXTENSION ... CASCADE` auto-installs dependencies — may introduce unintended extensions | ✓ | ✗ | warning |
| `ddl.pg.alter_extension.update.notice` | `ALTER EXTENSION ... UPDATE` / `UPDATE TO` upgrades an extension — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_extension.set_schema.notice` | `ALTER EXTENSION ... SET SCHEMA` moves the extension to a different schema — informational notice | ✓ | ✗ | notice |
| `ddl.pg.drop_extension.advisory` | `DROP EXTENSION` removes an extension — advises review of dependent objects | ✓ | ✗ | warning |
| `ddl.pg.drop_extension.cascade.warn` | `DROP EXTENSION ... CASCADE` uses cascading deletion — may silently drop dependent objects | ✓ | ✗ | warning |

### Unsupported / Deferred Operations

| SQL | Unsupported Feature |
|-----|-------------------|
| `ALTER EXTENSION ... ADD TABLE` | `alter_extension_add_member` |
| `ALTER EXTENSION ... DROP TABLE` | `alter_extension_drop_member` |

> **Note:** These rules are offline-only and do not require a database connection. `CREATE EXTENSION ... CASCADE` intentionally emits both `ddl.pg.create_extension.notice` and `ddl.pg.create_extension.cascade.warn`. `DROP EXTENSION ... CASCADE` intentionally emits both `ddl.pg.drop_extension.advisory` and `ddl.pg.drop_extension.cascade.warn`. DeltaScope does not perform live validation of extension availability, installed packages, version compatibility, or dependency graphs. Extension member mutation (`ALTER EXTENSION ... ADD/DROP TABLE`) is explicitly unsupported/deferred. No MySQL/TiDB behavior changes.

---

## DDL: PostgreSQL Table Privilege DCL (v0.60.0)

`v0.60.0` adds PostgreSQL table-level privilege DCL narrow support. DeltaScope normalizes `GRANT ... ON TABLE` and `REVOKE ... ON TABLE`, adds four PostgreSQL-only findings, and keeps `ALL TABLES IN SCHEMA`, sequence privileges, role membership GRANT/REVOKE, and `ALTER DEFAULT PRIVILEGES` as explicit unsupported/deferred boundaries. DeltaScope does not perform live validation of any kind for table privileges. These rules only apply when `--dialect postgresql` is set.

### Normalized Operations

| SQL | Normalized Operation |
|-----|---------------------|
| `GRANT SELECT ON users TO reader` | `grant_table_privilege` |
| `GRANT SELECT, INSERT ON users TO reader, writer` | `grant_table_privilege` (privileges=[SELECT, INSERT], grantees=[reader, writer]) |
| `GRANT ALL PRIVILEGES ON users TO admin` | `grant_table_privilege` (all_privileges=true) |
| `GRANT SELECT ON public.users TO reader` | `grant_table_privilege` (schema=public) |
| `REVOKE SELECT ON users FROM reader` | `revoke_table_privilege` |
| `REVOKE INSERT, UPDATE ON users FROM writer, editor` | `revoke_table_privilege` (privileges=[INSERT, UPDATE], grantees=[writer, editor]) |
| `REVOKE ALL PRIVILEGES ON users FROM admin` | `revoke_table_privilege` (all_privileges=true) |
| `REVOKE SELECT ON users FROM reader CASCADE` | `revoke_table_privilege` (cascade=true) |

### Table Privilege DCL Rules

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.pg.grant.table_privilege.notice` | `GRANT ... ON TABLE` grants table-level privileges — informational notice | ✓ | ✗ | notice |
| `ddl.pg.grant.table_privilege.all.warn` | `GRANT ALL PRIVILEGES ON TABLE` grants all privileges — warns about over-permission | ✓ | ✗ | warning |
| `ddl.pg.revoke.table_privilege.notice` | `REVOKE ... ON TABLE` revokes table-level privileges — informational notice | ✓ | ✗ | notice |
| `ddl.pg.revoke.table_privilege.cascade.warn` | `REVOKE ... ON TABLE ... CASCADE` cascades revocation — warns about cascade side-effects | ✓ | ✗ | warning |

### Unsupported / Deferred Operations

| SQL | Status |
|-----|--------|
| `GRANT ... ON ALL TABLES IN SCHEMA` | Not supported |
| Sequence privileges (`GRANT ... ON SEQUENCE`) | Not supported |
| Role membership (`GRANT role TO role`) | Not supported |
| `ALTER DEFAULT PRIVILEGES` | Not supported |

> **Note:** These rules are offline-only and do not require a database connection. `GRANT ALL PRIVILEGES ON TABLE` intentionally emits both `ddl.pg.grant.table_privilege.notice` and `ddl.pg.grant.table_privilege.all.warn`. `REVOKE ... ON TABLE ... CASCADE` intentionally emits both `ddl.pg.revoke.table_privilege.notice` and `ddl.pg.revoke.table_privilege.cascade.warn`. DeltaScope does not perform live validation — no grantee/role existence checks, no table/object existence checks, no grantor permission checks, no effective privilege computation, no role inheritance resolution, no ownership verification, and no RLS/policy evaluation. This is narrow table-level privilege DCL support — not broad governance or admin DCL support. No MySQL/TiDB behavior changes.

---

## DDL: PostgreSQL ALTER TABLE Coverage (v0.51.0 / v0.52.0 / v0.54.0 / v0.56.0)

`v0.51.0` extends PostgreSQL ALTER TABLE audit coverage with three new gap-fill rules. `v0.52.0` adds six more rules covering previously unsupported ALTER TABLE actions. `v0.54.0` normalizes trigger-scope forms (`ENABLE/DISABLE TRIGGER ALL/USER`) to reuse existing trigger rules and adds three replica identity rules. `v0.56.0` adds two logged-state rules for `SET LOGGED` and `SET UNLOGGED`. These rules cover the most common ALTER TABLE safety patterns beyond the existing migration-safety and object lifecycle rule families. They only apply when `--dialect postgresql` is set.

### ALTER TABLE Coverage Rules

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.pg.alter.drop_column.advisory` | `ALTER TABLE ... DROP COLUMN` removes a column — advises review of dependent queries and application logic | ✓ | ✗ | warning |
| `ddl.pg.alter.validate_constraint.advisory` | `ALTER TABLE ... VALIDATE CONSTRAINT` runs a validation scan — advises awareness of table-level lock duration on large tables | ✓ | ✗ | notice |
| `ddl.pg.alter.add_column.nullable.notice` | `ALTER TABLE ... ADD COLUMN` adds a nullable column without a DEFAULT — note that downstream code may encounter unexpected NULL values | ✓ | ✗ | notice |
| `ddl.pg.alter.set_schema.advisory` | `ALTER TABLE ... SET SCHEMA` moves the table to a different schema — advises review of dependent queries and application connections | ✓ | ✗ | notice |
| `ddl.pg.alter.owner.advisory` | `ALTER TABLE ... OWNER TO` changes the table owner — advises review of permission implications | ✓ | ✗ | notice |
| `ddl.pg.alter.enable_trigger.notice` | `ALTER TABLE ... ENABLE TRIGGER name` re-enables a specific trigger — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter.disable_trigger.warn` | `ALTER TABLE ... DISABLE TRIGGER name` disables a specific trigger — warns that triggers will not fire on the table | ✓ | ✗ | warning |
| `ddl.pg.alter.attach_partition.advisory` | `ALTER TABLE ... ATTACH PARTITION` attaches a partition to a partitioned table — advises review of partition boundary and data routing | ✓ | ✗ | notice |
| `ddl.pg.alter.detach_partition.warn` | `ALTER TABLE ... DETACH PARTITION` detaches a partition — warns that queries targeting the partition may fail | ✓ | ✗ | warning |
| `ddl.pg.alter.replica_identity_full.warn` | `ALTER TABLE ... REPLICA IDENTITY FULL` writes full old-row images to WAL — warns about replication overhead | ✓ | ✗ | warning |
| `ddl.pg.alter.replica_identity_nothing.warn` | `ALTER TABLE ... REPLICA IDENTITY NOTHING` writes no old-row images to WAL — warns that logical replication will not work | ✓ | ✗ | warning |
| `ddl.pg.alter.replica_identity_using_index.notice` | `ALTER TABLE ... REPLICA IDENTITY USING INDEX ...` uses a specific index for WAL old-row images — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter.set_logged.notice` | `ALTER TABLE ... SET LOGGED` changes an unlogged table to logged — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter.set_unlogged.notice` | `ALTER TABLE ... SET UNLOGGED` changes a logged table to unlogged — informational notice | ✓ | ✗ | notice |

> **Note:** Trigger-scope forms (`ENABLE/DISABLE TRIGGER ALL/USER`) are now normalized and reuse the `enable_trigger` and `disable_trigger` rules above. `REPLICA IDENTITY DEFAULT` is normalized and intentionally silent. This is not full PostgreSQL ALTER TABLE coverage. `SET TABLESPACE` remains an explicit boundary. These rules are offline-only and do not require a database connection. DeltaScope does not verify whether `REPLICA IDENTITY USING INDEX` names a valid, unique, or non-partial index. DeltaScope does not verify whether the target table is currently logged or unlogged.

---

## DDL: PostgreSQL RLS/Policy Lifecycle (v0.70.0)

`v0.70.0` adds PostgreSQL Row-Level Security policy and RLS toggle lifecycle coverage. DeltaScope normalizes `CREATE POLICY`, `ALTER POLICY`, `DROP POLICY`, and `ALTER TABLE ... ENABLE/DISABLE/FORCE/NO FORCE ROW LEVEL SECURITY`, adds seven PostgreSQL-only findings. These rules only apply when `--dialect postgresql` is set.

### Normalized Operations

| SQL | Normalized Operation |
|-----|---------------------|
| `CREATE POLICY p1 ON users USING (true)` | `create_policy` |
| `CREATE POLICY p1 ON users AS RESTRICTIVE ...` | `create_policy` (policy_type=restrictive) |
| `ALTER POLICY p1 ON users USING (true)` | `alter_policy` |
| `DROP POLICY p1 ON users` | `drop_policy` |
| `ALTER TABLE users ENABLE ROW LEVEL SECURITY` | `alter_table` (action=enable_rls) |
| `ALTER TABLE users DISABLE ROW LEVEL SECURITY` | `alter_table` (action=disable_rls) |
| `ALTER TABLE users FORCE ROW LEVEL SECURITY` | `alter_table` (action=force_rls) |
| `ALTER TABLE users NO FORCE ROW LEVEL SECURITY` | `alter_table` (action=no_force_rls) |

### RLS/Policy Lifecycle Rules

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.pg.create_policy.notice` | `CREATE POLICY` introduces a new RLS policy — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_policy.notice` | `ALTER POLICY` modifies an existing RLS policy — informational notice | ✓ | ✗ | notice |
| `ddl.pg.drop_policy.warn` | `DROP POLICY` removes an RLS policy — warns that row-level protection is removed | ✓ | ✗ | warning |
| `ddl.pg.alter.enable_rls.notice` | `ALTER TABLE ... ENABLE ROW LEVEL SECURITY` enables RLS — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter.disable_rls.warn` | `ALTER TABLE ... DISABLE ROW LEVEL SECURITY` disables RLS — warns that protection is turned off | ✓ | ✗ | warning |
| `ddl.pg.alter.force_rls.notice` | `ALTER TABLE ... FORCE ROW LEVEL SECURITY` forces RLS for table owner — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter.no_force_rls.notice` | `ALTER TABLE ... NO FORCE ROW LEVEL SECURITY` un-forces RLS for table owner — informational notice | ✓ | ✗ | notice |

> **Note:** These rules are offline-only and do not require a database connection. DeltaScope does not evaluate policy expressions, verify policy applicability, or inspect live RLS state. This is not full PostgreSQL RLS governance. No MySQL/TiDB behavior changes.

---

## DDL: PostgreSQL Trigger Lifecycle (v0.70.0)

`v0.70.0` adds PostgreSQL trigger lifecycle coverage for `CREATE TRIGGER`, `CREATE CONSTRAINT TRIGGER`, and `DROP TRIGGER`. These rules only apply when `--dialect postgresql` is set.

### Normalized Operations

| SQL | Normalized Operation |
|-----|---------------------|
| `CREATE TRIGGER t1 AFTER INSERT ON users FOR EACH ROW EXECUTE FUNCTION f()` | `create_trigger` |
| `CREATE CONSTRAINT TRIGGER t1 AFTER INSERT ON users DEFERRABLE INITIALLY DEFERRED ...` | `create_constraint_trigger` |
| `DROP TRIGGER t1 ON users` | `drop_trigger` |

### Trigger Lifecycle Rules

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.pg.create_trigger.notice` | `CREATE TRIGGER` introduces a new trigger — informational notice | ✓ | ✗ | notice |
| `ddl.pg.create_constraint_trigger.warn` | `CREATE CONSTRAINT TRIGGER` creates a constraint trigger — warns about constraint-trigger semantics | ✓ | ✗ | warning |
| `ddl.pg.drop_trigger.advisory` | `DROP TRIGGER` removes a trigger — advises review of dependent logic | ✓ | ✗ | notice |

> **Note:** These rules are offline-only and do not require a database connection. DeltaScope does not evaluate trigger bodies or verify trigger function existence. This is not full PostgreSQL trigger governance. No MySQL/TiDB behavior changes.

---

## DDL: PostgreSQL Function/Procedure Lifecycle (v0.70.0)

`v0.70.0` adds PostgreSQL function and procedure lifecycle coverage for `CREATE FUNCTION`, `CREATE OR REPLACE FUNCTION`, `DROP FUNCTION`, `CREATE PROCEDURE`, and `DROP PROCEDURE`. These rules only apply when `--dialect postgresql` is set.

### Normalized Operations

| SQL | Normalized Operation |
|-----|---------------------|
| `CREATE FUNCTION f() RETURNS void LANGUAGE plpgsql AS $$ ... $$` | `create_function` |
| `CREATE FUNCTION f() ... SECURITY DEFINER` | `create_function` (security_definer=true) |
| `CREATE OR REPLACE FUNCTION f() ...` | `create_or_replace_function` |
| `DROP FUNCTION f()` | `drop_function` |
| `CREATE PROCEDURE p() LANGUAGE plpgsql AS $$ ... $$` | `create_procedure` |
| `DROP PROCEDURE p()` | `drop_procedure` |

### Function/Procedure Lifecycle Rules

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.pg.create_function.notice` | `CREATE FUNCTION` introduces a new function — informational notice | ✓ | ✗ | notice |
| `ddl.pg.create_function.security_definer.warn` | `CREATE FUNCTION ... SECURITY DEFINER` executes with owner privileges — warns about privilege escalation | ✓ | ✗ | warning |
| `ddl.pg.create_or_replace_function.advisory` | `CREATE OR REPLACE FUNCTION` replaces an existing function — advises review of downstream dependencies | ✓ | ✗ | notice |
| `ddl.pg.drop_function.advisory` | `DROP FUNCTION` removes a function — advises review of dependent objects | ✓ | ✗ | notice |
| `ddl.pg.create_procedure.notice` | `CREATE PROCEDURE` introduces a new procedure — informational notice | ✓ | ✗ | notice |
| `ddl.pg.drop_procedure.advisory` | `DROP PROCEDURE` removes a procedure — advises review of dependent objects | ✓ | ✗ | notice |

> **Note:** These rules are offline-only and do not require a database connection. `CREATE FUNCTION ... SECURITY DEFINER` intentionally emits both `ddl.pg.create_function.notice` and `ddl.pg.create_function.security_definer.warn`. DeltaScope does not evaluate function bodies, verify argument types, or inspect live function state. This is not full PostgreSQL function/procedure governance. No MySQL/TiDB behavior changes.

---

## DDL: PostgreSQL Advanced View Lifecycle (v0.70.0)

`v0.70.0` adds PostgreSQL advanced view lifecycle coverage beyond the base `CREATE VIEW` / `DROP VIEW` forms. DeltaScope normalizes `CREATE OR REPLACE VIEW`, `CREATE TEMP VIEW`, `CREATE VIEW ... WITH CHECK OPTION`, `DROP VIEW ... CASCADE`, `ALTER VIEW ... RENAME TO`, and `ALTER VIEW ... SET SCHEMA`, adds six PostgreSQL-only findings. These rules only apply when `--dialect postgresql` is set.

### Normalized Operations

| SQL | Normalized Operation |
|-----|---------------------|
| `CREATE OR REPLACE VIEW v1 AS SELECT 1` | `create_or_replace_view` |
| `CREATE TEMP VIEW tv1 AS SELECT 1` | `create_temp_view` |
| `CREATE TEMPORARY VIEW tv1 AS SELECT 1` | `create_temp_view` |
| `CREATE VIEW v1 AS SELECT 1 WITH CHECK OPTION` | `create_view` (check_option=cascaded) |
| `CREATE VIEW v1 AS SELECT 1 WITH CASCADED CHECK OPTION` | `create_view` (check_option=cascaded) |
| `CREATE VIEW v1 AS SELECT 1 WITH LOCAL CHECK OPTION` | `create_view` (check_option=local) |
| `DROP VIEW v1 CASCADE` | `drop_view` (cascade=true) |
| `ALTER VIEW v1 RENAME TO v2` | `alter_view` (action=rename) |
| `ALTER VIEW v1 SET SCHEMA schema2` | `alter_view` (action=set_schema) |

### Advanced View Lifecycle Rules

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.pg.create_or_replace_view.advisory` | `CREATE OR REPLACE VIEW` replaces an existing view — advises review of downstream dependencies | ✓ | ✗ | notice |
| `ddl.pg.create_temp_view.notice` | `CREATE TEMP VIEW` / `CREATE TEMPORARY VIEW` creates a session-scoped view — informational notice | ✓ | ✗ | notice |
| `ddl.pg.create_view.check_option.notice` | `CREATE VIEW ... WITH CHECK OPTION` enforces check option — informational notice | ✓ | ✗ | notice |
| `ddl.pg.drop_view.cascade.warn` | `DROP VIEW ... CASCADE` uses cascading deletion — may silently drop dependent objects | ✓ | ✗ | warning |
| `ddl.pg.alter_view.rename.notice` | `ALTER VIEW ... RENAME TO` changes the view name — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_view.set_schema.notice` | `ALTER VIEW ... SET SCHEMA` moves the view to a different schema — informational notice | ✓ | ✗ | notice |

> **Note:** These rules are offline-only and do not require a database connection. DeltaScope does not evaluate view query bodies or inspect live view state. This is not full PostgreSQL view governance. No MySQL/TiDB behavior changes.

---

## DDL: PostgreSQL Selected ALTER Object Lifecycle (v0.70.0)

`v0.70.0` adds PostgreSQL selected ALTER object lifecycle coverage for schema, index, and materialized view. DeltaScope normalizes `ALTER SCHEMA ... RENAME TO`, `ALTER SCHEMA ... OWNER TO`, `ALTER INDEX ... RENAME TO`, `ALTER INDEX ... SET TABLESPACE`, `ALTER MATERIALIZED VIEW ... RENAME TO`, and `ALTER MATERIALIZED VIEW ... SET SCHEMA`, adds six PostgreSQL-only findings. These rules only apply when `--dialect postgresql` is set.

### Normalized Operations

| SQL | Normalized Operation |
|-----|---------------------|
| `ALTER SCHEMA s1 RENAME TO s2` | `alter_schema` (action=rename) |
| `ALTER SCHEMA s1 OWNER TO new_owner` | `alter_schema` (action=owner) |
| `ALTER INDEX idx1 RENAME TO idx2` | `alter_index` (action=rename) |
| `ALTER INDEX idx1 SET TABLESPACE ts2` | `alter_index` (action=set_tablespace) |
| `ALTER MATERIALIZED VIEW mv1 RENAME TO mv2` | `alter_materialized_view` (action=rename) |
| `ALTER MATERIALIZED VIEW mv1 SET SCHEMA schema2` | `alter_materialized_view` (action=set_schema) |

### Selected ALTER Object Lifecycle Rules

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.pg.alter_schema.rename.notice` | `ALTER SCHEMA ... RENAME TO` changes the schema name — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_schema.owner.notice` | `ALTER SCHEMA ... OWNER TO` changes the schema owner — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_index.rename.notice` | `ALTER INDEX ... RENAME TO` changes the index name — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_index.set_tablespace.notice` | `ALTER INDEX ... SET TABLESPACE` moves the index to a different tablespace — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_materialized_view.rename.notice` | `ALTER MATERIALIZED VIEW ... RENAME TO` changes the materialized view name — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_materialized_view.set_schema.notice` | `ALTER MATERIALIZED VIEW ... SET SCHEMA` moves the materialized view to a different schema — informational notice | ✓ | ✗ | notice |

> **Note:** These rules are offline-only and do not require a database connection. DeltaScope does not verify live schema/index/materialized-view existence, ownership, or tablespace availability. This is not full PostgreSQL ALTER object lifecycle coverage — remaining ALTER forms (e.g., `ALTER INDEX ... SET (...)`, `ALTER MATERIALIZED VIEW ... OWNER TO`) are deferred. No MySQL/TiDB behavior changes.

---

## DDL: PostgreSQL Selected Non-Permission DDL Deep Coverage (v0.80.0)

`v0.80.0` adds 36 new PostgreSQL-only DDL lifecycle rules for selected PostgreSQL non-permission DDL deep coverage across six rule families: composite type attribute mutations, extension member mutations, publication/subscription lifecycle, foreign object lifecycle (foreign data wrappers, foreign servers, user mappings, foreign tables), annotation operations (`COMMENT ON`, `SECURITY LABEL`), and event trigger/rewrite rule lifecycle. These rules only apply when `--dialect postgresql` is set.

### Composite Type Attribute Lifecycle Rules

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.pg.alter_type.add_attribute.notice` | `ALTER TYPE ... ADD ATTRIBUTE` adds a new attribute to a composite type — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_type.drop_attribute.warn` | `ALTER TYPE ... DROP ATTRIBUTE` removes an attribute — warns about dependent columns and functions | ✓ | ✗ | warning |
| `ddl.pg.alter_type.alter_attribute_type.warn` | `ALTER TYPE ... ALTER ATTRIBUTE ... TYPE` changes an attribute type — warns about potential data conversion | ✓ | ✗ | warning |
| `ddl.pg.alter_type.rename_attribute.notice` | `ALTER TYPE ... RENAME ATTRIBUTE` renames an attribute — informational notice | ✓ | ✗ | notice |

### Extension Member Lifecycle Rules

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.pg.alter_extension.add_member.notice` | `ALTER EXTENSION ... ADD TABLE` adds an object to the extension — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_extension.drop_member.warn` | `ALTER EXTENSION ... DROP TABLE` removes an object from the extension — warns about extension-drop cascade | ✓ | ✗ | warning |

### Publication/Subscription Lifecycle Rules

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.pg.create_publication.notice` | `CREATE PUBLICATION` introduces a new publication — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_publication.notice` | `ALTER PUBLICATION` modifies an existing publication — informational notice | ✓ | ✗ | notice |
| `ddl.pg.drop_publication.warn` | `DROP PUBLICATION` removes a publication — warns that subscribers will stop receiving changes | ✓ | ✗ | warning |
| `ddl.pg.create_subscription.notice` | `CREATE SUBSCRIPTION` establishes a new subscription — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_subscription.notice` | `ALTER SUBSCRIPTION` modifies an existing subscription — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_subscription.disable.warn` | `ALTER SUBSCRIPTION ... DISABLE` disables the subscription — warns that replication will stop | ✓ | ✗ | warning |
| `ddl.pg.drop_subscription.warn` | `DROP SUBSCRIPTION` removes a subscription — warns about replication slot cleanup | ✓ | ✗ | warning |

### Foreign Object Lifecycle Rules

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.pg.create_foreign_data_wrapper.notice` | `CREATE FOREIGN DATA WRAPPER` introduces a new FDW — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_foreign_data_wrapper.notice` | `ALTER FOREIGN DATA WRAPPER` modifies an existing FDW — informational notice | ✓ | ✗ | notice |
| `ddl.pg.drop_foreign_data_wrapper.warn` | `DROP FOREIGN DATA WRAPPER` removes an FDW — warns about dependent foreign servers and tables | ✓ | ✗ | warning |
| `ddl.pg.create_foreign_server.notice` | `CREATE SERVER` registers a new foreign server — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_foreign_server.notice` | `ALTER SERVER` modifies an existing foreign server — informational notice | ✓ | ✗ | notice |
| `ddl.pg.drop_foreign_server.warn` | `DROP SERVER` removes a foreign server — warns about dependent user mappings and foreign tables | ✓ | ✗ | warning |
| `ddl.pg.create_user_mapping.notice` | `CREATE USER MAPPING` registers a user mapping — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_user_mapping.notice` | `ALTER USER MAPPING` modifies an existing user mapping — informational notice | ✓ | ✗ | notice |
| `ddl.pg.drop_user_mapping.warn` | `DROP USER MAPPING` removes a user mapping — warns about dependent foreign table connections | ✓ | ✗ | warning |
| `ddl.pg.create_foreign_table.notice` | `CREATE FOREIGN TABLE` introduces a new foreign table — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_foreign_table.notice` | `ALTER FOREIGN TABLE` modifies an existing foreign table — informational notice | ✓ | ✗ | notice |
| `ddl.pg.drop_foreign_table.warn` | `DROP FOREIGN TABLE` removes a foreign table — warns about dependent queries | ✓ | ✗ | warning |

### Annotation Lifecycle Rules

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.pg.comment_on.notice` | `COMMENT ON ... IS 'text'` attaches a comment to a database object — informational notice | ✓ | ✗ | notice |
| `ddl.pg.comment_on.remove.notice` | `COMMENT ON ... IS NULL` removes the comment — informational notice | ✓ | ✗ | notice |
| `ddl.pg.security_label.notice` | `SECURITY LABEL ... IS 'label'` attaches a security label — informational notice | ✓ | ✗ | notice |
| `ddl.pg.security_label.remove.notice` | `SECURITY LABEL ... IS NULL` removes the security label — informational notice | ✓ | ✗ | notice |

### Event Trigger / Rewrite Rule Lifecycle Rules

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.pg.create_event_trigger.notice` | `CREATE EVENT TRIGGER` introduces a new event trigger — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_event_trigger.notice` | `ALTER EVENT TRIGGER` modifies an existing event trigger — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_event_trigger.disable.warn` | `ALTER EVENT TRIGGER ... DISABLE` disables an event trigger — warns that DDL event handling will stop | ✓ | ✗ | warning |
| `ddl.pg.drop_event_trigger.warn` | `DROP EVENT TRIGGER` removes an event trigger — warns about DDL event handling implications | ✓ | ✗ | warning |
| `ddl.pg.create_rule.notice` | `CREATE RULE` introduces a new rewrite rule — informational notice | ✓ | ✗ | notice |
| `ddl.pg.alter_rule.notice` | `ALTER RULE` modifies an existing rewrite rule — informational notice | ✓ | ✗ | notice |
| `ddl.pg.drop_rule.warn` | `DROP RULE` removes a rewrite rule — warns about dependent query behavior | ✓ | ✗ | warning |

> **Note:** These are 36 new PostgreSQL-only DDL lifecycle rules. All are offline rules and do not require a database connection. Composite type attribute operations replace the previously listed unsupported/deferred entries in the Composite Type Lifecycle section. Extension member operations replace the previously listed unsupported/deferred entries in the Extension Lifecycle section. `DROP SUBSCRIPTION ... WITH (drop_slot = true)` remains deferred (parser_error). DeltaScope does not verify live object state, validate data conversion safety, inspect replication slot status, verify FDW handler/validator functions, or evaluate trigger/rule bodies. This is selected PostgreSQL non-permission DDL deep coverage — not full PostgreSQL DDL support or complete PostgreSQL grammar coverage. No MySQL/TiDB behavior changes.

---

## DDL: PostgreSQL Long-Tail Object Lifecycle (v0.100.0)

`v0.100.0` extends PostgreSQL DDL audit coverage to selected long-tail object lifecycle families: collation, extended statistics, aggregate/operator/conversion, operator family/class, text search objects, and boundary closure (DROP TRANSFORM, DROP ACCESS METHOD, ALTER LARGE OBJECT). DeltaScope normalizes these DDL forms and adds 36 PostgreSQL-only lifecycle findings/rules. These rules only apply when `--dialect postgresql` is set.

### Normalized Operations

| SQL | Normalized Operation |
|-----|---------------------|
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

### Long-Tail Lifecycle Rules

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.pg.create_collation.notice` | CREATE COLLATION notice | ✓ | ✗ | notice |
| `ddl.pg.alter_collation.notice` | ALTER COLLATION notice | ✓ | ✗ | notice |
| `ddl.pg.drop_collation.warn` | DROP COLLATION warning | ✓ | ✗ | warning |
| `ddl.pg.create_statistics.notice` | CREATE STATISTICS notice | ✓ | ✗ | notice |
| `ddl.pg.alter_statistics.notice` | ALTER STATISTICS notice | ✓ | ✗ | notice |
| `ddl.pg.drop_statistics.warn` | DROP STATISTICS warning | ✓ | ✗ | warning |
| `ddl.pg.create_aggregate.notice` | CREATE AGGREGATE notice | ✓ | ✗ | notice |
| `ddl.pg.alter_aggregate.notice` | ALTER AGGREGATE notice | ✓ | ✗ | notice |
| `ddl.pg.drop_aggregate.warn` | DROP AGGREGATE warning | ✓ | ✗ | warning |
| `ddl.pg.create_operator.notice` | CREATE OPERATOR notice | ✓ | ✗ | notice |
| `ddl.pg.alter_operator.notice` | ALTER OPERATOR notice | ✓ | ✗ | notice |
| `ddl.pg.drop_operator.warn` | DROP OPERATOR warning | ✓ | ✗ | warning |
| `ddl.pg.create_conversion.notice` | CREATE CONVERSION notice | ✓ | ✗ | notice |
| `ddl.pg.alter_conversion.notice` | ALTER CONVERSION notice | ✓ | ✗ | notice |
| `ddl.pg.drop_conversion.warn` | DROP CONVERSION warning | ✓ | ✗ | warning |
| `ddl.pg.create_operator_family.notice` | CREATE OPERATOR FAMILY notice | ✓ | ✗ | notice |
| `ddl.pg.alter_operator_family.notice` | ALTER OPERATOR FAMILY notice | ✓ | ✗ | notice |
| `ddl.pg.drop_operator_family.warn` | DROP OPERATOR FAMILY warning | ✓ | ✗ | warning |
| `ddl.pg.create_operator_class.notice` | CREATE OPERATOR CLASS notice | ✓ | ✗ | notice |
| `ddl.pg.alter_operator_class.notice` | ALTER OPERATOR CLASS notice | ✓ | ✗ | notice |
| `ddl.pg.drop_operator_class.warn` | DROP OPERATOR CLASS warning | ✓ | ✗ | warning |
| `ddl.pg.create_text_search_configuration.notice` | CREATE TEXT SEARCH CONFIGURATION notice | ✓ | ✗ | notice |
| `ddl.pg.alter_text_search_configuration.notice` | ALTER TEXT SEARCH CONFIGURATION notice | ✓ | ✗ | notice |
| `ddl.pg.drop_text_search_configuration.warn` | DROP TEXT SEARCH CONFIGURATION warning | ✓ | ✗ | warning |
| `ddl.pg.create_text_search_dictionary.notice` | CREATE TEXT SEARCH DICTIONARY notice | ✓ | ✗ | notice |
| `ddl.pg.alter_text_search_dictionary.notice` | ALTER TEXT SEARCH DICTIONARY notice | ✓ | ✗ | notice |
| `ddl.pg.drop_text_search_dictionary.warn` | DROP TEXT SEARCH DICTIONARY warning | ✓ | ✗ | warning |
| `ddl.pg.create_text_search_parser.notice` | CREATE TEXT SEARCH PARSER notice | ✓ | ✗ | notice |
| `ddl.pg.alter_text_search_parser.notice` | ALTER TEXT SEARCH PARSER notice | ✓ | ✗ | notice |
| `ddl.pg.drop_text_search_parser.warn` | DROP TEXT SEARCH PARSER warning | ✓ | ✗ | warning |
| `ddl.pg.create_text_search_template.notice` | CREATE TEXT SEARCH TEMPLATE notice | ✓ | ✗ | notice |
| `ddl.pg.alter_text_search_template.notice` | ALTER TEXT SEARCH TEMPLATE notice | ✓ | ✗ | notice |
| `ddl.pg.drop_text_search_template.warn` | DROP TEXT SEARCH TEMPLATE warning | ✓ | ✗ | warning |
| `ddl.pg.create_transform.notice` | CREATE TRANSFORM notice | ✓ | ✗ | notice |
| `ddl.pg.create_access_method.notice` | CREATE ACCESS METHOD notice | ✓ | ✗ | notice |
| `ddl.pg.drop_transform.warn` | DROP TRANSFORM warning | ✓ | ✗ | warning |
| `ddl.pg.drop_access_method.warn` | DROP ACCESS METHOD warning | ✓ | ✗ | warning |
| `ddl.pg.alter_large_object.owner.notice` | ALTER LARGE OBJECT owner notice | ✓ | ✗ | notice |

---

## DDL: PostgreSQL Coverage Expansion (v0.21.0 / v0.23.0 / v0.24.0)

`v0.21.0` normalizes common PostgreSQL migration follow-up DDL through the shared audit pipeline. `v0.23.0` extends PostgreSQL `CREATE TABLE` coverage for more common constraint forms. `v0.24.0` deepens the semantic value of those create-table shapes by preserving parser-owned `ReferencedTable` and `ReferencedColumns` through the shared `spec.Constraint` model. These surfaces previously returned capability-boundary errors or incomplete structure; they now produce normal audit results with progressively richer semantics. No new rules are introduced — existing shared rule families apply where relevant.

| PostgreSQL DDL Action | Normalized As | Supported | Auditable | Rule-Mapped | Metadata-Dependent | Notes |
|-----------------------|---------------|:---------:|:---------:|:-----------:|:------------------:|-------|
| `ALTER COLUMN ... SET DEFAULT` | `set_default` | ✓ | ✓ | ✓ (shared alter) | — | Standard alter action |
| `ALTER COLUMN ... DROP DEFAULT` | `drop_default` | ✓ | ✓ | ✓ (shared alter) | — | Standard alter action |
| `ALTER COLUMN ... SET NOT NULL` | `set_not_null` | ✓ | ✓ | ✓ (shared alter) | — | Standard alter action |
| `ALTER COLUMN ... DROP NOT NULL` | `drop_not_null` | ✓ | ✓ | ✓ (shared alter) | — | Standard alter action |
| `VALIDATE CONSTRAINT` | `validate_constraint` | ✓ | ✓ | — | — | No dedicated rule; produces clean audit unless other findings apply |
| `DROP CONSTRAINT` (general) | `drop_constraint` | ✓ | ✓ | — | — | Standard alter action in offline mode |
| `DROP CONSTRAINT` (primary key) | `drop_constraint` | ✓ | ✓ | ✓ (`ddl.alter.drop_primary_key`) | ✓ | Primary-key mapping via metadata-aware rules |
| Table-level named `CHECK` | `create_table` shared facts | ✓ | ✓ | ✓ (shared constraint naming when configured) | — | Coverage expansion; no new rule family |
| Column-level inline `CHECK` | `create_table` shared facts | ✓ | ✓ | — | — | Supported structure; no dedicated rule family |
| Table-level named `UNIQUE` | `create_table` shared facts | ✓ | ✓ | ✓ (shared constraint naming when configured) | — | Named-constraint facts can reuse existing naming governance |
| Column-level inline `UNIQUE` | `create_table` shared facts | ✓ | ✓ | ✓ (shared index facts) | — | Existing shared index rules can consume normalized index facts |
| Table-level named `FOREIGN KEY` | `create_table` shared facts | ✓ | ✓ | ✓ (shared constraint naming when configured) | — | Naming rules matter only when policy allows foreign keys. `v0.24.0`: preserves `ReferencedTable`/`ReferencedColumns` |
| Column-level inline `REFERENCES` | `create_table` shared facts | ✓ | ✓ | — | — | Parser-owned shared facts only; no invented metadata semantics. `v0.24.0`: preserves `ReferencedTable`/`ReferencedColumns` |

### Surface Parity

All newly normalized PostgreSQL DDL actions and `v0.23.0`/`v0.24.0` create-table coverage shapes are confirmed across CLI, HTTP (`POST /v1/audit`), MCP (`audit_sql`), and the public Go API (`pkg/deltascope`).

## Confidence Entry Points (`v0.22.0`)

`v0.22.0` is the **E2E & Release Confidence Pack**. It does not add new SQL rule semantics; it documents and validates the existing CLI, HTTP, MCP, and release surfaces with canonical repository targets.

- `make pg-unit-test-gates` — PostgreSQL-tagged unit confidence without Docker
- `make pg-e2e-gates` — Docker-backed PostgreSQL CLI, HTTP, and MCP transport confidence
- `make pg-confidence-gates` — canonical combined PostgreSQL confidence closure
- `make release-surface-gates VERSION=vX.Y.Z` — package/release contract verification
- `make release-version-surface-gates VERSION=vX.Y.Z` — versioned docs/install/release-notes verification

## Release Contract Gates (`v0.44.0`)

`v0.44.0` adds the **Release Contract Hardening Pack** — a unified `make release-contract-gates VERSION=vX.Y.Z` that verifies version surfaces, binary version output, default policy dialect isolation, and archive integrity before every tagged release. No new rule IDs, parser features, or public API contracts were added.

## Corpus-Backed Confidence (`v0.25.0`)

`v0.25.0` is the **SQL Corpus & Boundary Confidence Pack**. It adds a dialect-wide SQL corpus harness (`testdata/sql-corpus/`) that runs representative MySQL, TiDB, and PostgreSQL cases through the existing audit application layer and asserts expected outcomes at two layers:

1. **Report-level** — unsupported count, statement kind, findings include/exclude.
2. **Semantic** — operation name and constraint facts (type, name, columns, referenced table/columns).

The corpus does not add new rules, change audit behavior, or affect end-user workflows. It is a release-confidence asset: it answers which SQL patterns have been verified and what the expected results are.

## PostgreSQL Primary Key Facts (`v0.37.0`)

`v0.37.0` is the **PostgreSQL Primary Key Fact Support Pack**. PostgreSQL `CREATE TABLE` inline, table-level, named, and composite primary-key declarations now populate DeltaScope's normalized primary-key contract, allowing existing primary-key rules to audit PostgreSQL `CREATE TABLE` statements.

| Aspect | Detail |
|--------|--------|
| Supported forms | Inline (`id bigint PRIMARY KEY`), table-level (`PRIMARY KEY (id)`), named (`CONSTRAINT t_pkey PRIMARY KEY (id)`), composite (`PRIMARY KEY (a, b)`) |
| Not-null inference | PK columns are treated as effectively NOT NULL |
| Rules unlocked | `ddl.table.primary_key.bigint.require`, `ddl.table.primary_key.columns.max_count` |
| `ddl.table.primary_key.not_null.require` | No stable negative case for PostgreSQL — PK columns are effectively NOT NULL |
| Parser/spec changes | PostgreSQL extractor populates shared `DDL.PrimaryKey` contract |
| New rule IDs | none |
| New CLI/API flags | none |

### What This Does Not Add

- Full PostgreSQL index support.
- `ALTER TABLE ADD PRIMARY KEY` support.
- Live schema primary-key introspection.
- Full PostgreSQL constraint/index parity.

## PostgreSQL Generated/Identity Rule Coverage (`v0.36.0`)

`v0.36.0` is the **PostgreSQL Generated/Identity Rule Coverage Pack**. Three new PostgreSQL-only forbid rules cover the generated/identity state-transition forms that became supported in v0.35.0. This is rule coverage — not parser support widening, not spec contract widening, not generated expression evaluation, and not complete PostgreSQL sequence semantics.

| Aspect | Detail |
|--------|--------|
| New rule IDs | `ddl.alter.drop_expression.forbid`, `ddl.alter.set_generated.forbid`, `ddl.alter.drop_identity.forbid` |
| Rule type | PostgreSQL-only forbid alter-action rules |
| Covered forms | `DROP EXPRESSION`, `SET GENERATED ALWAYS`, `SET GENERATED BY DEFAULT`, `DROP IDENTITY` |
| Parser/spec changes | none — state-transition forms were already supported in v0.35.0 |
| New spec fields | none |
| New CLI/API flags | none |

### Rule Coverage Matrix

| Rule ID | Action | Covered Form |
|---------|--------|-------------|
| `ddl.alter.drop_expression.forbid` | `drop_expression` | `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION` |
| `ddl.alter.set_generated.forbid` | `set_generated` | `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ALWAYS/BY DEFAULT` |
| `ddl.alter.drop_identity.forbid` | `drop_identity` | `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY` |

### Surface Contract

| Surface | Behavior |
|---------|----------|
| CLI | Normal audit output with `rule_id` findings for covered forms |
| pkg/deltascope | `Audit()` returns result with findings containing explicit `rule_id` |
| HTTP | Normal audit response with findings containing explicit `rule_id` |
| MCP | Normal tool result with findings containing explicit `rule_id` |

### What Did Not Change

- No parser support widening.
- No spec contract widening.
- No generated expression evaluation.
- No complete PostgreSQL sequence semantics.
- No MySQL/TiDB behavior changes.

## PostgreSQL Generated/Identity State-Transition Support (`v0.35.0`)

`v0.35.0` is the **PostgreSQL Generated/Identity State-Transition Pack**. State-transition forms for PostgreSQL generated and identity columns are now supported through the normal audit path. This is state-transition support — not full generated-column lifecycle support, not generated expression evaluation, not complete PostgreSQL sequence semantics, and no new rule IDs.

| Aspect | Detail |
|--------|--------|
| Supported forms | `DROP EXPRESSION`, `SET GENERATED ALWAYS`, `SET GENERATED BY DEFAULT`, `DROP IDENTITY` |
| Normalized contract | `drop_expression`, `set_generated` with `generated_when` (`"a"` / `"d"`), `drop_identity` |
| GeneratedExpression | Not in contract — no expression text preserved |
| New rule IDs | none |
| New CLI/API flags | none |

### Supported State-Transition Forms

| Form | Status | Audit Behavior |
|------|--------|---------------|
| `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION` | Supported | Normal audit result with findings where applicable |
| `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ALWAYS` | Supported | Normal audit result with findings where applicable |
| `ALTER TABLE ... ALTER COLUMN ... SET GENERATED BY DEFAULT` | Supported | Normal audit result with findings where applicable |
| `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY` | Supported | Normal audit result with findings where applicable |

### Surface Contract for Supported Forms

| Surface | Behavior |
|---------|----------|
| CLI | Normal audit output (exit code 0 or 1 depending on findings), no `unsupported` array |
| pkg/deltascope | `Audit()` returns result with statements, no `ErrUnsupportedStatement` |
| HTTP | Normal audit response, no unsupported error |
| MCP | Normal tool result with statements, no `IsError` |

### Previous: Narrow Definition Support (`v0.34.0`)

`v0.34.0` added narrow generated/identity definition form support (`CREATE TABLE` and `ALTER TABLE ADD COLUMN`). Those forms continue flowing through the normal audit path. Preserved facts: `generated_when`, `is_identity`, `identity_options` (from v0.33.0).

## PostgreSQL Generated/Identity Fact Preservation + Unsupported Metadata Surfacing (`v0.33.0`)

`v0.33.0` is the **PostgreSQL Generated/Identity Fact Preservation + Unsupported Metadata Surfacing Pack**. It preserves narrow generated/identity column facts in the shared DDL contract and surfaces structured metadata on unsupported generated/identity outcomes. This is fact preservation and metadata widening — not generated-column support, identity-column support, or rule behavior changes.

| Aspect | Detail |
|--------|--------|
| New shared contract fields | `GeneratedWhen` (string: `"a"` / `"d"`), `IsIdentity` (bool), `IdentityOptions` (finite map) on `spec.Column` |
| Unsupported metadata widening | `UnsupportedDetail.Metadata` carries `column`, `generated_when`, `is_identity`, `identity_options` |
| Applicable paths | `CREATE TABLE` and `ALTER TABLE ADD COLUMN` generated/identity columns |
| GeneratedExpression | Deferred — no stable expression renderer |
| IdentityOptions scope | Finite structured facts only; not complete PostgreSQL sequence semantics |
| New rule IDs | none |
| New CLI/API flags | none |
| Unsupported feature names | Unchanged: `generated_column`, `generated_as_identity` |

### Surface Contract for Unsupported Metadata

| Surface | Metadata Visibility |
|---------|-------------------|
| CLI (JSON) | `unsupported[0].metadata` visible in JSON output |
| pkg/deltascope | `result.Unsupported[i].Metadata` accessible as `map[string]any` |
| HTTP | Metadata available via captured result; transport returns unsupported error |
| MCP | **Not directly surfaced** — MCP returns `IsError=true` with error code/message only |

## PostgreSQL Boundary Support-Readiness Gate (`v0.32.0`)

`v0.32.0` is the **PostgreSQL Boundary Support-Readiness Gate**. It is a decision milestone — not a feature release. Characterization tests document stable AST facts about generated and identity columns; a readiness report recommends `v0.33.0` as a narrow fact-preservation pack.

| Aspect | Detail |
|--------|--------|
| Characterization tests | 7 tests in `parser_test.go` documenting `GeneratedWhen` encoding, constraint types, sequence option shape |
| Readiness report | Complete boundary inventory, AST fact coverage, v0.33.0 recommendation |
| New rule IDs | none |
| New CLI/API flags | none |
| Production code changes | none |

No new audit capabilities, rules, or surface contracts were added.

## PostgreSQL ALTER TABLE GENERATED Follow-up Pack (`v0.31.0`)

`v0.31.0` is the **PostgreSQL ALTER TABLE GENERATED Follow-up Pack**. It maps additional PostgreSQL generated/identity `ALTER TABLE` forms to explicit unsupported feature tags, closing the adjacent gap left by `v0.30.0`. These are explicit unsupported contracts, not new rule findings.

| Aspect | Detail |
|--------|--------|
| Drop expression | `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION` → `generated_column` |
| Set generated | `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ...` → `generated_as_identity` |
| Drop identity | `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY` → `generated_as_identity` |
| Backed by | Corpus cases, service checks, and surface parity across CLI, HTTP, MCP, and `pkg/deltascope` |
| New rule IDs | none |
| New CLI/API flags | none |

This is boundary tightening, not generated-column support, identity-column support, or complete PostgreSQL `ALTER TABLE` support.

## PostgreSQL ALTER TABLE GENERATED Boundary Pack (`v0.30.0`)

`v0.30.0` is the **PostgreSQL ALTER TABLE GENERATED Boundary Pack**. It tightens the unsupported boundary contract for PostgreSQL `ALTER TABLE ... ADD COLUMN` forms that carry generated stored or identity semantics. These are explicit unsupported contracts, not new rule findings.

| Aspect | Detail |
|--------|--------|
| Generated stored add-column | `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS (...) STORED` → `generated_column` |
| Identity add-column | `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS IDENTITY` → `generated_as_identity` |
| Backed by | Corpus cases, service checks, and surface parity across CLI, HTTP, MCP, and `pkg/deltascope` |
| Adjacent forms | `DROP EXPRESSION`, `SET GENERATED`, and `DROP IDENTITY` now receive explicit unsupported mappings in `v0.31.0` |
| New rule IDs | none |
| New CLI/API flags | none |

This is boundary tightening, not generated-column support, identity-column support, or broad PostgreSQL `ALTER TABLE` support.

## Schema-Aware FK Policy Pack (`v0.29.0`)

`v0.29.0` is the **Schema-Aware FK Policy Pack**. It is the first schema-aware FK policy step: DeltaScope emits the PostgreSQL-only notice rule `ddl.pg.table.foreign_key.cross_schema.advisory` when the owning table schema and referenced schema are both explicit and different.

| Aspect | Detail |
|--------|--------|
| New rule ID | `ddl.pg.table.foreign_key.cross_schema.advisory` |
| Default level | `notice` |
| Trigger | PostgreSQL only; owning table schema explicit; referenced schema explicit; schemas differ |
| Same-schema FK | no advisory |
| Bare `REFERENCES users(id)` | no advisory; referenced schema remains unknown |
| Metadata | findings may include `table_schema`, `referenced_schema`, `referenced_table`, `referenced_columns` |
| Normalized representation | `referenced_table` remains `"users"`, never `"auth.users"` |

This is not a broad PostgreSQL FK engine, not a cross-schema validation workflow, and not `search_path`-aware behavior.

## Referenced-Object Metadata Surface (`v0.28.0`)

`v0.28.0` is the **Referenced-Object Metadata Surface Pack**. It widens the outward FK forbid finding metadata to expose PostgreSQL referenced-object facts (`referenced_schema`, `referenced_table`, `referenced_columns`) that were already present in the shared semantic contract from `v0.27.0`. This is an additive metadata widening, not a new rule family.

| Aspect | Detail |
|--------|--------|
| Widened metadata | `ddl.table.foreign_key.forbid` finding metadata now includes `referenced_schema`, `referenced_table`, `referenced_columns` when the underlying constraint carries those facts |
| Conditional emission | `referenced_schema` is omitted when no schema qualifier is present; `referenced_table` and `referenced_columns` appear for all FK constraints that carry them |
| Normalized representation | `referenced_table` is never concatenated with `referenced_schema` (e.g., never `"public.users"`) |
| Backed by | Surface tests across CLI, HTTP, MCP, and `pkg/deltascope` |
| No new rule IDs | The `ddl.table.foreign_key.forbid` rule is unchanged; only its finding metadata is wider |

This is not schema-aware FK policy support, not a broad PostgreSQL FK implementation, and not a new rule family.

## Schema-Qualified Reference Semantics (`v0.27.0`)

`v0.27.0` is the **Schema-Qualified Reference Semantics Pack**. It preserves PostgreSQL schema-qualified referenced-object facts in the shared `spec.Constraint` contract. This is semantic contract preservation, not a new rule family.

| Aspect | Detail |
|--------|--------|
| Additive field | `ReferencedSchema` on `spec.Constraint` |
| Normalized representation | `ReferencedSchema = "public"`, `ReferencedTable = "users"` (never concatenated) |
| Backed by | Corpus cases + service-level semantic tests |
| Public finding metadata | `v0.28.0` widens the outward finding metadata to expose `referenced_schema`, `referenced_table`, `referenced_columns` on FK forbid findings |

Current public transports now expose referenced-object fields in FK forbid finding metadata (`v0.28.0`). This is not a broad PostgreSQL FK implementation and not schema-aware rule support.

## PostgreSQL Unsupported Boundaries (`v0.26.0`)

`v0.26.0` is the **PostgreSQL CREATE TABLE Unsupported Boundary Pack**. It tightens the extractor-level boundary contract for PostgreSQL `CREATE TABLE` forms that are explicitly outside the supported surface. Each boundary is backed by corpus cases and surface parity tests. No new rule IDs are involved — these are extractor-level unsupported contracts, not rule findings.

| Feature | Extractor Tag | Surface Contract |
|---------|---------------|-----------------|
| Identity columns (`GENERATED ... AS IDENTITY`) | `generated_as_identity` | Unsupported |
| Generated stored columns (`GENERATED ALWAYS AS ... STORED`) | `generated_column` | Unsupported |
| Exclusion constraints (`EXCLUDE USING`) | `exclusion_constraint` | Unsupported |
| Partitioned tables (`PARTITION BY`) | `partitioning` | Unsupported |

Surface contract for unsupported statements:

- **CLI** and **`pkg/deltascope`**: return a partial result with an `unsupported` array plus `ErrUnsupportedStatement`.
- **HTTP** and **MCP**: expose as transport-level error (HTTP error response, MCP tool error).
- **`v0.30.0` note**: PostgreSQL `ALTER TABLE ... ADD COLUMN` generated/identity forms now follow the same explicit unsupported contract shape through `generated_column` and `generated_as_identity`. Adjacent `DROP EXPRESSION`, `SET GENERATED`, and `DROP IDENTITY` now receive explicit unsupported mappings in `v0.31.0`.
- **`v0.33.0` note**: Unsupported generated/identity outcomes now carry structured metadata (`column`, `generated_when`, `is_identity`, `identity_options`) via `UnsupportedDetail.Metadata`. CLI and `pkg/deltascope` expose this directly; MCP does not.

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

### Impact Estimation

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `dml.impact.estimate` | Surface the precomputed conservative statement-level `impact` estimate for `UPDATE` and `DELETE` in rule output | ✓ | ✓ | notice |
| `dml.impact.rows.max_count` | Conservative estimated affected-row count exceeds the configured threshold | ✓ | ✓ | warning |
| `dml.impact.ratio.max_percent` | Conservative estimated affected-row ratio exceeds the configured threshold | ✓ | ✓ | warning |

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
| Index cardinality and `table_rows` | Metadata-aware refinement of conservative DML impact estimation |

### Object Metadata (v0.90.0)

PostgreSQL metadata-aware audit now resolves metadata for selected non-table objects and enriches lifecycle rule findings with safe attributes:

| Object Type | Example SQL | Projected Attributes |
|-------------|-------------|---------------------|
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

Findings include `metadata_status` (`confirmed`/`not_found`/`unavailable`), `metadata_exists`, `metadata_object_type`, `metadata_object_name`, `metadata_schema`. Only 8 safe attribute keys are projected; sensitive values (password, secret, connection, body, definition, comment, label, query, action_sql, options) are filtered by a dual blacklist/whitelist. MySQL/TiDB object metadata resolution returns `unavailable` — no behavior change.

### Dialect-Specific Considerations

**MySQL / TiDB only:**

| Feature | Rules Affected |
|---------|---------------|
| `innodb_large_prefix` | `ddl.index.key_length.max_bytes.require` — index key length limit depends on this setting |
| `innodb_default_row_format` | `ddl.table.row_size.max_bytes.require` — row size estimation uses the instance row format |
| `innodb_adaptive_hash_index` | `ddl.table.drop.adaptive_hash`, `ddl.table.truncate.adaptive_hash` — latency-spike cautions |
| Engine / row-format checks | `ddl.table.engine.allowlist`, `ddl.table.row_format.allowlist` — MySQL storage engine governance |

These rules fire only for MySQL and TiDB targets. For PostgreSQL, they are not applicable and will not appear in findings.

**PostgreSQL only:**

| Feature | Rules Affected |
|---------|---------------|
| `EXPLAIN`-based planner estimation | `dml.impact.estimate` and downstream impact rules may use the PostgreSQL planner to refine `UPDATE`/`DELETE` row estimates. This is a read-only `EXPLAIN` — DeltaScope does not execute `EXPLAIN ANALYZE`. |
| `DROP CONSTRAINT` → primary key mapping | `ALTER TABLE … DROP CONSTRAINT` that targets the primary key is recognized and triggers `ddl.alter.drop_primary_key.forbid` and `ddl.alter.primary_key.drop.exists`. |
| Migration-safety rules | `ddl.pg.create_index.concurrently.require`, `ddl.pg.alter.add_column.non_null_default.rewrite.warn`, `ddl.pg.alter.add_check.not_valid.require`, `ddl.pg.alter.set_data_type.rewrite.warn`, `ddl.pg.alter.not_valid_constraint.validate.require`, `ddl.pg.drop_index.advisory`, `ddl.pg.alter.add_column.non_null_no_default.warn`, `ddl.pg.alter.add_unique_constraint.concurrent_index.advisory`, `ddl.pg.alter.drop_constraint.advisory` — offline PostgreSQL-specific rules that flag lock contention, table-rewrite risks, index/constraint removal, and missing same-batch validation follow-up for named `NOT VALID` constraints. |
| Object lifecycle rules (v0.50.0) | `ddl.pg.drop_schema.advisory`, `ddl.pg.drop_schema.cascade.warn`, `ddl.pg.create_sequence.cycle.warn`, `ddl.pg.alter_sequence.restart.warn`, `ddl.pg.alter_sequence.cycle.warn`, `ddl.pg.drop_sequence.advisory`, `ddl.pg.drop_sequence.cascade.warn`, `ddl.pg.drop_materialized_view.advisory`, `ddl.pg.drop_materialized_view.cascade.warn` — offline PostgreSQL-specific rules that guard against cascade drops, sequence wraparound, and counter resets for schema, sequence, and materialized view lifecycle DDL. |
| ALTER TABLE coverage rules (v0.51.0 / v0.52.0 / v0.54.0 / v0.56.0) | `ddl.pg.alter.drop_column.advisory`, `ddl.pg.alter.validate_constraint.advisory`, `ddl.pg.alter.add_column.nullable.notice`, `ddl.pg.alter.set_schema.advisory`, `ddl.pg.alter.owner.advisory`, `ddl.pg.alter.enable_trigger.notice`, `ddl.pg.alter.disable_trigger.warn`, `ddl.pg.alter.attach_partition.advisory`, `ddl.pg.alter.detach_partition.warn`, `ddl.pg.alter.replica_identity_full.warn`, `ddl.pg.alter.replica_identity_nothing.warn`, `ddl.pg.alter.replica_identity_using_index.notice`, `ddl.pg.alter.set_logged.notice`, `ddl.pg.alter.set_unlogged.notice` — offline PostgreSQL-specific rules covering ALTER TABLE safety patterns. Trigger-scope forms (`ALL/USER`) normalized since v0.54.0. `REPLICA IDENTITY DEFAULT` normalized and intentionally silent. Offline only — DeltaScope does not verify `REPLICA IDENTITY USING INDEX` index validity. DeltaScope does not verify whether the target table is currently logged or unlogged. |

---

## Trust & Misconfiguration Guardrails

These are additive behaviors (not rules) that help identify dialect mismatches and unsupported surfaces. They do not change rule evaluation or trigger conditions.

| Capability | Status | Notes |
|------------|--------|-------|
| PostgreSQL syntax heuristic notice | covered | When auditing on MySQL/TiDB path, detects common PG-specific syntax tokens (`RETURNING`, `ON CONFLICT`, `::`, `ALTER COLUMN TYPE USING`, `GENERATED AS IDENTITY`) and emits `dialect.postgresql.syntax.detected.notice` as a global advisory finding. DeltaScope does not auto-switch dialect. |
| PostgreSQL capability-boundary errors | covered | Unsupported PG surfaces return typed `PostgreSQLCapabilityBoundaryError` instead of heuristic string matching, enabling CI and tooling to distinguish known limits from real failures. |
| Offline trust context visibility | covered | CLI output formats (json, markdown, quiet) report audit context: markdown includes `## Audit Context` section with trust note; JSON includes `context` object; quiet includes `[context]` line. The `github-actions` and `sarif` formats emit findings only and do not include context metadata. |
| Rule summary / skipped rules visibility | covered | CLI output formats (json, markdown, quiet) report loaded, applicable, and skipped rule counts, making it easy to confirm which rules ran for the current dialect. The `github-actions` and `sarif` formats emit findings only and do not include rule summary metadata. |
| Heuristic false-positive exclusion | covered | PostgreSQL syntax heuristic does not fire for tokens inside string literals, double-quoted identifiers, backtick identifiers, line comments, or block comments. |
| GitLab Code Quality output | covered | `--format gitlab-codequality` emits a GitLab Code Quality report (`gl-code-quality-report.json`) for merge request widgets and diff annotations. All tiers (Free+). See [use-deltascope-in-gitlab-ci.md](../recipe/use-deltascope-in-gitlab-ci.md). |
| Source location fidelity | covered | CI renderers (GitHub Actions, SARIF, GitLab Code Quality) carry the original file path and statement-start line number for each finding via a progressive source mapper. Works for multi-statement migration files across MySQL, TiDB, and PostgreSQL dialects. |
