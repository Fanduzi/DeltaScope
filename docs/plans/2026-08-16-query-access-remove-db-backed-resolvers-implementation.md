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
