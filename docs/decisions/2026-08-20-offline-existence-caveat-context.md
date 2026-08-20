# Decision: Offline Existence Caveat on CLI, HTTP, and MCP Context

Date: 2026-08-20
Status: Accepted
Related milestone/version: issue #28
Related commits:
Related tests:
- `TestOfflineDropColumnMissingStaysPassAndStatesExistenceNotChecked`
- `TestQuietJSONOfflineDropColumnKeepsStatementFindingsAndContextCaveat`
- `TestHandlerOfflineDropColumnStatesExistenceNotChecked`
- `TestHandlerCapabilitiesListsOfflineExistenceContextFields`
- `TestAuditSQLOfflineDropColumnStatesExistenceNotChecked`
- `TestGetCapabilitiesToolReturnsKnownSummary`
Related docs:
- `docs/recipe/use-with-ai-agents.md`
- `docs/recipe/audit-sql-offline.md`
- `docs/reference/http-api.md`
- `docs/reference/cli.md`
- `docs/concept/metadata-aware-mode.md`

## Context

Offline `ALTER ... DROP COLUMN` (and similar existence-dependent DDL) still
`pass`es because existence rules no-op without a table snapshot. That policy
is correct: do not fail an offline audit just because metadata is missing.

The presentation bug in #28 was that notices claimed the object existed, and
`mode=offline` was a label rather than a limitation. Agents that key on
`verdict` would treat the statement as safe to apply. The unique
metadata-aware value (existence blockers) is invisible on the default,
no-connection path.

The first implementation put `context.note` / `context.unproven` on CLI JSON
only, while agent docs told consumers to read those fields. HTTP
`executeOfflineAudit` and MCP `auditSQLOffline` already had the same `context`
envelope (`mode`, `dialect`, `dialect_source`, `schema`, `schema_source`,
`metadata_source`) but omitted the new fields. MCP is the default agent
surface.

## Decision

When a transport adapter runs the offline path (no metadata snapshot), it
emits the same caveat on `context`:

- `note`: `existence not checked (no database connection)`
- `unproven`: `["column_exists", "table_exists"]`

CLI, HTTP, and MCP share those values through
`internal/interfaces/metadata`. Metadata-aware `context` omits both fields
(`omitempty`).

MCP `content[0].text` adds that one `note` line to the compact finding
summary. It does not serialize `context` or `unproven` as JSON in the text
surface.

`pkg/deltascope.Result` does not gain `context`. SDK callers already know
whether they passed a `MetadataProvider`.

Verdict stays `pass` / exit 0 for notice-only offline ALTER. CREATE policy
pack, default rule levels, and the `--host` / connection requirement are
unchanged.

## Rationale

`mode=offline` is still a reliable synonym for "no snapshot" today, but #28
asked not to make humans or agents infer safety from a mode label. Putting
the fields only on CLI would teach agents a signal that MCP does not emit.

The three transports already share a `context` envelope. Adding two
`omitempty` fields there is additive. A shared helper keeps the English string
and `unproven` tokens from forking.

SDK `Result` is a different contract: it has no `context` today, and adding
one would be a public library shape change that library callers do not need.

## Public Contract

Offline CLI JSON, HTTP `POST /v1/audit`, and MCP `audit_sql`
`structuredContent.context`:

| Field | When present | Value |
|-------|----------------|-------|
| `note` | Offline only | `existence not checked (no database connection)` |
| `unproven` | Offline only | `["column_exists", "table_exists"]` |

HTTP `GET /v1/capabilities` and MCP `get_capabilities` advertise `note` and
`unproven` in `context_fields`.

MCP `content[0].text` includes the `note` line when the offline path ran.
It remains a compact finding summary, not a second copy of
`structuredContent`.

CLI `--quiet --format json` is unchanged: no top-level `findings` array;
findings stay on `statements[].findings`; the caveat is on `context`.

Do not infer "existence was checked" from `verdict=pass` or `mode=offline`
alone.

## Deferred / Out Of Scope

- Putting `context` on `pkg/deltascope.Result`
- Changing CREATE policy or default rule levels
- Requiring `--host` / a connection for offline audit
- Dumping skipped-rule IDs (#17)
- Connection-error handling (#23)
- Rewording other existence-flavored notices (DROP INDEX still says
  "removes an existing index")
- File-based or dump-based snapshots that would make offline mode able to
  prove existence

## Verification Evidence

CLI tests lock markdown Action Summary / Audit Context, quiet `[context]`,
and JSON `context.note` / `context.unproven` for offline DROP COLUMN and
missing-table ALTER, plus `--quiet --format json` and metadata-aware omit.

HTTP tests lock offline DROP COLUMN JSON context and capabilities
`context_fields`. Metadata-aware HTTP context omits `note` / `unproven`.

MCP tests lock offline DROP COLUMN structured context, the compact text
line, and `get_capabilities` `context_fields`. Metadata-aware MCP context
omits `note` / `unproven`. Text is not a JSON dump of `context`.

## Consequences

Future offline-capable transports must emit the same `note` / `unproven`
instead of inventing a parallel phrase. If a later path can prove existence
without a live connection, `mode=offline` must no longer imply these fields;
the fields follow "no snapshot", not the mode name.

Agents should read `context.note` / `context.unproven` on CLI, HTTP, and MCP.
SDK callers continue to inspect whether they supplied a metadata provider.

## Links

- Issue: https://github.com/Fanduzi/DeltaScope/issues/28
- Related: `docs/decisions/2026-08-18-mcp-audit-sql-text.md`
- Tests: `internal/interfaces/cli/cli_offline_existence_test.go`,
  `internal/interfaces/http/audit_offline_existence_test.go`,
  `internal/interfaces/mcp/audit_tool_test.go`
- Helper: `internal/interfaces/metadata/existence.go`
