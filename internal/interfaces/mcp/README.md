# MCP Interface Module

Thin MCP adapter for exposing DeltaScope audit and rule-discovery capabilities to agent clients.

## Files

| File | Responsibility |
|------|---------------|
| `audit_tool.go` | Implements the MCP `audit_sql` tool on top of the shared DeltaScope audit path |
| `connection.go` | Resolves `connection_ref` inputs, delegates direct connection validation/password lookup to `internal/interfaces/metadata`, and assembles MCP connection state |
| `connection_test.go` | Verifies MCP connection normalization and safety rules |
| `output_schema.go` | Publishes explicit success output schemas for official MCP tools |
| `rule_tools.go` | Builds structured payloads for MCP rule-discovery tools |
| `rule_tools_test.go` | Verifies `describe_rule`, `list_rules`, and `get_capabilities` behavior |
| `server.go` | Builds the MCP server and registers the official DeltaScope tools |
| `server_test.go` | Verifies MCP bootstrap metadata and core tool registration |
| `tool_errors.go` | Shapes stable structured MCP tool errors and error-code mapping |

## Exports

- `AuditSQLParams`
- `Config`
- `NewServer(config)`
- `ResolveAuditConnection(params, options)`

## Notes

- The MCP layer stays thin and reuses shared DeltaScope audit, rule-catalog, metadata-preparation, and direct-connection helper logic.
- The current scope supports stdio MCP bootstrap, offline audit for MySQL, TiDB, and PostgreSQL, plus metadata-aware audit for MySQL/TiDB-compatible instances and PostgreSQL on the PG-capable builds.
- Connection-backed PostgreSQL MCP audit requests follow the same shared metadata-preparation path as the other transports and should preserve explicit metadata-aware context rather than downgrading silently.
- `get_capabilities` is MCP-client-facing and summarizes transport, official tool names, dialect support, audit result fields, connection inputs, and stable structured error codes.
- Direct and named connection inputs accept `connect_timeout` (duration string like `5s`); empty/omitted/`0s` falls back to runtime config default, invalid/negative values return `connection_invalid`.
- `tool_errors.go` maps `connection connect_timeout` validation errors to `connection_invalid`.

## Dependencies

- Upstream: `cmd/deltascope-mcp`
- Downstream: shared audit/rule-catalog layers under `pkg/deltascope`, metadata helpers under `internal/interfaces/metadata`, and other `internal/...` adapter layers

## Update Rule

- If members/interfaces/dependencies change, update this file in same change.
