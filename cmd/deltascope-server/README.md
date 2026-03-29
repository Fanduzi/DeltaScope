# DeltaScope Server Command

HTTP service entrypoint for exposing the offline DeltaScope audit engine over JSON APIs.

## Files

| File | Responsibility |
|------|---------------|
| main.go | Parses process flags and starts the HTTP service |

## Notes

- This command is intentionally thin and delegates HTTP wiring to `internal/interfaces/http`.
- `-auth-enabled`, `-auth-keys`, and `-auth-allow-paths` configure optional `X-API-Key` protection.
- `-rate-limit-enabled`, `-rate-limit-rps`, `-rate-limit-burst`, and `-rate-limit-key` configure optional request throttling.
- `-trusted-proxies` controls which proxy CIDRs are trusted for client IP extraction (empty means trust none).
- `-metrics-enabled` controls whether `/metrics` is exposed in Prometheus text format.
- `-version` prints only the semantic version string and defaults to the repository `DefaultVersion` in source builds.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
