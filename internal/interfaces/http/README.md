# HTTP Interface Module

HTTP exposes DeltaScope audit and metadata-aware review capabilities as a JSON service.

## Files

| File | Responsibility |
|------|---------------|
| audit_metadata.go | Executes one HTTP audit request through offline or direct metadata-aware flows and adds adapter context to the JSON response |
| audit_metadata_test.go | Verifies HTTP metadata-aware execution wiring, additive context, and direct metadata client lifecycle handling |
| handler.go | Binds Gin HTTP requests to the public audit/rule/capability APIs, auth checks, JSON responses, structured access logging, and health/readiness endpoints |
| handler_test.go | Verifies HTTP request binding, error mapping, and JSON response shape |
| rule_catalog.go | Builds HTTP rule-list, rule-detail, and capability payloads from the shipped catalog metadata |
| server.go | Assembles the HTTP handler and long-running server wiring |

## Exports

- `NewHandler(configPath, version, opts ...HandlerOption) (http.Handler, error)`
- `WithAuthConfig(AuthConfig) HandlerOption`
- `WithMiddlewareConfig(MiddlewareConfig) HandlerOption`
- `WithAuditFunc(func(context.Context, deltascope.Request) (deltascope.Result, error)) HandlerOption`
- `NewServer(addr, configPath, version, opts ...HandlerOption)`

## Notes

- The HTTP layer is adapter-only: it reuses the shared public audit API and metadata-preparation helpers instead of reimplementing dialect or schema logic.
- Routing uses Gin while keeping the public JSON API contract unchanged.
- API-key auth is optional and configured through adapter options.
- Rate limiting is optional and supports `api-key` or `ip` bucketing.
- `/metrics` is exposed in Prometheus format by default and can be disabled via middleware config.
- Default middleware chain is request-id -> recovery -> timeout -> metrics -> auth -> rate-limit -> access log.
- Config hot-reload is achieved by re-reading the configured policy path on each audit request, so file updates take effect without restarting the server.
- Current scope supports offline and metadata-aware audit plus HTTP-native rule discovery and capability discovery.
- Responses preserve the public DeltaScope result body and add a `context` block describing mode, dialect/schema provenance, and metadata source.
- Direct connection input accepts `connect_timeout` (duration string like `5s`); empty/omitted/`0s` uses the default, invalid/negative values return 400.

## Dependencies
- Upstream: `cmd/deltascope-server`
- Downstream: `pkg/deltascope`, `internal/application/policy`, `internal/domain/rule/catalog`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
