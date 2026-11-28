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

DeltaScope provides two read-only metadata lookup commands for rule discovery. These commands inspect the shipped **Rule Catalog** — they do not execute audits, parse SQL, or call the audit service.

Catalog, Default Policy, Loaded, and suppressions are different counts:

- `deltascope rules list` reports the Rule Catalog, including default-disabled `dml.impact.*` rules.
- Default Policy is the enabled-rule set with no caller config. It does not enable `dml.impact.*`.
- Audit `rule_summary.loaded` is the registered statement-rule set for that audit. Do not treat Catalog count as Loaded count.
- The three `ddl.constraint.foreign_key.name.*` rules stay in Catalog and Default Policy. While `ddl.table.foreign_key.forbid` is enabled they are not Loaded; `config status` reports reason `fk_forbid` so they read as suppressed, not missing.

The existing `level` field (with values `blocker`, `warning`, `notice`) is DeltaScope's severity-like field. It is called `level` in the finding JSON, config YAML, and catalog output. There is no separate `severity` field.

### deltascope rules list

List rules from the shipped catalog with optional filters:

```bash
# All rules
deltascope rules list

# Filter by level
deltascope rules list --level blocker
deltascope rules list --level warning
deltascope rules list --level notice

# Filter by dialect and level, JSON output
deltascope rules list --dialect postgresql --level warning --format json

# Filter by kind
deltascope rules list --kind ddl
deltascope rules list --kind dml

# Search by keyword
deltascope rules list --search drop_column

# Filter by category
deltascope rules list --category alter_table

# Limit results
deltascope rules list --level blocker --limit 5
```

Filters:

| Flag | Values | Description |
|------|--------|-------------|
| `--dialect` | `mysql`, `tidb`, `postgresql`, `common` | Filter by dialect scope |
| `--level` | `blocker`, `warning`, `notice` | Filter by default level |
| `--kind` | `ddl`, `dml` | Filter by SQL statement kind |
| `--category` | (free text) | Case-insensitive category/family substring |
| `--search` | (free text) | Case-insensitive search across rule ID, summary, tags, config key |
| `--format` | `text`, `json` | Output format (default: `text`) |
| `--limit` | integer | Limit result count; `0` means no limit |

All filters are optional. Multiple filters combine as AND conditions. Empty results return success with zero rules.

Example text output:

```text
RULE ID                                        LEVEL    DIALECT  KIND  CATEGORY
---------------------------------------------  -------  -------  ----  -----------
ddl.alter.drop_column.forbid                   warning  common   ddl   alter_table
ddl.alter.drop_column.notice                   notice   common   ddl   alter_table
ddl.pg.alter.drop_column.advisory              warning  postgresql  ddl   alter_table
3 rules
```

Example JSON output:

```bash
deltascope rules list --dialect postgresql --level warning --format json
```

```json
{
  "version": "v0.290.0",
  "summary": {
    "total": 62,
    "returned": 62,
    "filters": {
      "dialect": "postgresql",
      "level": "warning"
    }
  },
  "rules": [
    {
      "rule_id": "ddl.pg.alter.add_check.not_valid.require",
      "level": "warning",
      "dialect": "postgresql",
      "kind": "ddl",
      "category": "alter_table",
      "summary": "Require DDL pg alter add check not valid require",
      "enabled": true,
      "tags": ["ddl", "postgresql", "alter_table", "require"]
    }
  ]
}
```

Invalid enum values produce a clear validation error and exit code 2:

```text
invalid level "severe": must be one of blocker, warning, notice
```

### deltascope rules explain

Display detailed information about a single rule by exact rule ID:

```bash
deltascope rules explain dml.where.require
deltascope rules explain dml.where.require --format json
```

`rules explain` does not run an audit and does not parse SQL. It returns static rule metadata from the shipped catalog.

Example text output:

```text
Rule ID:    dml.where.require
Level:      blocker
Enabled:    true
Dialects:   common
Kind:       dml
Category:   dml_safety
Config Key: dml.where.require

Summary:
  Require DML where require

Why:
  The statement is missing a clause, option, or object that the shipped policy requires.

Risk:
  Ignoring this rule can allow high-impact data changes to proceed with less safety review.

Suggestion:
  Add the required clause, option, or object explicitly so the rule no longer has to infer intent.

Tags: dml, common, dml_safety, require
Trigger Example:
  DELETE FROM users;
Valid Example:
  DELETE FROM users WHERE id = 1;

Default Params:
  required: true

Default policy:
  rules:
    dml.where.require:
      enabled: true
      level: blocker
      params:
        required: true

Safe override example:
  rules:
    dml.where.require:
      enabled: true
      level: warning
      params:
        required: true

Inspect effective rule status:
  deltascope config status dml.where.require --config deltascope.yaml
```

Example JSON output:

```bash
deltascope rules explain dml.where.require --format json
```

```json
{
  "version": "v0.290.0",
  "rule": {
    "rule_id": "dml.where.require",
    "level": "blocker",
    "enabled": true,
    "dialects": ["common"],
    "kind": "dml",
    "category": "dml_safety",
    "summary": "Require DML where require",
    "why": "The statement is missing a clause, option, or object that the shipped policy requires.",
    "risk": "Ignoring this rule can allow high-impact data changes to proceed with less safety review.",
    "suggestion": "Add the required clause, option, or object explicitly so the rule no longer has to infer intent.",
    "config_key": "dml.where.require",
    "tags": ["dml", "common", "dml_safety", "require"],
    "description": "Require DML where require. Default level is blocker, enabled=true, scope=dml, and the shipped policy treats it as a offline-safe rule.",
    "statement_kinds": ["dml"],
    "default_params": { "required": true },
    "metadata_aware": false,
    "trigger_example": "DELETE FROM users;",
    "valid_example": "DELETE FROM users WHERE id = 1;",
    "config_example": "rules:\n  dml.where.require:\n    enabled: true\n    level: blocker\n    params:\n      required: true",
    "remediation": "Add the required clause, option, or object explicitly so the rule no longer has to infer intent.",
    "config_hints": [
      "rules.dml.where.require.enabled",
      "rules.dml.where.require.level",
      "rules.dml.where.require.params.required"
    ]
  }
}
```

Unknown rule IDs produce a clear error and exit code 2:

```text
rule "nonexistent_rule" not found
```

### Config Key Relationship

For most rules, `config_key` equals `rule_id`. To configure a rule, use its `config_key` under the `rules` section in your policy YAML:

```yaml
rules:
  dml.where.require:
    enabled: true
    level: blocker
    params:
      required: true
```

### Non-Goals

Rule discovery commands do not:

- Audit SQL statements
- Add new audit rules or change rule behavior
- Change the finding JSON shape
- Introduce a `severity` field
- Claim to return every possible database risk
- Provide SDK, HTTP, or MCP surfaces

---

## DDL: Create Table Rules

### Table-Level Rules (23 rules)

These rules evaluate properties of the `CREATE TABLE` statement as a whole.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.table.comment.require` | Table must have a non-empty COMMENT | warning | No |
| `ddl.table.comment.max_length` | Table COMMENT must not exceed the Unicode code-point limit | warning | No |
| `ddl.table.name.max_length` | Table name length limit | blocker | No |
| `ddl.table.name.pattern.require` | Table name must match pattern (default: alphanumeric + underscore) | blocker | No |
| `ddl.table.name.keyword.forbid` | Table name must not be a reserved SQL keyword | blocker | No |
| `ddl.table.engine.allowlist` | Storage engine must be in allowlist (default: InnoDB) | blocker | No |
| `ddl.table.charset.allowlist` | Table default charset must be in allowlist | blocker | No |
| `ddl.table.collation.allowlist` | Table collation must be in allowlist (default: disabled; default list: utf8mb4_bin) | blocker | No |
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

**Default policy dialect isolation (v0.43.0):** Starting with v0.43.0, MySQL-family rules are automatically skipped when `--dialect postgresql` is set. This includes `ddl.table.primary_key.unsigned.require`, `ddl.table.primary_key.auto_increment.require`, `ddl.table.primary_key.not_null.require`, `ddl.table.engine.allowlist`, `ddl.table.charset.allowlist`, `ddl.table.collation.allowlist`, `ddl.table.row_format.allowlist`, `ddl.table.auto_increment.init_value.require`, `ddl.table.partition.forbid`, `ddl.table.create_as.forbid`, `ddl.table.create_like.forbid`, `ddl.column.charset.allowlist`, `ddl.column.collation.allowlist`, `ddl.column.charset_collation.match.require`, `ddl.alter.change_column.forbid`, `ddl.alter.modify_column.forbid`, and the `ON UPDATE CURRENT_TIMESTAMP` suggestion in the audit-columns check. Conversely, MySQL/TiDB audits exclude all `ddl.pg.*` and PostgreSQL-only dialect-gated rules. Isolation is enforced at the rule `AppliesTo` gate level.
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

### View Rules

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.view.create.forbid` | Forbid CREATE/REPLACE and MySQL ALTER VIEW definitions | blocker | No |
| `ddl.view.drop.forbid` | Forbid DROP VIEW | blocker | No |
| `ddl.view.target.valid` | Check view target existence and kind | blocker | Ordered facts |
| `ddl.view.source.valid` | Check dependencies, bindings, and output names | blocker | Ordered facts |
| `ddl.view.security.definer.warn` | Review effective DEFINER security or an explicitly named definer | warning | Ordered facts for inheritance |
| `ddl.view.algorithm.warn` | Review requested TEMPTABLE or known MERGE fallback (MySQL) | warning | No |
| `ddl.view.check_option.valid` | Reject CHECK OPTION on a known non-updatable view (MySQL) | blocker | Ordered facts when needed |
| `ddl.view.options.ignored.warn` | Review each explicitly declared option without a TiDB execution guarantee | warning | No |

The six semantic rules have no parameters. Target, source and security apply to
MySQL and TiDB; algorithm and check-option checks apply to MySQL, and
ignored-option review to TiDB. TiDB ALTER VIEW and bare CHECK OPTION remain
vendor boundaries. Unknown required facts yield evidence gaps, not invented
findings; disabling rules or changing levels does not change ordered state.
The security warning does not prove account existence or authorization.

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

### Type Compatibility Rules (12 rules)

These rules check that column type and attribute changes made via `MODIFY COLUMN` or `CHANGE COLUMN` are safe and compatible. Most require live metadata to compare the target against the current column definition. The unknown-prior-state advisory is the bounded offline fallback for explicit MODIFY nullability clauses.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.alter.modify_column.target_type_family.allowlist` | MODIFY COLUMN target type family must be in allowlist | blocker | No |
| `ddl.alter.change_column.target_type_family.allowlist` | CHANGE COLUMN target type family must be in allowlist | blocker | No |
| `ddl.alter.modify_column.compatibility.require` | MODIFY COLUMN must be compatible with current column type | blocker | **Yes** |
| `ddl.alter.change_column.compatibility.require` | CHANGE COLUMN must be compatible with current column type | blocker | **Yes** |
| `ddl.alter.modify_column.explicit_nullability_change.forbid` | MODIFY COLUMN must not explicitly change nullability | blocker | **Yes** |
| `ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory` | Explicit MODIFY NULL/NOT NULL with unknown prior state emits a non-blocking advisory | notice | No |
| `ddl.alter.change_column.explicit_nullability_change.forbid` | CHANGE COLUMN must not explicitly change nullability | blocker | **Yes** |
| `ddl.alter.modify_column.explicit_default_change.forbid` | MODIFY COLUMN must not explicitly change DEFAULT value | blocker | **Yes** |
| `ddl.alter.change_column.explicit_default_change.forbid` | CHANGE COLUMN must not explicitly change DEFAULT value | blocker | **Yes** |
| `ddl.alter.modify_column.explicit_auto_increment_change.forbid` | MODIFY COLUMN must not add or remove AUTO_INCREMENT | blocker | **Yes** |
| `ddl.alter.change_column.explicit_auto_increment_change.forbid` | CHANGE COLUMN must not add or remove AUTO_INCREMENT | blocker | **Yes** |
| `ddl.alter.table_option.compatibility.require` | Table option changes must be compatible with current table options | warning | **Yes** |

### Existence and Column-Identity Rules (20 rules — metadata-backed)

These rules verify that the objects referenced by a DDL statement actually exist (or do not
already exist) in the request-local ordered schema view: a live table snapshot when metadata
is configured, or a state derived from earlier statements in the same audit batch for MySQL
and TiDB. On MySQL/TiDB the table-existence rules (`exists.alter`, `exists.create`), every
column/index member-existence rule (`add`/`drop`/`modify`/`change`/`rename` column,
`add`/`drop`/`rename` index, the `set_default`/`drop_default` column targets, and
`AFTER`-relative column-position references), the CHANGE and RENAME destination-name rules, `ddl.alter.drop_primary_key.exists.require`,
`ddl.create_index.columns.exists.require`, and `ddl.table.drop.exists.require` emit
`unknown_table_state` evidence gaps when
that state cannot be established; the remaining rules are still silently skipped without
usable state. PostgreSQL uses its own evidence-gap reasons for the checks described below.

On MySQL and TiDB, `ddl.alter.change_column.target.exists.forbid` and `ddl.alter.rename_column.target.exists.forbid` report one blocker when the new name belongs to another column. The same old and new identity is not a self-conflict. An unknown column set is the existing `unknown_table_state` gap, not a claim that the new name is free. Closing either rule does not let ordered state publish a known duplicate name.

On observed PostgreSQL 17, the existing ADD and RENAME column rules use an exact,
fresh catalog of ordinary persistent, non-typed tables. ADD rejects an existing user name
unless `IF NOT EXISTS` is authored. A proven guarded duplicate suppresses the
three ADD execution advisories while independent declaration rules still run.
An observed system column conflicts even with the guard. RENAME checks the
source first, rejecting an absent or system source. An occupied destination,
including the same exact name, conflicts. Quotes preserve case and spaces.
Unproven scope, stale observations and multiple actions yield
`unproven_column_identity` gaps. The independent RENAME forbid still applies.

For PostgreSQL `ALTER TABLE IF EXISTS`, `ddl.table.exists.alter.require` is
silent on a proven qualified absence or an observed relation of any kind.
A schema-local miss for an unqualified name or stale relation evidence yields
`unproven_alter_table_relation`. Three-part names also require a database binding
that the current schema-local observation does not supply. Existing relation kinds can still fail the
action's independent prerequisites. The scope excludes type conversion,
expression evaluation, dependencies and permissions.

`ddl.alter.rename_column.version.require` checks `RENAME COLUMN` against the resolved version. MySQL 8.0.3+ inside 8.0, MySQL 8.4, and TiDB 8.5 pass. MySQL 5.7 and MySQL 8.0.0–8.0.2 are one incompatibility blocker: coverage stays `complete` and the verdict is `reject`. A missing version is a `missing_target_version` gap with no extra finding. A legal version outside the validated series is a `target_version_out_of_validated_range` gap and is not published. `CHANGE COLUMN` does not inherit this check. The golden profile `t05-a5-column-identity-isolated` enables the previous eight ordered-state blockers plus these three rules and disables every other catalog rule. It does not change the default meaning of existing rules.

On MySQL and TiDB, one ordinary single-action `DROP COLUMN` removes that column for later statements in the same request when the table is known-present, the column set is known, at least one column remains, and every loaded primary key, index, constraint, and sibling expression can be proved not to use it. A nil or empty index or constraint collection with its Unknown flag false is a known empty set. Unrelated members stay, including `PRIMARY(id)`. The same statistic counters cleared by an ordinary `MODIFY` are cleared here. A later same-name `ADD` uses the new definition. An ordinary index that names the column, an unknown member collection, the last column, `IF EXISTS`, or any other action in the statement tombstones the named table identities and already-loaded dependents. This subset does not delete or shrink indexes and does not rewrite constraints. PostgreSQL keeps the previous per-statement snapshot. The golden profile `t05-a6-drop-column-isolated` enables six existing blockers and disables every other catalog rule. It does not change their default meaning. The CLI `--schema` flag still starts metadata mode, so an offline CLI invocation has an empty request schema.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.table.exists.alter.require` | ALTER TABLE target must exist | blocker | **Yes** |
| `ddl.table.exists.create.forbid` | CREATE TABLE target must not already exist | blocker | **Yes** |
| `ddl.alter.add_column.exists.forbid` | ADD COLUMN must not already exist or repeat a name in an ADD-only MySQL or TiDB ALTER | blocker | **Yes** |
| `ddl.alter.drop_column.exists.require` | Checks DROP COLUMN target existence and direct PostgreSQL RESTRICT dependencies | blocker | **Yes** |
| `ddl.alter.modify_column.exists.require` | MODIFY COLUMN target must exist | blocker | **Yes** |
| `ddl.alter.change_column.exists.require` | CHANGE COLUMN source must exist | blocker | **Yes** |
| `ddl.alter.change_column.target.exists.forbid` | CHANGE COLUMN new name must not already belong to another column | blocker | **Yes** |
| `ddl.alter.rename_column.exists.require` | RENAME COLUMN source must exist | blocker | **Yes** |
| `ddl.alter.rename_column.target.exists.forbid` | RENAME COLUMN new name must not already belong to another column | blocker | **Yes** |
| `ddl.alter.rename_column.version.require` | RENAME COLUMN requires MySQL 8.0.3+ in 8.0, MySQL 8.4, or TiDB 8.5; a known older MySQL version is one blocker | blocker | **Yes** |
| `ddl.alter.set_default.exists.require` | ALTER COLUMN SET DEFAULT target must exist | blocker | **Yes** |
| `ddl.alter.drop_default.exists.require` | ALTER COLUMN DROP DEFAULT target must exist | blocker | **Yes** |
| `ddl.alter.column_position.exists.require` | The column an AFTER position clause references must exist (MySQL evaluates same-statement sibling columns from the accumulated working state; TiDB uses the pre-statement state) | blocker | **Yes** |
| `ddl.alter.add_index.exists.forbid` | ADD INDEX name must not already exist | blocker | **Yes** |
| `ddl.alter.drop_index.exists.require` | DROP INDEX target must exist | blocker | **Yes** |
| `ddl.alter.rename_index.exists.require` | RENAME INDEX source must exist | blocker | **Yes** |
| `ddl.alter.drop_primary_key.exists.require` | DROP PRIMARY KEY requires a primary key to exist | blocker | **Yes** |
| `ddl.create_index.columns.exists.require` | Standalone `CREATE INDEX` column references must exist on the target table | blocker | **Yes** |
| `ddl.alter.alter_check.exists.require` | `ALTER CHECK`/`ALTER CONSTRAINT` target must be a proven CHECK member | blocker | **Yes** |
| `ddl.alter.drop_check.exists.require` | `DROP CHECK`/`DROP CONSTRAINT` target must be a proven CHECK member | blocker | **Yes** |

### Advanced Column Rules (2 rules)

These rules cover the MySQL/TiDB advanced column family — generated columns, expression defaults (`DEFAULT (<expr>)` and `ALTER ... ALTER COLUMN ... SET/DROP DEFAULT`), and column visibility — through shared, evaluation-free expression facts collected from the parsed AST and the MySQL catalog.

`ddl.alter.column.dependency.require` blocks a `DROP COLUMN`, `RENAME COLUMN`, or identity-changing `CHANGE COLUMN` whose source column is referenced by a sibling generated or expression-default definition, reporting one blocker per dependent in declaration order. A same-name `MODIFY COLUMN` keeps the column identity and is outside this rule's scope. A withheld column set or an unprovable dependency graph yields `unknown_table_state`/`incomplete_alter_dependency_effects` evidence gaps instead of a silent pass, and ordered state never publishes a provably rejected transition as a reusable success shape.

`ddl.column.advanced.compatibility.require` applies the frozen product/version responsibility table: generated columns require MySQL 5.7.6+ or TiDB 8.5, expression defaults require MySQL 8.0.13+, and visibility requires MySQL 8.0.23+. The frozen TiDB default distinctions are explicit: `JSON_OBJECT`-family defaults on `CREATE` and `uuid()` on `ALTER ... SET DEFAULT` against `VARCHAR(36)` are observed positives; JSON expression defaults through `SET DEFAULT` and arithmetic/local-column defaults are observed negatives. Unobserved families — including standard functions absent from the observed sets — are reported as bounded evidence gaps, never as known database rejections. Subqueries, aggregates, nondeterministic generated functions, and storage-mode conversions are determined negatives. Declaration-level checks run offline; only checks that compare against the prior column consult the request-local ordered schema view and report `incomplete_source_column`/`target_version_out_of_validated_range` gaps rather than fabricating facts. The optional `features` list param narrows enforcement (`generated`, `expression_default`, `visibility`; omitted enables all three, `[]` leaves the rule loaded but inapplicable, and unknown names are rejected at load).

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.alter.column.dependency.require` | Column drop/rename/identity-change is blocked while a sibling generated or expression-default column references it (same-name MODIFY is out of scope) | blocker | **Yes** |
| `ddl.column.advanced.compatibility.require` | Generated, expression-default, and visibility declarations and transitions must satisfy the target product/version contract | blocker | **Yes** |

### CHECK Constraint Lifecycle Rules (5 rules — #91)

These rules cover the MySQL/TiDB CHECK constraint lifecycle — declarations on `CREATE TABLE`, column-level `CHECK` options, `ALTER TABLE ... ADD CONSTRAINT ... CHECK`, `DROP CHECK`/`DROP CONSTRAINT`, and `ALTER CHECK`/`ALTER CONSTRAINT ... [NOT] ENFORCED` — through a shared arm resolver that first decides whether the target actually stores and enforces CHECK constraints. `ALTER CONSTRAINT` is normalized as an alias of `ALTER CHECK`, and `DROP CHECK`/`DROP CONSTRAINT` share one target binding. Raw CHECK expression text is never retained or emitted; only normalized structure/reference facts and the constraint name participate.

`ddl.constraint.check.not_enforced.notice` fires once per declared or flipped `NOT ENFORCED` CHECK on an arm that stores and enforces constraints (MySQL 8.0.16+; TiDB with `tidb_enable_check_constraint=ON`). It does not fire for `ENFORCED`, ordinary column changes, or missing/non-CHECK flip targets — a proven-missing target instead takes the existence blocker, and an arm that cannot be resolved (missing version, missing TiDB flag) yields an attributable evidence gap instead of the finding.

`ddl.constraint.check.ignored.notice` exposes accepted-but-ignored declarations: MySQL below 8.0.16 parses a bare CHECK declaration but neither stores nor enforces it, and TiDB parses but silently ignores CHECK while `tidb_enable_check_constraint=OFF`. The notice fires instead of letting the declaration read as effective enforcement — but only for genuinely accepted declarations: an authored `ENFORCED`/`NOT ENFORCED` clause on MySQL's ignored arm is a proven 1064 syntax refusal (5.7 probes), handled by the version rule rather than this notice. An unresolved arm produces the bounded gap, never the notice.

`ddl.alter.alter_check.exists.require` and `ddl.alter.drop_check.exists.require` bind `ALTER CHECK`/`ALTER CONSTRAINT` and `DROP CHECK`/`DROP CONSTRAINT` targets through the shared authored-spelling classifier — the `CHECK` keyword binds only CHECK members while the generic `CONSTRAINT` keyword binds a uniquely-named check/unique/PK/FK member — so a proven-missing, proven non-CHECK (secondary-index), or cross-type same-name target is one blocker (native 3821/3940/3941/3939-class refusal); an unvouched namespace is a gap, not a guess. `ddl.alter.alter_check.version.require` reports the spelling-aware MySQL syntax refusal as one blocker — `CHECK` spelling executes from 8.0.16, the generic `CONSTRAINT` alias from 8.0.19 (probed 8.0.17), and the 5.7 family refuses either spelling; on TiDB and supported MySQL versions it stays silent, and a missing version or unrecoverable spelling stays a gap without a false refusal claim.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.constraint.check.not_enforced.notice` | Declared or flipped `NOT ENFORCED` CHECK on a storing/enforcing arm is a review notice | notice | **Yes** |
| `ddl.constraint.check.ignored.notice` | CHECK declaration on an accept-but-ignore arm (MySQL <8.0.16, TiDB flag OFF) is disclosed, not passed as enforced | notice | **Yes** |
| `ddl.alter.alter_check.exists.require` | `ALTER CHECK`/`ALTER CONSTRAINT` target must be a proven CHECK member | blocker | **Yes** |
| `ddl.alter.drop_check.exists.require` | `DROP CHECK`/`DROP CONSTRAINT` target must be a proven CHECK member | blocker | **Yes** |
| `ddl.alter.alter_check.version.require` | `ALTER CHECK`/`DROP CHECK` spelling requires MySQL 8.0.16+, the generic `CONSTRAINT` alias 8.0.19+ (5.7 family refuses both) | blocker | **Yes** |

> **Note:** Ordered schema state carries each CHECK member's normalized enforcement (`enforced`/`not_enforced`) across `ALTER CHECK`/`ALTER CONSTRAINT` transitions, so `ADD CHECK` → `ALTER`/`DROP` sequences stay bound within one request. On MySQL the metadata provider reads members from `TABLE_CONSTRAINTS` (`ENFORCED` when the column exists) joined to `CHECK_CONSTRAINTS` on catalog/schema/name; on TiDB members come from `TIDB_CHECK_CONSTRAINTS` — enforcement state is not observable through the catalog there (`SHOW CREATE TABLE` is the only surface), so ordered state tracks declared transitions instead of fabricating a catalog enforcement fact. A `CHANGE COLUMN` on a referenced column drops the CHECK silently on TiDB (MySQL refuses), matching probed native behavior.

---

## DDL: Object Lifecycle Rules (8 rules)

These rules govern `DROP TABLE` and `TRUNCATE TABLE` operations.

The PostgreSQL branch of `ddl.table.truncate.inbound_fk` checks for foreign keys from outside the exact RESTRICT set, including attached descendants and inherited partition foreign-key references. It treats an unvalidated inbound foreign key as a dependency. CASCADE traversal, sequence effects, foreign data wrappers, permissions and locks require further evidence.

On MySQL/TiDB, the enabled DROP existence rule emits one `unknown_table_state` gap requiring `target_table.existence` when the target state is unknown. A confirmed-absent target still produces the original blocker, including with `IF EXISTS`: this is strict policy, not a prediction of database execution failure. Conditional single-table DROP preserves absence for a following CREATE; multi-target forms remain conservative. PostgreSQL and TRUNCATE gap behavior is unchanged.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.table.drop.forbid` | DROP TABLE is forbidden | blocker | No |
| `ddl.table.drop.exists.require` | Checks DROP TABLE target existence and PostgreSQL relation kind. PostgreSQL IF EXISTS guards an absent member. An exact PostgreSQL 17 ordinary-table set under RESTRICT checks direct foreign-key, view and row-type column dependencies. | blocker | **Yes** |
| `ddl.table.drop.adaptive_hash.warn` | DROP TABLE warns when adaptive hash index is active | warning | **Yes** |
| `ddl.table.drop.rows.max_count` | DROP TABLE warns when table has too many rows | warning | **Yes** |
| `ddl.table.truncate.forbid` | TRUNCATE TABLE is forbidden | blocker | No |
| `ddl.table.truncate.exists.require` | Checks TRUNCATE target existence and PostgreSQL relation eligibility, including a partitioned ONLY target that no earlier member selected. | blocker | **Yes** |
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

`ddl.alter.online_ddl.applicability` checks explicit MySQL 8.x `ALGORITHM=INSTANT` additions against observed MyISAM, InnoDB `ROW_FORMAT=COMPRESSED`, and InnoDB FULLTEXT restrictions. The existing rule reports the native refusal class.

`ddl.alter.keys_toggle.notice` uses the observed MyISAM engine to distinguish stopping maintenance of nonunique indexes with `DISABLE KEYS` from rebuilding disabled nonunique indexes with `ENABLE KEYS`.

## DDL: MySQL/TiDB Database/Schema Lifecycle Rules (2 rules)

These rules guard against MySQL and TiDB database/schema lifecycle DDL operations — `CREATE DATABASE`, `CREATE SCHEMA`, `DROP DATABASE`, and `DROP SCHEMA`. In MySQL/TiDB, `SCHEMA` is a synonym for `DATABASE`. They only apply when `--dialect mysql` or `--dialect tidb` is set and are skipped for PostgreSQL dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.database.create.notice` | `CREATE DATABASE` / `CREATE SCHEMA` creates a new logical namespace — informational notice | notice | No |
| `ddl.database.drop.warn` | `DROP DATABASE` / `DROP SCHEMA` removes a database and all contained objects — should be reviewed | warning | No |

> **Note:** These rules are MySQL/TiDB-specific and are automatically skipped when auditing PostgreSQL SQL. They are offline rules and do not require a database connection. `CREATE DATABASE IF NOT EXISTS` and `CREATE SCHEMA IF NOT EXISTS` still emit the notice. `DROP DATABASE IF EXISTS` and `DROP SCHEMA IF EXISTS` still emit the warning. DeltaScope does not perform live database existence validation. `CREATE DATABASE ... CHARACTER SET` / `COLLATE` options are preserved as parser facts but no policy rule governs them in this milestone. This is not full DDL support — trigger, routine, event, and database privilege lifecycle remain deferred.

---

## DDL: PostgreSQL Constraint Targets (3 rules)

| Rule ID | Bounded check | Default Level | Metadata Required |
|---------|---------------|:-------------:|:-----------------:|
| `ddl.pg.alter.validate_constraint.target.require` | Existing FK/CHECK target for VALIDATE. An unvalidated CHECK with attached children can require traversal. | blocker | Yes |
| `ddl.pg.alter.constraint_deferrability.target.require` | Existing FK target and direct partition-derived FK refusal for ALTER CONSTRAINT deferrability | blocker | Yes |
| `ddl.pg.alter.drop_constraint.target.require` | Checks exact DROP targets, direct foreign-key dependencies on primary or unique constraints under RESTRICT, inherited CHECKs and partition foreign keys, and CHECKs on partitioned parents targeted with ONLY. IF EXISTS guards absence. | blocker | Yes |

All three default-enabled behaviors are explicitly approved. They require a fresh complete constraint directory bound to the exact table/schema and an observed stable PostgreSQL 17 server banner. Decoded identifiers are case-sensitive; duplicate/unowned bindings are not last-wins. Missing, ambiguous, stale or out-of-scope facts owe `unproven_constraint_target` gaps (`target_table.pg_constraints`, `instance.version` or `constraint.if_exists`) rather than invented absence. Evidence gaps use the existing **unverified/review** protocol; syntax/parser boundaries remain incomplete. Caller `target_version` does not replace the server observation or widen the MySQL/TiDB-only public version-identity contract.

A proven already-validated FK/CHECK no longer receives the existing scan notice. A proven absent guarded DROP no longer receives the removal notice. Unknown facts retain conditional advice. Existing FK/PK forbids and NOT VALID→VALIDATE follow-up remain independent.

Accepted stage boundary: prior related DDL and preceding ALTER actions withhold old directory facts, not a fabricated PG post-state. Opaque/parser failures and unmodeled cross-identity effects conservatively invalidate more broadly. This does not enable general PostgreSQL ordered execution. Target checks do not establish data satisfaction, dependency removals, permissions, inheritance/partition legality or execution success; native 23514/23503 data refusals remain distinct.

The public Go `PGConstraintState` provider alias supplies name, native `pg_constraint.contype` and validated state; directory fields stay out of JSON. New findings emit only bounded identities/action, `reason_code`, `target_exists` and optional one-letter `target_constraint_kind`, never expressions or raw provider errors.

Maintainer notes: [decision and frozen T37 roles](../decisions/2026-11-17-postgresql-constraint-target-evidence.md). This remains a partial F09 slice.

---

## DDL: PostgreSQL ALTER Constraint Migration Risk (2 rules)

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.alter.add_foreign_key.not_valid.require` | ALTER ADD FOREIGN KEY without NOT VALID requests initial validation of existing rows | warning | No |
| `ddl.pg.alter.add_primary_key.concurrent_index.warn` | ALTER ADD PRIMARY KEY without USING INDEX requests an index build under the ALTER lock | warning | No |

Both are default-enabled; the new review impact is explicitly approved. Authored `NOT VALID` and `USING INDEX` are silent for their respective rules, not proof of target existence, index eligibility or successful execution. Missing/unowned option facts owe `unproven_constraint_declaration` evidence gaps (`constraint.not_valid` / `constraint.using_index`). CREATE and unrelated ALTER actions, MySQL and TiDB do not match.

The FK warning does not relax `ddl.table.foreign_key.forbid`. Existing CHECK NOT VALID and NOT VALID→VALIDATE follow-up rules remain independent. Remediation is conditional: PostgreSQL 17 does not permit NOT VALID foreign keys on partitioned tables. A separately built eligible unique index can reduce the PK index-build step, but USING INDEX attachment and NOT NULL verification can still require locks/scans; no actual duration or lock-free execution is promised.

The existing `ddl.pg.alter.add_unique_constraint.concurrent_index.advisory` keeps its ID/default notice level, but is silent for explicit UNIQUE USING INDEX instead of incorrectly claiming an index build. Plain UNIQUE still receives the advisory, with the documented ACCESS EXCLUSIVE lock explanation. New findings carry only bounded table/schema/constraint identity, kind, action and the relevant boolean declaration; no SQL or index-expression payload is emitted.

Maintainer notes: [decision](../decisions/2026-11-17-postgresql-alter-constraint-build-risk.md). This is one F09 slice, not complete constraint-state or whole-family coverage.

---

## DDL: PostgreSQL Column Storage and Compression (2 rules)

Default-enabled, parameterless blockers, requiring real observed PG17:

| Rule | Bounded check |
| --- | --- |
| `ddl.pg.alter.column.storage.valid` | Invalid case-insensitive STORAGE mode, proven absent user column/system-target inapplicability, or non-PLAIN strategy on a proven non-TOAST-aware type |
| `ddl.pg.alter.column.compression.valid` | Invalid case-sensitive COMPRESSION method, absent/inapplicable target, nondefault compression on a non-TOAST-aware type, or lz4 unavailable in the observed build enum |

STORAGE accepts PLAIN/EXTERNAL/EXTENDED/MAIN/DEFAULT with native case ownership;
Unicode lowercase lookalikes are not silently accepted. Actual `pg_type.typstorage`
establishes type readiness and DEFAULT strategy, not a type-name/integer-width
guess or current column `attstorage`. COMPRESSION accepts pglz/lz4/default;
quoted upper-case names are not equivalent. Compression default is legal on
fixed-length types. A toastable text column using STORAGE PLAIN may still
receive pglz/lz4. A selected pglz default does not prove lz4 unavailable: support
comes from the complete observed `default_toast_compression` GUC enum in the
existing ordinary-reader pg_catalog.pg_settings read, without privileged
pg_config access. Version/settings and their comparisons, plus D7's new
attribute filter and actual-type join, bind explicitly to pg_catalog sources/
operators, not search_path namesakes. This is not blanket hardening of all
historical metadata queries.

Exact fresh ordinary persistent non-inheritance/non-partition target facts are
required; intrinsic mode/method refusals need the observed version, not an
invented target. Missing/stale/misbound/ambiguous/unmodeled proof retains
`unproven_column_storage` or `unproven_column_compression` (unverified, at least
review), requiring the relevant `instance.version`, `target_table.pg_columns`,
`column.storage_strategy`, `column.compression_method`, `column.type_storage`
or `instance.pg_compression_lz4`. Earlier related DDL/parser failures/ALTER
siblings invalidate old proof; no post-state, privilege or dependency simulation.

Observed negative-attnum system members are separate from user Columns; an
empty user directory never proves ctid absent. Related existing generation,
statistics/options and DROP checks reject proven inapplicable system targets,
not fictitious absence. IF EXISTS does not make an existing system column a
no-op; the old DROP ID remains and its deletion advice is suppressed for that
proved inapplicable target. Missing system observations remain gaps. Exact
quoted upper-case user names are not conflated with system members.

SET STORAGE/COMPRESSION does not itself rewrite/recompress existing rows, nor
certify future ratio, data, dependencies, privileges or successful execution.
New blocker findings contain bounded action/identifiers/reason, not method
payloads, private catalog/build facts or provider errors. Independent forbids
and MySQL/TiDB stay unchanged. Other F09 dimensions remain open.

[Decision and exact evidence](../decisions/2026-11-18-postgresql-column-storage-compression-targets.md).

## DDL: PostgreSQL Column Tuning and Guarded Drops (3 new rules)

Default-enabled, parameterless, with real observed PG17 facts:

| Rule | Bounded check | Default level |
| --- | --- | --- |
| `ddl.pg.alter.column.statistics.target.valid` | SET STATISTICS below -1 or a proven missing target column | blocker |
| `ddl.pg.alter.column.statistics.clamp.warn` | Applicable explicit target above 10000 is lowered to 10000 by PostgreSQL | warning |
| `ddl.pg.alter.column.options.valid` | Known builtin SET n_distinct/n_distinct_inherited range [-1, DBL_MAX], duplicate known keys and proven missing SET/RESET columns | blocker |

DEFAULT, -1 and targets 0..10000 are valid. A larger target is **not** illegal:
it succeeds with native clamping and a warning. Known-invalid targets suppress
clamp advice. Ordered duplicate SET keys are not last-wins. RESET removes keys;
an unknown RESET key is not inherently an invalid SET declaration.

Target-dependent checks use exact fresh ordinary persistent non-inheritance/
non-partition column facts. Intrinsic modeled numeric refusals require the
observed PG17 version, not an invented catalog. Unknown/extension SET keys,
quoted/nonfinite/overflow/underflow/subnormal/unmodeled values or stale/ambiguous/misbound facts
retain `unproven_column_tuning` gaps (unverified, at least review), requiring
`instance.version`, `target_table.pg_columns`, `column.statistics_target` or
`column.options`. Prior related DDL/parser failures and earlier ALTER actions
withhold old directories without simulating post-state. This does not certify
sampling effectiveness, data, expressions, privileges or broader table kinds.
PG17 rejects `strtod` underflow even when Go returns a finite subnormal or
rounds a nonzero literal to zero without error. Such literals, including the
minimum-normal rounding boundary, remain gaps; genuine zero is identified from
the AST numeric coefficient and ordinary small normal values remain modeled.
A natively valid minimum-normal literal can therefore still require review.

The existing `ddl.alter.drop_column.exists.require` keeps its ID/default but
uses exact fresh PG17 column/guard proof on PostgreSQL. A proven absent column
with parsed IF EXISTS is a no-op: neither the missing-column blocker nor its
old deletion advisory fires. Without the guard, proven absence still blocks;
existing columns retain deletion risk. Unknown proof owes
`unproven_column_target` (`instance.version`, `target_table.pg_columns` or
`column.if_exists`), not guessed absence/no-op. MySQL/TiDB behavior and
independent explicitly enabled forbids remain unchanged. CASCADE is retained,
not dependency/permission or successful-execution proof.

Storage advice is conditional: SET fillfactor does not itself rewrite existing
rows; selecting the current access method is not a rewrite, and partitioned
settings do not themselves rewrite partition data. No extra default predicate
is introduced. New finding metadata has bounded action/table/schema/column/
parameter/reason only, not option values, quoted payloads or provider errors.

[Decision and remaining scope](../decisions/2026-11-18-postgresql-column-tuning-and-drop-guards.md).
Other original F09 dimensions remain open; this is not whole-row support.

## DDL: PostgreSQL Existing-Column Generation (4 rules)

All four are enabled by default, parameterless, and PG17 metadata-aware.

| Rule | Bounded check | Default level |
| --- | --- | --- |
| `ddl.pg.alter.column.generation.transition.valid` | ADD/SET/DROP IDENTITY, DROP EXPRESSION and DEFAULT/NOT NULL generation/PK target preconditions, including disallowed SET DEFAULT references, and on qualified ordinary tables in single-action SET DEFAULT, direct bool/int4 cross-type literals and parameters used as the whole expression or a direct COALESCE argument, live replica identity key slots for DROP NOT NULL and PostgreSQL 17 partition NOT NULL traversal. For single-action SET NOT NULL on a qualified ordinary table, a validated exact-column `IS NOT NULL` CHECK can prove the `int4` column's nullability premise without a data scan | blocker |
| `ddl.pg.alter.identity.sequence_options.valid` | Duplicate modes/options and proven invalid increment/cache/type/bounds/START/RESTART | blocker |
| `ddl.pg.alter.identity.restart.warn` | Explicit RESTART value-reuse and concurrent-issuance blocking risk | warning |
| `ddl.pg.alter.identity.cycle.warn` | Explicit CYCLE in ADD IDENTITY or SET options | warning |

An observed PG17 banner and exact, fresh ordinary persistent non-inheritance/
non-partition table facts define this stage. ADD requires an existing ordinary
integer column already NOT NULL without DEFAULT or generation. SET identity/
options/RESTART requires an identity column. DROP IDENTITY/EXPRESSION respects
its parsed IF EXISTS for kind absence, never for a missing column. DEFAULT on
identity/stored-generated and DROP NOT NULL on identity/PK are proven conflicts.
Known-valid targets/options emit no blocker; known guarded no-ops emit no new
error. Existing blanket forbids and independent rules remain unchanged, so
silence is not an overall pass or execution promise.

ADD reuses bounded new-sequence declaration checks; SET uses the actual
catalog-dependency-bound sequence type and stable parameters, never a guessed
name or CREATE defaults. SET START alone changes recorded START, not the current
value, and does not trigger RESTART risk. NO CYCLE and omission do not trigger
CYCLE risk. D2 CREATE/ADD COLUMN rules remain separate without duplicate matches.

Unknown/ambiguous/unmodeled/stale or wrongly bound facts retain
`unproven_column_generation_target` / `unproven_identity_sequence_options`
evidence gaps (unverified, at least review). No observed PG17 version means a
version gap, not a guessed blocker. With that version alone, intrinsic options
refusals can be proven without a column catalog. Nullable SET NOT NULL requires
unread data proof; narrowing bounds without explicit RESTART can require unread
current-value proof. Prior related DDL/parser failures and earlier ALTER actions
withhold old facts; no PostgreSQL post-state is simulated. No data scan,
expression/permission analysis or sequence current-value read/advance is added.
New findings never carry expressions, provider errors or numeric option payloads.

## DDL: PostgreSQL Generated/Identity Declarations (2 rules)

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.column.generation.declaration.valid` | Checks authored CREATE TABLE / ADD COLUMN generation declarations for DEFAULT/NULL conflicts, proven scalar integer identity type and identity sequence bounds | blocker | No |
| `ddl.pg.column.identity.cycle.warn` | Warns on an unambiguous identity CYCLE declaration because values may be reused | warning | No |

Both rules are default-enabled; the resulting reject/review impact is explicitly approved. ALWAYS and BY DEFAULT without CYCLE, NO CYCLE, and omitted cycling emit no cycling finding. A proven invalid declaration suppresses the cycling warning. INCREMENT 0, nonpositive CACHE, type-incompatible MIN/MAX, MIN ≥ MAX and START outside effective bounds are validity findings; omitted sequence bounds use PostgreSQL's documented CREATE defaults and increment direction. Unresolved/custom type names, repeated generation clauses/options, unmodeled options and values outside modeled int64 remain `unproven_generation_declaration` evidence gaps (`column.generation_declaration`, `column.identity_type`, `column.identity_options`), never last-wins conclusions.

Scope is PostgreSQL CREATE TABLE and ALTER TABLE ADD COLUMN only. Existing-column ADD/SET/DROP IDENTITY and SET EXPRESSION keep their prior checks. Generated-expression legality, volatility and dependencies, inheritance/LIKE/partition semantics, privileges, uniqueness and runtime sequence progress are not validated. New finding fields contain bounded table/column/action/generation/mode/reason only, not expression SQL or numeric option payloads. Existing authored SQL output is unchanged. Native evidence targets PostgreSQL 17.7; parser errors and other unsupported boundaries stay explicit.

Maintainer notes: [decision](../decisions/2026-11-16-postgresql-generated-identity-governance.md).

---

### MySQL trigger and event identities

`ddl.mysql.stored_object.target.valid` checks visible qualified trigger and event conflicts, trigger ordering groups, and event rename targets.

## DDL: MySQL Stored-Routine Rules

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.routine.target.valid` | Checks qualified stored routine occupancy and the version prerequisite for CREATE IF NOT EXISTS. | blocker | Yes |
| `ddl.routine.security.definer.warn` | Stored `CREATE PROCEDURE` / `CREATE FUNCTION` with explicit or default DEFINER security, and `ALTER PROCEDURE` / `ALTER FUNCTION` with explicit `SQL SECURITY DEFINER`, require review | warning | No |

Enabled by default, MySQL only. Omitted CREATE security uses MySQL's documented DEFINER default; omitted ALTER security makes no claim about the existing mode. Explicit INVOKER emits no finding from this rule, not a safety certificate. A `DEFINER=account` clause is not an independent trigger: with INVOKER it is silent; with omitted CREATE security the default still warns. SONAME/loadable functions, DROP, TiDB vendor boundaries and PostgreSQL are outside this rule.

Findings contain bounded routine identity, kind, operation, `security=definer`, `security_source=authored|create_default`, and optional schema. No privilege/account resolution or body analysis is performed. Existing RETURN and dynamic-SQL boundaries remain incomplete alongside any warning. Unknown/repeated security characteristics remain `unproven_routine_security` evidence gaps (`routine.sql_security`), never a last-wins INVOKER pass. This approved default-on warning intentionally changes otherwise-pass audits to review; the usual enabled/level policy overrides apply. See [decision](../decisions/2026-11-16-mysql-routine-security-definer-warn.md).

---

## DDL: MySQL/TiDB DDL Notice Rules (27 rules)

These rules promote previously normalized-silent MySQL and TiDB DDL forms to finding-covered status. They cover lifecycle DDL events (standalone index, rename, database alteration, user/role/privilege management, procedure lifecycle, resource group), ALTER TABLE action notices (column add/drop/modify, constraint, index, foreign key), and TiDB-specific placement policy and sequence lifecycle. They only apply when `--dialect mysql` or `--dialect tidb` is set and are skipped for PostgreSQL dialects.

### Standalone DDL Lifecycle (20 rules, shared)

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.rename_table.notice` | `RENAME TABLE` renames a table | notice | No |
| `ddl.create_index.notice` | Standalone `CREATE INDEX` creates a new index | notice | No |
| `ddl.drop_index.foreign_key.warn` | Warns when a proven foreign key depends on the standalone DROP INDEX target. | warning | Yes |
| `ddl.drop_index.notice` | Standalone `DROP INDEX` removes an index | notice | No |
| `ddl.alter_database.notice` | `ALTER DATABASE` changes database-level settings | notice | No |
| `ddl.create_procedure.notice` | `CREATE PROCEDURE` defines a stored procedure | notice | No |
| `ddl.drop_procedure.notice` | `DROP PROCEDURE` removes a stored procedure | notice | No |
| `ddl.create_user.notice` | `CREATE USER` creates a database user | notice | No |
| `ddl.alter_user.notice` | `ALTER USER` modifies a database user | notice | No |
| `ddl.drop_user.notice` | `DROP USER` removes a database user | notice | No |
| `ddl.create_role.notice` | `CREATE ROLE` creates a new role | notice | No |
| `ddl.drop_role.notice` | `DROP ROLE` removes a role | notice | No |
| `ddl.grant.notice` | `GRANT` assigns privileges | notice | No |
| `ddl.revoke.notice` | `REVOKE` removes privileges | notice | No |
| `ddl.drop_resource_group.notice` | `DROP RESOURCE GROUP` removes a resource group (MySQL-only) | notice | No |
| `ddl.alter.add_column.notice` | `ALTER TABLE ... ADD COLUMN` adds a column | notice | No |
| `ddl.alter.add_constraint.notice` | `ALTER TABLE ... ADD CONSTRAINT` adds a constraint | notice | No |
| `ddl.alter.drop_column.notice` | `ALTER TABLE ... DROP COLUMN` removes a column | notice | No |
| `ddl.alter.modify_column.notice` | `ALTER TABLE ... MODIFY COLUMN` changes column definition | notice | No |
| `ddl.alter.drop_index.notice` | `ALTER TABLE ... DROP INDEX` removes an index | notice | No |
| `ddl.alter.drop_foreign_key.notice` | `ALTER TABLE ... DROP FOREIGN KEY` removes a foreign key | notice | No |

### TiDB-Specific (7 rules)

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.create_placement_policy.notice` | `CREATE PLACEMENT POLICY` defines a placement policy | notice | No |
| `ddl.alter_placement_policy.notice` | `ALTER PLACEMENT POLICY` modifies a placement policy | notice | No |
| `ddl.drop_placement_policy.notice` | `DROP PLACEMENT POLICY` removes a placement policy | notice | No |
| `ddl.create_sequence.notice` | `CREATE SEQUENCE` defines a sequence | notice | No |
| `ddl.alter_sequence.notice` | `ALTER SEQUENCE` modifies a sequence | notice | No |
| `ddl.drop_sequence.notice` | `DROP SEQUENCE` removes a sequence | notice | No |
| `ddl.tidb.alter_table.placement_policy.notice` | `ALTER TABLE ... PLACEMENT POLICY` attaches a placement policy to a table | notice | No |

> **Note:** These rules are MySQL/TiDB-specific and are automatically skipped when auditing PostgreSQL SQL. They are offline rules and do not require a database connection. These are informational notice rules only — they do not block, warn, or perform live validation. `ddl.drop_resource_group.notice` is MySQL-only and is automatically skipped for TiDB. TiDB placement policy and sequence rules are automatically skipped for MySQL. This is not full MySQL/TiDB DDL support — triggers, events, functions, TiDB ALTER VIEW, ALTER PROCEDURE, tablespace, and parser-error forms remain deferred.

---

## DDL: PostgreSQL Migration-Safety Rules (9 rules)

These rules guard against common PostgreSQL migration patterns that can cause table rewrites, long-held locks, or production incidents. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.create_index.concurrently.require` | `CREATE INDEX` must use `CONCURRENTLY` to avoid blocking reads/writes. Findings include bounded index shape metadata: `index_kind`, `access_method`, `column_count`, `included_column_count`, `has_predicate`, `has_expression_keys`, `expression_count`. Metadata is additive and does not emit predicate or expression SQL text. | warning | No |
| `ddl.pg.alter.add_column.non_null_default.rewrite.warn` | Adding a `NOT NULL` column with a volatile default may trigger a full table rewrite. Findings include: `not_null`, `has_default`, `default_kind` (`literal`, `null`, `function_call`, `expression`, `unknown`). Metadata does not emit default expression text or function names. | warning | No |
| `ddl.pg.alter.constraint_using_index.valid` | Checks exact `PRIMARY KEY` or `UNIQUE USING INDEX` prerequisites on a fresh ordinary PostgreSQL 17 table. Resolves default operator classes on key columns using exact and binary-compatible types, including PostgreSQL 17 polymorphic defaults. Unobserved applicable facts yield `unproven_constraint_using_index`. | blocker | No |
| `ddl.pg.alter.constraint_columns.valid` | Checks exact PostgreSQL 17 PRIMARY KEY and UNIQUE key and INCLUDE members on fresh ordinary tables. Repeated key columns, missing members and observed system columns block. Also checks partition-key membership and bounded equality and collation compatibility on qualified partitioned roots. | blocker | Yes |
| `ddl.pg.alter.add_foreign_key.targets.valid` | Checks exact table and column identities, referenced key eligibility, and column counts on fresh PostgreSQL 17 ordinary tables. An omitted reference column list uses the observed primary key. The selected key also supplies bounded equality and implicit conversion checks for scalar, domain, enum and array types. `ON DELETE SET NULL` and `SET DEFAULT` column lists must name members of the declared foreign key. Also rejects a single `ADD FOREIGN KEY … NOT VALID` on a confirmed schema-qualified partitioned source table in PostgreSQL 17. | blocker | No |
| `ddl.pg.alter.add_check.name.available` | Checks simple column and whole-row null tests with exact qualifiers, subqueries and supported literal Boolean coercion in a single PostgreSQL 17 ADD CHECK, and checks exact name availability. Numeric constants require observed cast facts. Also rejects unnamed `CHECK (TRUE)` with `NO INHERIT` on a confirmed persistent partition root or with `ONLY` when that root has an attached partition. Applies to NOT VALID. | blocker | No |
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
| `ddl.pg.create_schema.target.valid` | Checks PostgreSQL 17 schema creation names, owner existence and guarded destinations. | blocker | Yes |
| `ddl.pg.create_schema.notice` | `CREATE SCHEMA` creates a new namespace — informational notice | notice | No |
| `ddl.pg.drop_schema.advisory` | `DROP SCHEMA` removes a schema — advises review of dependent objects | notice | No |
| `ddl.pg.drop_schema.cascade.warn` | `DROP SCHEMA ... CASCADE` uses cascading deletion — may silently drop dependent objects | warning | No |
| `ddl.pg.sequence.declaration.valid` | Checks PostgreSQL 17 qualified CREATE and ALTER SEQUENCE targets, options, restart bounds, OWNED BY, RENAME and SET SCHEMA. | blocker | Yes |
| `ddl.pg.create_sequence.cycle.warn` | `CREATE SEQUENCE ... CYCLE` may cause sequence value wraparound | warning | No |
| `ddl.pg.alter_sequence.restart.warn` | `ALTER SEQUENCE ... RESTART` resets the sequence counter — may conflict with existing rows | warning | No |
| `ddl.pg.alter_sequence.cycle.warn` | `ALTER SEQUENCE ... CYCLE` enables value wraparound on an existing sequence | warning | No |
| `ddl.pg.drop_sequence.advisory` | `DROP SEQUENCE` removes a sequence — advises review of dependent columns | notice | No |
| `ddl.pg.drop_sequence.cascade.warn` | `DROP SEQUENCE ... CASCADE` uses cascading deletion — may silently drop dependent objects | warning | No |
| `ddl.pg.drop_materialized_view.advisory` | `DROP MATERIALIZED VIEW` removes a materialized view — advises review of dependent queries | notice | No |
| `ddl.pg.drop_materialized_view.cascade.warn` | `DROP MATERIALIZED VIEW ... CASCADE` uses cascading deletion — may silently drop dependent objects | warning | No |
| `ddl.pg.refresh_materialized_view.concurrently.warn` | Non-concurrent `REFRESH MATERIALIZED VIEW` (default or explicit `WITH DATA`) holds an exclusive lock | warning | No |
| `ddl.pg.refresh_materialized_view.no_data.notice` | `REFRESH MATERIALIZED VIEW ... WITH NO DATA` empties the view — PostgreSQL marks the view unpopulated until a refresh with data | notice | No |
| `ddl.pg.refresh_materialized_view.target.valid` | Checks exact PostgreSQL 17 refresh targets. Concurrent refresh requires a populated materialized view and a usable unique B-tree index. WITH NO DATA conflicts with CONCURRENTLY. Missing catalog facts produce an evidence gap. | blocker | Yes |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. `CREATE SCHEMA IF NOT EXISTS` still emits the notice from `ddl.pg.create_schema.notice`. Existing `DROP SCHEMA` behavior (`ddl.pg.drop_schema.advisory`, `ddl.pg.drop_schema.cascade.warn`) is unchanged. `CREATE SCHEMA AUTHORIZATION` and nested `CREATE SCHEMA ... CREATE TABLE ...` remain unsupported/deferred. DeltaScope does not perform live schema existence validation. `CONCURRENTLY` refreshes pass both materialized view rules without findings. `WITH NO DATA` triggers both rules because it is also non-concurrent. This is not live unique-index validation for `CONCURRENTLY` — DeltaScope does not verify whether a unique index exists on the materialized view.

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. Starting with `v0.41.0`, `ddl.pg.alter.add_check.not_valid.require` also fires on `ALTER TABLE ... ADD CONSTRAINT ... CHECK` statements. Starting with `v0.42.0`, `ddl.pg.alter.not_valid_constraint.validate.require` checks same-batch validation pairing for named CHECK and FOREIGN KEY `NOT VALID` constraints. Check naming rules (`ddl.constraint.check.name.prefix.require`, `ddl.constraint.check.name.suffix.require`, `ddl.constraint.check.name.contains.require`) also cover the ALTER TABLE CHECK path when configured.

---

## DDL: PostgreSQL Type Lifecycle Rules (5 rules)

These rules guard against risky PostgreSQL type lifecycle DDL operations — enum type creation, value addition, and type drops. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.alter_operator.change.target.valid` | Checks exact PostgreSQL 17 operator signatures, owner resolution, schema destinations and qualified RESTRICT/JOIN functions. | blocker | Yes |
| `ddl.pg.alter_type.change.target.valid` | Checks exact PostgreSQL 17 type rename, schema moves and owner resolution. | blocker | Yes |
| `ddl.pg.type.composite.valid` | Checks qualified PostgreSQL 17 composite type creation and single attribute actions. | blocker | Yes |
| `ddl.pg.type.base.valid` | Checks PostgreSQL 17 shell and base type declarations and bounded property changes. | blocker | Yes |
| `ddl.pg.create_type.range.valid` | Checks qualified PostgreSQL 17 range targets, scalar subtypes and bounded option refusals. | blocker | Yes |
| `ddl.pg.create_type.enum.valid` | Checks qualified PostgreSQL 17 enum creation and ordered label declarations. | blocker | Yes |
| `ddl.pg.alter_type.enum.valid` | Checks qualified PostgreSQL 17 enum type identity, exact labels and ADD/RENAME precedence. | blocker | Yes |
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
| `ddl.pg.create_language.handler.valid` | Checks PostgreSQL 17 explicit language callback signatures and replacement prerequisites. | blocker | Yes |
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
| `ddl.pg.grant.declaration.valid` | Checks PostgreSQL 17 privilege target, role and column prerequisites. | blocker | Yes |
| `ddl.pg.grant.table_privilege.notice` | `GRANT ... ON TABLE` grants table-level privileges — informational notice | notice | No |
| `ddl.pg.grant.table_privilege.all.warn` | `GRANT ALL PRIVILEGES ON TABLE` grants all privileges — warns about over-permission | warning | No |
| `ddl.pg.revoke.table_privilege.notice` | `REVOKE ... ON TABLE` revokes table-level privileges — informational notice | notice | No |
| `ddl.pg.revoke.table_privilege.cascade.warn` | `REVOKE ... ON TABLE ... CASCADE` cascades revocation to dependent privileges — warns about cascade side-effects | warning | No |

> **Note:** These rules are PostgreSQL-specific and are automatically skipped when auditing MySQL or TiDB SQL. They are offline rules and do not require a database connection. `GRANT ALL PRIVILEGES ON TABLE` intentionally emits both `ddl.pg.grant.table_privilege.notice` and `ddl.pg.grant.table_privilege.all.warn`. `REVOKE ... ON TABLE ... CASCADE` intentionally emits both `ddl.pg.revoke.table_privilege.notice` and `ddl.pg.revoke.table_privilege.cascade.warn`. Supports multiple privileges (e.g., `SELECT, INSERT`), multiple grantees, and schema-qualified table names (e.g., `public.users`). DeltaScope does not perform live validation — no grantee/role existence checks, no table/object existence checks, no grantor permission checks, no effective privilege computation, no role inheritance resolution, no ownership verification, and no RLS/policy evaluation. `ALL TABLES IN SCHEMA`, sequence privileges, role membership GRANT/REVOKE, and `ALTER DEFAULT PRIVILEGES` are not supported. This is narrow table-level privilege DCL support — not broad governance or admin DCL support.

---

## DDL: PostgreSQL ALTER TABLE Coverage Rules (53 config entries)

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
| `ddl.pg.alter.set_tablespace.notice` | `ALTER TABLE ... SET TABLESPACE` describes the request and recognizes proven same-space selection | notice | No |
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

### Table Relationship (v0.150.0)

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.alter.add_inherit.notice` | `ALTER TABLE ... INHERIT` adds a parent table relationship — informational notice | notice | No |
| `ddl.pg.alter.drop_inherit.notice` | `ALTER TABLE ... NO INHERIT` removes a parent table relationship — informational notice | notice | No |
| `ddl.pg.alter.add_of_type.notice` | `ALTER TABLE ... OF type_name` binds the table to a typed table type — informational notice | notice | No |
| `ddl.pg.alter.drop_of_type.notice` | `ALTER TABLE ... NOT OF` removes the typed table binding — informational notice | notice | No |

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
### Constraint Deferrability (v0.160.0)

| Rule ID | Default Level | Action | Description |
|---------|--------------|--------|-------------|
| `ddl.pg.alter.constraint_deferrable.notice` | notice | `alter_constraint_deferrable` | FK constraint marked DEFERRABLE |
| `ddl.pg.alter.constraint_initially_deferred.notice` | notice | `alter_constraint_initially_deferred` | FK constraint marked INITIALLY DEFERRED |

> **Note (v0.160.0):** These rules are PostgreSQL-specific and offline. Constraint deferrability findings emit bounded metadata only: operation, action, table, constraint name, constraint type (`foreign_key`), and deferrable/initially_deferred boolean flags. No raw SQL, expression text, predicate text, operator class names, exclusion operators, sequence options, catalog state, validation results, or dependency graphs are emitted. `SET WITHOUT OIDS` is normalized silently with action `set_without_oids` and produces no finding (obsolete since PG12).

### Final Parseable Boundary (v0.170.0)

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.alter.set_expression.notice` | `ALTER TABLE ... ALTER COLUMN ... SET EXPRESSION` sets a generated column expression — informational notice | notice | No |
| `ddl.pg.alter.add_identity.notice` | `ALTER TABLE ... ALTER COLUMN ... ADD GENERATED ... AS IDENTITY` adds identity to an existing column — informational notice | notice | No |
| `ddl.pg.alter.add_exclusion_constraint.notice` | `ALTER TABLE ... ADD CONSTRAINT ... EXCLUDE USING` adds an exclusion constraint — informational notice | notice | No |
| `ddl.pg.alter.move_all_tablespace.notice` | `ALTER TABLE ALL IN TABLESPACE ... SET TABLESPACE ...` requests a transactional tablespace move for matching tables | notice | No |

> **Note (v0.170.0):** These rules are PostgreSQL-specific and offline. Final parseable boundary findings emit bounded metadata only: operation, action, table, column (for SET EXPRESSION and ADD IDENTITY), and constraint name (for ADD EXCLUSION CONSTRAINT). No expression body, sequence options, exclusion operators/predicates, operator class names, catalog state, or raw SQL are emitted.

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
| `ddl.pg.alter_policy.target.valid` | Checks exact PostgreSQL 17 policy targets, roles, rename occupancy and bounded predicate clauses. | blocker | Yes |
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

## DDL: PostgreSQL Advanced View Lifecycle Rules (15 rules)

These rules guard PostgreSQL view lifecycle operations beyond the base `CREATE VIEW` / `DROP VIEW` forms. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.create_or_replace_view.advisory` | `CREATE OR REPLACE VIEW` replaces an existing view — advises review of downstream dependencies | notice | No |
| `ddl.pg.create_temp_view.notice` | `CREATE TEMP VIEW` / `CREATE TEMPORARY VIEW` creates a session-scoped view — informational notice | notice | No |
| `ddl.pg.create_view.check_option.notice` | `CREATE VIEW ... WITH CHECK OPTION` enforces check option on inserts/updates through the view — informational notice | notice | No |
| `ddl.pg.create_view.options.valid` | Checks PostgreSQL 17 view option declarations and refuses CHECK OPTION on a source-free literal query | blocker | Yes |
| `ddl.pg.drop_view.cascade.warn` | `DROP VIEW ... CASCADE` uses cascading deletion — may silently drop dependent objects | warning | No |
| `ddl.pg.drop_view.target.valid` | Checks ordered PostgreSQL 17 DROP VIEW targets, guarded absence, and direct rewrite and row-type or row-type array column dependencies under RESTRICT | blocker | Yes |
| `ddl.pg.alter_view.rename.notice` | `ALTER VIEW ... RENAME TO` changes the view name — informational notice | notice | No |
| `ddl.pg.alter_view.set_schema.notice` | `ALTER VIEW ... SET SCHEMA` moves the view to a different schema — informational notice | notice | No |
| `ddl.pg.alter_view.namespace.valid` | Checks exact PostgreSQL 17 VIEW and MATERIALIZED VIEW rename and schema-move sources, guards and destination collisions | blocker | Yes |
| `ddl.pg.alter_view.owner.valid` | Checks PostgreSQL 17 ordinary and materialized view owner-change source, guarded absence and observed role existence | blocker | Yes |
| `ddl.pg.alter_view.options.valid` | Checks ordinary PostgreSQL 17 ALTER VIEW SET/RESET targets and declarations. CHECK OPTION updatability requires separate evidence. | blocker | Yes |
| `ddl.pg.create_table.columns.valid` | Checks duplicate and system column names, and bounded qualified type targets in PostgreSQL 17 CREATE TABLE. | blocker | Yes |
| `ddl.pg.create_table.inherits.valid` | Checks qualified PostgreSQL 17 INHERITS parents, repeated parent names and relation eligibility. | blocker | Yes |
| `ddl.pg.create_table.foreign_key.valid` | Checks PostgreSQL 17 CREATE TABLE foreign-key references, member names, referenced key eligibility and ON DELETE SET membership. | blocker | Yes |
| `ddl.pg.create_table.expression.valid` | Checks bounded PostgreSQL 17 CREATE TABLE default, CHECK and generated expression refusals. | blocker | Yes |
| `ddl.pg.create_table.keys.valid` | Checks PostgreSQL 17 direct primary and unique key declarations, including exact members and creation guards. | blocker | Yes |
| `ddl.pg.create_table.storage.valid` | Checks PostgreSQL 17 qualified CREATE TABLE access methods, tablespaces and bounded storage settings. | blocker | Yes |
| `ddl.pg.create_table.of.valid` | Checks PostgreSQL 17 CREATE TABLE OF destination guards, standalone composite types and exact column option names. | blocker | Yes |
| `ddl.pg.create_table.on_commit.valid` | Checks PostgreSQL 17 CREATE TABLE persistence and ON COMMIT, including qualified namespace and absence-guard precedence. | blocker | Yes |
| `ddl.pg.create_table.like.target.valid` | Checks qualified PostgreSQL 17 CREATE TABLE LIKE source kinds, destination guards and copied column names. | blocker | Yes |
| `ddl.pg.create_view.output.valid` | Checks qualified PostgreSQL 17 CREATE VIEW targets and literal output compatibility for OR REPLACE. | blocker | Yes |
| `ddl.pg.alter_view.column_default.valid` | Checks PostgreSQL 17 view SET/DROP DEFAULT targets and rejects raw column references or subqueries in a default | blocker | Yes |
| `ddl.pg.alter_materialized_view.options.valid` | Checks a qualified PostgreSQL 17 materialized-view SET or RESET, including integer fillfactor and recognized heap and TOAST declarations. | blocker | Yes |
| `ddl.pg.alter_materialized_view.extension.valid` | Checks the exact PostgreSQL 17 materialized view and installed extension for dependency changes. | blocker | Yes |

The option check uses the observed server version and authored options. View updatability, query results and downstream effects require separate evidence.

---

## DDL: PostgreSQL Selected ALTER Object Lifecycle Rules (6 rules)

These rules guard selected PostgreSQL ALTER object operations for schema, index, and materialized view. They only apply when `--dialect postgresql` is set and are skipped for MySQL/TiDB dialects.

| Rule ID | Description | Default Level | Metadata Required |
|---------|-------------|:-------------:|:-----------------:|
| `ddl.pg.alter_schema.rename.notice` | `ALTER SCHEMA ... RENAME TO` changes the schema name — informational notice | notice | No |
| `ddl.pg.alter_schema.owner.notice` | `ALTER SCHEMA ... OWNER TO` changes the schema owner — informational notice | notice | No |
| `ddl.pg.alter_index.rename.notice` | `ALTER INDEX ... RENAME TO` changes the index name — informational notice | notice | No |
| `ddl.pg.create_index.include.valid` | Checks qualified PostgreSQL 17 index INCLUDE targets, method support and exact included columns. | blocker | Yes |
| `ddl.pg.alter_index.destination.valid` | Checks qualified PostgreSQL 17 ALTER INDEX rename and tablespace destinations. | blocker | Yes |
| `ddl.pg.alter_index.options.valid` | Checks exact PostgreSQL 17 ALTER INDEX SET/RESET targets, integer fillfactor and B-tree deduplicate_items. | blocker | Yes |
| `ddl.pg.alter_index.attach_partition.valid` | Checks qualified PostgreSQL 17 index partition attachment identities and direct parent relationships. | blocker | Yes |
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
| `ddl.pg.alter_publication.target.valid` | Checks PostgreSQL 17 publication identity, local membership and publish options. | blocker | Yes |
| `ddl.pg.alter_publication.notice` | `ALTER PUBLICATION` modifies an existing publication — informational notice | notice | No |
| `ddl.pg.drop_publication.warn` | `DROP PUBLICATION` removes a publication — warns that subscribers will stop receiving changes | warning | No |
| `ddl.pg.create_subscription.notice` | `CREATE SUBSCRIPTION` establishes a new subscription connection — informational notice | notice | No |
| `ddl.pg.alter_subscription.target.valid` | Checks PostgreSQL 17 subscription identity, local options and slot prerequisites. | blocker | Yes |
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
| `ddl.pg.alter_foreign_data_wrapper.target.valid` | Checks PostgreSQL 17 wrapper identity, routines, owner and ordered options. | blocker | Yes |
| `ddl.pg.alter_foreign_data_wrapper.notice` | `ALTER FOREIGN DATA WRAPPER` modifies an existing FDW — informational notice | notice | No |
| `ddl.pg.drop_foreign_data_wrapper.warn` | `DROP FOREIGN DATA WRAPPER` removes an FDW — warns about dependent foreign servers and tables | warning | No |
| `ddl.pg.create_foreign_server.notice` | `CREATE SERVER` registers a new foreign server — informational notice | notice | No |
| `ddl.pg.alter_foreign_server.notice` | `ALTER SERVER` modifies an existing foreign server — informational notice | notice | No |
| `ddl.pg.drop_foreign_server.warn` | `DROP SERVER` removes a foreign server — warns about dependent user mappings and foreign tables | warning | No |
| `ddl.pg.create_user_mapping.notice` | `CREATE USER MAPPING` registers a user mapping for a foreign server — informational notice | notice | No |
| `ddl.pg.alter_user_mapping.notice` | `ALTER USER MAPPING` modifies an existing user mapping — informational notice | notice | No |
| `ddl.pg.drop_user_mapping.warn` | `DROP USER MAPPING` removes a user mapping — warns about dependent foreign table connections | warning | No |
| `ddl.pg.create_foreign_table.notice` | `CREATE FOREIGN TABLE` introduces a new foreign table — informational notice | notice | No |
| `ddl.pg.alter_foreign_table.target.valid` | Checks PostgreSQL 17 foreign-table targets, columns, options, owners and destinations. | blocker | Yes |
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

## DML Rules (11 rules)

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
| `dml.table.exists.require` | MySQL/TiDB/PostgreSQL DML target table must exist in live metadata | blocker | **Yes** |

---

## Offline vs Metadata-Backed Rules

Rules fall into two categories based on whether they require a live database connection:

**Offline rules** always run, even when no database connection is provided. They evaluate the SQL text
and AST alone. All rules not listed as metadata-backed below are offline rules.

**Metadata-backed rules** normally silently skip when no `MetadataProvider` is active. During offline audits
they do not claim unavailable state. Two rules are explicit exceptions: the MODIFY unknown-prior-state advisory
runs offline at notice level while the existing MODIFY blocker only fires after a confirmed source-column
comparison, and `ddl.table.row_size.max_bytes.require` resolves the facts its estimate consumes field by field —
declaration-complete statements evaluate without an instance object, and a genuinely missing
`instance.default_charset`/`instance.innodb_default_row_format` reports a `missing_instance_fact` evidence gap
(lowering coverage to unverified) rather than silently passing (issue #85 T06-A15).

### Complete List of Metadata-Backed Rule IDs

The unknown-prior-state advisory is listed because it consumes live metadata
when available, but it intentionally remains metadata-optional and emits its
bounded notice during offline audits. `ddl.table.row_size.max_bytes.require`
is listed for the same reason: instance facts are its optional fallback
source, not a connectivity prerequisite.

| Rule ID |
|---------|
| `ddl.table.row_size.max_bytes.require` |
| `ddl.index.key_length.max_bytes.require` |
| `ddl.alter.modify_column.compatibility.require` |
| `ddl.alter.change_column.compatibility.require` |
| `ddl.alter.modify_column.explicit_nullability_change.forbid` |
| `ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory` |
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
| `ddl.alter.change_column.target.exists.forbid` |
| `ddl.alter.rename_column.exists.require` |
| `ddl.alter.rename_column.target.exists.forbid` |
| `ddl.alter.rename_column.version.require` |
| `ddl.alter.add_index.exists.forbid` |
| `ddl.alter.drop_index.exists.require` |
| `ddl.alter.rename_index.exists.require` |
| `ddl.alter.drop_primary_key.exists.require` |
| `ddl.create_index.columns.exists.require` |
| `ddl.alter.alter_check.exists.require` |
| `ddl.alter.drop_check.exists.require` |
| `ddl.alter.alter_check.version.require` |
| `ddl.constraint.check.not_enforced.notice` |
| `ddl.constraint.check.ignored.notice` |
| `ddl.alter.column.dependency.require` |
| `ddl.column.advanced.compatibility.require` |
| `ddl.table.drop.exists.require` |
| `ddl.table.drop.adaptive_hash.warn` |
| `ddl.table.drop.rows.max_count` |
| `ddl.table.truncate.exists.require` |
| `ddl.table.truncate.adaptive_hash.warn` |
| `ddl.table.truncate.rows.max_count` |
| `ddl.table.denylist.forbid` |
| `dml.table.denylist.forbid` |
| `dml.table.exists.require` |

---

## Trust & Misconfiguration Guardrails (Non-Rule Behaviors)

v0.20.0 introduces additive behaviors that help identify dialect mismatches and unsupported surfaces. These are **not** configurable rules and do not have entries in the policy YAML. They cannot be disabled or re-leveled.

| Behavior | Rule-like ID | Description |
|----------|-------------|-------------|
| PostgreSQL syntax heuristic notice | `dialect.postgresql.syntax.detected.notice` | Emitted as a global advisory finding when MySQL/TiDB path auditing detects common PG-specific syntax tokens (`ON CONFLICT`, `::`, `ALTER COLUMN TYPE USING`, `GENERATED AS IDENTITY`). `RETURNING` is no longer in this list; it is valid TiDB syntax. DeltaScope does not auto-switch dialect. |
| MySQL DML RETURNING unsupported notice | `dialect.mysql.returning.unsupported.notice` | Emitted as a global advisory finding on the MySQL dialect when a parsed DML statement carries a `RETURNING` clause (the TiDB parser recognizes `INSERT`, `UPDATE`, and single-table `DELETE` `RETURNING`). MySQL Server does not support DML `RETURNING`; re-run with `--dialect tidb` if the SQL targets TiDB. TiDB dialect accepts `RETURNING` and does not fire this notice. |
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

### PostgreSQL column type destinations

`ddl.pg.alter.column_type.destination.valid` defaults to blocker. On observed
PostgreSQL 17 it checks an exact two-part TYPE or TYPE USING destination once
the ordinary table and user source column are proven. Missing schemas, types
and associated arrays, shell types and direct pseudo-types can block.
The rule also checks bounded literal modifiers for varchar, bpchar and numeric,
exact qualified collations and assignment casts between the observed scalar
types for plain TYPE. Missing applicable facts yield
`unproven_column_type_destination`. Direct view, materialized-view and stored generated-column dependencies can independently block an otherwise eligible type change. Converting existing values, evaluating USING or defaults, permissions and rewrite outcomes require other evidence.

### PostgreSQL bulk table tablespace targets

`ddl.pg.alter.move_all_tablespace.target.valid` defaults to blocker. On
observed PostgreSQL 17 it checks exact `OWNED BY` role selections in order,
then source and destination tablespace existence and the `pg_global`
restriction. Bare session aliases resolve in the observed connection. A
missing or stale observation yields `unproven_bulk_tablespace_targets`.
The notice describes a transactional request or an observed same-space
selection. Privileges, table selection, storage and current locks are
separate premises. Findings omit role names and catalog identifiers.

### PostgreSQL ordinary-table owner targets

`ddl.pg.alter.owner.target.valid` defaults to blocker. On observed PostgreSQL 17
it checks a fresh, exact, schema-qualified ordinary persistent non-typed table.
It rejects a proven missing literal role or `PUBLIC`. Quoted role names retain
their exact spelling. Bare `CURRENT_USER`, `CURRENT_ROLE` and `SESSION_USER`
resolve in the observed audit session. Missing observations yield
`unproven_table_owner_target`. The existing notice describes the request or a
proven match in the observed session. Findings omit role names. This check does
not assess ownership privileges or namespace grants.

### PostgreSQL inherited and partitioned columns

The existing DROP and RENAME source checks also use the observed PostgreSQL 17 inheritance and partition catalog. For one exact target, DROP refuses inherited columns, direct partition keys and ONLY on a partitioned parent with attached children. RENAME refuses an inherited source or ONLY when child tables must be included. A recursive rename checks for destination collisions and inherited definitions that originate outside the selected tree. The same facts distinguish a childless partitioned parent from one with attached partitions. ADD COLUMN checks direct partition targets and attached-child traversal. A proved existing ordinary user column can satisfy IF NOT EXISTS. Recursive ADD of an identity column to an ordinary inheritance parent refuses. Broader descendant and dependency outcomes require additional evidence.

### PostgreSQL SET WITHOUT OIDS

For a single SET WITHOUT OIDS action on observed PostgreSQL 17, `ddl.table.exists.alter.require` checks the exact relation kind and applies IF EXISTS to proved absence. Ordinary, partitioned and foreign tables satisfy the kind check. Views and sequences fail it. PostgreSQL retains the clause as a compatibility operation with no change to column data.

### PostgreSQL ordinary-table rename and schema destinations

`ddl.pg.alter.rename_table.target.valid` and `ddl.pg.alter.set_schema.target.valid`
are default blockers for exact, fresh ordinary persistent non-typed table
bindings on observed PostgreSQL 17. RENAME checks the destination relation
name and row-type name. PostgreSQL can displace an observed autogenerated
array type, and the independent rename prohibition still applies. SET SCHEMA
checks the destination namespace and the exact names of the table, row type,
associated array, attached indexes and column-owned sequences. The existing
notice identifies an observed same-schema request. Missing applicable facts
yield `unproven_table_namespace_destination`. Execution permissions and
locks require separate evidence.

For a single `RENAME TO` on a confirmed schema-qualified persistent PostgreSQL
17 partition root, the existing rename blocker also detects relation and row-type
name collisions. Free names and displaceable array names still leave an evidence
gap. On the same root scope, the `SET SCHEMA` blocker checks a missing schema
and collisions of the root relation, its row type, root indexes and sequences
owned by root columns. Successful moves, array-type conflicts, special
namespaces and leaf or subpartition moves remain unverified.

### PostgreSQL ALTER COLUMN TYPE source checks

`ddl.pg.alter.column_type.source.valid` defaults to blocker for a fresh,
exact, schema-qualified ordinary persistent non-typed PostgreSQL 17 table.
It rejects a proven missing source column, system source column or `USING`
on a stored generated source. Inherited source columns and direct partition keys also refuse within the exact traversal scope. Unknown evidence yields
`unproven_column_type_source`. Destination types, dependencies, conversion
expressions and row data are outside this check. Existing migration advice
continues to report possible rewrite and lock impact.

### PostgreSQL ordinary-table persistence transitions

`ddl.pg.alter.persistence.transition.valid` defaults to blocker for a fresh,
exact, schema-qualified ordinary non-typed PostgreSQL 17 table. It checks
permanent-to-unlogged requests against direct publication membership and
inbound foreign keys from other permanent tables. An unlogged-to-logged
request checks outbound foreign keys to nonpermanent tables. Unknown scope
or dependencies yield `unproven_table_persistence`. Existing notices explain
that a proven request for the current persistence leaves files in place.

On a confirmed schema-qualified persistent PostgreSQL 17 partition root,
a single `SET UNLOGGED` also rejects direct publication membership or an
inbound foreign key from another persistent table. Without either proof, the
audit retains an evidence gap. The extension does not cover `SET LOGGED`
or leaf partitions.

### PostgreSQL ordinary-table access method destinations

`ddl.pg.alter.access_method.target.valid` is a default blocker for a fresh,
exact schema-qualified ordinary persistent non-typed table on observed
PostgreSQL 17. The catalog check excludes shared and mapped storage and
rejects a missing or index access method. Bare `DEFAULT` uses the effective
server setting. A quoted name is matched exactly. Unknown scope or catalog
facts produce `unproven_table_access_method`. The existing warning identifies
proven same-method selection and explains that it leaves table files in place.

### PostgreSQL single-table tablespace destinations

`ddl.pg.alter.tablespace.target.valid` is a default blocker. For observed
PostgreSQL 17, it checks a fresh exact schema-qualified ordinary persistent
non-typed table with nonshared, nonmapped storage. A missing destination or
`pg_global` is rejected. Unbound or stale facts yield
`unproven_table_tablespace`. The effective current location uses the actual
database default for zero `reltablespace`. The existing notice identifies
proven same-space selection and describes file moves conditionally.

### PostgreSQL partition identity target refusals

For a single identity action on a catalog-confirmed, schema-qualified persistent leaf partition or `ONLY` on a persistent partition root, the existing transition blocker rejects `ADD/DROP IDENTITY`, `SET GENERATED`, decoded sequence options and `RESTART`. `DROP IDENTITY IF EXISTS` does not bypass the target restriction. Recursive root changes and sequence runtime state are not verified.

### PostgreSQL partitioned-table destination refusals

For a single action on an observed, schema-qualified persistent PostgreSQL 17 partitioned table, the existing destination blockers also reject a missing or index-only access method, a missing tablespace or `pg_global`, and a missing named owner or `PUBLIC`. A valid destination still leaves an evidence gap. These checks do not cover leaf partitions, permissions, storage movement or effects on child partitions.

### PostgreSQL heap table storage options

`ddl.pg.alter.reloptions.valid` defaults to blocker. It checks SET and RESET
on an observed PostgreSQL 17 ordinary persistent non-typed heap table with a
fresh, exact schema-qualified binding. The checks cover integer `fillfactor`
values from 10 through 100, bounded Boolean `autovacuum_enabled` declarations,
duplicate known SET keys and a RESET argument. Unvalued RESET accepts repeated
and unknown names. Unknown SET keys, TOAST and extension namespaces, opaque
numerics and unproven scope yield `unproven_table_reloptions`. The original
advice still describes the requested changes and their effects.

### PostgreSQL replica identity index attachment

`ddl.pg.alter.replica_identity_using_index.valid` is a default blocker on
observed PG17 for `ALTER TABLE ... REPLICA IDENTITY USING INDEX`. The rule
checks an exact, fresh, schema-qualified ordinary persistent non-typed table.
Native refusals include a missing index, a different relation or owner table,
a nonunique or deferred index, an expression or predicate, and inapplicable
key attributes. Nullable INCLUDE columns are outside the key check. Unknown
binding, scope or catalog facts yield `unproven_replica_identity_index`.
The existing notice describes the requested selection. PostgreSQL can assign
an invalid index, so assignment alone cannot establish replication usability.

For a single `REPLICA IDENTITY USING INDEX` on a confirmed schema-qualified persistent partition root, the blocker also rejects a missing index, a non-index relation, an index owned by another table, or a nonunique index. Other partition-root index properties and successful assignment remain unverified. This does not extend leaf-partition coverage.

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
| `ddl.pg.alter_collation.change.target.valid` | Checks qualified PostgreSQL 17 collation rename, namespace and owner premises. | blocker | Yes |
| `ddl.pg.alter_collation.refresh.target.valid` | Checks PostgreSQL 17 collation refresh identity and version presence for supported providers and locales. | blocker | Yes |
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
| `ddl.pg.alter_operator_family.member.valid` | Checks one qualified PostgreSQL 17 B-tree search-operator ADD or DROP slot. | blocker | Yes |
| `ddl.pg.alter_operator_family.change.target.valid` | Checks qualified PostgreSQL 17 operator family rename, schema and owner changes. | blocker | Yes |
| `ddl.pg.create_operator_family.target.valid` | Checks the exact PostgreSQL 17 creation schema, index method and operator family identity. | blocker | Yes |
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
| `ddl.pg.text_search.change.target.valid` | Checks PostgreSQL 17 qualified text search targets, mappings and bounded dictionary options. | blocker | Yes |
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
