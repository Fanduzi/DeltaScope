# DeltaScope Server Command

HTTP service entrypoint exposes DeltaScope audit and metadata-aware review over JSON APIs.

## Files

| File | Responsibility |
|------|---------------|
| main.go | Parses process flags, loads runtime config, merges logging settings, and starts the HTTP service |
| main_test.go | Verifies CLI flag parsing, logging config from flags, and runtime config merge helpers |
| main_e2e_test.go | Runs Docker-backed metadata-aware HTTP e2e coverage against the real server binary |
| main_e2e_postgresql_test.go | Runs Docker-backed PostgreSQL metadata-aware HTTP audit e2e via registry-backed authorized `connection_id` |
| main_e2e_postgresql_query_access_test.go | Verifies Docker-backed PG17 COUNT(1) query-access behavior through the HTTP connection_id surface, including foreign-table fail-closed |

## Notes

- This command is intentionally thin and delegates HTTP wiring to `internal/interfaces/http`.
- `POST /v1/audit` accepts offline requests and metadata-aware requests that select a registry-backed, authorized `connection_id`. Clients never supply direct database endpoints, credentials, secret sources, or TLS settings on the request.
- `GET /v1/rules`, `GET /v1/rules/{rule_id}`, and `GET /v1/capabilities` expose rule discovery and HTTP contract metadata.
- `-auth-enabled`, `-auth-keys`, and `-auth-allow-paths` configure optional `X-API-Key` protection.
- `-rate-limit-enabled`, `-rate-limit-rps`, `-rate-limit-burst`, and `-rate-limit-key` configure optional request throttling.
- `-trusted-proxies` controls which proxy CIDRs are trusted for client IP extraction (empty means trust none).
- `-metrics-enabled` controls whether `/metrics` is exposed in Prometheus text format.
- `-log-level` sets log verbosity: `debug`, `info` (default), `warn`, `error`.
- `-log-format` sets log format: `json` (default), `text`.
- `-log-output` sets log destination: `stderr` (default), `stdout`, `file`.
- `-log-file` sets log file path (required when `-log-output=file`; plain append by default).
- `-log-rotate` enables log file rotation via lumberjack (requires `-log-output=file`). Default: false (plain append).
- `-log-max-size-mb` max log file size in MB before rotation. Default: 100.
- `-log-max-backups` max number of rotated log files to retain. Default: 3.
- `-log-max-age-days` max number of days to retain rotated log files. Default: 30.
- `-log-compress` compress rotated log files. Default: true.
- `-version` prints only the semantic version string and defaults to the repository `DefaultVersion` in source builds.
- `-runtime-config <path>` loads a runtime YAML config for logging and other service settings. Explicit flags override runtime config values; runtime config overrides hardcoded defaults.
- `metadata.connect_timeout` in runtime config sets the default metadata connect timeout for HTTP metadata-aware audit. Omitted or empty means no default (uses the opener's internal default). Invalid or negative values cause startup to fail with exit code 2.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
