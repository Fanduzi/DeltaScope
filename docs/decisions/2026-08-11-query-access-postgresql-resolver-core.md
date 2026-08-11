# Decision: Preserve PostgreSQL Resolver Ownership While Sharing Its Core

- Date: 2026-08-11
- Status: Proposed
- Related: [Query Access pure-read admissibility](2026-07-12-query-access-pure-read-admissibility.md), [PG17 `COUNT(1)` proof](2026-07-31-query-access-pg17-count-literal-proof.md), [PG17 online surface contract](2026-08-03-query-access-pg17-count-online-surface-contract.md), [follow-up issue #2](https://github.com/Fanduzi/DeltaScope/issues/2)
- Spec: `docs/plans/2026-08-11-query-access-postgresql-resolver-core-spec.md`
- Design: `docs/plans/2026-08-11-query-access-postgresql-resolver-core-design.md`
- Implementation: `docs/plans/2026-08-11-query-access-postgresql-resolver-core-implementation.md`

## Context

The PostgreSQL metadata package exposes an internal DB-backed schema resolver
and a separate conn-backed schema resolver. The trusted online Query Access
path relies on the second type because its metadata and identity proof must run
on one caller-owned backend session.

The two adapters currently duplicate catalog SQL, query and row scanning,
column loading, and associated error construction. Relation-kind mapping and
foreign-table rejection already have one implementation, but those helpers are
located in the DB-backed adapter file even though the conn-backed adapter uses
them. The remaining duplication creates drift risk and the shared policy lacks
a neutral owner.

Removing the type distinction would be a false simplification. A `*sql.DB`
pool and one pinned `*sql.Conn` are not interchangeable evidence for trusted
proof.

## Proposed Decision

Keep `QueryAccessResolver` and `QueryAccessConnResolver` as separate concrete
adapters with their current constructors, method signatures, concrete handle
fields, lifecycle behavior, and build-tag stubs.

Extract their common PostgreSQL relation lookup into stateless private
functions in the PostgreSQL metadata package. Those functions accept only the
minimal private query capability naturally implemented by `*sql.DB` and
`*sql.Conn`. The adapter structs do not store an abstract query capability and
the capability is not exported.

The shared core becomes the single production owner of:

- relation-kind catalog SQL and scanning;
- column catalog SQL and scanning;
- accepted relation-kind mapping;
- foreign-table fail-closed rejection;
- relation lookup and scan error construction.

The DB-backed adapter remains ordinary schema metadata infrastructure. The
conn-backed adapter remains the only one assembled into the caller-owned
trusted PostgreSQL session path. Sharing code does not make the DB-backed
adapter session-pinned or eligible as proof evidence.

## Rationale

The deepest useful module boundary is the stable PostgreSQL relation-resolution
algorithm, not the database handle type. Sharing that algorithm prevents policy
and SQL drift. Keeping two thin adapters makes ownership explicit and preserves
the same-session trust boundary in types and tests.

A private, stateless core avoids a third stateful resolver and prevents a
generic interface field from hiding a pool behind the conn-backed path. Keeping
the abstraction inside the PostgreSQL package also avoids pretending that
PostgreSQL and MySQL/TiDB have identical catalog, relation-kind, placeholder,
and error semantics.

## Contract

- This is a zero-observable-behavior PostgreSQL refactor.
- Existing internal types, constructors, method signatures, stubs, SQL order,
  errors, relation mappings, and `RelationSchema` output remain unchanged.
- `QueryAccessConnResolver` retains a concrete `*sql.Conn`, no `*sql.DB` field,
  no pool fallback, and no ownership of connection close.
- Conn lifecycle behavior remains exact: nil constructor input returns
  `ErrSessionNotPinned`; `ResolveRelation` checks context before detecting a nil
  receiver or nil internal connection as `ErrSessionClosed`; a non-nil but
  already-closed `*sql.Conn` reaches the shared query path and retains its
  existing wrapped query error.
- Trusted online PostgreSQL Query Access continues to use the conn-backed
  resolver and identity adapter created from the same caller-owned connection.
- MySQL/TiDB and all public SDK, CLI, HTTP, MCP, admission, privacy, and result
  contracts remain unchanged.

## Consequences

Positive consequences:

- PostgreSQL relation lookup and foreign-table safety policy have one
  implementation.
- DB and Conn adapter behavior can be compared through one parameterized test
  contract.
- Future catalog fixes no longer require synchronized edits to two production
  algorithms.
- The session-pinned guarantee stays visible in the conn adapter's concrete
  type and assembly path.

Costs and limitations:

- Two thin adapters and their ownership-specific tests remain intentionally.
- The private query capability adds a small internal abstraction.
- Historical DB-backed nil and error behavior remains untouched pending issue
  #2.
- MySQL/TiDB duplication is not solved by this milestone.

## Alternatives Rejected

- One resolver type with an interface-backed handle: rejected because it hides
  whether trusted code received a pool or a pinned connection.
- Cross-dialect `catalogQueryer` package: rejected because catalog and safety
  semantics differ by dialect.
- Remove the DB-backed adapter: rejected as a separate compatibility decision.
- Fix nil and error behavior during extraction: rejected because it would mix
  behavior changes into a structural refactor.

## Deferred Scope

- PostgreSQL DB-backed constructor, nil, lifecycle, and error-policy changes,
  tracked in GitHub issue #2.
- MySQL/TiDB DB/Conn resolver drift and any later dialect-local deduplication.
- Public online-analysis API consolidation, proof-engine extraction, and
  transport test-matrix restructuring.

## Acceptance Evidence Required

This decision remains Proposed until implementation provides:

- one neutral production implementation of the shared catalog algorithm, with
  evidence distinguishing newly deduplicated query behavior from the mapping
  and foreign-table helpers that were already shared;
- parameterized DB/Conn behavior-contract coverage;
- adapter-specific ownership and lifecycle coverage for nil constructor, nil
  receiver/field, already-closed connection, and cancellation precedence, plus
  the conn adapter's exact concrete field shape;
- real PG17 same-backend-PID and foreign-table fail-closed evidence;
- complete required Go, PostgreSQL, race, build, vet, lint, formatting,
  module-tidy, decision-record, and three-level documentation gates;
- final scope evidence showing no MySQL/TiDB or public-contract change;
- an independent read-only review with no P0, P1, or P2 finding.

Only after that evidence exists may a focused documentation commit change the
status to Accepted and record concrete test and commit references.
