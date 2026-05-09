# DeltaScope MCP Command

Native stdio runtime for the official DeltaScope MCP server. For client onboarding, start with [Use DeltaScope MCP](../../docs/recipe/use-deltascope-mcp.md).

## Files

| File | Responsibility |
|------|---------------|
| `main.go` | Parses process flags and starts the MCP stdio service |
| `main_test.go` | Verifies command bootstrap, version fast-path, and stdio smoke behavior |
| `main_e2e_test.go` | Verifies tagged Docker-backed metadata-aware MCP smoke against real MySQL/TiDB fixtures for direct and connection_ref flows |

## Notes

- This command is intentionally thin and delegates MCP wiring to `internal/interfaces/mcp`.
- The exposed MCP surface includes `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities`.
- `connection_ref` reads `~/.config/deltascope/connections.yaml` by default and can be overridden with `-connections-path`.
- After startup, stdout is reserved for MCP stdio protocol traffic.
- `-version` prints only the semantic version string and defaults to `v0.13.1` in source builds.
- `-log-level` sets log verbosity: `debug`, `info` (default), `warn`, `error`.
- `-log-format` sets log format: `json` (default), `text`.
- `-log-output` sets log destination: `stderr` (default), `file`. `stdout` is forbidden for MCP to protect the stdio protocol.
- `-log-file` sets log file path (required when `-log-output=file`; plain append, no rotation).

## Update Rule

- If members/interfaces/dependencies change, update this file in same change.
