# Implementation Plan: Consolidate Query Access Proof Orchestration

## Status

Proposed. This is one implementation ticket on one milestone branch. It does
not authorize merge, push, release, or issue closure.

## 1. Establish the Fixed Point

- Start from current `main` and record the full base SHA.
- Confirm the milestone ADR is Proposed and the tracked tree is clean.
- Use CodeGraph plus source inspection to record every caller and test owner of
  `Service.Analyze`, `reclassifyAfterResolution`, `recomputeAdmission`,
  `resolveAndProveEffects`, `proveBuiltinSemantics`, and the exact `COUNT(1)`
  requirement predicate.
- Record current representative results and recording-driver probe sequences
  before editing.

## 2. Add Characterization RED Evidence

- Add one table-driven application-layer orchestration contract test.
- Cover ordinary PostgreSQL proof, exact `COUNT(1)`, MySQL/TiDB builtin proof,
  no-effect applicability, rejected SQL, views, unqualified relations,
  unresolved/wildcard input, proof failure, cancellation, and reason ownership.
- Make the smallest deliberate mutations needed to demonstrate that proof
  before requirements, barrier bypass, wrong reason removal, and incorrect
  applicability each fail. Restore every mutation byte-for-byte.
- Do not copy SDK or transport semantic matrices.

## 3. Introduce the Private Orchestration Result

- Add `proof_orchestration.go` with one unexported result and one orchestration
  function.
- Default the result to fail closed.
- Keep PostgreSQL `trustProofResult` and MySQL/TiDB
  `builtinSemanticProofResult` private to their existing proof implementations.
- Add no interface, registry, factory, configuration, or public symbol.

## 4. Move Requirements Before Proof

- Keep extraction, resolver selection, metadata resolution, and Promotion
  Barriers in their current order.
- Build requirements immediately after barriers and function-reason setup.
- Invoke proof orchestration only after requirements are attached to the domain
  result.
- Preserve all existing proof invocation conditions and catalog operations.

## 5. Collapse Final State Computation

- Route ordinary PostgreSQL, exact `COUNT(1)`, MySQL/TiDB builtin, and
  no-effect applicability through the orchestration function.
- Remove proof-specific branches from `Service.Analyze`.
- Replace the proof-specific and variadic arguments of
  `reclassifyAfterResolution` with the common permission fact.
- Normalize reasons, reclassify, and recompute admission once each.
- Leave final sorting, output declaration order, validation, and error wrapping
  unchanged.

## 6. Synchronize Documentation

- Update `internal/application/queryaccess/README.md` and changed Go L3 headers.
- Keep `CONTEXT.md` limited to the agreed domain terms.
- Add concise evidence-maintenance links to prior Query Access decisions only
  where they describe the old three-clock orchestration as current behavior.
- Keep this ADR Proposed until a fixed candidate passes review.

## 7. Verify Equivalence

- Run focused application tests for proof orchestration, trusted PostgreSQL,
  exact `COUNT(1)`, builtin semantics, barriers, and result validation.
- Run default and PostgreSQL-tagged full tests and affected race tests.
- Run Query Access corpus, PostgreSQL unit/confidence gates, builds, vet, lint,
  and relevant recording/live tests for all supported profiles.
- Run decision-record, gofmt, three-level-doc, module-tidy, and diff checks.
- Confirm no public SDK/CLI/HTTP/MCP production, fixture, dependency, workflow,
  version, or release file changed.

## 8. Independent Review and Acceptance

- Freeze a full candidate SHA and review range.
- Request fresh read-only Standards and Spec/security reviews.
- Treat any result drift, probe drift, barrier bypass, reason over-removal,
  proof vacuity, duplicated state clock, or new extensibility framework as
  blocking.
- Fix every P1/P2, rerun affected and full gates, and repeat review.
- Only after no P0/P1/P2 remains, change the ADR from Proposed to Accepted in a
  focused final commit citing the fixed reviewed candidate and range.

## 9. Delivery Closure

- Fast-forward local `main` only with human authorization and rerun required
  gates on the merged SHA.
- Push only with separate authorization and verify exact-SHA CI.
- Do not tag, release, publish, force-push, or delete branches/worktrees unless
  separately authorized.

## Suggested Commits

1. `docs(queryaccess): propose unified proof orchestration`
2. `test(queryaccess): characterize proof orchestration`
3. `refactor(queryaccess): consolidate proof promotion pipeline`
4. `docs(queryaccess): accept unified proof orchestration`
