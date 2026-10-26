# Decision: CHECK-constraint lifecycle, enforcement state, and catalog loading

Date: 2026-10-30
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T12 under #91)
Related commits: this task's commits on the workfree line after `186710db`
Related tests: `internal/application/audit/batch_state_t12_check_test.go`,
`batch_state_t12_check_sequences_test.go`, `batch_state_t12r1_test.go`
(R1 remediation cells), provider `TestLoadTableSnapshotCheckConstraintMembers`,
`make ddl-golden TASK=T12 ARTIFACT_DIR=...`, `testdata/ddl-golden/T12.json`,
corpus fixtures under `testdata/sql-corpus/{mysql,tidb}/ddl/metadata/check_*`
Related docs: `docs/work-plans/t12-check-enforcement/CONTRACT.md`,
`docs/work-plans/t12-check-enforcement/probes/`, module README updates in the same change

## Context

Before T12, CHECK constraints were parse-only noise: `spec.Constraint`
carried no `Type: "check"` member, column-level `CHECK(...)` options
degraded to `UnextractedOptions`, `alter_check`/`drop_check` normalized but
had no audited coverage or ordered-state transition, the provider set
`ConstraintsUnknown` whenever a real CHECK existed, and naming rules judged
only CREATE-path `DDL.Constraints`. Four native anchors were probed before
the contract froze:

- MySQL 8.0.46/8.4.10: CHECKs store and enforce (3819 on violation);
  `ALTER CHECK|CONSTRAINT x NOT ENFORCED` is idempotent and deterministic;
  `ENFORCED` is data-conditional (3819 with violating rows); missing and
  non-check names refuse 3821; duplicate check names refuse 3822; both
  DROP keywords remove a present member; `DROP COLUMN` silently cascades
  referencing checks; `RENAME COLUMN`/renaming `CHANGE COLUMN` on a
  check-referenced column refuse 3959.
- MySQL 5.7.44: CHECK syntax parses but is silently dropped (never stored,
  never enforced); `ALTER|DROP CHECK` refuses 1064. Official boundaries:
  enforcement in 8.0.16, alter/drop syntax in 8.0.19.
- TiDB v8.5.0: `tidb_enable_check_constraint` gates everything — OFF parses
  and ignores, ON mirrors MySQL with 3940-class refusals; ON `CHANGE
  COLUMN` diverges (lands the rename, silently drops the referencing
  check). Catalog: members live only in `TIDB_CHECK_CONSTRAINTS`; no
  catalog column exposes enforcement — SHOW CREATE's `/*!80016 NOT
  ENFORCED */` marker is the only surface.

## Decision

1. **One normalized member, three declaration paths.** `spec.Constraint`
   gains `Type: "check"` with `Name`, `Columns` (expression local
   references), `Enforced` (`"enforced"|"not_enforced"|""` — empty is
   unvouched, not a third value), `InColumn`, `NameUnknown` for
   engine-generated names, and `Expression *ExpressionFacts`. CREATE table
   constraints, ALTER `add_constraint` payloads, and column-level CHECK
   options all land on it. `spec.Alter.Options["check_enforced"]` carries
   the ALTER CHECK direction; `InstanceFacts` gains the observed
   `TiDBEnableCheckConstraint` + `Known` pair.
2. **Provider reads two probed catalog shapes.** MySQL: `TABLE_CONSTRAINTS`
   + `ENFORCED`, clause joined from `CHECK_CONSTRAINTS` on
   catalog+schema+name (it has no `TABLE_NAME`). TiDB:
   `TIDB_CHECK_CONSTRAINTS` (has `TABLE_NAME`, no enforcement column), so
   TiDB members load with `Enforced` unvouched. Capability probes gate
   each shape; failures propagate; a real CHECK collection no longer
   unvouches `snapshot.Constraints`.
3. **Five new rules, stable IDs:** `ddl.constraint.check.not_enforced.notice`
   (declared/flipped NOT ENFORCED on real arms — skipped when the name
   provably refuses), `ddl.constraint.check.ignored.notice` (parse-but-
   ignore arms), `ddl.alter.alter_check.exists.require`,
   `ddl.alter.drop_check.exists.require` (refused-name blockers over the
   shared binding space), `ddl.alter.alter_check.version.require` (MySQL
   <8.0.19 syntax refusal, mysql-only). Unknown version/flag/member state
   emits attributable gaps — never fabricated pass/fail. CHECK naming
   rules now also see ALTER `add_constraint` names.
4. **Ordered state mirrors probed semantics.** `applyAlterCheck`:
   NOT ENFORCED → deterministic `enforced=not_enforced`; ENFORCED →
   data-conditional, member `Enforced` unvouches; refused/no-op arms keep
   or gate state per arm. `applyAddCheck`/`applyDropCheck` land and remove
   members per arm; `applyDropColumn` cascades referencing checks.
   `checkColumnIdentityOutcome` encodes the probed 3959-refusal arms and
   the TiDB-ON `CHANGE COLUMN` silent-cascade divergence — refused arms
   publish the exact pre-state, never a tombstone.
5. **No-leak boundary.** Raw `CHECK_CLAUSE`/literals never cross the
   provider or report boundary; only `ExpressionFacts`, `Enforced`,
   `Name`, `Columns` surface. T11 PK/UNIQUE/FK behavior is unchanged.

## Amendment: R1 acceptance remediation (2026-10-26)

Independent acceptance located five defects; each was re-probed natively
before being encoded (probe logs `probes/r1/`, public-path red baseline
`docs/verification/t12r1/red/baseline_red.log` — 13 `--- FAIL` markers on
the pre-remediation product, all green after):

1. **Authored spelling is now a recovered fact.** `ast.Constraint` drops
   whether the text wrote `CHECK` or `CONSTRAINT`; the extractor recovers
   it from the already-split clause text into
   `alter.Options["check_spelling"]`. The MySQL syntax floor splits per
   probed behavior — CHECK spelling executes from 8.0.16 (probed 8.0.17),
   the generic alias from 8.0.19 (probed 1064 on 8.0.17) — via
   `spec.CheckAlterSyntaxOutcome`. An unrecoverable spelling inside the
   8.0.16–8.0.18 band answers unproven, never a guessed floor.
2. **`Constraint.EnforcedExplicit` records only authored clauses.** MySQL
   5.7 parses a bare CHECK declaration but refuses an authored
   `[NOT] ENFORCED` clause with 1064 (probed on table-level, named,
   ADD-CONSTRAINT, and inline column forms); TiDB's ignored arm accepts
   and ignores it. `spec.CheckExplicitClauseRefused` gates CREATE/ADD
   publication so a refused statement lands neither table, column, nor
   member — the default `Enforced` value is never reverse-read as the
   clause's presence.
3. **One shared target classifier.** `spec.CheckDropTarget` /
   `spec.CheckAlterTarget` (`internal/domain/spec/check_target.go`)
   is consumed identically by findings, evidence gaps, and ordered state:
   CHECK spelling binds only check members; the generic keyword counts
   check+unique+PK+FK and refuses a cross-type same-name pair with the
   probed 3939 (MySQL) while TiDB-ON binds the check member first
   (probed). Secondary indexes never bind (probed 3821/3940), and an
   index member of unvouched kind downgrades generic binding to
   Unproven rather than Missing.
4. **Statement-atomic premises.** ADD CHECK requires every modeled
   referenced column to exist (probed 1054); a duplicate in-column CHECK
   name refuses the whole statement on MySQL (probed 3822 — the added
   column never lands) while TiDB-ON silently ignores the colliding
   member and lands the column (probed). `DROP COLUMN` cascades only a
   member whose proven reference set is exactly the dropped column;
   multi-column references refuse 3959 (probed) and unvouched reference
   sets stay unsafe.
5. **ALTER inline CHECKs join naming governance.** `alter.Constraints`
   members are read by the `check.name.*` applicability and selection
   paths, deduplicated against names already declared elsewhere.

Boundaries kept unproven: unrecoverable spelling in the 8.0.16–8.0.18
band, unknown-kind index members under generic binding, TiDB generic-drop
resolution beyond the probed cells, and MySQL patches other than
5.7.44/8.0.17/8.0.46/8.4.10.

## Amendment: R2 acceptance remediation (2026-10-26)

Reviewer groups A–D located four defect clusters against the R1 contract;
each was re-probed natively (`probes/r2/` — 5.7.44 attribution, 8.0.46
dependency/atomicity, TiDB v8.5.0 flag-ON storage/change edges) and
encoded after a public-path red baseline:

1. **Clause attribution, not keyword presence.** An authored
   `[NOT] ENFORCED` clause is proven only in binding position — the first
   bare word after the `CHECK(...)` close paren of the element/clause text
   already aligned to its AST spec. `ENFORCED` as a column or expression
   identifier never qualifies (probed 5.7: identifier form ignored,
   authored clause 1064). Comment/quote-aware splitting keeps `/* , */`
   and `/* ( */` from faking boundaries; unalignable text marks members
   `Constraint.EnforcedUnproven` and the version rule emits an evidence
   gap — a sibling action's spelling or clause fact is never copied.
   TiDB's flag-OFF arm keeps the ignored notice on authored clauses since
   suppression routes through the MySQL-only
   `spec.CheckExplicitClauseRefused`.
2. **Binding success does not imply landing.** A generic
   `DROP CONSTRAINT` bound to the primary object or a unique member runs
   the same FK sole-coverer dependency premise as the dedicated drops
   (probed 1553 on 8.0.46; a replacement coverer drops clean). Check +
   unknown-kind index same-name pairs keep binding unproven AND unvouch
   the check member's existence — both native worlds exist (secondary →
   dropped, unique → 3939). `PrimaryKeyUnknown` honors names that could
   bind the primary object as Unproven.
3. **Column-scoped CHECK storage.** TiDB never stores an in-column CHECK
   member — probed flag-ON: the column lands, the member does not
   (`DROP CHECK` then reads the proven 3940 absence) — so TiDB skips
   member publication there entirely. MySQL in-column CHECKs reference
   only their own column; a cross-column reference is the probed 3813
   whole-statement refusal that `NOT ENFORCED` does not relax.
4. **Multi-action identity goes through the CHECK verdict.**
   `prepareMultiAction` scans every action for a proven refusal
   (CHECK-referenced RENAME/CHANGE via `checkColumnIdentityOutcome`
   against the pre-state — probed 3959 atomic, or a syntax-gated action)
   before publishing anything; a refusal preserves the exact pre-state,
   an unprovable action withholds the successor. TiDB flag-ON `CHANGE
   COLUMN` cascades every provably-referencing CHECK member — the
   single-column limit is a DROP COLUMN rule only; the multi-column
   cascade is now a probed fact.

## Amendment: R2-tail corrections (2026-10-26)

Reviewer tail items A/B/D located three residual defects against the R2
contract; each was re-probed natively (`probes/r2/zz_r2tail_*`) and
encoded after a red baseline (`docs/verification/t12r2tail/`):

1. **`--` is a comment only before whitespace/control.** Every `--`
   comment check in `extractor.go` now routes through
   `dashDashOpensComment`, matching the MySQL manual and the locked TiDB
   lexer's `startWithDash`: `--` opens a comment only at end-of-input or
   before a whitespace/control character. `v--1` is `v - -1`, so
   `CHECK(v--1>0) NOT ENFORCED` keeps its authored clause and refuses
   1064 on 5.7 — it can no longer masquerade as parsed-and-ignored.
2. **Undecidable generic drops unvouch the PK too.**
   `dropCheckUndecidable` evaluates the PK binding before its
   collection-vouching early return, so a target name that could bind
   the PK object unvouches the PK projection at member granularity — a
   proven-absent PK stays absent and unrelated names never lower
   certainty. Rule-layer gaps name the actually missing side through
   `checkBindingGapFacts` (`target_table.primary_key` alone when that is
   the only gap).
3. **A same-statement proven DROP CHECK lifts the identity premise.**
   Probed on 8.0.46: `DROP CHECK chk_a` plus `RENAME COLUMN a TO a2` or
   `CHANGE COLUMN a a2 INT` lands in either action order; an unrelated
   drop still refuses 3959 atomically. The premise scan subtracts
   provably dropped members (`multiActionDroppedCheckNames` +
   `multiActionIdentityCheckVerdict`) — a voided or unprovable answer
   withholds the successor instead of fabricating a refusal that keeps
   the old member and column.

## Consequences and deferred scope

- Expression evaluation stays out of scope — facts, not semantics.
- MySQL sub-anchor versions (<8.0.46) inherit only the documented
  8.0.16/8.0.19 boundaries; TiDB flag toggles between provider load and
  audit stay bounded by the observed-fact mechanism.
- The `ENFORCED` flip's data-conditional outcome is intentionally
  unvouched — a claimed post-state would be a fabricated success.
- A legacy `alter_table_constraints` corpus oracle and three T03-era
  "check unsupported" expectations were promoted per the contract's
  recorded old→new boundary; all other oracles stay frozen.
