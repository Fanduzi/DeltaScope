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
