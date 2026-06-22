# DeltaScope MCP Command

Native stdio runtime for the official DeltaScope MCP server. For client onboarding, start with [Use DeltaScope MCP](../../docs/recipe/use-deltascope-mcp.md).

## Files

| File | Responsibility |
|------|---------------|
| `main.go` | Parses process flags, loads runtime config, merges logging settings, and starts the MCP stdio service |
| `main_test.go` | Verifies command bootstrap, version fast-path, logging config, runtime config merge, and stdio smoke behavior |
| `main_e2e_test.go` | Verifies tagged Docker-backed metadata-aware MCP smoke against real MySQL/TiDB fixtures for direct and connection_ref flows |

## Notes

- This command is intentionally thin and delegates MCP wiring to `internal/interfaces/mcp`.
- The exposed MCP surface includes `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities`.
- `connection_ref` reads `~/.config/deltascope/connections.yaml` by default and can be overridden with `-connections-path`.
- After startup, stdout is reserved for MCP stdio protocol traffic.
- `-version` prints only the semantic version string and defaults to the release/source version from `pkg/deltascope.DefaultVersion` in source builds.
- `-log-level` sets log verbosity: `debug`, `info` (default), `warn`, `error`.
- `-log-format` sets log format: `json` (default), `text`.
- `-log-output` sets log destination: `stderr` (default), `file`. `stdout` is forbidden for MCP to protect the stdio protocol.
- `-log-file` sets log file path (required when `-log-output=file`; plain append by default).
- `-log-rotate` enables log file rotation via lumberjack (requires `-log-output=file`). Default: false (plain append).
- `-log-max-size-mb` max log file size in MB before rotation. Default: 100.
- `-log-max-backups` max number of rotated log files to retain. Default: 3.
- `-log-max-age-days` max number of days to retain rotated log files. Default: 30.
- `-log-compress` compress rotated log files. Default: true.
- `-runtime-config <path>` loads a runtime YAML config for logging and other service settings. Explicit flags override runtime config values; runtime config overrides hardcoded defaults. `stdout` from runtime config is still forbidden and must be overridden by an explicit `--log-output` flag.
- `metadata.connect_timeout` in runtime config sets the default metadata connect timeout for MCP metadata-aware audit. Omitted or empty means no default (uses the opener's internal default). Invalid or negative values cause startup to fail with exit code 2.

## Update Rule

- If members/interfaces/dependencies change, update this file in same change.
