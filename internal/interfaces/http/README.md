# HTTP Interface Module

Thin HTTP adapter for exposing the offline DeltaScope audit engine as a JSON service.

## Files

| File | Responsibility |
|------|---------------|
| handler.go | Binds HTTP requests to the public audit API and renders JSON responses |
| handler_test.go | Verifies HTTP request binding, error mapping, and JSON response shape |
| server.go | Assembles the HTTP mux and long-running server wiring |

## Exports

- `NewHandler(configPath, version)`
- `NewServer(addr, configPath, version)`

## Notes

- The HTTP layer is adapter-only: it reuses the same offline audit engine as the CLI and library.
- Config hot-reload is achieved by re-reading the configured policy path on each audit request, so file updates take effect without restarting the server.
- The service intentionally stays JSON-only in this milestone.

## Dependencies
- Upstream: `cmd/deltascope-server`
- Downstream: `pkg/deltascope`, `internal/application/policy`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
