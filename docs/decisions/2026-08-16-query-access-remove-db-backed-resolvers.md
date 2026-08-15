# Decision: Remove DB-Backed Query Access Resolvers

- Date: 2026-08-16
- Status: Accepted
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

Implemented and reviewed on branch
`feat/query-access-remove-db-backed-resolvers-20260816`; fixed reviewed
candidate is base `d7ce52b782f99f0cb17e790a8a53873a418fb08b` (local `main`
at review time) through candidate `dccf4dd04f465635227f1d43feba68832c933b8b`
(commits `2eae1db`, `2a64b8a`, `52731a8`, `10ae97c`, `dccf4dd`):

- No DB-backed Query Access resolver type, constructor, stub, or caller remains:
  PostgreSQL `query_access_resolver.go` and its no-tag stub and MySQL
  `query_access_resolver.go` were deleted; `rg NewQueryAccessResolver --type go`
  returns nothing and CodeGraph shows no remaining symbol.
- Ownership reconciliation in the implementation plan (§2) maps every deleted
  PostgreSQL contract row and every deleted MySQL unit test to an exact
  retained conn/core/live owner or a migrated conn test; PostgreSQL
  trusted-service integration tests now build their schema resolver from the
  pinned `session.Conn()` via `NewQueryAccessConnResolver`.
- Conn-backed constructors, validation priority, errors, catalog queries,
  relation classification, caller ownership, no-execution, no-leak, and public
  result contracts are unchanged; the PostgreSQL core changed only its L3
  header comment. No MySQL/TiDB and PostgreSQL error parity was forced.
- Gates passed: default and PostgreSQL-tagged full tests, affected race tests
  (metadata packages and `pkg/deltascope`), build, `go vet` (both tags),
  `make lint`, `make query-access-corpus-gates`, `make pg-unit-test-gates`,
  `make pg-confidence-gates` (Docker CLI/HTTP/MCP PG17 e2e), and the unified
  SDK live matrix for MySQL 5.7/8.0/8.4 plus TiDB 8.5; decision-record,
  gofmt, three-level-doc, tidy, and diff checks pass.
- Independent read-only review rounds report no unresolved P0/P1/P2. The first
  fixed-SHA round (base `d7ce52b…` through `52731a88…`) passed; a follow-up
  independent review then raised four P2 and one P3 findings (stale L3 header
  wording, vestigial single-factory harness, partial column-order assertion,
  missing PG17 ADR evidence-maintenance notes, and a movable `main...SHA`
  review reference). All five were fixed in `dccf4dd`, the ADR was returned to
  Proposed, and a fresh read-only Standards and Spec review of the final
  fixed-SHA candidate (base `d7ce52b782f99f0cb17e790a8a53873a418fb08b` through
  `dccf4dd04f465635227f1d43feba68832c933b8b`) confirmed each fix and found no
  P0/P1/P2.
- A pre-existing `postgresql,integration` manifest-boundary failure
  (`TestScalarLive_PG17CatalogBoundQueriesPromote`, `coalesce` outside the
  closed PG17 manifest) reproduces identically on clean `main` and is not part
  of any gate; it is outside this decision's scope.

## Links

- Commits: `2eae1db` docs proposal, `2a64b8a` test reconciliation, `52731a8`
  refactor removal, `10ae97c` initial acceptance, `dccf4dd` review-fix commit,
  final acceptance commit following this record.
- Tests: `internal/infrastructure/metadata/mysql/query_access_resolver_test.go`
  (migrated conn tests), `internal/infrastructure/metadata/postgresql/query_access_resolver_test.go`
  (conn contract + lifecycle), `query_access_conn_resolver_integration_test.go`
  (same-session proof), `effect_identity_resolver_integration_test.go`
  (pinned trusted-service path).
- Docs: both metadata READMEs, this decision's spec/design/implementation,
  evidence-maintenance notes in
  `2026-07-11-query-access-analysis-foundation.md` and
  `2026-07-31-query-access-pg17-count-literal-proof.md`, follow-up link in
  `2026-08-11-query-access-postgresql-resolver-core.md`.
