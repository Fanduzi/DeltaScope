# Decision: TiDB AUTO_RANDOM declaration validity — typed bit facts, static refusal matrix, and policy separation for TiDB's native allocator

Date: 2026-11-13
Status: Accepted (offline candidate; native validation pending authorization)
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T23 under #102)
Related tests: `internal/domain/rule/ddl/auto_random_t23_test.go`,
`internal/application/audit/coverage_t03_test.go` (graduated aspect),
corpus fixture `testdata/sql-corpus/tidb/ddl/findings/auto_random_rules.*`
Related docs: module README updates in the same change
(`internal/domain/spec`, `internal/domain/rule/ddl`,
`internal/application/audit`, `internal/infrastructure/parser/tidb`),
`configs/deltascope.example.yaml`

## Context

T23 (issue #102) owns the TiDB physical-layout / ID / cache / replica
surface: `AUTO_RANDOM[(shard[,range])]`, `AUTO_RANDOM_BASE`,
`SHARD_ROW_ID_BITS`, `PRE_SPLIT_REGIONS`, `AUTO_ID_CACHE`, `ATTRIBUTES`,
table CACHE/NOCACHE, and `SET TIFLASH REPLICA`. Before this change the
typed `Column.AutoRandom` flag existed but carried no authored bits and
no validity adjudication: every AUTO_RANDOM declaration rode a bounded
unaudited `*.column.auto_random` aspect under TiDB (vendor boundary under
MySQL), and the generic PK policies fired literal-attribute false
positives — `AUTO_RANDOM` is itself TiDB's surrogate-key allocator, so
requiring a literal `AUTO_INCREMENT` on it is wrong.

Upstream anchors (verified against the v8.5.0 tag source):
`setTableAutoRandomBits`/`checkAutoRandom` in `pkg/ddl/create_table.go`,
`NewShardIDFormat` in `pkg/meta/autoid/autoid.go`, and the
`AlterTableOption` dispatch in `pkg/ddl/executor.go`.

## Decision

- `Column.AutoRandomShardBits`/`AutoRandomRangeBits` (`*int64`, nil =
  unwritten) carry the authored `AutoRandOpt` arguments; the parser's
  `UnspecifiedLength` sentinel never becomes a zero fact. Upstream
  normalization (shard default 5, range default 64) is applied only at
  adjudication time, mirroring `NewShardIDFormat`.
- New TiDB-only blocker rule `ddl.tidb.table.auto_random.valid` on
  CREATE TABLE mirrors the fixed v8.5.0 static refusals:
  non-BIGINT column; AUTO_RANDOM combined with AUTO_INCREMENT or an
  explicit DEFAULT; authored shard bits outside 1..15; authored range
  bits outside 32..64; incremental bits below 27
  (`range − shard − (signed ? 1 : 0)`).
- Position adjudication rides `spec.PrimaryKeyClustered`: no declared
  primary key, a proven non-clustered key (`NONCLUSTERED` keyword or an
  observed mode that yields no handle), or a column that is not the
  first PK member refuses statically. When the PK type is unwritten and
  `tidb_enable_clustered_index` (plus `alter_primary_key` under
  `int_only`) is unobserved, the clustered answer stays unproven and the
  rule emits `missing_instance_fact` — never a guessed pass or refusal.
- `ddl.table.primary_key.auto_increment.require` accepts a TiDB
  AUTO_RANDOM member: the policy's intent is an auto-allocated surrogate
  key and AUTO_RANDOM is exactly that allocator. `unsigned.require`
  keeps its literal semantics — a signed BIGINT is factually signed and
  teams may satisfy it via `BIGINT UNSIGNED`, so the finding stays an
  accurate team-policy complaint rather than a suppressed truth.
- Coverage: the `*.column.auto_random` aspect retires under TiDB
  CREATE TABLE (audited). It stays a vendor boundary under MySQL and a
  bounded unaudited aspect on TiDB ALTER-side declarations until ALTER
  validation is modeled.

## Rationale

The refusal matrix is the smallest complete mirror of what TiDB checks
before the DDL job exists — all statement-local except the clustered
premise, which already has a tri-state resolver and observed-fact seams.
Separating declaration facts (authored bits) from normalized defaults
keeps "unwritten" provable rather than laundering the product's default
into a claimed value.

## Public Contract

- TiDB CREATE TABLE AUTO_RANDOM declarations now reach one of three
  answers: a `ddl.tidb.table.auto_random.valid` blocker on a proven
  static refusal, a `missing_instance_fact` evidence gap naming
  `instance.tidb_enable_clustered_index`/`instance.alter_primary_key`
  when only the clustered premise is unprovable, or silence on a
  provably applicable declaration.
- MySQL statements keep the `*.column.auto_random` vendor boundary; the
  rule never fires there.
- Nothing is claimed about allocated IDs, allocator persistence, or
  runtime behavior — declarations only.

## Deferred / Out Of Scope

- ALTER-side AUTO_RANDOM validation (add/modify column paths) — bounded
  unaudited aspect retained.
- Table options `AUTO_RANDOM_BASE`, `AUTO_ID_CACHE`,
  `SHARD_ROW_ID_BITS`, `PRE_SPLIT_REGIONS`, `ATTRIBUTES`, table
  CACHE/NOCACHE, and `SET TIFLASH REPLICA` — extraction/state/provider
  wiring deferred to the T23 follow-up slices with the native run.
- Catalog facts the remaining refusals need (clustered-vs-not on an
  existing table, partition membership for ATTRIBUTES, TiFlash store
  count, cache-table 64MB bound) — no field is fabricated.

## Verification Evidence

- `auto_random_t23_test.go`: full refusal matrix plus the
  unknown-mode gap, the unsigned sign-bit edge ((15,42) pass vs (15,41)
  refuse), MySQL silence, and the `auto_increment.require` acceptance.
- `coverage_t03_test.go`: TiDB CREATE AUTO_RANDOM moved from the
  unaudited-aspect table to the complete-coverage assertion; MySQL and
  ALTER-side rows unchanged.
- Corpus fixture `auto_random_rules.*` exercises the rule under the
  default policy for the `tidb` dialect target.
- `go test ./...`, `go vet`, `go build` clean on the change commit.

## Consequences

- Any future ALTER-side modeling must reuse the same static matrix and
  the tri-state clustered resolver rather than re-deriving predicates.
- The `missing_instance_fact` gap means offline TiDB AUTO_RANDOM audits
  stay `unverified`/`incomplete` unless the instance facts are observed —
  the honest answer, matching the T22 capability-gap precedent.

## Links

- Upstream: pingcap/tidb v8.5.0 `pkg/ddl/create_table.go`,
  `pkg/ddl/executor.go`, `pkg/meta/autoid/autoid.go`
- Issue: #102 (T23), parent #79

## Amendment — T23-R2 (2026-11-08 follow-up): shared verdict, refusal-proof post-state, create-time options, explicit policy parameter

The review required the allocator acceptance to be team-configurable
rather than unconditional, and the refusal matrix to be authoritative in
ordered state — not merely a rule finding.

- `spec/auto_random.go` is now the single shared verdict:
  `AutoRandomColumnVerdict` feeds `ddl.tidb.table.auto_random.valid` and
  `applyCreateTable` identically. A statically-refused CREATE keeps the
  exact pre-state (no ghost shape even with the rule disabled — ordered
  state never consults the rule engine); an undecidable clustered
  premise invalidates the bound fact. `CREATE TABLE ... IF NOT EXISTS`
  on a proven-present target still settles before any premise engages.
- `auto_increment.require` gains `accept_tidb_auto_random` (unset =
  strict literal `AUTO_INCREMENT`; shipped default `true`). A legal
  AUTO_RANDOM under the strict profile emits the named policy blocker —
  never described as a parser/vendor/native refusal — and the native
  post-state still publishes. `unsigned.require` stays literal-only.
- `spec.CreateTableOptionOutcome` mirrors `handleTableOptions` for the
  four create-time options: `AUTO_ID_CACHE` >MaxInt64 refuses, nonzero
  `SHARD_ROW_ID_BITS` on a clustered PK refuses (same tri-state premise,
  same minimal gap), `PRE_SPLIT_REGIONS` on a temporary table refuses,
  `AUTO_RANDOM_BASE` accepts any authored uint. Declared values publish
  unchanged into `shape.Options` — source clamps (shard>15→15,
  pre-split>shardingBits) are effective-state normalization, never
  authored rewrites or blockers.
- New rule `ddl.tidb.table.options.valid` emits the finding/gap; MySQL
  keeps the vendor boundary on CREATE and ALTER
  (`spec.mysqlAlterVendorOptions`); ALTER-side mutability and ALTER
  column AUTO_RANDOM stay bounded unaudited gaps pending R2.6.
- CACHE/NOCACHE, ATTRIBUTES, and SET TIFLASH REPLICA remain
  inventory-pending: TiFlash success claims require a real TiFlash
  store; a unistore store-count=0 proves only a negative.
- `testdata/ddl-golden/T23.json` records the offline implementation
  candidate and the narrowed A/B/C restricted-fixture card; every
  mutable variable records scope, reads the original value, and restores
  the observed original. No native case has run; none is claimed.

Verification additions: `batch_state_t23_test.go` (refusal matrix → no
publish, undecidable → invalidate, applicable → publish with handle PK,
pre-existing shape preserved), `batch_state_t23opt_test.go`,
`auto_random_t23opt_test.go` (option refusals + minimal gaps),
`pkg/deltascope/audit_t23_test.go` (public-Audit seam through a stub
provider), `testdata/sql-corpus/tidb/ddl/findings/table_options_rules.*`,
CLI evidence for both policy profiles.

## Amendment — T23-R3 (2026-11-14 follow-up): ALTER/ID wiring, declared-to-effective post-state, reader chain

- `spec/alter_auto_random.go` adds the shared ALTER verdicts consumed
  identically by `ddl.tidb.alter.auto_random.valid`,
  `ddl.tidb.alter.table_options.valid`, and the ordered-state publisher:
  `AlterAutoRandomOldFacts` resolves the old column's clustered-PK
  membership, authored shard/range bits, PK-handle status, and the
  has-AUTO_RANDOM fact — including the vacuous arm where a table provably
  without AUTO_RANDOM answers zero bits instead of leaving the clustered
  premise unknown, so an ordinary PRIMARY KEY widen is never poisoned;
  `AlterAutoRandomColumnVerdict` mirrors the v8.5.0 `checkAutoRandom`
  ALTER matrix (`MODIFY`/`CHANGE`: removal, bit decrease, range/type
  change, non-BIGINT, AUTO_INCREMENT/DEFAULT conflicts refuse; same-bits
  redeclaration and increases owe the runtime allocator check as
  undecidable; `ADD COLUMN AUTO_RANDOM` is the fixed
  `AutoRandomAlterAddColumn` refusal); `AlterLayoutOptionOutcome`
  adjudicates the four layout options — AUTO_ID_CACHE overflow and the
  1↔non-1 boundary refuse, SHARD_ROW_ID_BITS temporary/clustered refuse
  with the >15 clamp and a runtime row-scan premise on real changes,
  AUTO_RANDOM_BASE non-AUTO_RANDOM/overflow refuse with FORCE publishing
  the declared base while non-FORCE unvouches the runtime-adjusted value,
  and PRE_SPLIT_REGIONS is the proven unsupported-alter refusal.
- Ordered state applies the same verdicts: proven refusal preserves the
  exact pre-state; undecidable premises poison the successor or unvouch
  only the affected option while preserving unrelated facts; rule
  disablement changes finding emission only — never the state verdict
  (pinned by `batch_state_t23alter_test.go` matrix + disabled-policy
  invariance cases).
- Declared-vs-effective layering: `DDL.Options` keeps authored values;
  `TableSnapshot.Options` publishes catalog-effective values via
  `AutoRandomEffectiveBits`/`CreateTableOptionsOutcome` normalization
  (unwritten shard/range normalize to 5/64, presplit clamps to
  shardingBits).
- Reader chain: the TiDB provider reads
  `information_schema.TABLES.TIDB_ROW_ID_SHARDING_INFO`
  (`PK_AUTO_RANDOM_BITS=n[, RANGE BITS=m]` flags the single-member PK
  column `AutoRandom` with stored bits, `SHARD_BITS=n` and the
  `NOT_SHARDED` family publish proven `Options["shard_row_id_bits"]` —
  zero included; a nil cell stays unobserved, an unrecognized shape
  fails the read) and projects the SHOW CREATE
  `/*T![auto_id_cache] AUTO_ID_CACHE=n */` feature comment into
  `Options["auto_id_cache"]` (absent comment = upstream-proven zero).
  The MySQL provider never issues the TiDB probes.
  `provider_t23_test.go` pins stub→provider→`InstanceFacts` fidelity.
- Manifest: `testdata/ddl-golden/T23.json` gains the baseline anchors
  mysql57/mysql84, the `t23-layout-id`/`t23-strict-literal` policy
  profiles, 11 offline CLI cases verified against the real binary, and
  8 metadata cases retained `pending_native` (TiFlash case keeps
  `requires_service: "tiflash"` — no TiFlash service exists in the
  golden stack).
- Remaining bounded scope: ATTRIBUTES, CACHE/NOCACHE, TiFlash replica
  success paths, COMPACT, SPLIT stay inventory-pending; all 8 native
  cases await the restricted-fixture authorization card.
- Verification: `go test ./...` all green; corpus 335 expected files,
  coverage_percent=100, missing_rule_dialect_targets=0; manifest
  offline validation — 19/19 required cases, no duplicates, policy IDs
  validated against the live rule catalog, baseline anchors clean.
- Commits: this file's task commit; see `git log` for the product
  commit range.

## R5 amendment — three proven blockers repaired (issue #102)

Independent R4 review found three real defects on the R3 head; this
amendment records the repairs without rewriting the R4 failure
receipts.

- **Zeroing `SHARD_ROW_ID_BITS` needs no allocator premise.** Upstream
  `verifyNoOverflowShardBits` returns nil for a zero target without
  reading the allocator; `AlterLayoutOptionOutcome` now classifies the
  zero arm `AutoRandomOK` after the same-value check and before the
  positive-target runtime premise, so `shard_row_id_bits=0` publishes
  `Options["shard_row_id_bits"]="0"` as a proven successor fact —
  distinct from omission or unknown. Positive targets keep the
  allocator (`NextGlobalAutoID`) premise; the `runtime.row_data` fact
  key is retained for compatibility but now documented as the
  allocator premise, not a row scan.
- **Table-level AUTO_RANDOM facts outlive member attribution.**
  `PK_AUTO_RANDOM_BITS` is a table-level catalog fact: the provider
  records `Options["auto_random_bits"]`/`["auto_random_range_bits"]`
  unconditionally and flags the first PK member only when the member
  list is proven. `SHARD_BITS`/`NOT_SHARDED` publish the proven
  `auto_random_bits=0` negative. `AlterAutoRandomOldFacts` derives the
  carrier from the table-level record when the member list is proven
  (upstream AUTO_RANDOM always rides the first clustered member) and
  keeps `memberKnown=false` when the list itself is unproven — the
  unattributed shape can no longer fabricate a
  `change_from_non_auto_increment` refusal or drop the bits fact an
  `AUTO_RANDOM_BASE` overflow verdict needs. An unproven carrier
  signedness evaluates both mask arms and only decides when they
  agree.
- **Quoted text is not a catalog feature comment.** `featureComments`
  now scans SHOW CREATE quote-aware: markers inside single-quoted or
  double-quoted string literals and backtick identifiers (with
  backslash and doubled-quote escapes) are user-authored text and
  never produce `AUTO_ID_CACHE`, AUTO_RANDOM or TTL facts. One shared
  lexical walk protects both the TTL and the auto_id_cache
  projections; real feature comments outside quotes parse unchanged.
  `audit_t23r5_injection_test.go` proves the reader-to-public-Audit
  boundary: the same TTL ALTER yields the add warning against a
  quoted-fake catalog and the `target_table.ttl_enable` gap against a
  real comment.

## R6 amendment — physical layout actions (issue #102, T23-R6)

The remaining `tidb.alter-table.layout-id` inventory surface is wired in
the same shared-verdict shape as R3/R5: `internal/domain/spec/
alter_layout_r6.go` owns one adjudicator per action, rules and the
ordered-state publisher consume identical verdicts, and proven refusals
preserve the exact pre-state.

- **CACHE/NOCACHE follows the upstream ordering.** `AlterTableCacheOutcome`
  mirrors the v8.5.0 executor: already-cache-enabled CACHE and
  already-disabled NOCACHE are proven no-ops; system/memory schemas,
  temporary tables and partitioned tables refuse statically; the 64 MiB
  data-size check is the sole undecidable arm (a `missing_instance_fact`,
  never a guessed answer). Proven CACHE publishes
  `Options["cache_status"]="enable"`, proven NOCACHE publishes
  `"disable"`, and an undecidable outcome unvouches the status key rather
  than keeping a stale claim. The provider projects the bare
  `/* CACHED ON */` SHOW CREATE trailer through the same quote-aware walk
  (quoted user text never matches); CREATE-side `CACHED` is dead syntax in
  the v8.5.0 parser and stays a parser boundary.
- **The cached-table guard preserves every target.** Upstream refuses
  every ALTER spec except a single CACHE/NOCACHE on a proven
  cache-enabled table. `CachedTableGuardRefused` fires before the
  generic audited gate and `cachedTableGuardBlocks` preserves each
  target exactly — including present-incomplete entries — so a guarded
  batch never tombstones or alternates existence gaps.
- **ATTRIBUTES publishes bounded presence, never the raw spec.**
  `AlterAttributesOutcome` emulates the upstream YAML-flow label-list
  syntax (`ParseAttributesSpecValue`: quoted items, embedded commas,
  `ATTRIBUTES=DEFAULT` clearing) plus the non-partitioned and
  missing-member refusals; an unproven member space stays undecidable.
  Ordered state publishes only `attributes_present` /
  `partition_attributes_present.<name>` markers — the authored spec text
  never lands in catalog facts.
- **SET TIFLASH REPLICA is declaration-only.** `AlterTiFlashReplicaOutcome`
  refuses sys-DB/temporary/view-or-sequence/unsupported-charset targets
  statically; a proven same-definition redeclare is a no-op (positive
  counts only — upstream nils the record on `REPLICA 0`, so count=0
  applies unconditionally and publishes the proven-absent marker); a
  positive count owes the live TiFlash store count as an undecidable
  premise and never claims replica readiness. Hypothetical
  (`HYPOTHETICAL`) declarations mutate session state upstream and never
  touch ordered catalog facts.
- **COMPACT and SPLIT stay bounded.** `COMPACT TABLE` parses as its
  own `compact_table` statement, and the independent SPLIT forms
  (`SPLIT TABLE t BETWEEN ... REGIONS n`, `SPLIT TABLE t BY (...)`)
  parse as `*ast.SplitRegionStmt` (`split_region`) — the v8.5.0 parser
  accepts all of them (R7 correction: an earlier draft wrongly claimed
  SPLIT was a parser boundary; only ALTER-embedded split spec forms
  fail to parse). Neither statement is modeled for semantic audit:
  the extractor keeps them as `kind:"unknown"` + `coverage:
  incomplete` bounded statements — no findings, no state transition —
  and that limitation is product modeling, not vendor parser support.
  Real TiFlash store success/recovery fixtures stay `pending_native`
  in `T23.json` (`B-cache-attributes`, `C-tiflash-replica`) until
  separately authorized.

### R7 amendment — cache status and ATTRIBUTES corrections (issue #102)

- **Cache status is a real catalog fact, not a marker guess.** A bare
  `/* CACHED ON */` comment in SHOW CREATE proves Enable; its absence
  proves nothing (Switching emits no marker). The auxiliary source is
  `mysql.table_cache_meta` LEFT-JOINed onto the anchor
  `information_schema.tables` row: upstream writes that row as a lock
  record before the cache DDL job and never deletes it, so it is not a
  status projection — a NULL lock cell proves never-cached Disable, a
  live lock row without the marker stays unobserved (a completed
  NOCACHE, a pending CACHE and true Switching share that exact shape),
  a missing anchor row stays unobserved, and query/scan failures
  propagate (R7-review correction: an earlier draft claimed
  row-without-marker proved Switching — the lock record is not a
  status projection, and the read's errors must propagate, not degrade
  into a missing key). The reader no longer fabricates `disable` for a
  missing marker.
- **NOCACHE converges without observing the old state.** Upstream's
  Disable→Switching→Enable machine makes every legal old status
  converge to Disable, so `ALTER ... NOCACHE` publishes `disable`
  unconditionally — a no-op reason only when the old status is a
  proven Disable. Batch-created tables start at the proven Disable
  default; temporary/sys-DB/partitioned/non-base-table objects are
  proven Disable by class because CACHE refuses them outright.
- **The `!= Disable` mutation gate covers every supported write.** The
  shared `spec.CacheGuardOutcome`/`CacheGuardPremiseNeeded` criterion
  guards ALTER (all specs except a lone CACHE/NOCACHE, counting only
  effective specs), DROP/TRUNCATE/RENAME TABLE, CREATE/DROP/RENAME
  INDEX: proven non-Disable refuses and preserves the exact pre-state,
  proven Disable keeps existing behavior, and an unobserved status
  unvouches only what the withheld transition could have moved —
  present-incomplete for shape-preserving operations, existence
  tombstones for DROP/RENAME source names. The `ddl.tidb.table.cache.valid`
  rule reports the refusal finding and the `table.cache_status`
  evidence gap on the same criterion.
- **ATTRIBUTES parses through the upstream-equivalent YAML engine.**
  `ParseAttributesSpecValue` wraps the spec text as a YAML flow
  sequence and applies NewLabel/Add semantics — exactly one `=` per
  item, non-empty trimmed key and value, same-key same-value
  deduplication, same-key conflicting-value refusal. Upstream v8.5.0
  runs `gopkg.in/yaml.v2` `UnmarshalStrict`; a bounded offline
  differential over the reachable input domain (32 cases: items,
  quoting, trailing commas, separators, YAML-1.1 bool-like scalars)
  showed zero divergence with the `go.yaml.in/yaml/v3` engine kept
  here, including the quote recovery and trailing-comma tolerance.
  `ATTRIBUTES=DEFAULT` and successfully parsed empty lists clear
  attributes. Authored attribute text stays out of findings, gaps,
  and published facts.
- **Rename endpoints keep independently-proven identities.** Upstream
  checks source/destination identity (`ExtractTblInfos`) before the
  cache gate, so when a destination is proven-live — or a RENAME
  TABLE source proven-absent — the statement refuses regardless of
  the unobserved cache status and no endpoint proof lapses (R7-review
  correction: withheld transitions unvouch only names that could
  actually have moved).

### R8 amendment — SPLIT/COMPACT audited through the common path (issue #102)

- **Both standalone statements are now normalized operations, not
  parser-boundary or request-only rows.** `SPLIT [REGION FOR]
  [PARTITION] TABLE` extracts as `spec.DDLOperationSplitRegion` and
  `ALTER TABLE ... COMPACT` as `spec.DDLOperationCompactTable`. The
  authored declaration keeps the index/member target, the
  `SPLIT PARTITION TABLE` clause variant, partition names, the
  BY-vs-BETWEEN bound shape, per-position value classes (NULL,
  provably-non-numeric string, plain literal, non-literal expression —
  the literal payload itself never enters the model), and the authored
  REGIONS count. `ALTER TABLE ... SPLIT INDEX` is an embedded spec, not
  the standalone statement — it stays bounded-unmodeled through the
  existing `alter_table.split_index` evidence.
- **`spec.SplitRegionOutcome`/`CompactTableOutcome` follow upstream
  v8.5.0 order.** SPLIT adjudicates temporary → SPLIT-PARTITION-clause
  on a non-partitioned target → index arms (clustered primary refuses;
  a proven nonclustered primary splits its own regions; missing index
  refuses) → value tuples (exact table-handle arity vs partial index
  arity; NULL refused on the table path, unproven on the index path;
  a provably non-numeric string refused against a proven integer
  member, while an unobserved member type stays a third state, never a
  guessed non-integer) → the REGIONS bound (`<1` refuses, `>limit`
  refuses only against a proven limit, unproven limit stays a gap) →
  exec-time partition-member resolution that ignores names on a
  non-partitioned table exactly like the executor does. COMPACT
  adjudicates replica kind (only TIFLASH/ALL accepted; the bare form
  normalizes to ALL; TIKV is refused statically) → proven
  TiKV-compatible storage → explicit member resolution → the proven
  no-TiFlash arm, which upstream answers with a warning no-op and the
  rule reports as a bounded warning finding, never claimed compaction.
- **Undecidable premises emit named facts, not guessed outcomes.** The
  evidence gaps name `table.handle_columns`, `table.index_members`,
  `table.partitioned`, `table.partition_names`,
  `table.member_column_types`, `statement.split_value_foldability`,
  `instance.split_region_max_num`, and
  `instance.tikv_compatible_storage`; a non-table object that is proven
  non-auditable stays a bounded undecidable without a required-fact
  name. `split-region-max-num` loads through `SHOW CONFIG` under the
  same every-instance-must-agree contract as `max-index-length` —
  query/scan/iteration failures are provider errors, and inconsistent
  or invalid values keep the fact unknown rather than applying the
  documented 1000 default.
- **Neither statement publishes a post-state.** SPLIT moves region
  topology and COMPACT drives replica work — neither mutates a catalog
  fact — so the ordered-state publisher keeps the exact pre-state on
  every verdict, while the statements still participate in
  target-table existence/lifecycle guards through the shared
  `targetTableName` seam. MySQL keeps explicit vendor boundaries
  because the shared parser accepts TiDB syntax under the MySQL
  dialect flag.
