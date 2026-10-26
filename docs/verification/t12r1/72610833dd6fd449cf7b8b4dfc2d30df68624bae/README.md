# T12-R1 verification — independent-acceptance remediation (reviewer groups A–E)

Product SHA: `72610833dd6fd449cf7b8b4dfc2d30df68624bae`
(`fix(t12): T12-R1 acceptance remediation — spelling-aware CHECK gates, shared
target binding, atomic premises (issue #91)`)

Parent chain: `2f16db0a` (T12 evidence) → `72610833` (R1 product). Product is
the diff `2f16db0a..72610833` (37 files, +2550/−350).

## What changed (product groups)

- **A — spelling-aware version gates.** `spec.CheckAlterSyntaxOutcome` splits
  the MySQL floor by authored spelling: `CHECK` works from 8.0.16, the generic
  `CONSTRAINT` alias from 8.0.19 (probed live on 8.0.17 — CHECK rc=0,
  CONSTRAINT 1064). `Constraint.EnforcedExplicit` distinguishes an authored
  `ENFORCED`/`NOT ENFORCED` clause from a defaulted member; on the MySQL
  parse-and-ignore arm the explicit clause is a proven 1064 syntax refusal
  (CREATE TABLE / ADD COLUMN publish nothing), while the bare declaration
  stays accepted-and-ignored. Spelling is recovered by a comment/quote-aware
  clause scanner in the extractor (`Alter.Options["check_spelling"]` ∈
  `check`/`constraint`/`""`), not by regex over the whole statement.
- **B — shared target binding.** `spec.CheckDropTarget` /
  `spec.CheckAlterTarget` are the single five-state classifier (proven /
  missing / wrong-kind / ambiguous / unproven) consumed by rule findings,
  EvidenceGaps, and ordered state. `CHECK` spelling binds only CHECK
  members; generic `CONSTRAINT` resolves among CHECK/UNIQUE/PK/FK.
  Secondary indexes are wrong-kind (3821-class); cross-type same-name
  generic targets are the probed 3939 ambiguity; unknown-kind members keep
  the result unproven instead of guessed-missing. TiDB's probed
  dialect-distinct generic binding stays separate.
- **C — column-drop cascade.** `dropColumnCascadedChecks` removes a CHECK
  only when its proven reference set is exactly the dropped single column;
  multi-column references refuse (3959-class), unvouched/partial reference
  sets keep the unsafe path instead of silently cascading.
- **D — atomic publication premises.** `applyAddCheck` proves referenced
  columns before appending (missing reference → 1054-style refusal, entry
  unchanged). `inColumnChecksOutcome` runs before column publication:
  MySQL duplicate in-column CHECK name → whole-statement refusal (3822 —
  the new column never lands); TiDB flag-ON's probed silent-ignore lands
  the column and drops the member. Multi-action `add_columns` carrying
  inline CHECK payloads are unmodeled → unprovable (no partial publish).
- **E — inline CHECK naming governance.** `alter.Constraints` (extracted
  in-column CHECK members) join `check.name.*` applicability and name
  selection with dedup against other declaration paths.

## Red → green

- `red/baseline_red.log` — 13 `--- FAIL` markers on the pre-remediation
  product (`2f16db0a` + the new R1 test file), captured via the public
  `Audit` path; each failure asserts first-statement result AND successor
  `Metadata.TargetTable`/findings/gaps/coverage/verdict.
- Green: `go test ./internal/application/audit -run 'TestT12R1'` — 17/17
  sub-tests pass on the product SHA; full `go test ./...` rc=0 (39 pkgs).

## Oracle updates (authorized, CONTRACT §3a)

- `T12.cli.t12-cli-{create-notenforced,add-check-declared}-offline-gap`:
  expected gaps 2 → 3 — explicit `NOT ENFORCED` on an unobserved-version
  MySQL target now also requires `target.version` for
  `ddl.alter.alter_check.version.require` (5.7 refusal vs 8.0.16+ stored).
- `batch_state_t11r1_f_test.go` / `batch_state_t11r2_f_test.go` (5 spots):
  recovered spelling makes proven generic-drop removals deterministic;
  unknown-kind and genuinely unvouched cases keep their gap expectations.

## Gates (bound to the product SHA)

18 groups under `gates/` — 17 standard gates plus the `three-level-doc-diff`
committed-diff group (each group keeps the four-tuple
`command/rc/stdout/stderr`; `three-level-doc-diff.stderr.txt` is empty
because the diff command writes nothing to stderr). 17×rc=0; `lint` rc=1
matches the 15-issue historical baseline (errcheck 1, gocritic 5, gofmt 2,
staticcheck 2, unconvert 3, unparam 2) with zero hits in R1-touched code —
same issue set, not byte-identical output: two line numbers inside the
pre-existing `batch_state_multi.go` entries moved with the diff. `gofmt`
reports only the two accepted T07 baseline files.

## Golden (serial, one anchor set, bound to the product SHA)

`golden/` — T09 125/125 cases / 3442 assertions; T10 126/126 / 3175;
T11 40/40 / 506; T12 25/25 / 282 (oracle rows §3a add two gap assertions).
All four artifacts: `head_sha` = `cli.build.head_sha` =
`72610833dd6fd449cf7b8b4dfc2d30df68624bae`, identical `cli.sha256`
`05aec94e1b2d09273e033bc480fec7f1a8b2739055afbb466d90dd3094e4342b`.

## Native probes

`docs/work-plans/t12-check-enforcement/probes/r1/` — 11 logs:
mysql:5.7.44 (bare CHECK ignored vs explicit-clause 1064, incl. in-column),
mysql:8.0.17 (CHECK spelling works, CONSTRAINT spelling 1064 — the
8.0.16–8.0.18 in-band anchor), mysql:8.0.46 + mysql:8.4.10 (secondary-index
3821/3940 wrong-kind, cross-type same-name 3939 ambiguity, generic-alter
wrong-kind 3941, multi-column-reference 3959 refusal, missing-reference
1054, duplicate in-column 3822 atomicity), TiDB v8.5.0 flag ON/OFF
(generic binding divergence, silent-ignore duplicate, OFF accepts explicit
clauses), plus multi-action atomicity edges.

## Untested boundaries (kept unproven)

- MySQL patch releases outside the probed anchors (8.0.16/8.0.18/8.0.19
  exact-version behavior is documented + 8.0.17-in-band, not anchored).
- `ALTER CHECK ... ENFORCED` post-state stays unvouched (data-conditional).
- TiDB generic-target resolution is modeled only for probed shapes; other
  TiDB constraint-mutation spellings remain unclaimed.
- CHECK expression evaluation/solving stays out of scope; only reference
  sets are modeled.
