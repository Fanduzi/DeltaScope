# T06-A2 verification evidence — CREATE TABLE primary-key member nullability

Code commit under test: `9d12aec81ecdca0be94252ae6b568127817cdf5d`
(`fix(parser): normalize CREATE TABLE primary-key member nullability`, #85/T06-A2).
Parent: `4332dcbd7c66b16daf51356f69890bfac28ec1a2` (T06-A1-R1 evidence).
Evidence commit: this file's own commit (separate from the code commit; the
artifact's `head_sha` is `9d12aec8`).

## Contents

- `artifact/` — final four-anchor run output: `artifact.json` (56 cases,
  1146 assertions, `head_sha=9d12aec8`), `cases/` (56 raw case records with
  argv, audit stdout/stderr, driver-side execute/structure/verify steps,
  container + image-digest + version evidence, teardown), and the five
  generated isolated policies (`golden-policy-*.yaml`).
- `plan-baseline/` — **old baseline from the T06-A2-PLAN phase** (built at
  `4332dcbd`, pre-fix): 8 isolated-policy CLI runs
  (`{a,b}-{mysql,tidb}-*.{argv,stdout.json,stderr,rc}`), `catalog.json`,
  and the two plan-phase policies. Kept as comparison originals only; they
  predate the fix and are not re-run evidence.
- `red/` — genuine pre-fix failures on the task base (`/tmp/ds-t06-a2-impl`,
  checkout at `4332dcbd` + the new tests only): `parser-red.txt` /
  `audit-red.txt` show table-level members `NotNull=false`, the fabricated
  `MODIFY ... NOT NULL` tightening finding, and the inline explicit-NULL
  miss. `pre.json`/`post.json`/`pre.err`/`post.err` record the corpus
  before/after audit used for the finding-delta accounting below.
- `gates/` — per-gate `argv`/`stdout`/`stderr`/`rc` triples, all rc=0 on
  `9d12aec8` (see table below).

## Root cause and fix (precise diff)

`extractCreateTable` extracted `DDL.PrimaryKey` for both inline and
table-level forms, but only the inline `ColumnOptionPrimaryKey` path marked
`Column.NotNull=true`. A private CREATE-scoped pass
(`normalizeCreatePrimaryKeyNullability`) now binds `DDL.PrimaryKey.Columns`
to declared columns by normalized name after collection: bound members get
`NotNull=true` (single/composite, key order ≠ column order); an explicit
`NULL` declaration keeps the conflict (`NotNull=false` → the existing
pk-not-null blocker, matching MySQL 5.7.3+/8.4 and TiDB ERROR 1171);
non-members are untouched; `extractColumn` (shared with ALTER) is unchanged.
Production diff: `internal/infrastructure/parser/tidb/extractor.go` only
(+45/-0).

## Corpus finding delta (measured, statement `badpk`)

Pre/post audits in `red/` diffed per (statement, rule_id):

- removed `ddl.table.primary_key.not_null.require` on `id`, `bad_varchar`
  (table-level composite members now correctly NOT NULL);
- removed `ddl.column.not_null.require` on the same two columns;
- added: none. All other fixture findings unchanged; the dedicated
  `primary_key_explicit_null` fixture keeps the rule's real negative.

## Gates (all rc=0, captured in `gates/`)

`focused-t06a2` (3 pkgs `-run T06A2 -v`), `sdk-http-t06a2`,
`validator-test` (257 contract cases, 0 failures), `golden-run`
(four anchors, 56/56 cases, 1146 assertions), `validate-artifact`
(`artifact valid`), `make-test` (`GOFLAGS=-count=1`),
`make-pg-unit-test-gates`, `make-sql-corpus-gates`,
`make-ddl-inventory-gate`, `make-ddl-coverage-catalog-test`,
`make-docs-example-gates`, `make-decision-record-gate`, `gofmt`
(no files listed), `task-diff-stat`, `source-identity`.

## Boundaries honestly recorded

- Product `pass`/`reject` are offline policy verdicts; native legality is
  proven only by the driver-side `execute` steps inside `cases/` (success
  with `IS_NULLABLE=NO` structure, or ERROR 1171 + post-failure absence).
- `DEFAULT NULL` extraction keeps its pre-existing bounded fidelity
  (`HasDefault=true`, `DefaultValue="<nil>"`, `DefaultIsNull=false`) —
  deferred, not silently fixed.
- `bin/` build output, credentials, and container internals are not
  committed; `connect.password_env` records the env name only.
- Temporary original directories (`/tmp/ds-t06-a2.eznhqT`,
  `/tmp/ds-t06-a2-impl`, `/tmp/ds-t06-a2-golden*`) exist only in the
  author's environment; this directory is the committed copy.
