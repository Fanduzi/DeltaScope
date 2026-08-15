# Spec: Remove DB-Backed Query Access Resolvers

## Status

Proposed. This document defines the scope of GitHub issue #2. It does not
authorize implementation, merge, push, release, or issue closure.

## Problem

The PostgreSQL and MySQL/TiDB metadata packages each retain a Query Access
resolver backed by `*sql.DB`. Both adapters are referenced only by tests.
Production online Query Access uses a caller-owned `*sql.Conn` so identity,
catalog lookup, and proof remain on one database session.

Keeping the unused adapters creates a second ownership model with nil-handle,
pool-lifecycle, and error-leak behavior that has no production purpose. The
PostgreSQL adapter also makes the shared catalog core appear usable for trusted
pool-backed proof even though that is explicitly unsupported.

## Objective

Delete both unused DB-backed adapters and make the existing conn-backed
resolvers the only infrastructure-owned online metadata path. Preserve all
production behavior and retain every durable test obligation under an accurate
conn/core/live owner.

## Required Contract

1. Delete PostgreSQL `QueryAccessResolver`, `NewQueryAccessResolver(*sql.DB)`,
   its no-tag stub, and all DB-only test construction.
2. Delete MySQL/TiDB `QueryAccessResolver`,
   `NewQueryAccessResolver(*sql.DB)`, and all DB-only test construction.
3. Do not add aliases, deprecated wrappers, panic stubs, factories, or a
   replacement pool-backed resolver.
4. Trusted online proof continues to use only caller-owned `*sql.Conn` through
   the existing conn-backed resolvers and unified Online Query Access Session.
5. Do not change conn-backed constructors, validation priority, error identity
   or text, catalog queries, relation classification, connection ownership,
   public results, supported SQL, or no-execution/no-leak behavior.
6. Before deleting tests, create an explicit ownership reconciliation for
   table/view classification, column order, missing relation, cancellation,
   unsupported relation kind, query/scan/iteration failure, caller ownership,
   same-session proof, and trusted-service integration.
7. Delete a DB-backed test row when existing conn/core/live evidence already
   owns the behavior. Migrate only obligations with no remaining owner; do not
   reproduce the old matrices merely to preserve test counts.
8. PostgreSQL trusted-service integration tests that still construct the
   DB-backed resolver must move to the existing pinned-session path.
9. MySQL/TiDB and PostgreSQL may retain different bounded error contracts. This
   milestone does not force cross-dialect parity.
10. Historical release notes remain unchanged. The Accepted 2026-08-11
    resolver-core ADR receives a follow-up link; older foundation and PG17 ADRs
    receive concise evidence-maintenance notes where they cite deleted owners.
11. The metadata module READMEs and changed Go file headers must describe only
    the remaining conn-backed ownership model.
12. Issue #2 closes only after implementation is merged, this decision is
    Accepted, and required CI for the merged SHA succeeds.

## Explicit Non-Goals

- No public SDK, CLI, HTTP, MCP, JSON, authorization, or release change.
- No new Query Access capability, product, version, SQL shape, or finding.
- No merge of PostgreSQL and MySQL/TiDB resolver implementations.
- No rewrite of the private PostgreSQL catalog core unless deletion exposes
  dead code that has no remaining caller.
- No normalization of production errors or new sentinels.
- No characterization contract for dead DB-backed nil or lifecycle behavior.
- No edit to v0.480.0 release notes, which describe that historical release.

## Acceptance Evidence

The ADR may become Accepted only after:

- source and CodeGraph checks show no DB-backed Query Access resolver type,
  constructor, stub, or caller remains;
- the ownership reconciliation maps every deleted test obligation to an exact
  retained or migrated test;
- focused conn/core tests cover any previously unowned behavior;
- PostgreSQL trusted-service tests use a pinned caller-owned connection;
- default and PostgreSQL-tagged full tests, affected race tests, build, vet,
  lint, Query Access corpus, PostgreSQL unit/confidence gates, and the live
  MySQL 5.7/8.0/8.4 plus TiDB 8.5 unified SDK matrix pass;
- decision-record, gofmt, three-level-documentation, module-tidy, and diff
  checks pass; and
- independent Standards and Spec review reports no unresolved P0, P1, or P2.
