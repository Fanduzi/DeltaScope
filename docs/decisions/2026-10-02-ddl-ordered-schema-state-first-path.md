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
3. **Conditional derivation only — the frozen success templates.** Fully
   audited statements derive deterministic effects: `CREATE TABLE` →
   present shape; `CREATE TABLE IF NOT EXISTS` on an unknown target →
   present-incomplete (existence proven, every member collection unknown);
   `ALTER TABLE` made of **exactly one** plain `ADD COLUMN` → append to a
   complete shape; standalone `CREATE INDEX` → append index to a complete
   shape; `TRUNCATE` → keep a known-present shape, clear row/index
   statistics; DML → clear row/auto-increment/cardinality statistics.
   `DROP TABLE` has **no** definite-absence derivation in this slice — a
   precise drop transition is deferred, so it invalidates its targets like
   every other unmodelled statement. A broken premise (duplicate plain
   `ADD COLUMN`, `CREATE INDEX` on a confirmed-absent table) **invalidates**
   the entry instead of fabricating a success. Statements that are not fully
   audited (unaudited aspects, `rename_table`, unmodeled mutations,
   multi-action `ALTER TABLE`) invalidate every touched entry. Parse
   failures contaminate all later statements in the batch. Policy blockers
   never erase a deterministic transition.
4. **Whitelisted existence rules emit evidence gaps instead of silently
   skipping.** `ddl.table.exists.create.forbid`,
   `ddl.table.exists.alter.require`, `ddl.alter.add_column.exists.forbid`, and
   the new `ddl.create_index.columns.exists.require` implement
   `EvidenceReporter` with the frozen `unknown_table_state` reason —
   `[target_table.columns, target_table.existence]` on unknown entries and
   `[target_table.columns]` on present tables whose column set was never
   provided. `ddl.alter.drop_primary_key.exists.require` additionally
   reports `target_table.primary_key` gaps. A gap lowers an
   otherwise-complete statement to
   `coverage=unverified` (existing T03/T04 machinery); provider errors remain
   errors and are never laundered into gaps.
5. **New rule `ddl.create_index.columns.exists.require`** (blocker by
   default, `required` param): `table_not_found` single blocker on a
   confirmed-absent target, deduplicated `missing_column` blockers in source
   order on a complete shape, evidence gaps on unknown state. It
   consumes `DDL.Table` + `Alter[create_index].Index.Definition.Columns`
   directly and does not masquerade as CREATE TABLE.
6. **Unsupported and executable effects settle before kind dispatch.** A
   recognized-but-unsupported statement can never pass through as a no-op:
   bound table targets invalidate, a bound schema scope (`DROP`/`ALTER
   DATABASE`) invalidates every cached entry and blocks later provider
   reads under it, and an unresolvable scope contaminates the batch. DML
   mutation targets bound the same way.
7. **Completeness is tracked per collection, not per shape.** A provider
   snapshot that supplies the primary key but withholds `Columns` keeps the
   primary key — member rules read each collection's own knowledge flag
   (`Columns == nil` marks a withheld column set; `PrimaryKeyUnknown` /
   `IndexesUnknown` / `ConstraintsUnknown` mark derived-incomplete
   projections). Unknown member state is a gap, never a fabricated
   "does not exist" finding.
8. **Extractor records existence-clause markers.** `if_not_exists` on
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
- `required_facts` vocabulary additions: `existence`, `columns`, and
  `primary_key` under the existing `target_table.*` naming.
- `spec.TableSnapshot` gains three internal markers
  (`PrimaryKeyUnknown`, `IndexesUnknown`, `ConstraintsUnknown`, all
  `json:"-"`) distinguishing "collection not provided" from "collection
  loaded empty". No serialized output changes.
- PostgreSQL never enters the ordered-state path — with or without a
  request, schema, provider, or target version it keeps its existing
  schema/version/object enrichment.

## Deferred / Out Of Scope

- `RENAME TABLE` / `ALTER TABLE ... RENAME` migration of state is deferred:
  both ends are invalidated (conservative), and this slice does **not** claim
  rename support.
- Broader post-state modeling (DROP COLUMN, MODIFY, constraints,
  multi-action `ALTER`, and the definite-absence `DROP TABLE` transition)
  is deferred to later T05 slices; they invalidate rather than fabricate.
- PostgreSQL keeps its existing metadata-enrichment path unchanged.
- The remaining metadata rules keep their silent-skip behavior on unusable
  snapshots; gap promotion beyond the whitelisted rules is future work.

## R1 corrections (review rework)

The first implementation was reviewed and reworked (`T05-A1-R1`):

- Effective-identity resolution was unified: the explicit table qualifier
  now wins over the request schema for provider reads, state keys,
  post-state writes, and invalidation — previously a qualified `a.t`
  collided with `b.t` under the request schema.
- Unsupported statements settle before kind dispatch; unbound executable
  effects (`EXECUTE`, unbound admin mutations) contaminate instead of
  passing as no-ops, and `DROP`/`ALTER DATABASE` invalidate their schema
  scope.
- The `ALTER TABLE` success template was narrowed to exactly one plain
  `ADD COLUMN` — multi-action alters invalidate rather than fabricating a
  column set from a deduplicated write-back; the self-added `DROP TABLE` →
  absent derivation was withdrawn (deferred, not frozen).
- Per-collection completeness replaced the single `shapeComplete` flag so
  a provider-partial snapshot keeps its known primary key/indexes; the
  slice-local `incomplete_table_structure` reason code was removed in
  favour of the frozen `unknown_table_state` vocabulary.
- PostgreSQL was pinned off the ordered path under every request/provider
  shape (the schema-only, provider-less branch had leaked in).
- R2 residuals closed: `alterObjectExistenceRule` now shares one knowledge
  premise between `Evaluate` and `EvidenceGaps`, so every column/index
  member-existence rule (`add`/`drop`/`modify`/`change`/`rename` column,
  `add`/`drop`/`rename` index) reports `unknown_table_state` on an unknown
  collection — a silent skip can no longer count as a completed check.
  Frozen fact ordering pins `[columns, existence]` for column members and
  `[existence, indexes]` for index members, matching
  `target_table.primary_key` for the primary-key rule.
- The new gap projection is dialect-gated at the rule layer
  (`orderedGapDialect`): PostgreSQL keeps its legacy finding/skip contract
  on every request shape — provider-backed findings still fire, and no
  `unknown_table_state` gap ever appears.
- Golden acceptance was completed for all four anchors: every anchor's
  first-path metadata case now carries the full six-phase oracle — the
  fixture confirms `t` absent, runs the CLI audit (which must not mutate),
  applies the three audited statements through the test driver, verifies
  the resulting column `c`, index `idx_c`, and `PRIMARY(id)` via
  `information_schema`, and cleans up with a residual check. The runner
  gained `execute`/`structure` phases and `verify` on `setup`/`teardown`;
  the validator re-derives every recorded step from the manifest, so a
  dropped statement, a moved finding, a falsely-completed invalidated
  statement, a tampered oracle answer, or a failed execute step recorded as
  success all fail validation.
