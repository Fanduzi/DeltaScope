# Decision: Partition count fidelity, declaration validity, bound domains, and vendor/engine gating (T14-R1)

Date: 2026-10-28
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task
T14-R1 remediation under #93)
Related commits: this task's commits on the workfree line after
`14850fb2`
Related tests: `internal/application/audit/batch_state_t14r1_test.go`
(79 subtests through the public `AuditSQL` seam),
`make ddl-golden TASK=T14 ARTIFACT_DIR=...`, `testdata/ddl-golden/T14.json`
(68 cases after this change)
Related docs: `docs/work-plans/t14-partition-topology/CONTRACT.md` §9,
native probe `docs/work-plans/t14-partition-topology/probes/probe14_t14r1.sh`
→ `probes/p15_t14r1_matrix.log` (292 bounded cells),
`docs/decisions/2026-11-07-ddl-partition-topology-maintenance.md`

## Context

T14's independent review (A–E groups) found that the first delivery kept
the per-action surface but broke the shared premise chain in four places:

- **A1 (signed arithmetic gated an unbounded append):**
  `applyAddPartitionCount` computed `base + int(count)` in signed space —
  `PARTITIONS 9223372036854775807` wrapped negative, skipped the 8192
  guard, and entered an unbounded member-append loop (red run: 9.4 GB /
  175 % CPU inside a resource-limited subprocess). The extractor also
  dropped authored zero (`Num>0`), so `COALESCE PARTITION 0` lost its
  refusal class 1515, and CREATE-side `PARTITIONS` was never preserved.
- **A2 (same-batch duplicates unchecked):** duplicate names inside one
  ADD/REORGANIZE/PARTITION BY clause were never compared, so a natively
  refused 1517 statement could publish a partial post-state.
- **A3 (CREATE bypassed the scheme gate):** `declaredSchemeOutcome` ran
  only for ALTER `PARTITION BY`; a provably-invalid CREATE scheme
  published the table anyway.
- **B (bound fidelity):** any `MaxValueExpr` collapsed the whole bound
  to +∞ and erased tuple position; `BoundRowCompare` used
  `strings.Compare` as a collation substitute; `listBoundOverlaps`
  folded "unknown" into "disjoint"; ADD bound refusals carried REORG's
  1520 instead of its own 1493.
- **C (gap/coverage divergence):** `add_partition.target` returned early
  on a nil directory so an unvouched catalog produced zero findings and
  zero gaps with verdict pass; `PartitionScheme.Defs` and CREATE defs
  were never scanned for expression bounds; feature IDs used the
  extractor's plural spellings (`add_partitions`) instead of rule names.
- **D (vendor/engine/scope):** TiDB DISCARD reused a T13 forbid rule
  without the vendor gate, minting a governance blocker on a 8200
  vendor refusal; `fullyAuditedStatement` counted vendor-boundary
  aspects as unaudited so a proven-refused TiDB maintenance statement
  tombstoned the entire pre-state; DISCARD/IMPORT ignored the engine
  fact (MyISAM → 1031); OPTIMIZE claimed named-member scope while p13
  recorded InnoDB's whole-table rebuild.

Native oracle for every disputed cell: `p15_t14r1_matrix.log` —
`add-dup-newname`/`reorg-dup-newname`/`pby-dup-newname`/`create-dup-name`
→ 1517, `coalesce-0` → 1515, `add-0`/`add-bare` → MySQL 1514 / TiDB
no-op (observationally identical fold), `add-8193`/`pby-hash-8193` →
1499, `partitions-0`/`range-counts` → parser refusals 1504/1492,
`myisam-*` → 1031 on 5.7 (CREATE 1178 on 8.x), `optimize-misam` →
"InnoDB: All partitions will be rebuilt and analyzed".

## Decision

1. **Unsigned authored counts end to end.** `Alter.PartitionCount` and
   `DDL.PartitionCount` are `*uint64`: nil = omitted, explicit 0
   preserved through `COALESCE PARTITION 0`, `*n > 8192` refuses 1499
   before any conversion or append. `ADD PARTITION PARTITIONS 0` folds
   to the bare form — matching both anchors' observed answers (MySQL
   1514, TiDB silent no-op). `PARTITIONS 0`/range|list `PARTITIONS n`
   without defs are parser refusals (1504/1492) recorded as
   diagnostics, never semantic findings.
2. **One declaration-validity judgment shared by CREATE and ALTER.**
   `firstDuplicateDefName` (case-insensitive) plus the bound/member
   checks run identically on `PartitionScheme.Defs`, ADD defs,
   REORGANIZE INTO defs, and CREATE `PartitionDefs`. A refused CREATE
   scheme never publishes the table; `IF NOT EXISTS` keeps the probed
   no-op pre-state.
3. **Faithful tuple bounds and honest comparison domains.**
   `PartitionBoundMaxValue` keeps MAXVALUE at its authored position so
   `(10,MAXVALUE)` projects `[numeric,maxvalue]` — exactly what
   `PARTITION_DESCRIPTION` stores; only an all-MAXVALUE row normalizes
   to the +∞ tail. String literals compare byte-equal only; ordering
   under an unobserved collation is unproven, and `InBoundsOverlap`
   returns tri-state (proven overlap / proven disjoint / unproven).
   ADD bound refusals carry 1493; REORGANIZE keeps 1520 (MySQL) / 1493
   (TiDB).
4. **Gaps and coverage consume the same facts as the state machine.**
   `add_partition.target` owes `target_table.partitions`/
   `partition_defs` whenever the directory is unvouched (defs-form on
   HASH/KEY and RANGE/LIST alike; a known-empty directory stays
   gap-free). Expression bound items emit capability aspects on every
   declaration channel — `alter_table.<action>.partition_bound`
   covering `PartitionScheme.Defs` plus `create_table.partition_bound`
   — while provider-unprojectable shells stay evidence gaps on the
   owning rule. Feature IDs normalize to the rule vocabulary.
   Native anchors fold constant expressions (`1+0` → 1495); the product
   reports the capability aspect instead of evaluating — a documented
   stricter boundary.
5. **Vendor refusals are adjudicated facts, not coverage holes.** All
   six TiDB vendor-refused maintenance actions share one gate,
   including DISCARD's forbid rule; governance rules mint nothing on a
   vendor refusal whether on or off. `fullyAuditedStatement` admits the
   probed vendor aspects so a refused statement preserves the exact
   pre-state for successors. DISCARD/IMPORT consume the engine fact:
   MyISAM → 1031, unobserved engine owes `target_table.engine`, InnoDB
   is the positive control. OPTIMIZE metadata records the InnoDB
   whole-table rebuild scope.

## Boundaries kept explicit

- Expression bound items are never evaluated — even where the anchors
  fold constants, the product's unproven-classification stands and is
  recorded as a capability aspect, not a verdict guess.
- No collation engine: string bounds that are not byte-identical stay
  unproven rather than borrowing Go byte order.
- The locked-parser boundary for `ALGORITHM|LOCK …, REORGANIZE` and the
  parser refusals 1504/1492 remain parser diagnostics — recorded, not
  promoted to implemented semantics.
- Tablespace artifact generation, INTERVAL evaluation, and
  subpartition modeling remain deferred (T15+). Row routing beyond
  the frozen REORGANIZE golden path (already verified there with
  live member-set and row-routing reads) also remains deferred.

## Verification

- Red-first: `batch_state_t14r1_test.go` (79 subtests, none skipped)
  failed on the unremediated product — the archived log counts 66
  FAIL markers / 65 unique failing paths / 18 unique parent tests
  (the earlier "36 cells" figure is withdrawn; see the R2 evidence
  bundle's E2 recount) — including a
  resource-limited subprocess that recorded the 9.4 GB unbounded append
  — then went fully green after remediation.
- Golden: `make ddl-golden TASK=T14` — 68/68 cases, 1065 assertions,
  `head_sha` bound to the product commit; T09–T13 denominators frozen
  (125/3442, 126/3175, 40/506, 25/282, 47/697).
- Gates: `go test ./...` exit 0; the original 18-gate set reruns on the
  final SHA per the task's closing workflow.
