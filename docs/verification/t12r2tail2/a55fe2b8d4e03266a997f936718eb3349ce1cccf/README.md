# T12-R2-tail2 evidence bundle — a55fe2b8

Acceptance-candidate evidence for T12 (#91) R2-tail2: one defect closed — an
unproven sibling drop could still prop up a proven identity refusal — plus
wording corrections to the prior `t12r2tail` archive. Product commit
`a55fe2b8d4e03266a997f936718eb3349ce1cccf` (parent `c70be195`, the R2-tail
evidence commit). A/B and the proven-drop arm of D are preserved.

## Defect and fix

`multiActionIdentityCheckVerdict` (`internal/application/audit/batch_state_multi.go`)
evaluated `checkColumnIdentityOutcome` on the pre-state minus only
*provably* dropped members (value 2). A member an unproven sibling drop
may hit (value 1) stayed in that shape, so a referencing member under an
undecidable generic `DROP CONSTRAINT` still forced `identityScanRefused` —
the maybe-dropped check ran too late to undo it.

Fix: members split three ways (proven-removed / maybe-hit / definite). The
refusal is evaluated on the definitely-remaining set only; a maybe-hit
member that references (or may reference) the column makes the verdict
undecidable before any refusal is honored; a proven removal still withholds
the successor (unmodeled landing).

## Red baseline

`red/tail2_red.log` — captured on the prior product (`f843b2c9`) before the
fix: 10 failing cells — `UnprovenSiblingDropWithholdsSuccessor` all 8
(kind `"unknown"`/`""` × RENAME both orders + CHANGE both orders) and
`UnprovenSiblingDropSuccessorReport` both kinds (`gaps=[]` — old chk_a
falsely bindable). The four controls already passed on the buggy product,
as intended.

## Regression cells (all green on a55fe2b8)

`internal/application/audit/batch_state_t12r2tail_test.go`, tail2 section:

- `TestT12R2Tail2UnprovenSiblingDropWithholdsSuccessor` — 8 `t.Run` cells
  (2 kinds × 4 statements); internal post-state must be nil (withheld),
  never an exact pre-state.
- `TestT12R2Tail2UnprovenSiblingDropSuccessorReport` — 2 `t.Run` cells on
  the public two-statement Audit path asserting the successor's gap carries
  `RuleID=ddl.alter.drop_check.exists.require` with
  `RequiredFacts=[target_table.existence]`, no fabricated exists finding,
  `Coverage=unverified`, `Verdict=review`.
- `TestT12R2Tail2SiblingDropControls` — 4 controls: proven-secondary same-name
  index keeps the unlock-withheld; proven-unique keeps the ambiguity refusal
  (exact pre-state); a definite second referencing member keeps the 3959
  refusal despite the unproven sibling drop; an unrelated unproven drop does
  not lift the refusal.

## Gates (18 groups, serial, bound to a55fe2b8)

`gates/` holds per-gate four-tuples
(`<gate>.command.txt`, `<gate>.stdout.txt`, `<gate>.stderr.txt`,
`<gate>.rc.txt`): 17 × rc=0; `lint` rc=1 with the 13-issue accepted baseline
(byte-identical output, zero new); `gofmt` lists only the two accepted T07
baseline files.

## Golden suites (serial, shared anchors, bound to a55fe2b8)

`golden/<TASK>/artifact.json`, cases under `golden/<TASK>/cases/`, audited
CLI at `golden/<TASK>/bin/deltascope`.

| suite | cases | assertions | status |
|-------|-------|------------|--------|
| T09 | 125 / 125 | 3442 | PASS |
| T10 | 126 / 126 | 3175 | PASS |
| T11 | 40 / 40 | 506 | PASS |
| T12 | 25 / 25 | 282 | PASS |

All four `head_sha`/`cli.build.head_sha` = `a55fe2b8…`; CLI binaries
sha256-identical (`207790c4434b160b04ba8625a01e701010ff2518912459828546f8e18f6b561d`,
CGO_ENABLED=0); `residual_containers=[]` per run.

## Wording corrections applied to the t12r2tail archive

- `red/tail_red.log` described per its actual content: 5 failing parent
  tests (A×1, B×2, D×2) + 2 passing controls; no per-cell claims past a
  parent's first `Fatal`.
- Green references point at real four-tuple names (`go_test______.*`,
  `go_race.*`); gate tuple suffixes documented as `.command/.stdout/
  .stderr/.rc.txt`; golden index lists `cases/` and `bin/deltascope` paths
  that exist.
- Assertion layers stated per item (A: public finding + internal post-state;
  B: public successor gaps; D: internal post-state + public gap).
- Native probe scope corrected: RENAME both orders probed; CHANGE probed
  drop-first only — the reverse order is unit-test coverage.
- CONTRACT §5.8 / ADR item 3 wording corrected to match, and extended with
  the tail2 three-way member split.

## Observed vs unobserved

Observed: nothing new natively this round — the unknown/empty-kind index is
an injected fixture; the two compatible worlds (secondary → drop removes
check; unique → 3939) were already probed in R2 (`probes/r2/`).
Unobserved: TiDB same-statement unproven-drop + identity combinations;
full success templates for unblocked identity actions (successor withheld
by design); member-level (non-tombstone) modeling of the unproven-drop
statement itself — the table-wide invalidate is the bounded answer.
