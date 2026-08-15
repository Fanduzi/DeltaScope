# Implementation Plan: Remove DB-Backed Query Access Resolvers

## Status

Proposed. This plan authorizes no implementation, merge, push, release, or
issue closure by itself.

## 1. Establish the Fixed Point

- Start one milestone branch/worktree from current `main` and record the full
  base SHA.
- Confirm issue #2 is open and this ADR is Proposed.
- Use CodeGraph and exact source search to enumerate both DB-backed types,
  constructors, stubs, callers, tests, and documentation references.
- Confirm every caller is test-only before editing. Any production caller is a
  blocker requiring renewed design review.

## 2. Reconcile Test Ownership

- Record every behavior asserted by the PostgreSQL and MySQL/TiDB DB-backed
  tests: relation kind, columns, missing relation, cancellation, unsupported
  kind, query/scan/iteration failure, lifecycle, same-session behavior, and
  trusted-service results.
- Map each row to an existing conn/core/live test or mark it genuinely unowned.
- Review the table before deletion. Do not use a permanent custom checker.

### Ownership Reconciliation (2026-08-16)

PostgreSQL DB-backed adapter obligations (all cases in
`TestQueryAccessResolvers_ResolveRelationContract` ran under a `db` factory):

| Obligation | Retained owner after deletion |
|---|---|
| Base/partitioned/view/materialized-view kind mapping | Conn factory run of the same parameterized contract (`query_access_resolver_test.go`) |
| Missing relation, foreign-table fail-closed | Conn factory run of the same contract |
| Relation/column query, scan, iteration errors | Conn factory run of the same contract |
| Cancellation (context checked before nil/closed handling) | `TestQueryAccessConnResolver_LifecycleContract` + cancellation row of the contract |
| Nil constructor (`ErrSessionNotPinned`), nil receiver/closed conn (`ErrSessionClosed`), closed-conn query path | `TestQueryAccessConnResolver_LifecycleContract` |
| Concrete `conn *sql.Conn` field, no `*sql.DB` field | `TestQueryAccessConnResolver_RetainsConcreteConnField` / `TestConnResolver_NoDBField` |
| Same-session metadata + backend-PID proof | `query_access_conn_resolver_integration_test.go` (`TestSameConnectionPID_Proof`) |
| Trusted-service promotion | `effect_identity_resolver_integration_test.go` migrated to pinned `session.Conn()` |

MySQL/TiDB DB-backed adapter obligations (`query_access_resolver_test.go`):

| Obligation | Deleted DB test | Retained/migrated owner |
|---|---|---|
| BASE TABLE kind + ordered columns | `TestQueryAccessResolver_TableExists`, `TestQueryAccessResolver_ColumnListing` | Migrated `TestQueryAccessConnResolver_TableKindAndColumns` |
| VIEW kind + `IsView` | `TestQueryAccessResolver_ViewExists` | Migrated `TestQueryAccessConnResolver_ViewKind` |
| Missing relation bounded error | `TestQueryAccessResolver_MissingTable` | Migrated `TestQueryAccessConnResolver_MissingRelation` (exact conn text `relation not found`) |
| Empty column listing | `TestQueryAccessResolver_MissingColumn` | Migrated `TestQueryAccessConnResolver_EmptyColumns` |
| Cancellation | `TestQueryAccessResolver_Cancellation` | Migrated `TestQueryAccessConnResolver_Cancellation` |
| Unsupported relation kind fail-closed | (DB adapter returned `table` for unknown types) | Retained `TestQueryAccessConnResolver_RejectsSystemView` (`unsupported relation type`) |

No MySQL DB-backed test asserted query/scan/iteration failure injection; the
MySQL test driver has no error-injection path, so no such obligation exists to
reconcile. No test asserted DB nil/pool-lifecycle behavior; those paths are
being deleted, not characterized.

## 3. Establish RED Evidence

- For each unowned durable obligation, add the smallest conn/core test and
  demonstrate RED before changing production or deleting old tests.
- Change PostgreSQL trusted-service integration setup to require the pinned
  resolver and demonstrate any missing wiring through focused RED evidence.
- Do not add tests for nil DB, nil DB-backed receiver, or pool lifecycle; those
  unsupported paths are being removed.

## 4. Delete PostgreSQL DB-Backed Wiring

- Delete `query_access_resolver.go` and its no-tag stub.
- Remove the DB variant from resolver contract tests while retaining or
  migrating each ledgered obligation.
- Migrate trusted-service integration tests to `PinSession` or an existing
  caller-owned `*sql.Conn` and `NewQueryAccessConnResolver`.
- Keep the private catalog core only to the extent required by the conn
  resolver; remove newly dead compatibility code.

## 5. Delete MySQL/TiDB DB-Backed Wiring

- Delete the DB-backed resolver implementation.
- Convert only unowned tests to the existing conn resolver; delete redundant
  DB-only matrix rows.
- Preserve all current conn constructor, lifecycle, bounded-error, relation
  classification, and catalog-query behavior.

## 6. Synchronize Documentation

- Update PostgreSQL and MySQL metadata READMEs and changed L3 headers.
- Add a follow-up/superseding link to the 2026-08-11 resolver-core ADR.
- Add concise evidence-maintenance notes to the 2026-07-11 foundation and
  2026-07-31 PG17 ADRs where necessary.
- Leave v0.480.0 release notes unchanged.
- Keep the new ADR Proposed until implementation and review evidence pass.

## 7. Verify Behavior and Scope

- Run focused resolver/core tests and affected race tests.
- Run default and PostgreSQL-tagged full tests, builds, and vet.
- Run `make lint`, `make query-access-corpus-gates`,
  `make pg-unit-test-gates`, and `make pg-confidence-gates`.
- Start the documented MySQL/TiDB builtin fixtures and run the unified SDK live
  matrix for MySQL 5.7/8.0/8.4 and TiDB 8.5; clean up only task-owned resources.
- Run decision-record, gofmt, three-level-doc, tidy, and diff checks.
- Confirm no SDK/CLI/HTTP/MCP production, public API, fixture, dependency,
  workflow, version, or release file changed.

## 8. Independent Review and Acceptance

- Request fresh read-only Standards and Spec reviews from the fixed base.
- Treat a missing evidence owner, remaining DB-backed symbol/caller, production
  error change, pool fallback, or historical-doc rewrite as blocking.
- Fix every P1/P2, rerun affected and full gates, and repeat review.
- Only after no P0/P1/P2 remains, change the ADR from Proposed to Accepted in a
  focused final commit citing a fixed reviewed candidate and range.

## 9. Delivery Closure

- Fast-forward local `main` only with human authorization and rerun required
  gates on the merged SHA.
- Push only with separate authorization; verify exact-SHA CI.
- Close issue #2 only after merge, push, green CI, and Accepted ADR evidence.
- Do not tag, release, publish, force-push, or delete branches/worktrees unless
  separately authorized.

## Suggested Commits

1. `docs(queryaccess): propose conn-only metadata resolver ownership`
2. `test(queryaccess): reconcile resolver evidence ownership`
3. `refactor(queryaccess): remove DB-backed metadata resolvers`
4. `docs(queryaccess): accept conn-only metadata resolver ownership`

## Milestone Evidence (2026-08-16)

Executed on branch `feat/query-access-remove-db-backed-resolvers-20260816` from
base `d7ce52b` (local `main`). Commits: `2eae1db` (docs propose),
`2a64b8a` (test reconcile), `52731a8` (refactor remove); the acceptance
commit follows this section.

- Source checks: `rg NewQueryAccessResolver --type go` → no matches;
  `rg QueryAccessResolver --type go` → only the `TestQueryAccessResolvers_*`
  test-function name and `QueryAccessConnResolver` types. CodeGraph confirms no
  DB-backed resolver symbol remains.
- Ownership reconciliation: table in §2 above; every deleted PostgreSQL contract
  row remains covered by the conn factory of the same parameterized test, and
  every deleted MySQL unit test maps to a migrated conn test or retained owner.
- Gates (all pass): `go test ./...` (4182), `go test -tags postgresql ./...`
  (4843), `go test -race` on `./pkg/deltascope` and both metadata packages,
  `go vet ./...` and `-tags postgresql`, `make lint` (0 issues),
  `make query-access-corpus-gates`, `make pg-unit-test-gates`,
  `make pg-confidence-gates` (incl. Docker CLI/HTTP/MCP PG e2e),
  unified SDK live matrix `go test -tags integration ./pkg/deltascope/ -run
  'TestLiveUnifiedSession'` (14) against MySQL 5.7/8.0/8.4 and TiDB 8.5,
  `go mod tidy` (no change), `git diff --check`, gofmt,
  `scripts/check_three_level_doc.sh`, `scripts/check_decision_record.sh`.
- Scope: `git diff --name-only main...HEAD` is confined to the two metadata
  packages and docs; no `pkg/`, `cmd/`, `internal/interfaces`, application,
  fixture, workflow, version, or release file changed. v0.480.0 release notes
  untouched.
- Pre-existing, out of scope: `TestScalarLive_PG17CatalogBoundQueriesPromote`
  (tags `postgresql,integration`) fails identically on clean `main` —
  `coalesce` is outside the closed PG17 manifest, so the query resolves as
  `unproven_function_effect`. The test is not part of any Makefile/CI gate.
- Independent review: fresh read-only Standards and Spec reviews of
  `main...52731a8` report no unresolved P0/P1/P2. Standards noted two
  non-blocking judgement calls (ADR link/evidence sections — resolved by the
  acceptance commit; single-factory parameterized harness — accepted).
