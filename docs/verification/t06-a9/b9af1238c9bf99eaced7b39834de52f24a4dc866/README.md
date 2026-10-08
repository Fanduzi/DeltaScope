# T06-A9 evidence: explicit audit-time-column roles — facts, not names

Tested code commit: `b9af1238c9bf99eaced7b39834de52f24a4dc866`
(`test(ddl): pin T06-A9 explicit audit-time-column role proof (#85)`).
This directory is the evidence-only commit on top of it — the evidence
HEAD is not the tested HEAD. **Zero production code changed in this
slice**; the baseline proved the shipped behavior already correct and the
commit adds only tests, the frozen manifest/profile, validator
extensions, and documentation.

## Scope

`ddl.table.audit_columns.require` assigns two policy roles from extracted
facts on `CREATE TABLE`:

- **created** — a time-like column with `DEFAULT <current-timestamp>`
  and no `ON UPDATE`;
- **updated** — a time-like column with `DEFAULT <current-timestamp>`
  *and* `ON UPDATE CURRENT_TIMESTAMP`.

Recognition is flag-based (`Column.DefaultIsCurrentTimestamp`,
`Column.OnUpdateCurrentTimestamp`), never name-based; an `ON UPDATE`
column does not double as created; `ON UPDATE` without the
current-timestamp default satisfies neither role. The parser normalizes
`CURRENT_TIMESTAMP`, `CURRENT_TIMESTAMP()`, `NOW()`, `LOCALTIME`, and
`LOCALTIMESTAMP` into one `FuncCallExpr` named `current_timestamp`
(fractional-second arguments preserved), which is why the `NOW()`
spelling passes.

ADR: `docs/decisions/2026-10-08-ddl-explicit-audit-time-column-proof.md`.

## Contents

- `artifact.json` — formal `make ddl-golden TASK=T06` artifact:
  **256/256 cases, 6163 assertions, head_sha=b9af1238c9bf**, cleanup
  rc=0. The T06 denominator moved 228 → 256: +12 offline CLI cases and
  +16 anchored metadata cases, all `t06-a9-*`.
- `cases/` — all 256 per-case records (sql/argv/stdout/stderr/rc, parsed
  JSON, structure/query/step records, version evidence). The A9 anchored
  records carry the raw `COLUMN_NAME:DATA_TYPE:ORDINAL:IS_NULLABLE:
  DATETIME_PRECISION:COLUMN_DEFAULT:EXTRA` observation rows alongside the
  semantic structure oracle (e.g. MySQL 8.4 stores
  `created_at:datetime:2:NO:0:CURRENT_TIMESTAMP:DEFAULT_GENERATED` and
  `updated_at:...:DEFAULT_GENERATED on update CURRENT_TIMESTAMP`).
- `policies/` — the 18 generated policy files the run used, including the
  new `t06-a9-audit-columns-isolated` profile (one rule, blocker,
  `required: true`, 380 rules disabled) and the all-rules-disabled
  control.
- `T06-manifest.json` — frozen manifest at b9af1238 (228→256 additive
  only; all prior case bodies byte-semantics identical).
- `anchors.json` — per-anchor identity (product/image/digest/container/
  version), artifact cli sha256, policy hashes, head_sha, cleanup
  verdict. Binary not stored; `build-identity.txt` carries
  `go version -m` and the recomputed sha256 (matches artifact).
- `baseline-3c8bac8b/` — the authorized offline A9 baseline collected at
  the previous evidence HEAD `3c8bac8b`: two probe policies (isolated
  blocker, all-off), the 12-run CLI matrix (6 roles × mysql/tidb) with
  argv/stdout/stderr/rc, the real parser/extractor AST probe (external
  module + `replace`, source + `probe-output.jsonl`), `catalog.json`,
  and build identity. This baseline measured that all six roles already
  behave correctly — including `NOW()` normalization — so no red phase
  exists.
- `gates/` — argv/stdout/stderr/rc captures at the tested SHA:
  `gofmt-check`, `focused-t06a9` (parser+rule+audit T06A9),
  `focused-entries` (SDK/HTTP T06A9), `make-test` (`go test ./...`,
  count=1), `pg-unit-test-gates`, `sql-corpus-gates`,
  `ddl-inventory-gate`, `ddl-coverage-catalog-test`,
  `docs-example-gates`, `decision-record-gate`,
  `ddl-golden-validator-test` (375 contract cases, 0 failures —
  including the 18 new A9 mutation defenses), `ddl-golden-T06`
  (256/256), `golden-validate` (binary check enabled), `task-diff`.

## Validator contract extension

The proof contract gained `finding_entries_full`: a complete
finding-entry multiset (statement index, statement kind, rule_id, level,
message, exact metadata map, location) compared with multiplicities
preserved. The 18 A9 mutations cover collapsed/duplicated/cross-swapped
double findings, findings smuggled to global, forged pass verdicts,
profile swaps, forged `EXTRA`/default/fsp structure output, skipped
driver replay after product reject, deleted structure queries,
cross-anchor donor reuse, offline-slot demotion, and denominator
shrinks. All are rejected; the synthetic control stays valid.

## Notes

- `vcs.modified=true` on both binaries reflects the repo's pre-existing
  untracked environment directories, not code changes; the tested-SHA
  golden binary hash matches the artifact (`cli.sha256` ==
  `build-identity.txt`).
- The anchored B/C roles prove a product `reject` and a driver CREATE
  success coexist: policy verdicts are team threshold results, never
  claims that the anchors reject the DDL. Both recorded.
- `DefaultValue` stays empty for function-call defaults by design; the
  rule consumes the typed flags, so no expression-text normalization was
  added — recorded as an observed fact, not a resolved limitation.
- No TIMESTAMP implicit defaults, fsp>0 policy, time-zone/runtime-value
  checks, ALTER-side governance, or DML execution are claimed; see the
  ADR's deferred-scope list.
- T02–T05 golden regression reuse was authorized for this slice; the
  shared Go, corpus, inventory, and validator suites above all ran at
  b9af1238.
- This slice also registered previously omitted L2 README entries
  (`coverage_t06a8_test.go` in audit; `audit_table_collation_t06a7`,
  `audit_comment_t06a8` in SDK/HTTP; `extractor_t06a7`,
  `extractor_t06a8` in the parser README) — documentation only.
