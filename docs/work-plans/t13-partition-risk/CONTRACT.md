# T13 partition DROP/TRUNCATE/EXCHANGE risk — implementation contract

Status: FROZEN for T13 implementation. `testdata/ddl-golden/T13.json` is the
materialized golden manifest. Internal freeze is authorized by the task order;
it is not external acceptance. Same-contract implementation/test/collector
bugs may be repaired without reducing the frozen responsibilities or
denominator.
Owner: #92 under #79. Task base: T12 accepted head
`89725eafbd7284f631c84db3b95e0d0eb248a3a7` on `workfree` (T12 product
`a55fe2b8d4e03266a997f936718eb3349ce1cccf`, closure `dcf250ea`).

## 1. Scope and semantic seam

Promote the three destructive/data-moving partition alter actions from
parse-only unsupported aspects to audited, governed semantics:

- `ALTER TABLE t DROP PARTITION p0[, p1 ...]` — removes named partition
  definitions and their rows.
- `ALTER TABLE t TRUNCATE PARTITION {p0[, p1 ...] | ALL}` — empties rows but
  keeps the partition definitions.
- `ALTER TABLE t EXCHANGE PARTITION p0 WITH TABLE x [WITH|WITHOUT VALIDATION]`
  — swaps a partition's rows with a non-partitioned table's rows; both object
  identities participate.

T14 scope is excluded: partition definitions, `PARTITION BY`, ADD/REORGANIZE/
COALESCE/REMOVE PARTITIONING, and maintenance ops (REBUILD/OPTIMIZE/ANALYZE/
CHECK/REPAIR) stay unaudited aspects.

Existing seams reused: `spec.Alter` (new partition payload fields),
`spec.DDL.Targets`/`TableTargets()` (the exchanged table enters the named
object-identities list so the denylist and affected-identity tracking see
both sides), `auditedAlterActions{MySQL,TiDB}` (actions join the audited set),
`batchState` modifyPublication (ordered post-state), `clearMutationStats`
(data-derived fact invalidation), `tableDenylistRule` (protected-object path).

### 1.1 Native facts (probed 2026-10-27; anchors mysql:5.7.44, mysql:8.0.46,
mysql:8.4.10, pingcap/tidb:v8.5.0 — unistore)

Identical on all four anchors unless marked:

- RANGE fixture `t13(id,c) PARTITION BY RANGE(id) (p0<100, p1<200)` lands;
  inserts route to named partitions (`information_schema.PARTITIONS` lists
  p0/p1 with per-partition TABLE_ROWS).
- `DROP PARTITION p0` → rc=0; p0 definition and its rows gone; p1 and its
  rows survive.
- `DROP PARTITION p99` (missing name): on a 2-partition table → ERROR 1507
  "Error in list of partitions to DROP"; on a 1-partition table → ERROR 1508
  "Cannot remove all partitions". `DROP PARTITION p0, p1` removing every
  partition → 1508 refusal, all partitions kept.
- `DROP PARTITION` on a non-partitioned table → ERROR 1505.
- `DROP PARTITION` on a HASH-partitioned table → ERROR 1512
  (RANGE/LIST only).
- `TRUNCATE PARTITION p0` and `TRUNCATE PARTITION ALL` → rc=0; definitions
  kept, rows gone. Works on HASH partitions too.
- `EXCHANGE PARTITION p0 WITH TABLE t13x` → rc=0; rows swap in both
  directions. Schema-qualified `WITH TABLE golden.t13x` works.
- `EXCHANGE ... WITH TABLE t2` where t2's row violates the partition range:
  `WITH VALIDATION` → refused (MySQL: ERROR 1526 "Table has no partition
  for value"; TiDB: ERROR 1737 "Found a row that does not match the
  partition"). `WITHOUT VALIDATION` and the absent clause → rc=0 and the
  violating row lands inside the partition.
- `EXCHANGE` refusal classes that preserve pre-state: 1736 different
  definitions, 1732 partitioned exchange target, 1146 missing table,
  1735 unknown partition.
- `EXCHANGE` on a HASH-partitioned table → rc=0 (not RANGE/LIST-only like
  DROP) on both 8.0.46 and TiDB 8.5.0.
- A partition action in a multi-spec ALTER (`ADD COLUMN x INT, DROP
  PARTITION p0`) is a **proven refusal on every anchor**: MySQL 5.7/8.0/8.4
  → ERROR 1064 (partition ops cannot join the alter list at grammar level);
  TiDB → ERROR 8200 "Unsupported multi schema change for drop partition"
  (parses, refused at the multi-schema-change gate). The shared PingCAP
  parser accepts `ADD COLUMN ..., DROP PARTITION` as two specs, so the
  product sees the shape; `DROP PARTITION p0, DROP COLUMN c` and
  `DROP PARTITION p0, DROP PARTITION p1` are rejected by the shared parser
  itself, matching native 1064.
- LIST partitioning: `DROP PARTITION pA` on a LIST table → rc=0.

### 1.2 What the metadata model proves and cannot prove

`spec.TableSnapshot` carries no partition member catalog: the only partition
fact is `Partitioned *bool` (TiDB provider-loaded; nil elsewhere). There is
no partition type, boundary, or member list, and no row/statistics fields
beyond `Options` counters (`table_rows`, `auto_increment`, `data_length`,
`index_length`, `avg_row_length`) and per-index `Cardinality`. Therefore:

- Whether partition `p0` exists is NEVER statically provable → executability
  of any partition action is always unproven.
- `Partitioned=true` does not prove the target partition exists and does not
  prove complete partition definitions; `Partitioned=false` proves refusal
  (1505) but member facts still survive; `Partitioned=nil` stays unknown.
- No rule may claim "validated compatible" from missing exchanged-table
  metadata, and `WITH|WITHOUT VALIDATION` is never proof data satisfies the
  partition constraint (observed: `WITHOUT VALIDATION` lands violating rows).

## 2. Frozen rule IDs, levels, configuration

| Rule ID | Params | Default level | Action consumed |
|---|---|---|---|
| `ddl.alter.drop_partition.forbid` | `forbid` (bool, default true) | blocker | `drop_partition` |
| `ddl.alter.truncate_partition.forbid` | `forbid` (bool, default true) | blocker | `truncate_partition` |
| `ddl.alter.exchange_partition.warn` | none | warning | `exchange_partition` |

The three rules register in the shared DDL registry for MySQL and TiDB (all
four anchors implement all three actions natively). `enabled:false` removes
the finding; it never removes the audited-capability answer (coverage).
`forbid:false` likewise disarms the forbid rules. Findings fire on the
submitted action alone — policy intent is independent of metadata
availability and of native executability; a blocker finding does not claim
the engine refused or executed the action.

### 2.1 Finding shape

- `drop_partition.forbid`: one finding per named partition in the action
  (`DROP PARTITION p0, p1` → 2 findings). Metadata: `action=drop_partition`,
  `table`, `partition`.
- `truncate_partition.forbid`: one finding per named partition; the `ALL`
  form yields one finding with `all_partitions=true` and no `partition` key.
- `exchange_partition.warn`: one finding per action. Metadata: `table`,
  `partition`, `exchange_table` (as-written name), `exchange_schema`
  (as-written qualifier, empty when unqualified), `validation` =
  `with`|`without`. `with` covers both the authored `WITH VALIDATION` and
  the absent clause — the parsed form cannot prove the keyword was explicit
  (the AST collapses both to `WithValidation=true`).

Finding location is the statement-level location pipeline (line/column of
the statement); no per-partition span exists in the normalized model.

## 3. Normalized facts added

`spec.Alter` gains:

- `Partitions []string` — partition member names, lower-cased as-written,
  in clause order (empty for `ALL`).
- `AllPartitions bool` — the `TRUNCATE PARTITION ALL` form.
- `ExchangeTable *spec.Table` — the exchanged table identity as written;
  schema empty when unqualified.
- `ExchangeValidation string` — `"with"`/`"without"`, per §2.1.
- `alter.Name` = first partition name (keeps name-keyed generic consumers
  meaningful).

`spec.DDL.Targets` gains the exchange counterpart after the primary table
(same position class as RENAME destinations — a named table-level object
identity). The extractor emits `[t, t2]` for
`EXCHANGE PARTITION p0 WITH TABLE t2`.

`auditmeta` loads the exchange counterpart as an existing object: the
`tables[:1]` truncation applies only to RENAME destinations, which are new
names; the exchange counterpart is an existing object and must participate
in metadata resolution (missing counterpart metadata is a gap input, never
"validated").

## 4. Applicability and evidence

- All four anchors implement all three actions → no version gate; the rules
  apply on mysql 5.7/8.0/8.4 and tidb 8.5 alike.
- The three rules declare no required facts → no evidence gaps from them.
  Action presence is fully proven from the statement.
- Partition-type applicability (RANGE/LIST vs HASH for DROP), member
  existence, and structure/validation compatibility are unmodeled facts:
  never fabricated; executability stays unproven by design.
- Coverage: the actions join `auditedAlterActions{MySQL,TiDB}`; a clean
  `ALTER TABLE t DROP PARTITION p0` is `complete` instead of the previous
  `alter_table.drop_partition` unsupported aspect. Rules disabled → finding
  gone, coverage still `complete` (capability and policy are separate
  dimensions).

## 5. Ordered-state contract (batch_state)

Premise: member-level facts provably survive every outcome of a
single-action partition ALTER — DROP/TRUNCATE/EXCHANGE never add, rename, or
remove columns/indexes/constraints/PK, and `Partitioned` is invariant
(success keeps ≥1 partition — the native engine refuses the last drop —
or the statement refuses; exchanged tables stay non-partitioned either
way). Row/statistics facts are NOT invariant — data moves or is destroyed
when the action lands:

- **Single-action `drop_partition`/`truncate_partition`**: if the target
  entry is present with a shape, publish the same shape minus
  `clearMutationStats` (`table_rows`, `auto_increment`, index cardinality).
  Unknown/absent/incomplete entries stay as they are (no fabricated
  upgrade). The partition-topology outcome is unmodeled — recorded as
  contract boundary, not as a kept "exact" fact.
- **Single-action `exchange_partition`**: same transition applied to BOTH
  named identities (targets[0] and the exchange counterpart).
- **Multi-spec ALTER containing any partition action**: proven refusal on
  all anchors (§1.1) → exact pre-state published (same mechanism as
  `identityScanRefused`): members and stats kept as the statement can never
  execute. Note the `len(ddl.Alter) > 1` check must use spec-level
  provenance: a partition action with any sibling action is refusal;
  `AlterTableAddColumns` expands to multiple alters from ONE spec but can
  only co-occur with a partition action via a second spec — refusal either
  way, so `len(ddl.Alter) > 1 && contains partition action` is a sound
  refusal test.
- **Policy blocker does not erase conditional migration state**: findings
  and state transitions are independent; the publish path is unchanged when
  a rule flags the statement.
- **Tombstone fallback retained**: any partition action on a non-audited
  statement (e.g. with unaudited sibling aspects) or an
  unextracted/anomalous payload still invalidates rather than guessing.

## 6. Golden manifest (T13.json) — frozen case set

DDL anchor steps (each anchor): create RANGE fixture, insert fixture rows,
DROP PARTITION p0 (verify p0 def+rows gone, p1 intact), TRUNCATE PARTITION
p1 (def kept, rows gone), EXCHANGE PARTITION p0 WITH TABLE x (rows swap),
cleanup. Syntax negative: `ALTER TABLE t DROP PARTITION (` → parse-refusal
class on every anchor.

CLI cases (offline, no metadata):

- `t13-cli-drop-partition-blocker`: mysql, `ALTER TABLE t DROP PARTITION
  p0;`, policy t13-drop → 1 stmt, complete, exactly 1 blocker
  `ddl.alter.drop_partition.forbid` {table=t, partition=p0}, verdict
  reject, `--fail-on blocker` exit 1, zero unsupported.
- `t13-cli-drop-partition-tidb-blocker`: same on tidb dialect.
- `t13-cli-drop-partition-multi`: `DROP PARTITION p0, p1` → 2 findings.
- `t13-cli-drop-partition-off`: policy `all-rules-disabled` → pass/exit0/0
  findings, still complete (capability kept).
- `t13-cli-truncate-partition-blocker` / `-all`: blocker; ALL form carries
  `all_partitions` metadata.
- `t13-cli-exchange-warn` / `-noval`: warning `exchange_partition.warn`
  carrying {table, partition, exchange_table, validation}; exit 0 under
  fail-on blocker, exit 1 under `--fail-on warning` in a second case.
- `t13-cli-exchange-qualified`: `WITH TABLE s2.t2` → exchange_schema=s2.
- `t13-cli-exchange-protected-other-side`: denylist tables=[t13x] →
  exchange hits denylist blocker on the counterpart side.
- `t13-cli-exchange-protected-left`: denylist tables=[t] → denylist blocker
  on the partitioned side.
- `t13-cli-addcol-control`: non-partition action → no partition finding.
- `t13-cli-mixed-refusal`: `ADD COLUMN x INT, DROP PARTITION p0` → audited,
  drop blocker still fires (policy dimension) — refusal modeled at state
  level, verified by unit tests (natives: 1064/8200).

Metadata cases (connected): per-anchor golden DROP PARTITION on mysql80 +
tidb85 fixture (isolated profile `t13-drop` → complete/reject/exit1, rule
metadata {table=t13, partition=p0}, `version` pinned); execute-then-verify
DB effects (p0 gone, p1 intact); a teardown. EXCHANGE protected-side case on
mysql80 denylist.

## 7. Inventory

- `mysql.alter-table.partition-destructive` → `semantically_checked` with
  T13 evidence refs.
- `tidb.alter-table.partition-ops` — split: new row
  `tidb.alter-table.partition-destructive` (DROP/TRUNCATE/EXCHANGE →
  semantically_checked, owner T13, evidence refs); the original row keeps
  only the T14 remainder (REORGANIZE/COALESCE/REMOVE PARTITIONING → stays
  parse_only, owner T14). The new ID is appended to `required_row_ids` and
  `required_rows.txt` — denominator grows, never shrinks.

## 8. Unverified boundaries (explicit)

- Partition member catalog (names/types/bounds) is unmodeled; partition
  existence and partition-type applicability are never statically proven.
- `WITH VALIDATION` explicit-vs-absent is indistinguishable in the AST;
  only the effective `with`/`without` fact is recorded.
- Sub-partitioned DROP PARTITION semantics unprobed (fixture is plain
  RANGE/LIST); TiDB `REORGANIZE` is T14.
- `Partitioned` stays provider-loaded for TiDB only; no MySQL-side
  `Partitioned` load is added this task.
- Statistics invalidation is bounded to `clearMutationStats`
  (table_rows/auto_increment/cardinality); data_length/index_length/
  avg_row_length are cleared too via the same call as defensible
  data-derived facts.
