# Decision: Partition catalog completeness, bounded comparison, and modifier coverage (T13-R2)

Date: 2026-11-05
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task
T13-R2 remediation under #92)
Related commits: this task's commits on the workfree line after
`b41b1f7d`
Related tests:
`internal/application/audit/batch_state_t13r2_test.go`,
`internal/application/audit/extract_partition_facts_t13r2_test.go`,
`internal/domain/rule/ddl/partition_applicability_rules_t13r2_test.go`,
`internal/infrastructure/metadata/mysql/provider_t13r2_test.go`,
`make ddl-golden TASK=T13 ARTIFACT_DIR=...`, `testdata/ddl-golden/T13.json`
(45 cases after this change)
Related docs: `docs/work-plans/t13-partition-risk/CONTRACT.md` §10,
`docs/work-plans/t13-partition-risk/probes/` (p9–p11 archives),
`docs/decisions/2026-11-04-ddl-partition-applicability-and-ordered-state-remediation.md`

## Context

T13-R1 independent review found four same-family defects the R1
commit did not close:

- **A (directory post-state):** `applyPartitionOperation` cloned the
  pre-state `PartitionNames` verbatim after DROP, so a successor
  `TRUNCATE PARTITION p0` on a dropped member still proved the old
  directory. Provider and extractor also projected only top-level
  members: a subpartitioned or `RANGE INTERVAL` declaration published
  a "complete" directory that was actually partial, and
  `PARTITIONS 18446744073709551615` reached an unbounded allocation
  (`makeslice: cap out of range` panic).
- **B (comparison validity):** the counterpart column comparison
  compared raw `Length` sentinels, so declared `CHAR` vs catalog
  `CHAR(1)` fabricated a 1736 mismatch; `primaryKeyIdentityFacts`
  reopened a proven PK absence whenever the wider constraint set was
  unobserved; the target rules' `EvidenceGaps` returned empty on
  unvouched primaries and silently depended on the sibling
  applicability rule; and the #92-required engine premise was missing
  entirely.
- **C (mount + coverage):** `metadata.go` assigned
  `Metadata.ExchangeTable` but the mount condition omitted the field,
  so an exchange-only statement dropped its metadata; `coverage.go`
  removed the `algorithm`/`lock` unsupported aspect for EVERY
  partition-adjacent modifier, laundering `ALGORITHM=COPY` and
  version-missing `INPLACE`+DROP into `complete`.
- **D (evidence):** the archived TiDB VALIDATION section was a
  1049-missing-database run and the TiDB post-log had no independent
  initial state; `ALGORITHM=INPLACE` + TRUNCATE/EXCHANGE had no
  archived arms at all (the p7 README overstated its scope).

## Decision

1. **Exact post-state, never republished pre-state.** A named
   `DROP PARTITION` on a complete observed directory removes exactly
   the named members via `dropPartitionMembers`; an unobserved or
   incomplete directory stays `nil` (a DROP is not permitted to
   invent member proof); TRUNCATE/EXCHANGE keep names; a
   proven-refused statement mutates nothing. Dropping the final
   member leaves the pre-state untouched — the native refusal
   (1505-class) is represented, not simulated.
2. **Completeness is a first-class fact.** `partitionMemberNames`
   returns `nil` for subpartitioned declarations
   (`partition.Sub != nil`), `RANGE ... INTERVAL` declarations
   (`partition.Interval != nil`), zero `Num`, and any `Num` above the
   native bound (8192, `maxPartitionMembers`; MySQL 1499 probed on
   8.0.46). `derivedCreateShape` stops collapsing `nil` into an empty
   slice. The provider reads `SUBPARTITION_NAME` alongside
   `PARTITION_NAME`; any non-null leaf name marks the top-level
   directory incomplete (`PartitionNames = nil`) — no rule can claim
   a leaf member proven-absent. TiDB interval catalogs are exempt
   from this conservatism only through the provider path, because the
   catalog itself materializes `P_LT_*` names.
3. **Effective-type comparison.** The counterpart rule compares
   column name/order, effective type family, relevant length and
   signedness: integer display widths (`INT(10)` vs `INT(11)`,
   proven compatible on 5.7 in p9) and defaulted lengths (`CHAR` ≡
   `CHAR(1)`, plus DECIMAL/NUMERIC/BIT defaults) are normalized;
   bounded faithful-type parameters keep their text. The global
   `Length` sentinel contract is unchanged — normalization lives
   inside the rule. Evaluate and EvidenceGaps share one
   `exchangeComparisonFacts` walk, so a reported refusal and an owed
   fact can never disagree about what was compared; a proven column
   mismatch reports independently of PK knownness.
4. **PK absence is honored, not reopened.** `primaryKeyIdentityFacts`
   distinguishes `primary == nil` (unvouched → that side's
   `*_table.primary_key` gap) from `PrimaryKey == nil &&
   !PrimaryKeyUnknown` (proven absence — never reopened by
   `ConstraintsUnknown`).
5. **Independent gap ownership.** Each enabled `*.target` rule owes
   `target_table.partitioned` (unvouched or unobserved partitioned
   state) and `target_table.partitions` (partitioned proven, member
   directory unobserved/incomplete) for the members it consumes — no
   borrowing from the sibling applicability rule. `ALL` consumes no
   member name and owes nothing.
6. **Engine is a MySQL-only EXCHANGE premise.** Both engines observed
   and different → `counterpart_engine_mismatch` (1497, probed
   5.7/8.0/8.4). Unobserved engine facts are owed per side
   (`target_table.engine`, `exchange_table.engine`). TiDB normalizes
   the ENGINE label in the catalog (MyISAM declaration stores InnoDB
   and exchange executes, p9) — there is no observable mismatch
   class, so the premise is not consumed there. The p8 1064/8200
   `ENGINE=` mixed-alter refusals are a grammar/multi-change
   mechanism, not evidence about the two tables' engines.
7. **Exchange-only metadata mounts.** The mount condition includes
   `metadata.ExchangeTable != nil`: a statement whose only fact is
   the batch-declared counterpart still reaches the rule.
8. **Coverage reflects the classifier verdict.** The unsupported
   `algorithm`/`lock` aspect is removed only when
   `partitionCombinationProven` — the same pure classifier the
   ordered state consumes, fed by the statement's enriched version
   fact — returns combinable or refused. `COPY`, non-`DEFAULT` lock
   values, multiple modifiers, and version-dependent forms without
   the version fact keep the aspect on the current statement
   (`incomplete` coverage, `ErrUnsupportedStatement`), never a
   successor-tombstone laundering. Standalone modifiers remain
   T15-owned.

## Deferred scope

Row-content validation for EXCHANGE, subpartitioned member operations
(the directory stays provably-unknown rather than modeling leaf
ops), INTERVAL expression evaluation, `placement_policy` and
multi-modifier combinations (tombstone), unprobed modifier values,
and all T14/T15 scope remain unmodeled. The explicit `WITH
VALIDATION` vs absent-clause AST distinction stays collapsed — it
matches native semantics (p10: absent ≡ WITH on TiDB 1737).
