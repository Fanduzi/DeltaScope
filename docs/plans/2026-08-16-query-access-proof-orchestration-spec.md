# Spec: Consolidate Query Access Proof Orchestration

## Status

Proposed. This document defines a behavior-preserving internal refactor. It
does not authorize implementation, merge, push, release, or issue closure.

## Problem

`Service.Analyze` currently reaches its final Query Access state through three
different proof clocks:

- ordinary PostgreSQL manifest proof runs before requirements are built;
- the exact PostgreSQL `COUNT(1)` proof runs after requirements are built; and
- MySQL/TiDB builtin semantic proof runs after requirements are built.

The method normalizes reasons, reclassifies reads, and recomputes admission at
multiple points. `reclassifyAfterResolution` accepts a PostgreSQL proof plus a
variadic MySQL/TiDB proof, exposing proof-specific control flow in the common
pipeline. A future change can therefore update one clock while missing the
others even though all three govern the same promotion boundary.

## Objective

Make `Service.Analyze` build requirements before every Effect Proof, route all
proof decisions through one private orchestration step, and compute final
classification and admission once. Preserve every existing public result,
catalog interaction, failure boundary, and fail-closed condition.

## Required Contract

1. The common pipeline is:
   `extract -> metadata and Promotion Barriers -> requirements -> proof
   orchestration -> reason normalization -> reclassification -> admission ->
   sorting and validation`.
2. Requirements are complete before any proof may allow promotion. Moving the
   ordinary PostgreSQL proof after the pure in-memory requirement builder is
   the only permitted execution-order change.
3. One private orchestration function owns proof applicability, invokes the
   existing PostgreSQL or MySQL/TiDB proof, removes only that proof's existing
   success reasons, and returns whether proof permits the common promotion
   checks to continue.
4. The orchestration result is not an admission decision. Promotion still
   requires the existing resolver, classification, reason, unresolved,
   requirement, and barrier conditions.
5. PostgreSQL may permit reclassification from `indeterminate` only after an
   applicable trusted manifest proof returns `all_proven`. No candidate or no
   trusted proof is not vacuous success.
6. MySQL/TiDB queries without effect candidates retain the ordinary
   reclassification path. Queries with effect candidates require the existing
   builtin semantic proof to succeed before promotion.
7. View, unqualified relation, unresolved, wildcard, parse-failure, and write
   barriers remain fail closed. Proof never changes `not_read_only`, never
   overrides `rejected`, and never removes a reason it does not own.
8. Exact PostgreSQL `COUNT(1)` retains its complete-statement and one-physical-
   table requirement predicate before catalog proof. Foreign tables and every
   excluded shape remain indeterminate.
9. PostgreSQL keeps its `TrustDecision`, atomic same-session catalog proof,
   batch validation, and bounded failure reasons. MySQL/TiDB keeps its builtin
   decision and manifest behavior. The common layer does not merge those
   domain-specific models.
10. Final reason normalization, reclassification, and admission recomputation
    each occur once after proof orchestration. Final sorting and validation
    remain unchanged.
11. Existing catalog probe count/order, cancellation behavior, caller-owned
    connection lifetime, no user-SQL execution, bounded errors, and no-leak
    behavior remain unchanged.
12. No public SDK, CLI, HTTP, MCP, JSON, SQL support, profile, manifest,
    authorization, dependency, fixture, workflow, version, or release surface
    changes.
13. Add only the smallest application-layer orchestration contract test.
    Existing SDK, transport, corpus, recording, and live tests remain the
    behavioral equivalence owners; do not duplicate their matrices.

## Explicit Non-Goals

- No `proofEngine` interface, implementation registry, factory, plugin, or
  configuration mechanism.
- No redesign of identity-resolution interfaces or trust-policy internals.
- No new proof type, SQL shape, database product, or server version.
- No optimization that skips a catalog call currently made by an eligible
  path, even for a rejected result.
- No reason-code cleanup, error-text normalization, or public schema change.
- No consolidation or deletion of SDK, CLI, HTTP, MCP, corpus, or live tests.

## Acceptance Evidence

The ADR may become Accepted only after:

- characterization tests demonstrate RED when proof runs before requirements,
  a barrier is allowed through, the wrong reason is removed, or a dialect's
  proof applicability is changed;
- the common pipeline contains one proof-orchestration call and one final
  normalize/reclassify/admission sequence without a custom source checker;
- representative PostgreSQL ordinary, exact `COUNT(1)`, MySQL/TiDB builtin,
  no-effect, rejected, barrier, unknown, and failure cases retain exact domain
  results and probe behavior;
- default and PostgreSQL-tagged full tests, affected race tests, Query Access
  corpus, PostgreSQL unit/confidence gates, builds, vet, lint, and relevant
  recording/live tests pass;
- decision-record, gofmt, three-level-documentation, module-tidy, and diff
  checks pass; and
- fresh Standards and Spec/security review of a fixed candidate reports no
  unresolved P0, P1, or P2.
