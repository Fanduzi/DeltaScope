# T07-R2 verification evidence — 9538c059

SHA-bound evidence for the T07-R2 fold-boundary amendment (issue #86,
advanced-column facts): the TiDB `SET DEFAULT` evaluation path no longer
generalizes observed sample outcomes into expression-category verdicts.
Internal review evidence only — not an external CI, reviewer, merge, or
release claim.

## Head and binaries

- HEAD: `9538c05905d2cf78895bcdcc8346b251453c3151`
  (`milestone/mysql-tidb-ddl-completion`), parent `c5761f5ce25b` (R1
  evidence), grandparent `aba447832887` (R1 code, the unfixed base used
  for the red captures).
- Golden-built CLI SHA-256: `identity/binary-sha256.txt` — the
  runner-built binary at this head and an independent
  `CGO_ENABLED=0 go build ./cmd/deltascope` rebuild are byte-identical
  (`39b76649…aacf`); deterministic build confirmed.
- `go version -m`: `identity/binary-version.txt` — revision `9538c05905d2`
  with `+dirty` caused by the tracked-but-uncommitted local `.mcp.json`
  environment file; `git status` otherwise clean at run time
  (`identity/git-status.txt`).
- Go: `identity/go-version.txt` (go1.26.1).

## Root cause and red record (unfixed `aba44783`)

`internal/domain/spec/advanced_columns.go` generalized observed sample
outcomes into expression-category results: `allFunctionsIn` matched the
uuid allowlist on any CASE containing `uuid()` (and passed on an empty
`Functions` set), wrapped `lower()` was rejected on INT regardless of
argument value, and no-function arithmetic was accepted from a bare
target-type table.

- `entry/prefix/F0*.json|.stderr`: real `deltascope audit` CLI runs at
  `aba44783` (binary `deltascope-base`, sha `a933db20…`) over the eight
  CONTRACT §12 roles, TiDB 8.5.0 target, `t07-compat` policy
  (`entry/golden-policy-t07-compat.yaml`). F01/F02 CASE cells, F04/F05
  wrapped cells and F06/F07 arithmetic all produced blocker findings or
  false positives — recorded verbatim including `raw_sql` echo per the
  existing CLI contract.
- `entry/prefix/probe_test_run.txt`: the same inputs through real
  parser/extractor/`AuditSQL` test seams on the unfixed tree —
  extracted `TopForm`/`RootParenthesized`/`Functions` facts plus
  assessment verdicts and post-state publication, proving the
  generalization at the internal seam, not inferred from `raw_sql`.

## Bounded native confirmation (10 cells, no sweeps)

- `native-probe/native_r2.sh`, `native-probe/native_r2_run.txt`:
  TiDB 8.5.0 (`pingcap/tidb:v8.5.0`) + MySQL 8.4 opposite controls,
  per-role dedicated tables (`v INT, x INT`), absence pre-check,
  create→set→catalog→cleanup, final table count 0 for all 10 roles.
  Results: F01 `set_rc=0 def=1`, F02 `set_rc=0 def=1`, F03 `rc=1 1067`,
  F04 `set_rc=0 def=1`, F05 `rc=1 1067`, F06 `set_rc=0 def=3`,
  F07 `rc=1 1292`, F08 verbatim `uuid()` stored (`DEFAULT_GENERATED`);
  MySQL 8.4 stores F01/F04 verbatim as expression defaults.
  Conclusion: fold outcomes are value-dependent; `TopForm`, deduplicated
  `Functions`, root-parenthesis state, and base type cannot prove branch
  selection, call results, or folded-value coercion.

## R2 contract supplement (committed in the code commit)

- `docs/work-plans/t07-advanced-columns/CONTRACT.md` §12 freezes the
  `tidb_set_default_eval_unproven` gap vocabulary
  (`required_facts = source_column.default_evaluation,
  target_table.default_coercion`), the bare-call allowlist re-scope
  (`TopForm=call` ∧ `!RootParenthesized` ∧ `CallCount=1` ∧ one
  deduplicated callee), and same-verdict publication.
- `EXPECTED-UPGRADES.md` U11; ADR
  `docs/decisions/2026-10-19-ddl-advanced-column-facts.md` R2 amendment.
- New internal fact `ExpressionFacts.CallCount` counts function-call AST
  nodes; it proves only whether a whole expression is exactly one call
  node — nothing about evaluation, arguments, or results.

## Green record (final SHA)

- `entry/postfix/F0*.json|.stderr`: identical inputs at `9538c059`
  (binary sha `39b76649…`, identical to the Golden CLI). F01–F07 →
  `verdict=review`, `coverage.status=unverified`, exactly one
  `tidb_set_default_eval_unproven` evidence gap with the frozen
  required_facts; F08 → `verdict=pass` (bare-call control preserved).
- `entry/postfix/probe_test_run.txt`: same-seam probe on the fixed
  tree — `UnresolvedTransition` set, post-state default withheld
  (tombstoned), proven verbatim/temporal cells unchanged.
- `entry/postfix/run_green_final.sh`: the re-runnable capture script;
  `entry/postfix/run.log` documents binary/policy/input provenance.

## Sensitivity record (constructed faults, not DB behavior)

- `sensitivity/mutation-m1.log|.out`: restoring callee-set
  generalization (removing `RootParenthesized`/`TopForm`/`CallCount`
  guards from `tidbAlterSetDefaultOutcome`) flips 14 formerly
  unproven cells — the regression matrix fails as designed, `rc=1`.
- `sensitivity/mutation-m2.log|.out`: bypassing the
  `UnresolvedTransition` tombstone gate in `batch_state.apply` lets 9
  unproven cells publish fabricated post-state defaults — post-state
  tests fail as designed, `rc=1`; the 6 proven cells stay valid.
- Both mutations were reverted immediately after capture; the committed
  tree is the unmutated code.

## Golden regressions (serial, final SHA)

| Task | Cases | Passed | Assertions | Head | Artifact |
|---|---|---|---|---|---|
| T07 | 207 | 207 | 6190 | 9538c059 | `golden/T07/` |
| T06 | 496 | 496 | 11772 | 9538c059 | `regressions/T06/` |

- `golden/T07/`: artifact, all 207 per-case files, generated policies.
- `regressions/T06/`: artifact, all 496 per-case files, generated
  policies.
- The frozen 207-case identity digest `d723ac95…f1d25` is unchanged
  (verified against the `aba44783` artifact); the frozen T06 496-case
  denominator is unchanged.
- `golden-run.log`: T07 attempt failing on
  `required fixture credential environment is unavailable` (missing
  `T07_MYSQL_PASSWORD`, environment failure preserved per contract),
  followed by the T06 496/496 pass in the same run.
- `golden-t07-retry.log`: T07 rerun with the credential env present —
  207/207.
- `golden-run.attempt1-env-fail.log`: an earlier stale-container/network
  failure, preserved; fixtures were recreated under the runner's own
  `deltascope-ddl-golden` compose project.
- Fixture credentials enter Docker only through `T07_MYSQL_PASSWORD` /
  `DS_T06_GOLDEN_PW`; no values in argv or artifacts.

## T02–T05 reuse index

R2 touches only the TiDB `ALTER COLUMN SET DEFAULT` adjudication
(`tidbAlterSetDefaultOutcome`), `ExpressionFacts.CallCount`, and the
ordered-state gate in `internal/application/audit`. `SET DEFAULT`
appears in no T02–T05 fixture; the changed paths are unreachable from
their inputs. T02–T05 artifacts therefore reuse the same-semantics runs
archived under `docs/verification/t07/aba4478328875790e39ed6aeaae0ad67c6d5a996/regressions/T02..T05/`
(10/118/163/247 cases, all pass, head `aba44783`). T06 was re-run at
this head because the R1 evidence already demonstrated T06 consumes
shared parser/extractor paths; the 496/496 result with unchanged
identity confirms no regression.

## Offline gates at HEAD (`gates/`)

Each `*.meta` records argv/cwd/start-end/HEAD/dirty-state; each `*.log`
is the complete stdout/stderr with the directly captured rc.

- `make-test`: `go test ./...` — all 39 packages ok.
- `pg-unit-test-gates`, `sql-corpus-gates`, `ddl-inventory-gate`,
  `ddl-coverage-catalog-test`, `docs-example-gates`,
  `decision-record-gate`, `ddl-golden-validator-test` (86 constructed
  T07 checks, zero failures): all pass.
- `gofmt.log`: no output over the changed Go files.
- `task-range-diff.txt`: `c5761f5c..HEAD` diff scope (13 files,
  +516/−417).
- Permanent regression coverage lives in
  `internal/application/audit/batch_state_t07_r1_test.go` (F01–F08
  matrix, post-state tombstoning, privacy, request isolation),
  `internal/domain/spec/advanced_columns_t07_r1_test.go`, and
  `internal/infrastructure/parser/tidb/expression_facts_t07_r1_test.go`.

## Boundaries

- Constructed validator checks are not product or native acceptance.
- The 10-cell native matrix bounds only the probed cells; the product
  deliberately does not evaluate expressions — eval-unproven cells stay
  bounded gaps by contract.
- This evidence does not close issues, merge, release, or claim external
  reviewer/CI acceptance; it supports human review of the milestone
  branch.
