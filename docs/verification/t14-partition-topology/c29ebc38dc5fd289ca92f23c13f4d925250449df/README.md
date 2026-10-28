# T14-R1 evidence bundle — c29ebc38

Independent-acceptance evidence for T14-R1 (#93 remediation): the
A/B/C/D defect groups closed under the shared premise contract, plus
the E-group evidence corrections. Product commit
`c29ebc38dc5fd289ca92f23c13f4d925250449df` (parent `14850fb2`, the
T14 evidence-closeout candidate commit — acceptance pending).
Contract: CONTRACT.md §9;
ADR: `docs/decisions/2026-10-28-ddl-partition-topology-r1-remediation.md`.

## Scope delivered

- **A — count safety & declaration validity.** Authored counts are
  `*uint64` end to end (`Alter.PartitionCount`, `DDL.PartitionCount`):
  `COALESCE PARTITION 0` keeps its authored zero → 1515; `>8192`
  refuses 1499 before any conversion or append loop; `MaxInt64` /
  `MaxUint64` inputs can no longer wrap past the guard into an
  unbounded member loop (the giant-count cell runs only inside a
  resource-limited subprocess). `ADD PARTITION PARTITIONS 0` folds
  into the bare form — matching MySQL's 1514 and TiDB's no-op
  observational identity. `PARTITIONS 0` and range|list
  `PARTITIONS n` without defs are parser refusals (1504/1492)
  recorded as diagnostics. `firstDuplicateDefName` (case-insensitive)
  refuses 1517 on ADD/REORGANIZE/PARTITION BY/CREATE same-batch
  duplicates, atomically. `applyCreateTable` now consumes the shared
  declared-scheme gate: a refused CREATE scheme never publishes table
  existence; `IF NOT EXISTS` keeps the probed no-op.
- **B — bound fidelity & comparison domains.**
  `PartitionBoundMaxValue` keeps MAXVALUE at its authored tuple
  position so `(10,MAXVALUE)` projects `[numeric,maxvalue]` —
  identical to the catalog's `PARTITION_DESCRIPTION` text; only an
  all-MAXVALUE row normalizes to the +∞ tail. String bounds compare
  byte-equal only — ordering under an unobserved collation is
  unproven, and the member directory unvouches rather than borrowing
  Go byte order. `InBoundsOverlap` is tri-state (proven overlap →
  1495 / proven disjoint / unproven); expression items never fold
  into "disjoint". ADD bound refusals carry their own 1493;
  REORGANIZE keeps 1520 (MySQL) / 1493 (TiDB).
- **C — gaps & coverage consistency.** `add_partition.target` owes
  `target_table.partitions`/`partition_defs` whenever the directory
  is unvouched (defs-form on HASH/KEY and RANGE/LIST alike); a
  known-empty directory stays gap-free — zero-finding + zero-gap +
  complete is impossible on an unvouched catalog. Expression bound
  items emit capability aspects on every declaration channel
  (`alter_table.<action>.partition_bound` including
  `PartitionScheme.Defs`, and `create_table.partition_bound`);
  provider-unprojectable shells stay evidence gaps on the owning
  rule. Feature IDs normalize to the rule vocabulary.
- **D — vendor/engine/scope consistency.** All six TiDB
  vendor-refused maintenance actions share one gate — including
  DISCARD's dedicated forbid rule; governance rules mint nothing on a
  vendor refusal whether on or off. `fullyAuditedStatement` admits
  the probed vendor aspects so a refused statement preserves the
  exact pre-state for successors (columns, indexes, PK, constraints,
  directory, statistics). DISCARD/IMPORT consume the engine fact:
  MyISAM → 1031 (MySQL 5.7 anchor), unobserved engine owes
  `target_table.engine`, InnoDB is the positive control. OPTIMIZE
  metadata records the InnoDB-observed whole-table rebuild scope
  ("All partitions will be rebuilt and analyzed").

## Red baseline

`red/` — captured on the unremediated product `b5b2ac70` before the
R1 fixes landed. `go-test-t14r1.*` records the real failure run of
`batch_state_t14r1_test.go`: 66 FAIL markers / 65 unique failing
paths / 18 unique parent tests / 47 unique paths containing "/" /
50 terminal failing paths across all four groups (the earlier "36
failing cells" summary is withdrawn — see the R2 bundle's E2
recount)
— including missing 1515/1499/1517/1031 findings, the TiDB DISCARD
forbid minting a governance blocker on a vendor refusal, the
`check_partitions` plural feature id, and the unvouched-directory
successor states. The `count_9223372036854775807` cell ran in a
resource-limited subprocess and was recorded at 9.4 GB RSS / 175 %
CPU — the unbounded-append defect's own runtime signature — before
the unsigned guard existed. No failure is compile-driven; every cell
is a behavioral miss on the public `AuditSQL` seam.

## Gates

`gates/` — 19 required gates + 2 committed-diff L2 replays + 1
full-repo lint diagnostic, all on `c29ebc38`, each with real argv,
stdout, stderr, and an rc saved immediately after exit. See
`gates/INDEX.md`: 17 required green, `lint-scoped` at exactly the
13-issue accepted baseline (file-set identical to the b5b2ac70
record), `gofmt` listing only the two accepted T07 baseline files,
`lint-fullrepo` recorded verbatim as a 37-issue diagnostic outside
the accepted scope. The committed-diff L2 replays cover both the
original T14 product range (`b54a0544..b5b2ac70`, 38 files — two
findings recorded and closed this round) and the R1 product range
(`14850fb2..c29ebc38`, 20 files).

## Golden

`golden/` — six serial suite runs on `c29ebc38`, each artifact
recording `head_sha=c29ebc38dc5f` and its own CLI sha256:

| Task | cases | assertions | result |
|------|-------|------------|--------|
| T09 | 125/125 | 3442 | PASS |
| T10 | 126/126 | 3175 | PASS |
| T11 | 40/40 | 506 | PASS |
| T12 | 25/25 | 282 | PASS |
| T13 | 47/47 | 697 | PASS |
| T14 | 68/68 | 1065 | PASS |

T09–T13 inputs/expectations/denominators are frozen and unchanged.
T14 grew 59 → 68 required cases — the nine R1 cells authored through
the public CLI/golden contract (overflow count, duplicate names,
CREATE scheme refusal, COLUMNS MAXVALUE, string-domain unproven,
LIST expression, TiDB vendor pre-state preservation, MyISAM engine
gate, OPTIMIZE scope). The 62→59 history is corrected in CONTRACT.md
§9.7: the original twelve failures split into expectation corrections
(notice/verdict wording, metadata keys) and three genuine
parser-boundary cells (`ALGORITHM|LOCK …, REORGANIZE`); the pre-edit
62-cell manifest was not archived and is recorded as honestly
missing.

## Native evidence

`native/p15_t14r1_matrix.log` — 292 bounded cells across
mysql 5.7.44 / 8.0.46 / 8.4.10 + tidb v8.5.0, generated by
`probes/probe14_t14r1.sh` (committed): same-batch duplicate names →
1517 on all four anchors; `COALESCE 0` → 1515; `ADD PARTITIONS 0` /
bare `ADD PARTITION` → MySQL 1514 / TiDB no-op; `PARTITIONS 8193` →
1499; `PARTITIONS 0` / range `PARTITIONS n` without defs → 1504/1492;
RANGE COLUMNS `(10,MAXVALUE)` stored per-item in
`PARTITION_DESCRIPTION`; utf8mb4_general_ci string bounds accepted
where byte order would disagree; `1+0` → 1495 / `100+100` → 1493
(native expression folding the product deliberately does not
emulate); MyISAM DISCARD/IMPORT → 1031 on 5.7 (8.x refuses MyISAM
partitioning at CREATE — 1178, recorded version asymmetry);
`golden` is created per-anchor inside the loop so no 1049/1146
contamination survives in this archive. The earlier p13/p14 logs
remain committed alongside and are not rewritten.

Unproven/unobserved boundaries kept explicit: the product never
evaluates bound expressions (capability aspect, stricter than the
native fold); string ordering without a proven collation domain is
unproven; tablespace artifact generation, INTERVAL evaluation,
subpartition modeling, and row routing beyond the frozen REORGANIZE
golden path (already verified there with live member-set and
row-routing reads) remain deferred; the
`ALGORITHM|LOCK …, REORGANIZE` parser boundary and the 1504/1492
parser refusals are recorded diagnostics, not implemented semantics.
