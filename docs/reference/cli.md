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
| `--dialect` | string | `mysql` | SQL dialect: `mysql`, `tidb`, or `postgresql`. PostgreSQL requires a PG-capable DeltaScope binary. Starting with the `v0.17.0` public release line, the supported macOS and Linux `deltascope` archives are PG-capable, so PostgreSQL offline audit uses the normal main CLI path. A legacy `deltascope-pg` download may remain available during the transition for older CLI-only workflows. In metadata-aware mode, dialect is auto-detected from the live MySQL/TiDB-compatible instance; an explicit `--dialect` that conflicts with the detected dialect causes exit 2. |
| `--format` | string | `markdown` | Output format: `markdown` (human-readable), `json` (stable machine-readable contract), `github-actions` (CI inline annotations), or `sarif` (SARIF 2.1.0 for GitHub Code Scanning). |
| `--fail-on` | string | `blocker` | Exit 1 threshold: `blocker`, `warning`, `notice`, or `none`. |
| `--quiet` | bool | false | Suppress non-result output. With markdown output, each finding is printed as a single line; JSON output is unchanged. |
| `--version` | bool | false | Print only the semantic version string and exit. |

Cobra also exposes a built-in `--help` flag on every command.

---

## deltascope audit

Audit one or more SQL statements from inline text, a file, or standard input.

### Input

The three input sources are mutually exclusive. If none is provided, `deltascope audit` reads from
stdin, making it easy to pipe SQL through the tool.

| Flag | Description |
|------|-------------|
| `--sql` | Inline SQL text to audit. Value: `<text>` |
| `--file` | Path to a `.sql` file to audit. Value: `<path>` |
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
specified MySQL, TiDB, or PostgreSQL instance to retrieve live schema facts (table structure, index
definitions, instance variables) and attaches them to each statement before rule evaluation.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--host` | `-h` | (none) | Database host address |
| `--port` | `-P` | `3306` | Port number (default 3306 for MySQL/TiDB, 5432 for PostgreSQL) |
| `--user` | `-u` | (none) | Database user |
| `--password` | `-p` | (none) | Password on the command line (avoid in production — it appears in shell history) |
| `--password-env` | | (none) | Environment variable that contains the database password |
| `--password-file` | | (none) | File path that contains the database password |
| `--ask-password` | | false | Prompt for password interactively. Mutually exclusive with `--password`, `--password-env`, and `--password-file`. |
| `--schema` | `-D` | (none) | Default schema for unqualified table name resolution |
| `--socket` | `-S` | (none) | Unix socket path. Mutually exclusive with `--host`/`--port`. |

**Behavior in metadata-aware mode:**

- For MySQL/TiDB: dialect is auto-detected from the live instance by querying `tidb_version()`. If `--dialect` is
  also set explicitly and conflicts, the command exits with code 2.
- For PostgreSQL: pass `--dialect postgresql` explicitly. Auto-detection is not supported for PostgreSQL.
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
  --user dba --password-env DELTASCOPE_DB_PASSWORD \
  --schema mydb \
  --sql "ALTER TABLE orders ADD COLUMN status TINYINT NOT NULL DEFAULT 0"

# Connect to a PostgreSQL instance
deltascope audit \
  --host 127.0.0.1 --port 5432 \
  --user readonly --ask-password \
  --dialect postgresql --schema public \
  --file ./migration.sql
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

### DML Impact Estimation

When DeltaScope audits `UPDATE` or `DELETE`, it may add an `impact` object to each statement result. The object is conservative by design and reports `estimated_rows`, `estimated_ratio`, `risk_level`, `confidence`, `source`, `reason_codes`, and optional `notes`.

```json
{
  "raw_sql": "DELETE FROM users WHERE id = 42",
  "impact": {
    "estimated_rows": 1,
    "estimated_ratio": 0.0001,
    "risk_level": "low",
    "confidence": "high",
    "source": "metadata",
    "reason_codes": ["pk_equality"],
    "notes": ["refined with table statistics"]
  }
}
```

Offline mode uses SQL shape only. Metadata-aware mode may refine the estimate with read-only table statistics (MySQL/TiDB) or via the PostgreSQL query planner (`EXPLAIN`). DeltaScope does not execute the DML and does not run `EXPLAIN ANALYZE`.

Threshold rules `dml.impact.rows.max_count` and `dml.impact.ratio.max_percent` consume this additive statement-level payload when it is available. The payload itself is attached in the audit flow before rule evaluation.

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
For CLI metadata-aware audits, `schema_source` values are `"flag"` (from `--schema`) or `"inferred"` (unique match across accessible schemas). When schema inference is unnecessary or unavailable, the field may be omitted instead of emitting an extra source value.

#### Quiet Mode

`--quiet` changes markdown output only. With markdown output, DeltaScope suppresses the normal report body and prints each finding as a single line. With `--format json`, the JSON contract is unchanged.

```
[blocker] dml.where.require: UPDATE and DELETE statements must include a WHERE clause
```

This is useful for scripted processing or minimalist CI log output.

#### GitHub Actions Output

Use `--format github-actions` to produce inline CI annotations that render in the GitHub Actions workflow log.

```bash
deltascope audit --dialect postgresql --file ./migrations/20260409_add_index.sql --format github-actions
```

Each finding maps to a GitHub Actions workflow command (`::error`, `::warning`, or `::notice`) based on the rule severity. Special characters in titles and messages are escaped per the GitHub workflow command specification.

#### SARIF Output

Use `--format sarif` to produce valid SARIF 2.1.0 JSON for GitHub Code Scanning, Azure DevOps, and other SARIF consumers.

```bash
deltascope audit --file ./migrations.sql --dialect postgresql --format sarif > deltascope.sarif
```

The output includes rule metadata (help text from explanation suggestions) under `tool.driver.rules` and maps severity levels to SARIF levels: `blocker` → `error`, `warning` → `warning`, `notice` → `note`.

#### Rule Summary

JSON, markdown, and quiet output include a rule summary showing how many rules were loaded, how many were applicable to the given dialect, and how many were skipped. In JSON this appears as `rule_summary`; in markdown it renders as `## Rule Summary` and `## Skipped Rules` sections. GitHub Actions and SARIF output do not include rule summary.

#### PostgreSQL Trust Signals

When auditing on the MySQL/TiDB path, DeltaScope may detect PostgreSQL-specific syntax and emit a `dialect.postgresql.syntax.detected.notice` global finding. This is an advisory notice — DeltaScope **does not auto-switch dialect**.

In markdown output, a `## Audit Context` section appears with an explicit trust note when this notice fires:

```text
## Audit Context
- Mode: `offline`
- Dialect: `mysql` (default)
- Trust Note: Dialect remains `mysql` (default). DeltaScope did not auto-switch dialect.
```

In JSON output, the top-level `context` object always reports `mode`, `dialect`, and `dialect_source`:

```json
{
  "context": {
    "mode": "offline",
    "dialect": "mysql",
    "dialect_source": "default"
  }
}
```

In quiet output, a `[context]` line is appended:

```text
[context] mode=offline dialect=mysql dialect_source=default
```

If the SQL does target PostgreSQL, re-run with `--dialect postgresql`. If not, the notice can be safely ignored.

#### PostgreSQL Capability-Boundary Errors

When a PG-capable DeltaScope binary encounters PostgreSQL-specific functionality that is not yet fully supported (e.g., DDL parsing), it returns a typed `PostgreSQLCapabilityBoundaryError`. This distinguishes known capability limits from real parse failures. The error includes a clear message about what surface was requested and what the current build supports.

#### PostgreSQL DDL Coverage

Starting with `v0.21.0`, DeltaScope normalizes common PostgreSQL migration follow-up DDL through the shared audit pipeline. These forms no longer return capability-boundary errors:

| PostgreSQL DDL | Action | Notes |
|----------------|--------|-------|
| `ALTER TABLE ... ALTER COLUMN ... SET DEFAULT` | `set_default` | Column default assignment during phased rollout |
| `ALTER TABLE ... ALTER COLUMN ... DROP DEFAULT` | `drop_default` | Column default removal |
| `ALTER TABLE ... ALTER COLUMN ... SET NOT NULL` | `set_not_null` | Nullability enforcement after backfill |
| `ALTER TABLE ... ALTER COLUMN ... DROP NOT NULL` | `drop_not_null` | Nullability relaxation |
| `ALTER TABLE ... VALIDATE CONSTRAINT` | `validate_constraint` | Constraint validation in the recommended `NOT VALID` → `VALIDATE` pattern |
| `ALTER TABLE ... DROP CONSTRAINT` | `drop_constraint` | Constraint removal; primary-key drops reuse `ddl.alter.drop_primary_key` rules when metadata is available |

Starting with `v0.23.0`, DeltaScope also audits more common PostgreSQL `CREATE TABLE` constraint shapes through the same shared pipeline:

| PostgreSQL `CREATE TABLE` shape | Supported | Auditable | Rule-mapped | Metadata-dependent | Notes |
|---------------------------------|:---------:|:---------:|:-----------:|:------------------:|-------|
| Table-level named `CHECK` | ✓ | ✓ | ✓ (shared constraint naming when configured) | — | Reuses existing constraint naming governance where applicable |
| Column-level inline `CHECK` | ✓ | ✓ | — | — | Supported structure; no dedicated new rule family |
| Table-level named `UNIQUE` | ✓ | ✓ | ✓ (shared constraint naming when configured) | — | Named constraint facts can flow into existing naming governance |
| Column-level inline `UNIQUE` | ✓ | ✓ | ✓ (shared index facts) | — | Contributes index facts to existing shared index rules |
| Table-level named `FOREIGN KEY` | ✓ | ✓ | ✓ (shared constraint naming when configured) | — | Foreign-key naming rules only matter when policy allows foreign keys |
| Column-level inline `REFERENCES` | ✓ | ✓ | — | — | Exposed as parser-owned shared facts only; no invented metadata semantics |

Examples:

```bash
# Create-table coverage: named + inline constraints
deltascope audit \
  --dialect postgresql \
  --sql "create table orders (id bigint primary key, user_id bigint references users(id), amount numeric not null check (amount >= 0), constraint uniq_orders_user unique (user_id), constraint chk_orders_amount check (amount >= 0));"

# Phased migration: set a column default
deltascope audit \
  --dialect postgresql \
  --sql "alter table users alter column status set default 'active';"

# Constraint lifecycle: validate a constraint
deltascope audit \
  --dialect postgresql \
  --sql "alter table users validate constraint chk_amount;"

# Constraint lifecycle: drop a constraint (primary-key mapping applies with metadata)
deltascope audit \
  --dialect postgresql \
  --sql "alter table orders drop constraint orders_pkey;"
```

`VALIDATE CONSTRAINT` without a corresponding rule produces a clean audit — it is supported and auditable, but does not guarantee a finding. `DROP CONSTRAINT` on a primary key triggers existing primary-key rules only when metadata is available; in offline mode it passes through as a normal alter action. The `v0.23.0` create-table expansion does not claim full PostgreSQL DDL support and does not add new CLI flags or contracts.

## Repository Confidence Targets

| Target | Purpose |
|--------|---------|
| `make pg-unit-test-gates` | Run PostgreSQL-tagged unit packages without Docker |
| `make pg-e2e-gates` | Run Docker-backed PostgreSQL CLI, HTTP, and MCP end-to-end suites |
| `make pg-confidence-gates` | Combine the canonical PostgreSQL unit + E2E confidence gates |
| `make release-surface-gates VERSION=vX.Y.Z` | Verify the package/release contract for the tagged release |
| `make release-version-surface-gates VERSION=vX.Y.Z` | Verify versioned docs/install surfaces and bilingual release notes |

`v0.22.0` is the **E2E & Release Confidence Pack**. It does not add new PostgreSQL SQL rule semantics; it documents and validates the existing PostgreSQL product and release surfaces with canonical repository entrypoints. `v0.23.0` then extends the documented PostgreSQL `CREATE TABLE` coverage while keeping these release-surface gates as the canonical verification path.

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

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--kind` | string | (none) | Filter to `ddl` or `dml` rules. |
| `--level` | string | (none) | Filter to `blocker`, `warning`, or `notice`. |
| `--enabled-only` | bool | false | Show only rules enabled in the shipped default policy. |

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

```text
RULE ID                              LEVEL    KIND  SUMMARY
-----------------------------------  -------  ----  ----------------------------------------------
ddl.table.comment.require           warning  ddl   Require DDL table comment require
ddl.table.row_size.max_bytes.require  blocker  ddl   Require DDL table row size max bytes require
dml.impact.rows.max_count           warning  dml   Require DML impact rows max count
dml.impact.ratio.max_percent        warning  dml   Require DML impact ratio max percent
dml.limit.forbid                    warning  dml   Forbid DML limit forbid
dml.where.require                   blocker  dml   Require DML where require
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

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--file` | string | (none) | Path to the YAML config file to lint. Required. |

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

## Release Validation

Starting with `v0.25.0`, DeltaScope release validation includes SQL corpus tests that run representative MySQL, TiDB, and PostgreSQL cases through the audit application layer with two-layer assertions (report-level and semantic). These corpus tests are release-confidence assets and do not affect CLI behavior or require any user action.

### PostgreSQL CREATE TABLE Unsupported Boundaries (`v0.26.0`)

Starting with `v0.26.0`, the PostgreSQL extractor explicitly rejects identity columns, generated stored columns, exclusion constraints, and partitioned tables as unsupported boundaries. The CLI exposes these through the unsupported result path: the audit output includes an `unsupported` array with `feature` and `reason` fields, and the process exits with the audit exit code. This is not a new CLI flag or contract — it is a boundary tightening that ensures these forms are no longer silently accepted or partially handled.

### Schema-Qualified Reference Semantics (`v0.27.0`)

Starting with `v0.27.0`, the PostgreSQL extractor preserves schema-qualified referenced-object facts (`ReferencedSchema`) in the shared contract. The CLI current output contract for FK forbid findings remains unchanged — `referenced_schema` is not exposed in the finding metadata. Schema-qualified reference facts are preserved in the shared contract and validated by corpus and service-level semantic tests. This is not a new CLI flag or output contract.

---

## Cross-References

- **Rule catalog** — [rules.md](rules.md)
- **Policy configuration** — [config.md](config.md)
- **HTTP API** — [http-api.md](http-api.md)
- **Metadata-aware mode** — [../concept/metadata-aware-mode.md](../concept/metadata-aware-mode.md)
