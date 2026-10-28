# T14-R2-tail2 evidence bundle — 2119f230

Independent-acceptance evidence for T14-R2-tail2 (#93 remediation): the
MAXVALUE-then-undecidable pair-refusal control-flow regression, plus the
accompanying doc/fixture corrections. Product commit
`2119f2305b92ea10b66f8c7c3cf79f714c198b8b` (parent `50393d40`, the
T14-R2-tail evidence-closeout candidate commit — acceptance pending).
Contract: CONTRACT.md §10.7; ADR:
`docs/decisions/2026-10-28-ddl-partition-topology-r2-remediation.md`
(T14-R2-tail2 section).

## Scope delivered

- **Pair check independent of member comparability.** The RANGE branch
  of `declaredSchemeOutcome` no longer joins the per-member
  comparability check and the pairwise adjudication with `else if`: an
  uncomparable member records undecidable while the predecessor's pair
  check still runs — a proven MAXVALUE predecessor refuses whatever
  follows (expression, nil, or empty-shell bound alike). A recorded
  undecidable still applies when no independent refusal exists, and a
  later provable violation still survives an earlier undecidable
  member. `LessThanPairOutcome`'s MAXVALUE semantics are unchanged; no
  rule finding was adapted to the state error.
- **ALTER preserves the exact pre-state.** `PARTITION BY RANGE` with
  `qmax MAXVALUE, qexpr (1+0)` is a proven refusal: the
  `partition_bound_violation` finding and the expression capability
  aspect stay, and the ordered state keeps the original directory,
  partition method, structure, and data statistics — the successor
  binds `p0` with no findings and no gaps.
- **CREATE keeps proven absence.** The same pair on a
  provider-proven-absent target is a proven refusal that keeps the
  known-absent entry (`Exists=false`) — never unvouching into an
  unknown successor premise.
- **Accompanying corrections.** The PARTITION BY unproven post-state is
  described at scheme level (`Partitioned`, `PartitionType`, directory,
  partition-dependent statistics unvouch) and kept distinct from an
  undecidable ADD's directory-level degradation (`Partitioned` and the
  known scheme survive); the `range_columns_string` positive-control
  fixture now carries a real `VARCHAR c` column; the tail bundle's
  `PartitionNames:[fresh]` red-log print is attributed to
  `TestT14R2TailRuleOffKeepsStateConclusion` where the log actually
  emits it.

## Red baseline

`red/` — captured on the unremediated product `85b6d01a` in the main
worktree before the fix (the product code at that checkout is exactly
the unremediated baseline; labeled a baseline replay of the tail2 test
file). `go-test-t14r2tail2.stdout.txt` records the real verbose
failure run: **4 failing top-level tests / 7 FAIL markers** across 10
`=== RUN` entries — both public arms
(`MaxvalueThenExprRefusesAlter/Create`), the rule-off refusal control,
and the three MAXVALUE-then-undecidable helper cells — while the
`expr_then_decreasing`, `single_expr`, and `legal_sequence` controls
already passed. `go-test-t14r2tail2.command.txt` records the exact
argv and SHA; rc=1 is archived.

## Green run

`green/` — the same verbose matrix at the final product SHA
`2119f230`: 10 `=== RUN` entries all PASS, rc=0, command + argv
archived.

## Gates

`gates/` — 19 required gates + 3 committed-diff/final-tree L2 checks
+ 1 full-repo lint diagnostic, all on `2119f230`, each with real argv,
stdout, stderr, and rc. See `gates/INDEX.md`: 18 required green,
`lint-scoped` at exactly the 13-issue accepted baseline (stdout
byte-identical to the ce9a73dc record), `gofmt` listing only the two
accepted T07 files, `lint-fullrepo` recorded verbatim as a 37-issue
diagnostic outside the accepted scope.

## Golden

`golden/` — six serial runs on `2119f230`, one CLI sha256
(`d5d6b33e4f53…`):

| Task | cases | assertions | result |
|------|-------|------------|--------|
| T09 | 125/125 | 3442 | PASS |
| T10 | 126/126 | 3175 | PASS |
| T11 | 40/40 | 506 | PASS |
| T12 | 25/25 | 282 | PASS |
| T13 | 47/47 | 697 | PASS |
| T14 | 69/69 | 1087 | PASS |

All denominators frozen — the tail2 counterexamples live in the public
test matrix; no golden cell was added or weakened. All six artifacts:
`head_sha` = `cli.build.head_sha` = `2119f2305b92`, required ==
executed, no duplicate IDs, `compose_down_rc=0`,
`residual_containers=[]`.

## Verified / unverified boundaries

Verified on this SHA: MAXVALUE predecessor refusing an
expression/nil/shell successor (ALTER + CREATE through the public
`AuditSQL` seam and the internal outcome matrix), exact-pre-state
preservation including statistics, kept-absent vs unvouched
distinction, rule-independence of the refusal, and all prior
single-member/IF-NOT-EXISTS/R2 C-controls. Unverified/deferred:
expression bound items are still never evaluated (capability aspect,
stricter than the native fold); string ordering without a proven
collation domain stays unproven; multi-action ALTERs and non-partition
aspects stay fail-closed; tablespace artifacts, INTERVAL evaluation,
subpartition modeling, and row routing beyond the frozen REORGANIZE
golden path (already verified there) remain deferred to T15+.
