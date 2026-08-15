# Spec: Consolidate Query Access Test Ownership

## Status

Proposed. This document defines the evidence-preserving cleanup tracked by
GitHub issue #4. It does not authorize implementation, test deletion, merging,
release, or issue closure.

## Problem

The unified online Query Access entry now owns identity-derived routing for
MySQL 5.7/8.0/8.4, TiDB 8.5, and PostgreSQL 17. CLI and HTTP delegate to that
entry, but their tests still repeat much of the SDK product, profile, SQL-shape,
admission, requirement, and reason matrix.

The current Query Access test inventory is approximately 10,758 lines across
SDK, CLI, HTTP, and real-binary suites. About 2,710 lines exercise the now-
deprecated dialect-specific SDK sessions. Duplication was intentionally kept
during the unified-entry migration so equivalence could be reviewed. Keeping
all of it indefinitely makes every new SQL shape pay for the same semantic
assertions on three surfaces.

Deletion is safe only if evidence ownership remains explicit. Transport tests
also prove properties the SDK cannot: exit codes, stdout/stderr, HTTP status and
JSON, authorization-before-dial, access logs, connection lifecycle, and leakage
through each public sink. Accepted ADRs additionally require PostgreSQL foreign
tables to fail closed on every online surface.

## Objective

Make the unified SDK entry the single owner of exhaustive semantic behavior
while retaining the smallest sufficient compatibility and transport evidence.
Reduce repeated test code and Docker work without changing production behavior,
public contracts, fixtures, or required gates.

## Required Ownership Matrix

| Evidence | Owning suite |
|---|---|
| All supported products, versions, profiles, SQL shapes, admission, requirements, reasons, and proof failures | Unified SDK |
| Exact catalog/identity probe sequence and complete no-execution proof | Unified SDK recording driver |
| Deprecated API source shape, errors, validation priority, caller ownership, build stubs, and per-target equivalence | Deprecated SDK compatibility suite |
| Flags, exit codes, stdout/stderr, connection/TLS lifecycle, bounded CLI errors, and CLI no-leak | CLI |
| HTTP status/code/body, `connection_id`, authorization/purpose, zero-dial guards, lifecycle, access logs, and HTTP no-leak | HTTP |
| Absence of a Query Access tool | MCP surface contract |

## Required Contract

1. The unified SDK retains the exhaustive semantic matrix for MySQL 5.7,
   MySQL 8.0, MySQL 8.4, TiDB 8.5, and PostgreSQL 17. It covers admitted,
   rejected, indeterminate, relationless, literal/reversed, aggregate, unknown,
   malformed, catalog-failure, and privacy boundaries already supported by the
   repository.
2. Future SQL shapes and profiles are tested exhaustively at the unified SDK
   seam. A new server version using an existing driver/configuration family is
   added there, not copied into each transport matrix.
3. CLI and HTTP each retain real online smoke evidence for MySQL 8.4, TiDB 8.5,
   and PostgreSQL 17. Each product family has at least one admissible and one
   fail-closed result with externally visible classification/admission plus the
   key requirement or reason asserted.
4. PostgreSQL transport coverage additionally retains a syntax-envelope
   negative, a foreign-table negative, and default/offline indeterminate
   behavior. Foreign-table rejection is a relation-kind trust boundary and may
   not be substituted by `COUNT(2)`, `FILTER`, or another syntax-only case.
5. CLI keeps transport-owned evidence for flags, TLS/session configuration,
   exit codes, stdout/stderr, cancellation, connection and catalog failures,
   close ownership, offline behavior, and no-leak markers for success,
   fail-closed, and failure paths.
6. HTTP keeps transport-owned evidence for request parsing, status/code/body,
   registry lookup, `connection_id`, authorization and purpose checks,
   unauthorized/unknown zero-dial behavior, cancellation, connection and
   catalog failures, close ownership, synchronized access logs, request IDs,
   offline behavior, and no-leak markers for success, fail-closed, and failure
   paths.
7. The unified SDK recording-driver suite owns complete identity/catalog probe
   sequencing and user-SQL no-execution. CLI and HTTP each retain one focused
   adapter recording test proving that a pinned connection reaches the unified
   entry, user SQL/`EXPLAIN`/prepare do not execute, and transport-owned close
   and error mapping remain correct.
8. Deprecated API tests retain tagged/untagged availability, deprecation
   markers, exact old error identities and validation priority, caller-owned
   connection behavior, and one unified-versus-legacy equivalence case for each
   supported target: MySQL 5.7, 8.0, 8.4, TiDB 8.5, and PostgreSQL 17. Full
   per-target SQL-shape matrices may move to the unified SDK.
9. Every removed test or table row appears in a deletion ledger. A deletion is
   allowed only when the ledger points to both the retained SDK semantic proof
   and, when externally observable, the retained owner-specific transport or
   compatibility proof.
10. Test files cited by Accepted ADRs should remain at their existing paths and
    be reduced in place. If an entire cited file must be removed, add a concise
    follow-up note to the old ADR pointing to this decision and the replacement
    evidence. Do not rewrite the old acceptance history.
11. Before final review, run temporary mutation probes that independently break
    SDK shape admission, CLI exit mapping, HTTP authorization/zero-dial,
    PostgreSQL foreign-table rejection, and the target stored by one deprecated
    MySQL/TiDB wrapper after identity derivation. The last mutation must affect
    only the deprecated wrapper, not the shared target-to-profile helper, so an
    equivalence test cannot pass while both paths are wrong. Each retained
    owning test must fail. Restore every mutation and commit no mutation tool or
    framework.
12. Add no shared cross-package test framework, permanent matrix generator,
    custom static checker, dependency, or production abstraction. Local helpers
    may remain within their owning package only when they reduce existing code.
13. Do not change production code, public APIs, SQL behavior, results, errors,
    authorization, privacy contracts, Makefile targets, workflows, fixture
    contracts, versions, or release surfaces.
14. Do not remove or weaken existing required default, PostgreSQL-tagged, race,
    corpus, Docker E2E, TLS, documentation, formatting, or module gates.
15. Record before/after test lines, case counts, Docker invocations, and measured
    runtimes, but set no deletion or speed KPI. Evidence equivalence, not a
    target percentage, determines success.
16. Update the durable ownership rule in `docs/dev/testing.md` and affected L2
    module READMEs. Keep implementation-specific deletion details in this spec
    and implementation plan rather than adding another permanent policy file.
17. MCP gains no Query Access surface. Its no-surface contract remains.
18. GitHub issue #4 closes only after the implementation and ownership docs are
    merged, this ADR is Accepted, and required CI for the exact merged SHA is
    successful.

## Deletion Ledger Requirements

Each ledger row records:

- deleted file/test/subtest or table rows;
- behavior formerly asserted;
- retained unified SDK test;
- retained transport or deprecated-API contract test, when applicable;
- relevant Accepted ADR constraint;
- mutation probe, if the behavior is one of the five required probe classes.

An unmapped deletion is blocked. Similar SQL text alone does not prove
redundancy when tests observe different sinks or lifecycle boundaries.

## Explicit Non-Goals

- No production refactor or public-contract change.
- No new Query Access capability, product, version, shape, output, or MCP tool.
- No removal of deprecated APIs or their minimum compatibility contract.
- No consolidation of CLI and HTTP privacy evidence into SDK tests.
- No replacement of real transport smoke with mocks only.
- No shared test-support package, generated matrix, static policy checker, or
  mutation-testing dependency.
- No Makefile/workflow/gate reduction, fixture redesign, version bump, release,
  or publication.

## Acceptance Evidence

The ADR may change to Accepted only after:

- the ownership matrix and deletion ledger cover every removed test;
- retained unified SDK tests cover every supported target and semantic class;
- retained CLI/HTTP/legacy/MCP tests satisfy the contracts above;
- all five temporary mutation probes fail at the intended retained tests and
  the restored tree is clean;
- public behavior and production files are byte-for-byte unchanged from the
  milestone base;
- default and PostgreSQL-tagged full tests, affected race tests, Query Access
  corpus, PostgreSQL unit/confidence, MySQL/TiDB and PostgreSQL CLI/HTTP/MCP
  Docker E2E, CLI/HTTP TLS, lint, vet, build, documentation, decision-record,
  gofmt, module-tidy, and diff checks pass; and
- an independent read-only review reports no unresolved P0, P1, or P2 and
  confirms that no security, privacy, lifecycle, compatibility, or semantic
  evidence was orphaned.
