# Decision: Request-local ordered schema state for the first MySQL/TiDB migration path

Date: 2026-10-02
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T05-A1 under #84; predecessor decision `2026-09-23-ddl-incomplete-coverage-contract.md`)
Related commits: this task's commit on milestone/mysql-tidb-ddl-completion
Related tests: `internal/application/audit/batch_state_test.go`, `internal/domain/rule/ddl/metadata_rules_test.go`, `internal/interfaces/{cli,http,mcp}/...offline_existence...`, `make ddl-golden TASK=T05 ARTIFACT_DIR=...`
Related docs: `testdata/ddl-golden/T05.json`, `docs/reference/rules.md`, `docs/reference/audit-capability-matrix.md`, `internal/application/audit/README.md`, `internal/domain/rule/ddl/README.md`, `internal/domain/policy/README.md`

## Context

Before this slice, metadata enrichment loaded each statement's `TargetTable`
snapshot lazily at first touch and cached it by `schema.table` for the rest of
the request. In a multi-statement batch every statement therefore evaluated
against the **pre-batch** provider view: `ALTER TABLE t ADD COLUMN c`
immediately after `CREATE TABLE t` reported "table t does not exist", and a
standalone `CREATE INDEX` never received a snapshot at all (`targetTableName`
returned `""`), so its column references were completely unchecked. Offline
audits silently skipped every existence rule, so the canonical migration path
`CREATE → ADD COLUMN → CREATE INDEX` could not be honestly evaluated either
online or offline.

## Decision

1. **Ordered, request-local schema state replaces first-touch snapshot
   caching.** `internal/application/audit/batch_state.go` maintains a
   per-request `batchState` keyed by (dialect, schema, table). Identity
   matching is case-normalized; provider calls keep caller-supplied casing and
   rendered snapshots keep display casing. Each statement's rules see an
   immutable **pre-state** projection; the entry's owned copy is mutated only
   by conditional post-state derivation after evaluation inputs are fixed.
   Each independent table (and each non-table object lookup) is resolved at
   most once per request.
2. **Four states, kept distinct.** `never-touched` (no entry), `unknown`
   (entry exists but facts are unusable — provider absent, invalidation,
   contamination), `known-absent`, and `known-present` (complete or
   incomplete structural shape). "We cannot prove anything about this table"
   is never collapsed into "the table does not exist".
3. **Conditional derivation only.** Fully audited statements derive
   deterministic effects: `CREATE TABLE` → present shape; `CREATE TABLE IF
   NOT EXISTS` → preserve present, else present-incomplete; `ALTER TABLE ADD
   COLUMN` → append to a complete shape; standalone `CREATE INDEX` → append
   index to a complete shape; `DROP TABLE` → absent; `TRUNCATE` → keep shape,
   clear row/index statistics; DML → clear row/auto-increment/cardinality
   statistics. A broken premise (duplicate plain `ADD COLUMN`, `CREATE INDEX`
   on a confirmed-absent table, `DROP` on confirmed-absent) **invalidates**
   the entry instead of fabricating a success. Statements that are not fully
   audited (unaudited aspects, `rename_table`, unmodeled mutations)
   invalidate every touched entry. Parse failures contaminate all later
   statements in the batch. Policy blockers never erase a deterministic
   transition.
4. **Whitelisted existence rules emit evidence gaps instead of silently
   skipping.** `ddl.table.exists.create.forbid`,
   `ddl.table.exists.alter.require`, `ddl.alter.add_column.exists.forbid`, and
   the new `ddl.create_index.columns.exists.require` implement
   `EvidenceReporter`: `unknown_table_state` for never-touched/unknown
   entries, `incomplete_table_structure` for present-but-shape-incomplete
   snapshots. A gap lowers an otherwise-complete statement to
   `coverage=unverified` (existing T03/T04 machinery); provider errors remain
   errors and are never laundered into gaps.
5. **New rule `ddl.create_index.columns.exists.require`** (blocker by
   default, `required` param): `table_not_found` single blocker on a
   confirmed-absent target, deduplicated `missing_column` blockers in source
   order on a complete shape, evidence gaps on unknown/incomplete state. It
   consumes `DDL.Table` + `Alter[create_index].Index.Definition.Columns`
   directly and does not masquerade as CREATE TABLE.
6. **Extractor records existence-clause markers.** `if_not_exists` on
   `create_table`, `if_exists`/`if_not_exists` on alter specs — the
   conditional-existence semantics feed post-state derivation.

## Public Contract

- Offline MySQL/TiDB `CREATE TABLE` / `ALTER TABLE` / standalone
  `CREATE INDEX` on never-touched tables now report `coverage=unverified`
  with `unknown_table_state` gaps instead of silently passing; aggregate
  verdict floors to `review`. This intentionally replaces the issue-#28
  "existence was not checked" silent-skip contract on CLI, HTTP, and MCP —
  the `context.note` / `context.unproven` fields are preserved.
- The canonical first path `CREATE TABLE t; ALTER TABLE t ADD COLUMN c;
  CREATE INDEX idx_c ON t(c)` audits to `complete` with zero findings when
  the whitelisted rules see the derived state (online and offline alike for
  statements 2–3; statement 1 stays `unverified` offline because prior
  existence cannot be proven).
- `required_facts` vocabulary additions: `existence`, `columns`,
  `table_structure` under the existing `target_table.*` naming.
- No public Go types change: the ordered state is private to the audit
  orchestrator; `spec.TableSnapshot` is untouched.

## Deferred / Out Of Scope

- `RENAME TABLE` / `ALTER TABLE ... RENAME` migration of state is deferred:
  both ends are invalidated (conservative), and this slice does **not** claim
  rename support.
- Broader post-state modeling (DROP COLUMN, MODIFY, constraints, multi-action
  alters beyond `add_columns`/`add_index`) is deferred to later T05 slices;
  they invalidate rather than fabricate.
- PostgreSQL keeps its existing metadata-enrichment path unchanged.
- The remaining metadata rules keep their silent-skip behavior on unusable
  snapshots; gap promotion beyond the four whitelisted rules is future work.
