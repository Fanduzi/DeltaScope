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
| No blockers; at least one `warning` or `notice` | `review` |
| No findings at all | `pass` |

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

```
RULE ID                                   KIND  LEVEL    METADATA
dml.where.require                         dml   blocker  false
dml.limit.forbid                          dml   warning  false
ddl.table.comment.require                 ddl   warning  false
ddl.table.row_size.max_bytes.require      ddl   blocker  true
...
```

### deltascope rules show

Display the full detail for a single rule:

```bash
deltascope rules show dml.where.require
```

Example output:

```
Rule ID:     dml.where.require
Kind:        dml
Level:       blocker
Description: UPDATE or DELETE must include a WHERE clause to prevent full-table modifications
Metadata:    false
Params:
  required (bool, default: true)
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
| `ddl.table.audit_columns.require` | Table must include audit timestamp columns | warning | No |
| `ddl.table.foreign_key.forbid` | FOREIGN KEY constraints are forbidden | blocker | No |
| `ddl.table.partition.forbid` | Partitioned tables are forbidden | blocker | No |
| `ddl.table.create_like.forbid` | CREATE TABLE … LIKE is forbidden | blocker | No |
| `ddl.table.create_as.forbid` | CREATE TABLE … AS SELECT is forbidden | blocker | No |
| `ddl.table.row_size.max_bytes.require` | Estimated row size must not exceed InnoDB limits | blocker | **Yes** |
| `ddl.table.denylist.forbid` | DDL on schema/table entries in the denylist is forbidden | blocker | No |

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

## DDL: Global Rules (2 rules)

Global rules evaluate across **all statements in a batch** after all statement-scoped rules have
completed. They cannot fire on a single statement in isolation.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.alter.merge.mysql.require` | Multiple ALTER TABLE on the same table should be merged (MySQL) | warning | No |
| `ddl.alter.merge.tidb.require` | Multiple ALTER TABLE on the same table guidance (TiDB, disabled by default) | warning | No |

> **Note:** `ddl.alter.merge.mysql.require` fires when two or more `ALTER TABLE` statements in the same
> input target the same table. In MySQL, each `ALTER TABLE` causes a table rebuild; merging them into a
> single statement dramatically reduces downtime. In TiDB, multiple alters are generally lighter-weight,
> so `ddl.alter.merge.tidb.require` is disabled in the default policy.

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
| `dml.table.denylist.forbid` | DML on schema/table entries in the denylist is forbidden | blocker | No |

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

---

## Cross-References

- **Parameter documentation** — [config.md](config.md)
- **Conceptual overview of rule evaluation** — [../concept/core-concepts.md](../concept/core-concepts.md)
- **Metadata-aware mode** — [../concept/metadata-aware-mode.md](../concept/metadata-aware-mode.md)
- **CLI usage** — [cli.md](cli.md)
- **Capability matrix** — [audit-capability-matrix.md](../audit-capability-matrix.md)
