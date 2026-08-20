# MCP Interface Module

Thin MCP adapter for exposing DeltaScope audit and rule-discovery capabilities to agent clients.

## Files

| File | Responsibility |
|------|---------------|
| `audit_tool.go` | Implements the MCP `audit_sql` tool on top of the shared DeltaScope audit path and renders compact finding-summary text, including the offline existence caveat |
| `audit_tool_test.go` | Verifies `audit_sql` compact text, structured result, empty-SQL `bad_request`, and offline `context.note` / `context.unproven` |
| `audit_tool_postgresql_tag_test.go` | Verifies compact review-verdict text on the PostgreSQL-capable build |
| `connection.go` | Resolves `connection_ref` inputs, delegates direct connection validation/password lookup to `internal/interfaces/metadata`, and assembles MCP connection state |
| `connection_test.go` | Verifies MCP connection normalization and safety rules |
| `output_schema.go` | Publishes explicit success output schemas for official MCP tools |
| `rule_tools.go` | Builds structured payloads and compact `list_rules` text for MCP rule-discovery tools, including `note` / `unproven` on `get_capabilities` context_fields |
| `rule_tools_test.go` | Verifies compact `list_rules` rows, the text-only surface, `describe_rule`, and `get_capabilities` |
| `server.go` | Builds the MCP server and registers the official DeltaScope tools |
| `server_test.go` | Verifies MCP bootstrap metadata, core tool registration, and metadata-aware omission of offline existence caveats |
| `tool_errors.go` | Shapes stable structured MCP tool errors and error-code mapping |

## Exports

- `AuditSQLParams`
- `Config`
- `NewServer(config)`
- `ResolveAuditConnection(params, options)`

## Notes

- The MCP layer stays thin and reuses shared DeltaScope audit, rule-catalog, metadata-preparation, and direct-connection helper logic.
- `audit_sql` `content[0].text` is a compact finding summary (verdict, counts, offline existence caveat when present, `[level] rule_id: message`, suggestion). `structuredContent` stays the full public result plus MCP `context`. Offline `context` includes `note` / `unproven`; metadata-aware `context` omits them. The text is not a second JSON copy and does not dump skipped rules or echo the audited SQL.
- `list_rules` returns compact catalog rows (`rule_id`, `level`, `dialect`, `kind`, `summary`) plus a text table. `describe_rule` remains the full-body tool. `content[0].text` is the text-only surface and is not a second JSON copy of `structuredContent`.
- `query_access_surface_contract_test.go` is the sole MCP proof that Query Access remains absent; no Query Access MCP tool is introduced.
- The current scope supports stdio MCP bootstrap, offline audit for MySQL, TiDB, and PostgreSQL, plus metadata-aware audit for MySQL/TiDB-compatible instances and PostgreSQL on the PG-capable builds.
- Connection-backed PostgreSQL MCP audit requests follow the same shared metadata-preparation path as the other transports and should preserve explicit metadata-aware context rather than downgrading silently.
- `get_capabilities` is MCP-client-facing and summarizes transport, official tool names, audit modes, dialect support, top-level and connection inputs, audit result fields, context fields (`mode`, `dialect`, `dialect_source`, `schema`, `schema_source`, `metadata_source`, `note`, `unproven`), metadata features, and the stable structured error codes the server advertises (`bad_request`, `connection_invalid`, `connection_failed`, `config_invalid`).
- Audit results also carry additive `unsupported` (`[]spec.UnsupportedDetail`) and `diagnostics` (`[]spec.Diagnostic`) arrays for partial-support and parser-error outcomes; both are omitted when empty and are not listed in the `result_fields` summary.
- `connect_timeout` is an accepted direct and named connection input (duration string like `5s`); empty/omitted/`0s` falls back to runtime config default, invalid/negative values return `connection_invalid`. It is not listed in the `connection_inputs` summary.
- In addition to the structured errors `get_capabilities` advertises, recovered tool panics return `internal_error`; this code is not part of the advertised `structured_errors` list.
- `tool_errors.go` maps `connection connect_timeout` validation errors to `connection_invalid`.

## Dependencies

- Upstream: `cmd/deltascope-mcp`
- Downstream: shared audit/rule-catalog layers under `pkg/deltascope`, metadata helpers under `internal/interfaces/metadata`, and other `internal/...` adapter layers

## Update Rule

- If members/interfaces/dependencies change, update this file in same change.
