# DeltaScope Server Command

HTTP service entrypoint exposes DeltaScope audit and metadata-aware review over JSON APIs.

## Files

| File | Responsibility |
|------|---------------|
| main.go | Parses process flags and starts the HTTP service |
| main_e2e_test.go | Runs Docker-backed metadata-aware HTTP e2e coverage against the real server binary |

## Notes

- This command is intentionally thin and delegates HTTP wiring to `internal/interfaces/http`.
- `POST /v1/audit` accepts offline requests and direct metadata-aware connection inputs.
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

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
