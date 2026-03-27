# DeltaScope MCP Command

MCP stdio service entrypoint for exposing DeltaScope audit and rule-discovery tools to agent clients.

## Files

| File | Responsibility |
|------|---------------|
| `main.go` | Parses process flags and starts the MCP stdio service |
| `main_test.go` | Verifies command bootstrap, version fast-path, and stdio smoke behavior |
| `main_e2e_test.go` | Verifies tagged Docker-backed metadata-aware MCP smoke against real MySQL/TiDB fixtures for direct and connection_ref flows |

## Notes

- This command is intentionally thin and delegates MCP wiring to `internal/interfaces/mcp`.
- The initial MCP milestone focuses on stdio transport only.
- The exposed MCP surface includes `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities`.
- `connection_ref` reads `~/.config/deltascope/connections.yaml` by default and can be overridden with `-connections-path`.
- After startup, stdout is reserved for MCP stdio protocol traffic.
- `-version` prints only the semantic version string and defaults to `v0.7.0` in source builds.
- The real metadata-aware MCP smokes stay behind `go test -tags=e2e ...`, `make test-e2e-mcp-mysql`, and `make test-e2e-mcp-tidb` so the default Go test loop remains fast.

## Update Rule

- If members/interfaces/dependencies change, update this file in same change.
