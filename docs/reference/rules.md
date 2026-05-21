# Rules Reference

DeltaScope ships its audit logic as discoverable, stable rule IDs rather than hidden heuristics. Every rule
can be inspected, filtered, enabled, disabled, and parameterized through the policy configuration.

## Rule Naming Convention

Rule IDs follow a structured dot-separated pattern:

```
<kind>.<area>.<check>
```

- **`ddl`** — DDL statements (CREATE TABLE, ALTER TABLE, DROP TABLE, TRUNCATE, CREATE VIEW, …)
- **`dml`** — DML statements (SELECT, INSERT, UPDATE, DELETE, REPLACE, …)

### Severity Levels

| Level | Meaning |
|-------|---------|
| `blocker` | Must fix before applying SQL. Indicates a high-risk or policy-violating change. |
| `warning` | Should review before applying. Indicates a potentially risky or non-standard pattern. |
| `notice` | Informational only. No immediate action required. |

### Verdict Mapping

The overall verdict for an audit batch is determined by the worst finding across all statements:

| Findings in batch | Verdict |
|-------------------|---------|
| Any `blocker` finding | `reject` |
| No blockers; at least one `warning` | `review` |
| No blockers and no warnings (including notice-only or no findings) | `pass` |

The `--fail-on` flag controls which verdict threshold causes the CLI to exit with code `1`. See the
[CLI Reference](cli.md) for details.

---

## Discovering Rules

### deltascope rules list

List all registered rules, with optional filters:

```bash
# All rules
deltascope rules list

# Filter by kind
deltascope rules list --kind ddl
deltascope rules list --kind dml

# Filter by severity level
deltascope rules list --level blocker
deltascope rules list --level warning

# Only show rules enabled under the loaded policy
deltascope rules list --enabled-only
```

Example output:

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

Display the full detail for a single rule:

```bash
deltascope rules show dml.where.require
```

Example output:

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

Search rules by keyword (matches against rule ID and description):

```bash
deltascope rules search "where clause"
deltascope rules search metadata
deltascope rules search "prefix"
```

---

## DDL: Create Table Rules

### Table-Level Rules (23 rules)

These rules evaluate properties of the `CREATE TABLE` statement as a whole.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.table.comment.require` | Table must have a non-empty COMMENT | warning | No |
| `ddl.table.comment.max_length` | Table COMMENT must not exceed character limit | warning | No |
| `ddl.table.name.max_length` | Table name length limit | blocker | No |
| `ddl.table.name.pattern.require` | Table name must match pattern (default: alphanumeric + underscore) | blocker | No |
| `ddl.table.name.keyword.forbid` | Table name must not be a reserved SQL keyword | blocker | No |
| `ddl.table.engine.allowlist` | Storage engine must be in allowlist (default: InnoDB) | blocker | No |
| `ddl.table.charset.allowlist` | Table default charset must be in allowlist | blocker | No |
| `ddl.table.row_format.allowlist` | ROW_FORMAT must be in allowlist (default: DYNAMIC) | blocker | No |
| `ddl.table.auto_increment.init_value.require` | AUTO_INCREMENT initial value must match required value | blocker | No |
| `ddl.table.columns.min_count` | Table must have at least N columns | blocker | No |
| `ddl.table.primary_key.require` | Table must have a PRIMARY KEY | blocker | No |
| `ddl.table.primary_key.columns.max_count` | PRIMARY KEY column count limit | warning | No |
| `ddl.table.primary_key.bigint.require` | PRIMARY KEY column must be BIGINT | blocker | No |
| `ddl.table.primary_key.unsigned.require` | PRIMARY KEY column must be UNSIGNED | blocker | No |
| `ddl.table.primary_key.auto_increment.require` | PRIMARY KEY column must be AUTO_INCREMENT | blocker | No |
| `ddl.table.primary_key.not_null.require` | PRIMARY KEY column must be NOT NULL | blocker | No |

**PostgreSQL primary-key availability (v0.37.0, v0.39.0):** `ddl.table.primary_key.bigint.require` and `ddl.table.primary_key.columns.max_count` now apply to PostgreSQL `CREATE TABLE` statements (v0.37.0) and `ALTER TABLE ... ADD CONSTRAINT ... PRIMARY KEY` statements (v0.39.0). `ddl.table.primary_key.not_null.require` does not produce a stable negative case for PostgreSQL because PK columns are treated as effectively NOT NULL.

**Default policy dialect isolation (v0.43.0):** Starting with v0.43.0, MySQL-family rules are automatically skipped when `--dialect postgresql` is set. This includes `ddl.table.primary_key.unsigned.require`, `ddl.table.primary_key.auto_increment.require`, `ddl.table.primary_key.not_null.require`, `ddl.table.engine.allowlist`, `ddl.table.charset.allowlist`, `ddl.table.row_format.allowlist`, `ddl.table.auto_increment.init_value.require`, `ddl.table.partition.forbid`, `ddl.table.create_as.forbid`, `ddl.table.create_like.forbid`, `ddl.column.charset.allowlist`, `ddl.column.collation.allowlist`, `ddl.column.charset_collation.match.require`, `ddl.alter.change_column.forbid`, `ddl.alter.modify_column.forbid`, and the `ON UPDATE CURRENT_TIMESTAMP` suggestion in the audit-columns check. Conversely, MySQL/TiDB audits exclude all `ddl.pg.*` and PostgreSQL-only dialect-gated rules. Isolation is enforced at the rule `AppliesTo` gate level.
| `ddl.table.audit_columns.require` | Table must include audit timestamp columns | warning | No |
| `ddl.table.foreign_key.forbid` | FOREIGN KEY constraints are forbidden | blocker | No |
| `ddl.table.partition.forbid` | Partitioned tables are forbidden | blocker | No |
| `ddl.table.create_like.forbid` | CREATE TABLE … LIKE is forbidden | blocker | No |
| `ddl.table.create_as.forbid` | CREATE TABLE … AS SELECT is forbidden | blocker | No |
| `ddl.table.row_size.max_bytes.require` | Estimated row size must not exceed InnoDB limits | blocker | **Yes** |
| `ddl.table.denylist.forbid` | DDL on schema/table entries in the denylist is forbidden | blocker | **Yes** |

### Column-Level Rules (16 rules)

These rules evaluate each column definition within a `CREATE TABLE` statement.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.column.name.max_length` | Column name length limit | blocker | No |
| `ddl.column.name.pattern.require` | Column name must match naming pattern | blocker | No |
| `ddl.column.name.keyword.forbid` | Column name must not be a reserved keyword | blocker | No |
| `ddl.column.comment.require` | Column must have a non-empty COMMENT | warning | No |
| `ddl.column.default.require` | Column must have a DEFAULT value | warning | No |
| `ddl.column.not_null.require` | Column must be NOT NULL | warning | No |
| `ddl.column.varchar.max_length` | VARCHAR length limit | blocker | No |
| `ddl.column.char.max_length` | CHAR length guidance | warning | No |
| `ddl.column.float_double.forbid` | FLOAT/DOUBLE types are discouraged | warning | No |
| `ddl.column.blob_text.forbid` | BLOB/TEXT types governance (disabled by default) | warning | No |
| `ddl.column.json.forbid` | JSON type governance (disabled by default) | warning | No |
| `ddl.column.bit.forbid` | BIT type governance (disabled by default) | warning | No |
| `ddl.column.timestamp.forbid` | TIMESTAMP type is forbidden (prefer DATETIME) | warning | No |
| `ddl.column.charset.allowlist` | Column charset must be in allowlist | blocker | No |
| `ddl.column.collation.allowlist` | Column collation must be in allowlist | blocker | No |
| `ddl.column.charset_collation.match.require` | Column charset and collation must be compatible | blocker | No |

### Index-Level Rules (11 rules)

These rules evaluate index definitions within a `CREATE TABLE` statement.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.index.total.max_count` | Maximum number of indexes per table | warning | No |
| `ddl.index.columns.max_count` | Maximum columns per index | warning | No |
| `ddl.index.name.pattern.require` | Index name must match naming pattern | blocker | No |
| `ddl.index.name.keyword.forbid` | Index name must not be a reserved keyword | blocker | No |
| `ddl.index.unique.prefix.require` | UNIQUE INDEX name must start with required prefix (default: `uniq_`) | warning | No |
| `ddl.index.secondary.prefix.require` | Regular INDEX name must start with required prefix (default: `idx_`) | warning | No |
| `ddl.index.fulltext.prefix.require` | FULLTEXT INDEX name must start with required prefix (default: `full_`) | warning | No |
| `ddl.index.duplicate.forbid` | Duplicate indexes (same columns, same order) are forbidden | warning | No |
| `ddl.index.redundant_left_prefix.forbid` | Index that is a left-prefix subset of another index is redundant | warning | No |
| `ddl.index.redundant_unique_overlap.forbid` | UNIQUE index made redundant by another UNIQUE index is forbidden | warning | No |
| `ddl.index.key_length.max_bytes.require` | Index key length must not exceed instance limits | blocker | **Yes** |

**PostgreSQL index availability (v0.38.0, updated v0.49.0):** `ddl.index.secondary.prefix.require`, `ddl.index.unique.prefix.require`, and `ddl.index.columns.max_count` now also apply to standalone PostgreSQL `CREATE INDEX`, `CREATE UNIQUE INDEX`, and `CREATE INDEX CONCURRENTLY` statements. Since v0.49.0, partial indexes, expression indexes, INCLUDE covering indexes, and non-btree access methods (GIN, hash, etc.) normalize through the audit pipeline instead of returning unsupported. Operator classes and NULLS NOT DISTINCT remain out of scope.

### View Rules (1 rule)

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.view.create.forbid` | CREATE VIEW is forbidden | blocker | No |

---

## DDL: Alter Table Rules

### Structural Changes (15 rules)

These rules govern the structural operations permitted within an `ALTER TABLE` statement.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.alter.drop_column.forbid` | DROP COLUMN governance (not enforced by default) | warning | No |
| `ddl.alter.drop_index.forbid` | DROP INDEX governance (not enforced by default) | warning | No |
| `ddl.alter.drop_primary_key.forbid` | DROP PRIMARY KEY is forbidden | blocker | No |
| `ddl.alter.rename_table.forbid` | RENAME TABLE is forbidden | blocker | No |
| `ddl.alter.rename_column.forbid` | RENAME COLUMN is forbidden | blocker | No |
| `ddl.alter.rename_index.forbid` | RENAME INDEX is forbidden | blocker | No |
| `ddl.alter.change_column.forbid` | CHANGE COLUMN is forbidden | blocker | No |
| `ddl.alter.modify_column.forbid` | MODIFY COLUMN governance (not enforced by default) | warning | No |
| `ddl.alter.add_index.columns.max_count` | ADD INDEX column count limit | warning | No |
| `ddl.alter.add_index.duplicate.forbid` | ADD INDEX must not duplicate an existing index | warning | No |
| `ddl.alter.add_index.redundant_left_prefix.forbid` | ADD INDEX must not be a left-prefix of an existing index | warning | No |
| `ddl.alter.add_index.redundant_unique_overlap.forbid` | ADD UNIQUE INDEX must not be made redundant by an existing UNIQUE index | warning | No |
| `ddl.alter.add_index.unique.prefix.require` | ADD UNIQUE INDEX name prefix requirement | warning | No |
| `ddl.alter.add_index.secondary.prefix.require` | ADD INDEX name prefix requirement | warning | No |
| `ddl.alter.add_index.fulltext.prefix.require` | ADD FULLTEXT INDEX name prefix requirement | warning | No |

**PostgreSQL ALTER TABLE ADD CONSTRAINT (v0.39.0):** `ddl.alter.add_index.unique.prefix.require` now also applies to PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... UNIQUE` forms. `ddl.table.primary_key.bigint.require` and `ddl.table.primary_key.columns.max_count` now also apply to PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... PRIMARY KEY`. These reuse existing shared alter-table index and primary-key rule families — no new rule IDs were added.

### Type Compatibility Rules (11 rules)

These rules check that column type changes made via `MODIFY COLUMN` or `CHANGE COLUMN` are safe and
compatible. Most of these rules require live metadata to compare the target type against the current
column definition.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.alter.modify_column.target_type_family.allowlist` | MODIFY COLUMN target type family must be in allowlist | blocker | No |
| `ddl.alter.change_column.target_type_family.allowlist` | CHANGE COLUMN target type family must be in allowlist | blocker | No |
| `ddl.alter.modify_column.compatibility.require` | MODIFY COLUMN must be compatible with current column type | blocker | **Yes** |
| `ddl.alter.change_column.compatibility.require` | CHANGE COLUMN must be compatible with current column type | blocker | **Yes** |
| `ddl.alter.modify_column.explicit_nullability_change.forbid` | MODIFY COLUMN must not explicitly change nullability | blocker | **Yes** |
| `ddl.alter.change_column.explicit_nullability_change.forbid` | CHANGE COLUMN must not explicitly change nullability | blocker | **Yes** |
| `ddl.alter.modify_column.explicit_default_change.forbid` | MODIFY COLUMN must not explicitly change DEFAULT value | blocker | **Yes** |
| `ddl.alter.change_column.explicit_default_change.forbid` | CHANGE COLUMN must not explicitly change DEFAULT value | blocker | **Yes** |
| `ddl.alter.modify_column.explicit_auto_increment_change.forbid` | MODIFY COLUMN must not add or remove AUTO_INCREMENT | blocker | **Yes** |
| `ddl.alter.change_column.explicit_auto_increment_change.forbid` | CHANGE COLUMN must not add or remove AUTO_INCREMENT | blocker | **Yes** |
| `ddl.alter.table_option.compatibility.require` | Table option changes must be compatible with current table options | warning | **Yes** |

### Existence Check Rules (11 rules — metadata-backed)

These rules verify that the objects referenced by an `ALTER TABLE` statement actually exist (or do not
already exist) in the live schema. They are silently skipped during offline audits.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.table.exists.alter.require` | ALTER TABLE target must exist | blocker | **Yes** |
| `ddl.table.exists.create.forbid` | CREATE TABLE target must not already exist | blocker | **Yes** |
| `ddl.alter.add_column.exists.forbid` | ADD COLUMN must not already exist | blocker | **Yes** |
| `ddl.alter.drop_column.exists.require` | DROP COLUMN target must exist | blocker | **Yes** |
| `ddl.alter.modify_column.exists.require` | MODIFY COLUMN target must exist | blocker | **Yes** |
| `ddl.alter.change_column.exists.require` | CHANGE COLUMN source must exist | blocker | **Yes** |
| `ddl.alter.rename_column.exists.require` | RENAME COLUMN source must exist | blocker | **Yes** |
| `ddl.alter.add_index.exists.forbid` | ADD INDEX name must not already exist | blocker | **Yes** |
| `ddl.alter.drop_index.exists.require` | DROP INDEX target must exist | blocker | **Yes** |
| `ddl.alter.rename_index.exists.require` | RENAME INDEX source must exist | blocker | **Yes** |
| `ddl.alter.drop_primary_key.exists.require` | DROP PRIMARY KEY requires a primary key to exist | blocker | **Yes** |

---

## DDL: Object Lifecycle Rules (8 rules)

These rules govern `DROP TABLE` and `TRUNCATE TABLE` operations.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.table.drop.forbid` | DROP TABLE is forbidden | blocker | No |
| `ddl.table.drop.exists.require` | DROP TABLE target must exist | blocker | **Yes** |
| `ddl.table.drop.adaptive_hash.warn` | DROP TABLE warns when adaptive hash index is active | warning | **Yes** |
| `ddl.table.drop.rows.max_count` | DROP TABLE warns when table has too many rows | warning | **Yes** |
| `ddl.table.truncate.forbid` | TRUNCATE TABLE is forbidden | blocker | No |
| `ddl.table.truncate.exists.require` | TRUNCATE TABLE target must exist | blocker | **Yes** |
| `ddl.table.truncate.adaptive_hash.warn` | TRUNCATE warns when adaptive hash index is active | warning | **Yes** |
| `ddl.table.truncate.rows.max_count` | TRUNCATE warns when table has too many rows | warning | **Yes** |

---

## DDL: Global Rules (3 rules)

Global rules evaluate across **all statements in a batch** after all statement-scoped rules have
completed. They cannot fire on a single statement in isolation.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.alter.merge.mysql.require` | Multiple ALTER TABLE on the same table should be merged (MySQL) | warning | No |
| `ddl.alter.merge.tidb.require` | Multiple ALTER TABLE on the same table guidance (TiDB, disabled by default) | warning | No |
| `ddl.pg.alter.not_valid_constraint.validate.require` | Named PostgreSQL CHECK/FK `NOT VALID` constraint should be followed by a later matching `VALIDATE CONSTRAINT` in the same audited SQL batch | warning | No |

> **Note:** `ddl.alter.merge.mysql.require` fires when two or more `ALTER TABLE` statements in the same
> input target the same table. In MySQL, each `ALTER TABLE` causes a table rebuild; merging them into a
> single statement dramatically reduces downtime. In TiDB, multiple alters are generally lighter-weight,
> so `ddl.alter.merge.tidb.require` is disabled in the default policy.
>
> Starting with `v0.42.0`, `ddl.pg.alter.not_valid_constraint.validate.require` fires as a PostgreSQL-only GlobalRule. It scans the same audited SQL batch for named `ALTER TABLE ... ADD CONSTRAINT ... NOT VALID` CHECK or FOREIGN KEY constraints and suppresses the warning only when a later `ALTER TABLE ... VALIDATE CONSTRAINT ...` matches the same schema, table, and constraint name. This is not first-time `VALIDATE CONSTRAINT` support, does not query live validation state, skips unnamed constraints, and does not track cross-file deployment windows.

---

## DDL: MySQL/TiDB Database/Schema Lifecycle Rules (2 rules)

These rules guard against MySQL and TiDB database/schema lifecycle DDL operations — `CREATE DATABASE`, `CREATE SCHEMA`, `DROP DATABASE`, and `DROP SCHEMA`. In MySQL/TiDB, `SCHEMA` is a synonym for `DATABASE`. They only apply when `--dialect mysql` or `--dialect tidb` is set and are skipped for PostgreSQL dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.database.create.notice` | `CREATE DATABASE` / `CREATE SCHEMA` creates a new logical namespace — informational notice | notice | No |
| `ddl.database.drop.warn` | `DROP DATABASE` / `DROP SCHEMA` removes a database and all contained objects — should be reviewed | warning | No |

> **Note:** These rules are MySQL/TiDB-specific and are automatically skipped when auditing PostgreSQL SQL. They are offline rules and do not require a database connection. `CREATE DATABASE IF NOT EXISTS` and `CREATE SCHEMA IF NOT EXISTS` still emit the notice. `DROP DATABASE IF EXISTS` and `DROP SCHEMA IF EXISTS` still emit the warning. DeltaScope does not perform live database existence validation. `CREATE DATABASE ... CHARACTER SET` / `COLLATE` options are preserved as parser facts but no policy rule governs them in this milestone. This is not full DDL support — trigger, routine, event, and database privilege lifecycle remain deferred.

---

## DDL: PostgreSQL Migration-Safety Rules (9 rules)

These rules guard against common PostgreSQL migration patterns that can cause table rewrites, long-held locks, or production incidents. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.create_index.concurrently.require` | `CREATE INDEX` must use `CONCURRENTLY` to avoid blocking reads/writes. Findings include bounded index shape metadata: `index_kind`, `access_method`, `column_count`, `included_column_count`, `has_predicate`, `has_expression_keys`, `expression_count`. Metadata is additive and does not emit predicate or expression SQL text. | warning | No |
| `ddl.pg.alter.add_column.non_null_default.rewrite.warn` | Adding a `NOT NULL` column with a volatile default may trigger a full table rewrite. Findings include: `not_null`, `has_default`, `default_kind` (`literal`, `null`, `function_call`, `expression`, `unknown`). Metadata does not emit default expression text or function names. | warning | No |
| `ddl.pg.alter.add_check.not_valid.require` | `ADD CHECK` constraint should use `NOT VALID` to avoid a full table scan with `ACCESS EXCLUSIVE` lock | warning | No |
| `ddl.pg.alter.set_data_type.rewrite.warn` | Changing a column type may require a full table rewrite depending on the conversion. Findings include: `has_using` (whether a USING clause is present). Metadata does not emit USING expression SQL text. | warning | No |
| `ddl.pg.alter.not_valid_constraint.validate.require` | Named CHECK/FK `NOT VALID` constraint lacks a later matching `VALIDATE CONSTRAINT` in the same audited SQL batch | warning | No |
| `ddl.pg.drop_index.advisory` | `DROP INDEX` removes an index — advises review of dependent queries | notice | No |
| `ddl.pg.alter.add_column.non_null_no_default.warn` | Adding a `NOT NULL` column without a `DEFAULT` can cause a full table rewrite on large tables. Findings include: `not_null`, `has_default`. | warning | No |
| `ddl.pg.alter.add_unique_constraint.concurrent_index.advisory` | `ADD UNIQUE CONSTRAINT` without `NOT VALID` and no subsequent `CREATE UNIQUE INDEX CONCURRENTLY` — advises concurrent index creation for zero-downtime deployments | notice | No |
| `ddl.pg.alter.drop_constraint.advisory` | `DROP CONSTRAINT` removes a CHECK, UNIQUE, or FOREIGN KEY constraint — advises review of dependent queries and data integrity | notice | No |

---

## DDL: PostgreSQL Object Lifecycle Rules (10 rules)

These rules guard against risky PostgreSQL object lifecycle DDL operations — schema, sequence, and materialized view CREATE/DROP/ALTER. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.create_schema.notice` | `CREATE SCHEMA` creates a new namespace — informational notice | notice | No |
| `ddl.pg.drop_schema.advisory` | `DROP SCHEMA` removes a schema — advises review of dependent objects | notice | No |
| `ddl.pg.drop_schema.cascade.warn` | `DROP SCHEMA ... CASCADE` uses cascading deletion — may silently drop dependent objects | warning | No |
| `ddl.pg.create_sequence.cycle.warn` | `CREATE SEQUENCE ... CYCLE` may cause sequence value wraparound | warning | No |
| `ddl.pg.alter_sequence.restart.warn` | `ALTER SEQUENCE ... RESTART` resets the sequence counter — may conflict with existing rows | warning | No |
| `ddl.pg.alter_sequence.cycle.warn` | `ALTER SEQUENCE ... CYCLE` enables value wraparound on an existing sequence | warning | No |
| `ddl.pg.drop_sequence.advisory` | `DROP SEQUENCE` removes a sequence — advises review of dependent columns | notice | No |
| `ddl.pg.drop_sequence.cascade.warn` | `DROP SEQUENCE ... CASCADE` uses cascading deletion — may silently drop dependent objects | warning | No |
| `ddl.pg.drop_materialized_view.advisory` | `DROP MATERIALIZED VIEW` removes a materialized view — advises review of dependent queries | notice | No |
| `ddl.pg.drop_materialized_view.cascade.warn` | `DROP MATERIALIZED VIEW ... CASCADE` uses cascading deletion — may silently drop dependent objects | warning | No |
| `ddl.pg.refresh_materialized_view.concurrently.warn` | Non-concurrent `REFRESH MATERIALIZED VIEW` (default or explicit `WITH DATA`) holds an exclusive lock | warning | No |
| `ddl.pg.refresh_materialized_view.no_data.notice` | `REFRESH MATERIALIZED VIEW ... WITH NO DATA` empties the view — downstream readers may see empty results | notice | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. `CREATE SCHEMA IF NOT EXISTS` still emits the notice from `ddl.pg.create_schema.notice`. Existing `DROP SCHEMA` behavior (`ddl.pg.drop_schema.advisory`, `ddl.pg.drop_schema.cascade.warn`) is unchanged. `CREATE SCHEMA AUTHORIZATION` and nested `CREATE SCHEMA ... CREATE TABLE ...` remain unsupported/deferred. DeltaScope does not perform live schema existence validation. `CONCURRENTLY` refreshes pass both materialized view rules without findings. `WITH NO DATA` triggers both rules because it is also non-concurrent. This is not live unique-index validation for `CONCURRENTLY` — DeltaScope does not verify whether a unique index exists on the materialized view.

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. Starting with `v0.41.0`, `ddl.pg.alter.add_check.not_valid.require` also fires on `ALTER TABLE ... ADD CONSTRAINT ... CHECK` statements. Starting with `v0.42.0`, `ddl.pg.alter.not_valid_constraint.validate.require` checks same-batch validation pairing for named CHECK and FOREIGN KEY `NOT VALID` constraints. Check naming rules (`ddl.constraint.check.name.prefix.require`, `ddl.constraint.check.name.suffix.require`, `ddl.constraint.check.name.contains.require`) also cover the ALTER TABLE CHECK path when configured.

---

## DDL: PostgreSQL Type Lifecycle Rules (5 rules)

These rules guard against risky PostgreSQL type lifecycle DDL operations — enum type creation, value addition, and type drops. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.create_type.enum.notice` | `CREATE TYPE ... AS ENUM` introduces a new enum type — informational notice | notice | No |
| `ddl.pg.alter_type.add_value.advisory` | `ALTER TYPE ... ADD VALUE` appends a value to an existing enum — advises review of application usage | warning | No |
| `ddl.pg.alter_type.add_value.position.notice` | `ALTER TYPE ... ADD VALUE ... BEFORE/AFTER` positions a new enum value — informational notice | notice | No |
| `ddl.pg.drop_type.advisory` | `DROP TYPE` removes a user-defined type — advises review of dependent columns and functions | warning | No |
| `ddl.pg.drop_type.cascade.warn` | `DROP TYPE ... CASCADE` uses cascading deletion — may silently drop dependent objects | warning | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. DeltaScope does not inspect live dependent objects, validate whether enum values are already used by data or application code, or model full PostgreSQL type system semantics. Composite types are now supported — see Composite Type Lifecycle Rules below. Domains are supported — see Domain Lifecycle Rules below.

---

## DDL: PostgreSQL Composite Type Lifecycle Rules (3 rules)

These rules guard PostgreSQL composite type lifecycle DDL operations — composite type creation, rename, and schema move. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.create_type.composite.notice` | `CREATE TYPE ... AS (...)` introduces a new composite type — informational notice | notice | No |
| `ddl.pg.alter_type.composite_rename.notice` | `ALTER TYPE ... RENAME TO` changes the composite type name — informational notice | notice | No |
| `ddl.pg.alter_type.composite_set_schema.notice` | `ALTER TYPE ... SET SCHEMA` moves the composite type to a different schema — informational notice | notice | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. `DROP TYPE` is not covered by composite-specific rules — it reuses the existing `ddl.pg.drop_type.advisory` and `ddl.pg.drop_type.cascade.warn` rules from the Type Lifecycle family. Attribute-level operations (`ADD ATTRIBUTE`, `DROP ATTRIBUTE`, `ALTER ATTRIBUTE ... TYPE`, `RENAME ATTRIBUTE`) are explicitly unsupported/deferred. DeltaScope recognizes `COLLATE` annotations on composite type attributes structurally but does not render, interpret, or validate collation semantics.

---

## DDL: PostgreSQL Domain Lifecycle Rules (7 rules)

These rules guard against risky PostgreSQL domain lifecycle DDL operations — domain creation, constraint/default/nullability changes, renames, and drops. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.create_domain.notice` | `CREATE DOMAIN` introduces a reusable type constraint — informational notice | notice | No |
| `ddl.pg.alter_domain.constraint.notice` | `ALTER DOMAIN ... ADD/DROP/VALIDATE CONSTRAINT` modifies the type contract — informational notice | notice | No |
| `ddl.pg.alter_domain.default.notice` | `ALTER DOMAIN ... SET/DROP DEFAULT` changes the implicit value — informational notice | notice | No |
| `ddl.pg.alter_domain.not_null.notice` | `ALTER DOMAIN ... SET/DROP NOT NULL` changes nullability — informational notice | notice | No |
| `ddl.pg.alter_domain.rename.notice` | `ALTER DOMAIN ... RENAME TO` changes the domain name — informational notice | notice | No |
| `ddl.pg.drop_domain.advisory` | `DROP DOMAIN` removes a domain — advises review of dependent columns | warning | No |
| `ddl.pg.drop_domain.cascade.warn` | `DROP DOMAIN ... CASCADE` uses cascading deletion — may silently drop dependent objects | warning | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. DeltaScope does not render `CHECK` or `DEFAULT` expression text — rules emit boolean facts (`has_check`, `has_default`, `not_null`) and constraint names where available, but never the expression body. DeltaScope does not perform live dependency validation on domains. `DROP DOMAIN IF EXISTS ... CASCADE` intentionally emits both `ddl.pg.drop_domain.advisory` and `ddl.pg.drop_domain.cascade.warn`. Composite types are now supported — see Composite Type Lifecycle Rules above. Extensions are now supported — see Extension Lifecycle Rules below.

---

## DDL: PostgreSQL Extension Lifecycle Rules (6 rules)

These rules guard against risky PostgreSQL extension lifecycle DDL operations — extension creation, updates, schema moves, and drops. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.create_extension.notice` | `CREATE EXTENSION` installs an extension into the database — informational notice | notice | No |
| `ddl.pg.create_extension.cascade.warn` | `CREATE EXTENSION ... CASCADE` auto-installs dependencies — may introduce unintended extensions | warning | No |
| `ddl.pg.alter_extension.update.notice` | `ALTER EXTENSION ... UPDATE` / `UPDATE TO` upgrades an extension — informational notice | notice | No |
| `ddl.pg.alter_extension.set_schema.notice` | `ALTER EXTENSION ... SET SCHEMA` moves the extension to a different schema — informational notice | notice | No |
| `ddl.pg.drop_extension.advisory` | `DROP EXTENSION` removes an extension — advises review of dependent objects | warning | No |
| `ddl.pg.drop_extension.cascade.warn` | `DROP EXTENSION ... CASCADE` uses cascading deletion — may silently drop dependent objects | warning | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. `CREATE EXTENSION ... CASCADE` intentionally emits both `ddl.pg.create_extension.notice` and `ddl.pg.create_extension.cascade.warn`. `DROP EXTENSION ... CASCADE` intentionally emits both `ddl.pg.drop_extension.advisory` and `ddl.pg.drop_extension.cascade.warn`. DeltaScope does not perform live validation of extension availability, installed packages, version compatibility, or dependency graphs. Extension member mutation (`ALTER EXTENSION ... ADD/DROP TABLE`) is explicitly unsupported/deferred. Table-level privilege DCL is now supported — see Table Privilege DCL Rules below.

---

## DDL: PostgreSQL Table Privilege DCL Rules (4 rules)

These rules guard PostgreSQL table-level privilege DCL operations — `GRANT ... ON TABLE` and `REVOKE ... ON TABLE`. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.grant.table_privilege.notice` | `GRANT ... ON TABLE` grants table-level privileges — informational notice | notice | No |
| `ddl.pg.grant.table_privilege.all.warn` | `GRANT ALL PRIVILEGES ON TABLE` grants all privileges — warns about over-permission | warning | No |
| `ddl.pg.revoke.table_privilege.notice` | `REVOKE ... ON TABLE` revokes table-level privileges — informational notice | notice | No |
| `ddl.pg.revoke.table_privilege.cascade.warn` | `REVOKE ... ON TABLE ... CASCADE` cascades revocation to dependent privileges — warns about cascade side-effects | warning | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. `GRANT ALL PRIVILEGES ON TABLE` intentionally emits both `ddl.pg.grant.table_privilege.notice` and `ddl.pg.grant.table_privilege.all.warn`. `REVOKE ... ON TABLE ... CASCADE` intentionally emits both `ddl.pg.revoke.table_privilege.notice` and `ddl.pg.revoke.table_privilege.cascade.warn`. Supports multiple privileges (e.g., `SELECT, INSERT`), multiple grantees, and schema-qualified table names (e.g., `public.users`). DeltaScope does not perform live validation — no grantee/role existence checks, no table/object existence checks, no grantor permission checks, no effective privilege computation, no role inheritance resolution, no ownership verification, and no RLS/policy evaluation. `ALL TABLES IN SCHEMA`, sequence privileges, role membership GRANT/REVOKE, and `ALTER DEFAULT PRIVILEGES` are not supported. This is narrow table-level privilege DCL support — not broad governance or admin DCL support.

---

## DDL: PostgreSQL ALTER TABLE Coverage Rules (22 rules)

These rules extend PostgreSQL ALTER TABLE audit coverage beyond the migration-safety and object lifecycle families. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.alter.drop_column.advisory` | `ALTER TABLE ... DROP COLUMN` removes a column — advises review of dependent queries and application logic | warning | No |
| `ddl.pg.alter.validate_constraint.advisory` | `ALTER TABLE ... VALIDATE CONSTRAINT` runs a validation scan — advises awareness of table-level lock duration on large tables | notice | No |
| `ddl.pg.alter.add_column.nullable.notice` | `ALTER TABLE ... ADD COLUMN` adds a nullable column without a DEFAULT — note that downstream code may encounter unexpected NULL values | notice | No |
| `ddl.pg.alter.set_schema.advisory` | `ALTER TABLE ... SET SCHEMA` moves the table to a different schema — advises review of dependent queries and application connections | notice | No |
| `ddl.pg.alter.owner.advisory` | `ALTER TABLE ... OWNER TO` changes the table owner — advises review of permission implications | notice | No |
| `ddl.pg.alter.enable_trigger.notice` | `ALTER TABLE ... ENABLE TRIGGER name` re-enables a specific trigger — informational notice | notice | No |
| `ddl.pg.alter.disable_trigger.warn` | `ALTER TABLE ... DISABLE TRIGGER name` disables a specific trigger — warns that triggers will not fire on the table | warning | No |
| `ddl.pg.alter.attach_partition.advisory` | `ALTER TABLE ... ATTACH PARTITION` attaches a partition to a partitioned table — advises review of partition boundary and data routing | notice | No |
| `ddl.pg.alter.detach_partition.warn` | `ALTER TABLE ... DETACH PARTITION` detaches a partition — warns that queries targeting the partition may fail | warning | No |
| `ddl.pg.alter.set_logged.notice` | `ALTER TABLE ... SET LOGGED` changes an unlogged table to logged — informational notice | notice | No |
| `ddl.pg.alter.set_unlogged.notice` | `ALTER TABLE ... SET UNLOGGED` changes a logged table to unlogged — informational notice | notice | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. Trigger-scope forms (`ENABLE/DISABLE TRIGGER ALL/USER`) are now normalized and reuse these same rules. This is not full PostgreSQL ALTER TABLE coverage. DeltaScope does not verify whether the target table is currently logged or unlogged.

### Storage / Layout (v0.130.0)

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.alter.set_tablespace.notice` | `ALTER TABLE ... SET TABLESPACE` moves the table to a different tablespace — informational notice | notice | No |
| `ddl.pg.alter.set_access_method.warn` | `ALTER TABLE ... SET ACCESS METHOD` changes the table access method — warns about rewrite and compatibility implications | warning | No |

### Trigger / Rule Residual (v0.130.0)

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.alter.enable_replica_trigger.notice` | `ALTER TABLE ... ENABLE REPLICA TRIGGER` enables a trigger in replica mode — informational notice | notice | No |
| `ddl.pg.alter.enable_always_trigger.notice` | `ALTER TABLE ... ENABLE ALWAYS TRIGGER` enables a trigger in always mode — informational notice | notice | No |
| `ddl.pg.alter.enable_rule.notice` | `ALTER TABLE ... ENABLE RULE` enables a rewrite rule — informational notice | notice | No |
| `ddl.pg.alter.disable_rule.warn` | `ALTER TABLE ... DISABLE RULE` disables a rewrite rule — warns that the rule will not fire | warning | No |
| `ddl.pg.alter.enable_replica_rule.notice` | `ALTER TABLE ... ENABLE REPLICA RULE` enables a rule in replica mode — informational notice | notice | No |
| `ddl.pg.alter.enable_always_rule.notice` | `ALTER TABLE ... ENABLE ALWAYS RULE` enables a rule in always mode — informational notice | notice | No |

### Reloptions (v0.130.0)

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.alter.set_reloptions.warn` | `ALTER TABLE ... SET (...)` sets storage parameters — warns about potential rewrite or behavior changes | warning | No |
| `ddl.pg.alter.reset_reloptions.notice` | `ALTER TABLE ... RESET (...)` resets storage parameters to defaults — informational notice | notice | No |

> **Note (v0.130.0):** These rules are PostgreSQL-specific and offline. No trigger function names, trigger body text, rule query text, rule command text, tablespace names, access method names, or reloption keys/values (e.g., `fillfactor`, `autovacuum_enabled`) are emitted in findings.

### Column Attributes (v0.140.0)

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.alter.set_column_statistics.notice` | `ALTER TABLE ... ALTER COLUMN ... SET STATISTICS` overrides column statistics target — informational notice | notice | No |
| `ddl.pg.alter.set_column_options.notice` | `ALTER TABLE ... ALTER COLUMN ... SET (...)` sets attribute options — informational notice | notice | No |
| `ddl.pg.alter.reset_column_options.notice` | `ALTER TABLE ... ALTER COLUMN ... RESET (...)` resets attribute options — informational notice | notice | No |
| `ddl.pg.alter.set_column_storage.notice` | `ALTER TABLE ... ALTER COLUMN ... SET STORAGE` changes column storage strategy — informational notice | notice | No |
| `ddl.pg.alter.set_column_compression.notice` | `ALTER TABLE ... ALTER COLUMN ... SET COMPRESSION` changes column compression method — informational notice | notice | No |

### Cluster / Detach-Finalize (v0.140.0)

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.alter.cluster_on.notice` | `ALTER TABLE ... CLUSTER ON` clusters the table on an index — informational notice | notice | No |
| `ddl.pg.alter.set_without_cluster.notice` | `ALTER TABLE ... SET WITHOUT CLUSTER` removes the cluster specification — informational notice | notice | No |
| `ddl.pg.alter.detach_partition_finalize.notice` | `ALTER TABLE ... DETACH PARTITION ... FINALIZE` forcibly completes a pending detach — informational notice | notice | No |

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.alter.replica_identity_full.warn` | `ALTER TABLE ... REPLICA IDENTITY FULL` writes full old-row images to WAL — warns about replication overhead | warning | No |
| `ddl.pg.alter.replica_identity_nothing.warn` | `ALTER TABLE ... REPLICA IDENTITY NOTHING` writes no old-row images to WAL — warns that logical replication will not work | warning | No |
| `ddl.pg.alter.replica_identity_using_index.notice` | `ALTER TABLE ... REPLICA IDENTITY USING INDEX ...` uses a specific index for WAL old-row images — informational notice | notice | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. `REPLICA IDENTITY DEFAULT` is normalized and intentionally silent. DeltaScope does not verify whether `REPLICA IDENTITY USING INDEX` names a valid, unique, or non-partial index.

---

## DDL: PostgreSQL RLS/Policy Lifecycle Rules (7 rules)

These rules guard PostgreSQL Row-Level Security policy and RLS toggle operations. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.create_policy.notice` | `CREATE POLICY` introduces a new RLS policy — informational notice | notice | No |
| `ddl.pg.alter_policy.notice` | `ALTER POLICY` modifies an existing RLS policy — informational notice | notice | No |
| `ddl.pg.drop_policy.warn` | `DROP POLICY` removes an RLS policy — warns that row-level protection is removed | warning | No |
| `ddl.pg.alter.enable_rls.notice` | `ALTER TABLE ... ENABLE ROW LEVEL SECURITY` enables RLS — informational notice | notice | No |
| `ddl.pg.alter.disable_rls.warn` | `ALTER TABLE ... DISABLE ROW LEVEL SECURITY` disables RLS — warns that row-level protection is turned off | warning | No |
| `ddl.pg.alter.force_rls.notice` | `ALTER TABLE ... FORCE ROW LEVEL SECURITY` forces RLS for table owner — informational notice | notice | No |
| `ddl.pg.alter.no_force_rls.notice` | `ALTER TABLE ... NO FORCE ROW LEVEL SECURITY` un-forces RLS for table owner — informational notice | notice | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. DeltaScope does not evaluate policy expressions, verify policy applicability for specific roles, or inspect live RLS state. `CREATE POLICY ... AS PERMISSIVE` and `CREATE POLICY ... AS RESTRICTIVE` are both covered by `ddl.pg.create_policy.notice`. Policy `WITH CHECK` and `USING` expression text is not rendered. This is not full PostgreSQL RLS governance.

---

## DDL: PostgreSQL Trigger Lifecycle Rules (3 rules)

These rules guard PostgreSQL trigger lifecycle operations. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.create_trigger.notice` | `CREATE TRIGGER` introduces a new trigger — informational notice | notice | No |
| `ddl.pg.create_constraint_trigger.warn` | `CREATE CONSTRAINT TRIGGER` creates a constraint trigger — warns about constraint-trigger semantics | warning | No |
| `ddl.pg.drop_trigger.advisory` | `DROP TRIGGER` removes a trigger — advises review of dependent logic | notice | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. DeltaScope does not evaluate trigger bodies, verify trigger function existence, or inspect live trigger state. `INSTEAD OF` triggers and transition tables (`REFERENCING OLD TABLE / NEW TABLE`) are normalized but do not produce separate findings. This is not full PostgreSQL trigger governance.

---

## DDL: PostgreSQL Function/Procedure Lifecycle Rules (6 rules)

These rules guard PostgreSQL function and procedure lifecycle operations. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.create_function.notice` | `CREATE FUNCTION` introduces a new function — informational notice | notice | No |
| `ddl.pg.create_function.security_definer.warn` | `CREATE FUNCTION ... SECURITY DEFINER` executes with owner privileges — warns about privilege escalation | warning | No |
| `ddl.pg.create_or_replace_function.advisory` | `CREATE OR REPLACE FUNCTION` replaces an existing function — advises review of downstream dependencies | notice | No |
| `ddl.pg.drop_function.advisory` | `DROP FUNCTION` removes a function — advises review of dependent objects | notice | No |
| `ddl.pg.create_procedure.notice` | `CREATE PROCEDURE` introduces a new procedure — informational notice | notice | No |
| `ddl.pg.drop_procedure.advisory` | `DROP PROCEDURE` removes a procedure — advises review of dependent objects | notice | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. `CREATE FUNCTION ... SECURITY DEFINER` intentionally emits both `ddl.pg.create_function.notice` and `ddl.pg.create_function.security_definer.warn`. DeltaScope does not evaluate function bodies, verify argument types, inspect live function state, or resolve `LANGUAGE` / `VOLATILITY` / `PARALLEL` safety. Function argument signatures are not modeled. This is not full PostgreSQL function/procedure governance.

---

## DDL: PostgreSQL Advanced View Lifecycle Rules (6 rules)

These rules guard PostgreSQL view lifecycle operations beyond the base `CREATE VIEW` / `DROP VIEW` forms. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.create_or_replace_view.advisory` | `CREATE OR REPLACE VIEW` replaces an existing view — advises review of downstream dependencies | notice | No |
| `ddl.pg.create_temp_view.notice` | `CREATE TEMP VIEW` / `CREATE TEMPORARY VIEW` creates a session-scoped view — informational notice | notice | No |
| `ddl.pg.create_view.check_option.notice` | `CREATE VIEW ... WITH CHECK OPTION` enforces check option on inserts/updates through the view — informational notice | notice | No |
| `ddl.pg.drop_view.cascade.warn` | `DROP VIEW ... CASCADE` uses cascading deletion — may silently drop dependent objects | warning | No |
| `ddl.pg.alter_view.rename.notice` | `ALTER VIEW ... RENAME TO` changes the view name — informational notice | notice | No |
| `ddl.pg.alter_view.set_schema.notice` | `ALTER VIEW ... SET SCHEMA` moves the view to a different schema — informational notice | notice | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. `CREATE OR REPLACE VIEW` intentionally emits both the base `ddl.view.create.forbid` (when enabled) and `ddl.pg.create_or_replace_view.advisory`. `CASCADED` vs `LOCAL` check option semantics are not modeled. DeltaScope does not evaluate view query bodies or inspect live view state. This is not full PostgreSQL view governance.

---

## DDL: PostgreSQL Selected ALTER Object Lifecycle Rules (6 rules)

These rules guard selected PostgreSQL ALTER object operations for schema, index, and materialized view. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.alter_schema.rename.notice` | `ALTER SCHEMA ... RENAME TO` changes the schema name — informational notice | notice | No |
| `ddl.pg.alter_schema.owner.notice` | `ALTER SCHEMA ... OWNER TO` changes the schema owner — informational notice | notice | No |
| `ddl.pg.alter_index.rename.notice` | `ALTER INDEX ... RENAME TO` changes the index name — informational notice | notice | No |
| `ddl.pg.alter_index.set_tablespace.notice` | `ALTER INDEX ... SET TABLESPACE` moves the index to a different tablespace — informational notice | notice | No |
| `ddl.pg.alter_materialized_view.rename.notice` | `ALTER MATERIALIZED VIEW ... RENAME TO` changes the materialized view name — informational notice | notice | No |
| `ddl.pg.alter_materialized_view.set_schema.notice` | `ALTER MATERIALIZED VIEW ... SET SCHEMA` moves the materialized view to a different schema — informational notice | notice | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. DeltaScope does not verify live schema/index/materialized-view existence, ownership, or tablespace availability. This is not full PostgreSQL ALTER object lifecycle coverage — remaining ALTER forms for these object types (e.g., `ALTER INDEX ... SET (...)`, `ALTER MATERIALIZED VIEW ... OWNER TO`) are deferred.

---

## DDL: PostgreSQL Composite Type Attribute Lifecycle Rules (4 rules)

`v0.80.0` adds selected PostgreSQL non-permission DDL deep coverage for composite type attribute mutations. These 4 rules cover `ALTER TYPE ... ADD ATTRIBUTE`, `DROP ATTRIBUTE`, `ALTER ATTRIBUTE ... TYPE`, and `RENAME ATTRIBUTE`, which were previously listed as unsupported/deferred in the Composite Type Lifecycle section. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.alter_type.add_attribute.notice` | `ALTER TYPE ... ADD ATTRIBUTE` adds a new attribute to a composite type — informational notice | notice | No |
| `ddl.pg.alter_type.drop_attribute.warn` | `ALTER TYPE ... DROP ATTRIBUTE` removes an attribute from a composite type — warns about dependent columns and functions | warning | No |
| `ddl.pg.alter_type.alter_attribute_type.warn` | `ALTER TYPE ... ALTER ATTRIBUTE ... TYPE` changes an attribute type — warns about potential data conversion issues | warning | No |
| `ddl.pg.alter_type.rename_attribute.notice` | `ALTER TYPE ... RENAME ATTRIBUTE` renames an attribute — informational notice | notice | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. These rules replace the previously listed unsupported/deferred entries for `ADD ATTRIBUTE`, `DROP ATTRIBUTE`, `ALTER ATTRIBUTE ... TYPE`, and `RENAME ATTRIBUTE` in the Composite Type Lifecycle section. DeltaScope does not inspect live dependent objects, validate data conversion safety, or model full PostgreSQL type system semantics. `DROP TYPE` reuses existing rules from the Type Lifecycle family. No MySQL/TiDB behavior changes.

---

## DDL: PostgreSQL Extension Member Lifecycle Rules (2 rules)

`v0.80.0` adds selected PostgreSQL non-permission DDL deep coverage for extension member mutation. These 2 rules cover `ALTER EXTENSION ... ADD TABLE` and `ALTER EXTENSION ... DROP TABLE`, which were previously listed as unsupported/deferred in the Extension Lifecycle section. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.alter_extension.add_member.notice` | `ALTER EXTENSION ... ADD TABLE` adds an object to the extension — informational notice | notice | No |
| `ddl.pg.alter_extension.drop_member.warn` | `ALTER EXTENSION ... DROP TABLE` removes an object from the extension — warns that the object may be dropped when the extension is dropped | warning | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. These rules replace the previously listed unsupported/deferred entries for extension member mutation (`ALTER EXTENSION ... ADD/DROP TABLE`) in the Extension Lifecycle section. DeltaScope does not validate whether the referenced object exists, verify extension membership state, or inspect live dependency graphs. No MySQL/TiDB behavior changes.

---

## DDL: PostgreSQL Publication/Subscription Lifecycle Rules (7 rules)

`v0.80.0` adds selected PostgreSQL non-permission DDL deep coverage for logical replication publication and subscription lifecycle. These 7 rules cover `CREATE PUBLICATION`, `ALTER PUBLICATION`, `DROP PUBLICATION`, `CREATE SUBSCRIPTION`, `ALTER SUBSCRIPTION`, `ALTER SUBSCRIPTION ... DISABLE`, and `DROP SUBSCRIPTION`. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.create_publication.notice` | `CREATE PUBLICATION` introduces a new publication for logical replication — informational notice | notice | No |
| `ddl.pg.alter_publication.notice` | `ALTER PUBLICATION` modifies an existing publication — informational notice | notice | No |
| `ddl.pg.drop_publication.warn` | `DROP PUBLICATION` removes a publication — warns that subscribers will stop receiving changes | warning | No |
| `ddl.pg.create_subscription.notice` | `CREATE SUBSCRIPTION` establishes a new subscription connection — informational notice | notice | No |
| `ddl.pg.alter_subscription.notice` | `ALTER SUBSCRIPTION` modifies an existing subscription — informational notice | notice | No |
| `ddl.pg.alter_subscription.disable.warn` | `ALTER SUBSCRIPTION ... DISABLE` disables the subscription — warns that replication will stop | warning | No |
| `ddl.pg.drop_subscription.warn` | `DROP SUBSCRIPTION` removes a subscription — warns about replication slot cleanup | warning | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. `DROP SUBSCRIPTION ... WITH (drop_slot = true)` remains deferred (parser_error) — DeltaScope does not parse the `WITH` options clause on `DROP SUBSCRIPTION`. DeltaScope does not verify live publication/subscription state, replication slot status, or connection parameters. Publication column lists and row filters are preserved as parser facts but no policy rule governs them. This is not full PostgreSQL logical replication governance. No MySQL/TiDB behavior changes.

---

## DDL: PostgreSQL Foreign Object Lifecycle Rules (12 rules)

`v0.80.0` adds selected PostgreSQL non-permission DDL deep coverage for foreign data wrapper, foreign server, user mapping, and foreign table lifecycle. These 12 rules cover CREATE/ALTER/DROP operations for all four foreign-data object types. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.create_foreign_data_wrapper.notice` | `CREATE FOREIGN DATA WRAPPER` introduces a new FDW — informational notice | notice | No |
| `ddl.pg.alter_foreign_data_wrapper.notice` | `ALTER FOREIGN DATA WRAPPER` modifies an existing FDW — informational notice | notice | No |
| `ddl.pg.drop_foreign_data_wrapper.warn` | `DROP FOREIGN DATA WRAPPER` removes an FDW — warns about dependent foreign servers and tables | warning | No |
| `ddl.pg.create_foreign_server.notice` | `CREATE SERVER` registers a new foreign server — informational notice | notice | No |
| `ddl.pg.alter_foreign_server.notice` | `ALTER SERVER` modifies an existing foreign server — informational notice | notice | No |
| `ddl.pg.drop_foreign_server.warn` | `DROP SERVER` removes a foreign server — warns about dependent user mappings and foreign tables | warning | No |
| `ddl.pg.create_user_mapping.notice` | `CREATE USER MAPPING` registers a user mapping for a foreign server — informational notice | notice | No |
| `ddl.pg.alter_user_mapping.notice` | `ALTER USER MAPPING` modifies an existing user mapping — informational notice | notice | No |
| `ddl.pg.drop_user_mapping.warn` | `DROP USER MAPPING` removes a user mapping — warns about dependent foreign table connections | warning | No |
| `ddl.pg.create_foreign_table.notice` | `CREATE FOREIGN TABLE` introduces a new foreign table — informational notice | notice | No |
| `ddl.pg.alter_foreign_table.notice` | `ALTER FOREIGN TABLE` modifies an existing foreign table — informational notice | notice | No |
| `ddl.pg.drop_foreign_table.warn` | `DROP FOREIGN TABLE` removes a foreign table — warns about dependent queries | warning | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. DeltaScope does not verify live foreign-data object existence, connection parameters, FDW handler/validator functions, or foreign table column compatibility. FDW options (`OPTIONS (...)`) are preserved as parser facts but no policy rule governs them. `IMPORT FOREIGN SCHEMA` remains unsupported/deferred. This is not full PostgreSQL foreign-data governance. No MySQL/TiDB behavior changes.

---

## DDL: PostgreSQL Annotation Lifecycle Rules (4 rules)

`v0.80.0` adds selected PostgreSQL non-permission DDL deep coverage for object annotation operations. These 4 rules cover `COMMENT ON` and `SECURITY LABEL` for setting and removing annotations. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.comment_on.notice` | `COMMENT ON ... IS 'text'` attaches a comment to a database object — informational notice | notice | No |
| `ddl.pg.comment_on.remove.notice` | `COMMENT ON ... IS NULL` removes the comment from a database object — informational notice | notice | No |
| `ddl.pg.security_label.notice` | `SECURITY LABEL ... IS 'label'` attaches a security label to a database object — informational notice | notice | No |
| `ddl.pg.security_label.remove.notice` | `SECURITY LABEL ... IS NULL` removes the security label from a database object — informational notice | notice | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. DeltaScope does not validate whether the target object exists, verify comment/label content against policies, or inspect live annotation state. `SECURITY LABEL ... FOR provider ...` provider names are preserved as parser facts but no policy rule governs them. This is not full PostgreSQL annotation governance. No MySQL/TiDB behavior changes.

---

## DDL: PostgreSQL Event Trigger / Rewrite Rule Lifecycle Rules (7 rules)

`v0.80.0` adds selected PostgreSQL non-permission DDL deep coverage for event triggers and rewrite rules. These 7 rules cover `CREATE EVENT TRIGGER`, `ALTER EVENT TRIGGER`, `ALTER EVENT TRIGGER ... DISABLE`, `DROP EVENT TRIGGER`, `CREATE RULE`, `ALTER RULE`, and `DROP RULE`. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.create_event_trigger.notice` | `CREATE EVENT TRIGGER` introduces a new event trigger — informational notice | notice | No |
| `ddl.pg.alter_event_trigger.notice` | `ALTER EVENT TRIGGER` modifies an existing event trigger — informational notice | notice | No |
| `ddl.pg.alter_event_trigger.disable.warn` | `ALTER EVENT TRIGGER ... DISABLE` disables an event trigger — warns that DDL event handling will stop | warning | No |
| `ddl.pg.drop_event_trigger.warn` | `DROP EVENT TRIGGER` removes an event trigger — warns about DDL event handling implications | warning | No |
| `ddl.pg.create_rule.notice` | `CREATE RULE` introduces a new rewrite rule — informational notice | notice | No |
| `ddl.pg.alter_rule.notice` | `ALTER RULE` modifies an existing rewrite rule — informational notice | notice | No |
| `ddl.pg.drop_rule.warn` | `DROP RULE` removes a rewrite rule — warns about dependent query behavior | warning | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. DeltaScope does not evaluate event trigger bodies or rewrite rule actions, verify trigger function existence, or inspect live event trigger/rule state. Event trigger `WHEN` conditions and rule `INSTEAD` / `ALSO` semantics are preserved as parser facts but no policy rule governs them. This is not full PostgreSQL event trigger/rewrite rule governance. No MySQL/TiDB behavior changes.

---

## DML Rules (10 rules)

These rules evaluate DML statements: `SELECT`, `INSERT`, `UPDATE`, `DELETE`, and `REPLACE`.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `dml.where.require` | UPDATE/DELETE must have a WHERE clause | blocker | No |
| `dml.limit.forbid` | UPDATE/DELETE with LIMIT is discouraged | warning | No |
| `dml.order_by.forbid` | UPDATE/DELETE with ORDER BY is discouraged | warning | No |
| `dml.subquery.forbid` | DML with subqueries is forbidden | blocker | No |
| `dml.join.on.require` | JOIN must have an ON condition | blocker | No |
| `dml.insert.rows.max_count` | INSERT VALUES row count limit | warning | No |
| `dml.replace.forbid` | REPLACE INTO is forbidden | blocker | No |
| `dml.insert.select.forbid` | INSERT INTO … SELECT is forbidden | blocker | No |
| `dml.insert.on_duplicate.forbid` | INSERT … ON DUPLICATE KEY UPDATE is forbidden | blocker | No |
| `dml.table.denylist.forbid` | DML on schema/table entries in the denylist is forbidden | blocker | **Yes** |

---

## Offline vs Metadata-Backed Rules

Rules fall into two categories based on whether they require a live database connection:

**Offline rules** always run, even when no database connection is provided. They evaluate the SQL text
and AST alone. All rules not listed as metadata-backed below are offline rules.

**Metadata-backed rules** silently skip when no `MetadataProvider` is active. During offline audits
they never produce findings — no errors, no false positives. They only activate when connection flags
are supplied to `deltascope audit`.

### Complete List of Metadata-Backed Rule IDs

| Rule ID |
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

## Trust & Misconfiguration Guardrails (Non-Rule Behaviors)

v0.20.0 introduces additive behaviors that help identify dialect mismatches and unsupported surfaces. These are **not** configurable rules and do not have entries in the policy YAML. They cannot be disabled or re-leveled.

| Behavior | Rule-like ID | Description |
|----------|-------------|-------------|
| PostgreSQL syntax heuristic notice | `dialect.postgresql.syntax.detected.notice` | Emitted as a global advisory finding when MySQL/TiDB path auditing detects common PG-specific syntax tokens (`RETURNING`, `ON CONFLICT`, `::`, `ALTER COLUMN TYPE USING`, `GENERATED AS IDENTITY`). DeltaScope does not auto-switch dialect. |
| PostgreSQL capability-boundary errors | — | Unsupported PG surfaces return typed `PostgreSQLCapabilityBoundaryError` instead of heuristic string matching. |
| Heuristic false-positive exclusion | — | The PostgreSQL syntax heuristic does not fire for tokens inside string literals, double-quoted identifiers, backtick identifiers, line comments, or block comments. |
| Trust context visibility | — | CLI output formats (json, markdown, quiet) include audit context with dialect source and trust notes. `github-actions` and `sarif` formats do not. |
| Rule summary visibility | — | CLI output formats (json, markdown, quiet) include loaded, applicable, and skipped rule counts. `github-actions` and `sarif` formats do not. |

See the [capability matrix](audit-capability-matrix.md) for the authoritative status of each capability.

---

## PostgreSQL DDL Coverage (v0.21.0 / v0.23.0 / v0.24.0)

`v0.21.0` expands PostgreSQL DDL normalization so that common migration follow-up statements are processed through the shared audit pipeline instead of returning capability-boundary errors. `v0.23.0` expands PostgreSQL `CREATE TABLE` coverage for more common constraint shapes. `v0.24.0` deepens the semantic value of those create-table shapes by preserving parser-owned referenced table and referenced column facts through the shared `spec.Constraint` model. None of these releases add new rule IDs. The newly normalized actions and create-table structures reuse existing shared rule families where applicable.

### PostgreSQL Boundary Support-Readiness Gate (v0.32.0)

`v0.32.0` is the **PostgreSQL Boundary Support-Readiness Gate**. It is a decision milestone, not a feature release. No new rule IDs were added. Characterization tests document stable AST facts about generated and identity columns; a readiness report recommends `v0.33.0` as a narrow fact-preservation pack. No production extractor, spec, rule, or policy code changed.

### PostgreSQL ALTER TABLE GENERATED Follow-up Pack (v0.31.0)

`v0.31.0` maps additional PostgreSQL generated/identity `ALTER TABLE` forms to explicit unsupported feature tags, closing the adjacent gap left by `v0.30.0`. These outcomes are **not** rule findings and **no new rule IDs** are involved. They are extractor-level contracts that return `UnsupportedDetail` entries with feature tags and reason strings.

- `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION` → `generated_column`
- `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ...` → `generated_as_identity`
- `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY` → `generated_as_identity`
- Corpus, service, and CLI / HTTP / MCP / `pkg/deltascope` parity lock this contract.
- This is boundary tightening, not generated-column support, identity-column support, or complete PostgreSQL `ALTER TABLE` support.

### PostgreSQL ALTER TABLE GENERATED Boundary Pack (v0.30.0)

`v0.30.0` tightens the PostgreSQL `ALTER TABLE ... ADD COLUMN` unsupported boundary contract for generated stored and identity forms. These outcomes are **not** rule findings and **no new rule IDs** are involved. They are extractor-level contracts that return `UnsupportedDetail` entries with feature tags and reason strings.

- `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS (...) STORED` → `generated_column`
- `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS IDENTITY` → `generated_as_identity`
- Corpus, service, and CLI / HTTP / MCP / `pkg/deltascope` parity lock this contract.
- Adjacent `DROP EXPRESSION`, `SET GENERATED`, and `DROP IDENTITY` forms now receive explicit unsupported mappings in `v0.31.0`.
- This is boundary tightening, not generated-column support, identity-column support, or broad PostgreSQL `ALTER TABLE` support.

### PostgreSQL CREATE TABLE Unsupported Boundaries (v0.26.0)

`v0.26.0` tightens the PostgreSQL `CREATE TABLE` unsupported boundary contract at the extractor level. The following forms are explicitly marked as unsupported — they are **not** rule findings and **no new rule IDs** are involved. They are extractor-level contracts that return `UnsupportedDetail` entries with feature tags and reason strings.

### Schema-Qualified Reference Semantics (v0.27.0)

`v0.27.0` preserves PostgreSQL schema-qualified referenced-object facts in the shared `spec.Constraint` contract via the additive `ReferencedSchema` field. This is **not** a new rule ID — it belongs to extractor/shared semantic facts. Starting with `v0.28.0`, FK forbid finding metadata now exposes `referenced_schema`, `referenced_table`, and `referenced_columns` when the underlying constraint carries those facts.

### Referenced-Object Metadata Surface (v0.28.0)

`v0.28.0` widens the outward `ddl.table.foreign_key.forbid` finding metadata to expose PostgreSQL referenced-object fields (`referenced_schema`, `referenced_table`, `referenced_columns`). These fields were already present in the shared semantic contract from `v0.27.0`; `v0.28.0` makes them visible in CLI JSON, HTTP responses, MCP structured content, and `pkg/deltascope` finding metadata.

- **No new rule IDs** — the `ddl.table.foreign_key.forbid` rule is unchanged; only its finding metadata is wider.
- **Conditional emission** — `referenced_schema` is omitted when no schema qualifier is present; `referenced_table` and `referenced_columns` appear for all FK constraints that carry them.
- **Normalized representation** — `referenced_table` is never concatenated with `referenced_schema`.
- This is **not** schema-aware FK policy support, not a broad PostgreSQL FK implementation, and not a new rule family.

### Schema-Aware FK Policy Pack (v0.29.0)

`v0.29.0` is the first schema-aware FK policy step. DeltaScope now ships the PostgreSQL-only notice rule `ddl.pg.table.foreign_key.cross_schema.advisory` for explicit cross-schema foreign keys.

- **Rule contract** — the rule fires only when the audit dialect is PostgreSQL, the owning table schema is explicit, the referenced schema is explicit, and the two schemas differ.
- **No fire cases** — same-schema foreign keys do not trigger it, and bare references such as `REFERENCES users(id)` do not trigger it because the referenced schema remains unknown.
- **No inference/modeling** — DeltaScope does not infer `public`, and it does not model PostgreSQL `search_path` semantics.
- **Metadata contract** — the finding may include `table_schema`, `referenced_schema`, `referenced_table`, and `referenced_columns`; `referenced_table` remains normalized as `"users"`, never `"auth.users"`.
- **Boundary** — this is the first schema-aware FK policy step, not a broad PostgreSQL FK implementation and not a cross-schema validation workflow.

### PostgreSQL ALTER TABLE FK Fact Support (v0.40.0)

`v0.40.0` extends FK rule coverage to PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` forms. Existing FK rules now produce findings for ALTER TABLE FK additions in addition to the `CREATE TABLE` FK path already covered.

| Rule ID | What It Flags | Covered Path |
|---------|---------------|-------------|
| `ddl.table.foreign_key.forbid` | Foreign key constraints are forbidden under the default policy | `CREATE TABLE` + `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` |
| `ddl.pg.table.foreign_key.cross_schema.advisory` | Cross-schema FK reference (owning and referenced schemas differ) | `CREATE TABLE` + `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` |

- **No new rule IDs** — existing rules now cover ALTER TABLE FK additions through the `DDL.Constraints` projection.
- **Preserved facts** — local columns, referenced table, referenced columns, referenced schema (for schema-qualified references).
- **No live schema FK existence validation** — statement-local facts only.
- **No deferrable/MATCH FULL policy expansion**.
- **No MySQL/TiDB behavior changes**.

| Feature | Extractor Tag |
|---------|---------------|
| Identity columns (`GENERATED ... AS IDENTITY`) | `generated_as_identity` |
| Generated stored columns (`GENERATED ALWAYS AS ... STORED`) | `generated_column` |
| Exclusion constraints (`EXCLUDE USING`) | `exclusion_constraint` |
| Partitioned tables (`PARTITION BY`) | `partitioning` |

Each boundary is backed by corpus cases and surface parity tests across CLI, HTTP, MCP, and `pkg/deltascope`. See the [capability matrix](audit-capability-matrix.md) for the surface contract details.

### Supported PostgreSQL DDL Actions

| Action | Normalized As | Rule Behavior |
|--------|---------------|---------------|
| `ALTER COLUMN ... SET DEFAULT` | `set_default` | Processed as a standard alter action through existing alter semantic rules. |
| `ALTER COLUMN ... DROP DEFAULT` | `drop_default` | Processed as a standard alter action through existing alter semantic rules. |
| `ALTER COLUMN ... SET NOT NULL` | `set_not_null` | Processed as a standard alter action through existing alter semantic rules. |
| `ALTER COLUMN ... DROP NOT NULL` | `drop_not_null` | Processed as a standard alter action through existing alter semantic rules. |
| `VALIDATE CONSTRAINT` | `validate_constraint` | Supported and auditable. No dedicated rule exists; it produces a clean audit unless other findings apply. |
| `DROP CONSTRAINT` | `drop_constraint` | Constraint removal. When the target is a primary key and metadata is available, existing `ddl.alter.drop_primary_key` rules apply. Otherwise, processed as a standard alter action. |
| Table-level named `CHECK` | `create_table` shared facts | Supported and auditable. Existing constraint naming governance can apply when configured. |
| Column-level inline `CHECK` | `create_table` shared facts | Supported and auditable. No dedicated new rule; produces findings only when existing shared semantics apply. |
| Table-level named `UNIQUE` | `create_table` shared facts | Supported and auditable. Existing constraint naming governance can apply when configured. |
| Column-level inline `UNIQUE` | `create_table` shared facts | Supported and auditable. Existing shared index rules can consume the normalized index facts. |
| Table-level named `FOREIGN KEY` | `create_table` shared facts | Supported and auditable. Existing foreign-key naming governance applies only when policy allows foreign keys. `v0.24.0`: preserves `ReferencedTable` and `ReferencedColumns` as parser-owned shared contract facts. |
| Column-level inline `REFERENCES` | `create_table` shared facts | Supported and auditable. Exposed as parser-owned shared facts only; no invented metadata semantics or dedicated new rule. `v0.24.0`: preserves `ReferencedTable` and `ReferencedColumns` as parser-owned shared contract facts. |

### Key Points

- No new rule configuration items are needed. The value of these releases is that existing shared rules and metadata-aware semantics now cover more PostgreSQL DDL actions and create-table shapes with richer semantics.
- `DROP CONSTRAINT` on a primary key (`DROP CONSTRAINT users_pkey`) maps to existing primary-key rules only in metadata-aware mode. In offline mode it passes through as a normal alter action without a dedicated finding.
- `VALIDATE CONSTRAINT` is supported and auditable but does not have a dedicated rule. It produces a clean audit result unless other findings apply to the same statement.
- Inline `REFERENCES` should be read narrowly: DeltaScope now keeps the parser-owned shared relationship facts instead of failing the surface, but this does not imply new metadata-aware foreign-key semantics beyond already existing rule behavior.
- `v0.24.0` deepens `v0.23.0` foreign-key semantics: `ReferencedTable` and `ReferencedColumns` are parser-owned structural facts, not metadata truth. They represent what the SQL statement declares, not what the database schema currently contains.
- The `v0.23.0`/`v0.24.0` create-table work should not be described as full PostgreSQL `CREATE TABLE` support; it is targeted coverage for common, shared-rule-compatible structures with progressively richer semantics.

---

## Cross-References

- **Parameter documentation** — [config.md](config.md)
- **Conceptual overview of rule evaluation** — [../concept/core-concepts.md](../concept/core-concepts.md)
- **Metadata-aware mode** — [../concept/metadata-aware-mode.md](../concept/metadata-aware-mode.md)
- **CLI usage** — [cli.md](cli.md)
- **Capability matrix** — [audit-capability-matrix.md](audit-capability-matrix.md)

Starting with `v0.44.0`, `make release-contract-gates VERSION=vX.Y.Z` verifies version surfaces, binary version output, default policy dialect isolation, and archive integrity as a unified pre-publish gate. No new rule IDs, parser features, or public API contracts were added.

### PostgreSQL Collation Lifecycle (v0.100.0)

v0.100.0 adds lifecycle audit rules for PostgreSQL collation objects. These rules only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Check Description | Default Level | Metadata Required |
|---------|-------------------|:-------------:|:-----------------:|
| `ddl.pg.create_collation.notice` | CREATE COLLATION emits an informational notice | notice | No |
| `ddl.pg.alter_collation.notice` | ALTER COLLATION (rename/owner/schema) emits an informational notice | notice | No |
| `ddl.pg.drop_collation.warn` | DROP COLLATION emits a destructure warning | warning | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection.

### PostgreSQL Extended Statistics Lifecycle (v0.100.0)

v0.100.0 adds lifecycle audit rules for PostgreSQL extended statistics objects. These rules only apply when `--dialect postgresql` is set.

| Rule ID | Check Description | Default Level | Metadata Required |
|---------|-------------------|:-------------:|:-----------------:|
| `ddl.pg.create_statistics.notice` | CREATE STATISTICS emits an informational notice | notice | No |
| `ddl.pg.alter_statistics.notice` | ALTER STATISTICS (rename/owner/schema) emits an informational notice | notice | No |
| `ddl.pg.drop_statistics.warn` | DROP STATISTICS emits a destructure warning | warning | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection.

### PostgreSQL Aggregate/Operator/Conversion Lifecycle (v0.100.0)

v0.100.0 adds lifecycle audit rules for PostgreSQL aggregate, operator, and conversion objects. These rules only apply when `--dialect postgresql` is set.

| Rule ID | Check Description | Default Level | Metadata Required |
|---------|-------------------|:-------------:|:-----------------:|
| `ddl.pg.create_aggregate.notice` | CREATE AGGREGATE emits an informational notice | notice | No |
| `ddl.pg.alter_aggregate.notice` | ALTER AGGREGATE (rename/owner/schema) emits an informational notice | notice | No |
| `ddl.pg.drop_aggregate.warn` | DROP AGGREGATE emits a destructure warning | warning | No |
| `ddl.pg.create_operator.notice` | CREATE OPERATOR emits an informational notice | notice | No |
| `ddl.pg.alter_operator.notice` | ALTER OPERATOR (owner/schema) emits an informational notice | notice | No |
| `ddl.pg.drop_operator.warn` | DROP OPERATOR emits a destructure warning | warning | No |
| `ddl.pg.create_conversion.notice` | CREATE CONVERSION emits an informational notice | notice | No |
| `ddl.pg.alter_conversion.notice` | ALTER CONVERSION (rename/owner/schema) emits an informational notice | notice | No |
| `ddl.pg.drop_conversion.warn` | DROP CONVERSION emits a destructure warning | warning | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. Normalized findings avoid projecting aggregate functions, operator procedures, or conversion function names into output.

### PostgreSQL Operator Family/Class Lifecycle (v0.100.0)

v0.100.0 adds lifecycle audit rules for PostgreSQL operator family and operator class objects. These rules only apply when `--dialect postgresql` is set.

| Rule ID | Check Description | Default Level | Metadata Required |
|---------|-------------------|:-------------:|:-----------------:|
| `ddl.pg.create_operator_family.notice` | CREATE OPERATOR FAMILY emits an informational notice | notice | No |
| `ddl.pg.alter_operator_family.notice` | ALTER OPERATOR FAMILY (rename/owner/schema) emits an informational notice | notice | No |
| `ddl.pg.drop_operator_family.warn` | DROP OPERATOR FAMILY emits a destructure warning | warning | No |
| `ddl.pg.create_operator_class.notice` | CREATE OPERATOR CLASS emits an informational notice | notice | No |
| `ddl.pg.alter_operator_class.notice` | ALTER OPERATOR CLASS (rename/owner/schema) emits an informational notice | notice | No |
| `ddl.pg.drop_operator_class.warn` | DROP OPERATOR CLASS emits a destructure warning | warning | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection.

### PostgreSQL Text Search Object Lifecycle (v0.100.0)

v0.100.0 adds lifecycle audit rules for PostgreSQL text search configuration, dictionary, parser, and template objects. These rules only apply when `--dialect postgresql` is set.

| Rule ID | Check Description | Default Level | Metadata Required |
|---------|-------------------|:-------------:|:-----------------:|
| `ddl.pg.create_text_search_configuration.notice` | CREATE TEXT SEARCH CONFIGURATION emits an informational notice | notice | No |
| `ddl.pg.alter_text_search_configuration.notice` | ALTER TEXT SEARCH CONFIGURATION (rename/owner/schema) emits an informational notice | notice | No |
| `ddl.pg.drop_text_search_configuration.warn` | DROP TEXT SEARCH CONFIGURATION emits a destructure warning | warning | No |
| `ddl.pg.create_text_search_dictionary.notice` | CREATE TEXT SEARCH DICTIONARY emits an informational notice | notice | No |
| `ddl.pg.alter_text_search_dictionary.notice` | ALTER TEXT SEARCH DICTIONARY (rename/owner/schema) emits an informational notice | notice | No |
| `ddl.pg.drop_text_search_dictionary.warn` | DROP TEXT SEARCH DICTIONARY emits a destructure warning | warning | No |
| `ddl.pg.create_text_search_parser.notice` | CREATE TEXT SEARCH PARSER emits an informational notice | notice | No |
| `ddl.pg.alter_text_search_parser.notice` | ALTER TEXT SEARCH PARSER (rename/schema) emits an informational notice | notice | No |
| `ddl.pg.drop_text_search_parser.warn` | DROP TEXT SEARCH PARSER emits a destructure warning | warning | No |
| `ddl.pg.create_text_search_template.notice` | CREATE TEXT SEARCH TEMPLATE emits an informational notice | notice | No |
| `ddl.pg.alter_text_search_template.notice` | ALTER TEXT SEARCH TEMPLATE (rename/schema) emits an informational notice | notice | No |
| `ddl.pg.drop_text_search_template.warn` | DROP TEXT SEARCH TEMPLATE emits a destructure warning | warning | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. Normalized findings avoid projecting text search function names (start/end/lextype/lexize) into output.

### PostgreSQL Boundary Closure (v0.100.0)

v0.100.0 adds lifecycle audit rules for selected PostgreSQL boundary objects: DROP TRANSFORM, DROP ACCESS METHOD, and ALTER LARGE OBJECT owner changes. These rules only apply when `--dialect postgresql` is set.

| Rule ID | Check Description | Default Level | Metadata Required |
|---------|-------------------|:-------------:|:-----------------:|
| `ddl.pg.create_transform.notice` | CREATE TRANSFORM emits an informational notice | notice | No |
| `ddl.pg.create_access_method.notice` | CREATE ACCESS METHOD emits an informational notice | notice | No |
| `ddl.pg.drop_transform.warn` | DROP TRANSFORM emits a destructure warning | warning | No |
| `ddl.pg.drop_access_method.warn` | DROP ACCESS METHOD emits a destructure warning | warning | No |
| `ddl.pg.alter_large_object.owner.notice` | ALTER LARGE OBJECT ... OWNER TO emits an informational notice | notice | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection.
>
> **Deferred boundary cases:** CREATE TRANSFORM and CREATE ACCESS METHOD are intentionally not covered because their handler/function names are the object identity, making safe normalization incompatible with payload safety constraints.

### PostgreSQL Metadata-Aware Object Validation (v0.90.0)

v0.90.0 adds metadata-aware object validation for selected PostgreSQL lifecycle rule findings. When a PostgreSQL metadata connection is configured, DeltaScope resolves non-table objects through `pg_catalog` queries and enriches lifecycle findings with object existence and safe attribute information. **No new rule IDs were added.** Existing lifecycle rule findings are enriched with metadata fields when metadata is available.

#### Metadata Projection Fields

When object metadata is resolved, the following fields appear on the finding's `metadata` object:

| Field | Type | Description |
|-------|------|-------------|
| `metadata_status` | string | `confirmed`, `not_found`, or `unavailable` |
| `metadata_exists` | boolean | Whether the object exists in the database |
| `metadata_object_type` | string | Resolved object type (e.g. `domain`, `type`, `extension`) |
| `metadata_object_name` | string | Resolved object name |
| `metadata_schema` | string | Schema containing the object (when ambiguous) |

#### Safe Projectable Attributes

Only the following attribute keys are projected from the object snapshot into findings:

`type_kind`, `extension_version`, `enabled`, `server`, `foreign_data_wrapper`, `target_type`, `has_options`, `table`

These appear as `metadata_<key>` on the finding (e.g. `metadata_type_kind`, `metadata_extension_version`).

#### Blocked Attributes

The following attribute categories are **never** projected into findings, even when present in the object snapshot: password, secret, token, api_key, connection, dsn, connstr, body, definition, comment, label, query, action_sql, options.

#### Supported Lifecycle Rules

Object metadata enrichment applies to these PostgreSQL lifecycle rules:

`ddl.pg.drop_schema.advisory`, `ddl.pg.drop_type.advisory`, `ddl.pg.drop_domain.advisory`, `ddl.pg.drop_extension.advisory`, `ddl.pg.drop_sequence.advisory`, `ddl.pg.drop_materialized_view.advisory`, `ddl.pg.drop_publication.warn`, `ddl.pg.drop_foreign_server.warn`, `ddl.pg.drop_user_mapping.warn`, `ddl.pg.comment_on.notice`

Without a metadata connection, these rules produce findings as before — no metadata fields appear.

---

### PostgreSQL Generated/Identity Rule Coverage (v0.36.0)

v0.36.0 is the **PostgreSQL Generated/Identity Rule Coverage Pack**. Three new PostgreSQL-only forbid rules cover the generated/identity state-transition forms that became supported in v0.35.0. These are forbid alter-action rules using the existing `newForbiddenAlterActionRule` constructor with a PostgreSQL-only dialect allowlist.

| Rule ID | Action | Covered Form | Dialect |
|---------|--------|-------------|---------|
| `ddl.alter.drop_expression.forbid` | `drop_expression` | `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION` | PostgreSQL only |
| `ddl.alter.set_generated.forbid` | `set_generated` | `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ...` | PostgreSQL only |
| `ddl.alter.drop_identity.forbid` | `drop_identity` | `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY` | PostgreSQL only |

These rules produce explicit `rule_id` findings across CLI, HTTP, MCP, and `pkg/deltascope` surfaces. This is rule coverage — not parser support widening, not spec contract widening, not generated expression evaluation, not complete PostgreSQL sequence semantics. No MySQL/TiDB behavior changes.

### PostgreSQL Generated/Identity State-Transition Support (v0.35.0)

v0.35.0 is the **PostgreSQL Generated/Identity State-Transition Pack**. State-transition forms for PostgreSQL generated and identity columns are now supported through the normal audit path. It is extractor-level support widening — not a rule behavior change. No new rule IDs were added. Existing rules apply to these newly supported forms the same way they apply to other PostgreSQL DDL statements.

- Supported forms: `DROP EXPRESSION`, `SET GENERATED ALWAYS`, `SET GENERATED BY DEFAULT`, `DROP IDENTITY`.
- Normalized contract: `drop_expression`, `set_generated` with `generated_when` (`"a"` / `"d"`), `drop_identity`.
- This is not full generated-column lifecycle support, not generated expression evaluation, not complete PostgreSQL sequence semantics.
- No new rule IDs, CLI flags, or rule behavior changes.

### PostgreSQL Generated/Identity Narrow Support (v0.34.0)

v0.34.0 added narrow generated/identity definition form support. See [audit-capability-matrix.md](audit-capability-matrix.md) for the full supported forms table. Preserved facts: `generated_when`, `is_identity`, `identity_options` (from v0.33.0) continue flowing.

## v0.33.0 Note: Shared Contract & Unsupported Metadata

v0.33.0 does not add new rules or change rule behavior. It introduces shared contract fields (`GeneratedWhen`, `IsIdentity`, `IdentityOptions` on `spec.Column`) and unsupported metadata (`Metadata` on `spec.UnsupportedDetail`) for PostgreSQL generated/identity outcomes. These are additive contract changes visible to rule consumers, not rule trigger or level changes.

Unsupported feature names remain: `generated_column`, `generated_as_identity`. The metadata keys surfaced on these unsupported outcomes are `column`, `generated_when`, `is_identity` (identity cases), and `identity_options` (options cases).
