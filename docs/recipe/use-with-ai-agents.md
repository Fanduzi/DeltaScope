# Use With AI Agents

DeltaScope is designed for AI agent integration. The JSON output format provides stable, machine-readable findings that agents can reason about without parsing text. Rule IDs are stable within a major version, and findings now include structured `explanation` fields so agents can distinguish the summary, why, risk, suggestion, and metadata-availability notes without scraping prose.

## Why JSON Mode

- **Stable field names** across versions — agents can rely on `verdict`, `rule_id`, `level`, `message`, `suggestion`, and nested `explanation` fields without adapting to text changes.
- **`rule_id` values are stable** — agents can look them up with `deltascope rules show` to get full descriptions and fix guidance.
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
deltascope rules show dml.where.require
```

Output:

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

This gives the agent stable rule metadata in one response: rule ID, defaults, supported statement kinds, trigger examples, config knobs, and remediation guidance.

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

When fixing findings, use `deltascope rules show <rule_id>` to understand the exact requirement before making changes.
```

## MCP / Tool-Use Integration

DeltaScope now ships an official MCP stdio server via `deltascope-mcp`. It exposes a fixed tool surface for agent clients instead of relying on an ad hoc shell wrapper.

Official tools:

- `audit_sql`: audit one SQL payload and return the normal DeltaScope result body plus `context`
- `describe_rule`: return the shipped metadata for one rule ID
- `list_rules`: search the shipped rule catalog
- `get_capabilities`: return MCP-client-facing server capabilities, result fields, connection inputs, and stable error codes

Example MCP client configuration:

```json
{
  "mcpServers": {
    "deltascope": {
      "command": "deltascope-mcp",
      "args": []
    }
  }
}
```

Use `audit_sql` for the actual audit call. The request contract supports:

- `sql`
- `dialect`
- `config_path`
- `connection_ref`
- `connection.host`
- `connection.port`
- `connection.socket`
- `connection.user`
- `connection.schema`
- `connection.dialect`
- `connection.password`
- `connection.password_env`
- `connection.password_file`

If both `dialect` and `connection.dialect` are present, the top-level `dialect` wins.

Success responses preserve the normal DeltaScope audit result fields and add:

- `context.mode`
- `context.dialect`
- `context.dialect_source`
- `context.schema`
- `context.schema_source`
- `context.metadata_source`

The advertised result fields from `get_capabilities` include:

- `verdict`
- `summary`
- `statements`
- `global_findings`
- `explanation`
- `context`

Structured tool errors use stable codes:

- `bad_request`
- `connection_invalid`
- `connection_failed`
- `config_invalid`

Rule lookup stays available through the MCP server too. Example `describe_rule` input:

```json
{
  "rule_id": "dml.where.require"
}
```

If a client needs a compact contract summary before its first call, use `get_capabilities`.
That summary includes top-level audit inputs such as `sql`, `dialect`, `config_path`, `connection_ref`, `connection`, plus the rules that `connection_ref` and `connection` are mutually exclusive and top-level `dialect` overrides `connection.dialect`.

For `connection_ref`, `deltascope-mcp` reads `~/.config/deltascope/connections.yaml` by default. You can override that with `-connections-path /path/to/connections.yaml`.

Expected file shape:

```yaml
connections:
  prod_readonly:
    host: 10.0.0.12
    port: 3306
    user: audit_bot
    schema: app
    dialect: mysql
    password_env: PROD_DB_PASSWORD
```

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
