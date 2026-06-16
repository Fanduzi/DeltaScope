# Core Concepts

## Audit Request

An audit request is the unit of work submitted to DeltaScope. It carries:

- **SQL text**: one or more SQL statements to evaluate, as a string, file, or stdin stream
- **Dialect**: `mysql` or `tidb`; controls parser behavior and dialect-specific rule activation
- **Config path** (optional): path to a YAML policy file; when omitted, `policy.Default()` is used
- **Metadata provider** (optional): connection settings that activate live instance and schema enrichment

The same request structure is accepted across all three product surfaces:

| Surface | How the request is submitted |
|---|---|
| CLI (`deltascope audit`) | Flags and arguments on the command line |
| HTTP server (`POST /v1/audit`) | JSON request body |
| Go library (`pkg/deltascope`) | `deltascope.Request` struct passed to `deltascope.Audit()` |

All three surfaces run the same audit pipeline and produce structurally identical findings.

---

## Findings and Verdicts

### Finding

A finding is the result of a single rule evaluation. Every finding carries:

- **Rule ID**: a stable, dotted identifier such as `dml.where.require` or `ddl.column.comment.require`. Rule IDs are stable across versions within a major release.
- **Level**: one of `blocker`, `warning`, or `notice` (described below).
- **Message**: a human-readable explanation of what the rule detected.
- **Suggestion** (optional): a recommended action to resolve the issue.
- **Location** (optional): line and column in the original SQL text where the issue was found.
- **Metadata** (optional): a key/value map with additional context (e.g., table name, column name, current value).

Example finding in JSON:

```json
{
  "rule_id": "dml.where.require",
  "level": "blocker",
  "message": "UPDATE statement is missing a WHERE clause",
  "suggestion": "Add a WHERE clause to restrict the rows affected",
  "location": { "line": 1, "column": 1 }
}
```

### Verdict

The verdict is the aggregated outcome for the entire audit request. It is computed from all findings across all statements:

| Verdict | Condition |
|---|---|
| `reject` | At least one finding with level `blocker` is present |
| `review` | No blockers are present, but at least one finding with level `warning` is present |
| `pass` | No blockers or warnings are present; notice-only results also remain `pass` |

The verdict reflects the request as a whole, not individual statements. A single blocker in any statement produces a `reject` verdict regardless of how many other statements pass cleanly.

### --fail-on and Exit Codes

The `--fail-on` flag controls which verdict level causes the CLI to exit with code 1. This lets you tune CI gate strictness without changing rules or policy.

| `--fail-on` value | Exit 1 when | Exit 0 when |
|---|---|---|
| `blocker` (default) | Any blocker finding is present | No blockers (warnings and notices allowed) |
| `warning` | Any blocker or warning finding is present | No blockers or warnings (notices allowed) |
| `notice` | Any finding of any level is present | No findings at all |
| `none` | Never | Always |

**CLI exit code reference:**

| Exit code | Meaning |
|---|---|
| `0` | Audit completed; findings are below the `--fail-on` threshold |
| `1` | Audit completed; findings crossed the `--fail-on` threshold |
| `2` | Bad user input: invalid flags, malformed SQL, unreadable config file |
| `3` | Runtime or internal failure |

---

## Offline-First

Offline-first is the default operating mode. DeltaScope can parse, normalize, and evaluate SQL with no database connection at any stage.

This means:

- You can run `deltascope audit` on a developer laptop with no access to the target database.
- CI pipelines can audit SQL migrations without connecting to a staging or production instance.
- AI agents can call the library without injecting database credentials into their execution context.

The offline path is not a reduced-capability mode. All offline-eligible rules run at full fidelity. Rules that require live schema facts are registered normally but no-op gracefully when no metadata is attached — they never block an offline audit or produce spurious errors.

Metadata-aware mode (see [Metadata-Aware Mode](./metadata-aware-mode.md)) adds live enrichment on top of the same offline pipeline; it does not replace it.

---

## Policy and Rules

**Rules** are the checks built into DeltaScope. They are versioned with the product and stable across patch and minor releases within a major version. Rules are organized by domain:

- `ddl.*` — CREATE TABLE governance, ALTER TABLE restrictions, object lifecycle
- `dml.*` — WHERE/LIMIT requirements, subquery guards, INSERT restrictions

**Policy** is a YAML configuration file that controls how rules behave at runtime:

- Enable or disable individual rules by rule ID
- Override the default severity level of a rule
- Supply per-rule parameters (e.g., maximum allowed columns, required comment patterns)

Policy changes alter evaluation behavior without any code changes. The same binary can enforce different standards across teams or environments by using different policy files.

`policy.Default()` is the shipped baseline policy. To inspect it:

```bash
deltascope config show-default
```

To generate a local copy for customization:

```bash
deltascope config init
```

To validate a config file:

```bash
deltascope config lint --config ./deltascope.yaml
```

---

## Rule Evaluation Order

Rules are evaluated **deterministically in registration order**. This means:

- Given the same SQL and policy, findings are always produced in the same order.
- Adding or removing rules does not reorder findings from other rules.

There are two evaluation scopes:

**Statement-scoped rules** are applied to each statement independently. A rule such as `dml.where.require` is evaluated once per DML statement and produces findings for that statement only.

**Global rules** are applied once, with access to all statements in the batch. They can detect patterns that span multiple statements.

Example of a global rule: `ddl.alter.merge.mysql.require` — this rule inspects all ALTER TABLE statements in the batch and warns when multiple ALTER TABLE operations target the same table. Because it must see all statements before deciding, it runs as a global rule after all statement-scoped rules have completed.

---

## Rule IDs

Rule IDs follow a stable dotted format:

```
ddl.<area>.<check>
dml.<area>.<check>
```

Examples:

| Rule ID | What it checks |
|---|---|
| `dml.where.require` | DML statements must have a WHERE clause |
| `dml.limit.require` | SELECT statements must include a LIMIT clause |
| `ddl.column.comment.require` | Columns must have a comment |
| `ddl.table.comment.require` | Tables must have a comment |
| `ddl.alter.merge.mysql.require` | Multiple ALTER TABLE on the same table should be merged |
| `ddl.index.key_length.check` | Index key length must not exceed instance limits |

Rule IDs are stable within a major version. Renaming or removing a rule constitutes a major-version change.

To explore rules:

```bash
deltascope rules list                      # list all rules
deltascope rules explain dml.where.require # detailed info for one rule
deltascope rules list --search "where"     # full-text search
```
