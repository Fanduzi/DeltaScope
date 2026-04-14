# Audit Capability Matrix

This matrix lists every rule shipped with DeltaScope, its rule ID, whether it runs in offline mode, whether it requires live metadata, and its default finding level. Use this reference to understand what DeltaScope will and will not flag for a given SQL statement and audit configuration.

**Offline** rules fire on SQL text alone — no database connection required. **Metadata-aware** rules additionally consume live schema or instance facts when a metadata provider is configured; without metadata they are silently skipped.

**Pattern legality checks** such as `*.name.pattern.require` and `*.name.keyword.forbid` enforce lexical validity. **Structured naming governance** such as `prefix`, `suffix`, and `contains` enforces team naming conventions. These are complementary layers, not replacements for one another.

## Supported Dialects for Metadata-Aware Audit

| Dialect | Metadata Sources | Notes |
|---------|-----------------|-------|
| MySQL | `information_schema`, `performance_schema.global_variables`, `InnoDB` stats | Full support. Engine, row-format, adaptive-hash, and InnoDB-specific rules apply. |
| TiDB | `information_schema`, `performance_schema` (optional) | Same sources as MySQL. `performance_schema` is optional — DeltaScope falls back gracefully. |
| PostgreSQL | `pg_catalog`, `pg_constraint`, `pg_indexes`, `EXPLAIN` (read-only) | Supported for metadata-aware audit. MySQL-specific features (InnoDB, adaptive hash, row format) are not applicable. PG-specific: `ALTER TABLE … DROP CONSTRAINT` maps to primary-key detection; DML impact estimation uses the PostgreSQL planner via `EXPLAIN` for `UPDATE`/`DELETE`. |

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

## DDL: PostgreSQL Migration-Safety

These rules guard against common PostgreSQL migration patterns that can cause table rewrites, long-held locks, or production incidents. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Check Description | Offline | Metadata | Default Level |
|---------|-------------------|:-------:|:--------:|---------------|
| `ddl.pg.create_index.concurrently.require` | `CREATE INDEX` without `CONCURRENTLY` holds an exclusive lock, blocking reads and writes | ✓ | ✗ | warning |
| `ddl.pg.alter.add_column.non_null_default.rewrite.warn` | Adding a `NOT NULL` column with a volatile default may trigger a full table rewrite | ✓ | ✗ | warning |
| `ddl.pg.alter.add_check.not_valid.require` | `ADD CHECK` constraint without `NOT VALID` requires a full table scan with `ACCESS EXCLUSIVE` lock | ✓ | ✗ | warning |
| `ddl.pg.alter.set_data_type.rewrite.warn` | Changing a column type may require a full table rewrite depending on the conversion | ✓ | ✗ | warning |

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

## Corpus-Backed Confidence (`v0.25.0`)

`v0.25.0` is the **SQL Corpus & Boundary Confidence Pack**. It adds a dialect-wide SQL corpus harness (`testdata/sql-corpus/`) that runs representative MySQL, TiDB, and PostgreSQL cases through the existing audit application layer and asserts expected outcomes at two layers:

1. **Report-level** — unsupported count, statement kind, findings include/exclude.
2. **Semantic** — operation name and constraint facts (type, name, columns, referenced table/columns).

The corpus does not add new rules, change audit behavior, or affect end-user workflows. It is a release-confidence asset: it answers which SQL patterns have been verified and what the expected results are.

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
| Migration-safety rules | `ddl.pg.create_index.concurrently.require`, `ddl.pg.alter.add_column.non_null_default.rewrite.warn`, `ddl.pg.alter.add_check.not_valid.require`, `ddl.pg.alter.set_data_type.rewrite.warn` — offline PostgreSQL-specific rules that flag lock contention and table-rewrite risks. |

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
