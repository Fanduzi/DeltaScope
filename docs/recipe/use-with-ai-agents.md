# Use With AI Agents

DeltaScope is designed for AI agent integration. The JSON output format provides stable, machine-readable findings that agents can reason about without parsing text. Rule IDs are stable across patch versions, and every finding includes a `suggestion` field that gives the agent an actionable fix.

## Why JSON Mode

- **Stable field names** across versions — agents can rely on `verdict`, `rule_id`, `level`, `message`, and `suggestion` without adapting to text changes.
- **`rule_id` values are stable** — agents can look them up with `deltascope rules show` to get full descriptions and fix guidance.
- **`verdict` field** gives a single actionable outcome: `pass` / `review` / `reject` — no text parsing required.
- **`global_findings`** captures cross-statement issues (e.g., the merge-alter rule fires when multiple `ALTER TABLE` statements target the same table in one batch).
- **`--quiet` flag** suppresses progress output so only the JSON object is written to stdout — safe to capture with `$(...)` or pipe to `jq`.

## Basic Integration

```bash
deltascope audit --sql "$SQL" --format json --quiet
```

Complete JSON output example for a risky DML statement:

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
          "location": {
            "line": 1,
            "column": 1
          }
        }
      ]
    }
  ],
  "global_findings": []
}
```

## Looking Up Rule Details

When a finding contains a `rule_id`, agents can fetch the full rule description including parameters:

```bash
deltascope rules show dml.where.require
```

Output:

```
Rule ID:     dml.where.require
Kind:        dml
Level:       blocker
Description: UPDATE or DELETE must include a WHERE clause to prevent full-table modifications
Metadata:    false
Params:
  required (bool, default: true)
```

This gives the agent precise vocabulary for the fix: it knows the rule name, the severity, whether metadata is needed, and what parameter controls it. The agent can use this to instruct the user or to self-correct the generated SQL.

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
   deltascope rules show <rule_id>
   ```
   Use the `Description` and `Params` fields to understand exactly what is required.
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

When fixing findings, use `deltascope rules show <rule_id>` to understand the exact requirement before making changes.
```

## MCP / Tool-Use Integration

DeltaScope can be wrapped as a tool in an MCP server or agent tool definition. The shell command is a thin, stateless wrapper — ideal for tool-use patterns.

Example tool definition (pseudocode / JSON schema):

```json
{
  "name": "audit_sql",
  "description": "Audit SQL statements for policy violations using DeltaScope. Returns verdict (pass/review/reject), per-statement findings with rule IDs, severity levels, and suggestions for fixing violations. Call this before returning any SQL to the user.",
  "parameters": {
    "type": "object",
    "properties": {
      "sql": {
        "type": "string",
        "description": "The SQL text to audit. May contain multiple statements separated by semicolons."
      },
      "dialect": {
        "type": "string",
        "enum": ["mysql", "tidb"],
        "default": "mysql",
        "description": "SQL dialect. Use 'tidb' for TiDB targets."
      }
    },
    "required": ["sql"]
  }
}
```

The tool implementation runs:

```bash
deltascope audit --sql "$SQL" --dialect "$DIALECT" --format json --quiet
```

And returns the JSON output directly to the agent — no parsing needed.

Example rule lookup tool definition:

```json
{
  "name": "describe_rule",
  "description": "Get the full description, severity, and parameters for a DeltaScope rule ID. Use this after audit_sql returns findings to understand what each rule requires.",
  "parameters": {
    "type": "object",
    "properties": {
      "rule_id": {
        "type": "string",
        "description": "The rule ID from an audit finding, e.g. 'dml.where.require'"
      }
    },
    "required": ["rule_id"]
  }
}
```

Implementation:

```bash
deltascope rules show "$RULE_ID"
```

## Stable Contracts Agents Can Rely On

| Field | Guarantee |
|-------|-----------|
| `verdict` | Always one of `"pass"` / `"review"` / `"reject"` |
| `rule_id` | Stable within a major version; safe to store and look up |
| `level` | Always `"blocker"` / `"warning"` / `"notice"` |
| `message` | Human-readable; may change across versions — do not parse |
| `suggestion` | Present when available; gives actionable fix guidance |
| `global_findings` | Always present as an array (may be empty) |
| Exit code `0` | Findings below `--fail-on` threshold (or no findings) |
| Exit code `1` | Findings crossed `--fail-on` threshold |
| Exit code `2` | Bad input or config — do not retry without fixing input |
| Exit code `3` | Internal error — surface to user, do not retry silently |
