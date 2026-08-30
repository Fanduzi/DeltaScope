# HTTP Interface Module

HTTP exposes DeltaScope audit and metadata-aware review capabilities as a JSON service.

## Files

| File | Responsibility |
|------|---------------|
| audit_metadata.go | Executes one HTTP audit request through offline or registry metadata-aware flows and preserves adapter context plus partial public results when an audit returns diagnostics and an error |
| audit_metadata_test.go | Verifies HTTP metadata-aware execution wiring, additive context, and direct metadata client lifecycle handling |
| audit_impact_postgresql_tag_test.go | Verifies PostgreSQL offline primary-key equality impact in HTTP JSON output |
| audit_dml_table_existence_test.go | Verifies registry-backed MySQL/TiDB INSERT/UPDATE/DELETE missing-target findings and stable HTTP result shape |
| audit_offline_existence_test.go | Locks offline ALTER DROP COLUMN HTTP JSON `context.note` / `context.unproven` and capabilities `context_fields` |
| handler.go | Binds Gin HTTP requests to public APIs and emits diagnostic error envelopes that retain the full partial audit result beside the bounded transport error |
| handler_unsupported_diagnostics_evidence_test.go | Verifies HTTP parser diagnostics preserve the review-floored partial result, valid statements/findings, locations, context, error status, and no-leak boundaries |
| handler_test.go | Verifies HTTP request binding, error mapping, JSON response shape, and metadata-aware omission of offline existence caveats |
| rule_catalog.go | Builds HTTP rule-list, rule-detail, and capability payloads from the shipped catalog metadata, including `note` / `unproven` on `context_fields` and stable online identity/authentication error codes |
| query_access.go | Handles HTTP query access analysis requests, canonicalizes named MySQL/TiDB database/schema aliases with the missing default qualifier, keeps request-only defaults out of catalog selection, rejects conflicting hints before open, preserves registry/authorization/connection/error/log ownership, maps bounded PostgreSQL PG17 identity and database-authentication boundaries, and routes online analysis through the opaque unified SDK session |
| query_access_test.go | Verifies request binding, MySQL/TiDB database/schema/default-schema aliases and conflicts, PostgreSQL database/schema preservation and PG17 boundary, response shape, unified online routing, bounded failures, zero-open authorization paths, and close ownership |
| query_access_issue35_postgresql_tag_test.go | Verifies CLI and HTTP share the normalized PostgreSQL `read_only`/`admissible` state and reason codes |
| query_access_unified_entry_test.go | Structurally verifies `handleQueryAccessOnline` contains no product inspection or dialect-specific Query Access constructor/analysis calls and uses both unified SDK entry symbols |
| query_access_postgresql_online_recording_test.go | Focused recording-driver proof that the PostgreSQL online HTTP connection_id path delegates through a pinned session, closes once, maps bounded catalog failures, and never executes submitted SQL, EXPLAIN, or prepare operations |
| query_access_e2e_mixed_literal_test.go | Docker-backed HTTP smoke for admitted and fail-closed MySQL 8.4 and TiDB 8.5 routes, including unqualified seeded-table schema-only resolution through named connections, with response and access-log scans that keep registered DSN credential markers out of admitted paths, plus HTTP default/offline and bounded credential-failure no-leak coverage |
| query_access_probe_boundary_no_leak_test.go | No-leak regression for the MySQL/TiDB builtin-identity probe boundary on the HTTP surface: asserts injected markers, identity facts, candidates, session/context, manifest, raw SQL, and `severity` are absent from the response body (including the error boundary) |
| query_access_postgresql_no_leak_test.go | PostgreSQL 17 integration no-leak coverage for online `COUNT(1)`, excluded shapes, missing `connection_id`, and unauthorized HTTP paths |
| server.go | Assembles the HTTP handler and long-running server wiring |

## Exports

- `NewHandler(configPath, version, opts ...HandlerOption) (http.Handler, error)`
- `WithAuthConfig(AuthConfig) HandlerOption`
- `WithMiddlewareConfig(MiddlewareConfig) HandlerOption`
- `WithAuditFunc(func(context.Context, deltascope.Request) (deltascope.Result, error)) HandlerOption`
- `WithMetadataConfig(MetadataConfig) HandlerOption`
- `NewServer(addr, configPath, version, opts ...HandlerOption)`

## Notes

- Online Query Access keeps registry lookup, authorization, TLS/credential resolution, cancellation, connection close, HTTP errors, request IDs, and access logs in HTTP, canonicalizes named MySQL/TiDB database/schema aliases with the default qualifier and bounded conflict rejection, then passes the caller-owned pinned connection to the opaque unified SDK session without inspecting observed product or constraining the analysis request dialect. A reachable PostgreSQL identity outside PG17 returns `502 identity_error` with the fixed bounded requirement message; database authentication failure returns `502 authentication_failed`; both are advertised by `/v1/capabilities`.
- Query Access semantic breadth and detailed probe tests live in the unified SDK suite; this module retains only HTTP-owned transport, registry, authorization, sink, lifecycle, and real-route evidence.
- The HTTP layer is adapter-only: it reuses the shared public audit API and metadata-preparation helpers instead of reimplementing dialect or schema logic.
- Routing uses Gin while keeping the public JSON API contract unchanged.
- API-key auth is optional and configured through adapter options.
- Rate limiting is optional and supports `api-key` or `ip` bucketing.
- `/metrics` is exposed in Prometheus format by default and can be disabled via middleware config.
- Default middleware chain is request-id -> recovery -> timeout -> metrics -> auth -> rate-limit -> access log.
- Config hot-reload is achieved by re-reading the configured policy path on each audit request, so file updates take effect without restarting the server.
- Current scope supports offline and metadata-aware audit, HTTP-native rule discovery, capability discovery, and query access analysis.
- Responses preserve the public DeltaScope result body and add a `context` block describing mode, dialect/schema provenance, and metadata source. Parser-error responses use the same top-level result/context shape plus an `error` object, retaining valid statement findings and the shared partial-result review floor while still returning a non-success status.
- Direct connection input accepts `connect_timeout` (duration string like `5s`); empty/omitted/`0s` falls back to runtime config default, invalid/negative values return 400.

## Dependencies
- Upstream: `cmd/deltascope-server`
- Downstream: `pkg/deltascope`, `internal/application/policy`, `internal/application/queryaccess`, `internal/domain/rule/catalog`, `internal/interfaces/metadata`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
