# MCP Interface Module

Thin MCP adapter for exposing DeltaScope audit and rule-discovery capabilities to agent clients.

## Files

| File | Responsibility |
|------|---------------|
| `audit_tool.go` | Implements the MCP `audit_sql` tool on top of the shared DeltaScope audit path |
| `connection.go` | Validates and resolves direct and referenced metadata-aware connection inputs |
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

- The MCP layer stays thin and reuses shared DeltaScope audit, rule-catalog, and metadata-preparation logic.
- The current scope supports stdio MCP bootstrap, offline audit, metadata-aware audit context, and rule-discovery tools.
- `get_capabilities` is MCP-client-facing and summarizes transport, official tool names, audit result fields, connection inputs, and stable structured error codes.

## Dependencies

- Upstream: `cmd/deltascope-mcp`
- Downstream: shared audit and rule-catalog layers under `pkg/deltascope` and `internal/...`

## Update Rule

- If members/interfaces/dependencies change, update this file in same change.
