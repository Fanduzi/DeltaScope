# T14 — Partition Definition, Topology & Maintenance Semantics

Issue: [#93](https://github.com/Fanduzi/DeltaScope/issues/93). Parent: [#79](https://github.com/Fanduzi/DeltaScope/issues/79).
Baseline: `b54a0544a17ad2eb25bb58e51a0c371e57f68978` (T13-R3 closeout).
Frozen 2026-11-07. Native evidence: `probes/p13_topology_maintenance.log` (anchors: MySQL 5.7.44 / 8.0.46 / 8.4.10, TiDB v8.5.0) plus `probes/probe13_t14.sh`.

## 1. Scope & action inventory

T14 owns the partition actions the T13 contract left unaudited:

| AST action (`alterActionName`) | Form | Product reach |
|---|---|---|
| `add_partitions` | `ADD PARTITION (defs)` / `ADD PARTITION PARTITIONS n` / bare `ADD PARTITION` | both dialects |
| `reorganize_partition` | `REORGANIZE PARTITION src… INTO (defs)` | both dialects |
| `coalesce_partitions` | `COALESCE PARTITION n` | both dialects |
| `partition` | `ALTER TABLE t PARTITION BY scheme` (redefine) | both dialects |
| `remove_partitioning` | `REMOVE PARTITIONING` | both dialects |
| `rebuild_partition` | `REBUILD PARTITION names|ALL` | MySQL executes; TiDB refuses (8200) |
| `check_partitions` | `CHECK PARTITION names|ALL` | MySQL executes; TiDB refuses (8200) |
| `optimize_partition` | `OPTIMIZE PARTITION names|ALL` | MySQL executes; TiDB refuses (8200) |
| `repair_partition` | `REPAIR PARTITION names|ALL` | MySQL executes (InnoDB → rebuild map); TiDB refuses (8200) |
| `import_partition_tablespace` | `IMPORT PARTITION names|ALL TABLESPACE` | MySQL executes (needs exported tablespace precondition); TiDB refuses (8200) |
| `discard_partition_tablespace` | `DISCARD PARTITION names|ALL TABLESPACE` | MySQL executes (data-destructive); TiDB refuses (8200) |

Explicitly out of scope (recorded boundaries, stay unaudited aspects):

- `partition_attributes`, `partition_options` — TiDB-only grammar forms (`PARTITION p ATTRIBUTES`/`PARTITION p ENGINE|PLACEMENT POLICY`). Under MySQL they are **vendor boundaries** (native 1064); under TiDB they remain unaudited analysis. MySQL-side classification upgraded to `vendor=true` this round; no rule semantics added.
- `drop_first_partition`, `add_last_partition`, `reorganize_last_partition`, `reorganize_first_partition` — MariaDB INTERVAL forms the locked parser accepts but **no anchor grammar executes** (1064 everywhere). Dead action names; left alone.
- `ALTER TABLE t ANALYZE PARTITION …` — the locked parser normalizes this spelling to `AnalyzeTableStmt`, not an alter spec. It is outside the alter-action space entirely (parser quirk, recorded; the statement's existing coverage classification is unchanged).
- INTERVAL partitioning evaluation, subpartition member modeling, database partition algorithms (hash/consistent-hash member assignment) — never implemented; member sets that cannot be projected stay unknown.

## 2. Probed native matrix (all observations in `p13_topology_maintenance.log`)

### 2.1 REORGANIZE PARTITION src… INTO (defs)

- Executes on RANGE and LIST on all four anchors; **executes on HASH/KEY on all three MySQL anchors** (rename/merge; e.g. `REORGANIZE p0 INTO (q0)` → directory `[q0,p1,p2,p3]`). **TiDB refuses HASH/KEY with 8200** ("Unsupported reorganize partition").
- Source names must exist: MySQL 1507 "Error in list of partitions to REORGANIZE"; **TiDB 1567** "Incorrect partition name".
- Sources must be **consecutive**: MySQL 1519; TiDB 8200 ("not adjacent partitions").
- Target names must not collide with surviving members: 1517 both products.
- RANGE bound rules: targets must preserve the combined source range exactly — shrink **or** extension of a non-tail source refuses (MySQL **1520** "cannot change total ranges except for last partition where it can extend the range"; TiDB **1493** "VALUES LESS THAN value must be strictly increasing"). Reorganizing a MAXVALUE tail into `(<500, MAXVALUE)` executes on both.
- LIST reorg executes even when values are dropped (`pa IN (1) → px IN (9)` rc=0 both products; the dropped value's rows have nowhere to go — data-loss-by-omission is a *risk* the warn rule states, not a refusal).
- Non-partitioned target: 1505 both.
- Post-state (proven): targets replace the contiguous source span at the first source's ordinal; remaining members keep order. Verified positions: `p0→p1,p2` ⇒ `[p1,p2,pmax]`; `pm1,pm2→pz` ⇒ `[p0,pz]`; `p0,p1→n0,n1,n2` ⇒ `[n0,n1,n2,p2]` — identical on all four anchors.

### 2.2 ADD PARTITION

- `ADD PARTITION (defs)`: executes on RANGE, LIST **and HASH/KEY** on all four anchors; defs append at the tail ordinal (verified `p9` → position 5). Named-def-on-hash carries no value clause (`PartitionDefinitionClauseNone`).
- `ADD PARTITION PARTITIONS n`: executes on HASH/KEY (implicit `p<count>…` names appended); **refuses on RANGE/LIST with 1492** "each partition must be defined" (both products).
- Bare `ADD PARTITION` (no defs, no n): MySQL refuses — RANGE 1492, HASH **1514** "At least one partition must be added". TiDB refuses RANGE with 1492 but **silently no-ops (rc=0) on HASH** — a product asymmetry.
- Refusals: duplicate name 1517; new bound not above existing tail 1493; any add while last member is MAXVALUE 1481; LIST duplicate value 1495; non-partitioned 1505 — identical on all four anchors.

### 2.3 COALESCE PARTITION n

- Executes on HASH/KEY on all four anchors; removes the last `n` members (`p0..p(k)` survives in order).
- RANGE/LIST → **1509** "COALESCE PARTITION can only be used on HASH/KEY partitions" (both products).
- `n ≥ member count` → **1508** "Cannot remove all partitions" (both).
- Non-partitioned → 1505.

### 2.4 PARTITION BY redefine / REMOVE PARTITIONING

- `ALTER TABLE t PARTITION BY HASH(id) PARTITIONS 2` on a **non-partitioned** table executes on all four anchors; `PARTITION BY KEY(id) PARTITIONS 3` and `PARTITION BY RANGE…` on a partitioned table execute — new method + new member set installed, rows preserved. TiDB emits warning **1105** (statistics of new partitions outdated) — the statement still executes.
- `REMOVE PARTITIONING` executes on all four; table becomes non-partitioned, rows preserved (verified `id` 100/150 merged). On a non-partitioned table → 1505 (all four).

### 2.5 Maintenance ops

MySQL (all three anchors, InnoDB fixtures):

| Op | Named form | Missing name | Non-partitioned | Data effect |
|---|---|---|---|---|
| REBUILD | rc=0 | **1507** "Error in list of partitions to REBUILD" | 1505 | rows preserved; stats kept |
| CHECK | rc=0, status table | **soft-fail**: rc=0 + `Error in list of partitions` / `Operation failed` rows | soft-fail status rows (rc=0) | none |
| OPTIMIZE | rc=0 | soft-fail status rows (`Op` reported as `analyze`) | — | rows preserved; MySQL re-analyzes → stats refreshed |
| ANALYZE* | rc=0 | soft-fail status rows | — | stats refreshed |
| REPAIR | rc=0 (`repair status OK` — InnoDB maps to rebuild) | soft-fail status rows | — | InnoDB rebuild-equivalent |
| DISCARD … TABLESPACE | rc=0 | **1735** unknown partition | — | partition data detached/destroyed; member retained (dir row keeps `PARTITION_NAME` with stale `TABLE_ROWS`) |
| IMPORT … TABLESPACE | **1812** tablespace-missing precondition error with valid fixture | **1735** | — | loads external tablespace into partition |

* `ALTER TABLE t ANALYZE PARTITION` parses as `AnalyzeTableStmt` — see §1 boundary note; the observed rows above come from the equivalent maintenance statement for completeness. ANALYZE is not part of the alter-action rule set.

TiDB v8.5.0: REBUILD/CHECK/OPTIMIZE/REPAIR/IMPORT/DISCARD → **8200 "Unsupported … partition"** (vendor refusal; ANALYZE PARTITION runs — 1105 hard error on a missing name).

### 2.6 Modifier combinations (ALGORITHM/LOCK + T14 action)

| Combo | MySQL 5.7 | MySQL 8.x | TiDB 8.5 |
|---|---|---|---|
| `ALGORITHM=DEFAULT` + any | combinable (observed ADD) | combinable | combinable |
| `ALGORITHM=INPLACE` + REORGANIZE | 1846 refused | **executes** | 1064 grammar |
| `ALGORITHM=COPY` / `LOCK=NONE` + REORGANIZE | 1846 | **1507** refused | 1064 grammar |
| `ALGORITHM=INPLACE` + ADD | 1846 | executes | executes with error-level warning row 1846 recorded (`ALGORITHM=INPLACE is not supported… Try ALGORITHM=INSTANT`); partition lands |
| `LOCK=NONE` / `ALGORITHM=DEFAULT` + ADD | 1846 / executes | executes | executes (rc=0) |
| `ALGORITHM=INPLACE` + COALESCE | 1846 | executes | executes (rc=0) |
| `ALGORITHM=INPLACE` + REBUILD | 1846 | executes | 8200 |
| modifier + REMOVE PARTITIONING | 1064 grammar | 1064 grammar | 1064 grammar |
| modifier + PARTITION BY | 1064 grammar | 1064 grammar | 1064 grammar |
| partition action + table-option/structural/second-partition sibling | 1064 grammar | 1064 grammar | 1064 grammar |
| modifier + vendor-refused maintenance op (TiDB) | — | — | refused (1064/8200) |

Unprobed modifier value×action cells stay **unproven**.

### 2.7 Known product asymmetries & anomalies (recorded, modeled as observed)

- TiDB `ANALYZE PARTITION` is reachable and executes; every other named maintenance op is a vendor refusal.
- TiDB bare `ADD PARTITION` on HASH: rc=0 silent no-op; MySQL refuses 1514. Modeled per product.
- MySQL `CHECK/OPTIMIZE/REPAIR/ANALYZE` on a missing member produce an **error-status result table at rc=0** — a soft failure, never a hard refusal, and no state change either way.
- TiDB `ALGORITHM=INPLACE, ADD PARTITION` executes the partition add while recording an error-level warning for the modifier.

## 3. Normalized facts (spec additions)

```go
// PartitionDef — one declared partition member definition.
type PartitionDef struct {
    Name    string          // as written, lower-cased
    Ordinal int             // 1-based declaration position
    Bound   *PartitionBound // nil: no value clause (HASH/KEY members)
}

type PartitionBound struct {
    Kind     string          // "less_than" | "in"
    Exprs    []BoundLiteral  // LESS THAN per-column bounds, in order
    Values   [][]BoundLiteral // VALUES IN member rows
    Complete bool            // every clause expr projected to a literal/maxvalue
}
type BoundLiteral struct {
    Kind string // "number" | "string" | "null" | "maxvalue" | "unmodeled"
    Text string // normalized literal text for number/string; never raw SQL
}
```

- `spec.DDL` gains `PartitionDefs []PartitionDef`, `PartitionExpr *ExpressionFacts`, `PartitionColumns []string` (RANGE/LIST COLUMNS member names). `PartitionNames` stays the member-name directory (unchanged contract; nil = unprojectable member space: subpartitioned/INTERVAL/unbounded-n).
- `spec.Alter` gains `PartitionDefs []PartitionDef` (ADD/REORGANIZE targets and PARTITION BY members), `PartitionCount *uint64` (`PARTITIONS n` / `COALESCE n` authored count), `PartitionType string` + `PartitionScheme *PartitionScheme` (`partition` action's authored method/defs/expr/count/columns).
- `spec.TableSnapshot` gains `PartitionDescriptions []string` — provider-observed `information_schema.PARTITIONS.PARTITION_DESCRIPTION` aligned by ordinal with `PartitionNames`; nil = unobserved. Provider query extended to select `partition_description`.

Completeness rules mirror `partitionMemberNames`: defs list is nil exactly when the member space is unprojectable (subpartitioned members, INTERVAL declarations, `PARTITIONS n` above the 8192 bound). An unmodeled clause expression yields `BoundLiteral{Kind:"unmodeled"}` and `Complete=false` — never a fabricated literal.

## 4. Ordered-state publication

Dispatch: `isPartitionAlterAction` grows to the eleven audited actions. The existing multi-spec matrix (`partitionMixedOutcome`) gains a per-action modifier column per §2.6; unprobed cells stay unproven; `partition_attributes`/`partition_options` siblings land in the existing default-refused arm only where grammar-refusal applies — otherwise they remain unaudited aspects that invalidate via the unaudited-statement fallback (not partition actions).

Per action, given `entry.state == tablePresent`:

| Action | Proven refusal (exact pre-state) | Proven apply (publish) | Unproven (minimal unvouch) |
|---|---|---|---|
| `reorganize_partition` | partitioned=false (1505); TiDB hash/key (8200); missing source (1507/1567); non-consecutive sources (1519/8200); surviving-name collision (1517); provably-invalid bounds (1520/1493) | method applicable + sources present & consecutive + no collisions + bounds provably valid ⇒ replace contiguous span with target names at first-source ordinal | any premise unknown → `PartitionNames=nil` only |
| `add_partitions` | partitioned=false (1505); `PARTITIONS n` on range/list (1492); bare form on MySQL (1492/1514); name collision (1517); provable bound violation (1493/1481/1495); count overflow 8192 (1499) | defs form: names proven non-colliding + bounds provably valid ⇒ append defs names; `PARTITIONS n` on hash/key ⇒ append `p<count>…`; TiDB bare-on-hash ⇒ no-op (dir unchanged) | unknowns → `PartitionNames=nil` |
| `coalesce_partitions` | range/list (1509); partitioned=false (1505); n≥count (1508) | hash/key + n<count ⇒ drop last n names | unknowns → `PartitionNames=nil` |
| `partition` (PARTITION BY) | none observed — executes on partitioned and non-partitioned alike; provably-invalid inner defs (1493-class) refuse | `Partitioned=true`, `PartitionType=scheme type`, `PartitionNames=projected member names` (nil when unprojectable); stats cleared | invalidity unproven ⇒ all partition facts unvouched (`Partitioned`,`PartitionType`,`PartitionNames`,`PartitionDescriptions` → nil) |
| `remove_partitioning` | partitioned=false (1505) | `Partitioned=false`, partition facts cleared (known non-partitioned); stats cleared | `Partitioned` unknown ⇒ partition facts unvouched |
| `rebuild_partition` | missing named member (1507); non-partitioned (1505); TiDB → vendor refusal (statement-level) | dir unchanged; data preserved; **stats kept** | member/dir unknown → nothing changes (no stat unvouch — rebuild preserves data) |
| `check_partitions` | none — soft-fail rows only (MySQL); TiDB vendor refusal | dir+stats unchanged | ditto |
| `optimize_partition` | none hard — soft-fail on missing names (MySQL); TiDB vendor refusal | dir unchanged; **stats cleared** (native re-analyzes) | unknown member-set on MySQL ⇒ stats unvouched (cleared) |
| `repair_partition` | none hard — soft-fail on missing names; TiDB vendor refusal | dir unchanged; stats: proven InnoDB → kept (rebuild map), engine unknown/MyISAM → cleared | unknown member-set ⇒ stats unvouched |
| `discard_partition_tablespace` | missing named member (1735); non-partitioned; TiDB vendor refusal | dir unchanged (member retained — verified); **mutation stats cleared** (data destroyed) | unknown member-set ⇒ stats cleared |
| `import_partition_tablespace` | missing named member (1735); non-partitioned; TiDB vendor refusal | dir unchanged; **mutation stats cleared** | unknown member-set ⇒ stats cleared |

Structural facts (columns/indexes/constraints/PK) are untouched by every partition op. Rule enable/disable never changes state-machine facts. A proven-refused topology op keeps the exact pre-state including statistics; an unproven topology op unvouches only what it could have changed (never column/index/PK facts).

## 5. Coverage aspects

- `auditedAlterActionsMySQL` += all eleven actions. `auditedAlterActionsTiDB` += `add_partitions`, `reorganize_partition`, `coalesce_partitions`, `partition`, `remove_partitioning` (the five that execute on TiDB); the six maintenance ops stay non-audited on TiDB and emit `alter_table.<action>` **vendor** aspects (not unaudited).
- `partition_attributes`, `partition_options` emit vendor aspects under MySQL (upgraded from unaudited — they are TiDB-only forms; unchanged unaudited on TiDB).
- `PARTITIONS n` above 8192 or an unprojectable member set stays inside the existing completeness rule (gap, not fabricated directory).

## 6. Rule inventory (frozen IDs & defaults)

Governance findings — all fire on the authored action under the dialect where it can execute; none fire under a dialect that vendor-refuses the action:

| Rule ID | Default | Fires when |
|---|---|---|
| `ddl.alter.reorganize_partition.warn` | warning | authored action (risk: redistribution/rebuild; LIST value omission may drop data) |
| `ddl.alter.add_partition.warn` | warning | authored action (hash/key ⇒ full redistribution) |
| `ddl.alter.coalesce_partition.warn` | warning | authored action (removes members + redistributes) |
| `ddl.alter.partition_by.warn` | warning | authored action (full-table repartition) |
| `ddl.alter.remove_partitioning.warn` | warning | authored action (destroys layout, merges data) |
| `ddl.alter.rebuild_partition.warn` | warning | authored action |
| `ddl.alter.optimize_partition.warn` | warning | authored action |
| `ddl.alter.repair_partition.warn` | warning | authored action |
| `ddl.alter.check_partition.warn` | notice | authored action (read-only diagnostic) |
| `ddl.alter.import_partition_tablespace.warn` | warning | authored action |
| `ddl.alter.discard_partition_tablespace.forbid` | blocker | authored action (direct data destruction — own ID, never a generic warning) |

Semantic checks (metadata-decided; blocker default; `partition_absent`-family findings carry the dialect-correct native_error code):

| Rule ID | Checks |
|---|---|
| `ddl.alter.reorganize_partition.applicability` | non-partitioned target (1505); TiDB hash/key method (8200) |
| `ddl.alter.reorganize_partition.target` | missing source name (1507 mysql / 1567 tidb); non-consecutive sources (1519/8200); target name collides with surviving member (1517); provable bound violation (1520 mysql / 1493 tidb) |
| `ddl.alter.add_partition.applicability` | non-partitioned (1505); `PARTITIONS n` on range/list (1492); bare form on MySQL (1492/1514); member bound 8192 overflow (1499) |
| `ddl.alter.add_partition.target` | name collision (1517); provable bound violations (1493/1481/1495) |
| `ddl.alter.coalesce_partition.applicability` | non-partitioned (1505); range/list method (1509); n ≥ member count (1508) |
| `ddl.alter.partition_by.applicability` | provably-invalid declared scheme defs (1493-class) |
| `ddl.alter.remove_partitioning.applicability` | non-partitioned target (1505) |
| `ddl.alter.rebuild_partition.applicability` | non-partitioned (1505) |
| `ddl.alter.rebuild_partition.target` | missing named member (1507) — hard refusal |
| `ddl.alter.optimize_partition.applicability` | non-partitioned (1505) |
| `ddl.alter.optimize_partition.target` | missing named member — soft-fail (warning-level finding; status-table error, rc=0) |
| `ddl.alter.repair_partition.applicability` | non-partitioned (1505) |
| `ddl.alter.repair_partition.target` | missing named member — soft-fail (warning) |
| `ddl.alter.check_partition.applicability` | non-partitioned — soft-fail (warning) |
| `ddl.alter.discard_partition_tablespace.applicability` | non-partitioned (1505) |
| `ddl.alter.discard_partition_tablespace.target` | missing named member (1735) — hard refusal |
| `ddl.alter.import_partition_tablespace.applicability` | non-partitioned (1505) |
| `ddl.alter.import_partition_tablespace.target` | missing named member (1735) — hard refusal |

Gap semantics mirror T13: each enabled check owes `target_table.partitioned` / `partition_type` / `partitions` facts it consumes but the projection never answered; disabling a rule removes the policy judgment, never the aspect/gap accounting; soft-fail vs hard-refusal findings record `native_error` + `soft_fail=true` metadata where applicable.

Bound comparability contract: RANGE bound checks (1493/1520/1481) apply only when every involved bound literal is `number`/`maxvalue` (numeric ordering); LIST checks (1495) apply on literal-text equality of member values. Uncomparable bounds are `Complete=false` → the check is not owed a refusal decision; the statement-level bound facts are still recorded and the coverage gap marks the bound semantics unaudited (`alter_table.<action>.partition_bounds` aspect, unaudited).

## 7. First Golden Path (frozen oracle — #93 card)

Fixture: `golden.t14r` RANGE table, `p0 VALUES LESS THAN (200)` (+ `pmax MAXVALUE`), rows `id=100,150`. Statement:

```sql
ALTER TABLE t REORGANIZE PARTITION p0 INTO (
  PARTITION p1 VALUES LESS THAN (120),
  PARTITION p2 VALUES LESS THAN (200));
```

Policy: isolate `ddl.alter.reorganize_partition.warn` (and applicability/target enabled) on MySQL 8.0.46.

Frozen expectations:
- Extracted action: source `p0`, defs `p1`(ordinal1, less_than[number 120]) and `p2`(ordinal2, less_than[number 200]) in order.
- Findings: exactly `ddl.alter.reorganize_partition.warn` (warning) on the action; **zero** applicability/target findings; `coverage.status=complete`; verdict governed by warning threshold (reject at warning, per shared oracle table); CLI rc per threshold; no `alter_table.reorganize_partition` aspect.
- Successor check in the same batch (`ALTER TABLE t TRUNCATE PARTITION p1` under `ddl.alter.truncate_partition.target`): p1 proven present; `p0` no longer provably present.
- Native fixture: post-execute directory `p1(120),p2(200),pmax(MAXVALUE)` ordinals 1..3; `id=100→p1`, `id=150→p2`; total rows unchanged.
- Counter-example cells (must not pass as supported): overlap/shrink bounds (1520), missing source (1507), name collision (1517), non-consecutive sources (1519), TiDB hash method (8200), TiDB non-adjacent (8200).

`make ddl-golden TASK=T14 ARTIFACT_DIR=…` drives the real CLI case.

## 8. Deferred boundaries (explicit)

- The locked parser cannot express `ALGORITHM=<v>|LOCK=<v>, REORGANIZE PARTITION …` — the clause parses only as the leading spec. Native MySQL 8.x executes `ALGORITHM=INPLACE, REORGANIZE` (probed `mix-inplace-reorg` rc=0) and refuses `ALGORITHM=COPY`/`LOCK=NONE` (1507); MySQL 5.x refuses all non-default modifiers on partition ops (1846); TiDB parses the modifier+REORGANIZE form as a native 1064. The `partitionModifierOutcome` reorg cells therefore stay unreachable but correct — no golden case is authored against a shape the parser cannot produce, and the boundary is recorded here rather than silently dropped.
- Provider `PARTITION_DESCRIPTION` observation added for bound comparability; **no** row routing, hashing, INTERVAL evaluation, or subpartition member modeling.
- `ANALYZE PARTITION` lives outside the alter space (parser quirk); partition-level `ATTRIBUTES`/`OPTIONS` stay unaudited on TiDB, vendor boundary on MySQL.
- IMPORT/EXPORT tablespace precondition artifacts (`.cfg`/`.ibd`) are not generated; the supported-form classification stands on the 1812 precondition error + member-presence refusals.
- Soft-fail maintenance semantics (status-table rows, rc=0) are findings/notice facts, never hard refusals.
- T15 modifier governance, T16 temporary tables, out of scope.

## 9. T14-R1 remediation contract (2026-10-28)

R1 closes the independent-review defect groups A–D and records the evidence corrections E. Every judgment below is consumed identically by rules, `EvidenceGaps`, coverage aspects, and the ordered state; rule enable/disable never changes state-machine facts. Native oracle: `probes/probe14_t14r1.sh` → `probes/p15_t14r1_matrix.log` (292 bounded cells across mysql 5.7.44/8.0.46/8.4.10 + tidb v8.5.0).

### 9.1 Count safety (A1)

- Authored counts are `*uint64` end to end (`Alter.PartitionCount`, `DDL.PartitionCount`): explicit zero is preserved (`COALESCE PARTITION 0` reaches the semantic layer → 1515), nil = omitted, any count past 8192 refuses 1499 **before** conversion/append. `MaxInt64`/`MaxUint64` inputs can no longer wrap past the guard into an unbounded member loop; the giant-count cell runs only inside a resource-limited subprocess.
- `ADD PARTITION PARTITIONS 0` folds into the bare form — MySQL answers 1514 and TiDB a silent no-op for both spellings (probed), so the fold is observationally exact.
- `PARTITIONS 0` on CREATE/PARTITION BY and `PARTITIONS n` on range|list without definitions are **parser refusals** (1504/1492, `[ddl:1504]`/`[ddl:1492]`): they surface as parser diagnostics, never semantic findings, and are not claimed as implemented semantics.

### 9.2 Same-batch duplicate member names (A2)

- `firstDuplicateDefName` (case-insensitive) refuses 1517 on ADD defs, REORGANIZE INTO, PARTITION BY defs, and CREATE schemes; the statement keeps the exact pre-state — no partial directory, no retired members, stats untouched. p15 cells `add-dup-newname`/`reorg-dup-newname`/`pby-dup-newname`/`create-dup-name` → 1517 on all four anchors, directory unchanged. The older p13 "new name collides with an existing member" cells prove a different refusal and are not used as evidence here.

### 9.3 CREATE consumes the declared-scheme gate (A3)

- `applyCreateTable` runs the shared `partitionSchemeOutcome` before `derivedCreateShape`: refused → the table never publishes; unproven → the key unvouches; valid → the projected member directory publishes. `IF NOT EXISTS` on an existing table keeps the probed no-op. Golden `t14r1-mysql80-create-decreasing-stays-absent` pins the absent post-state; native `create-range-decreasing`/`create-list-dupvalue`/`create-dup-name`/`create-maxvalue-mid` cells prove the refusal classes (1493/1495/1517) keep the table absent.

### 9.4 Bound fidelity and comparison domains (B1–B4)

- `PartitionBoundMaxValue` keeps MAXVALUE at its authored tuple position — `(10,MAXVALUE)` projects `[numeric,maxvalue]`, matching `PARTITION_DESCRIPTION` (`10,MAXVALUE`, probed). An all-MAXVALUE row normalizes to the scalar +∞ tail; a per-item MAXVALUE inside a tuple never does.
- String bounds compare only byte-equal. Ordering across differing bytes under an unobserved collation is unproven — no Go byte-order substitute; the member directory unvouches instead of publishing or refusing on guessed order.
- `InBoundsOverlap` is tri-state: proven overlap (1495), proven disjoint, unproven (expression items, unprojectable catalog shells, nil-vs-empty-row mismatches). "Unknown" never folds into "disjoint".
- ADD carries its own bound-refusal code 1493 (p13 `add-low-bound`); REORGANIZE span-shrink keeps 1520 on MySQL / 1493 on TiDB.

### 9.5 Current-statement gaps and coverage (C1–C2)

- `add_partition.target` owes `target_table.partitions` / `target_table.partition_defs` whenever the directory is unvouched — defs-form on HASH/KEY and RANGE/LIST alike; a known-empty directory stays gap-free. Zero-finding + zero-gap + complete on an unvouched directory is impossible.
- Expression bound items — evaluated or not — are capability aspects on every declaration channel: `alter_table.<action>.partition_bound` for ADD/REORGANIZE defs and `PartitionScheme.Defs`, `create_table.partition_bound` for CREATE defs. Provider-side unprojectable bound shells stay evidence gaps on the owning rule. Feature IDs normalize to the rule vocabulary (`add_partition`, `check_partition`, `coalesce_partition`, `partition_by`), not the extractor's plural spellings.
- Native anchors fold constant bound expressions (`VALUES IN (1+0)` → 1495, `LESS THAN (100+100)` → 1493 on all four anchors — p15). The product deliberately does not evaluate expressions and reports the capability aspect instead — a documented stricter-than-native boundary, not a fabricated verdict.

### 9.6 Vendor / engine / scope consistency (D1–D4)

- The six TiDB vendor-refused maintenance actions share one gate — including DISCARD's dedicated forbid rule; governance rules mint no finding on a vendor-refused statement, whether the rule is on or off. T13 DROP/TRUNCATE/EXCHANGE rule behavior is unchanged.
- A vendor-refused statement is fully adjudicated, not a coverage gap: `fullyAuditedStatement` admits the probed vendor aspects, so the refused statement preserves the exact pre-state — directory, columns, indexes, PK, constraints, statistics — and successors still bind proven members (golden `t14r1-tidb85-vendor-check-preserves`). Coverage aspects are not removed to fix state; they stop being treated as unaudited.
- DISCARD/IMPORT consume the engine fact: proven MyISAM → 1031 (MySQL 5.7 anchor); unobserved engine owes `target_table.engine`; InnoDB is the positive control. MySQL 8.x refuses MyISAM native partitioning at CREATE itself (1178 — recorded boundary, the 5.7 anchor owns the 1031 oracle).
- OPTIMIZE PARTITION metadata records the InnoDB-observed whole-table scope ("All partitions will be rebuilt and analyzed", p13 `optimize-warnings`), replacing the named-member phrasing; the data-derived-statistics clearing mechanism is unchanged.

### 9.7 Golden and evidence corrections (E)

- `T14.json`: 68 required cases (59 frozen cells + 9 R1 cells). T09–T13 inputs/expectations/denominators are frozen: 125/3442, 126/3175, 40/506, 25/282, 47/697.
- Correction of the 62→59 history: the first 62-cell run failed 12 cells for **three distinct reasons** — expectation corrections (notice-vs-verdict wording, metadata key sets) and three genuine parser-boundary cells. The earlier "all twelve are parser boundaries" summary was wrong. The three removed cells targeted `ALTER TABLE … ALGORITHM=INPLACE, REORGANIZE PARTITION …`, `… ALGORITHM=COPY, REORGANIZE …`, `… LOCK=NONE, REORGANIZE …` on mysql80 — natively-valid or natively-refused syntax the locked parser cannot express (§8); they are recorded as a parser boundary, never counted as implemented semantics. The pre-edit 62-cell manifest file was not archived — that artifact is honestly missing; the surviving record is this note plus the §8 boundary entry.
- Correction of the "unified 1512" phrasing: T14 actions never shared one code — p13 shows ADD defs on hash|key **executes** on all four anchors and REORGANIZE on hash|key executes on MySQL while TiDB refuses 8200; 1512 belongs to DROP PARTITION on hash|key (T13 evidence). The archived p13 log is not rewritten.

## 10. T14-R2 remediation contract (2026-10-28)

R2 restores the frozen rule-ownership split for ADD bound checks, makes
undecidable comparisons explicit evidence gaps on the current statement, lets
bounded partition adjudication reach the public entry without weakening the
fail-closed default, and conditions OPTIMIZE's scope claim on proven engine
facts.

### 10.1 ADD ownership restoration (A)

- `ddl.alter.add_partition.target` owns the observed-directory bound contract
  again: non-increasing RANGE boundary (1493), adding after a MAXVALUE tail
  (1481), and provable LIST value duplication against existing members (1495).
- `ddl.alter.add_partition.applicability` keeps the declared-form duties added
  in R1: `ADD PARTITIONS n`/`ADD PARTITION (...)` form validity (1492), member
  count overflow (1499), and same-batch duplicate member names (1517). It no
  longer emits the bound-contract findings.
- `Evaluate` and `EvidenceGaps` consume the same shared adjudication
  (`spec.LessThanPairOutcome`, `spec.InBoundsOverlap`, `firstDuplicateDefName`);
  enabling both rules does not duplicate findings, and disabling a rule removes
  its own gap without erasing the other rule's gap.

### 10.2 Undecidable comparisons are current-statement gaps (B)

- `BoundComparable` certifies shape, not decidability. An undecidable
  comparison now produces an `EvidenceGap` on the owning rule instead of a
  false complete/pass. The gap vocabulary follows the premise owner:
  ADD/REORGANIZE undecidable comparisons against the observed directory
  report `reason_code=unknown_table_state` with
  `target_table.partition_defs`; PARTITION BY undecidable comparisons
  against the declared member set report
  `reason_code=unproven_statement_fact` with `statement.partition_defs`.
- A provable violation later in the same def list is not swallowed by an
  earlier undecidable member: `applyAddPartitionDefs` and
  `declaredSchemeOutcome` adjudicate pairwise and keep the provable finding.
- Approved oracle correction: `t14r1-mysql80-string-domain-unproven` now runs
  under `t14r2-add-truncate-target` and expects one
  `unknown_table_state` gap on `add_partition.target` for the current
  statement (was wrongly `coverage=complete` under the applicability profile).
- Approved oracle correction: `t14r1-mysql80-rcol-decreasing-tuple` now runs
  under `t14-add-target` and expects the 1493 finding owned by
  `add_partition.target` (was wrongly attributed to applicability). Case IDs,
  fixtures, and native semantics are unchanged; prior artifacts are not
  rewritten.

### 10.3 Bounded partition adjudication reaches the public entry (C)

- `fullyAuditedStatement`/`fullyAuditedCreateTable` route single partition
  actions through `partitionStateAdmissible`: the only coverable unknown is a
  partition-bound/vendor aspect; every other unmodeled aspect stays
  fail-closed.
- A proven same-batch duplicate name (1517) refuses the whole ALTER and
  preserves the exact pre-state — catalog, columns, indexes, PK, checks, and
  statistics — even when an unmodeled expression aspect is also present.
- `CREATE TABLE IF NOT EXISTS` on a known-present table stays a no-op before
  partition-body aspects are adjudicated; existing table facts are preserved.
- An undecidable ADD degrades only the partition directory/definition facts
  and partition-dependent statistics; table existence, columns, indexes, PK,
  checks, `Partitioned`, and the known partition scheme survive. Successors
  then owe directory facts, not table existence.

### 10.4 OPTIMIZE scope wording (D)

- Proven InnoDB keeps the full-table rebuild/reanalysis wording.
- Any other proven engine keeps named-member scope.
- Unproven engine metadata emits the conditional wording ("depends on the
  storage engine") instead of asserting a scope, consistent with the
  `target_table.engine` gap contract.

### 10.5 Golden and evidence corrections (E)

- `T14.json`: 68 required cases — 59 frozen cells + 9 R1 cells, two of which
  carry the approved R2 ownership corrections above (IDs retained).
- Red baseline `t14r2/red/go-test-t14r2.stdout.txt` is archived verbatim;
  failure counts are reported per-dimension (FAIL markers, unique leaf paths,
  unique parent tests) and never summed.

### 10.6 Single-member undecidable cannot pass on zero comparisons (T14-R2-tail)

- `declaredSchemeOutcome` accumulates two separate premises: (A) each
  member's own bound projection/completeness satisfies the adjudicable
  condition for the current method, and (B) the member set satisfies the
  pairwise ordering/non-overlap condition. A one-member scheme runs (A)
  even though no pair exists — a nil bound, an empty bound shell, or an
  expression bound marks the scheme unproven instead of combinable.
- Undecidable members accumulate without an early return: a later
  provable pairwise violation still refuses and preserves the exact
  pre-state. HASH/KEY members without bounds keep their original
  combinable outcome.
- Consequences reach the public ordered state: ALTER `PARTITION BY`
  keeps table existence and unrelated structural facts while unvouching
  the partition facts; CREATE on a known-absent target stays unproven —
  no proven directory and no proven-absent guess. Successors owe the
  actually-unknown facts (e.g. `target_table.partitioned`), never
  complete/pass on a fabricated directory. Rule enable/disable does not
  change the state conclusion.
- Golden: `t14r2tail-mysql80-pby-single-expr-member` pins the
  single-member → successor regression on a live anchor (manifest now
  69 required cases).
