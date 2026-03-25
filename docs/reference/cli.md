# CLI Reference

`deltascope` is the primary operator surface for local audits, CI pipeline checks, and agent
workflows. It provides commands for auditing SQL, inspecting rules, managing policy configuration,
and querying engine capabilities.

---

## Global Flags

These flags apply to all subcommands.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | (none) | Path to YAML policy config file. When omitted, `policy.Default()` is used. |
| `--dialect` | string | `mysql` | SQL dialect: `mysql` or `tidb`. Controls parser mode and dialect-specific rules. In metadata-aware mode, dialect is auto-detected from the live instance; an explicit `--dialect` that conflicts with the detected dialect causes exit 2. |
| `--format` | string | `markdown` | Output format: `markdown` (human-readable) or `json` (stable machine-readable contract). |
| `--fail-on` | string | `blocker` | Exit 1 threshold: `blocker`, `warning`, `notice`, or `none`. |
| `--quiet` | bool | false | Suppress non-result output. With markdown output, each finding is printed as a single line; JSON output is unchanged. |
| `--version` | bool | false | Print only the semantic version string and exit. |

---

## deltascope audit

Audit one or more SQL statements from inline text, a file, or standard input.

### Input

The three input sources are mutually exclusive. If none is provided, `deltascope audit` reads from
stdin, making it easy to pipe SQL through the tool.

| Flag | Description |
|------|-------------|
| `--sql <text>` | Inline SQL text to audit |
| `--file <path>` | Path to a `.sql` file to audit |
| _(none)_ | Read SQL from stdin |

Examples:

```bash
# Inline SQL
deltascope audit --sql "DELETE FROM users"

# From a file
deltascope audit --file ./migrations/v2.sql

# From stdin
cat migrations/v2.sql | deltascope audit

# With a non-default policy and JSON output
deltascope audit --config ./deltascope.yaml --format json --file ./migrations/v2.sql
```

### Connection Flags (Metadata-Aware Mode)

Supplying any one of the following flags activates metadata-aware mode. DeltaScope connects to the
specified MySQL or TiDB instance to retrieve live schema facts (table structure, index definitions,
instance variables) and attaches them to each statement before rule evaluation.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--host` | `-h` | (none) | MySQL/TiDB host address |
| `--port` | `-P` | `3306` | Port number |
| `--user` | `-u` | (none) | Database user |
| `--password` | `-p` | (none) | Password on the command line (avoid in production — it appears in shell history) |
| `--ask-password` | | false | Prompt for password interactively. Mutually exclusive with `--password`. |
| `--schema` | `-D` | (none) | Default schema for unqualified table name resolution |
| `--socket` | `-S` | (none) | Unix socket path. Mutually exclusive with `--host`/`--port`. |

**Behavior in metadata-aware mode:**

- Dialect is auto-detected from the live instance by querying `tidb_version()`. If `--dialect` is
  also set explicitly and conflicts, the command exits with code 2.
- Schema resolution order for unqualified table names: SQL-level qualifier → `--schema` flag →
  unique match across accessible schemas → error if ambiguous.

Examples:

```bash
# Connect to a local MySQL instance
deltascope audit \
  --host 127.0.0.1 --port 3306 \
  --user dba --ask-password \
  --schema mydb \
  --file ./migration.sql

# Use a Unix socket
deltascope audit \
  --socket /var/run/mysqld/mysqld.sock \
  --user dba --password secret \
  --schema mydb \
  --sql "ALTER TABLE orders ADD COLUMN status TINYINT NOT NULL DEFAULT 0"
```

### Output Formats

#### Markdown Output (default)

Human-readable output, suitable for review in terminals and pull request comments. Statement headings are 1-based in markdown, even though the JSON `index` field is 0-based.

```
# DeltaScope Audit Result

Verdict: `reject`

- Statements: 1
- Blockers: 1
- Warnings: 0
- Notices: 0

## Result Explanation

Audit produced 1 finding(s) across 1 statement(s)
- UPDATE and DELETE statements must include a WHERE clause

## Statement 1

- Kind: `dml`
- SQL: `delete from users`

### Explanation

Statement 1 has 1 finding(s)
- UPDATE and DELETE statements must include a WHERE clause

### Findings

- [blocker] `dml.where.require`: UPDATE and DELETE statements must include a WHERE clause
  Why: The statement is missing a clause, option, or object that the shipped policy requires.
  Risk: Ignoring this rule can allow high-impact data changes to proceed with less safety review.
  Suggestion: add a WHERE clause that narrows the affected rows
  Statement kind: `dml`
  Metadata:
  - `operation`: `delete`
```

#### JSON Output

Machine-readable output with a stable schema. Use `--format json` in CI pipelines and tooling
integrations.

```bash
deltascope audit --format json --sql "DELETE FROM users"
```

CLI JSON always includes a top-level `context` object. In offline mode it reports the configured dialect source; in metadata-aware mode it also reports resolved schema details.

```json
{
  "verdict": "reject",
  "summary": {
    "statements": 1,
    "blockers": 1,
    "warnings": 0,
    "notices": 0
  },
  "explanation": {
    "summary": "Audit produced 1 finding(s) across 1 statement(s)",
    "reasons": [
      "UPDATE and DELETE statements must include a WHERE clause"
    ]
  },
  "statements": [
    {
      "index": 0,
      "kind": "dml",
      "raw_sql": "delete from users",
      "normalized_sql": "delete from users",
      "explanation": {
        "summary": "Statement 1 has 1 finding(s)",
        "reasons": [
          "UPDATE and DELETE statements must include a WHERE clause"
        ]
      },
      "findings": [
        {
          "rule_id": "dml.where.require",
          "level": "blocker",
          "message": "UPDATE and DELETE statements must include a WHERE clause",
          "statement_kind": "dml",
          "suggestion": "add a WHERE clause that narrows the affected rows",
          "explanation": {
            "summary": "Require DML where require",
            "why": "The statement is missing a clause, option, or object that the shipped policy requires.",
            "risk": "Ignoring this rule can allow high-impact data changes to proceed with less safety review.",
            "suggestion": "add a WHERE clause that narrows the affected rows"
          }
        }
      ]
    }
  ],
  "context": {
    "mode": "offline",
    "dialect": "mysql",
    "dialect_source": "default"
  }
}
```

#### Metadata-Aware JSON Output

When metadata-aware mode is active, the JSON response includes an additional `context` field
describing how dialect and schema were resolved.

```json
{
  "verdict": "pass",
  "summary": { "statements": 1, "blockers": 0, "warnings": 0, "notices": 0 },
  "statements": [...],
  "context": {
    "mode": "metadata-aware",
    "dialect": "mysql",
    "dialect_source": "detected",
    "schema": "app",
    "schema_source": "inferred"
  }
}
```

`dialect_source` values: `"default"` (offline default), `"flag"` (from `--dialect`), or `"detected"` (from a live instance in metadata-aware mode).
`schema_source` values: `"flag"` (from `--schema`), `"inferred"` (unique match), or `"qualified"` (SQL-level qualifier).

#### Quiet Mode

`--quiet` changes markdown output only. With markdown output, DeltaScope suppresses the normal report body and prints each finding as a single line. With `--format json`, the JSON contract is unchanged.

```
[blocker] dml.where.require: UPDATE and DELETE statements must include a WHERE clause
```

This is useful for scripted processing or minimalist CI log output.

---

## deltascope rules

Commands for discovering, inspecting, and searching the registered rule set.

### rules list

List all registered rules. Combine filters freely.

```bash
# All rules
deltascope rules list

# DDL rules only
deltascope rules list --kind ddl

# DML rules only
deltascope rules list --kind dml

# Blocker-level rules only
deltascope rules list --level blocker

# Warning-level rules only
deltascope rules list --level warning

# Only rules enabled under the currently loaded policy
deltascope rules list --enabled-only
```

Example output:

```md
- `ddl.table.comment.require` [warning] (ddl) Require DDL table comment require
- `ddl.table.row_size.max_bytes.require` [blocker] (ddl) Require DDL table row size max bytes require
- `dml.limit.forbid` [warning] (dml) Forbid DML limit forbid
- `dml.where.require` [blocker] (dml) Require DML where require
```

### rules show

Display full detail for a single rule, including its defaults, examples, and remediation guidance.

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
DELETE FROM users WHERE id = 42;
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

### rules search

Search rules by keyword. Matches against both rule ID and description text.

```bash
deltascope rules search "where"
deltascope rules search "metadata"
deltascope rules search "prefix"
```

---

## deltascope config

Commands for managing the policy configuration file.

### config init

Prints the complete default policy YAML to stdout. Redirect to a file to create a local config:

```bash
deltascope config init > deltascope.yaml
```

The generated file contains every rule with its default enabled state and all parameter values
explicitly set. Edit it to customize your policy.

### config lint

Validates a config file for YAML syntax correctness and valid rule IDs. Useful as a pre-commit check
or in CI.

```bash
deltascope config lint --file ./deltascope.yaml
```

Success output:

```
Config OK
```

Failure output (example):

```
Error: unknown rule ID "ddl.table.comments.require" in ./deltascope.yaml (did you mean "ddl.table.comment.require"?)
```

The command exits with code 2 on any validation error.

### config show-default

Prints the built-in default policy. Equivalent to `config init`.

```bash
deltascope config show-default
```

---

## deltascope capabilities

Prints a human-readable summary of all registered capabilities, rule families, and supported
dialects. Useful for verifying what a particular build of DeltaScope supports.

```bash
deltascope capabilities
```

---

## deltascope version

Prints the full version string, including build metadata if available.

```bash
deltascope version
```

You can also use the global flag form to get just the version and exit from any invocation:

```bash
deltascope --version
```

---

## Exit Codes

| Exit Code | Meaning |
|:---------:|---------|
| `0` | Audit completed and findings are below the `--fail-on` threshold; or a non-audit command completed successfully. |
| `1` | Audit completed and at least one finding met or exceeded the `--fail-on` threshold. |
| `2` | Bad user input: invalid flags, malformed SQL, unreadable or invalid config file, conflicting `--dialect`, or ambiguous schema resolution. |
| `3` | Runtime or internal failure (unexpected error, connection failure in metadata-aware mode, etc.). |

---

## Cross-References

- **Rule catalog** — [rules.md](rules.md)
- **Policy configuration** — [config.md](config.md)
- **HTTP API** — [http-api.md](http-api.md)
- **Metadata-aware mode** — [../concept/metadata-aware-mode.md](../concept/metadata-aware-mode.md)
