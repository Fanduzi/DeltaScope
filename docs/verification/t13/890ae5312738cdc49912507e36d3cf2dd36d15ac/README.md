# T13 evidence bundle — 890ae531

Acceptance-candidate evidence for T13 (#92): destructive partition
actions — DROP/TRUNCATE/EXCHANGE PARTITION — promoted from parse-only to
audited semantics. Product commit
`890ae5312738cdc49912507e36d3cf2dd36d15ac` (parent `dcf250ea`, the T12
evidence-closure commit). Contract: `docs/work-plans/t13-partition-risk/CONTRACT.md`
(frozen before implementation); ADR:
`docs/decisions/2026-11-03-ddl-partition-destructive-actions.md`.

## Scope delivered

- `spec.Alter` partition payload: `Partitions`, `AllPartitions`,
  `ExchangeTable`, `ExchangeValidation`; `alter.Name` = first partition;
  `DDL.Targets` gains the exchange counterpart as an existing object.
- Three frozen rules: `ddl.alter.drop_partition.forbid` /
  `ddl.alter.truncate_partition.forbid` (blockers, per-named-partition
  findings; `ALL` carries `all_partitions:true`),
  `ddl.alter.exchange_partition.warn` (warning carrying both identities
  + effective validation). All MySQL+TiDB; `forbid:false` disarms.
- Ordered state: multi-spec partition ALTER = proven refusal on every
  anchor (exact pre-state kept); single action keeps member facts and
  clears data-derived statistics (`table_rows`, `auto_increment`,
  `data_length`, `index_length`, `avg_row_length`, index cardinality);
  EXCHANGE invalidates both identities; unextracted payloads tombstone.
- Coverage: the three actions joined the audited set for MySQL and TiDB;
  the T14 remainder (REORGANIZE/COALESCE/REMOVE PARTITIONING) stays
  parse-only under `tidb.alter-table.partition-ops` (owner T14).
- Inventory: `mysql.alter-table.partition-destructive` and new row
  `tidb.alter-table.partition-destructive` are `semantically_checked`;
  `required_row_ids` + `required_rows.txt` grew (denominator never
  shrank).

## Red baseline

`red/` — captured on the pre-T13 product `dcf250ea` with the committed
T13 test files copied in:

- `t13_red_parser.log` — build failure: `spec.Alter` lacked
  `Partitions`/`AllPartitions`/`ExchangeTable`/`ExchangeValidation`.
- `t13_red_rules.log` — build failure: `newDropPartitionForbidRule` et
  al. undefined.
- `t13_red_state.log` — ran to failure: partition actions reported
  `unsupported` / entries vacated instead of the contracted post-state.
- `t13_fulltest.log` — green full-repo `go test ./...` on the product
  SHA (kept alongside the red set for the before/after pair).

## Test layers (as actually written)

- Public report path (`AuditSQL`): findings per named partition, ALL
  metadata shape, both-sided exchange metadata + effective validation,
  denylist hitting either side, enabled/disabled profiles, mixed-action
  refusal keeping the policy finding, `coverage=complete` under rules
  disabled.
- Internal state reads (`t13r4PostState`-style helpers plus direct
  `batchState` application): stat-set clearing on the bound identity,
  both-side invalidation on EXCHANGE, exact pre-state on multi-spec,
  tombstone fallback on unextracted payloads.
- Native observations: `docs/work-plans/t13-partition-risk/probes/`
  (four anchors: DROP 1507/1508/1505/1512; TRUNCATE incl. ALL + HASH;
  EXCHANGE both directions, 1526 vs TiDB 1737 on `WITH VALIDATION`
  violations, 1736/1732/1146/1735 refusals; multi-spec 1064 on MySQL /
  8200 on TiDB) — plus the golden DDL cases' live before/after catalog +
  row checks.

## Gates

`gates/` — 18 command tuples (`.command.txt`/`.stdout.txt`/
`.stderr.txt`/`.rc.txt`): 17 × rc=0; `lint` rc=1 is the accepted
13-issue baseline (stdout byte-identical to the `t12r2tail2` bundle —
zero new issues). `gofmt` clean except the two accepted T07 files.

## Golden

`golden/<TASK>/` — five serial runs sharing anchors, each bound to
`890ae531` (`head_sha` and `cli.build.head_sha` in every artifact.json;
one CLI sha256 `2dcdce5f68ab673a` across all five; `cli.path` is the
runtime build path — the binary under `bin/` is archived here for
inspection):

| Task | Cases | Assertions |
|------|-------|------------|
| T09  | 125   | 3442 |
| T10  | 126   | 3175 |
| T11  | 40    | 506  |
| T12  | 25    | 282  |
| T13  | 24    | 289  |

All PASS; `executed_count` == required case set for every task;
`residual_containers=[]` in cleanup. T13 DB cases verified live
partition effects (p0 rows gone/p1 intact on DROP; definitions kept,
rows gone on TRUNCATE; bidirectional row movement on EXCHANGE) via
explicit schema-qualified `COUNT(*)`/`PARTITIONS` catalog queries —
`information_schema.TABLE_ROWS` statistics were rejected as
unreliable during this task's run.

## Unverified boundaries (contract §8)

- Partition member catalog/type/bounds unmodeled; member existence and
  partition-type applicability never statically proven.
- `WITH VALIDATION` explicit-vs-absent indistinguishable in the AST —
  only the effective `with`/`without` fact recorded.
- Sub-partitioned DROP, MySQL-side `Partitioned` catalog load, and the
  T14 topology actions stay unprobed/out of scope.
