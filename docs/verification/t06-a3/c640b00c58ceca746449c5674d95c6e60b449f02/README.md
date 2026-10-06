# T06-A3 verification evidence — typed SQL DEFAULT NULL fidelity

Code commit under test: `c640b00c58ceca746449c5674d95c6e60b449f02`
(`fix(parser): recognize SQL DEFAULT NULL as a typed literal`, #85/T06-A3).
Parent: `d2cba4dd120b3482a8251d3e38f60939685cb6cc` (T06-A2 evidence).
Evidence commit: this file's own commit (separate from the code commit; the
artifact's `head_sha` is `c640b00c`).

## Contents

- `artifact/` — final four-anchor run output: `artifact.json` (72 cases,
  1524 assertions, `head_sha=c640b00c`), `cases/` (72 raw case records with
  argv, audit stdout/stderr, driver-side execute/structure/verify steps,
  container + image-digest + version evidence, teardown), and the six
  generated isolated policies (`golden-policy-*.yaml`, including the new
  `t06-a3-null-drop-state-isolated` four-rule profile).
- `plan-baseline/` — **old baseline from the T06-A3-PLAN phase** (built at
  `d2cba4dd`, pre-fix): 8 isolated-policy CLI runs for the four
  default spellings × mysql/tidb (`{a,b,c,d}-{mysql,tidb}.{argv,sql,stdout.json,stderr,rc}`),
  `catalog.json`, `policy-*.yaml`, `build-head.txt`. Kept as comparison
  originals only; they predate the fix and are not re-run evidence.
- `red/` — genuine pre-fix failures on the task base (detached worktree at
  `d2cba4dd` + the new tests only): `parser-red.txt` shows
  `sql_null = {HasDefault:true DefaultValue:"<nil>" DefaultIsNull:false}`
  while `'NULL'`/`'<nil>'` spellings pass; `audit-red.txt` shows the
  three-statement path degrading to `review/unverified` with the
  `unknown_table_state` gap on the index statement.
  `golden-collation-fork.txt` is the first full run's honest failure: the
  initial `ORDER BY INDEX_NAME` oracle forked by anchor collation
  (`idx_keep` first on MySQL's case-insensitive information_schema, `PRIMARY`
  first on TiDB's binary ordering) — fixed by `ORDER BY BINARY INDEX_NAME`.
- `gates/` — per-gate `argv`/`stdout`/`stderr`/`rc` triples, all rc=0 on
  `c640b00c` (see table below).

## Root cause and fix (precise diff)

`normalizedExprText` rendered the default expression's typed value via
`fmt.Sprint`; the pinned TiDB parser driver's NULL datum returns Go `nil`
from `GetValue()`, so `DEFAULT NULL` recorded `DefaultValue="<nil>"` and the
text-derived `DefaultIsNull` could never be true. The
`ColumnOptionDefaultValue` site now calls the private `exprIsNullLiteral`
(typed `ast.ValueExpr` with nil `GetValue()`, `ast.ParamMarkerExpr`
excluded first, non-literals never evaluated): SQL NULL records
`DefaultValue="NULL"` + `DefaultIsNull=true`; every other spelling keeps
`normalizedExprText` verbatim (`'NULL'`, `'<nil>'`, `0`, `''`); an absent
clause keeps `HasDefault=false`. `normalizedExprText`,
`exprIsCurrentTimestamp`, COMMENT extraction, the provider, all rules, and
the A6 conservative drop boundary are unchanged; ALTER column `Definition`
inherits the facts via shared `extractColumn`. Production diff:
`internal/infrastructure/parser/tidb/extractor.go` only (+29/-2).

## Proven downstream effect

`dropOtherColumnMayReference` reads `DefaultIsNull` — with the typed fact a
`DEFAULT NULL` sibling no longer forces `dropColumnLocalUnsafe`, so the
frozen `CREATE t(id PK, obsolete, keep_c VARCHAR(8) DEFAULT NULL)` →
`DROP COLUMN obsolete` → `CREATE INDEX idx_keep ON t(keep_c)` batch stays
complete/pass on all four anchors with mid-flight structure oracles
(column order, `keep_c` length/NULL flag, `PRIMARY(id)`, final
`PRIMARY:id:1,idx_keep:keep_c:1`). String defaults `'NULL'`/`'<nil>'` keep
the A6 `unknown_table_state` gap — verified as a negative control in Go
tests and left unpinned in the golden manifest by design (the conservative
boundary is not relaxed).

## Gates (all rc=0, captured in `gates/`)

`focused-t06a3` (4 pkgs `-run T06A3 -v`), `ddl-golden-validator-test`
(272 contract cases, 0 failures, incl. 15 T06-A3 mutations), `golden-run`
(four anchors, 72/72 cases, 1524 assertions), `validate`
(`artifact valid`), `make-test` (`GOFLAGS=-count=1`),
`pg-unit-test-gates`, `sql-corpus-gates`, `ddl-inventory-gate`,
`ddl-coverage-catalog-test`, `docs-example-gates`, `decision-record-gate`,
`gofmt` (no files listed), `task-diff-stat`, `source-identity`.

## Boundaries honestly recorded

- Product `pass`/`reject` are policy verdicts; native storage reality is
  proven only by the driver-side `execute`/`structure` records inside
  `cases/` (e.g. `COLUMN_DEFAULT IS NULL` flags and `HEX` bytes
  `4E554C4C`/`3C6E696C3E` distinguish `b`/`c`/`d`).
- `COLUMN_DEFAULT` SQL NULL conflates "no clause" with explicit
  `DEFAULT NULL`; the provider is unchanged and no declaration-source
  equivalence is claimed — the metadata default-value semantics remain a
  registered deferred item in `T06-base-traceability.md`.
- `bin/` build output, credentials, and container internals are not
  committed; `connect.password_env` records the env name only.
- Temporary original directories (`/tmp/ds-t06-a3plan.1791293015`,
  `/tmp/ds-t06-a3.mgYSm4`, `/tmp/ds-t06-a3-red`) exist only in the author's
  environment; this directory is the committed copy.
