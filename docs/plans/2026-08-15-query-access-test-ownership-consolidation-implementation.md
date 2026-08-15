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

### Ownership-Focused File Manifest

This manifest separately inventories every file that can supply or lose an
ownership row. Each parenthesized tuple is `lines; named tests; subtest sites`;
the aggregate table above inventories the remaining application, domain,
metadata, and parser Query Access tests by directory.

| Category | Files |
|---|---|
| Unified/deprecated SDK | `query_access_deprecation_test.go` (74;1;1), `query_access_online_session_test.go` (1059;15;9), `query_access_online_session_postgresql_tag_test.go` (497;9;4), `query_access_online_session_postgresql_notag_test.go` (108;4;1), `query_access_online_session_postgresql_integration_test.go` (272;5;2), `query_access_session_mysql_tidb_test.go` (454;6;3), `query_access_session_mysql_tidb_live_e2e_test.go` (934;2;3), `query_access_session_integration_test.go` (809;22;1), `query_access_session_postgresql_recording_test.go` (340;3;0), `query_access_session_postgresql_test.go` (83;5;0), `query_access_session_test.go` (90;4;0), `query_access_test.go` (331;11;2), `query_access_profile_test.go` (196;8;3), `query_access_probe_boundary_no_leak_test.go` (316;2;2), `query_access_postgresql_tag_test.go` (31;1;0), `query_access_pure_effect_surface_contract_postgresql_test.go` (69;1;1), `query_access_unproven_reasons_postgresql_tag_test.go` (218;4;1) |
| CLI adapter, recording, and no-leak | `query_access_test.go` (333;14;1), `query_access_unified_entry_test.go` (71;1;0), `query_access_e2e_mixed_literal_test.go` (433;2;5), `query_access_postgresql_online_recording_test.go` (449;5;0), `query_access_postgresql_no_leak_test.go` (156;3;1), `query_access_probe_boundary_no_leak_test.go` (190;2;2), `query_access_unproven_reasons_postgresql_tag_test.go` (91;1;1) |
| HTTP adapter, recording, and no-leak | `query_access_test.go` (846;23;3), `query_access_unified_entry_test.go` (71;1;0), `query_access_e2e_mixed_literal_test.go` (671;1;5), `query_access_postgresql_online_recording_test.go` (394;2;0), `query_access_postgresql_no_leak_test.go` (294;7;1), `query_access_probe_boundary_no_leak_test.go` (236;3;2), `query_access_unproven_reasons_postgresql_tag_test.go` (98;1;1) |
| MCP | `internal/interfaces/mcp/query_access_surface_contract_test.go` (33;1;1) |
| Real binaries | `cmd/deltascope/main_e2e_postgresql_query_access_test.go` (295;5;1), `cmd/deltascope-server/main_e2e_postgresql_query_access_test.go` (249;5;1) |
| Corpus | `internal/application/queryaccess/corpus_test.go` (395;1;1), `corpus_pg_test.go` (29;1;1), `corpus_session_test.go` (135;0;0), and 104 SQL/expected pairs under `testdata/query-access` |

The remaining source-semantic inventory is 36 application files, 2 domain
files, 4 metadata-resolver files, and 15 parser files, as counted above. It is
not a deletion target in this milestone. Dynamic table expansion is recorded in
the ledger by its owning test and named table rows; the `subtest sites` count is
the reproducible static count of `t.Run` declarations rather than an unstable
runtime expansion count.

### Baseline Gate Measurements

Measured once in this isolated worktree; timings are descriptive only.

| Gate | Result | Wall clock |
|---|---|---:|
| `make test` | pass | 18s |
| `CGO_ENABLED=1 go test -tags postgresql ./...` | pass | 20s |
| `go test -race ./pkg/deltascope ./internal/interfaces/cli ./internal/interfaces/http ./internal/interfaces/mcp` | pass | 7s |
| `CGO_ENABLED=1 go test -race -tags postgresql ./pkg/deltascope ./internal/interfaces/cli ./internal/interfaces/http ./internal/interfaces/mcp` | pass | 11s |
| `make query-access-corpus-gates` | pass | 2s |
| `make pg-unit-test-gates` | pass | 4s |
| `go test -tags integration -count=1 ./pkg/deltascope -run 'TestLiveProfile_AssertsVersionAndAdmitsAggregates\|TestLiveUnifiedSession_MatchesDialectSpecificForAllProfiles'` | pass | 14s |
| `make test-e2e-cli` | pass | 35s |
| `make test-e2e-mcp-mysql test-e2e-mcp-tidb` | pass | 29s |
| `make test-e2e-http-mysql test-e2e-http-tidb` | pass | 39s |
| `make pg-confidence-gates` | pass | 71s |
| `make test-e2e-cli-tls` | pass | 17s |
| `make test-e2e-http-tls` | pass | 28s |
| `make test-e2e-cli-tls-regression` | pass | 34s |
| `make build` | pass | 8s |
| `go vet ./...` | pass | 1s |
| `CGO_ENABLED=1 go vet -tags postgresql ./...` | pass | <1s |
| `make lint` | pass | 2s |
| `npm test --prefix packages/deltascope-mcp` | pass | 1s |
| `make docs-example-gates` | pass | <1s |
| `make decision-record-gate` | pass | 1s |
| `go mod tidy` | pass | <1s |
| `/Users/fan/.agents/skills/check-three-level-doc/scripts/check_three_level_doc.sh` | pass | <1s |
| `git diff --check db4e73a19233d0475a480f2f333784d85f2d616a...HEAD` | pass | <1s |

The baseline performed 14 successful existing Docker stack lifecycles (28 compose
`up`/`down` operations): one SDK builtin fixture; two CLI metadata; two MCP
metadata; two HTTP metadata; three PostgreSQL confidence; one CLI TLS; one HTTP
TLS; and two CLI TLS regression. One additional HTTP TLS lifecycle failed before
readiness (two compose operations) because Docker used a stale local amd64
`mysql:8.4` image on an arm64 host. Pulling the tag explicitly for `linux/arm64`
changed only the local Docker cache; the unchanged retry passed. Total observed
Docker accounting was 15 lifecycles and 30 compose operations. No Docker target,
fixture, script, or Makefile changed.

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
| S1 | `pkg/deltascope/query_access_online_session_test.go`: unified MySQL/TiDB construction, validation, direct semantic/recording no-execution, and no-leak tests; per-target deprecated equivalence is owned only by S2. |
| S2 | `pkg/deltascope/query_access_session_mysql_tidb_live_e2e_test.go`: `TestLiveUnifiedSession_MatchesDialectSpecificForAllProfiles` is the live per-target unified-versus-legacy compatibility owner; it is not the exhaustive unified semantic owner. |
| S3 | `pkg/deltascope/query_access_online_session_postgresql_tag_test.go`: tagged PG17 route, excluded-shape, foreign-table, failure, ownership, and no-execution tests |
| S4 | `pkg/deltascope/query_access_online_session_postgresql_integration_test.go`: live PG17 admissible, excluded-shape, foreign-table, same-session, and legacy equivalence tests |
| S5 | `pkg/deltascope/query_access_session_postgresql_recording_test.go`: deprecated PG17 foreign-table and bounded-failure no-leak compatibility, plus the recording driver shared by unified tagged tests; it is not a unified semantic owner. |
| S6 | `internal/application/queryaccess/{corpus_test.go,corpus_pg_test.go}` and `testdata/query-access`: offline semantic corpus |
| U1 | `pkg/deltascope/query_access_online_session_test.go`: `TestOnlineQueryAccessSession_MySQLTiDBSemanticMatrix`, plus `query_access_session_mysql_tidb_live_e2e_test.go`: `TestLiveUnifiedSession_AssertsVersionAndSemanticMatrix`, own the complete unified MySQL 5.7/8.0/8.4 and TiDB 8.5 semantic matrix; focused default and Docker gates are recorded in Issue #11 evidence. |
| U2 | `pkg/deltascope/query_access_online_session_postgresql_integration_test.go`: `TestUnifiedSession_PostgreSQLCountOneAdmissible`, `TestUnifiedSession_PostgreSQLSemanticMatrix`, `TestUnifiedSession_PostgreSQLExcludedShapesRemainIndeterminate`, and `TestUnifiedSession_PostgreSQLForeignTableFailClosed` own the unified PG17 semantic matrix; focused PostgreSQL Docker evidence is recorded in Issue #11. |
| U3 | `pkg/deltascope/query_access_online_session_postgresql_tag_test.go`: `TestOnlineQueryAccessSession_PostgreSQLRoutesThroughUnifiedEntry` owns the complete unified PG17 recording-driver probe sequence and no-execution assertion; focused tagged evidence is recorded in Issue #11. |
| C1 | `pkg/deltascope/query_access_deprecation_test.go`, `query_access_online_session*_test.go`, and the legacy session tests: deprecated API source, stub, exact-error, validation-order, and caller-ownership contract |
| C2 | Unified-versus-legacy equivalence in S2 for MySQL 5.7/8.0/8.4 and TiDB 8.5, and S4 for PostgreSQL 17 |
| T1 | CLI adapter and real-binary tests: flags, TLS/session construction, exit code, streams, close, bounded failures, no-leak, and real routing |
| T2 | HTTP adapter and real-binary tests: request/status/body, registry, `connection_id`, authorization-before-dial, close, access log, bounded failures, no-leak, and real routing |
| M1 | `internal/interfaces/mcp/query_access_surface_contract_test.go`: no Query Access MCP tool |

### Row-by-Row Deletion Ledger

All rows were **retain** during Issue #10; no row then authorized a
deletion. `Blocked` identifies a missing unified owner. `Non-substitutable`
identifies boundary evidence that must remain. Retained-owner rows were baseline
inventory only, not replacement authorization: a later issue must replace each
with exact test/subtest identifiers and focused green evidence before deletion.
Issue #11 and #12 recorded those replacements on the rows they deleted.

| Current row or table | Current behavior | Unified semantic evidence | Boundary / compatibility evidence | Status |
|---|---|---|---|---|
| `query_access_session_mysql_tidb_test.go`: `PromotesProvenMySQL84CountStar`, `PromotesLiteralAndReversedOperands`, `RemainsFailClosedForUnknownFunction`, and `PromotesRelationlessLiteralShapes` | MySQL/TiDB promotion, requirements, fail-closed, and relationless shapes | `TestOnlineQueryAccessSession_MySQLTiDBSemanticMatrix` plus `TestLiveUnifiedSession_AssertsVersionAndSemanticMatrix` for exact requirement and relationless-state assertions | C1, C2 | Deleted by Issue #11 after focused default and live SDK green evidence |
| `query_access_session_mysql_tidb_test.go`: `DoesNotExecuteUserSQL` | legacy no execution | S1 | C1, C2 | Non-substitutable compatibility |
| `query_access_session_mysql_tidb_live_e2e_test.go`: `TestLiveUnifiedSession_AssertsVersionAndSemanticMatrix` table rows | MySQL 5.7/8.0/8.4 and TiDB 8.5 live shape/profile matrix | This retained unified owner | C2: `TestLiveUnifiedSession_MatchesDialectSpecificForAllProfiles` | Retained unified owner; Issue #11 moved direct semantic assertions to the unified API |
| `query_access_session_mysql_tidb_live_e2e_test.go`: `TestLiveUnifiedSession_MatchesDialectSpecificForAllProfiles/{mysql57,count_star}`, `{mysql80,count_star}`, `{mysql84,count_star}`, and `{tidb85,count_star}` | per-target routing, deprecated-session derived-target, and result equivalence | S2 | C2 | Non-substitutable compatibility; each subtest asserts `legacySession.target` equals its exact identity-derived `online.Target…` value |
| `query_access_online_session_test.go`: `TestOnlineQueryAccessSession_MatchesDialectSpecificMySQLTiDB` table rows `mysql57_count_star`, `mysql80_count_star`, `mysql84_count_star`, and `tidb85_count_star` | duplicate default per-target routing equivalence | U1 | C2 retains the live `count_star` routing row per target | Deleted by Issue #11 after focused live SDK green evidence |
| `query_access_session_integration_test.go`: `TestTrustedSDK_CountStarAdmissible`, `TestTrustedSDK_CountIntegerOneAdmissible`, `TestTrustedSDK_CountIntegerOneExcludedShapesRemainIndeterminate`, `TestTrustedSDK_RowNumberAdmissible`, `TestTrustedSDK_RankDenseRankAdmissible`, `TestTrustedSDK_SumAvgMinMaxAdmissible`, `TestTrustedSDK_ComparisonAdmissible`, `TestTrustedSDK_FilterDistinctRemainIndeterminate`, `TestTrustedSDK_UnqualifiedRelationIndeterminate`, and `TestTrustedSDK_LiteralComparisonIndeterminate` | legacy PG17 admitted aggregate/comparison and excluded-shape boundaries | `TestUnifiedSession_PostgreSQLCountOneAdmissible`, `TestUnifiedSession_PostgreSQLSemanticMatrix/comparison` (exact four requirements), and `TestUnifiedSession_PostgreSQLExcludedShapesRemainIndeterminate` | C1, C2 | Deleted by Issue #11 after focused PostgreSQL Docker green evidence |
| `query_access_session_integration_test.go`: `TestNewSessionFromConn_Success`, `TestSession_CallerOwnsClose`, `TestNewTrustedServiceFromSession_Success`, `TestNewSessionFromConn_ClosedConn`, `TestNewSessionFromConn_CanceledCtx`, `TestTrustedSDK_CallerRetainsConnection`, `TestTrustedSDK_RejectsExternalSchemaResolver`, `TestTrustedSDK_DefaultRemainsFailClosed`, `TestTrustedSDK_NonPostgreSQLDialectRejected`, `TestTrustedSDK_NilContextRejected`, `TestTrustedSDK_NilSessionRejected`, and `TestSameConnection_ActualLookups` | old session lifecycle, validation, offline/default, and same-connection contract | S3, S4 | C1 | Non-substitutable compatibility |
| `query_access_session_postgresql_recording_test.go`: `TestTrustedSDK_CountIntegerOneDoesNotExecuteUserSQL` fixed identity/catalog probe assertions | legacy detailed PG17 probe sequence | `TestOnlineQueryAccessSession_PostgreSQLRoutesThroughUnifiedEntry` | C1; retained `TestTrustedSDK_CountIntegerOneForeignTableNoExecNoLeak` and `TestTrustedSDK_CountIntegerOneCatalogLookupFailureNoLeak` | Deleted by Issue #11 after focused tagged SDK green evidence |
| `query_access_session_postgresql_recording_test.go`: `TestTrustedSDK_CountIntegerOneForeignTableNoExecNoLeak` and `TestTrustedSDK_CountIntegerOneCatalogLookupFailureNoLeak` | relation-kind trust and bounded failure/no-leak | S5 | C1 | Non-substitutable trust/compatibility |
| `query_access_online_session_test.go`: `TestOnlineQueryAccessSession_NoExecutionNoLeakMatchesDialectSpecificMySQLTiDB/{mysql57,mysql80,mysql84,tidb85}` | duplicate default unified-versus-deprecated recording/result equivalence | `TestOnlineQueryAccessSession_MySQLTiDBRecordingMatrix/{mysql57,mysql80,mysql84,tidb85}` | C2 retains `TestLiveUnifiedSession_MatchesDialectSpecificForAllProfiles/{mysql57,count_star}`, `{mysql80,count_star}`, `{mysql84,count_star}`, and `{tidb85,count_star}` | Deleted by Issue #11 after focused default and live SDK green evidence |
| `query_access_online_session_test.go`: `onlineEquivalenceCases` `mysql84_sum`, `mysql84_literal`, `mysql84_relationless`, `mysql84_unknown_function`, and `mysql84_insert` rows | duplicate MySQL 8.4 semantic equivalence | `TestOnlineQueryAccessSession_MySQLTiDBSemanticMatrix` | C2 retains one `COUNT(*)` routing row per MySQL/TiDB target | Deleted by Issue #11 after focused default SDK green evidence |
| `query_access_session_mysql_tidb_live_e2e_test.go`: `TestLiveUnifiedSession_MatchesDialectSpecificForAllProfiles` `direct_aggregate`, `literal_only`, `relationless_literal`, `unknown_function`, and `unqualified` rows for every target | duplicate live semantic equivalence | `TestLiveUnifiedSession_AssertsVersionAndSemanticMatrix` | C2 retains one `count_star` routing row per target | Deleted by Issue #11 after focused live SDK green evidence |
| `query_access_online_session_postgresql_tag_test.go`: all ten `TestOnlineQueryAccessSession_PostgreSQLMatchesLegacyAPI` table rows, including `SELECT COUNT(1) FROM app.orders` | duplicate tagged PG17 semantic and routing equivalence | S3 | C2 retains the live `TestUnifiedSession_MatchesLegacyPG17` `COUNT(1)` routing row | Deleted by Issue #11 after focused tagged SDK green evidence |
| `query_access_online_session_postgresql_integration_test.go`: `TestUnifiedSession_MatchesLegacyPG17` table rows `SELECT count(*) FROM app.users`, `SELECT COUNT(1) FILTER (WHERE true) FROM app.orders`, `SELECT COUNT(1) FROM app.remote_orders`, `SELECT count(amount), sum(amount), avg(amount), min(amount), max(amount) FROM app.orders`, and `SELECT u.id FROM app.users u JOIN app.orders o ON u.id = o.user_id` | duplicate live PG17 semantic equivalence | U2 | C2 retains one `COUNT(1)` routing row | Deleted by Issue #11 after focused PostgreSQL Docker green evidence |
| `query_access_online_session_test.go`: `TestOnlineQueryAccessSession_MySQLTiDBRecordingMatrix` per-target rows | direct ordered MySQL/TiDB recording probes, no-execution, and no-leak | U1 | C1 | Retained unified owner |
| `query_access_online_session_postgresql_tag_test.go`: route/excluded-shape rows | tagged PG17 semantic matrix | S3 | C2 | Retained unified owner |
| `query_access_online_session_postgresql_tag_test.go`: foreign-table, lookup failure, ownership, validation, constructor, and no-execution rows | PG17 trust, lifecycle, and bounded failure | S3 | C1, C2 | Retained unified owner |
| `query_access_online_session_postgresql_integration_test.go`: all five rows | live PG17 route, foreign-table, same-session, and equivalence | S4 | C2 | Retained unified owner |
| `query_access_online_session_postgresql_notag_test.go` | no-tag capability and legacy stub contract | S1 | C1 | Non-substitutable compatibility |
| `query_access_deprecation_test.go` | six deprecation notices | -- | C1 | Non-substitutable compatibility |
| `query_access_e2e_mixed_literal_test.go`: deleted `TestQueryAccessOnline_BuiltBinarySupportedProfiles/{mysql57,mysql80,mysql84,tidb85}` and `TestQueryAccessOnline_MixedLiteralScalars` online product/profile/shape rows | repeated MySQL/TiDB product/profile/shape results | U1 | T1: `TestQueryAccessOnline_BuiltBinaryTransportSmoke/{mysql84_admissible,mysql84_unknown_function,tidb85_admissible,tidb85_unknown_function}` retains one real route per supported CLI family with admitted/fail-closed result, exit code, key requirement/reason, and no-leak checks | Deleted by Issue #12 after focused RED exit-mapping mutation and live Docker GREEN evidence |
| `query_access_e2e_mixed_literal_test.go`: `TestQueryAccessOnline_DefaultOffline` | CLI default/offline behavior | S1, S6 | T1 | Non-substitutable transport; retained by Issue #12 with indeterminate exit/result and no-leak assertions |
| `query_access_test.go` CLI admitted/rejected/mode/JSON result rows | CLI command serialization and exit mapping | S1, S6 | T1 | Non-substitutable transport |
| `query_access_test.go` CLI flags, removed-password, TLS, and constructor rows | CLI input/session/error contract | -- | T1 | Non-substitutable transport |
| `query_access_postgresql_online_recording_test.go`: `TestCLIOnlinePG17_CountIntegerOne_Recording` fixed-probe assertions | duplicated adapter probe sequence | U3 | T1: the same focused adapter test retains session configuration, one close, no user SQL/`EXPLAIN`/prepare, and bounded CLI output; lifecycle/error rows remain below | Deleted by Issue #12 after focused tagged recording GREEN evidence |
| `query_access_postgresql_online_recording_test.go` cancellation, closed-session, connection-failure, and catalog-failure rows | CLI lifecycle and bounded errors | S3, S5 | T1 | Non-substitutable transport |
| `query_access_postgresql_no_leak_test.go`: `CountIntegerOne`, `ExcludedShapes`, and `DefaultOffline` | CLI stdout/stderr sink privacy | S3 | T1 | Non-substitutable per-sink no-leak and offline |
| `query_access_probe_boundary_no_leak_test.go` CLI rows | CLI MySQL/TiDB identity-boundary privacy | S1 | T1 | Non-substitutable per-sink no-leak |
| `main_e2e_postgresql_query_access_test.go` CLI: `CountIntegerOne` | real CLI PG17 admissible route | S4 | T1 | Non-substitutable real-route smoke |
| `main_e2e_postgresql_query_access_test.go` CLI: syntax-envelope exclusions other than `app.remote_orders`, default, no-leak, and connection-failure rows | CLI syntax, offline, sink privacy, and bounded failure | S3, S4 | T1 | Non-substitutable transport |
| `main_e2e_postgresql_query_access_test.go` CLI: `SELECT COUNT(1) FROM app.remote_orders` | PostgreSQL foreign-table relation-kind trust | S3, S4 | T1 | Non-substitutable foreign-table evidence |
| `query_access_e2e_mixed_literal_test.go`: deleted `TestQueryAccessOnline_MixedLiteralScalars/{mysql57,mysql80,mysql84,tidb85}/{COALESCE,NULLIF,IFNULL,LOWER_literal,UPPER_literal,LENGTH_literal,CHAR_LENGTH_literal,ABS_literal,CEIL_literal,CEILING_literal,FLOOR_literal,COUNT_literal,COALESCE_reversed,NULLIF_reversed,IFNULL_reversed,COALESCE_all_constant,NULLIF_all_constant,IFNULL_all_constant,relationless_lower,relationless_upper,relationless_length,relationless_char_length,relationless_abs,relationless_ceil,relationless_ceiling,relationless_floor,relationless_count_literal,relationless_coalesce,relationless_nullif,relationless_ifnull}` (4×30 online product/profile/shape rows) | repeated MySQL/TiDB product/profile/shape results | U1 | T2: `TestQueryAccessOnline_TransportSmoke/{mysql84_admissible,mysql84_unknown_function,tidb85_admissible,tidb85_unknown_function}` retains one real HTTP route per supported family with admitted/fail-closed status/body, key requirement/reason, request ID/access-log synchronization, and no-leak checks | Deleted by Issue #13 after focused HTTP authorization/zero-dial mutation RED-GREEN and MySQL/TiDB real-route GREEN evidence |
| `query_access_e2e_mixed_literal_test.go`: `TestQueryAccessOnline_DefaultOffline` | HTTP default/offline behavior | S1, S6 | T2 | Non-substitutable transport; retained by Issue #13 with indeterminate HTTP result and response/access-log no-leak assertions |
| `query_access_test.go` HTTP offline request/result rows | HTTP parsing, status, body, and defaults | S1, S6 | T2 | Non-substitutable transport |
| `query_access_test.go` HTTP `connection_id`, registry, authorization, and zero-open rows | authorization-before-dial and registry boundary | -- | T2 | Non-substitutable authorization-before-dial |
| `query_access_postgresql_online_recording_test.go`: `TestHTTPOnlinePG17_CountIntegerOne_Recording` fixed-probe assertions | duplicated adapter probe sequence | U3 | T2: the same focused adapter test retains pinned-session delegation, one close, no user SQL/`EXPLAIN`/prepare, and bounded HTTP response; catalog-failure behavior remains below | Deleted by Issue #13 after focused tagged recording GREEN evidence |
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

### Issue #13 Focused Green Evidence

- `go test ./internal/interfaces/http -run '^TestHandlerQueryAccessOnlineGuardPathsOpenNothing$' -count=1` failed while the authorization guard was temporarily bypassed: the unauthorized subtest returned 500 after one opener call and the zero-open assertion failed; it passed after the original guard was restored.
- `go test -tags integration ./internal/interfaces/http -run '^TestQueryAccessOnline_(TransportSmoke|DefaultOffline|ConnectionFailureNoLeak)$' -count=1` covers the retained MySQL 8.4/TiDB 8.5 admitted/fail-closed real routes, HTTP default/offline boundary, bounded real MySQL credential failures, response/access-log no-leak, and synchronized log entry.
- `CGO_ENABLED=1 go test -tags 'postgresql integration' ./internal/interfaces/http -run 'TestHTTPOnlinePG17_(CountIntegerOne_Recording|CatalogFailure_NoLeak|CountIntegerOne_NoLeak|ExcludedShapes_NoLeak|NoConnectionID_NoLeak|Unauthorized_ZeroDial|UnknownConnection_ZeroDial|ConnectionFailure_NoLeak)' -count=1` covers the focused PG17 recording adapter, catalog failure, syntax-envelope, offline/default, authorization-before-dial, bounded error, and per-sink no-leak boundaries.

### Issue #11 Focused Green Evidence

- `go test ./pkg/deltascope -run 'Test(OnlineQueryAccessSession_MySQLTiDBSemanticMatrix|MySQLTiDBQueryAccessSession_)' -count=1` passed after the default unified MySQL/TiDB matrix was added and the four authorized deprecated semantic rows were removed.
- `CGO_ENABLED=1 go test -tags postgresql ./pkg/deltascope -run 'TestOnlineQueryAccessSession_(MySQLTiDBSemanticMatrix|PostgreSQLRoutesThroughUnifiedEntry|PostgreSQLDoesNotExecuteUserSQL)|TestTrustedSDK_CountIntegerOne(ForeignTableNoExecNoLeak|CatalogLookupFailureNoLeak)' -count=1` passed after the unified PG17 recording owner replaced the authorized legacy detailed-probe row.
- `go test -tags integration ./pkg/deltascope -run 'TestLiveUnifiedSession_(AssertsVersionAndSemanticMatrix|MatchesDialectSpecificForAllProfiles)' -count=1` passed against the existing MySQL 5.7/8.0/8.4 and TiDB 8.5 Docker fixtures.
- `CGO_ENABLED=1 go test -tags 'postgresql integration' ./pkg/deltascope -run 'TestUnifiedSession_(PostgreSQLCountOneAdmissible|PostgreSQLSemanticMatrix|PostgreSQLExcludedShapesRemainIndeterminate|PostgreSQLForeignTableFailClosed|MatchesLegacyPG17)' -count=1` passed against the existing PG17 Docker fixture.

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
