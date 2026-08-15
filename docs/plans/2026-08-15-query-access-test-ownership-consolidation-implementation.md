# Implementation Plan: Consolidate Query Access Test Ownership

## Status

Proposed implementation plan. It authorizes no deletion, merge, push, issue
closure, release, or gate change by itself.

## 1. Establish Baseline

- Start one milestone branch/worktree from current `origin/main` and record the
  full base SHA, worktree state, and Issue #4 state.
- Use CodeGraph before edits and reconcile it with exact source search because
  isolated-worktree indexes may lag.
- Inventory every Query Access SDK, deprecated API, CLI, HTTP, MCP, integration,
  real-binary, recording, no-leak, and corpus test.
- Record baseline file/line counts, named/subtest counts, Docker invocations,
  and wall-clock time for required gates. Set no reduction target.
- Run the existing full default/tagged, race, corpus, Docker, TLS, lint, vet,
  build, documentation, and module gates before deletion.

## 2. Commit the Ownership Ledger First

- Add the complete deletion ledger to this implementation document or a
  dedicated section of the spec; do not add a fifth permanent policy document.
- For each candidate test/subtest/table row, link the retained unified SDK
  semantic test and any required transport/compatibility owner.
- Mark PostgreSQL foreign-table, offline/default, no-leak sink, authorization-
  before-dial, and old per-target compatibility rows as non-substitutable.
- Update `docs/dev/testing.md` with the durable ownership and future-change
  rules.
- Obtain a read-only ledger review before deleting tests.

## 3. Make Unified SDK Evidence Complete

- Fill only gaps identified by the ledger; do not copy transport helpers or add
  a generated/shared matrix package.
- Ensure the unified SDK covers all MySQL 5.7/8.0/8.4, TiDB 8.5, and PostgreSQL
  17 semantic classes and complete recording-driver behavior.
- Prefer moving an existing table row to the unified owner over writing another
  equivalent test.
- Run focused SDK default/tagged, recording, race, and live Docker tests before
  deleting the corresponding duplicates.

## 4. Reduce Deprecated API Tests

- Keep declarations/deprecation, tagged/untagged stub, exact error and
  validation-priority, caller-owned connection, and no-leak contracts.
- Keep one unified-versus-old live or recording equivalence case for each of
  MySQL 5.7, 8.0, 8.4, TiDB 8.5, and PostgreSQL 17.
- Remove only redundant per-target shape tables after their unified owners pass.
- Preserve ADR-cited file paths where they retain a responsibility; otherwise
  add a follow-up evidence note before deleting the file.
- Commit this phase separately and rerun default/tagged compatibility and live
  equivalence gates.

## 5. Reduce CLI Tests

- Keep CLI-owned flags, TLS/session construction, close, cancellation,
  connection/catalog failures, bounded stderr, exit mapping, offline/default,
  and no-leak evidence.
- Keep real online MySQL 8.4, TiDB 8.5, and PG17 smoke with one admissible and
  one fail-closed result per family.
- Keep PG17 syntax-envelope, foreign-table, and offline/default negatives.
- Reduce recording coverage to one adapter no-execution/lifecycle/error test;
  remove duplicate detailed shape/probe rows mapped in the ledger.
- Preserve cited paths or update follow-up evidence notes as required.
- Commit and rerun CLI unit, recording, real-binary, MySQL/TiDB/PG Docker, TLS,
  lifecycle-regression, race, and no-leak gates.

## 6. Reduce HTTP Tests

- Keep HTTP-owned parsing, status/code/body, registry, `connection_id`, purpose,
  authorization, unauthorized/unknown zero-dial, close, cancellation,
  connection/catalog failures, request IDs, synchronized access logs,
  offline/default, and no-leak evidence.
- Keep real online MySQL 8.4, TiDB 8.5, and PG17 smoke with one admissible and
  one fail-closed result per family.
- Keep PG17 syntax-envelope, foreign-table, and offline/default negatives.
- Reduce recording coverage to one adapter no-execution/lifecycle/error test;
  remove duplicate detailed shape/probe rows mapped in the ledger.
- Preserve cited paths or update follow-up evidence notes as required.
- Commit and rerun HTTP unit, recording, MySQL/TiDB/PG Docker, TLS, race,
  authorization, access-log, and no-leak gates.

## 7. Run Temporary Mutation Probes

- Apply one mutation at a time for the five classes in the design.
- Run only the expected owning retained test and capture the RED test name and
  failure.
- Restore the mutation before proceeding; verify the mutation leaves no staged
  or unstaged diff.
- If a mutation does not fail, restore it, treat the ledger row as unsupported,
  and add or retain the missing test before continuing.
- Commit no mutation code, script, dependency, or generated report. Summarize
  commands and outcomes in ADR acceptance evidence and the final report.

## 8. Reconcile Documentation and Full Gates

- Update L3 headers and affected L2 READMEs for every changed test file; run the
  staged three-level documentation checker.
- Add follow-up notes to older Accepted ADRs only when a cited evidence file is
  removed. Preserve their original decision and acceptance history.
- Run default and PostgreSQL-tagged full tests and affected race tests; Query
  Access corpus; PostgreSQL unit/confidence; MySQL/TiDB and PostgreSQL
  CLI/HTTP/MCP E2E; CLI and HTTP TLS; build; vet; lint; npm contract; decision-
  record; gofmt; docs; module-tidy; and diff checks.
- Confirm production files, public API, Makefile, workflows, fixtures, versions,
  and release surfaces are unchanged.
- Record before/after counts and timings without claiming causality from noisy
  single-run timing.

## 9. Independent Review and ADR Decision

- Run independent Standards and Spec reviews against the fixed milestone base.
- Treat any orphaned semantic, compatibility, lifecycle, authorization,
  privacy, foreign-table, offline/default, or MCP-absence evidence as blocking.
- Keep the ADR Proposed while any P0, P1, or P2 remains.
- After all evidence passes, update only the ADR status and concise acceptance
  evidence in a focused commit.

## 10. Delivery Closure

- Fast-forward local `main` only after human approval and rerun required gates
  on the merged SHA.
- Push only with separate authorization and observe automatic CI without
  dispatch/rerun/cancel.
- Close Issue #4 only after merged SHA, remote CI, and independent verification
  agree.
- Do not tag, release, publish, or create an open-ended follow-up merely to hit
  a deletion target.

## Suggested Commit Boundaries

1. `docs(testing): define Query Access test ownership`
2. `test(queryaccess): consolidate unified and legacy SDK matrices`
3. `test(cli): reduce duplicated Query Access behavior cases`
4. `test(http): reduce duplicated Query Access behavior cases`
5. `docs(testing): accept Query Access evidence ownership`

Additional focused evidence-gap commits are allowed, but production changes are
not. Each phase must be green before the next owner deletes evidence.
