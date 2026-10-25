# T10-R6 closeout — attribution ordering, ambiguous consumers, batch-state closure

Product `6cec4e6c98f9258ddda168494d7bca7f95bfcad3` (appended after R5
evidence `b24655fa`). Scope: R5-A residual only — provider attribution,
consumer-side ambiguity reflection, ordered-state binding order. T11 not
entered; no T10 expansion.

## A–D counterexample outcomes (red → green)

- **A. Provider attribution.** Two same-sequence unique folds beside a
  renamed PK (`aa(id)` + `pk2(id)`, both NON_UNIQUE=0/CLUSTERED=NO)
  withheld PK object identity instead of promoting the STATISTICS-first
  fold; reversed `[b,a]` never binds `[a,b]`; unrecognized/missing
  clustered rows keep storage unknown; `pkCount > 1` demotes every
  candidate. Red: 5 provider FAILs on `2f37fbd4` → green on 6cec4e6c.
- **B. Ambiguous candidates.** `ALTER INDEX pk2 INVISIBLE` under
  withheld attribution emits `target_table.primary_key` gap +
  unverified coverage + review verdict (no fabricated 3522, no silent
  pass); the exists rule adds no masking; kind-withheld pairings emit
  `target_table.indexes`; post-state tombstones rather than publishing a
  maybe-refused flip. Red: audit FAILs → green.
- **C. DROP PRIMARY KEY routing.** Genuine `drop_primary_key` resolves
  `ResolvePrimaryNameBinding`: freed `primary` → proven-missing finding
  (native 1091 reused from R5 probe a2b); member-owned `primary` →
  member-first; ambiguous → only the unproven side's fact.
  `drop_constraint`-derived actions keep constraint-identity semantics
  (plain PK existence). Red: rule FAILs → green; two R4 regressions
  found and closed.
- **D. PK real names past member gates.** `applyDropIndex`,
  `applyRenameIndex`, `applyAlterIndexVisibility` resolve the shared
  binding before member provability gates: `RENAME INDEX pk2 TO pk3`
  with unvouched member names unvouches only `PrimaryKey.NameUnknown`;
  the next statement stops reading pk2 as proven. Handle/clustered
  uncertainty no longer downgrades a proven non-`primary` object name
  (spec F1 fix). Red: audit FAILs → green.

## Independent review closure

Standards + spec axes ran against the full R6 diff. Findings closed:
dead `indexSubjectName`, four unused `statement` params (whole unparam
class removed from `batch_state_index.go`), gap-fact documentation drift
(`indexes` vs `indexes.key_parts`, four sites aligned), duplicated
member-drop/VISIBLE-pin/binding-preamble logic extracted into helpers,
stale provider algorithm comment rewritten, spec F1 (handle-unknown vs
proven stored name) and F3 (gap fact accumulation) fixed and red-pinned.
`indexMemberExistenceGaps` PK-side ambiguity was re-assessed: the
wrapper emits `primary_key` only on TiDB non-`primary` ambiguous targets
where PK-side facts are withheld — no over- or under-report. The
rename-path asymmetry (ambiguous source unvouches both sides; proven
PK-object source unvouches only the stored name) is semantically
intentional.

## Gates bound to 6cec4e6c

`gates/` quads: go test ./... 0 · go vet 0 · race ./internal/... 0 ·
gofmt clean · sql-corpus 0 · query-access-corpus 0 · ddl-inventory 0 ·
ddl-coverage-catalog 0 · ddl-golden-validator 0 ·
parser-error-unsupported-contract 0 · unsupported-diagnostics-evidence 0
· docs-example 0 · decision-record 0 · pg-confidence-gates 0 ·
three-level-doc OK · lint rc=1 baseline-only (R6 removed 5 findings,
added 0).

Golden: T09 125/125 cases / 3442 assertions, T10 126/126 / 3175 —
identical to the frozen R5 denominators, both artifacts record
`head_sha=6cec4e6c…` and the same CLI sha256
(`363782e6…`, also the local `bin/deltascope` build — deterministic
CGO_ENABLED=0 build).

## Verified / unverified boundary

Verified: everything above on MySQL 8.0.46/8.4.10/5.7.44 and TiDB v8.5.0
anchors plus all offline gates. Native R6 probes cover exactly the three
catalog shapes in `native-probe/r6-attribution-probe.log`; MySQL's
PK-less member-named-`primary` reservation cell remains unprobed (R5
boundary, unchanged). Not claimed: release, tag, PR, CI status, T11.
