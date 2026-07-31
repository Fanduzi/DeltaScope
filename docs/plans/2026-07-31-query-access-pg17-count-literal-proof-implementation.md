# Implementation Plan: PG17 Query Access `COUNT(1)` Proof

## Status

Proposed implementation plan. No implementation work is authorized by this
document alone.

## 1. Establish the Evidence Baseline

- Start a milestone branch from the current `main`.
- Record the current outcomes for admitted `COUNT(*)` and `COUNT(column)`, and
  indeterminate outcomes for every excluded `COUNT(1)` variant.
- Add parser characterization tests only after confirming the PostgreSQL
  extractor exposes complete candidate facts for one literal operand.
- Stop and document a defer decision if the extractor cannot distinguish the
  exact safe shape from casts, parameters, modifiers, or expressions.

## 2. Map the Session-Bound Catalog Proof

- Trace the existing PG17 session constructor, metadata resolver, and pure
  effect proof to identify how aggregate identity and argument types are
  represented.
- Add a minimal, internal proof entry for exactly `COUNT(1)` only if the
  connected catalog provides sufficient identity evidence.
- Do not add a hard-coded OID-only allowlist, a name-only allowlist, or any
  fallback that silently treats a literal as a column.

## 3. Preserve Strict Requirements

- Keep the existing physical relation completeness predicate unchanged.
- Require a schema-qualified physical base relation and assert the output is
  exactly one `read_table` requirement for the minimal positive fixture.
- Add direct regressions for relationless, view, CTE, derived, wildcard,
  unqualified, ambiguous, and unresolved sources.

## 4. Implement the Narrow Candidate Gate

- Match aggregate name, class, arity, and one `const` operand at the exact
  proof seam identified in step 2.
- Reject DISTINCT, FILTER, aggregate ordering, windows, casts, parameters,
  nested calls, operators, malformed candidate vectors, and all non-`COUNT`
  functions.
- Preserve existing outcomes for `COUNT(*)` and `COUNT(column)`.

## 5. Test Public and Security Boundaries

- Add corpus and domain/application tests for the positive shape and every
  exclusion category.
- Add SDK caller-owned PG17 Docker integration with exact requirements,
  server/version identity mismatch, and no-execution recording-driver tests.
- Assert marker literals are absent from SDK struct formatting and JSON, plus
  all error paths reached by the new proof.
- Do not add CLI, HTTP, or MCP tests as positive surfaces: those surfaces are
  explicitly out of scope. Add regressions proving they remain indeterminate.

## 6. Verify and Review

- Run focused PostgreSQL-tagged tests while iterating, then the full default
  and `-tags postgresql` suites, race tests for affected packages, builds, vet,
  corpus gates, Docker PG17 evidence, formatting, `go mod tidy`, and
  `git diff --check`.
- Run GitNexus impact analysis before changing each existing production symbol
  and `gitnexus_detect_changes` before committing.
- Obtain an independent read-only security review. It must confirm catalog
  binding, no global Phase-1 widening, no user-SQL execution, no literal or
  connection leaks, and no unintended public-surface expansion.

## 7. Document the Outcome

- Keep the ADR Proposed until the evidence and independent review pass.
- If all conditions pass, update it to Accepted with exact test references and
  an explicit list of still-deferred PostgreSQL shapes.
- If catalog evidence is insufficient, record the failure as a deliberate
  defer; do not ship a weaker approximation.
