# T12-R2 verification — independent-acceptance remediation (reviewer groups A–D)

Product SHA: `a4ce0e2d6d5ae2dfa168273147ecdd53e33deea5`
(`fix(t12): T12-R2 acceptance remediation — clause attribution, generic-drop
dependency premises, column-scoped CHECK storage, multi-action identity
(issue #91)`)

Parent chain: `ebc9fbf0` (T12-R1 evidence) → `a4ce0e2d` (R2 product). Product
is the diff `ebc9fbf0..a4ce0e2d` (19 files, +1360/−152); the R1 product diff
remains `2f16db0a..72610833`.

## What changed (product groups)

- **A — clause attribution, not keyword presence.** The extractor proves an
  authored `[NOT] ENFORCED` clause only in binding position — the first bare
  word after the `CHECK(...)` close paren of the element/clause text already
  aligned to its AST spec (`clauseDeclaresEnforced`, `nextBareWordAfter`,
  `skipCommentOrQuoted`, `skipSpaceAndComments`, `matchingParenClose` in
  `internal/infrastructure/parser/tidb/extractor.go`). An `enforced` column or
  expression identifier never sets `EnforcedExplicit` (probed 5.7: identifier
  form ignored, authored clause 1064). `createTableElementBody` and
  `splitAlterTableClauses` skip comments and quoted text, so `/* ( */` and
  `/* , */` cannot fake element bodies or action boundaries. Unalignable
  attribution sets `spec.Constraint.EnforcedUnproven` (mutually exclusive
  with `EnforcedExplicit`) — `alterCheckVersionRule` and
  `checkIgnoredNoticeRule.EvidenceGaps` emit an attributable
  `statement.check.enforced_clause` gap instead of a guessed omission, and
  `extractAlterTable` passes an empty clause rather than copying a sibling
  action's spelling. **A4**: `checkIgnoredNoticeRule` suppression now routes
  through `spec.CheckExplicitClauseRefused`, which is MySQL-only — TiDB's
  flag-OFF arm keeps the ignored finding on authored clauses (accepts and
  ignores, probed).
- **B — binding success does not imply landing.** `publishCheckDropTarget`
  in `batch_state_fk.go` routes a generic `DROP CONSTRAINT` bound to the
  primary object or a unique member through the same FK sole-coverer
  dependency premise the dedicated drops use (`foreignKeyPKObjectDependency`
  / `foreignKeyIndexDependency` — probed 1553 on 8.0.46 for
  `DROP CONSTRAINT \`primary\``, `DROP PRIMARY KEY`, and a sole-covering
  `DROP CONSTRAINT ux_p`; a replacement coverer drops clean).
  `genericConstraintKinds` in `internal/domain/spec/check_target.go` honors a
  standalone `PrimaryKeyUnknown` fact (a `primary` name that could bind the
  PK object answers Unproven, not Missing) and reports definite vs.
  uncertain kinds separately, so `dropCheckUndecidable` unvouches a
  same-named check member via `unvouchCheckMember` when the unknown-kind
  index could conceal a unique — both native worlds exist (secondary →
  dropped, unique → 3939).
- **C — column-scoped CHECK storage.** `applyInColumnChecks` in
  `batch_state_check.go` returns immediately on TiDB — the server never
  stores an in-column CHECK member (probed flag-ON: fresh name lands the
  column but no member, and the successor `DROP CHECK` reads the proven
  3940 absence). `inColumnChecksOutcome` refuses the whole MySQL statement
  when an in-column member references any column but its own (probed 3813;
  `NOT ENFORCED` does not relax the structural premise; self-column
  references land normally).
- **D — multi-action identity.** `prepareMultiAction` in
  `batch_state_multi.go` runs a statement-level refusal scan
  (`multiActionCheckActionRefused`, `multiActionIdentityCheckRefused`)
  before any action publishes: a CHECK-referenced RENAME/CHANGE evaluated
  through `checkColumnIdentityOutcome` against the pre-state (probed 3959
  atomic on 8.0.46 — sibling ADD never lands) or a syntax-gated action
  preserves the exact pre-state via `modifyPublication{replaceKey,
  replace: entry}`. Each action now returns the three-state
  `multiActionVerdict` (applied / refused→pre-state / unprovable→withheld).
  TiDB flag-ON `CHANGE COLUMN` cascades every provably-referencing CHECK
  member through `dropIdentityCascadedChecks` — the single-column
  `len==1` limit applies to `dropColumnCascadedChecks` (DROP COLUMN) only;
  the multi-column TiDB cascade is a probed fact, not an extrapolation.

## Red → green

- `red/baseline_red.log` — 12 `--- FAIL` markers, re-collected on the
  baseline product (`ebc9fbf0` tree = R1 product `72610833` plus the new
  R2 test file) after the original run output was not preserved — a real
  behavior-red re-capture, not a retained first run.
- Green: `go test ./internal/application/audit -run 'TestT12R2'` — 12/12
  pass on the product SHA; full `go test ./...` rc=0.

## Gates (bound to the product SHA)

18 groups under `gates/` — 17 standard gates plus the `three-level-doc-diff`
committed-diff group (each keeps the four-tuple
`command/rc/stdout/stderr`; `three-level-doc-diff.stderr.txt` is empty
because the diff command writes nothing to stderr). 17×rc=0; `lint` rc=1 —
the historical baseline shrank from 15 to 13 issues because R2 removed the
two `unparam` entries on `multiActionPremiseHolds`/`multiActionApply`
(signature cleanup, not suppression); zero hits in R2-touched code;
`gofmt` reports only the two accepted T07 baseline files. The
committed-diff range is `72610833..a4ce0e2d`.

## Golden (serial, one anchor set, bound to the product SHA)

`golden/` — T09 125/125 cases / 3442 assertions; T10 126/126 / 3175;
T11 40/40 / 506; T12 25/25 / 282 — all four identical to the R1 baselines
(no oracle changes required by R2). All four artifacts:
`head_sha` = `cli.build.head_sha` =
`a4ce0e2d6d5ae2dfa168273147ecdd53e33deea5`, identical `cli.sha256`
`71afae66a09f4dbec970ae224a33e6d554bc8991cc6046b6b1a02e6107bdc2f5`
across the four runs.

## Native probes

`docs/work-plans/t12-check-enforcement/probes/r2/` — 3 logs, each with the
exact SQL, per-statement rc, stderr, and before/after catalog state:

- `zz_r2p1_mysql57_attribution.log` — MySQL 5.7.44: `enforced` as a
  column/expression identifier parses and ignores (bare `CHECK(enforced>0)`
  lands the table with no catalog member); the authored `NOT ENFORCED`
  clause refuses 1064; a comment before the table body and a comment-
  contained comma do not confuse attribution.
- `zz_r2p2_mysql80_depend_atomic.log` — MySQL 8.0.46: generic
  `DROP CONSTRAINT \`primary\`` and `DROP PRIMARY KEY` both refuse 1553
  under a self-FK (PK stays); a sole-covering unique member refuses 1553
  while a replacement-coverer control drops clean; in-column
  `CHECK(v > id)` refuses 3813 atomically while the self-column control
  lands; multi-action `RENAME COLUMN a TO a2, ADD COLUMN c` and the
  `CHANGE COLUMN` form both refuse 3959 atomically (columns stay `a,b`,
  no `c`) while the unreferenced-column control lands `a,b2,c`.
- `zz_r2p3_tidb85_incolumn_change.log` — TiDB v8.5.0 flag-ON
  (`tidb_enable_check_constraint`): a fresh-name in-column CHECK lands the
  column but stores no member (`TIDB_CHECK_CONSTRAINTS` stays 0; the
  successor `DROP CHECK` reads 3940), a standalone `ADD CONSTRAINT` lands
  the member (catalog 1), and a single-action `CHANGE COLUMN` drops both
  multi-column and single-column referencing members (catalog 0).

An earlier B1c fixture and a D2 fixture in the first pass collided with
fixture errors (1822 missing referenced key, 3822 duplicate check name);
both were corrected and re-run — the archived log contains only the
corrected observations.

## Untested boundaries (kept unproven)

- TiDB generic-drop target resolution beyond the probed cells stays
  unclaimed (the R1 binding rule is preserved, not extended).
- `ALTER CHECK ... ENFORCED` post-state stays unvouched (data-conditional).
- TiDB multi-action CHANGE cascades are modeled only for the probed
  enforced arm with a provable reference set; undecidable reference/arm
  facts withhold publication.
- CHECK expression evaluation stays out of scope; only reference sets are
  modeled.

## Wording corrections applied during T12-R2-tail

The R2-tail round corrected the following descriptions without touching
the archived logs or re-running established evidence:

- `red/baseline_red.log` is a real behavior-red log **re-collected** on
  the baseline product after the original run output was not preserved —
  it is not a retained first run, and earlier wording suggesting
  otherwise was corrected above.
- The R2 test file mixes two test paths: public `Audit`-path cells
  (`t10Audit`, asserting report findings/gaps/coverage) and internal
  ordered-state cells (`t10r4PostState`/`t12r1PostStateFor`, asserting
  successor `Metadata.TargetTable` shape). Earlier wording implying every
  cell ran the public path and asserted every report field was overbroad.
- The probed TiDB CHANGE-cascade evidence is **single-action**
  `ALTER ... CHANGE COLUMN` on a multi-column CHECK reference. The
  `CHANGE + ADD` multi-action composition is covered by unit tests
  (`TestT12R2TiDBMultiActionChangeDropsMultiColCheck`), not by that probe.
- "The refusal does not depend on sibling order" (the R2 premise-scan
  comment) was overbroad: a same-statement proven `DROP CHECK` does lift
  the identity premise natively. R2-tail replaced the scan with
  `multiActionIdentityCheckVerdict`, which subtracts provably dropped
  members before judging; the R2 claim holds only absent such a sibling
  drop.
