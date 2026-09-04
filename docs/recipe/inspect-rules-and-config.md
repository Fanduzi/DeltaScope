# Inspect Rules and Config

Use the built-in discovery commands to understand what DeltaScope will enforce before running a large audit. These commands require no database connection and work entirely from the compiled Rule Catalog and your policy file.

`rules list` is the Rule Catalog. Audit `rule_summary.loaded` is the registered set for that run. They are different counts: Catalog includes default-disabled `dml.impact.*`, and Default Policy still lists the three `ddl.constraint.foreign_key.name.*` rules that `ddl.table.foreign_key.forbid` suppresses. `config status` names that suppression as `fk_forbid` rather than treating the rules as missing.

The recommended loop when you are about to change a rule override:

1. **Find the rule** — `deltascope rules list` (with `--kind`, `--level`, `--dialect`, or `--search`).
2. **Understand it** — `deltascope rules explain <rule-id>` for the default policy and a copyable
   `Safe override example:`.
3. **Lint your config** — `deltascope config lint --file deltascope.yaml` catches rule-level replacement
   hazards before you deploy.
4. **Confirm the effective result** — `deltascope config status <rule-id> --config deltascope.yaml` shows
   whether the rule is ON or OFF under your file.

## Discovering Rules

### List all rules

```bash
deltascope rules list
```

Output example (truncated — your build may include more rules):

```
RULE ID                                    LEVEL    DIALECT  KIND  CATEGORY
dml.where.require                          blocker  common   dml   dml_safety
dml.limit.forbid                           warning  common   dml   dml_safety
dml.subquery.forbid                        blocker  common   dml   dml_safety
dml.join.on.require                        blocker  common   dml   dml_safety
dml.insert.rows.max_count                  warning  common   dml   dml_safety
dml.order_by.forbid                        warning  common   dml   dml_safety
ddl.table.comment.require                  warning  common   ddl   table
ddl.table.name.max_length                  blocker  common   ddl   table
ddl.column.comment.require                 warning  common   ddl   column
ddl.alter.drop_column.forbid               warning  common   ddl   alter_table
...
```

`DIALECT` is `common` (applies to MySQL, TiDB, and PostgreSQL) or a specific dialect such as
`mysql` or `postgresql`. Some rules need a live database connection to evaluate; those are
documented in [Audit SQL with metadata](audit-sql-with-metadata.md) and run as no-ops offline.

### Filter by kind or level

```bash
# Only DML rules
deltascope rules list --kind dml

# Only DDL rules at blocker level
deltascope rules list --kind ddl --level blocker

# All warning-level rules (any kind)
deltascope rules list --level warning

# Rules scoped to one dialect
deltascope rules list --dialect postgresql

# Rules whose ID or metadata mentions a keyword
deltascope rules list --search drop
```

### Show rule details

```bash
deltascope rules explain dml.where.require
```

Output:

```
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

A DDL rule with a numeric parameter:

```bash
deltascope rules explain ddl.table.name.max_length
```

Output (tail):

```
Default Params:
  limit: 64

Default policy:
  rules:
    ddl.table.name.max_length:
      enabled: true
      level: blocker
      params:
        limit: 64

Safe override example:
  rules:
    ddl.table.name.max_length:
      enabled: true
      level: warning
      params:
        limit: 64

Inspect effective rule status:
  deltascope config status ddl.table.name.max_length --config deltascope.yaml
```

`rules explain` reads only the shipped catalog — it does not look at your config. To see what your
config makes a rule do, use `config status <rule-id>` (see [Managing Config](#validate-a-config-file)
below).

### Search rules by keyword

```bash
# Find rules whose ID or metadata mentions a keyword
deltascope rules list --search drop
```

Output (truncated):

```
RULE ID                                  LEVEL    DIALECT  KIND  CATEGORY
ddl.alter.drop_column.exists.require     blocker  common   ddl   alter_table
ddl.alter.drop_column.forbid             warning  common   ddl   alter_table
ddl.alter.drop_index.forbid              warning  common   ddl   alter_table
ddl.table.drop.forbid                    blocker  common   ddl   table
...
```

## Managing Config

### Generate default config

Emit the full default policy as a YAML file you can check into your repository:

```bash
deltascope config init > deltascope.yaml
```

This produces a YAML file with every rule listed, its default `enabled` state, `level`, and any `params`. Empty string params are written as `""` so `config lint` accepts the file. Use it as the starting point for your team's policy.

### Validate a config file

Before deploying a modified config, lint it to catch typos in rule IDs, invalid parameter types,
and rule-level replacement hazards:

```bash
deltascope config lint --file ./deltascope.yaml
```

A clean file prints `Config OK` and exits 0:

```
Config OK
```

A valid file that mentions a rule without all of its fields prints a warning per omitted field and
still exits 0. This is the same replacement hazard shown below in [Common Config Tasks](#disable-a-rule):
mentioning a rule replaces its whole policy, so an omitted `enabled` turns the rule OFF.

```yaml
rules:
  dml.where.require:
    level: warning
```

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

Add `--strict` to fail (exit 2) when warnings are present, which is what you want in CI:

```bash
deltascope config lint --file ./deltascope.yaml --strict
```

Validation errors print to stderr and exit 2, and take precedence over warnings:

```
unknown rule "ddl.table.comments.require"
invalid level "critical" for rule "ddl.column.comment.require"
invalid type for dml.insert.rows.max_count.limit: got string, want int
```

`config lint` has no JSON output. To confirm the effective state a warned rule lands in, follow up
with `config status`:

```bash
deltascope --config ./deltascope.yaml config status dml.where.require
```

```text
Current status:
  OFF
  This rule will not produce findings.

Config effect:
  Your config mentions this rule, so it replaces the default rule policy.
  `enabled` is omitted, so the effective value is false.
  `level` changes from blocker to warning.
  This rule is OFF.
```

See [cli.md](../reference/cli.md#config-lint) for the full `config lint` warning list and exit-code
contract.

### Print built-in defaults

Inspect the compiled-in defaults without generating a file:

```bash
deltascope config show-default
```

This prints the same YAML that `config init` would write, but to stdout only — useful for diffing against your checked-in config:

```bash
diff <(deltascope config show-default) ./deltascope.yaml
```

## Common Config Tasks

### Disable a rule

Set `enabled: false` to turn a rule off completely:

```yaml
rules:
  ddl.alter.drop_column.forbid:
    enabled: false
```

### Lower a rule's level

Change `level` to downgrade a finding from `warning` to `notice` (or any valid level):

```yaml
rules:
  ddl.table.comment.require:
    enabled: true
    level: notice    # default is warning — lowered for this team
```

### Upgrade a rule's level

```yaml
rules:
  ddl.column.comment.require:
    enabled: true
    level: blocker   # default is warning — escalated for this team
```

### Adjust a rule's parameter

Override a numeric or boolean parameter to suit your environment:

```yaml
rules:
  dml.insert.rows.max_count:
    enabled: true
    level: warning
    params:
      limit: 500    # default is 100 — relaxed for bulk-import workflows

  ddl.table.name.max_length:
    enabled: true
    level: blocker
    params:
      limit: 32    # stricter than the default 64
```

### Full example config snippet

```yaml
# deltascope.yaml
rules:
  # DML rules
  dml.where.require:
    enabled: true
    level: blocker

  dml.limit.forbid:
    enabled: true
    level: warning

  dml.insert.rows.max_count:
    enabled: true
    level: warning
    params:
      limit: 500

  # DDL rules
  ddl.table.comment.require:
    enabled: true
    level: notice           # downgraded — team is still adopting comments

  ddl.column.comment.require:
    enabled: true
    level: warning

  ddl.alter.drop_column.forbid:
    enabled: false          # disabled — ops team handles column drops manually

  ddl.table.name.max_length:
    enabled: true
    level: blocker
    params:
      limit: 48
```

After editing, always validate before use:

```bash
deltascope config lint --file ./deltascope.yaml
```
