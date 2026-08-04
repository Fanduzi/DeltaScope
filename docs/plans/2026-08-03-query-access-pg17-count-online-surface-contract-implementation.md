# Implementation Plan: PG17 `COUNT(1)` Online Surface Contract

## Status

Proposed. This plan authorizes no behavior change until the implementation
evidence establishes that the existing transport delegation is safe.

## 1. Establish Current Transport Behavior

- Start one milestone branch from current `main`.
- Record CLI and HTTP outcomes for the exact query, default/offline requests,
  and existing `COUNT(*)`/`COUNT(column)` regression cases.
- Trace the two transport paths to prove they hand one pinned PostgreSQL
  connection to the existing session API; do not assume this from older docs.
- If build-tag or ownership differences prevent a common proof, stop and leave
  both public surfaces out of scope rather than creating a weaker adapter.

## 2. Create Reproducible PG17 Fixtures

- Start a task-owned PostgreSQL 17 fixture from committed compose and init
  sources, wait for the intended database and sentinel relation, and record
  the image/version evidence.
- Ensure cleanup removes only task-owned containers, network, volumes, and
  temporary credentials. Do not reuse or remove another task's PG fixture.
- Repair fixture defects in a focused commit only if they block the committed
  E2E configuration; add a regression that would have detected the defect.

## 3. Prove CLI Online Parity

- Add a Docker-backed CLI test that invokes the actual command with the
  supported PostgreSQL connection flags and proves the exact positive result
  and table-only requirement.
- Cover excluded shapes, unsupported version/identity handling where
  controllable, and the no-connection offline regression.
- Assert bounded stderr/stdout and no marker, credential, endpoint, catalog,
  or raw driver disclosure. Do not add a flag or CLI-only proof path.

## 4. Prove HTTP Online Parity

- Add an HTTP integration test that provisions a PostgreSQL connection through
  runtime configuration and invokes `POST /v1/query-access/analyze` with only
  its allowed `connection_id`.
- Prove the positive result, exclusions, no-connection default, direct-input
  rejection, profile-plus-connection rejection, and authorization behavior.
- Capture the configured access logger with a synchronization-safe sink. Assert
  a request entry exists before checking it for markers; verify response and
  log disclosure boundaries independently.

## 5. Prove Shared and Adapter Safety Properties

- Keep the shared-session recording-driver proof, but do not use it as a
  substitute for adapter-level evidence. Add one observable transport-level
  test seam for CLI online and one for HTTP `connection_id`; the seam may be a
  test-only injected opener/dialer, recording driver, or controlled proxy
  selected by the implementation. It must observe database operations before
  and after the shared session boundary without requiring a new production API
  contract.
- For each successful online path, observe at least one expected fixed
  identity/catalog probe, then prove that the submitted SQL's unique marker,
  `EXPLAIN`, and prepare operations never reach the driver or proxy. The fixed
  probe makes the no-execution assertion non-vacuous.
- For HTTP rejected or unauthorized `connection_id` paths, assert zero
  dial/open-session operations and no leakage of connection configuration or
  credentials.
- Treat these CLI and HTTP adapter-level proofs, together with the shared
  proof, as mandatory evidence before changing the ADR from Proposed to
  Accepted.
- Retain dedicated `COUNT(1)` catalog checks and generic `COUNT(column)`
  regression coverage. Do not add `AggregateClass` back to the shared
  `count(anyelement)` manifest entry.
- Add no-leak tests for normal and reachable failure paths, with unique marker
  values and a bounded public error taxonomy.

## 6. Verify and Review

- Run focused PostgreSQL-tagged CLI, HTTP, SDK, parser, application, and
  metadata tests; then full default and PostgreSQL-tagged suites, affected race
  tests, builds, vet, lint, corpus, PG gates, TLS E2E, decision-record,
  gofmt, module-tidy, and diff checks.
- Run GitNexus impact analysis before each production-symbol edit and
  `gitnexus_detect_changes` before every commit.
- Obtain an independent read-only review of the final `main...HEAD` diff.
  It must confirm transport parity uses the shared proof, default paths remain
  fail-closed, user SQL is never executed, no-leak evidence is non-vacuous, and
  no P0/P1/P2 findings remain.

## 7. Publish the Decision Only After Evidence

- Keep the successor ADR Proposed until all required transport evidence and
  review pass.
- Then update the existing PG17 `COUNT(1)` ADR and the successor ADR with the
  exact supported surface matrix and test references.
- If either online transport cannot meet the contract, document it as deferred;
  do not claim parity or widen the SDK-only decision.
