# HTTP Interface Module

HTTP exposes DeltaScope audit and metadata-aware review capabilities as a JSON service.

## Files

| File | Responsibility |
|------|---------------|
| audit_metadata.go | Executes one HTTP audit request through offline or direct metadata-aware flows and adds adapter context to the JSON response |
| audit_metadata_test.go | Verifies HTTP metadata-aware execution wiring, additive context, and direct metadata client lifecycle handling |
| handler.go | Binds Gin HTTP requests to the public audit/rule/capability/query-access APIs, auth checks, JSON responses, structured access logging, and health/readiness endpoints |
| handler_test.go | Verifies HTTP request binding, error mapping, and JSON response shape |
| query_access.go | Handles HTTP query access analysis requests, preserves registry/authorization/connection/error/log ownership, and routes online analysis through the opaque unified SDK session |
| query_access_test.go | Verifies HTTP query access request binding, response shape, defaults, unified online entry wiring with an empty analysis dialect, bounded constructor/capability failures, zero-open authorization paths, and close ownership |
| query_access_unified_entry_test.go | Structurally verifies `handleQueryAccessOnline` contains no product inspection or dialect-specific Query Access constructor/analysis calls and uses both unified SDK entry symbols |
| query_access_postgresql_online_recording_test.go | Verifies the PostgreSQL online HTTP connection_id path uses shared session probes without executing submitted SQL |
| query_access_probe_boundary_no_leak_test.go | No-leak regression for the MySQL/TiDB builtin-identity probe boundary on the HTTP surface: asserts injected markers, identity facts, candidates, session/context, manifest, raw SQL, and `severity` are absent from the response body (including the error boundary) |
| query_access_postgresql_no_leak_test.go | PostgreSQL 17 integration no-leak coverage for online `COUNT(1)`, excluded shapes, missing `connection_id`, and unauthorized HTTP paths |
| rule_catalog.go | Builds HTTP rule-list, rule-detail, and capability payloads from the shipped catalog metadata |
| server.go | Assembles the HTTP handler and long-running server wiring |

## Exports

- `NewHandler(configPath, version, opts ...HandlerOption) (http.Handler, error)`
- `WithAuthConfig(AuthConfig) HandlerOption`
- `WithMiddlewareConfig(MiddlewareConfig) HandlerOption`
- `WithAuditFunc(func(context.Context, deltascope.Request) (deltascope.Result, error)) HandlerOption`
- `WithMetadataConfig(MetadataConfig) HandlerOption`
- `NewServer(addr, configPath, version, opts ...HandlerOption)`

## Notes

- Online Query Access keeps registry lookup, authorization, TLS/credential resolution, cancellation, connection close, HTTP errors, request IDs, and access logs in HTTP, then passes the caller-owned pinned connection to the opaque unified SDK session without inspecting observed product or constraining the analysis request dialect.
- Query Access semantic breadth and detailed probe tests live in the unified SDK suite; this module retains only HTTP-owned transport, registry, authorization, sink, lifecycle, and real-route evidence.
- The HTTP layer is adapter-only: it reuses the shared public audit API and metadata-preparation helpers instead of reimplementing dialect or schema logic.
- Routing uses Gin while keeping the public JSON API contract unchanged.
- API-key auth is optional and configured through adapter options.
- Rate limiting is optional and supports `api-key` or `ip` bucketing.
- `/metrics` is exposed in Prometheus format by default and can be disabled via middleware config.
- Default middleware chain is request-id -> recovery -> timeout -> metrics -> auth -> rate-limit -> access log.
- Config hot-reload is achieved by re-reading the configured policy path on each audit request, so file updates take effect without restarting the server.
- Current scope supports offline and metadata-aware audit, HTTP-native rule discovery, capability discovery, and query access analysis.
- Responses preserve the public DeltaScope result body and add a `context` block describing mode, dialect/schema provenance, and metadata source.
- Direct connection input accepts `connect_timeout` (duration string like `5s`); empty/omitted/`0s` falls back to runtime config default, invalid/negative values return 400.

## Dependencies
- Upstream: `cmd/deltascope-server`
- Downstream: `pkg/deltascope`, `internal/application/policy`, `internal/domain/rule/catalog`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
