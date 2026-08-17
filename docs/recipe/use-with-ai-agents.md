# Use With AI Agents

DeltaScope is designed for AI agent integration. The JSON output format provides stable, machine-readable findings that agents can reason about without parsing text. Rule IDs are stable within a major version, and findings now include structured `explanation` fields so agents can distinguish the summary, why, risk, suggestion, and metadata-availability notes without scraping prose.

## Why JSON Mode

- **Stable field names** across versions — agents can rely on `verdict`, `rule_id`, `level`, `message`, `suggestion`, and nested `explanation` fields without adapting to text changes.
- **`rule_id` values are stable** — agents can look them up with `deltascope rules explain` to get full descriptions and fix guidance.
- **`verdict` field** gives a single actionable outcome: `pass` / `review` / `reject` — no text parsing required.
- **`global_findings`** captures cross-statement issues (e.g., the merge-alter rule fires when multiple `ALTER TABLE` statements target the same table in one batch).
- **`--quiet` flag** is safe to combine with `--format json`; the JSON contract is unchanged, so the command remains safe to capture with `$(...)` or pipe to `jq`.

## Basic Integration

```bash
deltascope audit --sql "$SQL" --format json --quiet
```

Complete CLI JSON output example for a risky DML statement:

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
          "metadata": {
            "operation": "delete"
          },
          "explanation": {
            "summary": "Require DML where require",
            "why": "The statement is missing a clause, option, or object that the shipped policy requires.",
            "risk": "Ignoring this rule can allow high-impact data changes to proceed with less safety review.",
            "suggestion": "add a WHERE clause that narrows the affected rows"
          },
          "location": {
            "line": 1,
            "column": 1
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

## Looking Up Rule Details

When a finding contains a `rule_id`, agents can fetch the shipped rule summary, defaults, examples, and remediation guidance:

```bash
deltascope rules explain dml.where.require
```

Output:

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

This gives the agent stable rule metadata in one response: rule ID, defaults, supported statement kinds, trigger examples, a copyable safe override, and remediation guidance.

## Suggested Agent Workflow

1. Receive SQL from the user or from an upstream tool.
2. Audit the SQL:
   ```bash
   deltascope audit --sql "$SQL" --format json --quiet
   ```
3. Check the `verdict` field:
   - `"pass"` → return the SQL to the user directly.
   - `"review"` → explain the warnings to the user; offer to fix them.
   - `"reject"` → the SQL must be revised before returning it.
4. Collect all `rule_id` values from findings where `level == "blocker"`.
5. For each `rule_id`, run:
   ```bash
   deltascope rules explain <rule_id>
   ```
   Use the rule summary, default params, examples, and remediation sections to understand exactly what is required.
6. Revise the SQL based on the rule descriptions and suggestions.
7. Re-audit the revised SQL and repeat until `verdict` is `"pass"` (or `"review"` if warnings are acceptable).

**Tip:** Set a maximum retry count (e.g., 3) to prevent infinite loops. If the SQL still fails after N attempts, surface the raw findings to the user for manual review.

## Example System Prompt Fragment

Include this in your agent's system prompt to make DeltaScope part of its default behavior:

```
When the user asks you to write or modify SQL, always audit it using the deltascope tool before returning it.

Rules:
- If the verdict is "reject", fix ALL blocker findings before returning the SQL. Never return SQL with a "reject" verdict.
- If the verdict is "review", explain the warnings to the user and ask whether they want them fixed.
- If the verdict is "pass", return the SQL directly.

When fixing findings, use `deltascope rules explain <rule_id>` to understand the exact requirement before making changes.
```

## MCP / Tool-Use Integration

DeltaScope now ships an official MCP stdio server via `deltascope-mcp`. If you need setup commands, launcher usage, direct connection, `connection_ref`, proxy configuration, or generic stdio examples, use [Use DeltaScope MCP](use-deltascope-mcp.md).

The MCP tool surface remains:

- `audit_sql`
- `describe_rule`
- `list_rules`
- `get_capabilities`

For agent workflow design, the important contract points are:

- `audit_sql` preserves the normal DeltaScope audit result body and adds top-level `context`. `content[0].text` is a compact finding summary (verdict, counts, `[level] rule_id: message`, suggestion), not a second JSON copy of `structuredContent`
- `list_rules` returns compact catalog rows (`rule_id`, `level`, `dialect`, `kind`, `summary`); use `describe_rule` for the full body of one rule
- `connection_ref` and `connection` are mutually exclusive
- top-level `dialect` overrides `connection.dialect`
- stable structured tool errors use `bad_request`, `connection_invalid`, `connection_failed`, and `config_invalid`
- `get_capabilities` is the compact MCP contract summary for clients that want to inspect the surface before the first tool call

## Stable Contracts Agents Can Rely On

| Field | Guarantee |
|-------|-----------|
| `verdict` | Always one of `"pass"` / `"review"` / `"reject"` |
| `rule_id` | Stable within a major version; safe to store and look up |
| `level` | Always `"blocker"` / `"warning"` / `"notice"` |
| `message` | Human-readable; may change across versions — do not parse |
| `suggestion` | Present when available; gives actionable fix guidance |
| `explanation` | Present when available; provides structured summary / why / risk / suggestion / metadata notes |
| `global_findings` | Present when cross-statement rules emit findings; omitted when empty |
| Exit code `0` | Findings below `--fail-on` threshold (or no findings) |
| Exit code `1` | Findings crossed `--fail-on` threshold |
| Exit code `2` | Bad input or config — do not retry without fixing input |
| Exit code `3` | Internal error — surface to user, do not retry silently |
