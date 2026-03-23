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
| `--quiet` | bool | false | Suppress non-result output. Each finding is printed as a single line. |
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

Human-readable output, suitable for review in terminals and pull request comments.

```
## Audit Result

Verdict: reject
Statements: 1 | Blockers: 1 | Warnings: 0 | Notices: 0

### Statement 1: DELETE
**Raw SQL:** `DELETE FROM users`

| Rule ID | Level | Message |
|---------|-------|---------|
| dml.where.require | blocker | DELETE statement is missing a WHERE clause |
```

#### JSON Output

Machine-readable output with a stable schema. Use `--format json` in CI pipelines and tooling
integrations.

```bash
deltascope audit --format json --sql "DELETE FROM users"
```

```json
{
  "verdict": "reject",
  "summary": {
    "statements": 1,
    "blockers": 1,
    "warnings": 0,
    "notices": 0
  },
  "statements": [
    {
      "index": 1,
      "kind": "DELETE",
      "raw_sql": "DELETE FROM users",
      "findings": [
        {
          "rule_id": "dml.where.require",
          "level": "blocker",
          "message": "DELETE statement is missing a WHERE clause",
          "suggestion": "Add a WHERE clause to restrict the rows affected",
          "location": { "line": 1, "column": 1 }
        }
      ]
    }
  ],
  "global_findings": []
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
  "global_findings": [],
  "context": {
    "mode": "metadata-aware",
    "dialect": "mysql",
    "dialect_source": "detected",
    "schema": "app",
    "schema_source": "inferred"
  }
}
```

`dialect_source` values: `"detected"` (from live instance) or `"explicit"` (from `--dialect` flag).
`schema_source` values: `"flag"` (from `--schema`), `"inferred"` (unique match), or `"qualified"` (SQL-level qualifier).

#### Quiet Mode

`--quiet` suppresses all output except findings. Each finding is printed as a single line:

```
dml.where.require [blocker] DELETE statement is missing a WHERE clause
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

```
RULE ID                                   KIND  LEVEL    METADATA
dml.where.require                         dml   blocker  false
dml.limit.forbid                          dml   warning  false
ddl.table.comment.require                 ddl   warning  false
ddl.table.row_size.max_bytes.require      ddl   blocker  true
...
```

### rules show

Display full detail for a single rule, including its parameters and defaults.

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
Config file ./deltascope.yaml is valid.
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
