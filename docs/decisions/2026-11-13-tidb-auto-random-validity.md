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
