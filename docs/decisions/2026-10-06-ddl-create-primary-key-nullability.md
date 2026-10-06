# Decision: T06-A2 CREATE TABLE primary-key member nullability normalization

Date: 2026-10-06
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (#79), task T06 / issue #85
Related commits: T06-A2 implementation commit on milestone/mysql-tidb-ddl-completion
Related tests: `internal/infrastructure/parser/tidb/extractor_t06a2_test.go`, `internal/application/audit/batch_state_t06a2_test.go`, `pkg/deltascope/audit_create_table_pk_nullability_t06a2_test.go`, `internal/interfaces/http/audit_create_table_pk_nullability_t06a2_test.go`, `scripts/test_ddl_golden.py` T06-A2 contract group
Related docs: `testdata/ddl-golden/T06.json`, `testdata/ddl-inventory/T06-base-traceability.md`, `docs/decisions/2026-08-30-mysql-tidb-column-primary-key-extraction.md`, `docs/decisions/2026-10-06-ddl-create-table-pk-golden-proof.md`

## Context

T06-A2-PLAN reconciled NULL and explicit `DEFAULT` base facts on CREATE TABLE
and found one real modeling asymmetry. The inline `PRIMARY KEY` column option
already records the database-implied non-null fact (`extractColumn` sets
`Column.NotNull=true`, an accepted decision from 2026-08-30), but a
table-level `PRIMARY KEY (...)` only populated `DDL.PrimaryKey` and left
member columns `NotNull=false`. The faithful clone in `applyCreateTable` then
carried the wrong fact into follower statements, where the
modify-column compatibility rule fabricated a "nullable → not null" tightening
finding.

MySQL marks primary-key members NOT NULL regardless of the written clause and
explicitly rejects `NULL` on key columns (5.7.3 release notes; 8.4 CREATE
TABLE reference); TiDB keeps the same ERROR 1171 boundary (release-8.5
constraints docs). So the missing fact was a source-level normalization gap,
not a rule bug.

## Decision

`extractCreateTable` runs a private normalization pass,
`normalizeCreatePrimaryKeyNullability`, after columns and the primary-key
declaration are collected:

- every `DDL.PrimaryKey.Columns` member that binds to a declared column
  (normalized lowercase names; single, composite, or key order ≠ column
  order) records `NotNull=true` — the same implied fact inline `PRIMARY KEY`
  already carried;
- an explicit `NULL` declaration (`ast.ColumnOptionNull`) keeps the conflict
  visible: the member stays `NotNull=false` so the existing
  `ddl.table.primary_key.not_null.require` rule reports it — the model never
  pretends a nullable primary key was created (both engines reject the DDL);
- non-PK columns are untouched, and members that do not bind to a declared
  column (expression parts, unknown names) are never guessed or fabricated.

The pass is CREATE-scoped: shared `extractColumn` semantics used by ALTER and
other statements are unchanged. No rule, policy, spec field, provider, state
machine, or transport changed. `DEFAULT NULL` is a default clause, not a NULL
nullability declaration — a `DEFAULT NULL` PK member records `NotNull=true`
and keeps its default facts.

`testdata/ddl-golden/T06.json` extends the frozen T06 set to 56 cases: the
accepted 32 A1 cases verbatim plus 24 A2 cases (8 offline CLI controls —
legal table-single/composite and no-default/default-null per dialect — and 16
anchored metadata cases pinning pass/complete plus real `IS_NULLABLE=NO`,
ordered composite members `b:1,a:2`, and ERROR 1171 native negatives with
post-failure absence on all four anchors). The validator binds the same
frozen identity map to the extended set and gains mutations for masking,
structure forgery, and negative laundering.

## Public Contract

Consumers can rely on: a declared `PRIMARY KEY` — inline, single, or
composite table-level — makes its bound member columns `NotNull=true` in the
normalized model and in the CREATE-derived prospective pre-state, so
`ddl.table.primary_key.not_null.require` and `ddl.column.not_null.require`
stay silent for them and `ALTER ... MODIFY COLUMN ... NOT NULL` on a member
no longer fabricates a tightening finding. An explicit `NULL` member keeps
the conflict: exactly one `primary key column "…" must be NOT NULL` blocker
(reject, exit 1 under `--fail-on blocker`), and the driver-side CREATE fails
with ERROR 1171 — policy rejection and native legality remain independent
claims.

## Deferred / Out Of Scope

- `DefaultValue` records the bounded literal `"<nil>"` for `DEFAULT NULL` and
  `DefaultIsNull` is never set on the MySQL/TiDB path (`DefaultKind` is
  PostgreSQL-only): explicit-NULL defaults are distinguishable from absent
  clauses only via `HasDefault`. Tightening this fidelity is a separate slice.
- `ColumnOptionNull` is a no-op marker on non-PK columns, so "declared NULL"
  vs "omitted" is only observable on primary-key members today.
- Expression defaults, generated/invisible columns, temporal implicit
  defaults, CHECK, LIKE/SELECT lifecycle, and full table options stay with
  their owning tasks (T07/T12/T15/T16).

## Verification Evidence

- `go test -run T06A2` on parser/audit/SDK/HTTP — member normalization matrix
  (inline/table/composite/reordered/prefix/mixed-case), explicit-NULL
  conflict preserved in both forms, non-PK controls, shared `NOT NULL`
  consumer, pre-state consumption, and unchanged default facts.
- `python3 scripts/test_ddl_golden.py` — 257 contract cases, 0 failures,
  including 13 T06-A2 mutations (inline masking a table-level slot, composite
  nullability/spare/order forgery, explicit-null recorded as pass, driver
  success or permission error masquerading as ERROR 1171, DEFAULT NULL
  recorded as missing and absent DEFAULT recorded as pass, paired-side
  deletion, cross-anchor rebind, offline demotion, profile swap).
- `make ddl-golden TASK=T06` — 56 cases across the four locked anchors
  (evidence recorded with the task evidence commit).

## Consequences

- `Column.NotNull` on CREATE TABLE now means effective database nullability
  for primary-key members, matching what `information_schema` would report;
  consumers that previously read the literal "was NOT NULL written" meaning
  for PK members must use the declaration options instead.
- Corpus `rule_coverage_offline` no longer expects `pk-not-null`/
  `column-not-null` findings on `badpk` members; the dedicated
  `primary_key_explicit_null` fixtures keep the real negative.
- Later CREATE TABLE slices extend `T06.json` through the same frozen
  contract; the A1 subset stays byte-identical in oracle shape.

## Links

- Tests: `internal/infrastructure/parser/tidb/extractor_t06a2_test.go`,
  `internal/application/audit/batch_state_t06a2_test.go`,
  `pkg/deltascope/audit_create_table_pk_nullability_t06a2_test.go`,
  `internal/interfaces/http/audit_create_table_pk_nullability_t06a2_test.go`
- Docs: `testdata/ddl-golden/T06.json`,
  `testdata/ddl-inventory/T06-base-traceability.md`,
  `testdata/sql-corpus/{mysql,tidb}/ddl/findings/primary_key_explicit_null.*`
