# Configuration Reference

DeltaScope uses a YAML policy file to tune rule enablement, levels, and rule-specific parameters without recompiling or changing the binary.

## Config File Format

```yaml
rules:
  <rule-id>:
    enabled: true          # bool — whether this rule is active
    level: warning         # string — blocker | warning | notice
    params:
      <key>: <value>       # rule-specific parameters (see per-rule docs below)
```

### Level Meanings

| Level | Meaning |
|-------|---------|
| `blocker` | Must fix before applying SQL. Indicates a high-risk or policy-violating change. |
| `warning` | Should review before applying. Indicates a potentially risky or non-standard pattern. |
| `notice` | Informational only. No immediate action required. |

### Level and `--fail-on` Interaction

| Level | `--fail-on blocker` (default) | `--fail-on warning` | `--fail-on notice` | `--fail-on none` |
|-------|-------------------------------|---------------------|--------------------|------------------|
| `blocker` | exit 1 | exit 1 | exit 1 | exit 0 |
| `warning` | exit 0 | exit 1 | exit 1 | exit 0 |
| `notice` | exit 0 | exit 0 | exit 1 | exit 0 |

---

## Config Commands

```bash
# Generate default config file
deltascope config init > deltascope.yaml

# Validate a config file for syntax and rule ID correctness
deltascope config lint --file ./deltascope.yaml

# Show the effective status of one rule under your config
deltascope config status dml.where.require
deltascope --config ./deltascope.yaml config status dml.where.require --format json

# Print the built-in default policy to stdout
deltascope config show-default
```

`config init` and `config show-default` encode empty string params as `""`. A bare `suffix:` is YAML
null and fails `config lint`.

**`config lint` clean:**
```
Config OK
```

`config lint` also warns when a mentioned rule omits fields, because rule-level replacement turns
omitted `enabled` into `false` (see [Rule-Level Replacement Semantics](#rule-level-replacement-semantics)).
Warnings are advisory by default (exit 0); add `--strict` to fail with exit 2. For the partial override
shown below (`level: warning` with `enabled` and `params` omitted), `config lint` prints one warning per
omitted field, each handing off to `config status`:

```
Config OK with warnings

Warnings:
- dml.where.require is OFF because "enabled" is omitted.
  This config replaces the whole rule policy; it does not merge with defaults.
  Inspect effective rule status:
    deltascope config status dml.where.require --config ./deltascope.yaml
- dml.where.require removes default params because "params" is omitted.
  This config replaces the whole rule policy; it does not merge with defaults.
  Inspect effective rule status:
    deltascope config status dml.where.require --config ./deltascope.yaml
```

**`config lint` error (unknown rule ID) — errors take precedence over warnings:**
```
unknown rule "ddl.table.comments.require"
```

### config status

`deltascope config status <rule-id>` shows whether one rule is ON or OFF under the active config,
which `level` it will use if it fires, and how your config changed `enabled`, `level`, or params
versus the default. The config file is selected with the global `--config` flag.

```bash
deltascope config status dml.where.require
deltascope --config ./deltascope.yaml config status dml.where.require
deltascope --config ./deltascope.yaml config status dml.where.require --format json
```

It answers a different question from the other rule commands:

- `rules explain <rule-id>` explains what the rule means (it ignores your config).
- `config status <rule-id>` shows what your config makes the rule do.
- `config lint --file` validates a config file's shape and values.

`config status` does not run an audit, parse SQL, connect to a database, change audit or rule
behavior, change the finding JSON shape, or add a `severity` field. See [cli.md](cli.md#config-status)
for the full text and JSON output contracts.

`config status` reports effective policy exactly as the audit path applies it. That makes one
config-file behavior important to understand before you edit partial rules: rule-level replacement.

---

## Rule-Level Replacement Semantics

When you **mention** a rule in the YAML, the loader replaces that rule's whole policy — it does
**not** partial-merge the fields you wrote onto the defaults. Omitted fields become their zero
values:

| Field | Omitted effective value |
|---|---|
| `enabled` | `false` |
| `level` | `""` (empty) |
| `params` | empty |

Rules **not mentioned** in the YAML keep their default policy unchanged.

This is the same behavior the audit path applies, so `config status` reports it faithfully rather
than hiding it. The common trap is writing only the field you want to change:

```yaml
rules:
  dml.where.require:
    level: warning
```

This looks like "soften the level from `blocker` to `warning`". It is not. Because the rule is now
mentioned, its whole policy is replaced, `enabled` is omitted and therefore becomes `false`, and
the rule ends up **OFF** — it will not produce findings at all. `config status` calls this out:

```text
Current status:
  OFF
  This rule will not produce findings.

Config effect:
  Your config mentions this rule, so it replaces the default rule policy.
  `enabled` is omitted, so the effective value is false.
  `level` changes from blocker to warning.
  `params.required` is removed.
  This rule is OFF.
```

`config lint` flags this hazard before you deploy. Mentioning a rule without all of its fields makes
`config lint` print a warning for each omitted field; with `--strict` the command fails instead.
Inspect the effective result with `config status <rule-id>`. See [cli.md](cli.md#config-lint) for the
full warning list and exit-code contract.

To change only the `level` while keeping the rule on, specify every field so the replacement
leaves the others intact:

```yaml
rules:
  dml.where.require:
    enabled: true
    level: warning
    params:
      required: true
```

You do not have to remember the full field set by hand. `rules explain <rule-id>` prints a
`Safe override example:` block built from the rule's real defaults — copy it and adjust the level.
The recommended loop when editing rule overrides:

```bash
deltascope config lint --file deltascope.yaml                      # catch the hazard
deltascope rules explain dml.where.require                         # copy a safe full override
deltascope config status dml.where.require --config deltascope.yaml   # confirm the effective result
```

Whether the loader should adopt partial-merge semantics instead is a separate, larger decision and
is out of scope for this release. Until then, treat a mentioned rule as a full replacement.

---

## Rule Configuration Reference

### Structured Naming Governance

DeltaScope ships a structured naming governance model alongside the existing regex-based `pattern` rules.

- Use `*.name.pattern.require` when you want a single regex gate such as `^[A-Za-z0-9_]+$`.
- Use the naming governance rules below when you want explicit semantic checks like `prefix`, `suffix`, or `contains`.
- These two layers are complementary. Naming governance is not a replacement for `pattern`.
- `contains` uses OR semantics. Any configured token match passes the rule.
- Naming findings are emitted only for explicitly named objects. Unnamed or implicit objects are skipped.

All naming governance rules follow the same shape:

```yaml
rules:
  <rule-id>:
    enabled: true
    level: warning
    params:
      prefix: "..."
      suffix: "..."
      contains: ["...", "..."]
```

Configure only the parameter that matches the rule ID. Empty values keep the shipped rule inert.

| Target | Rule IDs |
|--------|----------|
| Table name | `ddl.table.name.prefix.require`, `ddl.table.name.suffix.require`, `ddl.table.name.contains.require` |
| Column name | `ddl.column.name.prefix.require`, `ddl.column.name.suffix.require`, `ddl.column.name.contains.require` |
| Unique index name | `ddl.index.unique.prefix.require`, `ddl.index.unique.suffix.require`, `ddl.index.unique.contains.require` |
| Secondary index name | `ddl.index.secondary.prefix.require`, `ddl.index.secondary.suffix.require`, `ddl.index.secondary.contains.require` |
| Fulltext index name | `ddl.index.fulltext.prefix.require`, `ddl.index.fulltext.suffix.require`, `ddl.index.fulltext.contains.require` |
| Primary key constraint name | `ddl.constraint.primary_key.name.prefix.require`, `ddl.constraint.primary_key.name.suffix.require`, `ddl.constraint.primary_key.name.contains.require` |
| Unique key constraint name | `ddl.constraint.unique_key.name.prefix.require`, `ddl.constraint.unique_key.name.suffix.require`, `ddl.constraint.unique_key.name.contains.require` |
| Foreign key constraint name | `ddl.constraint.foreign_key.name.prefix.require`, `ddl.constraint.foreign_key.name.suffix.require`, `ddl.constraint.foreign_key.name.contains.require` |
| Check constraint name | `ddl.constraint.check.name.prefix.require`, `ddl.constraint.check.name.suffix.require`, `ddl.constraint.check.name.contains.require` |

Representative config:

```yaml
rules:
  ddl.table.name.prefix.require:
    enabled: true
    level: warning
    params:
      prefix: "tbl_"

  ddl.column.name.suffix.require:
    enabled: true
    level: warning
    params:
      suffix: "_id"

  ddl.index.secondary.prefix.require:
    enabled: true
    level: warning
    params:
      prefix: "idx_"

  ddl.constraint.foreign_key.name.contains.require:
    enabled: true
    level: warning
    params:
      contains: ["user", "account"] # OR semantics
```

### DDL: Create Table Rules

---

#### ddl.table.comment.require

Requires `CREATE TABLE` to include a non-empty `COMMENT`.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, a missing or empty table comment triggers a finding |

**Trigger example:**
```sql
CREATE TABLE orders (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  PRIMARY KEY (id)
);
```

**Pass example:**
```sql
CREATE TABLE orders (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  PRIMARY KEY (id)
) COMMENT='order records';
```

**Config example:**
```yaml
rules:
  ddl.table.comment.require:
    enabled: true
    level: warning
    params:
      required: true
```

---

#### ddl.table.comment.max_length

Limits the maximum character length of a table `COMMENT`.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | `128` | Maximum allowed character length for a table comment |

**Trigger example:**
```sql
-- Comment exceeds 128 characters
CREATE TABLE t (id INT) COMMENT='This is a very long comment that exceeds the maximum allowed length of one hundred and twenty eight characters in total';
```

**Config example:**
```yaml
rules:
  ddl.table.comment.max_length:
    enabled: true
    level: warning
    params:
      limit: 128
```

---

#### ddl.table.name.max_length

Limits the maximum character length of a table name.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | `64` | Maximum allowed character length for a table name |

**Trigger example:**
```sql
CREATE TABLE this_is_a_very_long_table_name_that_exceeds_sixty_four_characters_total (id INT);
```

**Config example:**
```yaml
rules:
  ddl.table.name.max_length:
    enabled: true
    level: blocker
    params:
      limit: 64
```

---

#### ddl.table.name.pattern.require

Requires table names to match a configurable regex pattern.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, names not matching the pattern trigger a finding |
| `pattern` | string | `"^[A-Za-z0-9_]+$"` | Regex pattern that table names must match |

**Trigger example:**
```sql
CREATE TABLE user-data (id INT);  -- hyphen not in default pattern
```

**Config example:**
```yaml
rules:
  ddl.table.name.pattern.require:
    enabled: true
    level: blocker
    params:
      required: true
      pattern: "^[A-Za-z0-9_]+$"
```

---

#### ddl.table.name.keyword.forbid

Forbids table names that are reserved SQL keywords.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, reserved keyword names trigger a finding |

**Trigger example:**
```sql
CREATE TABLE select (id INT);
```

**Config example:**
```yaml
rules:
  ddl.table.name.keyword.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### ddl.table.engine.allowlist

Requires the storage engine to be in a configured allowlist.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `values` | []string | `["InnoDB"]` | Allowed storage engine names |

**Trigger example:**
```sql
CREATE TABLE t (id INT) ENGINE=MyISAM;
```

**Pass example:**
```sql
CREATE TABLE t (id INT) ENGINE=InnoDB;
```

**Config example:**
```yaml
rules:
  ddl.table.engine.allowlist:
    enabled: true
    level: blocker
    params:
      values: [InnoDB]
```

---

#### ddl.table.charset.allowlist

Requires the table-level default character set to be in a configured allowlist (when explicitly set).

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `values` | []string | `["utf8", "utf8mb4"]` | Allowed character set names |

**Trigger example:**
```sql
CREATE TABLE t (id INT) DEFAULT CHARSET=latin1;
```

**Config example:**
```yaml
rules:
  ddl.table.charset.allowlist:
    enabled: true
    level: blocker
    params:
      values: [utf8, utf8mb4]
```

---

#### ddl.table.row_format.allowlist

Requires the table `ROW_FORMAT` to be in a configured allowlist.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `values` | []string | `["DYNAMIC"]` | Allowed ROW_FORMAT values |
| `require_explicit` | bool | `false` | When `true`, fires when ROW_FORMAT is omitted entirely |

**Trigger example:**
```sql
CREATE TABLE t (id INT) ROW_FORMAT=COMPACT;
```

**Config example:**
```yaml
rules:
  ddl.table.row_format.allowlist:
    enabled: true
    level: blocker
    params:
      values: [DYNAMIC]
      require_explicit: false
```

---

#### ddl.table.auto_increment.init_value.require

Requires the `AUTO_INCREMENT` initial value to match a configured value when explicitly set.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `value` | int | `1` | Required AUTO_INCREMENT starting value |

**Trigger example:**
```sql
CREATE TABLE t (id INT AUTO_INCREMENT, PRIMARY KEY (id)) AUTO_INCREMENT=1000;
```

**Config example:**
```yaml
rules:
  ddl.table.auto_increment.init_value.require:
    enabled: true
    level: blocker
    params:
      value: 1
```

---

#### ddl.table.columns.min_count

Requires `CREATE TABLE` to define at least a minimum number of columns.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | `1` | Minimum number of columns required |

**Trigger example:**
```sql
CREATE TABLE t ();
```

**Config example:**
```yaml
rules:
  ddl.table.columns.min_count:
    enabled: true
    level: blocker
    params:
      limit: 1
```

---

#### ddl.table.primary_key.require

Requires every `CREATE TABLE` to include a `PRIMARY KEY` constraint.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, tables without a PRIMARY KEY trigger a finding |

**Trigger example:**
```sql
CREATE TABLE users (id BIGINT, name VARCHAR(50));
```

**Config example:**
```yaml
rules:
  ddl.table.primary_key.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

#### ddl.table.primary_key.columns.max_count

Limits the number of columns in a `PRIMARY KEY`.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | `1` | Maximum allowed columns in the PRIMARY KEY |

**Trigger example:**
```sql
CREATE TABLE t (a INT, b INT, PRIMARY KEY (a, b));
```

**Config example:**
```yaml
rules:
  ddl.table.primary_key.columns.max_count:
    enabled: true
    level: warning
    params:
      limit: 1
```

---

#### ddl.table.primary_key.bigint.require

Requires the `PRIMARY KEY` column to be of type `BIGINT`.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, non-BIGINT primary keys trigger a finding |

**Trigger example:**
```sql
CREATE TABLE t (id INT UNSIGNED NOT NULL AUTO_INCREMENT, PRIMARY KEY (id));
```

**Config example:**
```yaml
rules:
  ddl.table.primary_key.bigint.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

#### ddl.table.primary_key.unsigned.require

Requires the `PRIMARY KEY` column to be `UNSIGNED`.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, non-UNSIGNED primary key columns trigger a finding |

**Trigger example:**
```sql
CREATE TABLE t (id BIGINT NOT NULL AUTO_INCREMENT, PRIMARY KEY (id));
```

**Config example:**
```yaml
rules:
  ddl.table.primary_key.unsigned.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

#### ddl.table.primary_key.auto_increment.require

Requires the `PRIMARY KEY` column to have `AUTO_INCREMENT`.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, primary key columns without AUTO_INCREMENT trigger a finding |

**Trigger example:**
```sql
CREATE TABLE t (id BIGINT UNSIGNED NOT NULL, PRIMARY KEY (id));
```

**Config example:**
```yaml
rules:
  ddl.table.primary_key.auto_increment.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

#### ddl.table.primary_key.not_null.require

Requires the `PRIMARY KEY` column to be explicitly `NOT NULL`.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, nullable primary key columns trigger a finding |

**Trigger example:**
```sql
CREATE TABLE t (id BIGINT UNSIGNED AUTO_INCREMENT, PRIMARY KEY (id));
```

**Config example:**
```yaml
rules:
  ddl.table.primary_key.not_null.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

#### ddl.table.audit_columns.require

Requires `CREATE TABLE` to include standard audit timestamp columns (`created_at`, `updated_at`).

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, tables missing audit timestamp columns trigger a finding |

**Trigger example:**
```sql
CREATE TABLE orders (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  PRIMARY KEY (id)
) COMMENT='orders';
```

**Config example:**
```yaml
rules:
  ddl.table.audit_columns.require:
    enabled: true
    level: warning
    params:
      required: true
```

---

#### ddl.table.foreign_key.forbid

Forbids `FOREIGN KEY` constraints in `CREATE TABLE`.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, FOREIGN KEY constraints trigger a finding |

**Trigger example:**
```sql
CREATE TABLE orders (id INT, user_id INT, FOREIGN KEY (user_id) REFERENCES users(id));
```

**Config example:**
```yaml
rules:
  ddl.table.foreign_key.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### ddl.table.partition.forbid

Forbids `PARTITION BY` in `CREATE TABLE`.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, partitioned table definitions trigger a finding |

**Trigger example:**
```sql
CREATE TABLE t (id INT, created_at DATE)
PARTITION BY RANGE (YEAR(created_at)) (
  PARTITION p2024 VALUES LESS THAN (2025)
);
```

**Config example:**
```yaml
rules:
  ddl.table.partition.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### ddl.table.create_like.forbid

Forbids `CREATE TABLE ... LIKE ...`.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, CREATE TABLE ... LIKE triggers a finding |

**Trigger example:**
```sql
CREATE TABLE t_new LIKE t_old;
```

**Config example:**
```yaml
rules:
  ddl.table.create_like.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### ddl.table.create_as.forbid

Forbids `CREATE TABLE ... AS SELECT ...`.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, CREATE TABLE ... AS SELECT triggers a finding |

**Trigger example:**
```sql
CREATE TABLE t_copy AS SELECT * FROM t_original;
```

**Config example:**
```yaml
rules:
  ddl.table.create_as.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### ddl.table.row_size.max_bytes.require

> **Metadata-aware mode only.** This rule no-ops during offline audits. It activates when a live database connection is provided.

Checks that the estimated row size does not exceed InnoDB row size limits, based on instance facts loaded from the connected database.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, rows estimated to exceed the limit trigger a finding |

**Config example:**
```yaml
rules:
  ddl.table.row_size.max_bytes.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

#### ddl.table.denylist.forbid

Blocks DDL operations on specific schemas, tables, or schema.table pairs.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `schemas` | []string | `[]` | Block ALL DDL on any table in these schema names |
| `tables` | []string | `[]` | Block DDL on these unqualified table names (in any schema) |
| `qualified_tables` | []string | `[]` | Block DDL on specific `schema.table` pairs |

**Config example:**
```yaml
rules:
  ddl.table.denylist.forbid:
    enabled: true
    level: blocker
    params:
      schemas: []
      tables: [audit_log, system_config]
      qualified_tables: [prod.payments]
```

---

#### ddl.view.create.forbid

Forbids `CREATE VIEW` statements.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, CREATE VIEW triggers a finding |

**Trigger example:**
```sql
CREATE VIEW active_users AS SELECT * FROM users WHERE status = 'active';
```

**Config example:**
```yaml
rules:
  ddl.view.create.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

### DDL: Column Rules (in CREATE TABLE)

---

#### ddl.column.name.max_length

Limits the maximum character length of a column name.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | `64` | Maximum allowed character length for a column name |

**Trigger example:**
```sql
CREATE TABLE t (this_is_a_very_long_column_name_that_exceeds_the_configured_limit INT);
```

**Config example:**
```yaml
rules:
  ddl.column.name.max_length:
    enabled: true
    level: blocker
    params:
      limit: 64
```

---

#### ddl.column.name.pattern.require

Requires column names to match a configurable regex pattern.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, names not matching the pattern trigger a finding |
| `pattern` | string | `"^[A-Za-z0-9_]+$"` | Regex pattern that column names must match |

**Trigger example:**
```sql
CREATE TABLE t (user-name VARCHAR(50));  -- hyphen not in default pattern
```

**Config example:**
```yaml
rules:
  ddl.column.name.pattern.require:
    enabled: true
    level: blocker
    params:
      required: true
      pattern: "^[A-Za-z0-9_]+$"
```

---

#### ddl.column.name.keyword.forbid

Forbids column names that are reserved SQL keywords.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, reserved keyword column names trigger a finding |

**Trigger example:**
```sql
CREATE TABLE t (select INT, from VARCHAR(50));
```

**Config example:**
```yaml
rules:
  ddl.column.name.keyword.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### ddl.column.comment.require

Requires every column to have a non-empty `COMMENT`.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, columns without a non-empty comment trigger a finding |

**Trigger example:**
```sql
CREATE TABLE t (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name VARCHAR(50) NOT NULL COMMENT 'user name',
  PRIMARY KEY (id)
) COMMENT='users';
-- Finding: column `id` has no comment
```

**Config example:**
```yaml
rules:
  ddl.column.comment.require:
    enabled: true
    level: warning
    params:
      required: true
```

---

#### ddl.column.default.require

Requires non-AUTO_INCREMENT columns to have an explicit `DEFAULT` value.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, columns without a DEFAULT trigger a finding (AUTO_INCREMENT columns are exempt) |

**Trigger example:**
```sql
CREATE TABLE t (id INT, name VARCHAR(50) NOT NULL);
-- Finding: column `name` has no DEFAULT value
```

**Config example:**
```yaml
rules:
  ddl.column.default.require:
    enabled: true
    level: warning
    params:
      required: true
```

---

#### ddl.column.not_null.require

Requires columns to be defined as `NOT NULL`.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, nullable columns trigger a finding |
| `allow_time_null` | bool | `true` | When `true`, TIMESTAMP and DATETIME columns are exempt from the NOT NULL requirement |

**Trigger example:**
```sql
CREATE TABLE t (id INT, description TEXT);
-- Finding: column `description` is nullable (missing NOT NULL)
```

**Config example:**
```yaml
rules:
  ddl.column.not_null.require:
    enabled: true
    level: warning
    params:
      required: true
      allow_time_null: true
```

---

#### ddl.column.varchar.max_length

Limits the maximum length of `VARCHAR` columns.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | `16383` | Maximum allowed VARCHAR length |

**Trigger example:**
```sql
CREATE TABLE t (id INT, data VARCHAR(20000));
```

**Config example:**
```yaml
rules:
  ddl.column.varchar.max_length:
    enabled: true
    level: blocker
    params:
      limit: 16383
```

---

#### ddl.column.char.max_length

Warns when `CHAR` columns exceed a configured length.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | `64` | Maximum recommended CHAR length |

**Trigger example:**
```sql
CREATE TABLE t (id INT, code CHAR(100));
```

**Config example:**
```yaml
rules:
  ddl.column.char.max_length:
    enabled: true
    level: warning
    params:
      limit: 64
```

---

#### ddl.column.float_double.forbid

Discourages use of `FLOAT` and `DOUBLE` column types (imprecise for monetary and scientific values).

**Default:** `enabled: true`, `level: warning`, `forbid: true`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, FLOAT or DOUBLE column types trigger a finding |

**Trigger example:**
```sql
CREATE TABLE t (id INT, price FLOAT, ratio DOUBLE);
```

**Config example:**
```yaml
rules:
  ddl.column.float_double.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

#### ddl.column.blob_text.forbid

Governs use of `BLOB` and `TEXT` family column types. By default `forbid` is `false` — the rule is registered but does not produce findings. Set `forbid: true` to enable enforcement.

**Default:** `enabled: true`, `level: warning`, `forbid: false`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `false` | When `true`, BLOB/TEXT/TINYBLOB/TINYTEXT/MEDIUMBLOB/MEDIUMTEXT/LONGBLOB/LONGTEXT columns trigger a finding |

**Config example (to enable):**
```yaml
rules:
  ddl.column.blob_text.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

#### ddl.column.json.forbid

Governs use of `JSON` column type. By default `forbid` is `false` — the rule is registered but does not produce findings.

**Default:** `enabled: true`, `level: warning`, `forbid: false`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `false` | When `true`, JSON column types trigger a finding |

**Config example (to enable):**
```yaml
rules:
  ddl.column.json.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

#### ddl.column.bit.forbid

Governs use of `BIT` column type. By default `forbid` is `false` — the rule is registered but does not produce findings.

**Default:** `enabled: true`, `level: warning`, `forbid: false`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `false` | When `true`, BIT column types trigger a finding |

**Config example (to enable):**
```yaml
rules:
  ddl.column.bit.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

#### ddl.column.timestamp.forbid

Forbids `TIMESTAMP` column type. Prefer `DATETIME` for portability and range.

**Default:** `enabled: true`, `level: warning`, `forbid: true`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, TIMESTAMP column types trigger a finding |

**Trigger example:**
```sql
CREATE TABLE t (id INT, created_at TIMESTAMP);
```

**Config example:**
```yaml
rules:
  ddl.column.timestamp.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

#### ddl.column.charset.allowlist

Requires column-level `CHARACTER SET` to be in a configured allowlist.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `values` | []string | `["utf8", "utf8mb4"]` | Allowed character set names |

**Trigger example:**
```sql
CREATE TABLE t (id INT, name VARCHAR(50) CHARACTER SET latin1);
```

**Config example:**
```yaml
rules:
  ddl.column.charset.allowlist:
    enabled: true
    level: blocker
    params:
      values: [utf8, utf8mb4]
```

---

#### ddl.column.collation.allowlist

Requires column-level `COLLATE` to be in a configured allowlist.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `values` | []string | `["utf8_general_ci", "utf8mb4_general_ci", "utf8mb4_bin"]` | Allowed collation names |

**Trigger example:**
```sql
CREATE TABLE t (id INT, name VARCHAR(50) COLLATE latin1_swedish_ci);
```

**Config example:**
```yaml
rules:
  ddl.column.collation.allowlist:
    enabled: true
    level: blocker
    params:
      values: [utf8_general_ci, utf8mb4_general_ci, utf8mb4_bin]
```

---

#### ddl.column.charset_collation.match.require

Requires that a column's charset and collation are compatible with each other.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, incompatible charset/collation pairs trigger a finding |

**Trigger example:**
```sql
CREATE TABLE t (id INT, name VARCHAR(50) CHARACTER SET utf8mb4 COLLATE utf8_general_ci);
-- utf8_general_ci is for utf8, not utf8mb4
```

**Config example:**
```yaml
rules:
  ddl.column.charset_collation.match.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

### DDL: Index Rules (in CREATE TABLE)

---

#### ddl.index.total.max_count

Limits the total number of indexes (excluding the primary key) in a `CREATE TABLE`.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | `12` | Maximum allowed number of secondary indexes |

**Config example:**
```yaml
rules:
  ddl.index.total.max_count:
    enabled: true
    level: warning
    params:
      limit: 12
```

---

#### ddl.index.columns.max_count

Limits the number of columns in a single index.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | `8` | Maximum allowed columns per index |

**Trigger example:**
```sql
CREATE TABLE t (a INT, b INT, c INT, d INT, e INT, f INT, g INT, h INT, i INT,
  INDEX idx_many (a,b,c,d,e,f,g,h,i));
```

**Config example:**
```yaml
rules:
  ddl.index.columns.max_count:
    enabled: true
    level: warning
    params:
      limit: 8
```

---

#### ddl.index.name.pattern.require

Requires index names to match a configurable regex pattern.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, names not matching the pattern trigger a finding |
| `pattern` | string | `"^[A-Za-z0-9_]+$"` | Regex pattern that index names must match |

**Trigger example:**
```sql
CREATE TABLE t (id INT, name VARCHAR(50), INDEX idx-name (name));  -- hyphen not allowed
```

**Config example:**
```yaml
rules:
  ddl.index.name.pattern.require:
    enabled: true
    level: blocker
    params:
      required: true
      pattern: "^[A-Za-z0-9_]+$"
```

---

#### ddl.index.name.keyword.forbid

Forbids index names that are reserved SQL keywords.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, reserved keyword index names trigger a finding |

**Config example:**
```yaml
rules:
  ddl.index.name.keyword.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### ddl.index.unique.prefix.require

Requires `UNIQUE INDEX` names to start with a configured prefix.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, UNIQUE INDEX names not starting with the prefix trigger a finding |
| `prefix` | string | `"uniq_"` | Required prefix for UNIQUE INDEX names |

**Trigger example:**
```sql
CREATE TABLE t (id INT, email VARCHAR(100), UNIQUE INDEX user_email (email));
-- Should be: UNIQUE INDEX uniq_user_email (email)
```

**Config example:**
```yaml
rules:
  ddl.index.unique.prefix.require:
    enabled: true
    level: warning
    params:
      required: true
      prefix: "uniq_"
```

---

#### ddl.index.secondary.prefix.require

Requires regular (non-unique, non-fulltext) `INDEX` names to start with a configured prefix.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, INDEX names not starting with the prefix trigger a finding |
| `prefix` | string | `"idx_"` | Required prefix for INDEX names |

**Trigger example:**
```sql
CREATE TABLE t (id INT, email VARCHAR(100), INDEX email (email));
-- Should be: INDEX idx_email (email)
```

**Config example:**
```yaml
rules:
  ddl.index.secondary.prefix.require:
    enabled: true
    level: warning
    params:
      required: true
      prefix: "idx_"
```

---

#### ddl.index.fulltext.prefix.require

Requires `FULLTEXT INDEX` names to start with a configured prefix.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, FULLTEXT INDEX names not starting with the prefix trigger a finding |
| `prefix` | string | `"full_"` | Required prefix for FULLTEXT INDEX names |

**Trigger example:**
```sql
CREATE TABLE t (id INT, body TEXT, FULLTEXT INDEX description (body));
-- Should be: FULLTEXT INDEX full_description (body)
```

**Config example:**
```yaml
rules:
  ddl.index.fulltext.prefix.require:
    enabled: true
    level: warning
    params:
      required: true
      prefix: "full_"
```

---

#### ddl.index.duplicate.forbid

Forbids duplicate indexes — two indexes in the same table that cover exactly the same columns in the same order.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, duplicate indexes trigger a finding |

**Trigger example:**
```sql
CREATE TABLE t (a INT, INDEX idx_a1 (a), INDEX idx_a2 (a));
```

**Config example:**
```yaml
rules:
  ddl.index.duplicate.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

#### ddl.index.redundant_left_prefix.forbid

Forbids indexes that are left-prefix subsets of another index in the same table (the longer index already covers the shorter one).

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, redundant left-prefix indexes trigger a finding |

**Trigger example:**
```sql
CREATE TABLE t (a INT, b INT, INDEX idx_a (a), INDEX idx_ab (a, b));
-- idx_a is redundant — idx_ab already covers queries on (a)
```

**Config example:**
```yaml
rules:
  ddl.index.redundant_left_prefix.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

#### ddl.index.redundant_unique_overlap.forbid

Forbids `UNIQUE` indexes that are made redundant by another `UNIQUE` index covering a superset of columns.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, redundant unique indexes trigger a finding |

**Trigger example:**
```sql
CREATE TABLE t (a INT, b INT, UNIQUE uniq_a (a), UNIQUE uniq_ab (a, b));
-- uniq_a is redundant given uniq_ab
```

**Config example:**
```yaml
rules:
  ddl.index.redundant_unique_overlap.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

#### ddl.index.key_length.max_bytes.require

> **Metadata-aware mode only.** This rule no-ops during offline audits. It activates when a live database connection is provided.

Checks that the estimated index key length does not exceed InnoDB limits, based on instance facts (charset, `innodb_large_prefix`) loaded from the connected database.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, indexes estimated to exceed key length limits trigger a finding |

**Config example:**
```yaml
rules:
  ddl.index.key_length.max_bytes.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

### DDL: Alter Table Rules

---

#### ddl.alter.drop_column.forbid

Governs `ALTER TABLE ... DROP COLUMN`. By default `forbid` is `false` — the rule is registered but does not produce findings. Set `forbid: true` to enable enforcement.

**Default:** `enabled: true`, `level: warning`, `forbid: false`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `false` | When `true`, DROP COLUMN triggers a finding |

**Config example (to enable):**
```yaml
rules:
  ddl.alter.drop_column.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### ddl.alter.drop_index.forbid

Governs `ALTER TABLE ... DROP INDEX`. By default `forbid` is `false` — the rule is registered but does not produce findings.

**Default:** `enabled: true`, `level: warning`, `forbid: false`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `false` | When `true`, DROP INDEX triggers a finding |

**Config example (to enable):**
```yaml
rules:
  ddl.alter.drop_index.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

#### ddl.alter.drop_primary_key.forbid

Forbids `ALTER TABLE ... DROP PRIMARY KEY`.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, DROP PRIMARY KEY triggers a finding |

**Trigger example:**
```sql
ALTER TABLE users DROP PRIMARY KEY;
```

**Config example:**
```yaml
rules:
  ddl.alter.drop_primary_key.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### ddl.alter.rename_table.forbid

Forbids `ALTER TABLE ... RENAME TO ...` and `RENAME TABLE ...`.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, RENAME TABLE operations trigger a finding |

**Trigger example:**
```sql
ALTER TABLE users RENAME TO accounts;
```

**Config example:**
```yaml
rules:
  ddl.alter.rename_table.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### ddl.alter.rename_column.forbid

Forbids `ALTER TABLE ... RENAME COLUMN ... TO ...`.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, RENAME COLUMN triggers a finding |

**Trigger example:**
```sql
ALTER TABLE users RENAME COLUMN name TO full_name;
```

**Config example:**
```yaml
rules:
  ddl.alter.rename_column.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### ddl.alter.rename_index.forbid

Forbids `ALTER TABLE ... RENAME INDEX ... TO ...`.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, RENAME INDEX triggers a finding |

**Trigger example:**
```sql
ALTER TABLE users RENAME INDEX idx_email TO idx_user_email;
```

**Config example:**
```yaml
rules:
  ddl.alter.rename_index.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### ddl.alter.change_column.forbid

Forbids `ALTER TABLE ... CHANGE COLUMN ...` (which can simultaneously rename and retype a column).

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, CHANGE COLUMN triggers a finding |

**Trigger example:**
```sql
ALTER TABLE users CHANGE COLUMN name full_name VARCHAR(100) NOT NULL;
```

**Config example:**
```yaml
rules:
  ddl.alter.change_column.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### ddl.alter.modify_column.forbid

Governs `ALTER TABLE ... MODIFY COLUMN ...`. By default `forbid` is `false` — the rule is registered but does not produce findings.

**Default:** `enabled: true`, `level: warning`, `forbid: false`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `false` | When `true`, MODIFY COLUMN triggers a finding |

**Config example (to enable):**
```yaml
rules:
  ddl.alter.modify_column.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

#### ddl.alter.add_index.columns.max_count

Limits the number of columns in an index added via `ALTER TABLE ... ADD INDEX`.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | `8` | Maximum allowed columns per added index |

**Trigger example:**
```sql
ALTER TABLE t ADD INDEX idx_many (a, b, c, d, e, f, g, h, i);
```

**Config example:**
```yaml
rules:
  ddl.alter.add_index.columns.max_count:
    enabled: true
    level: warning
    params:
      limit: 8
```

---

#### ddl.alter.add_index.duplicate.forbid

Forbids `ADD INDEX` that duplicates an existing index (same columns, same order). Requires metadata-aware mode to compare against existing indexes.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, duplicate ADD INDEX triggers a finding |

**Config example:**
```yaml
rules:
  ddl.alter.add_index.duplicate.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

#### ddl.alter.add_index.redundant_left_prefix.forbid

Forbids `ADD INDEX` that creates an index which is a left-prefix of an existing index. Requires metadata-aware mode.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, redundant left-prefix ADD INDEX triggers a finding |

**Config example:**
```yaml
rules:
  ddl.alter.add_index.redundant_left_prefix.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

#### ddl.alter.add_index.redundant_unique_overlap.forbid

Forbids `ADD UNIQUE INDEX` that is made redundant by an existing UNIQUE index. Requires metadata-aware mode.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, redundant unique ADD INDEX triggers a finding |

**Config example:**
```yaml
rules:
  ddl.alter.add_index.redundant_unique_overlap.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

#### ddl.alter.add_index.unique.prefix.require

Requires `ADD UNIQUE INDEX` names to start with a configured prefix.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, UNIQUE INDEX names not starting with the prefix trigger a finding |
| `prefix` | string | `"uniq_"` | Required prefix |

**Trigger example:**
```sql
ALTER TABLE t ADD UNIQUE INDEX email (email);  -- Should be uniq_email
```

**Config example:**
```yaml
rules:
  ddl.alter.add_index.unique.prefix.require:
    enabled: true
    level: warning
    params:
      required: true
      prefix: "uniq_"
```

---

#### ddl.alter.add_index.secondary.prefix.require

Requires `ADD INDEX` names to start with a configured prefix.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, INDEX names not starting with the prefix trigger a finding |
| `prefix` | string | `"idx_"` | Required prefix |

**Trigger example:**
```sql
ALTER TABLE t ADD INDEX email (email);  -- Should be idx_email
```

**Config example:**
```yaml
rules:
  ddl.alter.add_index.secondary.prefix.require:
    enabled: true
    level: warning
    params:
      required: true
      prefix: "idx_"
```

---

#### ddl.alter.add_index.fulltext.prefix.require

Requires `ADD FULLTEXT INDEX` names to start with a configured prefix.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, FULLTEXT INDEX names not starting with the prefix trigger a finding |
| `prefix` | string | `"full_"` | Required prefix |

**Config example:**
```yaml
rules:
  ddl.alter.add_index.fulltext.prefix.require:
    enabled: true
    level: warning
    params:
      required: true
      prefix: "full_"
```

---

#### ddl.alter.modify_column.target_type_family.allowlist

Limits the target type family when using `MODIFY COLUMN`. Prevents changing a column to exotic type families.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, target types outside the allowed families trigger a finding |
| `allowed_type_families` | []string | `["integer", "decimal", "string", "binary", "time"]` | Allowed target type families |

**Trigger example:**
```sql
-- Changing a column to ENUM or GEOMETRY type triggers this rule
ALTER TABLE t MODIFY COLUMN status ENUM('a','b');
```

**Config example:**
```yaml
rules:
  ddl.alter.modify_column.target_type_family.allowlist:
    enabled: true
    level: blocker
    params:
      required: true
      allowed_type_families: [integer, decimal, string, binary, time]
```

---

#### ddl.alter.change_column.target_type_family.allowlist

Same as `ddl.alter.modify_column.target_type_family.allowlist` but applies to `CHANGE COLUMN`.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, target types outside the allowed families trigger a finding |
| `allowed_type_families` | []string | `["integer", "decimal", "string", "binary", "time"]` | Allowed target type families |

**Config example:**
```yaml
rules:
  ddl.alter.change_column.target_type_family.allowlist:
    enabled: true
    level: blocker
    params:
      required: true
      allowed_type_families: [integer, decimal, string, binary, time]
```

---

#### ddl.alter.modify_column.compatibility.require

> **Metadata-aware mode only.** This rule no-ops during offline audits.

Checks that a `MODIFY COLUMN` type change is compatible with the current column definition (e.g., no narrowing that would truncate data, no signedness change that would corrupt values).

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, incompatible type changes trigger a finding |

**Config example:**
```yaml
rules:
  ddl.alter.modify_column.compatibility.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

#### ddl.alter.change_column.compatibility.require

> **Metadata-aware mode only.** This rule no-ops during offline audits.

Same as `ddl.alter.modify_column.compatibility.require` but applies to `CHANGE COLUMN`.

**Default:** `enabled: true`, `level: blocker`

**Config example:**
```yaml
rules:
  ddl.alter.change_column.compatibility.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

#### ddl.alter.modify_column.explicit_nullability_change.forbid

> **Metadata-aware mode only.** This rule no-ops during offline audits.

Forbids `MODIFY COLUMN` that explicitly changes nullability (e.g., adding or removing `NOT NULL`) compared to a confirmed current column definition. A restated `NULL` or `NOT NULL` clause is allowed when it matches the live column; unknown prior state is reported by `ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory` instead.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, explicit nullability changes trigger a finding |

**Config example:**
```yaml
rules:
  ddl.alter.modify_column.explicit_nullability_change.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory

> **Offline and metadata-aware fallback.** This rule is non-blocking.

Emits a notice when `MODIFY COLUMN` explicitly states `NULL` or `NOT NULL` but the prior column nullability cannot be confirmed. It never claims that a transition occurred. With confirmed live metadata, the notice is suppressed and the existing `...explicit_nullability_change.forbid` rule evaluates only a real transition.

**Default:** `enabled: true`, `level: notice`

**Parameters:** none.

**Example:**
```sql
ALTER TABLE users MODIFY COLUMN email VARCHAR(320) NOT NULL;
```

**Config example:**
```yaml
rules:
  ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory:
    enabled: true
    level: notice
    params:
```

---

#### ddl.alter.change_column.explicit_nullability_change.forbid

> **Metadata-aware mode only.** This rule no-ops during offline audits.

Same as `ddl.alter.modify_column.explicit_nullability_change.forbid` but applies to `CHANGE COLUMN`.

**Default:** `enabled: true`, `level: blocker`

**Config example:**
```yaml
rules:
  ddl.alter.change_column.explicit_nullability_change.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### ddl.alter.modify_column.explicit_default_change.forbid

> **Metadata-aware mode only.** This rule no-ops during offline audits.

Forbids `MODIFY COLUMN` that explicitly changes the `DEFAULT` value compared to the current column definition.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, explicit DEFAULT changes trigger a finding |

**Config example:**
```yaml
rules:
  ddl.alter.modify_column.explicit_default_change.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### ddl.alter.change_column.explicit_default_change.forbid

> **Metadata-aware mode only.** This rule no-ops during offline audits.

Same as `ddl.alter.modify_column.explicit_default_change.forbid` but applies to `CHANGE COLUMN`.

**Default:** `enabled: true`, `level: blocker`

**Config example:**
```yaml
rules:
  ddl.alter.change_column.explicit_default_change.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### ddl.alter.modify_column.explicit_auto_increment_change.forbid

> **Metadata-aware mode only.** This rule no-ops during offline audits.

Forbids `MODIFY COLUMN` that adds or removes `AUTO_INCREMENT` compared to the current column definition.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, AUTO_INCREMENT changes trigger a finding |

**Config example:**
```yaml
rules:
  ddl.alter.modify_column.explicit_auto_increment_change.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### ddl.alter.change_column.explicit_auto_increment_change.forbid

> **Metadata-aware mode only.** This rule no-ops during offline audits.

Same as `ddl.alter.modify_column.explicit_auto_increment_change.forbid` but applies to `CHANGE COLUMN`.

**Default:** `enabled: true`, `level: blocker`

**Config example:**
```yaml
rules:
  ddl.alter.change_column.explicit_auto_increment_change.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### ddl.alter.table_option.compatibility.require

> **Metadata-aware mode only.** This rule no-ops during offline audits.

Checks that table option changes (ENGINE, CHARSET, COLLATE, ROW_FORMAT) are compatible with the current table's option values.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, incompatible table option changes trigger a finding |

**Config example:**
```yaml
rules:
  ddl.alter.table_option.compatibility.require:
    enabled: true
    level: warning
    params:
      required: true
```

---

### DDL: Alter Table — Existence Check Rules (Metadata-Backed)

All rules in this section require metadata-aware mode. They silently no-op during offline audits.

---

#### ddl.table.exists.alter.require

> **Metadata-aware mode only.**

Fails when `ALTER TABLE` targets a table that does not exist in the connected schema.

**Default:** `enabled: true`, `level: blocker`

**Config example:**
```yaml
rules:
  ddl.table.exists.alter.require:
    enabled: true
    level: blocker
    params:    # no configurable parameters
```

---

#### ddl.table.exists.create.forbid

> **Metadata-aware mode only.**

Fails when `CREATE TABLE` (without `IF NOT EXISTS`) targets a table that already exists.

**Default:** `enabled: true`, `level: blocker`

**Config example:**
```yaml
rules:
  ddl.table.exists.create.forbid:
    enabled: true
    level: blocker
    params:    # no configurable parameters
```

---

#### ddl.alter.add_column.exists.forbid

> **Metadata-aware mode only.**

Fails when `ADD COLUMN` targets a column that already exists in the current table schema.

**Default:** `enabled: true`, `level: blocker`

**Config example:**
```yaml
rules:
  ddl.alter.add_column.exists.forbid:
    enabled: true
    level: blocker
    params:    # no configurable parameters
```

---

#### ddl.alter.drop_column.exists.require

> **Metadata-aware mode only.**

Fails when `DROP COLUMN` targets a column that does not exist in the current table schema.

**Default:** `enabled: true`, `level: blocker`

**Config example:**
```yaml
rules:
  ddl.alter.drop_column.exists.require:
    enabled: true
    level: blocker
    params:    # no configurable parameters
```

---

#### ddl.alter.modify_column.exists.require

> **Metadata-aware mode only.**

Fails when `MODIFY COLUMN` targets a column that does not exist in the current table schema.

**Default:** `enabled: true`, `level: blocker`

**Config example:**
```yaml
rules:
  ddl.alter.modify_column.exists.require:
    enabled: true
    level: blocker
    params:    # no configurable parameters
```

---

#### ddl.alter.change_column.exists.require

> **Metadata-aware mode only.**

Fails when `CHANGE COLUMN` targets a source column that does not exist in the current table schema.

**Default:** `enabled: true`, `level: blocker`

**Config example:**
```yaml
rules:
  ddl.alter.change_column.exists.require:
    enabled: true
    level: blocker
    params:    # no configurable parameters
```

---

#### ddl.alter.rename_column.exists.require

> **Metadata-aware mode only.**

Fails when `RENAME COLUMN` targets a source column that does not exist.

**Default:** `enabled: true`, `level: blocker`

**Config example:**
```yaml
rules:
  ddl.alter.rename_column.exists.require:
    enabled: true
    level: blocker
    params:    # no configurable parameters
```

---

#### ddl.alter.add_index.exists.forbid

> **Metadata-aware mode only.**

Fails when `ADD INDEX` specifies an index name that already exists on the table.

**Default:** `enabled: true`, `level: blocker`

**Config example:**
```yaml
rules:
  ddl.alter.add_index.exists.forbid:
    enabled: true
    level: blocker
    params:    # no configurable parameters
```

---

#### ddl.alter.drop_index.exists.require

> **Metadata-aware mode only.**

Fails when `DROP INDEX` targets an index that does not exist on the table.

**Default:** `enabled: true`, `level: blocker`

**Config example:**
```yaml
rules:
  ddl.alter.drop_index.exists.require:
    enabled: true
    level: blocker
    params:    # no configurable parameters
```

---

#### ddl.alter.rename_index.exists.require

> **Metadata-aware mode only.**

Fails when `RENAME INDEX` targets a source index that does not exist on the table.

**Default:** `enabled: true`, `level: blocker`

**Config example:**
```yaml
rules:
  ddl.alter.rename_index.exists.require:
    enabled: true
    level: blocker
    params:    # no configurable parameters
```

---

#### ddl.alter.drop_primary_key.exists.require

> **Metadata-aware mode only.**

Fails when `DROP PRIMARY KEY` is issued on a table that has no primary key.

**Default:** `enabled: true`, `level: blocker`

**Config example:**
```yaml
rules:
  ddl.alter.drop_primary_key.exists.require:
    enabled: true
    level: blocker
    params:    # no configurable parameters
```

---

### DDL: Object Lifecycle Rules

---

#### ddl.table.drop.forbid

Forbids `DROP TABLE` statements.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, DROP TABLE triggers a finding |

**Trigger example:**
```sql
DROP TABLE users;
```

**Config example:**
```yaml
rules:
  ddl.table.drop.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### ddl.table.drop.exists.require

> **Metadata-aware mode only.**

Fails when `DROP TABLE` (without `IF EXISTS`) targets a table that does not exist.

**Default:** `enabled: true`, `level: blocker`

**Config example:**
```yaml
rules:
  ddl.table.drop.exists.require:
    enabled: true
    level: blocker
    params:    # no configurable parameters
```

---

#### ddl.table.drop.adaptive_hash.warn

> **Metadata-aware mode only.**

Warns when `DROP TABLE` is issued while `innodb_adaptive_hash_index` is `ON`. Dropping a table with adaptive hash index enabled can cause a latch contention spike.

**Default:** `enabled: true`, `level: warning`

**Config example:**
```yaml
rules:
  ddl.table.drop.adaptive_hash.warn:
    enabled: true
    level: warning
    params:    # no configurable parameters
```

---

#### ddl.table.drop.rows.max_count

> **Metadata-aware mode only.**

Warns when `DROP TABLE` targets a table whose `table_rows` in `information_schema` exceeds a configured limit.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | `100` | Maximum allowed row count before the warning fires |

**Config example:**
```yaml
rules:
  ddl.table.drop.rows.max_count:
    enabled: true
    level: warning
    params:
      limit: 100
```

---

#### ddl.table.truncate.forbid

Forbids `TRUNCATE TABLE` statements.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, TRUNCATE TABLE triggers a finding |

**Trigger example:**
```sql
TRUNCATE TABLE users;
```

**Config example:**
```yaml
rules:
  ddl.table.truncate.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### ddl.table.truncate.exists.require

> **Metadata-aware mode only.**

Fails when `TRUNCATE TABLE` targets a table that does not exist.

**Default:** `enabled: true`, `level: blocker`

**Config example:**
```yaml
rules:
  ddl.table.truncate.exists.require:
    enabled: true
    level: blocker
    params:    # no configurable parameters
```

---

#### ddl.table.truncate.adaptive_hash.warn

> **Metadata-aware mode only.**

Warns when `TRUNCATE TABLE` is issued while `innodb_adaptive_hash_index` is `ON`.

**Default:** `enabled: true`, `level: warning`

**Config example:**
```yaml
rules:
  ddl.table.truncate.adaptive_hash.warn:
    enabled: true
    level: warning
    params:    # no configurable parameters
```

---

#### ddl.table.truncate.rows.max_count

> **Metadata-aware mode only.**

Warns when `TRUNCATE TABLE` targets a table whose `table_rows` exceeds a configured limit.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | `100` | Maximum allowed row count before the warning fires |

**Config example:**
```yaml
rules:
  ddl.table.truncate.rows.max_count:
    enabled: true
    level: warning
    params:
      limit: 100
```

---

### DDL: Global Rules (Cross-Statement)

These rules evaluate **across all statements in a batch** after statement-scoped rules complete. They detect patterns that span multiple statements.

---

#### ddl.alter.merge.mysql.require

Warns when multiple `ALTER TABLE` statements in the same batch target the same table. On MySQL, merging ALTERs into a single statement reduces the number of table rebuilds and lock durations.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, multiple ALTER TABLE on the same table trigger a finding |

**Trigger example (two statements in the same batch):**
```sql
ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email';
ALTER TABLE users ADD INDEX idx_email (email);
```

**Config example:**
```yaml
rules:
  ddl.alter.merge.mysql.require:
    enabled: true
    level: warning
    params:
      required: true
```

---

#### ddl.alter.merge.tidb.require

Similar to `ddl.alter.merge.mysql.require`, but targets TiDB. Disabled by default (`required: false`) because TiDB handles online DDL concurrently, making merge-alter less critical.

**Default:** `enabled: true`, `level: warning`, `required: false`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `false` | When `true`, multiple ALTER TABLE on the same table trigger a finding in TiDB audits |

**Config example (to enable for TiDB):**
```yaml
rules:
  ddl.alter.merge.tidb.require:
    enabled: true
    level: warning
    params:
      required: true
```

---

### DML Rules

---

#### dml.where.require

Requires `UPDATE` and `DELETE` statements to include a `WHERE` clause.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, UPDATE/DELETE without WHERE triggers a finding |

**Trigger example:**
```sql
UPDATE users SET status = 'inactive';
DELETE FROM orders;
```

**Config example:**
```yaml
rules:
  dml.where.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

#### dml.limit.forbid

Warns when `UPDATE` or `DELETE` includes a `LIMIT` clause (non-deterministic behavior; can produce different results across replicas).

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, LIMIT in UPDATE/DELETE triggers a finding |

**Trigger example:**
```sql
DELETE FROM users WHERE status = 'inactive' LIMIT 100;
```

**Config example:**
```yaml
rules:
  dml.limit.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

#### dml.order_by.forbid

Warns when `UPDATE` or `DELETE` includes an `ORDER BY` clause (non-deterministic behavior in replication).

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, ORDER BY in UPDATE/DELETE triggers a finding |

**Trigger example:**
```sql
DELETE FROM users ORDER BY created_at LIMIT 10;
```

**Config example:**
```yaml
rules:
  dml.order_by.forbid:
    enabled: true
    level: warning
    params:
      forbid: true
```

---

#### dml.subquery.forbid

Forbids DML statements containing subqueries (`UPDATE`/`DELETE` with nested `SELECT`, or `INSERT ... SELECT`).

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, DML with subqueries triggers a finding |

**Trigger example:**
```sql
DELETE FROM users WHERE id IN (SELECT id FROM temp_users);
```

**Config example:**
```yaml
rules:
  dml.subquery.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### dml.join.on.require

Requires `JOIN` clauses to include an `ON` condition (prevents accidental Cartesian products).

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `required` | bool | `true` | When `true`, JOIN without ON triggers a finding |

**Trigger example:**
```sql
SELECT * FROM users JOIN orders;
```

**Config example:**
```yaml
rules:
  dml.join.on.require:
    enabled: true
    level: blocker
    params:
      required: true
```

---

#### dml.insert.rows.max_count

Limits the number of rows in a single `INSERT ... VALUES (...)` statement.

**Default:** `enabled: true`, `level: warning`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | `100` | Maximum allowed rows in a single INSERT VALUES list |

**Trigger example:**
```sql
-- More than 100 value tuples in a single INSERT
INSERT INTO logs (msg) VALUES ('a'), ('b'), ..., ('z');  -- 101 rows
```

**Config example:**
```yaml
rules:
  dml.insert.rows.max_count:
    enabled: true
    level: warning
    params:
      limit: 100
```

---

#### dml.replace.forbid

Forbids `REPLACE INTO` statements (which internally perform a DELETE + INSERT and can interact poorly with triggers and foreign keys).

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, REPLACE INTO triggers a finding |

**Trigger example:**
```sql
REPLACE INTO users (id, name) VALUES (1, 'Alice');
```

**Config example:**
```yaml
rules:
  dml.replace.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### dml.insert.select.forbid

Forbids `INSERT INTO ... SELECT ...` statements (unbounded row count, can lock source table).

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, INSERT INTO ... SELECT triggers a finding |

**Trigger example:**
```sql
INSERT INTO archive SELECT * FROM users WHERE created_at < '2020-01-01';
```

**Config example:**
```yaml
rules:
  dml.insert.select.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### dml.insert.on_duplicate.forbid

Forbids `INSERT INTO ... ON DUPLICATE KEY UPDATE ...`.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `forbid` | bool | `true` | When `true`, INSERT ... ON DUPLICATE KEY UPDATE triggers a finding |

**Trigger example:**
```sql
INSERT INTO users (id, name) VALUES (1, 'Alice') ON DUPLICATE KEY UPDATE name = VALUES(name);
```

**Config example:**
```yaml
rules:
  dml.insert.on_duplicate.forbid:
    enabled: true
    level: blocker
    params:
      forbid: true
```

---

#### dml.table.denylist.forbid

Blocks DML operations on specific schemas, tables, or schema.table pairs.

**Default:** `enabled: true`, `level: blocker`

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `schemas` | []string | `[]` | Block ALL DML in these schema names |
| `tables` | []string | `[]` | Block DML on these unqualified table names (in any schema) |
| `qualified_tables` | []string | `[]` | Block DML on specific `schema.table` pairs |

**Trigger example (with `tables: [audit_log]`):**
```sql
UPDATE audit_log SET reviewed = 1 WHERE id = 1;
```

**Config example:**
```yaml
rules:
  dml.table.denylist.forbid:
    enabled: true
    level: blocker
    params:
      schemas: []
      tables: [audit_log]
      qualified_tables: []
```

---

## DDL: PostgreSQL Migration-Safety Rules

> These rules only apply when `--dialect postgresql` is set and are automatically skipped for MySQL/TiDB dialects.

### `ddl.pg.create_index.concurrently.require`

Flags `CREATE INDEX` without `CONCURRENTLY` on PostgreSQL. Non-concurrent `CREATE INDEX` holds an exclusive lock on the table, blocking reads and writes until the index build finishes.

- **Default**: enabled, level `warning`
- **Params**: none

**Trigger example:**
```sql
CREATE INDEX idx_name ON users (email);
```

**Valid example:**
```sql
CREATE INDEX CONCURRENTLY idx_name ON users (email);
```

**Config example:**
```yaml
rules:
  ddl.pg.create_index.concurrently.require:
    enabled: true
    level: warning
```

---

### `ddl.pg.alter.add_column.non_null_default.rewrite.warn`

Warns when `ALTER TABLE … ADD COLUMN … NOT NULL DEFAULT …` may trigger a full table rewrite on PostgreSQL. Adding a non-null column with a volatile default (e.g. `gen_random_uuid()`) requires PostgreSQL to rewrite every row.

- **Default**: enabled, level `warning`
- **Params**: none

**Trigger example:**
```sql
ALTER TABLE users ADD COLUMN uuid UUID NOT NULL DEFAULT gen_random_uuid();
```

**Valid example:**

Add the column as nullable first, backfill, then add the `NOT NULL` constraint in a separate migration.

**Config example:**
```yaml
rules:
  ddl.pg.alter.add_column.non_null_default.rewrite.warn:
    enabled: true
    level: warning
```

---

### `ddl.pg.alter.add_check.not_valid.require`

Flags `ALTER TABLE … ADD CHECK (…)` without `NOT VALID` on PostgreSQL. Adding a `CHECK` constraint without `NOT VALID` requires a full table scan to validate existing rows, which holds an `ACCESS EXCLUSIVE` lock.

- **Default**: enabled, level `warning`
- **Params**: none

**Trigger example:**
```sql
ALTER TABLE orders ADD CHECK (total >= 0);
```

**Valid example:**
```sql
ALTER TABLE orders ADD CHECK (total >= 0) NOT VALID;
```

**Config example:**
```yaml
rules:
  ddl.pg.alter.add_check.not_valid.require:
    enabled: true
    level: warning
```

---

### `ddl.pg.alter.set_data_type.rewrite.warn`

Warns when `ALTER TABLE … ALTER COLUMN … TYPE …` may require a full table rewrite on PostgreSQL. Certain type changes (e.g. `varchar` to `integer`) require PostgreSQL to rewrite every row.

- **Default**: enabled, level `warning`
- **Params**: none

**Trigger example:**
```sql
ALTER TABLE users ALTER COLUMN age TYPE bigint;
```

**Valid example:**

Use a three-step safe migration: add new column → backfill → drop old column.

**Config example:**
```yaml
rules:
  ddl.pg.alter.set_data_type.rewrite.warn:
    enabled: true
    level: warning
```

---

## Trust & Misconfiguration Guardrails

v0.20.0 introduces PostgreSQL trust and misconfiguration guardrails as additive engine behaviors. These are **not configurable through the policy YAML** and do not have entries in the `rules:` block:

- **PostgreSQL syntax heuristic notice** (`dialect.postgresql.syntax.detected.notice`): a global advisory finding emitted when MySQL/TiDB auditing detects PG-specific syntax tokens. This behavior is always active and cannot be disabled or re-leveled.
- **PostgreSQL capability-boundary errors**: unsupported PG surfaces return typed `PostgreSQLCapabilityBoundaryError`. This is engine behavior, not a rule.
- **Heuristic false-positive exclusion**: the PG syntax heuristic ignores tokens inside string literals, quoted identifiers, and comments. No configuration needed.
- **Trust context and rule summary visibility**: CLI output formats (json, markdown, quiet) include audit context and rule counts. These are output-layer behaviors, not rule parameters.

See [audit-capability-matrix.md](audit-capability-matrix.md) for the full capability table.

---

## PostgreSQL DDL Coverage (v0.21.0 / v0.23.0 / v0.24.0)

`v0.21.0` expands PostgreSQL DDL normalization for common migration follow-up statements. `v0.23.0` expands common PostgreSQL `CREATE TABLE` constraint coverage. `v0.24.0` deepens the semantic value of the PostgreSQL `CREATE TABLE` shapes that `v0.23.0` brought into the shared audit pipeline. These are coverage and semantics improvements that reuse existing shared rules and metadata-aware semantics — they do **not** introduce new rule configuration items.

The following PostgreSQL `ALTER TABLE` forms are now normalized through the shared audit pipeline instead of returning capability-boundary errors:

- `ALTER COLUMN ... SET DEFAULT` (action: `set_default`)
- `ALTER COLUMN ... DROP DEFAULT` (action: `drop_default`)
- `ALTER COLUMN ... SET NOT NULL` (action: `set_not_null`)
- `ALTER COLUMN ... DROP NOT NULL` (action: `drop_not_null`)
- `VALIDATE CONSTRAINT` (action: `validate_constraint`) — supported and auditable, no dedicated rule
- `DROP CONSTRAINT` (action: `drop_constraint`) — primary-key mapping applies via `ddl.alter.drop_primary_key` when metadata is available

The following PostgreSQL `CREATE TABLE` shapes are additionally supported in `v0.23.0` without adding new config keys:

- table-level named `CHECK`
- column-level inline `CHECK`
- table-level named `UNIQUE`
- column-level inline `UNIQUE`
- table-level named `FOREIGN KEY`
- column-level inline `REFERENCES`

`v0.24.0` deepens the foreign-key semantics for these create-table shapes:

- Named `FOREIGN KEY` and inline `REFERENCES` now preserve parser-owned `ReferencedTable` and `ReferencedColumns` as shared contract facts.
- These are parser-owned structural facts, not live metadata truth — they represent what the SQL statement declares, not what the database schema currently contains.

Configuration implications:

- Existing structured naming governance for `ddl.constraint.check.*`, `ddl.constraint.unique_key.*`, and `ddl.constraint.foreign_key.*` is reused when the normalized PostgreSQL create-table facts match those rule families.
- Existing shared index rules can consume index facts emitted by inline `UNIQUE`.
- Existing `ddl.table.foreign_key.forbid` continues to fire for all foreign-key forms, including inline `REFERENCES` carrying the richer semantics.
- `ReferencedTable` and `ReferencedColumns` are additive fields with `omitempty` JSON encoding — no new policy block is needed.

No changes to `configs/deltascope.example.yaml` are required for these releases.

---

## Cross-References

- Rule discovery commands: [rules.md](rules.md)
- CLI flags and exit codes: [cli.md](cli.md)
- Conceptual overview: [../concept/core-concepts.md](../concept/core-concepts.md)
- Metadata-aware mode: [../concept/metadata-aware-mode.md](../concept/metadata-aware-mode.md)
- Example config file: `configs/deltascope.example.yaml`
