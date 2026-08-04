# Decision: PG17 `COUNT(1)` Online Surface Contract

- Date: 2026-08-03
- Status: Accepted
- Baseline: `main@dd861f8`
- Related: [PG17 `COUNT(1)` proof](2026-07-31-query-access-pg17-count-literal-proof.md), [online connection registry](2026-07-20-query-access-online-connection-registry.md)
- Spec: `docs/plans/2026-08-03-query-access-pg17-count-online-surface-contract-spec.md`
- Design: `docs/plans/2026-08-03-query-access-pg17-count-online-surface-contract-design.md`
- Implementation: `docs/plans/2026-08-03-query-access-pg17-count-online-surface-contract-implementation.md`

## Context

The accepted PG17 decision proves exact `COUNT(1)` through a caller-owned SDK
session. The online CLI and HTTP paths now have transport-level evidence that
they pass a pinned PostgreSQL connection to that same session API. The CLI uses
its existing direct local connection options; HTTP selects an operator-managed,
authorized `connection_id`. Their lifecycle, error, and logging boundaries are
bounded by the evidence recorded below.

## Decision

Accept online CLI and HTTP as surfaces for the already-accepted exact PG17
`COUNT(1)` proof. Both delegate to the same catalog-bound session proof without
creating a new trust path.

The supported SQL envelope on each listed online surface is only:

```sql
SELECT COUNT(1) FROM app.orders
```

where `app.orders` is one schema-qualified resolved physical base table. The
result is `read_only` plus `admissible` with exactly `app.orders / read_table`.
CLI requires its existing online PostgreSQL connection options. HTTP requires
an operator-configured, authorized PostgreSQL 17 `connection_id` for the
`query_access` purpose.

## Rationale

The session-bound catalog proof, not a transport-specific function switch, is
the trust boundary. Reusing that proof avoids a second parser or catalog path.
The recording-driver and live transport evidence demonstrates version identity,
authorization-before-dial behavior, connection cleanup, and bounded output and
log privacy under each transport's ownership model.

Keeping default CLI and HTTP analysis offline preserves the fail-closed policy:
function-bearing `COUNT(1)` remains indeterminate until an operator or local
user intentionally establishes an online PG17 session.

## Public Contract

- SDK, online CLI, and HTTP with authorized `connection_id` share the same
  exact PG17 `COUNT(1)` proof and table-only requirement result.
- HTTP clients never provide endpoints, credentials, secret sources, TLS
  settings, profile, or server-version claims; `connection_id` selection stays
  operator controlled and authorized.
- Submitted SQL is never executed, prepared, explained, or returned. Public
  outputs and logs do not expose SQL/literal markers, connection data,
  credentials, catalog data, or raw driver errors.
- Offline CLI, HTTP without `connection_id`, default SDK, MCP, all other
  PostgreSQL literal shapes, and every existing deferred shape remain outside
  the contract.

## Alternatives

### Declare transport support from existing delegation

Rejected. Existing code routing does not prove live behavior, build-tag
coverage, authorization, bounded errors, or no-leak properties.

### Add a transport-specific literal feature flag

Rejected. It would duplicate or bypass the session-bound catalog proof and
make a caller-provided setting appear to be a trust root.

### Broaden PostgreSQL literal support concurrently

Rejected. The only question here is safe transport parity for an already
bounded SQL envelope. Parser and catalog scope remain unchanged.

## Deferred / Out Of Scope

- Any expansion beyond exact `COUNT(1)` over one schema-qualified physical
  table.
- Relationless queries, non-`1` literals, casts, parameters, modifiers, joins,
  CTEs, views, derived tables, unqualified sources, scalar or binary literal
  functions, and arbitrary aggregates.
- New CLI flags, HTTP request/result fields, direct HTTP credentials, MCP Query
  Access, SQL execution, authorization/grant/RLS/masking evaluation, or query
  data retrieval.

## Consequences

- An operator or local user must intentionally establish the supported online
  session; default and offline paths remain indeterminate for this query.
- Future PostgreSQL literal shapes or transport surfaces require separate
  evidence and a separate decision; this record does not widen the SDK proof.
- The contract remains static requirement analysis. It does not execute SQL,
  retrieve query results, or decide authorization, grants, RLS, or masking.

## Verification Evidence

- The shared session recording-driver proof passes in
  `pkg/deltascope/query_access_session_postgresql_recording_test.go`. It
  observes fixed identity/catalog probes and proves that the submitted SQL
  marker does not reach the driver or public result.
- The CLI adapter recording-driver proof passes in
  `internal/interfaces/cli/query_access_postgresql_online_recording_test.go`.
  It observes fixed probes, closes the CLI-owned session, and proves that
  submitted SQL, `EXPLAIN`, and prepare operations do not reach the driver.
- The HTTP adapter recording-driver proof passes in
  `internal/interfaces/http/query_access_postgresql_online_recording_test.go`.
  It observes fixed probes and proves that submitted SQL, `EXPLAIN`, and
  prepare operations do not reach the driver.
- The no-leak proofs pass for normal, connection-failure, and catalog-failure
  paths in the CLI and HTTP adapter tests. HTTP unauthorized and unknown
  `connection_id` cases assert zero opener operations. HTTP access-log capture
  uses a synchronized sink and asserts a request entry exists before checking
  it; the real HTTP E2E also asserts the request entry and request ID.
- The real CLI binary E2E passes in
  `cmd/deltascope/main_e2e_postgresql_query_access_test.go`, including the
  online positive case, excluded shapes, default/offline regression, bounded
  no-leak output, and the deterministic RST failure listener. The real HTTP
  server E2E passes in
  `cmd/deltascope-server/main_e2e_postgresql_query_access_test.go`, including
  the authorized `connection_id`, excluded shapes, no-connection default,
  authorization boundary, and bounded response/log output.
- The direct tagged evidence commands passed against the PG17 fixture:
  `CGO_ENABLED=1 go test -tags='postgresql' -count=1 ./pkg/deltascope -run
  TestTrustedSDK_CountIntegerOne`, the corresponding CLI and HTTP
  `postgresql,integration` adapter tests, and the CLI and HTTP
  `e2e,postgresql` binary tests. `make decision-record-gate`,
  `make release-gofmt-gate`, and `git diff --check main...HEAD` also pass.

These proofs establish only PG17 online CLI and authorized HTTP
`connection_id` support for the exact statement envelope above. They do not
claim SQL execution, an authorization decision, or broader PostgreSQL literal
support. Default/offline, MCP, and every other deferred shape remain outside
the contract.
