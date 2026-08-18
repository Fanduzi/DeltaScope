# Decision: Compact MCP `list_rules` Output

Date: 2026-08-18
Status: Accepted
Related milestone/version: issue #18
Related commits:
Related tests:
- `TestListRulesCallReturnsCompactCatalogRows`
- `TestListRulesCallTextIsNotStructuredJSON`
- `TestListRulesCallQueryWhereReturnsOneCompactRule`
- `TestDescribeRuleCallStillReturnsFullBody`
- `TestListRulesToolFiltersByQuery`
- `TestNewServerPublishesOutputSchemasForCoreTools`
Related docs:
- `docs/recipe/use-deltascope-mcp.md`
- `docs/recipe/use-deltascope-mcp.zh-CN.md`
- `docs/recipe/use-with-ai-agents.md`
- `docs/recipe/use-with-ai-agents.zh-CN.md`

## Context

Unfiltered MCP `list_rules` returned every shipped rule as a full
`describe_rule` body. Returning a nil tool result also let the Go MCP SDK copy
that object into `content[0].text` as a second JSON string. On v0.480.0 and
current `main` that produced about 371 full-body rows, ~458 KB of structured
JSON, the same JSON again as text, and a ~0.9 MB JSON-RPC response.

Agents call `list_rules` to discover IDs, then `describe_rule` for one rule.
The default dump filled the context window before an audit could run. CLI
`deltascope rules list` was already a compact table (~41 KB). HTTP `GET /v1/rules`
was not part of this bug.

Parent tracker #26 states the shared MCP content contract: `structuredContent`
is machine-readable; `content[0].text` is the text-only surface; never serialize
the structured payload a second time as JSON in `content`.

## Decision

Default MCP `list_rules` returns compact catalog rows only:

| Field | Meaning |
| --- | --- |
| `rule_id` | Stable shipped rule identifier |
| `level` | Default policy level (`blocker` / `warning` / `notice`) |
| `dialect` | Dialect scope, comma-joined when a rule has more than one |
| `kind` | Statement kind, comma-joined when a rule has more than one |
| `summary` | One-line catalog summary |

`describe_rule` remains the full-body tool and keeps its existing field names
(`default_level`, `statement_kinds`, why / risk / suggestion / examples).

`list_rules` returns an explicit text table in `content[0].text`. That table
lists the same compact columns. It is not `json.Marshal` of `structuredContent`.

Optional `query` keeps using the existing catalog search. A `query=where`
result stays one compact row for `dml.where.require`.

## Rationale

Compacting the default list is the smallest fix that matches how agents
already use the two tools. Requiring `query`, or adding `limit` / `offset`,
would change discovery more than the bug requires. Pagination stays deferred
unless a compact full dump is still too large.

Compact `list_rules` field names follow the CLI list vocabulary (`level`,
`kind`, `dialect`) rather than the describe-rule names (`default_level`,
`statement_kinds`). List is an index; describe is the full record.

An explicit `CallToolResult.Content` is required because the Go MCP SDK fills
empty `Content` with the structured JSON. Setting text ourselves is how the
parent content contract is enforced.

## Public Contract

- Unfiltered `list_rules` returns every shipped rule as a compact row, not a
  full describe body.
- `structuredContent` shape is `{count, rules[], query?}` where each rule has
  only `rule_id`, `level`, `dialect`, `kind`, and `summary`.
- `content[0].text` is a compact text table of those columns, not a JSON
  object or array dump of the structured payload.
- Response size stays far below the previous ~0.9 MB / 458 KB dump. CLI
  `rules list` (~41 KB without summaries) is the size reference.
- `query` still filters. `query=where` returns one compact `dml.where.require`
  row.
- `describe_rule` still returns the full catalog body for one rule ID.
- `audit_sql` is unchanged by this decision.

## Privacy / No-Leak Contract

`list_rules` and `describe_rule` still emit shipped catalog metadata only.
They do not accept, parse, or echo user SQL, credentials, connection strings,
or parser fragments.

## Deferred / Out Of Scope

- Requiring `query` on `list_rules`
- `limit` / `offset` pagination
- Changing `audit_sql` text (#22)
- Changing CLI markdown skipped-rule output (#17)
- Changing HTTP `GET /v1/rules`, which still returns full describe bodies
- Changing rule evaluation, finding JSON, or catalog search semantics
- Bumping `capability_version` (`mcp-v1` stays)

## Verification Evidence

In-process MCP `CallTool` tests lock compact row keys, the `dml.where.require`
literals, `query=where` count `1`, a text table that is not structured JSON,
and an unchanged `describe_rule` full body. The published output schema for
`list_rules` items includes the five compact fields and omits describe-only
fields.

## Consequences

MCP clients that parsed full `list_rules` bodies must call `describe_rule` for
why / risk / suggestion / examples. Future MCP tools must set `Content` when
the text-only surface should differ from `structuredContent`. HTTP rule list
parity is a separate decision.

## Links

- Issue: https://github.com/Fanduzi/DeltaScope/issues/18
- Parent: https://github.com/Fanduzi/DeltaScope/issues/26
- Tests: `internal/interfaces/mcp/rule_tools_test.go`,
  `internal/interfaces/mcp/server_test.go`
- Implementation: `internal/interfaces/mcp/rule_tools.go`,
  `internal/interfaces/mcp/server.go`
