# Decision: Bounded column-position ordering and dialect-split multi-action ALTER state

Date: 2026-10-28
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T08 under #87; extends `2026-10-02-ddl-ordered-schema-state-first-path.md`)
Related commits: this task's commits on milestone/mysql-tidb-ddl-completion
Related tests: `internal/application/audit/t08_column_governance_test.go`, `alter_compatibility_rules_test.go`, `make ddl-golden TASK=T08 ARTIFACT_DIR=...`, `scripts/test_t08_contract.py`
Related docs: `testdata/ddl-golden/T08.json`, `docs/work-plans/t08-column-governance/`, `internal/application/audit/README.md`, official references below

## Context

Before T08, the `column_position` aspect was parsed but unaudited: every
`FIRST`/`AFTER` clause produced an unsupported entry, and any multi-action
`ALTER TABLE` (`ADD COLUMN a, ADD COLUMN b` in one statement) tombstoned the
table because no ordered successor existed. T08's frozen contract
(`docs/work-plans/t08-column-governance/CONTRACT.md`) promoted both surfaces
to audited behavior after independent four-anchor native probing.

Two facts from the native matrix shaped the design:

- **Omitted `DEFAULT` is a clearing operation.** On MySQL 5.7.44, 8.0.46,
  8.4.10, and TiDB 8.5.0 alike, `MODIFY`/`CHANGE` that omits `DEFAULT`
  removes the stored default — so an effective-default comparison, not a
  syntactic presence check, is the correct transition test.
- **Multi-action premise evaluation is dialect-split.** MySQL validates
  actions left-to-right against accumulated state, so
  `ADD c1 INT, ADD c2 INT AFTER c1` succeeds. TiDB validates premises
  against the pre-state, so the same statement fails natively with 1054 —
  and the position-existence rule must mirror that split or fabricate a
  false blocker on MySQL / a false pass on TiDB.

## Decision

1. `spec.Alter` carries `Position *spec.AlterPosition` populated by the
   extractors. `FIRST` is an ordering operation with no existence premise;
   `AFTER <col>` requires the referenced column to exist and is governed by
   the new `ddl.alter.column_position.exists.require` rule.
2. Single-action ADD/MODIFY/CHANGE with a position now apply a bounded
   reorder to the published successor; a `CHANGE COLUMN a a2 ... AFTER c`
   both renames and repositions.
3. A bounded multi-action successor (`batch_state_multi.go`) handles an
   `ALTER TABLE` whose member actions are disjoint, fully audited, and
   position-capable: MySQL applies actions against accumulated working
   state; TiDB validates premises against the pre-state while still
   applying the same working successor. Any repeated primary object,
   unprovable member, or failed premise tombstones the whole statement —
   never a published prefix. Loaded foreign-key dependents are invalidated
   alongside the published successor exactly as in the single-action path;
   dependent invalidation does not gate publication.
4. Parenthesized multi-column `ADD (a, b) FIRST|AFTER` is recorded as an
   unextracted spec-level `column_position` option on the first emitted
   alter only — the native anchors resolve the position for the group, but
   DeltaScope does not yet model per-group placement, so it stays a gap
   rather than a duplicated per-column fact.
5. The modify/change compatibility rules compare *effective* default
   classes (`none|literal|null|current_timestamp|expression`), so omitted
   `DEFAULT` correctly surfaces a `literal → none` transition warning, and
   undecodable source expression defaults surface a bounded
   `source_column.default` evidence gap instead of a fabricated finding.
6. New rules `ddl.alter.set_default.exists.require`,
   `ddl.alter.drop_default.exists.require`, and
   `ddl.alter.column_position.exists.require` gate their respective targets;
   unknown state produces evidence gaps, never fabricated absence.

## Consequences

- `column_position` left the unsupported set — T03 coverage cases moved out
  of the unsupported table and the T05/T07 expectations were upgraded per
  `EXPECTED-UPGRADES.md`.
- The frozen T08 manifest binds 171 cases (162-role denominator minus the
  unit-admitted cancel role, plus 10 baseline anchor-health cases) across
  all four anchors; `scripts/t08_contract.py` is the independent oracle —
  the manifest is regenerated from it and never edited by hand.
- Deferred: per-group position modeling for parenthesized multi-column ADD,
  expression-default fingerprint comparison beyond the catalog class, and
  same-object multi-action sequences (always refused upstream anyway).

## Verification

- `go test ./internal/application/audit -run 'TestBatchStateT08|TestAuditSQLT08'`
- `python3 scripts/test_t08_contract.py` (constructed valid control + 24
  adversarial mutations, all rejected)
- `make ddl-golden TASK=T08 ARTIFACT_DIR=...` — 171/171 cases, 4210
  assertions, PASS on MySQL 5.7.44 / 8.0.46 / 8.4.10 / TiDB 8.5.0.

## References

- MySQL 8.4 `ALTER TABLE` (multiple operations, FIRST/AFTER):
  https://dev.mysql.com/doc/refman/8.4/en/alter-table.html
- TiDB `ALTER TABLE` compatibility notes:
  https://docs.pingcap.com/tidb/stable/mysql-compatibility/#ddl
- Contract and frozen role matrix: `docs/work-plans/t08-column-governance/`
