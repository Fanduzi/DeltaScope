# Decision: Compact MCP `audit_sql` Text

Date: 2026-08-18
Status: Accepted
Related milestone/version: issue #22
Related commits:
Related tests:
- `TestAuditSQLCallTextIncludesFindingSummary`
- `TestAuditSQLCallTextIsNotStructuredJSON`
- `TestAuditSQLCallTextOmitsSQLAndSkippedRules`
- `TestAuditSQLCallTextIncludesCreateTableFindings`
- `TestAuditSQLCallTextIncludesPostgreSQLDropColumnFinding`
- `TestAuditSQLEmptySQLRemainsBadRequest`
- `TestAuditSQLToolReturnsStructuredErrorForEmptySQL`
Related docs:
- `docs/recipe/use-deltascope-mcp.md`
- `docs/recipe/use-deltascope-mcp.zh-CN.md`
- `docs/recipe/use-with-ai-agents.md`
- `docs/recipe/use-with-ai-agents.zh-CN.md`

## Context

Successful MCP `audit_sql` results already returned the full public audit object in
`structuredContent`, but `content[0].text` was only `Audit verdict: …` (19–21
bytes). Findings, suggestions, and rule IDs lived only in `structuredContent`.
On v0.480.0 and current `main`, `delete from users` therefore gave text-only
hosts a reject verdict with no way to fix the statement.

Parent tracker #26 states the shared MCP content contract: `structuredContent`
is machine-readable; `content[0].text` is the text-only surface; never serialize
the structured payload a second time as JSON in `content`. `list_rules` (#18)
is the opposite density bug and is out of scope here.

## Decision

`audit_sql` success text is a compact finding summary:

```text
Audit verdict: reject
Statements: 1
Blockers: 1
Warnings: 0
Notices: 0

[blocker] dml.where.require: UPDATE and DELETE statements must include a WHERE clause
  Suggestion: add a WHERE clause that narrows the affected rows
```

Each finding is one `[level] rule_id: message` line. A `Suggestion:` line is
emitted when `finding.suggestion` is set, otherwise when
`finding.explanation.suggestion` is set. That fallback is how catalog/rule
advice reaches text-only clients for findings that leave the top-level field
empty, including `ddl.pg.alter.drop_column.advisory`. Statement findings appear
in statement order, then global findings.

`structuredContent` remains the full public audit result plus MCP `context`.
`content[0].text` is not `json.Marshal` of that object.

## Rationale

Text-only MCP hosts never see `structuredContent`. A short finding list is the
smallest change that lets an agent read rule IDs, messages, and suggestions
without duplicating the structured payload. Echoing the full JSON is the #18
failure mode. Dumping skipped rules is a CLI markdown concern (#17), not an MCP
audit-text concern.

An explicit `CallToolResult.Content` is required because the Go MCP SDK fills
empty `Content` with the structured JSON.

## Public Contract

- `content[0].text` includes verdict, statement/blocker/warning/notice counts,
  and each finding as `[level] rule_id: message`, plus `Suggestion:` from
  `finding.suggestion` or, if that is empty, `finding.explanation.suggestion`.
- Typical `delete from users` text stays on the order of 1–2 KB, not hundreds
  of KB.
- `content[0].text` is not a second copy of the structured payload.
- `structuredContent` still has the full result (`verdict`, `summary`,
  `statements`, findings, `context`).
- No skipped-rule dump is added.
- The audited SQL is not echoed into the text surface.
- Empty SQL remains `bad_request` with `audit SQL must not be empty`.
- `capability_version` stays `mcp-v1`.

## Privacy / No-Leak Contract

`audit_sql` text does not echo the request SQL. It repeats existing public
finding fields (`level`, `rule_id`, `message`, `suggestion`) only. Finding
messages may still mention identifiers the rules already emit.

## Deferred / Out Of Scope

- Changing rule evaluation or finding messages
- Compact `list_rules` catalog output (#18)
- CLI markdown skipped-rule dump (#17)
- Changing MCP tool names or adding Query Access
- Pagination or truncation of very large finding lists
- Changing HTTP or CLI audit output

## Verification Evidence

In-process MCP `CallTool` tests lock the `delete from users` literals, the
create-table count of 3 blockers and 6 warnings, a text body that is not
structured JSON and does not echo SQL or skipped-rule dumps, empty SQL as
`bad_request`, and the PostgreSQL drop-column review finding on the
`postgresql`-tagged build.

## Consequences

Text-only MCP clients can now act on `audit_sql` without reading
`structuredContent`. Structured clients are unchanged. Future MCP tools must
set `Content` when the text-only surface should differ from
`structuredContent`.

## Links

- Issue: https://github.com/Fanduzi/DeltaScope/issues/22
- Parent: https://github.com/Fanduzi/DeltaScope/issues/26
- Tests: `internal/interfaces/mcp/audit_tool_test.go`,
  `internal/interfaces/mcp/audit_tool_postgresql_tag_test.go`,
  `internal/interfaces/mcp/server_test.go`
- Implementation: `internal/interfaces/mcp/audit_tool.go`
