# Decision: Partition-action applicability checks and ordered-state remediation (T13-R1)

Date: 2026-11-04
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task
T13-R1 remediation under #92)
Related commits: this task's commits on the workfree line after
`d65ac868`
Related tests: `internal/domain/rule/ddl/partition_applicability_rules_test.go`,
`internal/application/audit/batch_state_t13r1_test.go`,
`internal/application/audit/batch_state_t13r1_applicability_test.go`,
`internal/application/audit/extract_partition_facts_test.go`,
`make ddl-golden TASK=T13 ARTIFACT_DIR=...`, `testdata/ddl-golden/T13.json`
(38 cases after this change), corpus fixtures
`testdata/sql-corpus/{mysql,tidb}/ddl/findings/alter_table_partition_applicability.sql`
Related docs: `docs/work-plans/t13-partition-risk/CONTRACT.md` §9,
`docs/work-plans/t13-partition-risk/probes/` (p4–p8 archives)

## Context

The T13 baseline (890ae531) deliberately scoped `partition_rules.go` to
AST-only risk findings and declared member existence, partition-type
applicability, and EXCHANGE compatibility "never statically provable".
Independent acceptance found that scope reduction was never approved by
the #92 contract, plus two concrete defects:

- **B (counterpart staleness):** ordered state preloaded only the
  primary identity. `applyPartitionOperation` skips identities absent
  from `s.entries`, so an EXCHANGE counterpart never seen before kept
  its batch-prior provider statistics on first successor touch.
- **C (over-broad refusal):** `len(ddl.Alter) > 1 && hasPartitionAlter`
  preserved exact pre-state for ANY multi-spec ALTER — including
  `ALGORITHM=INPLACE, DROP PARTITION`, which probes showed executes on
  MySQL 8.0/8.4/TiDB (and refuses only on MySQL 5.x with 1846).
- **D (evidence):** the archived absent-validation arm was contaminated
  (it ran after WITHOUT had emptied the counterpart); the corrected
  three-arm rerun proves the absent clause refuses violating rows
  exactly like `WITH VALIDATION` (MySQL 1526 / TiDB 1737). The
  `with`-covers-absent normalization stays correct.

## Decision

1. **Bounded partition catalog facts.** `spec.TableSnapshot` gains
   `Partitioned *bool` (now provider-loaded on MySQL AND TiDB),
   `PartitionType *string` (normalized `PARTITION_METHOD` vocabulary),
   and `PartitionNames []string` (catalog-order member names; nil =
   unobserved, non-nil empty = observed-empty). `spec.DDL` carries the
   same pair on CREATE TABLE so `derivedCreateShape` publishes batch
   knownness. The provider reads `information_schema.partitions`
   (method + name rows); an unanswerable probe keeps every fact nil —
   never fabricated. `spec.Metadata.ExchangeTable` carries the
   counterpart's ordered projection through the same once-per-key seam.
2. **Seven applicability rules, all default blockers, separate from
   risk policy.** `drop|truncate|exchange_partition.applicability` bind
   primary partitioned-state/type facts (1505 not-partitioned; 1512
   hash/key-gated DROP). `drop|truncate|exchange_partition.target`
   bind the observed member set against named partitions (1507/1735;
   the `ALL` form consumes no member name). `exchange_partition.
   counterpart` binds counterpart existence (1146), partitioned state
   (1732), and a bounded structural compare — column name/type sequence
   and PK identity only (1736); a clean compare never claims the engine
   validated all exchange conditions. Every rule implements
   `EvidenceGaps` and reports ONLY the facts it consumes: unproven
   premises surface as `unknown_table_state` gaps on the owning rule,
   `enabled:false` owes nothing, and policy findings stay
   metadata-independent.
3. **Ordered-state fixes.** `enrichStatements` preloads the EXCHANGE
   counterpart through `state.preState` with its own (possibly
   qualified) identity, so `applyPartitionOperation`'s per-identity
   post-state applies to both sides — first-touch successors see
   cleared data-derived statistics, never resurrected batch-prior ones.
   The refusal classifier replaces the blanket multi-spec rule:
   partition+structural or `table_option` siblings and multiple
   partition actions are proven refusals (exact pre-state); ONE
   preceding `algorithm`/`lock` modifier is probe-gated
   (`LOCK=DEFAULT`/`ALGORITHM=DEFAULT` combine everywhere;
   `ALGORITHM=INPLACE` combines except MySQL 5.x+DROP → 1846; version
   unknown stays unproven); anything else tombstones.
4. **ALGORITHM/LOCK extraction is partition-scoped.** `spec.Alter.
   Options` records `algorithm`/`lock` keyword values so the classifier
   can gate modifiers attached to partition actions; standalone
   modifier governance stays owned by T15.

## Deferred scope

Sub-partitioned DROP, bounds values, `PARTITIONS n` expression solving
beyond implicit `p0..p(n-1)` naming, `placement_policy` siblings, and
T14 topology actions (ADD/REORGANIZE/COALESCE/REMOVE PARTITIONING)
remain unmodeled — their cells report gaps or route through the
unaudited/tombstone paths, never fabricated verdicts.
