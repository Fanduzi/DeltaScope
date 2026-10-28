# Decision: Partition topology and maintenance semantics (ADD/REORGANIZE/COALESCE/PARTITION BY/REMOVE + maintenance ops)

Date: 2026-11-07
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T14 under #93)
Related commits: this task's commits on the workfree line after `b54a0544`
Related tests: `internal/application/audit` corpus + T03 boundary tests,
`internal/infrastructure/metadata/mysql/provider_t13r2_test.go`,
`make ddl-golden TASK=T14 ARTIFACT_DIR=...`, `testdata/ddl-golden/T14.json`,
corpus fixtures `testdata/sql-corpus/{mysql,tidb}/ddl/findings/alter_table_partition_topology.sql`
Related docs: `docs/work-plans/t14-partition-topology/CONTRACT.md`,
native probe log `docs/work-plans/t14-partition-topology/probes/p13_topology_maintenance.log`,
module README updates in the same change

## Context

Before T14, `ALTER TABLE ... ADD/REORGANIZE/COALESCE PARTITION`,
`PARTITION BY`, `REMOVE PARTITIONING`, and the maintenance operations
(`CHECK`/`REBUILD`/`REPAIR`/`OPTIMIZE`/`DISCARD`/`IMPORT PARTITION`)
surfaced only per-action unsupported evidence — the inventory rows
`mysql.alter-table.partition-topology`, `mysql.alter-table.partition-maintenance`,
and `tidb.alter-table.partition-ops` sat at `parse_only`. The p13 probe
froze the anchor matrix on MySQL 5.7.44 / 8.0.46 / 8.4.10 and TiDB
v8.5.0 before the contract:

- REORGANIZE on a RANGE table executes on all four anchors (MySQL 8.x
  also accepts `ALGORITHM=INPLACE`); proven refusals: non-partitioned
  target 1505, TiDB hash/key method 8200, missing source 1507 (MySQL) /
  1567 (TiDB), non-consecutive sources 1519 (MySQL) / 8200 (TiDB),
  surviving-name collision 1517, bound violation 1520 (MySQL) / 1493
  (TiDB).
- ADD: non-partitioned 1505; `PARTITIONS n` on range/list 1492; member
  overflow 1499; name collision 1517; bound violations 1493/1481/1495;
  TiDB bare `ADD PARTITION` on a hash method is a no-op (not +1).
- COALESCE: non-partitioned 1505; range/list 1509; `n >= member count`
  1508.
- Maintenance splits into hard refusals and soft fails natively: REBUILD
  refuses 1505 on a plain table and 1507 on a missing member;
  DISCARD/IMPORT refuse 1505/1735; CHECK/OPTIMIZE/REPAIR return rc=0
  status-table rows on a non-partitioned table or missing member — a
  warning surface, never an error.
- TiDB refuses the whole maintenance set with 8200 (vendor boundary);
  `ANALYZE PARTITION` parses as `AnalyzeTableStmt`, not an alter action.

## Decision

1. **Normalized definition model on `spec`.** `PartitionBound` /
   `BoundLiteral` / `PartitionDef` / `PartitionScheme` /
   `PartitionMaintenance` carry the faithful declaration: method
   (bounded vocabulary incl. `range columns`/`list columns`), member
   names in clause order, `VALUES LESS THAN` literal/MAXVALUE rows, and
   `VALUES IN` literal rows. An item the bounded model cannot project
   keeps an expression mark with `Expression.Known=false` — never a
   guessed literal. `DDL.PartitionScheme` holds CREATE declarations;
   `Alter.PartitionDefs`/`PartitionMethod`/`PartitionNum`/
   `CoalesceNum`/`PartitionMaintenance` hold ALTER-side facts.
   `spec.CatalogPartitionDefs` projects the provider's
   `PARTITION_DESCRIPTION` text into the same comparable vocabulary so
   declared-vs-catalog bound reads share one field.
2. **Thirty frozen rules in three layers.** Eleven governance rules
   (`warn`/`notice`/`forbid` — `check_partition.warn` defaults notice
   because the action is a read-only status report;
   `discard_partition_tablespace.forbid` stays a dedicated blocker,
   never reduced to a generic warning), nine `.applicability` rules
   binding observed partition state to the probed refusal classes, and
   eight `.target` rules binding the member directory against named
   members. Soft-fail classes emit warning findings carrying
   `soft_fail`/`member_absent` metadata — never fabricated hard
   refusals. Every enabled metadata-bounded rule independently owes
   `target_table.partitioned`/`partition_type`/`partitions` gaps;
   disabling a rule removes the policy judgment, not the aspect or gap
   accounting.
3. **Ordered-state transitions.** `partition_transitions.go` resolves
   each action per bound target to a proven refusal (exact pre-state),
   a proven execution (modeled post-state), or an unproven outcome
   (member facts unvouch — directory nil, never a guessed set).
   REORGANIZE splices target defs into the proven contiguous source
   span with ordinals preserved; RANGE bound checks fire only when
   every relevant bound is comparable numeric/MAXVALUE. Maintenance
   actions preserve the directory and clear data-derived statistics.
   `CREATE`-derived shapes publish `PartitionScheme`-projected
   `PartitionDefs`/`Subpartitioned`.
4. **Coverage.** The five topology actions audit on both dialects; the
   six maintenance actions audit on MySQL and keep per-action
   vendor-boundary aspects on TiDB (proven 8200). Unprojectable bound
   items add `partition_bound`/`partition_expression` aspects so a
   boundary the model cannot compare never reports complete-by-omission.

## Boundaries kept explicit

- The locked parser cannot express `ALGORITHM=<v>|LOCK=<v>,
  REORGANIZE PARTITION ...` — the clause parses only as the leading
  spec. The probed modifier cells stay implemented but unreachable; no
  golden case is authored against a shape the parser cannot produce.
- No row routing, hashing, INTERVAL evaluation, or subpartition member
  modeling; a leaf-bearing catalog marks `Subpartitioned` and keeps the
  top-level directory unobserved.
- `ANALYZE PARTITION` stays outside the alter space; TiDB partition
  attributes/options stay unaudited (MySQL vendor boundary);
  IMPORT/EXPORT precondition artifacts are not generated.
- T15 modifier governance and temporary tables remain out of scope.

## Verification

- Golden: `make ddl-golden TASK=T14` — 59/59 cases, 917 assertions
  across mysql 5.7.44/8.0.46/8.4.10 + tidb v8.5.0, including the frozen
  first path (REORGANIZE p0→p1/p2 with live member-set and row-routing
  verification) and soft-fail, vendor-boundary, applicability, and
  counter-example cells.
- Corpus: MySQL/TiDB `alter_table_partition_topology` fixtures keep the
  rule/dialect target inventory at 100% (683/683).
- Gates: `go test ./...`, `make ddl-inventory-gate`,
  `make ddl-coverage-catalog-test`, MCP compact-list-rules regression.
