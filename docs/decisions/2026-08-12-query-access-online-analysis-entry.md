# Decision: Route Online Query Access Through One Public Session Entry

- Date: 2026-08-12
- Status: Proposed
- Related: [PG17 online surface contract](2026-08-03-query-access-pg17-count-online-surface-contract.md), [PostgreSQL resolver core](2026-08-11-query-access-postgresql-resolver-core.md), [API lifecycle issue #3](https://github.com/Fanduzi/DeltaScope/issues/3), [test ownership issue #4](https://github.com/Fanduzi/DeltaScope/issues/4)
- Spec: `docs/plans/2026-08-12-query-access-online-analysis-entry-spec.md`
- Design: `docs/plans/2026-08-12-query-access-online-analysis-entry-design.md`
- Implementation: `docs/plans/2026-08-12-query-access-online-analysis-entry-implementation.md`

## Context

Online Query Access uses a caller-owned pinned connection and identity-derived
proof capability. The public SDK currently exposes separate PostgreSQL and
MySQL/TiDB session wrappers. CLI and HTTP each repeat product inspection,
dialect selection, wrapper construction, and dialect-specific dispatch after
they have already opened and identified an online connection.

That routing is a Query Access responsibility, not a transport responsibility.
Keeping it in transports multiplies wiring and makes future online proof work
pay a repeated SDK/CLI/HTTP integration cost.

Official DeltaScope release binaries already include PostgreSQL support. A
separate no-`postgresql`-tag Go source build remains useful for compile-time
compatibility and testing, but is not an official PostgreSQL-disabled product
edition.

## Proposed Decision

Add an opaque `OnlineQueryAccessSession` constructed only from a caller-owned
`*sql.Conn`, plus `AnalyzeOnlineQueryAccessWithSession`. Construction pings and
identifies the server on that connection and stores a private capability route.
The wrapper does not own the connection and exposes no identity or capability
state.

An empty request dialect uses observed identity. A non-empty request dialect is
only a constraint and must match; it cannot choose the proof engine. Caller
analysis profiles and schema resolvers remain forbidden.

The new API uses bounded generic online-session errors and a fixed validation
order. Existing dialect-specific public APIs remain available with their exact
errors and validation behavior. They may share private proof helpers but are
not deprecated or removed by this decision.

Migrate CLI and HTTP product routing to the unified SDK entry. Keep connection
selection, authorization, configuration, opening/closing, cancellation, and
external error presentation in their transports. MCP remains without Query
Access.

## Rationale

One public routing seam makes the SDK own the product-to-proof decision while
leaving security-sensitive connection ownership visible. Deriving the route
from observed identity prevents a caller hint from selecting a stronger or
incorrect proof path. Keeping the wrapper opaque prevents the session API from
becoming a capability-discovery or identity-export surface.

Additive migration is safer than immediate replacement. The old APIs have
different public errors and validation order, and existing consumers may rely
on them. A private execution core can remove production duplication without
silently rewriting those contracts.

Official and source-build concerns are separated deliberately: published
binaries support all current products, while an untagged source build preserves
the same API shape and fails closed if it observes PostgreSQL without the
linked capability.

## Contract

- The caller supplies and closes one pinned `*sql.Conn`; the SDK never opens,
  pools, closes, or retries it.
- Server identity, not caller input, determines the proof route.
- Supported capability remains exactly MySQL 5.7/8.0/8.4, TiDB 8.5, and
  PostgreSQL 17 with their existing proof boundaries.
- The generic API rejects dialect mismatch, caller profile, caller resolver,
  unavailable session, and unsupported linked capability with bounded
  sentinels in the documented fixed order.
- No user SQL is executed and no identity, version, endpoint, credential,
  catalog, OID, backend, SQL, or driver detail is exposed.
- Existing dialect-specific API behavior and non-PostgreSQL build stubs remain
  unchanged.
- CLI and HTTP external behavior remains unchanged; MCP gains no Query Access
  surface.
- Existing repeated behavior tests remain until a separate issue proves safe
  consolidation.

## Consequences

Positive consequences:

- CLI and HTTP stop owning database-product proof routing.
- Direct SDK callers get one online entry across supported products.
- Future proof-path work has one public integration seam.
- Identity-derived routing and caller-owned connection semantics remain
  explicit and testable.

Costs and limitations:

- Existing dialect-specific APIs and tests remain, so the milestone is
  temporarily additive rather than immediately smaller.
- The SDK needs a small tagged/untagged private capability boundary.
- Generic error mapping adds a second compatibility contract alongside old API
  errors.
- Transport test duplication remains pending issue #4.

## Alternatives Rejected

- Keep transport product switches: rejected because they duplicate an SDK
  routing decision.
- Accept caller identity/product/profile: rejected because those are trust facts
  derived from the connection.
- Open the connection inside the SDK: rejected because credentials,
  authorization, TLS, and lifecycle belong to callers/transports.
- Expose session getters: rejected because they create an unnecessary public
  identity/capability surface.
- Immediately deprecate old APIs: rejected pending issue #3 and compatibility
  evidence.
- Delete repeated tests during migration: rejected pending issue #4 and an
  explicit ownership matrix.

## Deferred Scope

- Deprecation or removal of dialect-specific session APIs, tracked in issue #3.
- Consolidation of repeated SDK/CLI/HTTP behavior matrices, tracked in issue
  #4. Transport lifecycle, authorization, failure mapping, and no-leak evidence
  must remain at the owning surface.
- New products, versions, SQL shapes, proof engines, outputs, or MCP surface.
- Removal of the no-PostgreSQL-tag source compatibility build.
- New session concurrency, caching, retry, discovery, or connection lifecycle
  APIs.

## Acceptance Evidence

This section must be completed before changing the status to Accepted:

- exact public API, opacity, ownership, validation-priority, and build-boundary
  tests;
- unified-versus-dialect API equivalence across every supported product;
- recording-driver no-execution/no-leak and real same-connection integration
  evidence;
- CLI and HTTP external compatibility with product switches removed;
- unchanged old APIs, MCP absence, supported SQL boundaries, and repeated test
  matrices;
- complete default, PostgreSQL-tagged, race, build, vet, lint, corpus, Docker,
  documentation, formatting, tidy, and diff gates;
- independent review with no remaining P0, P1, or P2 finding.
