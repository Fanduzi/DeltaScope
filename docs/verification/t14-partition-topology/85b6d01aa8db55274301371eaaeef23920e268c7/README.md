# T14-R2-tail evidence bundle — 85b6d01a

Independent-acceptance evidence for T14-R2-tail (#93 remediation): the
single-member undecidable premise regression, plus merged evidence-text
corrections. Product commit
`85b6d01aa8db55274301371eaaeef23920e268c7` (parent `f95dc949`, the
T14-R2 evidence-closeout candidate commit — acceptance pending).
Contract: CONTRACT.md §10.6; ADR:
`docs/decisions/2026-10-28-ddl-partition-topology-r2-remediation.md`
(T14-R2-tail section).

## Scope delivered

- **Single-member undecidable premise.** `declaredSchemeOutcome` now
  accumulates the per-member bound premise (A) separately from the
  pairwise ordering/non-overlap adjudication (B). A one-member
  RANGE/LIST scheme with a nil, empty-shell, or expression bound is
  unproven — never combinable on zero comparisons — so no fabricated
  member directory reaches the public ordered state. Undecidable
  members accumulate without early return; a later provable pairwise
  violation still refuses and preserves the exact pre-state. Boundless
  HASH/KEY members keep their combinable outcome.
- **ALTER vs CREATE unproven post-states.** ALTER `PARTITION BY`
  degrades at scheme level — table existence and unrelated structural
  facts (columns, indexes, PK, constraints) survive while `Partitioned`,
  `PartitionType`, the member directory/definitions, and
  partition-dependent statistics unvouch; an undecidable ADD degrades
  only at directory level, where `Partitioned` and the known scheme
  survive. CREATE on a provider-proven-absent target publishes neither a
  proven directory nor a proven-absent guess. Member-touching
  successors owe the actually-unknown facts (e.g.
  `target_table.partitioned`) instead of passing on a pseudo-directory.
  Rule enable/disable does not change the state conclusion.
- **Evidence-text corrections** (same commit chain): CONTRACT §10.2 /
  R2 ADR / R2 bundle README now distinguish the two gap vocabularies —
  `unknown_table_state`+`target_table.partition_defs` for ADD/REORGANIZE
  vs `unproven_statement_fact`+`statement.partition_defs` for
  PARTITION BY; the R1 red-log leaf counts state both reproducible
  calibers (47 paths containing "/", 50 terminal failing paths incl. 3
  top-level tests with no failing child) and the "36 cells" figure stays
  withdrawn; the "36 `=== RUN`" markers are red-log data — no verbose
  green run of the R2 matrix was archived and the earlier citation is
  withdrawn (the `go test ./...` tuple remains valid green evidence);
  p15's four 1146s record the missing table `golden.my` after the 1178
  MyISAM CREATE refusal, not a missing `golden` database; pre-acceptance
  commits are labeled "candidate"; row routing stays verified on the
  frozen REORGANIZE golden path rather than blanket-deferred.

## Red baseline

`red/` — captured on the unremediated product `ce9a73dc` (detached
worktree + the tail test file). `go-test-t14r2tail.stdout.txt` records
the real verbose failure run: **4 failing top-level tests**, with the
fabricated `PartitionNames:[fresh]` directory printed verbatim under
`TestT14R2TailRuleOffKeepsStateConclusion` — a bound-undecidable
single member publishing as proven even with every rule off.
`go-test-t14r2tail.command.txt`
records the exact argv and worktree SHA; rc=1 is archived.

## Green run

`green/` — the same verbose matrix at the final product SHA
`85b6d01a`: `go-test-t14r2tail.stdout.txt` records 22 `=== RUN`
markers / all PASS (rc=0, command + argv archived). This is the
archived verbose green log for the tail matrix.

## Gates

`gates/` — 19 required gates + 3 committed-diff/final-tree L2 checks
+ 1 full-repo lint diagnostic, all on `85b6d01a`, each with real argv,
stdout, stderr, and rc. See `gates/INDEX.md`: 18 required green,
`lint-scoped` at exactly the 13-issue accepted baseline (stdout
byte-identical to the ce9a73dc record), `gofmt` listing only the two
accepted T07 files, `lint-fullrepo` recorded verbatim as a 37-issue
diagnostic outside the accepted scope.

## Golden

`golden/` — six serial runs on `85b6d01a`, one CLI sha256
(`6bf916bfc578…`):

| Task | cases | assertions | result |
|------|-------|------------|--------|
| T09 | 125/125 | 3442 | PASS |
| T10 | 126/126 | 3175 | PASS |
| T11 | 40/40 | 506 | PASS |
| T12 | 25/25 | 282 | PASS |
| T13 | 47/47 | 697 | PASS |
| T14 | 69/69 | 1087 | PASS |

T09–T13 denominators frozen. T14 keeps all 68 previously-required IDs
and adds `t14r2tail-mysql80-pby-single-expr-member` — live-anchor
coverage of the single-member → successor path (assertions 1066→1087).
All six artifacts: `head_sha` = `cli.build.head_sha` = `85b6d01aa8db`,
required == executed, no duplicate IDs, `compose_down_rc=0`,
`residual_containers=[]`.

## Verified / unverified boundaries

Verified on this SHA: single-member nil/shell/expression bound →
unproven for ALTER and CREATE through the public `AuditSQL` seam and
the internal outcome helper; positive controls (legal numeric single
member, single-member MAXVALUE, boundless HASH/KEY, CREATE
IF NOT EXISTS no-op, later provable refusal past an earlier
undecidable member); successor gap ownership; rule-independence of
the state conclusion; 19 gates + 3 L2 checks + six golden suites.
Unverified/deferred: expression bound items are still never evaluated
(capability aspect, stricter than the native fold); string ordering
without a proven collation domain stays unproven; multi-action ALTERs
and non-partition aspects stay fail-closed; tablespace artifacts,
INTERVAL evaluation, subpartition modeling, and row routing beyond the
frozen REORGANIZE golden path (already verified there) remain deferred
to T15+.
