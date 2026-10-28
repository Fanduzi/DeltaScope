# Decision: ADD bound-ownership restoration, current-statement undecidable disclosure, and bounded state admissibility (T14-R2)

Date: 2026-10-28
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task
T14-R2 remediation under #93)
Related commits: this task's commits on the workfree line after
`17539dbb`
Related tests: `internal/application/audit/batch_state_t14r2_test.go`
(public `AuditSQL` seam; current/successor `Metadata.TargetTable`
assertions), `make ddl-golden TASK=T14 ARTIFACT_DIR=...`,
`testdata/ddl-golden/T14.json` (68 cases, two approved oracle
corrections)
Related docs: `docs/work-plans/t14-partition-topology/CONTRACT.md` §10,
`docs/decisions/2026-10-28-ddl-partition-topology-r1-remediation.md`

## Context

T14-R1's independent review (A–E groups) found that the remediation
itself introduced or exposed four contract breaks:

- **A (frozen ownership migrated):** R1 moved the ADD bound contract
  out of `ddl.alter.add_partition.target` into the applicability rule.
  With only the target rule enabled, a provably non-increasing RANGE
  ADD produced zero findings, zero gaps, and verdict pass.
- **B (shape substituted for decidability):** several `EvidenceGaps`
  paths treated `spec.BoundComparable` or an outer non-nil slice as
  proof that a comparison could be decided. RANGE/LIST COLUMNS string
  bounds, `PARTITION BY` string bounds, and catalog bound shells with
  missing rows therefore reported complete coverage on the current
  statement. An early-return tri-state loop could also let an
  undecidable member swallow a later provable violation in the same
  def list.
- **C (bounded adjudication unreachable):**
  `fullyAuditedStatement`/`fullyAuditedCreateTable` invalidated the
  state before the partition-specific adjudication could run, so a
  proven same-batch duplicate name still tombstoned the whole
  pre-state, `CREATE TABLE IF NOT EXISTS` on a known-present table
  invalidated known facts before the no-op check, and a bound-only
  undecidable ADD tombstoned unrelated structural facts.
- **D (OPTIMIZE scope asserted without engine facts):** the warn rule
  claimed named-member scope even when the storage engine was not
  proven.

## Decision

1. **Bound contract returns to the target rule.** `add_partition.target`
   again emits 1493 (non-increasing RANGE vs. catalog), 1481 (ADD after
   a MAXVALUE tail), and 1495 (provable LIST value duplication against
   existing members). The applicability rule keeps its R1 declared-form
   duties only: 1492 form validity, 1499 count overflow, 1517 same-batch
   duplicate names. Both rules consume the same shared predicates —
   `spec.LessThanPairOutcome`, `spec.InBoundsOverlap`,
   `firstDuplicateDefName` — so enabling both never duplicates a
   finding, and disabling a rule removes only its own gaps.
2. **Decidability is adjudicated, not inferred from shape.** A shared
   pairwise tri-state judgment (`LessThanPairOutcome` returns
   violates/undecidable per adjacent pair; `InBoundsOverlap` returns
   tri-state overlap) drives `Evaluate`, `EvidenceGaps`, coverage
   aspects, and ordered-state degradation from one result. Undecidable
   comparisons produce an `unknown_table_state` evidence gap on the
   owning rule with the missing premise fact; provable violations later
   in the def list still mint findings.
3. **Partition adjudication is admissible without weakening
   fail-closed.** `partitionStateAdmissible` admits a statement whose
   only unmodeled coverage aspects are partition-bound or probed
   vendor-refusal aspects; every other aspect still invalidates. Within
   that boundary: a proven refusal preserves the exact pre-state,
   `CREATE TABLE IF NOT EXISTS` on a known-present table is a no-op
   before body aspects are considered, and an undecidable ADD degrades
   only directory/definition facts and partition-dependent statistics —
   successors then owe `target_table.partitions`/`partition_defs`, not
   table existence.
4. **OPTIMIZE scope is conditional on proven engine.** InnoDB keeps
   the whole-table rebuild wording, other proven engines keep
   named-member scope, and an unproven engine emits the conditional
   wording consistent with the `target_table.engine` gap contract.

## Oracle corrections (approved)

- `t14r1-mysql80-string-domain-unproven`: the current statement now
  expects one `unknown_table_state` gap owned by `add_partition.target`
  under the new `t14r2-add-truncate-target` profile (was wrongly
  `coverage=complete` under the applicability-only profile). Case ID,
  fixture, and SQL unchanged.
- `t14r1-mysql80-rcol-decreasing-tuple`: the 1493 finding is now
  expected from `add_partition.target` under the existing
  `t14-add-target` profile (was wrongly attributed to applicability).
- Prior artifacts are not rewritten; corrections are recorded as
  `notes` on the manifest cells and in CONTRACT §10.5.

## Boundaries kept explicit

- Expression bound items are still never evaluated — an undecidable
  member degrades partition facts rather than guessing order.
- No collation engine: non-byte-identical string bounds stay
  undecidable and now correctly surface as gaps instead of pass.
- `partitionStateAdmissible` is deliberately narrow: any non-partition,
  non-vendor aspect keeps the statement fail-closed, and multi-action
  ALTER statements outside the single partition action are unchanged.
- Tablespace artifacts, row routing, INTERVAL evaluation, and
  subpartition modeling remain deferred (T15+).

## Verification

- Red-first: `batch_state_t14r2_test.go` produced 27 FAIL markers /
  11 failing top-level tests on the unremediated product
  (`t14r2/red/go-test-t14r2.stdout.txt`), covering A–D; all green after
  remediation. Six R1 test expectations were corrected to the restored
  ownership (fixture/ownership fixes, not product regressions).
- Golden: `make ddl-golden TASK=T14` — 68/68 cases, 1066 assertions on
  the remediated SHA; T09–T13 denominators frozen.
- Gates: `go test ./...` exit 0; the full gate set reruns on the final
  SHA per the closing workflow.
