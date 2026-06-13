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
| `--dialect` | string | `mysql` | SQL dialect: `mysql`, `tidb`, or `postgresql`. PostgreSQL requires a PG-capable DeltaScope binary. Starting with the `v0.17.0` public release line, the supported macOS and Linux `deltascope` archives are PG-capable, so PostgreSQL offline audit uses the normal main CLI path. In metadata-aware mode, dialect is auto-detected from the live MySQL/TiDB-compatible instance; an explicit `--dialect` that conflicts with the detected dialect causes exit 2. |
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

## Action Summary

- [blocker] `dml.where.require`: 1 finding
  Summary: Require DML where require
  Suggestion: Add the required clause, option, or object explicitly so the rule no longer has to infer intent.
  Explain: deltascope rules explain dml.where.require
  Statements: 1

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

#### Action Summary

When a markdown audit has findings, both the default `deltascope audit` path and `--format markdown` render an `## Action Summary` section between the counts (`Statements / Blockers / Warnings / Notices`) and `## Result Explanation`. It groups findings by `rule_id` so you can see what to fix first without reading the report statement-by-statement.

Each rule group shows, at most:

- `[level] \`rule_id\`: N finding(s)` — the priority `level` (`blocker`, `warning`, or `notice`) and the deduplicated finding count
- `Summary:` and `Suggestion:` — rule catalog text when available, otherwise the first finding's message and suggestion
- `Explain: deltascope rules explain <rule_id>` — a copy-paste command to inspect the rule
- `Statements:` — 1-based indexes of the statements that triggered the rule (deduplicated; omitted for global-only findings)
- `Scope: global` — present only when the group includes a global finding

Groups are ordered by remediation priority: `blocker`, then `warning`, then `notice`, then by finding count descending, then by `rule_id` ascending. At most 10 rule groups are shown; when there are more, a final `Showing 10 of N rule groups.` line is appended.

Clean audits (no findings) omit the section entirely. The summary carries no raw SQL and no finding metadata — only rule IDs, levels, counts, catalog text, 1-based statement indexes, and the explain command.

Scope and non-goals:

- The Action Summary is markdown-only. Audit JSON output does not add an `action_summary` field, and the finding JSON shape is unchanged.
- `level` remains the priority field; no `severity` field is introduced.
- SDK, HTTP, MCP, SARIF, GitHub Actions, and GitLab Code Quality outputs are unchanged.
- This adds no parser support, no audit rules, and no change to audit or rule behavior.
- The text layout is a human-readable aid, not a machine contract. For automation, use `--format json` and aggregate findings yourself.

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

Each finding maps to a GitHub Actions workflow command (`::error`, `::warning`, or `::notice`) based on the rule severity. Special characters in titles and messages are escaped per the GitHub workflow command specification. When `--file` is provided, each annotation includes `file=<path>,line=N,col=N` pointing at the exact statement that triggered the finding.

#### SARIF Output

Use `--format sarif` to produce valid SARIF 2.1.0 JSON for GitHub Code Scanning, Azure DevOps, and other SARIF consumers.

```bash
deltascope audit --file ./migrations.sql --dialect postgresql --format sarif > deltascope.sarif
```

The output includes rule metadata (help text from explanation suggestions) under `tool.driver.rules` and maps severity levels to SARIF levels: `blocker` → `error`, `warning` → `warning`, `notice` → `note`. When `--file` is provided, each result includes `artifactLocation.uri`, `startLine`, and `startColumn` pointing at the exact statement.

#### GitLab Code Quality Output

Use `--format gitlab-codequality` to produce a GitLab Code Quality report for merge request Code Quality widgets and diff annotations. Available in all GitLab tiers (Free+).

```bash
deltascope audit --file migrations.sql --format gitlab-codequality --fail-on none > gl-code-quality-report.json
```

In `.gitlab-ci.yml`, publish the report as an artifact:

```yaml
artifacts:
  reports:
    codequality: gl-code-quality-report.json
  when: always
```

Field mapping:

| DeltaScope | GitLab Code Quality |
|-----------|---------------------|
| Rule ID | `check_name` |
| Message + suggestion | `description` |
| blocker → major, warning → minor, notice → info | `severity` |
| `--file` path or `deltascope.sql` | `location.path` |
| Finding line or 1 | `location.lines.begin` |
| SHA-256 hash | `fingerprint` |

Fingerprints are stable across runs so GitLab can track findings across pipelines. Unsupported statements (parser diagnostics) are not emitted as Code Quality issues. `location.lines.begin` carries the statement-start line number from the source mapper. See [use-deltascope-in-gitlab-ci.md](../recipe/use-deltascope-in-gitlab-ci.md) for a complete recipe.

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

#### Parser-Error Unsupported Contract

When the selected dialect parser cannot parse a tracked DDL statement, DeltaScope returns a diagnostic stating that no audit was performed and no findings were inferred from the unparsed SQL. This is an unsupported parser surface, not a fallback parser. DeltaScope does not infer findings from unparsed SQL. The parser-error count is not reduced by this contract. No parser support is added, no fallback parser is introduced, and no new SQL audit rules are created. The diagnostic message is: `statement was not audited because the selected dialect parser could not parse it; no audit findings were inferred`.

#### Unsupported Diagnostics Evidence (v0.230.0)

Starting with `v0.230.0`, parser-error and unsupported statement outcomes expose structured diagnostic evidence through all public surfaces (CLI JSON/text, HTTP, MCP, SDK). Each diagnostic carries these fields:

| Field | Type | Meaning |
|-------|------|---------|
| `classification` | string | Stable category: `parser_error` or `unsupported_statement` |
| `reason` | string | Safe human-readable explanation of why the statement was not audited |
| `action_hint` | string | Generic next step for the user |
| `audited` | bool | `false` — the statement was not audited |
| `dialect` | string | Selected dialect when available |
| `guidance_code` | string | Optional machine-readable boundary category (v0.260.0+) |
| `evidence_ref` | string | Optional GitHub documentation URL for the boundary evidence (v0.260.0+) |

For `parser_error` diagnostics, `reason` contains the v0.220.0 standard diagnostic message and `action_hint` suggests verifying the dialect and syntax, splitting multi-statement input, or upgrading DeltaScope.

For `unsupported_statement` diagnostics, `reason` reuses the existing `UnsupportedDetail.Reason` and `action_hint` suggests manual review.

Diagnostics do not contain raw SQL text, parser `near ...` fragments, routine bodies, or any other forbidden payload. This is not parser support, not a fallback parser, and not new SQL audit rules. Parser-error counts are not reduced. The census is unchanged.

#### Parser Upgrade Candidate Evidence (v0.250.0)

Starting with `v0.250.0`, all 29 remaining parser-error DDL cases (MySQL 15, TiDB 9, PostgreSQL 5) are classified by feasibility bucket. This classification is a documented evidence pack — not current parser support, not a fallback parser, and not new SQL audit rules.

Feasibility bucket facts:

| Bucket | MySQL | TiDB | PostgreSQL | Total |
|---|---:|---:|---:|---:|
| `parser_upgrade_candidate` | 5 | 0 | 5 | 10 |
| `bounded_fallback_candidate` | 1 | 3 | 0 | 4 |
| `product_unsupported_or_inapplicable` | 0 | 6 | 0 | 6 |
| `unsafe_fallback_defer` | 9 | 0 | 0 | 9 |
| `needs_research` | 0 | 0 | 0 | 0 |

Key points:

- `parser_upgrade_candidate` identifies 10 DDL forms (MySQL 5, PostgreSQL 5) that would become parseable after a parser/library upgrade. This is not current support.
- DeltaScope does not infer findings from failed parse text. No fallback parser is used.
- CLI output shape is unchanged. `parser_upgrade_candidate` is a documented classification, not a new CLI field.
- The `parser_error` diagnostic still means the statement was not audited. Users should not treat `parser_error` as PASS.
- Users should review parser-error statements manually.
- No promise that future versions will support these syntax forms.

#### Unsupported Diagnostics Guidance Codes (v0.260.0)

Starting with `v0.260.0`, parser-error diagnostics for parser-upgrade candidates carry two additional fields to explain why the statement was not audited and where to find detailed evidence:

- `guidance_code` — a stable machine-readable string identifying the unsupported boundary category. For parser-upgrade candidates, the value is `parser_upgrade_candidate`.
- `evidence_ref` — a GitHub documentation URL pointing to the relevant evidence section. For parser-upgrade candidates, this links to the [Parser Upgrade Candidate Evidence (v0.250.0)](#parser-upgrade-candidate-evidence-v02500) section above.

These fields are optional. They appear only when the diagnostic matches a known unsupported boundary. When absent, the diagnostic still carries `classification`, `reason`, `action_hint`, `audited`, and `dialect`.

All four public surfaces (SDK, CLI JSON, CLI text, HTTP, MCP) expose these fields consistently. CLI text output appends `guidance_code=` and `evidence_ref=` key-value pairs to the `[diagnostic]` line when present.

Example CLI text output for a parser-upgrade candidate:

```text
[diagnostic] classification=parser_error action_hint=verify the selected dialect and syntax... guidance_code=parser_upgrade_candidate evidence_ref=https://github.com/Fanduzi/DeltaScope/blob/main/docs/reference/cli.md#parser-upgrade-candidate-evidence-v02500
```

Neither field contains raw SQL, parser near-text, object names, function bodies, or any user payload. This is not new parser support, not a fallback parser, and not new SQL audit rules.

The developer/verification entry point `make parser-upgrade-candidate-evidence-report` delegates to the existing `ddl-parser-error-feasibility-report` target. It is not a CLI user command.

For a complete DDL coverage catalog across all dialects with per-form classification, see [ddl-coverage.md](ddl-coverage.md).

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

### PostgreSQL Primary-Key Audit (v0.37.0)

Starting with `v0.37.0`, DeltaScope populates primary-key facts for PostgreSQL `CREATE TABLE` statements. Inline, table-level, named, and composite primary-key declarations flow into the normalized primary-key contract, allowing existing primary-key rules to audit PostgreSQL:

```bash
# Inline primary key — triggers ddl.table.primary_key.bigint.require if not BIGINT
deltascope audit \
  --dialect postgresql \
  --format json \
  --sql "create table users (id integer primary key, name text not null);"
```

Example JSON finding:

```json
{
  "rule_id": "ddl.table.primary_key.bigint.require",
  "level": "warning",
  "message": "primary key column \"id\" is not BIGINT",
  "statement_kind": "ddl"
}
```

```bash
# Composite primary key — triggers ddl.table.primary_key.columns.max_count if over limit
deltascope audit \
  --dialect postgresql \
  --sql "create table order_items (order_id bigint, item_id bigint, quantity int, primary key (order_id, item_id));"
```

Supported PostgreSQL primary-key forms:

| Form | Example |
|------|---------|
| Inline | `id bigint PRIMARY KEY` |
| Table-level | `PRIMARY KEY (id)` |
| Named | `CONSTRAINT users_pkey PRIMARY KEY (id)` |
| Composite | `PRIMARY KEY (a, b)` |

`ddl.table.primary_key.not_null.require` does not produce a stable negative case for PostgreSQL — primary-key columns are treated as effectively NOT NULL.

### PostgreSQL Unique/Index Audit (v0.38.0)

Starting with `v0.38.0`, DeltaScope extends index rule coverage to standalone PostgreSQL `CREATE INDEX` and `CREATE UNIQUE INDEX` statements for approved btree forms:

```bash
deltascope audit \
  --dialect postgresql \
  --format json \
  --sql "CREATE UNIQUE INDEX bad_email_unique ON users (email);"
```

Example JSON finding:

```json
{
  "rule_id": "ddl.index.unique.prefix.require",
  "level": "warning",
  "message": "unique index \"bad_email_unique\" must use prefix \"uniq_\"",
  "statement_kind": "ddl"
}
```

Rules now covering PostgreSQL standalone `CREATE INDEX`:

| Rule ID | What It Flags |
|---------|---------------|
| `ddl.index.secondary.prefix.require` | Secondary index name does not start with the required prefix |
| `ddl.index.unique.prefix.require` | Unique index name does not start with the required prefix |
| `ddl.index.columns.max_count` | Index spans more columns than the allowed maximum |

`v0.49.0` extends the PostgreSQL `CREATE INDEX` path so partial indexes, expression indexes, `INCLUDE` covering indexes, and non-btree access methods are normalized at a coarse fact level. DeltaScope records access method, included columns, predicate presence, and expression-key presence/count, but it does not render or semantically analyze predicate SQL or expression SQL. Operator classes, NULLS NOT DISTINCT, and live schema index introspection remain out of scope.

### PostgreSQL ALTER TABLE ADD CONSTRAINT Audit (v0.39.0)

Starting with `v0.39.0`, DeltaScope extends unique-index prefix and primary-key rule coverage to PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... UNIQUE` and `ALTER TABLE ... ADD CONSTRAINT ... PRIMARY KEY` forms:

```bash
# ALTER TABLE ADD CONSTRAINT UNIQUE — triggers ddl.alter.add_index.unique.prefix.require if prefix is wrong
deltascope audit \
  --dialect postgresql \
  --sql "ALTER TABLE users ADD CONSTRAINT bad_email_key UNIQUE (email);"
```

Example JSON finding:

```json
{
  "rule_id": "ddl.alter.add_index.unique.prefix.require",
  "level": "warning",
  "message": "unique index \"bad_email_key\" must use prefix \"uniq_\"",
  "statement_kind": "ddl"
}
```

```bash
# ALTER TABLE ADD CONSTRAINT PRIMARY KEY — triggers ddl.table.primary_key.bigint.require if not BIGINT
deltascope audit \
  --dialect postgresql \
  --sql "ALTER TABLE users ADD CONSTRAINT users_pkey PRIMARY KEY (id);"
```

Rules now covering PostgreSQL `ALTER TABLE ... ADD CONSTRAINT`:

| Rule ID | What It Flags |
|---------|---------------|
| `ddl.alter.add_index.unique.prefix.require` | Unique constraint name does not start with the required prefix |
| `ddl.table.primary_key.bigint.require` | Primary-key column is not BIGINT |
| `ddl.table.primary_key.columns.max_count` | Composite primary key exceeds the configured column limit |

These reuse existing shared alter-table index and primary-key rule families. No new rule IDs were added. This does not add full PostgreSQL constraint support, metadata-aware constraint introspection, or support for `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` or `ALTER TABLE ... ADD CONSTRAINT ... CHECK`.

### PostgreSQL ALTER TABLE ADD CONSTRAINT FOREIGN KEY Audit (v0.40.0)

Starting with `v0.40.0`, DeltaScope extends FK rule coverage to PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` forms:

```bash
# ALTER TABLE ADD CONSTRAINT FOREIGN KEY — triggers ddl.table.foreign_key.forbid under default policy
deltascope audit \
  --dialect postgresql \
  --sql "ALTER TABLE orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id);"
```

Example JSON finding:

```json
{
  "rule_id": "ddl.table.foreign_key.forbid",
  "level": "blocker",
  "message": "foreign key constraints are not allowed",
  "statement_kind": "ddl"
}
```

Rules now covering PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY`:

| Rule ID | What It Flags |
|---------|---------------|
| `ddl.table.foreign_key.forbid` | Foreign key constraints are forbidden under the default policy |
| `ddl.pg.table.foreign_key.cross_schema.advisory` | Cross-schema FK reference when owning and referenced schemas differ |

These reuse existing shared FK rule families. No new rule IDs were added. This does not add live schema FK existence validation, deferrable constraint support, MATCH FULL policy expansion, or MySQL/TiDB behavior changes.

### PostgreSQL ALTER TABLE ADD CONSTRAINT CHECK Audit (v0.41.0)

Starting with `v0.41.0`, DeltaScope extends check constraint naming and `NOT VALID` advisory rule coverage to PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... CHECK` forms:

```bash
# ALTER TABLE ADD CONSTRAINT CHECK — triggers ddl.pg.alter.add_check.not_valid.require by default
deltascope audit \
  --dialect postgresql \
  --sql "ALTER TABLE orders ADD CONSTRAINT amount_positive CHECK (amount >= 0);"
```

Example JSON finding:

```json
{
  "rule_id": "ddl.pg.alter.add_check.not_valid.require",
  "level": "warning",
  "message": "ADD CHECK constraint should use NOT VALID to avoid full table scan with ACCESS EXCLUSIVE lock",
  "statement_kind": "ddl"
}
```

```bash
# ALTER TABLE ADD CONSTRAINT CHECK — triggers naming rule when prefix is configured
deltascope audit \
  --dialect postgresql \
  --config deltascope.yaml \
  --sql "ALTER TABLE orders ADD CONSTRAINT amount_positive CHECK (amount >= 0);"
```

With a config file enabling `ddl.constraint.check.name.prefix.require` with `prefix: ck_`, the above produces:

```json
{
  "rule_id": "ddl.constraint.check.name.prefix.require",
  "level": "warning",
  "message": "check constraint \"amount_positive\" must use prefix \"ck_\"",
  "statement_kind": "ddl"
}
```

Rules now covering PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... CHECK`:

| Rule ID | What It Flags |
|---------|---------------|
| `ddl.pg.alter.add_check.not_valid.require` | ADD CHECK constraint should use `NOT VALID` to avoid full table scan |
| `ddl.constraint.check.name.prefix.require` | Check constraint name does not start with the required prefix (when configured) |
| `ddl.constraint.check.name.suffix.require` | Check constraint name does not end with the required suffix (when configured) |
| `ddl.constraint.check.name.contains.require` | Check constraint name does not contain any configured token (when configured) |

These reuse existing shared check naming rule families and the PostgreSQL migration-safety rule. `ddl.pg.alter.add_check.not_valid.require` was already registered; check naming rules cover the ALTER CHECK path through extended applicability. This does not add live schema CHECK existence validation, `NOT VALID` validation enforcement, deferred constraint support, or MySQL/TiDB behavior changes.

### PostgreSQL NOT VALID Constraint Validation Pairing (v0.42.0)

Starting with `v0.42.0`, DeltaScope adds a PostgreSQL-only GlobalRule for named CHECK and FOREIGN KEY constraints added with `NOT VALID`. The rule warns when the same audited SQL batch does not contain a later matching `ALTER TABLE ... VALIDATE CONSTRAINT ...` using the same schema, table, and constraint name.

```bash
deltascope audit \
  --dialect postgresql \
  --format json \
  --sql "ALTER TABLE orders ADD CONSTRAINT chk_orders_amount CHECK (amount >= 0) NOT VALID;"
```

Example JSON excerpt:

```json
{
  "global_findings": [
    {
      "rule_id": "ddl.pg.alter.not_valid_constraint.validate.require",
      "level": "warning",
      "message": "NOT VALID constraint \"chk_orders_amount\" on table \"orders\" should be followed by VALIDATE CONSTRAINT in the audited migration batch"
    }
  ]
}
```

The finding is suppressed when the batch includes a later matching validation:

```sql
ALTER TABLE orders ADD CONSTRAINT chk_orders_amount CHECK (amount >= 0) NOT VALID;
ALTER TABLE orders VALIDATE CONSTRAINT chk_orders_amount;
```

This does not add first-time `VALIDATE CONSTRAINT` parser support, live database validation-state lookup, cross-file deployment tracking, unnamed-constraint matching, CHECK expression validation, FK referenced-table validation, MySQL/TiDB behavior changes, or a new public API contract.

### Default Policy Dialect Isolation (v0.43.0)

Starting with `v0.43.0`, the shipped default policy isolates rules by `--dialect`. PostgreSQL audits no longer emit MySQL/TiDB-only rule IDs or MySQL-specific remediation text. MySQL/TiDB audits no longer emit PostgreSQL-only rule IDs.

```bash
# PostgreSQL audit — no MySQL-only rules appear
deltascope audit \
  --dialect postgresql \
  --sql "CREATE TABLE users (id bigint PRIMARY KEY, name varchar(64) NOT NULL);"

# MySQL audit — no ddl.pg.* rules appear
deltascope audit \
  --dialect mysql \
  --sql "CREATE TABLE users (id bigint unsigned NOT NULL AUTO_INCREMENT, PRIMARY KEY (id)) ENGINE=InnoDB;"
```

This does not add new rule IDs, parser features, public API contracts, live schema validation, cross-database tracking, or MySQL/TiDB behavior changes beyond dialect isolation.

## Repository Confidence Targets

| Target | Purpose |
|--------|---------|
| `make pg-unit-test-gates` | Run PostgreSQL-tagged unit packages without Docker |
| `make pg-e2e-gates` | Run Docker-backed PostgreSQL CLI, HTTP, and MCP end-to-end suites |
| `make pg-confidence-gates` | Combine the canonical PostgreSQL unit + E2E confidence gates |
| `make release-surface-gates VERSION=vX.Y.Z` | Verify the package/release contract for the tagged release |
| `make release-version-surface-gates VERSION=vX.Y.Z` | Verify versioned docs/install surfaces, bilingual release notes, and release semantic consistency (census, corpus, rule counts, no-overclaim, no-leak) |
| `make ddl-census-report` | Print tracked DDL coverage census for MySQL, TiDB, and PostgreSQL — inventory/reporting gate, not a full SQL grammar coverage claim |
| `make ddl-parser-error-feasibility-report` | Print parser-error feasibility classification for all tracked DDL parser-error cases (MySQL 15, TiDB 9, PostgreSQL 5) — classification/report gate, not parser support or fallback extraction |
| `make parser-error-unsupported-contract-test` | Run parser-error unsupported contract tests across application, SDK, CLI, HTTP, and MCP surfaces — verifies diagnostic clarity, no findings inferred, no forbidden payload leak; does not add parser support, fallback parsing, or new SQL audit rules |
| `make unsupported-diagnostics-evidence-test` | Run unsupported diagnostics evidence contract tests across application, SDK, CLI, HTTP, and MCP surfaces — verifies structured diagnostic evidence (classification, reason, action_hint, audited, dialect, guidance_code, evidence_ref) without leaking raw SQL or parser internals; does not add parser support, fallback parsing, or new SQL audit rules |

`v0.22.0` is the **E2E & Release Confidence Pack**. It does not add new PostgreSQL SQL rule semantics; it documents and validates the existing PostgreSQL product and release surfaces with canonical repository entrypoints. `v0.23.0` then extends the documented PostgreSQL `CREATE TABLE` coverage while keeping these release-surface gates as the canonical verification path.

Starting with `v0.44.0`, `make release-contract-gates VERSION=vX.Y.Z` combines version surface verification, binary version smoke, default policy dialect isolation smoke, and archive verification into a single pre-publish gate. See the release notes for the full gate inventory.

#### Quiet Mode

`--quiet` changes markdown output only. With markdown output, DeltaScope suppresses the normal report body and prints each finding as a single line. With `--format json`, the JSON contract is unchanged.

```
[blocker] dml.where.require: UPDATE and DELETE statements must include a WHERE clause
```

This is useful for scripted processing or minimalist CI log output.

---

## deltascope rules

Commands for discovering and inspecting the registered rule set. These are read-only metadata lookup commands — they do not execute audits, parse SQL, or call the audit service.

### rules list

List rules from the shipped catalog with optional filters.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--dialect` | string | (none) | Filter by dialect: `mysql`, `tidb`, `postgresql`, or `common`. |
| `--level` | string | (none) | Filter by level: `blocker`, `warning`, or `notice`. |
| `--kind` | string | (none) | Filter by kind: `ddl` or `dml`. |
| `--category` | string | (none) | Case-insensitive category/family substring match. |
| `--search` | string | (none) | Case-insensitive search across rule ID, summary, tags, and config key. |
| `--format` | string | `text` | Output format: `text` or `json`. |
| `--limit` | int | `0` | Limit result count; `0` means no limit. |

All filters are optional and combine as AND conditions. Invalid enum values produce a clear validation error and exit code 2. Empty results return success with zero rules.

```bash
# All rules
deltascope rules list

# Blocker-level rules
deltascope rules list --level blocker

# PostgreSQL warning rules in JSON
deltascope rules list --dialect postgresql --level warning --format json

# Search by keyword
deltascope rules list --search drop_column

# DDL rules in the alter_table category
deltascope rules list --kind ddl --category alter_table

# Limit output
deltascope rules list --level blocker --limit 5
```

Example text output:

```text
RULE ID                               LEVEL    DIALECT     KIND  CATEGORY
------------------------------------  -------  ----------  ----  -----------
ddl.alter.drop_column.exists.require  blocker  common      ddl   alter_table
ddl.alter.drop_column.forbid          warning  common      ddl   alter_table
ddl.pg.alter.drop_column.advisory     warning  postgresql  ddl   alter_table
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
    "filters": { "dialect": "postgresql", "level": "warning" }
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

### rules explain

Show detailed information about a single rule by exact rule ID.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--format` | string | `text` | Output format: `text` or `json`. |

```bash
# Text output
deltascope rules explain dml.where.require

# JSON output
deltascope rules explain dml.where.require --format json
```

`rules explain` does not run an audit and does not parse SQL. It returns static rule metadata from the shipped catalog. The JSON output contains `level`, not `severity`.

Unknown rule IDs produce a clear error and exit code 2:

```text
rule "nonexistent_rule" not found
```

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

Config Example:
  rules:
    dml.where.require:
      enabled: true
      level: blocker
      params:
        required: true
```

Example JSON output (abbreviated):

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
    "config_key": "dml.where.require",
    "tags": ["dml", "common", "dml_safety", "require"]
  }
}
```

### Non-Goals

These commands do not:

- Audit SQL statements
- Add new audit rules or change rule behavior
- Change the finding JSON shape
- Introduce a `severity` field
- Provide SDK, HTTP, or MCP rule discovery surfaces

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

## deltascope ddl-coverage

Query the generated DDL coverage catalog for verified DeltaScope entries. This is a catalog lookup command — it does not execute audits, parse SQL, or call the audit service.

### Synopsis

```bash
deltascope ddl-coverage [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dialect` | (none) | Filter by dialect: `mysql`, `tidb`, `postgresql` |
| `--classification` | (none) | Filter by classification: `finding_covered`, `normalized_silent`, `unsupported_boundary`, `parser_error`, `unclassified` |
| `--guidance-code` | (none) | Filter by guidance code: `parser_upgrade_candidate` |
| `--family` | (none) | Case-insensitive substring match on catalog family |
| `--form` | (none) | Case-insensitive substring match on catalog form |
| `--search` | (none) | Case-insensitive substring match across family, form, notes, guidance code, and rule IDs |
| `--format` | `text` | Output format: `text` or `json` |
| `--limit` | `0` | Limit result count; `0` means no limit |

All filters are optional. Multiple filters combine as AND conditions.

### Examples

```bash
# MySQL parser-upgrade candidates
deltascope ddl-coverage --dialect mysql --classification parser_error --guidance-code parser_upgrade_candidate

# PostgreSQL DROP SUBSCRIPTION in JSON
deltascope ddl-coverage --dialect postgresql --search "drop subscription" --format json

# All TiDB entries in JSON
deltascope ddl-coverage --dialect tidb --format json

# Empty lookup returns success with entries: []
deltascope ddl-coverage --search definitely-not-present --format json
```

### Output Formats

Text output (default) prints a column-aligned table with `DIALECT`, `CLASSIFICATION`, `FAMILY`, `FORM`, and `GUIDANCE` columns, followed by a count.

JSON output (`--format json`) returns a stable machine-readable contract:

```json
{
  "version": "v0.280.0",
  "summary": {
    "total": 2,
    "returned": 2,
    "filters": { "dialect": "mysql" }
  },
  "entries": [...]
}
```

### Non-Goals

This command does not:

- Audit SQL statements
- Add parser support or fallback parser behavior
- Add new SQL audit rules
- Claim full DDL support or dialect parity
- Claim vendor grammar completeness

Query results reflect verified catalog entries. An empty result means no catalog match — not a failure, and not a statement about database support. See [ddl-coverage.md](ddl-coverage.md) and [ddl-coverage-catalog.json](ddl-coverage-catalog.json) for full catalog details.

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

### PostgreSQL ALTER TABLE GENERATED Follow-up Pack (`v0.31.0`)

Starting with `v0.31.0`, additional PostgreSQL generated/identity `ALTER TABLE` forms are surfaced as explicit unsupported boundaries, closing the adjacent gap left by `v0.30.0`. The CLI exposes these through the same unsupported result path: the audit output includes an `unsupported` array with `feature` and `reason` fields, and the process exits with the audit exit code.

- `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION` → `generated_column`
- `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ...` → `generated_as_identity`
- `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY` → `generated_as_identity`
- Corpus, service, and CLI / HTTP / MCP / `pkg/deltascope` parity lock the same contract.
- This is not a new CLI flag or support expansion — it is boundary tightening.

### PostgreSQL ALTER TABLE GENERATED Boundary Pack (`v0.30.0`)

Starting with `v0.30.0`, PostgreSQL `ALTER TABLE ... ADD COLUMN` forms that carry generated stored or identity semantics are surfaced as explicit unsupported boundaries. The CLI exposes these through the same unsupported result path: the audit output includes an `unsupported` array with `feature` and `reason` fields, and the process exits with the audit exit code.

- `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS (...) STORED` → `generated_column`
- `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS IDENTITY` → `generated_as_identity`
- Corpus, service, and CLI / HTTP / MCP / `pkg/deltascope` parity lock the same contract.
- Adjacent `DROP EXPRESSION`, `SET GENERATED`, and `DROP IDENTITY` forms now receive explicit unsupported mappings in `v0.31.0`.
- This is not a new CLI flag or support expansion — it is boundary tightening.

### PostgreSQL CREATE TABLE Unsupported Boundaries (`v0.26.0`)

Starting with `v0.26.0`, the PostgreSQL extractor explicitly rejects identity columns, generated stored columns, exclusion constraints, and partitioned tables as unsupported boundaries. The CLI exposes these through the unsupported result path: the audit output includes an `unsupported` array with `feature` and `reason` fields, and the process exits with the audit exit code. This is not a new CLI flag or contract — it is a boundary tightening that ensures these forms are no longer silently accepted or partially handled.

### Schema-Qualified Reference Semantics (`v0.27.0`)

Starting with `v0.27.0`, the PostgreSQL extractor preserves schema-qualified referenced-object facts (`ReferencedSchema`) in the shared contract. Starting with `v0.28.0`, FK forbid finding metadata now exposes these referenced-object fields. This is not a new CLI flag or output contract.

### Referenced-Object Metadata Surface (`v0.28.0`)

Starting with `v0.28.0`, the `ddl.table.foreign_key.forbid` finding metadata in CLI JSON output now includes `referenced_schema` (e.g., `"public"`), `referenced_table` (e.g., `"users"`), and `referenced_columns` (e.g., `["id"]`) when the underlying PostgreSQL FK constraint carries those facts. This is an additive metadata widening — no new CLI flags, no new output contract fields beyond the finding metadata object. `referenced_table` is never concatenated with `referenced_schema` (e.g., never `"public.users"`).

Example finding metadata for a schema-qualified FK:

```json
{
  "rule_id": "ddl.table.foreign_key.forbid",
  "level": "blocker",
  "message": "...",
  "metadata": {
    "table": "orders",
    "constraint": "fk_orders_approver",
    "columns": ["approver_id"],
    "referenced_schema": "public",
    "referenced_table": "users",
    "referenced_columns": ["id"]
  }
}
```

This is not a new CLI flag, not schema-aware FK policy support, and not a new rule family.

### Schema-Aware FK Policy Pack (`v0.29.0`)

Starting with `v0.29.0`, CLI JSON output can also surface the PostgreSQL-only notice rule `ddl.pg.table.foreign_key.cross_schema.advisory` for explicit cross-schema foreign keys.

- The rule fires only when the owning table schema and referenced schema are both explicit and different.
- Same-schema foreign keys do not trigger it.
- Bare references such as `REFERENCES users(id)` remain schema unknown and do not trigger it.
- DeltaScope does not infer `public` and does not model PostgreSQL `search_path`.
- No new CLI flag is introduced.

Example notice-level finding metadata for an explicit cross-schema FK:

```json
{
  "rule_id": "ddl.pg.table.foreign_key.cross_schema.advisory",
  "level": "notice",
  "message": "...",
  "metadata": {
    "table": "orders",
    "table_schema": "billing",
    "constraint": "fk_orders_approver",
    "columns": ["approver_id"],
    "referenced_schema": "auth",
    "referenced_table": "users",
    "referenced_columns": ["id"]
  }
}
```

`referenced_table` remains normalized as `"users"`, never `"auth.users"`.

---

## Cross-References

- **Rule catalog** — [rules.md](rules.md)
- **Policy configuration** — [config.md](config.md)
- **HTTP API** — [http-api.md](http-api.md)
- **Metadata-aware mode** — [../concept/metadata-aware-mode.md](../concept/metadata-aware-mode.md)

---

## v0.36.0: Rule Coverage for Generated/Identity State-Transition Forms

Starting with v0.36.0, PostgreSQL generated/identity state-transition forms that were supported in v0.35.0 now produce explicit `rule_id` findings. The supported parser/output path is unchanged — the difference is that these forms now trigger PostgreSQL-only forbid rules instead of passing silently.

New rule IDs:

| Rule ID | Covered Form |
|---------|-------------|
| `ddl.alter.drop_expression.forbid` | `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION` |
| `ddl.alter.set_generated.forbid` | `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ...` |
| `ddl.alter.drop_identity.forbid` | `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY` |

Example JSON finding:

```json
{
  "findings": [
    {
      "rule_id": "ddl.alter.set_generated.forbid",
      "level": "blocker",
      "message": "ALTER action 'set_generated' is not allowed"
    }
  ]
}
```

This is rule coverage — not parser support widening, not spec contract widening, not generated expression evaluation, not complete PostgreSQL sequence semantics. No new CLI flags were added.

## v0.35.0: State-Transition Support for Generated/Identity Columns

Starting with v0.35.0, PostgreSQL state-transition forms for generated and identity columns are processed through the normal supported audit path. CLI output for these forms no longer includes an `unsupported` array — instead, the audit produces normal results with findings where applicable.

Supported forms:

- `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION`
- `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ALWAYS`
- `ALTER TABLE ... ALTER COLUMN ... SET GENERATED BY DEFAULT`
- `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY`

These forms now produce standard CLI output (exit code 0 for clean pass, 1 if findings meet the `--fail-on` threshold). The normalized contract is: `drop_expression`, `set_generated` with `generated_when` (`"a"` / `"d"`), `drop_identity`.

This is state-transition support — not full generated-column lifecycle support, not generated expression evaluation, not complete PostgreSQL sequence semantics. No new CLI flags were added.

## v0.34.0: Narrow Support for Generated/Identity Definitions

Starting with v0.34.0, narrow PostgreSQL generated/identity definition forms are processed through the normal supported audit path. Shared facts (`generated_when`, `is_identity`, `identity_options`) from v0.33.0 continue flowing through the normal result path. No new CLI flags were added.

## v0.33.0: Unsupported Metadata for Generated/Identity Statements

Starting with `v0.33.0`, PostgreSQL unsupported generated/identity outcomes carry structured metadata in CLI JSON output. When auditing a `CREATE TABLE` or `ALTER TABLE ADD COLUMN` statement that contains `GENERATED ALWAYS AS (...) STORED` or `GENERATED ... AS IDENTITY`, the `unsupported` array entry now includes a `metadata` object:

```json
{
  "unsupported": [
    {
      "feature": "generated_as_identity",
      "reason": "...",
      "metadata": {
        "column": "id",
        "generated_when": "a",
        "is_identity": true,
        "identity_options": { "start": 10, "increment": 5, "cache": 20, "cycle": true }
      }
    }
  ]
}
```

Metadata keys:

| Key | Present When | Type |
|-----|-------------|------|
| `column` | always | string |
| `generated_when` | always | `"a"` (ALWAYS) or `"d"` (BY DEFAULT) |
| `is_identity` | identity columns | boolean (`true`) |
| `identity_options` | identity with options | object with numeric/boolean values |

This is an additive metadata widening on the unsupported contract. No new CLI flags or output format changes were introduced.
