# Decision: PG17 `COUNT(1)` Online Surface Contract

- Date: 2026-08-03
- Status: Proposed
- Baseline: `main@dd861f8`
- Related: [PG17 `COUNT(1)` proof](2026-07-31-query-access-pg17-count-literal-proof.md), [online connection registry](2026-07-20-query-access-online-connection-registry.md)
- Spec: `docs/plans/2026-08-03-query-access-pg17-count-online-surface-contract-spec.md`
- Design: `docs/plans/2026-08-03-query-access-pg17-count-online-surface-contract-design.md`
- Implementation: `docs/plans/2026-08-03-query-access-pg17-count-online-surface-contract-implementation.md`

## Context

The accepted PG17 decision proves exact `COUNT(1)` only through a caller-owned
SDK session. The current online CLI and HTTP code paths appear to pass a pinned
PostgreSQL connection to that same session API, but that fact has not yet been
validated as a public transport contract for this literal shape. In particular,
CLI uses direct local credentials while HTTP selects an operator-managed,
authorized connection. Their lifecycle, error, and logging boundaries need
live evidence before the project promises equivalent behavior.

## Proposed Decision

Treat CLI and HTTP as candidate online surfaces for the already-accepted exact
PG17 `COUNT(1)` proof. Promote neither until task-owned PostgreSQL 17 E2E,
no-execution, no-leak, default-path, and independent-review evidence confirms
that both delegate to the same catalog-bound session proof without creating a
new trust path.

If accepted, the supported SQL envelope on each listed online surface is only:

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
the trust boundary. Reusing that proof avoids a second parser or catalog path,
but reuse must be demonstrated under each transport's ownership and privacy
model. Source inspection cannot establish live version identity, HTTP
authorization-before-dial behavior, connection cleanup, or absence of output
and log leakage.

Keeping default CLI and HTTP analysis offline preserves the fail-closed policy:
function-bearing `COUNT(1)` remains indeterminate until an operator or local
user intentionally establishes an online PG17 session.

## Public Contract If Accepted

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

## Acceptance Evidence

This record remains Proposed until live PG17 SDK/CLI/HTTP evidence proves the
shared session chain, positive requirements, deferred-shape indeterminacy,
offline/default boundaries, authorization-before-dial, non-vacuous
no-execution, no-leak outputs/logs, and cleanup. The final review must find no
P0/P1/P2 issues. If any condition fails, the current SDK-only decision remains
the published boundary.
