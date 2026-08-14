# Implementation Plan: Unified Online Query Access Analysis Entry

## Status

Proposed implementation plan. It authorizes no code change by itself. The
milestone is additive and behavior-preserving.

## 1. Establish Baseline and Impact

- Start from the reviewed documentation branch and record the exact base SHA.
- Use CodeGraph before edits when the repository index is available. Record all
  callers of the four existing dialect-specific session functions and both
  transport product switches.
- Run focused existing SDK, CLI, and HTTP online Query Access tests in default,
  PostgreSQL-tagged, integration, and recording-driver configurations.
- Characterize exact old API validation order, sentinel errors, wrapping,
  connection ownership, no-execution probes, and no-leak output.
- Record official build evidence (`make build` and GoReleaser use PostgreSQL
  tags) separately from no-tag source build compatibility.

## 2. Add RED Public Contract Tests

- Add compile-time signature tests for `OnlineQueryAccessSession`, its
  constructor, its analysis function, and the five generic sentinel errors.
- Add reflection and JSON tests proving no exported field, getter, identity,
  product, profile, capability, connection, or marshalable state.
- Add constructor tests for nil context, nil connection, failed ping, failed
  identity, unsupported product/version, and caller connection ownership.
- Add table-driven combined-invalid-input tests that enforce the exact analysis
  validation priority from the spec.
- Add default-build tests proving unified symbols compile and an observed
  PostgreSQL target fails with
  `ErrOnlineQueryAccessCapabilityUnsupported`, without changing old PostgreSQL
  stub errors.
- Confirm all new tests fail for the intended missing behavior before production
  edits.

## 3. Introduce the Unified Opaque Session

- Add the untagged opaque session type and generic sentinel errors in
  `pkg/deltascope`.
- Implement construction from a caller-owned `*sql.Conn`: validate input, ping,
  identify on that connection, derive a private route, and retain no public
  identity state.
- Keep the connection caller-owned; do not add `Close`, pooling, retry, cache,
  finalizer, or concurrency claims.
- Add small private tagged/untagged PostgreSQL capability helpers so both source
  builds expose the same public symbols while official builds retain PG17
  support.
- Ensure all newly returned online errors are bounded sentinels and raw internal
  failures are not wrapped into public text.

## 4. Extract Private Execution Helpers Safely

- Characterize the old PostgreSQL and MySQL/TiDB public APIs before extraction.
- Extract the minimum private helpers needed to assemble and execute each
  existing proof path from a pinned connection and identity-derived target.
- Preserve PostgreSQL same-connection identity/schema/function proof, manifest
  and trust policy, `COUNT(1)` envelope, and foreign-table fail-closed behavior.
- Preserve MySQL/TiDB identity-derived profile mapping, builtin semantic proof,
  resolver behavior, and relationless/literal boundaries.
- Keep old public functions' validation and error translation in place. Do not
  route them blindly through the generic API.

## 5. Implement Generic Analysis Routing

- Apply the fixed validation order: session, dialect constraint, profile,
  external resolver, linked capability, existing request validation.
- When request dialect is empty, derive the application dialect privately from
  observed identity. When non-empty, compare and fail on mismatch before proof.
- Route only the currently supported targets to the existing private proof
  helpers. Return generic capability unsupported for all other trustworthy
  targets.
- Preserve `QueryAccessResult`, existing parser/application errors, no user SQL
  execution, and all fail-closed outcomes.

## 6. Prove Unified SDK Equivalence

- Run a shared matrix across MySQL 5.7/8.0/8.4, TiDB 8.5, and PostgreSQL 17,
  comparing the unified and existing dialect-specific API results.
- Cover representative admitted, rejected, indeterminate, invalid mode,
  dialect mismatch, profile/resolver rejection, context cancellation, closed
  connection, catalog failure, and unsupported capability cases.
- Retain and extend recording-driver tests proving the marker SQL is never
  executed and identity/catalog probes remain bounded.
- Run Docker-backed caller-owned session tests for every supported target and
  PostgreSQL same-backend-session/foreign-table cases.
- Do not delete or weaken existing dialect-specific tests.

## 7. Migrate CLI Without External Change

- Replace the product switch in `runQueryAccessOnline` with the unified
  constructor and analysis function.
- Keep CLI flag parsing, online session configuration/open/close, dialect
  expectations at the connection layer, exit codes, stdout/stderr categories,
  and bounded error presentation unchanged. Map unified constructor/capability
  failures to the existing bounded connection-failure category rather than
  exposing dialect-specific SDK identity or liveness strings.
- Leave request dialect empty at the unified analysis boundary unless an
  existing externally visible contract requires a constraint; do not derive it
  by inspecting the unified wrapper.
- Run recording, real-binary, Docker, timeout/cancellation, connection-failure,
  excluded-shape, no-execution, and no-leak tests. Keep the existing behavior
  matrix intact.

## 8. Migrate HTTP Without External Change

- Replace the product switch in the Query Access handler with the unified
  constructor and analysis function.
- Preserve runtime registry lookup, `connection_id` authorization and purpose,
  session close, status/error mappings, request IDs, access-log synchronization,
  and bounded messages.
- Prove unauthorized/no-connection paths still perform zero session opens and
  zero database dials.
- Run recording and Docker-backed tests for success, excluded shapes,
  constructor/catalog failure, timeout/cancellation, no-execution, and no-leak.
  Keep the existing behavior matrix intact.

## 9. Protect Old APIs and MCP Absence

- Run exact compatibility tests for existing PostgreSQL and MySQL/TiDB public
  constructors, analysis functions, stubs, errors, and validation ordering.
- Add structural checks that CLI and HTTP no longer switch on server product for
  online Query Access.
- Keep the MCP no-Query-Access surface contract test unchanged and passing.
- Do not add deprecation annotations; issue #3 owns that decision.
- Do not consolidate tests; issue #4 owns the later ownership matrix and
  deletion review.

## 10. Close Documentation and Verification

- Update L3 headers for every changed Go file and synchronize affected L2
  module READMEs. Run the staged three-level documentation checker.
- Update `pkg/deltascope` public documentation with the additive API, caller
  ownership, observed identity, empty/matching dialect semantics, official
  distribution support, and no-tag compatibility behavior.
- Run default and PostgreSQL-tagged full tests, affected race tests, builds,
  vets, lint, Query Access corpus gates, PostgreSQL unit/confidence gates,
  Docker-backed SDK/CLI/HTTP tests, MCP contract tests, formatting,
  decision-record, module-tidy, and diff checks.
- Run CodeGraph final change detection when available and review all changed
  public symbols and transport call paths.
- Obtain an independent read-only Standards and Spec review. Keep the ADR
  Proposed while any P0, P1, or P2 remains.

## 11. Decide the ADR

- After all acceptance evidence passes, update only the ADR's status and
  concise evidence in a focused commit.
- If implementation requires changing old errors, transport output, supported
  SQL shapes, authorization behavior, connection ownership, or test evidence,
  stop and revise the Proposed decision instead of broadening the milestone.
- Preserve issues #3 and #4 as deferred follow-ups; do not close them merely
  because this milestone lands.

## Suggested Commit Boundaries

1. `docs(queryaccess): propose unified online analysis entry`
2. `test(queryaccess): specify unified online session contract`
3. `refactor(queryaccess): share online proof execution cores`
4. `feat(queryaccess): add unified online session API`
5. `refactor(queryaccess): route CLI and HTTP through unified entry`
6. `test(queryaccess): prove online entry transport equivalence`
7. `docs(queryaccess): accept unified online analysis boundary`

The final commit is conditional on completed verification and independent
review. No merge, push, tag, release, or test deletion is implied.
