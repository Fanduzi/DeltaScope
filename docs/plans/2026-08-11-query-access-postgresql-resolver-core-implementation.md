# Implementation Plan: PostgreSQL Query Access Resolver Core Deduplication

## Status

Proposed implementation plan. It authorizes no code change by itself. The
milestone remains PostgreSQL-only and behavior-preserving.

## 1. Establish the Baseline

- Start implementation from the reviewed documentation branch and record the
  exact base SHA.
- Use CodeGraph impact analysis before changing resolver symbols and record the
  callers of both constructors and both `ResolveRelation` methods.
- Run the existing PostgreSQL resolver unit and integration tests before edits.
- Capture current query order, returned `RelationSchema` values, and exact
  errors for base tables, partitioned tables, views, materialized views,
  missing relations, foreign tables, cancellation, query failures, scan
  failures, and iteration failures.
- Characterize conn state ordering explicitly: nil constructor input, context
  cancellation on a nil receiver, nil receiver/field with a live context, and
  a non-nil but already-closed `*sql.Conn` that reaches the query path.
- Confirm that the production trusted session path constructs
  `QueryAccessConnResolver` from the same caller-owned `*sql.Conn` as the
  identity adapter, and that no production trusted path constructs the
  DB-backed resolver.

## 2. Add the Shared Behavior Contract

- Refactor or add tests so one parameterized contract runs the same relation
  cases against DB-backed and conn-backed adapter factories.
- Keep adapter-only tests for DB compatibility; conn nil-constructor, nil
  receiver/field, already-closed connection, and cancellation precedence; the
  conn adapter's exact unexported `conn *sql.Conn` field and lack of a
  `*sql.DB` or interface-backed handle; caller ownership; and
  same-backend-session proof.
- Ensure the contract fails if either adapter changes query order, relation
  mapping, foreign-table rejection timing, error text, or returned columns.
- Do not change an expected result to make the extraction pass. Any newly
  discovered historical defect belongs in issue #2 or another focused issue.

## 3. Extract the Private Core

- Add a PostgreSQL-tagged `query_access_resolver_core.go` with a private minimal
  query capability implemented by `*sql.DB` and `*sql.Conn`.
- Move the duplicated relation-kind SQL, column SQL, scanning, and lookup error
  construction into stateless private functions. Relocate the already-shared
  unsupported-relkind guard and kind mapping from the DB adapter file into the
  same neutral core.
- Preserve SQL text, placeholders, ordering, wrapping, and error messages.
- Do not store a query capability in a shared resolver struct, export the
  capability, add caching, or add trust/admission logic.

## 4. Thin the DB-backed Adapter

- Keep `QueryAccessResolver`, its `db *sql.DB` field,
  `NewQueryAccessResolver`, and the current `ResolveRelation` signature.
- Preserve its existing context, constructor, nil, and lifecycle behavior.
- Delegate only the common catalog algorithm to the private core.
- Remove duplicated SQL and scanning behavior from the adapter file after the
  shared contract passes; move the existing shared mapping helpers without
  changing them.

## 5. Thin the Conn-backed Adapter

- Keep `QueryAccessConnResolver`, its `conn *sql.Conn` field,
  `NewQueryAccessConnResolver`, and the current `ResolveRelation` signature.
- Preserve nil-constructor rejection, context-first ordering, nil
  receiver/field handling, delegation for a non-nil but already-closed
  connection, caller ownership, and the absence of pool fallback.
- Delegate only the common catalog algorithm to the private core.
- Confirm the trusted SDK assembly code and build-tag stubs require no behavior
  change.

## 6. Close Documentation and Structural Checks

- Update L3 headers for every changed Go file and synchronize
  `internal/infrastructure/metadata/postgresql/README.md`.
- Run the staged three-level documentation checker.
- Add a structural source check or equivalent review evidence showing the two
  adapter files no longer own duplicate catalog SQL or scanning behavior and
  that the mapping and foreign-table helpers now live in the neutral core.
- Confirm `git diff --name-only main...HEAD` contains no MySQL/TiDB, parser,
  application admission, transport, public SDK, release, or output-contract
  changes.
- Keep the ADR Proposed until all implementation and review evidence is
  complete.

## 7. Verify Behavior and Session Safety

- Run focused PostgreSQL resolver tests during extraction.
- Run full default and PostgreSQL-tagged Go test suites.
- Run affected race tests, default and PostgreSQL builds, vet, and lint.
- Run `make pg-unit-test-gates` and Docker-backed `make pg-confidence-gates`.
- Explicitly run or identify the tests that prove real PG17 same-backend-PID
  behavior and foreign-table rejection before column and trusted `COUNT(1)`
  probes.
- Run the decision-record, release-gofmt, module-tidy, and `git diff --check`
  gates.
- Run CodeGraph change detection on the final diff and explain any reported
  risk instead of treating a green test count as a substitute for scope review.

## 8. Review and Decide the ADR

- Obtain an independent read-only review of the complete milestone diff.
- Require the reviewer to verify the two concrete ownership types, absence of
  pool fallback, single implementation of catalog behavior, zero observable
  behavior change, and no cross-dialect or public-surface expansion.
- If any P0, P1, or P2 finding remains, keep the ADR Proposed and iterate.
- After all required evidence passes, update only the ADR evidence and status
  to Accepted in a focused commit.
- If the extraction requires behavior changes, stop this milestone and move
  those changes to issue #2 or a new decision record rather than weakening the
  zero-change contract.

## Suggested Commit Boundaries

1. `docs(queryaccess): propose PostgreSQL resolver core deduplication`
2. `test(queryaccess): unify PostgreSQL resolver behavior contract`
3. `refactor(queryaccess): share PostgreSQL relation resolver core`
4. `docs(queryaccess): accept PostgreSQL resolver core boundary`

The final commit is conditional on completed verification and independent
review. No merge, push, tag, or release is implied by this plan.
