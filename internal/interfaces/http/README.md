# HTTP Interface Module

HTTP exposes DeltaScope audit and metadata-aware review capabilities as a JSON service.

## Files

| File | Responsibility |
|------|---------------|
| audit_metadata.go | Executes one HTTP audit request through offline or registry metadata-aware flows and preserves adapter context plus partial public results when an audit returns diagnostics and an error; `target_version` is forwarded to the shared request, `spec.ValidateTargetVersion` preflight runs at the start of `executeAuditRequest` before any registry lookup or metadata preparation, and typed version input errors map to HTTP 400 |
| audit_metadata_test.go | Verifies HTTP metadata-aware execution wiring, additive context, and direct metadata client lifecycle handling |
| audit_impact_postgresql_tag_test.go | Verifies PostgreSQL offline primary-key equality impact in HTTP JSON output |
| audit_dml_table_existence_test.go | Verifies registry-backed MySQL/TiDB INSERT/UPDATE/DELETE missing-target findings and stable HTTP result shape |
| audit_offline_existence_test.go | Locks offline ALTER DROP COLUMN HTTP JSON review verdict, `unknown_table_state` evidence gaps, `context.note` / `context.unproven`, capabilities `context_fields`, and the four-rule ordered first-path projection (first-statement gap, derived-complete followers) |
| handler.go | Binds Gin HTTP requests to public APIs, emits diagnostic error envelopes that retain the full partial audit result beside the bounded transport error, and maps MCP `connection_ref` to field-level `invalid_request` instead of opaque `invalid_json` |
| handler_unsupported_diagnostics_evidence_test.go | Verifies HTTP parser diagnostics preserve the review-floored partial result, valid statements/findings, locations, context, error status, and no-leak boundaries |
| handler_unsupported_verdict_floor_postgresql_tag_test.go | Verifies HTTP PostgreSQL `SELECT 1` keeps non-success status and serializes the review-floored unsupported result with the retained incomplete-coverage statement |
| audit_coverage_t03_test.go | Verifies issue #82/T03 transport contract: MySQL `CREATE SEQUENCE` + supported `ALTER TABLE` returns HTTP 400 with the partial result, retained incomplete coverage, and bounded unsupported evidence |
| audit_evidence_gap_t04_test.go | Verifies issue #83 T04-A transport contract: an evidence-gap-only audit returns HTTP 200 with `unverified` coverage and per-statement `evidence_gaps`, while a mixed gap+unsupported batch keeps the HTTP 400 partial-result contract |
| audit_ordered_drop_a3_test.go | Verifies the T05-A3 HTTP contract under the offline five-rule policy: the drop-recreate first path returns 200/review with only the leading CREATE's `unknown_table_state` gap while the DROP and rebuilt followers stay complete, and isolated DROP/`DROP TABLE IF EXISTS` requests return 200 with exactly one `ddl.table.drop.exists.require` `unknown_table_state` gap requiring `target_table.existence` and zero findings |
| audit_ordered_modify_a4_test.go | Verifies the T05-A4 HTTP contract under the offline four-rule policy: `VARCHAR(10) → 20 → 15` returns 200/`reject`/`unverified` with the CREATE gap and one `source_length=20`/`target_length=15` blocker; `10 → 20 → 30` returns 200/`review`/`unverified` with derived-complete followers; an isolated unknown MODIFY returns 200 with existence and `missing_source_column` gaps and zero findings; empty SQL returns 400 `bad_request` |
| audit_ordered_drop_column_a6_test.go | Verifies the T05-A6 HTTP contract under the offline six-rule policy with schema golden: the first path returns 200/`review`/`unverified` with only the CREATE gap, and `ix_removed(obsolete)` returns 200/`reject` with one index-column blocker whose schema is golden |
| audit_resource_limit_a7_test.go | Verifies the T05-A7 HTTP contract: a 1025-statement MySQL `/v1/audit` request returns HTTP 400 with the existing error envelope and `unsupported_statement` diagnostic, the retained `review`/`incomplete` result keeping the first 1024 statements complete and one `audit.resource_limit` entry with ordered_state metadata |
| audit_procedure_state_a8_test.go | Verifies the T05-A8 HTTP contract under the isolated four-rule policy at schema golden: a TiDB `CREATE PROCEDURE` `/v1/audit` request returns HTTP 400 `bad_request` with `review`/`incomplete` and one vendor boundary while a MySQL `DROP PROCEDURE` request returns HTTP 200 `review`/`unverified`, both keeping the four ddl identities, the exact first `unknown_table_state` gap, and complete followers |
| audit_create_table_pk_t06a1_test.go | Verifies the T06-A1 HTTP contract under the isolated one-rule policy: a no-PK CREATE TABLE `/v1/audit` request returns HTTP 200 `reject`/`complete` with one `ddl.table.primary_key.require` blocker for both dialects, while a table-level-PK request returns HTTP 200 `pass`/`complete` with zero findings and gaps |
| audit_create_table_pk_nullability_t06a2_test.go | Verifies the T06-A2 HTTP contract: a legal table-level PK `/v1/audit` request returns HTTP 200 `pass`/`complete`, and an explicit `NULL` member returns HTTP 200 `reject`/`complete` with one `ddl.table.primary_key.not_null.require` blocker |
| audit_default_null_state_t06a3_test.go | Verifies the T06-A3 HTTP contract: the three-statement `DEFAULT NULL` drop-state `/v1/audit` request returns HTTP 200 `pass`/`complete`, and a missing `DEFAULT` returns HTTP 200 `reject` with the `ddl.column.default.require` blocker |
| audit_provider_defaults_t06a4_test.go | Verifies the T06-A4 HTTP contract with the real `mysqlmeta.Provider` behind a controlled driver: a stored `'NULL'` literal sibling returns HTTP 200 `review`, a stored SQL NULL sibling returns HTTP 200 `pass`, and the provider is read exactly once per request |
| audit_char_length_t06a5_test.go | Verifies the T06-A5 HTTP contract under the isolated `ddl.column.varchar.max_length` policy (limit=8, blocker): a VARCHAR(8) `/v1/audit` request returns HTTP 200 `pass`/`complete`, and VARCHAR(9) returns HTTP 200 `reject`/`complete` with the one pinned blocker — a policy verdict is ordinary result data, never an HTTP error |
| audit_table_collation_t06a7_test.go | Verifies the T06-A7 HTTP contract: a declared table COLLATE is an ordinary HTTP 200 audit payload with no unsupported-statement diagnostics, and a policy rejection stays a populated response |
| audit_comment_t06a8_test.go | Verifies the T06-A8 HTTP contract: an empty column COMMENT and a rune-counted table comment arrive as ordinary HTTP 200 audit payloads, never transport errors |
| audit_columns_t06a9_test.go | Verifies the T06-A9 HTTP contract under the isolated `ddl.table.audit_columns.require` policy: the complete pair returns HTTP 200 `pass`/`complete`, and missing both roles returns HTTP 200 `reject` with the exact two-blocker pair — a policy reject is ordinary audit data, not an HTTP error |
| audit_auto_increment_t06a10_test.go | Verifies the T06-A10 HTTP contract under the isolated AUTO_INCREMENT policies: the pk-auto declaration returns HTTP 200 `pass`, and the `AUTO_INCREMENT=9` mismatch returns HTTP 200 `reject` with the exact init-value blocker identity — a policy reject is ordinary audit data, not an HTTP error |
| audit_version_t04b_test.go | Verifies issue #83 T04-B transport contract: `target_version` projects the canonical `version` block, malformed or mismatched input maps to HTTP 400 with no audit result, and missing/out-of-range versions produce bounded `evidence_gaps` |
| handler_ddl_lifecycle_mysql_test.go | Verifies HTTP lifecycle findings for MySQL/TiDB DDL, including 400 envelopes for supported statements that still carry extracted-but-unaudited option aspects (sequence/placement-policy option lists, per-spec account `identified` auth) |
| handler_test.go | Verifies HTTP request binding, error mapping, JSON response shape without CLI-only `fail_on_triggered`, metadata-aware omission of offline existence caveats, field-level rejection of MCP `connection_ref`, and per-target denylist findings for multi-target DROP/RENAME via `config_path` |
| rule_catalog.go | Builds HTTP rule-list, rule-detail, and capability payloads from the shipped catalog metadata, including `note` / `unproven` on `context_fields` and stable online identity/authentication error codes |
| query_access.go | Handles HTTP query access analysis requests, canonicalizes named MySQL/TiDB database/schema aliases with the missing default qualifier, keeps request-only defaults out of catalog selection, rejects conflicting hints before open, preserves registry/authorization/connection/error/log ownership, maps bounded PostgreSQL PG17 identity and database-authentication boundaries, and routes online analysis through `attachOnlineQueryAccessSession` → `metadata.AttachOnlineQueryAccessSession` (reuses OpenSession identity; no second probe) |
| query_access_test.go | Verifies request binding, MySQL/TiDB database/schema/default-schema aliases and conflicts, PostgreSQL database/schema preservation and PG17 boundary, response shape, unified online routing, bounded failures, zero-open authorization paths, and close ownership |
| query_access_issue35_postgresql_tag_test.go | Verifies CLI and HTTP share the normalized PostgreSQL `read_only`/`admissible` state and reason codes |
| query_access_unified_entry_test.go | Structurally verifies `handleQueryAccessOnline` contains no product inspection or dialect-specific Query Access constructor/analysis calls, uses both unified SDK entry symbols, and reuses Observed Server Identity |
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
