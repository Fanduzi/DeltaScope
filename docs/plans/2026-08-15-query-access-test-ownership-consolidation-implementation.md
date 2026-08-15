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

### Issue #10 Baseline

- Milestone worktree: `.worktrees/issue-10-query-access-test-ownership-ledger`
- Milestone branch: `feat/query-access-test-ownership-milestone-20260815`
- Exact base: `db4e73a19233d0475a480f2f333784d85f2d616a`
- Base parent: `bfbaff5b198b078415120e7f2a6e61236b770da4`
- Worktree state before this issue: clean. The root `main` worktree remained at
  `bfbaff5` with its pre-existing untracked observation-only WIP untouched.
- Issue #4 state: open, with the planned follow-up explicitly requiring a
  documented ownership matrix and coverage equivalence before deletion.

The exact-source inventory below was taken at the milestone base. `Named` counts
`func Test...` declarations; `subtest sites` counts `t.Run` call sites, not the
number of dynamic table cases.

| Evidence area | Files | Lines | Named | Subtest sites |
|---|---:|---:|---:|---:|
| Unified and deprecated public SDK | 17 | 5,881 | 103 | 37 |
| CLI adapter | 7 | 1,723 | 28 | 16 |
| HTTP adapter | 7 | 2,610 | 38 | 18 |
| MCP no-surface contract | 1 | 33 | 1 | 1 |
| CLI real binary | 1 | 295 | 5 | 1 |
| HTTP real binary | 1 | 249 | 5 | 1 |
| Application semantics and corpus | 36 | 14,005 | 358 | 183 |
| Domain semantics | 2 | 1,021 | 34 | 22 |
| Metadata resolver | 4 | 838 | 15 | 7 |
| Parser semantics | 15 | 5,303 | 132 | 40 |
| **Total** | **91** | **31,958** | **719** | **326** |

The corpus inventory is 208 files and 2,568 lines under `testdata/query-access`
(104 SQL/expected pairs). The Docker baseline is the existing command set, with
no new fixture or invocation: the unified SDK uses
`docker/query-access-builtin-compose.yaml`; CLI, HTTP, and MCP metadata gates
use their existing MySQL/TiDB and PostgreSQL compose scripts; CLI and HTTP TLS
use their existing TLS scripts. Gate wall-clock measurements are recorded below
when the unchanged baseline gates are run. There is no line, case, Docker, or
runtime reduction target.

### Baseline Gate Measurements

Measured once in this isolated worktree; timings are descriptive only.

| Gate | Result | Wall clock |
|---|---|---:|
| `make test` | pass | 18s |
| `CGO_ENABLED=1 go test -tags postgresql ./...` | pass | 20s |
| affected default and tagged race tests | pass | 7s / 11s |
| `make query-access-corpus-gates` | pass | 2s |
| `make pg-unit-test-gates` | pass | 4s |
| `make test-e2e-cli` | pass | 35s |
| MySQL/TiDB MCP metadata E2E | pass | 29s |
| MySQL/TiDB HTTP metadata E2E | pass | 39s |
| `make pg-confidence-gates` | pass | 71s |
| `make test-e2e-cli-tls` | pass | 17s |
| `make test-e2e-http-tls` | pass | 28s |
| `make test-e2e-cli-tls-regression` | pass | 34s |
| `make build` | pass | 8s |
| default and tagged `go vet` | pass | 1s / <1s |
| `make lint`, npm launcher, docs, decision-record, tidy, three-level-doc, and diff checks | pass | 2s / 1s / <1s / 1s / <1s / <1s / <1s |

The first HTTP TLS attempt stopped while Docker used a stale local amd64
`mysql:8.4` image on an arm64 host. Pulling the tag explicitly for `linux/arm64`
changed only the local Docker cache; the unchanged gate then passed. Docker gate
invocations were exactly `make test-e2e-cli`, both MySQL/TiDB MCP and HTTP
metadata targets, `make pg-confidence-gates`, and the CLI/HTTP TLS targets plus
its CLI lifecycle regression. No Docker target, fixture, script, or Makefile
changed.

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

### Ledger Keys

| Key | Retained evidence |
|---|---|
| S1 | `pkg/deltascope/query_access_online_session_test.go`: unified MySQL/TiDB construction, validation, per-target legacy equivalence, no-execution, and no-leak tests |
| S2 | `pkg/deltascope/query_access_session_mysql_tidb_live_e2e_test.go`: legacy live profile matrix and six-probe per-target unified-versus-legacy equivalence; it is not an exhaustive unified owner |
| S3 | `pkg/deltascope/query_access_online_session_postgresql_tag_test.go`: tagged PG17 route, excluded-shape, foreign-table, failure, ownership, and no-execution tests |
| S4 | `pkg/deltascope/query_access_online_session_postgresql_integration_test.go`: live PG17 admissible, excluded-shape, foreign-table, same-session, and legacy equivalence tests |
| S5 | `pkg/deltascope/query_access_session_postgresql_recording_test.go`: complete PG17 identity/catalog probes, no-execution, foreign-table, and bounded-failure tests |
| S6 | `internal/application/queryaccess/{corpus_test.go,corpus_pg_test.go}` and `testdata/query-access`: offline semantic corpus |
| U1 | Missing at the Issue #10 baseline: a complete unified MySQL/TiDB live semantic matrix for every legacy profile/shape row. Issue #11 must establish it before any row currently mapped to U1 can be deleted. |
| U2 | Missing at the Issue #10 baseline: complete unified PG17 evidence for legacy aggregate/comparison shapes beyond the current exact `COUNT(1)` envelope. Issue #11 must establish it before those legacy rows can be deleted. |
| C1 | `pkg/deltascope/query_access_deprecation_test.go`, `query_access_online_session*_test.go`, and the legacy session tests: deprecated API source, stub, exact-error, validation-order, and caller-ownership contract |
| C2 | Unified-versus-legacy equivalence in S1/S2 for MySQL 5.7/8.0/8.4 and TiDB 8.5, and S3/S4 for PostgreSQL 17 |
| T1 | CLI adapter and real-binary tests: flags, TLS/session construction, exit code, streams, close, bounded failures, no-leak, and real routing |
| T2 | HTTP adapter and real-binary tests: request/status/body, registry, `connection_id`, authorization-before-dial, close, access log, bounded failures, no-leak, and real routing |
| M1 | `internal/interfaces/mcp/query_access_surface_contract_test.go`: no Query Access MCP tool |

### Row-by-Row Deletion Ledger

All rows are **retain** during Issue #10. `Candidate after owner gate` means a
later issue may remove only the named semantic rows after the listed replacement
evidence and its focused gates pass. `Non-substitutable` means the row remains
at its observed boundary even if its SQL text also appears elsewhere.

| Current row or table | Current behavior | Unified semantic evidence | Boundary / compatibility evidence | Status |
|---|---|---|---|---|
| `query_access_session_mysql_tidb_test.go`: `PromotesProvenMySQL84CountStar` | MySQL 8.4 aggregate promotion | U1 (missing) | C1, C2 | Blocked: retain until Issue #11 |
| `query_access_session_mysql_tidb_test.go`: `PromotesLiteralAndReversedOperands` | MySQL/TiDB literal and reversed operands | U1 (missing) | C1, C2 | Blocked: retain until Issue #11 |
| `query_access_session_mysql_tidb_test.go`: `RemainsFailClosedForUnknownFunction` | unknown-function fail-closed | U1 (missing) | C1, C2 | Blocked: retain until Issue #11 |
| `query_access_session_mysql_tidb_test.go`: `PromotesRelationlessLiteralShapes` | relationless literal promotion | U1 (missing) | C1, C2 | Blocked: retain until Issue #11 |
| `query_access_session_mysql_tidb_test.go`: `DoesNotExecuteUserSQL` | legacy no execution | S1 | C1, C2 | Non-substitutable compatibility |
| `query_access_session_mysql_tidb_live_e2e_test.go`: `TestLiveProfile_AssertsVersionAndAdmitsAggregates` table rows | MySQL 5.7/8.0/8.4 and TiDB 8.5 live shape/profile matrix | U1 (missing) | C2 | Blocked: retain until Issue #11 |
| `query_access_session_mysql_tidb_live_e2e_test.go`: `TestLiveUnifiedSession_MatchesDialectSpecificForAllProfiles` table rows | per-target routing equivalence | S2 | C2 | Non-substitutable compatibility |
| `query_access_session_integration_test.go`: `TestTrustedSDK_CountStarAdmissible` through `TestTrustedSDK_ComparisonAdmissible` | legacy PG17 admitted aggregate and comparison shapes | U2 (missing) | C1, C2 | Blocked: retain until Issue #11 |
| `query_access_session_integration_test.go`: `TestTrustedSDK_CountIntegerOneExcludedShapesRemainIndeterminate`, `FilterDistinctRemainIndeterminate`, `UnqualifiedRelationIndeterminate`, and `LiteralComparisonIndeterminate` | legacy PG17 excluded-shape boundaries | U2 (missing) | C1, C2 | Blocked: retain until Issue #11 |
| `query_access_session_integration_test.go`: session creation, closed/cancelled input, close, resolver, dialect, nil, and same-connection rows | old session lifecycle and validation contract | S3, S4 | C1 | Non-substitutable compatibility |
| `query_access_session_postgresql_recording_test.go`: identity/catalog probe rows | detailed PG17 probe sequence | S5 | C1 | Candidate after owner gate; preserve one legacy compatibility proof |
| `query_access_session_postgresql_recording_test.go`: foreign-table and catalog-failure rows | relation-kind trust and bounded failure/no-leak | S5 | C1 | Non-substitutable trust/compatibility |
| `query_access_online_session_test.go`: per-target rows in `MatchesDialectSpecificMySQLTiDB` and `NoExecutionNoLeakMatchesDialectSpecificMySQLTiDB` | unified semantic and equivalence matrices | S1 | C2 | Retained unified owner |
| `query_access_online_session_postgresql_tag_test.go`: route/excluded-shape rows | tagged PG17 semantic matrix | S3 | C2 | Retained unified owner |
| `query_access_online_session_postgresql_tag_test.go`: foreign-table, lookup failure, ownership, validation, constructor, and no-execution rows | PG17 trust, lifecycle, and bounded failure | S3 | C1, C2 | Retained unified owner |
| `query_access_online_session_postgresql_integration_test.go`: all five rows | live PG17 route, foreign-table, same-session, and equivalence | S4 | C2 | Retained unified owner |
| `query_access_online_session_postgresql_notag_test.go` | no-tag capability and legacy stub contract | S1 | C1 | Non-substitutable compatibility |
| `query_access_deprecation_test.go` | six deprecation notices | -- | C1 | Non-substitutable compatibility |
| `query_access_e2e_mixed_literal_test.go` CLI profile table rows | repeated MySQL/TiDB product/profile/shape results | U1 (missing) | T1 real-route smoke per family | Blocked: retain until Issue #11 |
| `query_access_e2e_mixed_literal_test.go` CLI offline row | CLI default/offline behavior | S1, S6 | T1 | Non-substitutable transport |
| `query_access_test.go` CLI admitted/rejected/mode/JSON result rows | CLI command serialization and exit mapping | S1, S6 | T1 | Non-substitutable transport |
| `query_access_test.go` CLI flags, removed-password, TLS, and constructor rows | CLI input/session/error contract | -- | T1 | Non-substitutable transport |
| `query_access_postgresql_online_recording_test.go` detailed fixed-probe assertions | duplicated adapter probe sequence | S5 | T1 | Candidate after owner gate; retain one adapter no-execution/close/error test |
| `query_access_postgresql_online_recording_test.go` cancellation, closed-session, connection-failure, and catalog-failure rows | CLI lifecycle and bounded errors | S3, S5 | T1 | Non-substitutable transport |
| `query_access_postgresql_no_leak_test.go`: `CountIntegerOne`, `ExcludedShapes`, and `DefaultOffline` | CLI stdout/stderr sink privacy | S3 | T1 | Non-substitutable per-sink no-leak and offline |
| `query_access_probe_boundary_no_leak_test.go` CLI rows | CLI MySQL/TiDB identity-boundary privacy | S1 | T1 | Non-substitutable per-sink no-leak |
| `main_e2e_postgresql_query_access_test.go` CLI: `CountIntegerOne` | real CLI PG17 admissible route | S4 | T1 | Non-substitutable real-route smoke |
| `main_e2e_postgresql_query_access_test.go` CLI: syntax-envelope exclusions other than `app.remote_orders`, default, no-leak, and connection-failure rows | CLI syntax, offline, sink privacy, and bounded failure | S3, S4 | T1 | Non-substitutable transport |
| `main_e2e_postgresql_query_access_test.go` CLI: `SELECT COUNT(1) FROM app.remote_orders` | PostgreSQL foreign-table relation-kind trust | S3, S4 | T1 | Non-substitutable foreign-table evidence |
| `query_access_e2e_mixed_literal_test.go` HTTP profile table rows | repeated MySQL/TiDB product/profile/shape results | U1 (missing) | T2 real-route smoke per family | Blocked: retain until Issue #11 |
| `query_access_e2e_mixed_literal_test.go` HTTP default row | HTTP default/offline behavior | S1, S6 | T2 | Non-substitutable transport |
| `query_access_test.go` HTTP offline request/result rows | HTTP parsing, status, body, and defaults | S1, S6 | T2 | Non-substitutable transport |
| `query_access_test.go` HTTP `connection_id`, registry, authorization, and zero-open rows | authorization-before-dial and registry boundary | -- | T2 | Non-substitutable authorization-before-dial |
| `query_access_postgresql_online_recording_test.go` detailed fixed-probe assertions | duplicated adapter probe sequence | S5 | T2 | Candidate after owner gate; retain one adapter no-execution/close/error test |
| `query_access_postgresql_online_recording_test.go` catalog-failure row | HTTP bounded error and access-log behavior | S3, S5 | T2 | Non-substitutable transport |
| `query_access_postgresql_no_leak_test.go`: Count, excluded, no-connection, unauthorized, and failure rows | HTTP body/log sink privacy | S3 | T2 | Non-substitutable per-sink no-leak |
| `query_access_postgresql_no_leak_test.go`: unauthorized and unknown zero-dial rows | HTTP authorization-before-dial | -- | T2 | Non-substitutable authorization-before-dial |
| `query_access_probe_boundary_no_leak_test.go` HTTP rows | HTTP MySQL/TiDB identity-boundary privacy | S1 | T2 | Non-substitutable per-sink no-leak |
| `main_e2e_postgresql_query_access_test.go` HTTP: `CountIntegerOne` | real HTTP PG17 admissible route | S4 | T2 | Non-substitutable real-route smoke |
| `main_e2e_postgresql_query_access_test.go` HTTP: syntax-envelope exclusions other than `app.remote_orders`, no-connection, unauthorized, and no-leak rows | HTTP syntax, registry/auth, and body/log privacy | S3, S4 | T2 | Non-substitutable transport |
| `main_e2e_postgresql_query_access_test.go` HTTP: `SELECT COUNT(1) FROM app.remote_orders` | PostgreSQL foreign-table relation-kind trust | S3, S4 | T2 | Non-substitutable foreign-table evidence |
| `query_access_surface_contract_test.go` | MCP has no Query Access tool | -- | M1 | Non-substitutable MCP contract |
| `corpus_test.go`, `corpus_pg_test.go`, and all `testdata/query-access` pairs | offline corpus semantic classes | S6 | -- | Retained unified/offline semantic owner |

The ledger does not authorize a deletion by SQL-text similarity. A later change
must name the exact test or table row, cite this table, preserve its listed
non-substitutable boundary where applicable, and add a row before deleting an
unlisted candidate.

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
