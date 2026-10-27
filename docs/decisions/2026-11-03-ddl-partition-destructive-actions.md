# Decision: Destructive partition actions (DROP/TRUNCATE/EXCHANGE PARTITION)

Date: 2026-11-03
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T13 under #92)
Related commits: this task's commits on the workfree line after `dcf250ea`
Related tests: `internal/infrastructure/parser/tidb/extractor_t13_test.go`,
`internal/domain/rule/ddl/partition_rules_test.go`,
`internal/application/audit/batch_state_t13_test.go`,
`make ddl-golden TASK=T13 ARTIFACT_DIR=...`, `testdata/ddl-golden/T13.json`,
corpus fixtures `testdata/sql-corpus/{mysql,tidb}/ddl/findings/alter_table_partition_destructive.sql`
Related docs: `docs/work-plans/t13-partition-risk/CONTRACT.md`, module
README updates in the same change

## Context

Before T13, `ALTER TABLE ... DROP/TRUNCATE/EXCHANGE PARTITION` parsed
silently with zero findings — the named parse-only defect the inventory
recorded. Three of the four anchors were probed against real partitioned
fixtures before the contract froze:

- DROP PARTITION removes the named partition and its rows (a missing name
  refuses 1507, the last partition refuses 1508, a non-partitioned table
  1505, HASH partitioning 1512); TRUNCATE PARTITION keeps definitions and
  removes rows (`ALL` form included; HASH supports it); EXCHANGE swaps
  rows both ways (`WITH VALIDATION` refuses an out-of-range row — MySQL
  1526, TiDB 1737 — while `WITHOUT VALIDATION` and the omitted form land
  it; refusals 1736/1732/1146/1735 observed).
- A partition action cannot join a multi-spec ALTER: MySQL refuses 1064
  on `ADD COLUMN ..., DROP PARTITION ...`; TiDB parses the same shape but
  refuses 8200. Both are proven whole-statement refusals.
- The model carries no partition member catalog — `spec.TableSnapshot`
  exposes only `Partitioned *bool`; member names, types, and bounds are
  never statically proven.

## Decision

1. **Normalized payload on `spec.Alter`.** `Partitions` (as-written
   lower-cased names in clause order), `AllPartitions` (distinguishes
   `TRUNCATE PARTITION ALL` from an empty list), `ExchangeTable`, and
   `ExchangeValidation` — `"without"` only for explicit `WITHOUT
   VALIDATION`; `"with"` covers explicit `WITH` and the omitted form
   because the PingCAP AST collapses both to `WithValidation=true`.
   `alter.Name` is the first partition name; `DDL.Targets` gains the
   EXCHANGE counterpart as an *existing* object (unlike a RENAME
   destination, which is a new name) so `auditmeta` metadata resolution,
   the denylist, and affected-identity tracking all see both sides.
2. **Three frozen rules.** `ddl.alter.drop_partition.forbid` and
   `ddl.alter.truncate_partition.forbid` are default blockers (one
   finding per named partition; `ALL` emits one finding carrying
   `all_partitions:true` and no `partition` key; `forbid:false`
   disarms), and `ddl.alter.exchange_partition.warn` is a default
   warning carrying `exchange_table`/`exchange_schema`/`validation`.
   Findings use statement-level locations — the model has no
   per-partition spans. Rules fire on the authored action independent of
   metadata availability and native executability; disabled rules remove
   findings but never lift the capability-coverage marking (the three
   actions are in `auditedAlterActionsMySQL`, inherited by TiDB).
3. **Ordered state.** A multi-spec ALTER containing any partition
   action keeps the exact pre-state — a proven refusal on all anchors.
   A successful single action keeps every member fact (columns, indexes,
   constraints, `Partitioned` — natively impossible to drop the last
   partition) and clears only data-derived statistics (`table_rows`,
   `auto_increment`, `data_length`, `index_length`, `avg_row_length`,
   index cardinality) through the shared mutation-stat path; EXCHANGE
   applies the same invalidation to both named identities. Unextracted
   partition payloads fall back to tombstone invalidation.
4. **Evidence boundaries.** Partition existence, partition-type
   applicability, and validation compatibility are never statically
   proven — `Partitioned=false` can prove a refusal class but never
   proves members. Missing exchange-side metadata is never treated as
   validation success. `WITH VALIDATION` explicit-vs-absent is
   indistinguishable in the AST; only the effective `with`/`without`
   fact is recorded.

## Deferred scope

T14 keeps partition topology/definition/maintenance: ADD/REORGANIZE/
COALESCE/REMOVE PARTITIONING, member catalogs, and bounds stay
parse-only under `tidb.alter-table.partition-ops` (owner T14) and the
MySQL `partition-topology`/`partition-maintenance` rows.
