# T09 closeout — unified standalone & nested index governance

Issue: https://github.com/Fanduzi/DeltaScope/issues/88 (parent #79, task T09)
Status: implemented, golden PASS at the frozen contract, evidence archived.
No merge/release/PR claim is made — internal milestone evidence only.

## Bound code SHA and commit chain

- Final code SHA: `51c8eec3a05db087b92f014b6ccc9378c5746dda`
  (`milestone/mysql-tidb-ddl-completion`).
- Chain since `1ce23297` (T08 closeout):
  - `016ed8d8` docs(t09): freeze index-governance contract for issue #88
  - `f2ea7b43` feat(ddl): unify standalone and nested index governance
    with ordered index state (T09)
  - `a11b448b` test(ddl-golden): T09 frozen index-governance contract
    oracle
  - `51c8eec3` fix(ddl): regenerate coverage catalog for T09 standalone
    index-rule coverage
- Task-range diff: `gates/task-range-diff.txt` in the evidence archive.

## What was delivered

- Unified declared-index accessor (`declaredIndexes`) joins `DDL.Indexes`
  with standalone `create_index` action definitions — every `ddl.index.*`
  rule (columns.max_count, secondary/unique/fulltext prefix, name.pattern,
  name.keyword, duplicate, left-prefix, unique-overlap) now fires
  identically on create-table, standalone `CREATE INDEX`, and ALTER
  `ADD INDEX` forms.
- Index-syntax vs constraint-syntax naming split:
  `ddl.alter.add_index.*` naming judges `ADD INDEX`/`ADD KEY`/
  `ADD UNIQUE KEY`; `ADD CONSTRAINT ... UNIQUE` is judged by
  `ddl.constraint.unique_key.name.prefix.require` under MySQL-family
  dialects. PostgreSQL keeps constraint-path attribution (dialect gate).
- `batch_state_index.go` ordered post-state for single-action
  `add_index`, index-producing `add_constraint`, `drop_index`,
  `rename_index`, `drop_primary_key`, and standalone `DROP INDEX`:
  known-present removes/renames publish; known-absent without `IF EXISTS`
  invalidates; conditional over known absence no-ops; unknown presence
  stays present-incomplete. Loaded FK dependents survive index
  transitions; multi-action ALTERs keep the tombstone path.
- Standalone lifecycle coverage: operation-driven standalone detection,
  `Definition`/`OldName` name fallback, `if_exists`/`if_not_exists`
  markers on standalone index actions; standalone `DROP INDEX` now
  reaches `ddl.alter.drop_index.forbid`.
- `ddl.alter.add_index.redundant_unique_overlap.forbid` un-deferred with
  real corpus coverage on both dialects; deferred denominator is now 1.
- Frozen contract oracle: `scripts/t09_contract.py` →
  `testdata/ddl-golden/T09.json` (125 identities, ids-sha256
  `1f7c3ea3dd95c217d1c894cddd537db73606d6dd9fc81d41f2280f5d81ffc805`),
  wired into `validate_artifact`; `scripts/test_t09_contract.py` = valid
  constructed control + 35 adversarial rejections.
- Coverage catalog regenerated (`51c8eec3`): standalone FULLTEXT/UNIQUE
  CREATE INDEX rows now list their prefix-rule coverage on the
  `ddl-coverage` CLI surface.

## Native facts recorded (independent probes)

- `RENAME INDEX` accepted on all four anchors including MySQL 5.7.44 —
  the earlier contract-design assumption that 5.7 rejects was corrected
  to actual acceptance.
- `ADD INDEX IF NOT EXISTS` / `DROP INDEX IF EXISTS`: ERROR 1064 on all
  three MySQL anchors; accepted on TiDB 8.5.0.
- `DROP PRIMARY KEY` accepted on MySQL anchors; TiDB rejects clustered-
  index PK drops with ERROR 8200.
- `STATISTICS.IS_VISIBLE`: absent on MySQL 5.7.44; present on 8.0.46,
  8.4.10, TiDB 8.5.0 — drives per-anchor oracle query selection.
- Error identities: duplicate name 1061; missing column 1072; missing
  index 1091 (DROP) / 1176 (RENAME) on all anchors.

## Evidence archive

`docs/verification/t09/51c8eec3a05db087b92f014b6ccc9378c5746dda/`:

- `golden/T09/` — full runner artifact: artifact.json, 125 per-case
  records, rendered policies, runner-built binary record (sha256
  `bfd3212af6755a119a73fcf2a823c0b000d63fd9b46b4a53de8452c54a1c043a`,
  CGO_ENABLED=0, head 51c8eec3).
- `golden-run.log` — run/validate output and run history (one fixture-
  contamination run included and explained).
- `gates/` — make test, sql-corpus-gates, ddl-golden-validator-test
  (758 generic + T07 + T08 25/25 + T09 36/36), ddl-inventory-gate,
  ddl-coverage-catalog-test, pg-unit-test-gates, docs-example-gates,
  decision-record-gate, three-level-doc, gofmt, task-range-diff —
  all rc=0 at the bound SHA.
- `native-probe/` — probe.py, native-matrix.json/.txt, RUN.txt:
  independent per-anchor cells for rename acceptance, IF-variant grammar,
  error identities, IS_VISIBLE capability, drop-primary outcomes, and
  cleanup (residual owned tables = 0 everywhere).
- `identity/` — binary sha256/version, go version, git status.

## Deferred / recorded boundaries

- Optimizer benefit estimation, EXPLAIN/cost analysis, index
  recommendations, caches — no performance claims.
- Full functional/FULLTEXT/SPATIAL/vector semantics; SUB_PART/expression/
  descending key parts remain declared-side facts.
- New index-registration framework; complete FK/CHECK/PK migration
  engineering — sibling-ticket scope.
- `t09-err-cancel` and `t09-err-driver-exec` remain named unit-test roles,
  not golden cells.
- Deferred corpus denominator: 1 (PostgreSQL-rewrite carry) —
  `corpus_coverage_test.go`.

## Decision record

`docs/decisions/2026-10-22-ddl-index-governance-unification.md` — public
contract, rationale, deferred scope, and verification links.
