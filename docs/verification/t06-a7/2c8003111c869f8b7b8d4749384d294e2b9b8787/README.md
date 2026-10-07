# T06-A7 evidence: column charset/collation isolation proof + table COLLATE policy

Tested code commit: `2c8003111c869f8b7b8d4749384d294e2b9b8787`
(`feat(ddl): add table collation allowlist and prove column
charset/collation rules (#85)`). This directory is the evidence-only commit
on top of it — the evidence HEAD is not the tested HEAD.

## Scope

- New rule `ddl.table.collation.allowlist` (default disabled, `blocker`,
  `values=[utf8mb4_bin]`, `require_explicit=true`), reusing the existing
  `tableOptionAllowlistRule` implementation on `DDL.Options["collate"]`.
- Coverage reclassification: `extractedOptionGap` no longer reports
  `create_table.option.collate` for `CreateTable`. CREATE/ALTER DATABASE
  `COLLATE` keep the schema-level unsupported boundary; ALTER TABLE
  unchanged (deferred to T15/#94).
- Isolated proof of the pre-existing column rules
  `ddl.column.charset.allowlist`, `ddl.column.collation.allowlist`,
  `ddl.column.charset_collation.match.require` — unchanged behavior, now
  pinned per rule.
- Declared-vs-resolved distinction: empty `Column.Charset`/`Collation`
  means "not declared"; anchored `information_schema` values show native
  inheritance. No inheritance inference added.
- ADR: `docs/decisions/2026-10-07-ddl-create-table-collation-policy.md`.

## Contents

- `artifact.json` — formal `make ddl-golden TASK=T06` artifact:
  **188/188 cases, 4477 assertions, head_sha=2c8003111c86**, cleanup rc=0.
- `cases/` — all 188 per-case records (sql/argv/stdout/stderr/rc, parsed
  JSON, structure/query/step records, version evidence).
- `policies/` — the 14 generated policy files the run used, incl. the five
  A7 isolated profiles (`t06-a7-column-charset-isolated`,
  `t06-a7-column-collation-isolated`, `t06-a7-column-match-isolated`,
  `t06-a7-table-collation-optional`, `t06-a7-table-collation-required`)
  and the all-rules-disabled control.
- `T06-manifest.json` — frozen manifest at 2c800311.
- `anchors.json` — per-anchor identity (product/image/digest/container/
  version) extracted from executed records, artifact cli sha256, policy
  hashes, head_sha, cleanup verdict. Binary not stored; `build-identity.txt`
  carries `go version -m` and the recomputed sha256 (matches artifact).
- `baseline-f8326032/` — the read-only A7 baseline at old HEAD: eight CLI
  runs (S1–S4 × mysql/tidb), probe source+output, catalog.json, build
  identity. S4 shows the real pre-change behavior (`review`/`incomplete`,
  `create_table.option.collate`).
- `s4-green/` — the same S4 input rerun at the tested SHA under the
  artifact's own all-off policy: `pass`/`complete`, 0 unsupported, rc 0,
  both dialects. This is the true red→green pair for the coverage upgrade.
- `EXPECTED-UPGRADES.md` — itemized old→new table for every expectation
  touched by the reclassification.
- `regressions/T02..T05/` — full Golden re-runs at the tested SHA (shared
  coverage/catalog changed): T02 10/10 (57 assertions), T03 118/118
  (1395), T04 163/163 (1899), T05 247/247 (5122); each artifact passes
  `ddl_golden.py validate` (`gates/golden-validate-*.rc.txt`).
- `gates/` — argv/stdout/stderr/rc captures:
  `gofmt-check`, `focused-t06a7` (rule+audit T06A7), `focused-parser-t06a7`,
  `focused-entry-t06a7` (SDK/HTTP/MCP), `make-test` (`GOFLAGS=-count=1`),
  `pg-unit-test-gates`, `sql-corpus-gates`, `ddl-inventory-gate`,
  `ddl-coverage-catalog-test`, `docs-example-gates`,
  `decision-record-gate`, `ddl-golden-validator-test` (335 contract cases,
  0 failures), `golden-run.log`, `golden-validate-{t06,T02..T05}`,
  `task-diff` (`f8326032..HEAD` name-only). All rc=0.

## Notes

- Compiled binaries are deliberately excluded (hashes recorded instead).
- No secrets, DSNs, or credentials are present; golden policies and case
  records use the ephemeral per-run container endpoints.
- Validator mutations added in this slice cover: charset/collation rule-ID
  swaps, reject-recorded-as-pass, native-success-recorded-as-failure,
  wrong ERROR 1253 substitutes, table/column oracle swaps,
  inherited-as-declared fact substitution, retired unsupported evidence,
  schema-boundary deletion, wrong-profile/off-strict/cross-anchor/offline
  degradations, and paired manifest+artifact deletions.
