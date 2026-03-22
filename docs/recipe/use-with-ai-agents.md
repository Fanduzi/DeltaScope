# Use With AI Agents

The most stable handoff for agents is JSON output plus explicit rule IDs.

## Example

```bash
deltascope audit --sql "drop table users" --format json --quiet
```

## Suggested Loop

1. Send SQL to DeltaScope in JSON mode.
2. Inspect `verdict`, `global_findings`, and per-statement findings.
3. Feed the returned `rule_id` values into `deltascope rules show`.
4. Ask the agent to revise SQL only after the failing rules are explicit.
