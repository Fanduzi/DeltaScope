# Decision: Remove DB-Backed Query Access Resolvers

- Date: 2026-08-16
- Status: Proposed
- Related: [Issue #2](https://github.com/Fanduzi/DeltaScope/issues/2), [PostgreSQL resolver core](2026-08-11-query-access-postgresql-resolver-core.md), [Unified online analysis entry](2026-08-12-query-access-online-analysis-entry.md)
- Spec: `docs/plans/2026-08-16-query-access-remove-db-backed-resolvers-spec.md`
- Design: `docs/plans/2026-08-16-query-access-remove-db-backed-resolvers-design.md`
- Implementation: `docs/plans/2026-08-16-query-access-remove-db-backed-resolvers-implementation.md`

## Context

The PostgreSQL and MySQL/TiDB metadata packages retain Query Access resolvers
backed by `*sql.DB`. Repository-wide call analysis finds that both constructors
are used only by tests. Production online Query Access already uses a
caller-owned `*sql.Conn` so server identity, relation metadata, function
identity, and proof stay on one session.

The PostgreSQL resolver-core milestone intentionally retained its DB adapter
and deferred nil, lifecycle, and error-policy decisions to issue #2. Further
analysis shows that accepting those edge cases as a durable contract would
maintain an unsupported ownership model rather than protect a consumer.

## Proposed Decision

Delete both DB-backed Query Access resolver types, constructors, PostgreSQL
no-tag stub, and DB-only tests. Do not leave compatibility wrappers.

Keep the existing conn-backed resolvers as the only infrastructure-owned
online metadata path. Preserve their current constructors, errors, validation,
catalog behavior, caller ownership, same-session guarantee, no-execution, and
no-leak contracts.

Before deleting tests, reconcile every asserted behavior with an exact retained
conn/core/live owner. Migrate only obligations that would otherwise become
unowned. PostgreSQL trusted-service integration must use a pinned connection,
matching production.

## Rationale

Dead internal adapters are cheaper and safer to delete than to specify. A
`*sql.DB` can select different pooled connections and therefore cannot satisfy
the trusted online proof boundary. No external module can import these
`internal/` types, and no production code constructs them.

Deleting both dialect families resolves the same accidental duplication once.
It does not imply cross-dialect implementation or error parity; each remaining
conn adapter keeps its established production contract.

## Contract

- No DB-backed Query Access resolver, constructor, stub, or caller remains.
- Trusted online metadata and proof use caller-owned `*sql.Conn` only.
- Public Query Access API, output, SQL support, authorization, and transports
  remain unchanged.
- Conn-backed lifecycle, error, privacy, catalog, and relation behavior remain
  unchanged.
- Every deleted test obligation has a named retained or migrated owner.
- Historical release notes remain historical; prior ADRs link to this later
  decision instead of being rewritten.

## Consequences

Positive:

- One supported online metadata ownership model remains.
- Nil DB and pool-lifecycle behavior no longer need a contract.
- Tests and documentation match the production same-session trust boundary.

Costs:

- Test-only callers must migrate or be deleted after evidence reconciliation.
- Prior ADRs that describe the retained DB adapter need a follow-up link.
- Future non-trusted pool metadata, if ever required, must justify a new adapter
  rather than reviving this one implicitly.

## Alternatives Rejected

- Characterize and harden nil DB behavior: rejected because no supported caller
  benefits from the additional contract.
- Keep deprecated wrappers: rejected because a pool cannot forward safely to a
  pinned-session contract.
- Delete PostgreSQL only: rejected because MySQL/TiDB has the same test-only
  adapter and no separate requirement.
- Normalize conn-backed errors now: rejected as an unrelated production and
  privacy behavior change.

## Deferred Scope

- Any new pool-backed metadata product requirement.
- Conn-backed error normalization or cross-dialect parity.
- Public API, transport, authorization, SQL capability, release, or package
  changes.

## Acceptance Evidence

This section remains intentionally incomplete while the decision is Proposed.
Acceptance requires the fixed implementation candidate, ownership
reconciliation, full specified gates, real PostgreSQL and MySQL/TiDB evidence,
and independent Standards/Spec review with no unresolved P0/P1/P2.
