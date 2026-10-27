# Decision: Partition drop applicability gating, effective DECIMAL comparison, and PK proof separation (T13-R3)

Date: 2026-11-06
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task
T13-R3 remediation under #92)
Related commits: this task's commits on the workfree line after
`1f10ea99`
Related tests:
`internal/application/audit/batch_state_t13r3_test.go`,
`internal/domain/rule/ddl/partition_applicability_rules_t13r3_test.go`,
`make ddl-golden TASK=T13 ARTIFACT_DIR=...`, `testdata/ddl-golden/T13.json`
(47 cases after this change)
Related docs: `docs/work-plans/t13-partition-risk/CONTRACT.md` §11,
`docs/work-plans/t13-partition-risk/probes/` (p12 archive),
`docs/decisions/2026-11-05-ddl-partition-r2-remediation.md`,
`docs/decisions/2026-11-04-ddl-partition-applicability-and-ordered-state-remediation.md`

## Context

T13-R2 independent review closed the catalog post-state and comparison
premises it scoped, but three same-family defects remained:

- **A (directory publication ignored action applicability):**
  `applyPartitionOperation` removed members from the proven directory
  whenever the names were present and a member survived — without asking
  whether the action itself could have executed. A `DROP PARTITION p0` on
  a HASH- or KEY-partitioned table is a proven native refusal (1512,
  archived in p6), yet the successor still published a directory missing
  `p0`, so a following `TRUNCATE PARTITION p0` fabricated
  `partition_absent`/1735. The same hole fired when the partition method
  was unobserved: an undecidable drop was silently published as a
  successful deletion.
- **B (DECIMAL effective parameters):**
  `normalizeFaithfulColumnType` folded only the bare `decimal` spelling.
  `DECIMAL(12)` and `DECIMAL(12,0)` — the same effective type per the
  fixed-point default-parameter contract — still compared as verbatim
  text and fabricated a `counterpart_definition_mismatch`/1736.
- **C (PK presence vs member proof conflated):**
  `primaryKeyIdentityFacts` read `PrimaryKey != nil` as proven presence
  before checking `PrimaryKeyUnknown`. `markPrimaryUnknown` — a
  legitimate ordered-state shape — leaves the residual pointer attached
  while unvouching it, so an ambiguous `DROP INDEX` followed by EXCHANGE
  re-proved the unvouched key as present. Separately, a proven-present
  placeholder (`Kind=Primary, NameUnknown, Columns=nil`) was compared
  member-for-member against a real member list, so the unvouched
  sequence fabricated another 1736.

## Decision

1. **Directory publication consumes the action's own applicability.**
   `partitionDropApplicability` resolves DROP PARTITION against the
   observed method using the same vocabulary the rule side carries:
   `range|list` families combine, `hash|key` (incl. linear) refuse,
   anything unobserved or unmodeled stays undecidable. Proven refusal
   (method refusal, missing name, last member) keeps the exact
   pre-state — no stats cleared, no member touched. Undecidable
   unvouches the directory alone (`PartitionNames = nil`): neither a
   removal it cannot prove nor a retention it cannot vouch is
   published, and every unrelated structural fact stays. Rule
   enable/disable never changes ordered-state semantics.
2. **Effective DECIMAL/NUMERIC parameters.** The faithful-base
   normalizer folds spellings to `base(M,D)` against native defaults —
   bare is `(10,0)`, one parameter is `(M,0)` — after consuming the
   trailing `unsigned` attribute word (signedness stays its own
   dimension). Any faithful form the bounded parser cannot project
   returns unproven: the comparison owes the column fact instead of
   choosing a verdict. Declared and catalog spellings share the same
   walk, so declared `DECIMAL(12)` equals catalog `decimal(12,0)`.
   The T06 faithful-spelling contract is untouched — normalization
   stays inside the rule.
3. **PK presence and member sequence are separate proofs.**
   `primaryKeyIdentityFacts` returns presence, members, membersOK, and
   ok independently: `PrimaryKeyUnknown` outranks any residual pointer
   (presence itself unproven → that side's `*_table.primary_key` gap);
   a proven-present placeholder with nil members owes the same fact
   (presence answered, members unproven — a real key always has
   members, so nil is never an empty key); proven absence is still
   never reopened by an unrelated `ConstraintsUnknown`. The mismatch
   comparison consumes members only when both sides' sequences are
   vouched, while an independently proven presence-vs-absence pair
   still emits 1736 on its own.
4. **Bounded native confirmation only.** p12 archives the B-group
   equivalence pairs on mysql80 + mysql57 — `DECIMAL(12)`↔`DECIMAL(12,0)`,
   `DECIMAL`↔`DECIMAL(10,0)`, `DECIMAL(12) UNSIGNED`↔`DECIMAL(12,0)
   UNSIGNED`, `INT(10)`↔`INT(11)` all execute; `DECIMAL(12,1)`,
   `DECIMAL(11)`, and signedness-mismatched controls refuse 1736.
   The A-group 1512 refusal reuses p6; the C-group is a
   metadata-certainty regression verified through the public audit
   path, not a manufactured live catalog.

## Deferred scope

Row-content compatibility solving for EXCHANGE, member-level
subpartition/interval operations (gap, not finding), INTERVAL
expression evaluation, TiDB index-object attribution modeling (the
placeholder shape is consumed, not produced), T14 topology actions,
and T15 modifier governance remain unchanged scope. No provider query
or declaration-side contract changed.
