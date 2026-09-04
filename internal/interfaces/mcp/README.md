# MCP Interface Module

Thin MCP adapter for exposing DeltaScope audit and rule-discovery capabilities to agent clients.

## Files

| File | Responsibility |
|------|---------------|
| `audit_tool.go` | Implements `audit_sql` on the shared DeltaScope path, including catalog-aware metadata audits, and passes full partial public results and context into bounded diagnostic tool errors |
| `audit_tool_test.go` | Verifies `audit_sql` compact text, structured result, empty-SQL `bad_request`, and offline `context.note` / `context.unproven` |
| `audit_impact_postgresql_tag_test.go` | Verifies PostgreSQL offline primary-key equality impact in MCP structured output |
| `audit_dml_table_existence_test.go` | Verifies metadata-aware MySQL/TiDB INSERT/UPDATE/DELETE missing-target findings and stable MCP structured-result shape |
| `audit_tool_postgresql_tag_test.go` | Verifies compact review-verdict text on the PostgreSQL-capable build |
| `server_metadata_postgresql_tag_test.go` | Verifies PostgreSQL metadata-aware MCP result behavior and separate database/schema propagation |
| `connection.go` | Resolves `connection_ref` inputs, delegates direct connection validation/password lookup to `internal/interfaces/metadata`, and assembles MCP connection state while preserving separate database/catalog and schema values |
| `connection_test.go` | Verifies MCP connection normalization and safety rules |
| `output_schema.go` | Publishes explicit success output schemas for official MCP tools |
| `rule_tools.go` | Builds structured payloads and compact `list_rules` text for MCP rule-discovery tools, including `query_access` unavailability on `get_capabilities`, database-aware connection inputs, and `note` / `unproven` on `get_capabilities` context_fields |
| `rule_tools_test.go` | Verifies compact `list_rules` rows, the text-only surface, `describe_rule`, and `get_capabilities` including Query Access unavailability |
| `server.go` | Builds the MCP server and registers the official DeltaScope tools |
| `server_test.go` | Verifies MCP bootstrap, registration, metadata-aware context, and parser-error partial-result preservation |
| `server_unsupported_diagnostics_evidence_test.go` | Verifies MCP parser errors preserve review-floored partial results, audited siblings/findings, context, structured error signaling, locations, and no-leak boundaries |
| `server_unsupported_verdict_floor_postgresql_tag_test.go` | Verifies MCP PostgreSQL `SELECT 1` keeps tool-error signaling and serializes the review-floored unsupported result |
| `tool_errors.go` | Shapes stable structured MCP tool errors while embedding the partial audit result and context when diagnostics are present |

## Exports

- `AuditSQLParams`
- `Config`
- `NewServer(config)`
- `ResolveAuditConnection(params, options)`

## Notes

- The MCP layer stays thin and reuses shared DeltaScope audit, rule-catalog, metadata-preparation, and direct-connection helper logic.
- `audit_sql` `content[0].text` is a compact finding summary (verdict, counts, offline existence caveat when present, `[level] rule_id: message`, suggestion). `structuredContent` stays the full public result plus MCP `context`. Offline `context` includes `note` / `unproven`; metadata-aware `context` omits them. The text is not a second JSON copy and does not dump skipped rules or echo the audited SQL.
- `list_rules` returns compact catalog rows (`rule_id`, `level`, `dialect`, `kind`, `summary`) plus a text table. `describe_rule` remains the full-body tool. `content[0].text` is the text-only surface and is not a second JSON copy of `structuredContent`.
- `query_access_surface_contract_test.go` is the MCP proof that Query Access remains absent from the tool list; no Query Access MCP tool is introduced. `get_capabilities` declares the gap as `query_access: { available: false, surfaces: ["cli", "http"] }`.
- The current scope supports stdio MCP bootstrap, offline audit for MySQL, TiDB, and PostgreSQL, plus metadata-aware audit for MySQL/TiDB-compatible instances and PostgreSQL on the PG-capable builds.
- Connection-backed PostgreSQL MCP audit requests follow the same shared metadata-preparation path as the other transports and should preserve explicit metadata-aware context rather than downgrading silently.
- `get_capabilities` is MCP-client-facing and summarizes transport, official tool names, Query Access unavailability on MCP (`query_access.available` is `false`; `query_access.surfaces` is the stable list `cli`, then `http`), audit modes, dialect support, top-level and connection inputs (including `connection.database` and `connection.connect_timeout`), audit result fields, context fields (`mode`, `dialect`, `dialect_source`, `schema`, `schema_source`, `metadata_source`, `note`, `unproven`), metadata features, and the stable structured error codes the server advertises (`bad_request`, `connection_invalid`, `connection_failed`, `config_invalid`).
- Audit results also carry additive `unsupported` (`[]spec.UnsupportedDetail`) and `diagnostics` (`[]spec.Diagnostic`) arrays. Parser-error tool results retain valid audited statements/findings, the shared partial-result review floor, and normal MCP context inside `structuredContent`, add a bounded code/message, and remain errors so clients cannot treat a partial audit as success; empty arrays are omitted and are not listed in `result_fields`.
- `connect_timeout` is an accepted direct and named connection input (duration string like `5s`) and is advertised as `connection.connect_timeout`; empty/omitted/`0s` falls back to runtime config default, invalid/negative values return `connection_invalid`.
- In addition to the structured errors `get_capabilities` advertises, recovered tool panics return `internal_error`; this code is not part of the advertised `structured_errors` list.
- `tool_errors.go` maps `connection connect_timeout` validation errors to `connection_invalid`.

## Dependencies

- Upstream: `cmd/deltascope-mcp`
- Downstream: shared audit/rule-catalog layers under `pkg/deltascope`, metadata helpers under `internal/interfaces/metadata`, and other `internal/...` adapter layers

## Update Rule

- If members/interfaces/dependencies change, update this file in same change.
