# T06-A3-R1 verification evidence — per-option DefaultIsNull recompute

Code commit under test: `d3ddf9c13e5c216a455c86d21c30293001123bb1`
(`fix(parser): recompute DefaultIsNull per DEFAULT option`, #85/T06-A3-R1).
Parent: `577a253b740acd45ebbf1113dc1f2e5f809d8276` (T06-A3 evidence).
Evidence commit: this file's own commit (separate from the code commit; the
artifact's `head_sha` is `d3ddf9c1`).

## Contents

- `artifact/` — final four-anchor run output: `artifact.json` (72 cases,
  1524 assertions, `head_sha=d3ddf9c1`), `cases/` (72 raw case records with
  argv, audit stdout/stderr, driver-side execute/structure/verify steps,
  container + image-digest + version evidence, teardown), and the six
  generated isolated policies (`golden-policy-*.yaml`, including the
  `t06-a3-null-drop-state-isolated` four-rule profile). The T06 case set is
  unchanged from A3 — same 72 cases, same profiles, same oracles
  (`ORDER BY BINARY INDEX_NAME` intact).
- `red/` — genuine pre-fix failures: the R1 tests were added to the
  pre-fix production code (A3 code, `extractor.go` still setting
  `DefaultIsNull` only on the NULL arm) and run before the one-line fix.
  `parser-red.txt` shows the field-level contradiction — for
  `DEFAULT NULL DEFAULT 'NULL'` (and `'<nil>'`/`0`/`''`) the extractor
  returned `{HasDefault:true DefaultValue:"'NULL'" DefaultIsNull:true}` on
  both mysql and tidb dialects: a stale `DefaultIsNull=true` left over from
  the earlier NULL option while `DefaultValue` already held the final
  non-NULL literal. `audit-red.txt` shows the real `AuditSQL` consequence:
  the three-statement `CREATE → DROP COLUMN → CREATE INDEX` batch with a
  `DEFAULT NULL DEFAULT 'NULL'` sibling reported `aggregate =
  pass/complete` instead of `review/unverified` — the stale flag let the
  drop-template take the precise path and suppressed the expected
  `unknown_table_state` gap on the index statement. The
  `NullEndingDuplicateStaysComplete` control passed even pre-fix,
  confirming only the flag-clearing path was broken.
- `gates/` — per-gate `argv`/`stdout`/`stderr`/`rc` triples, all rc=0 on
  `d3ddf9c1` (see table below).

## Root cause and fix (precise diff)

The A3 `ColumnOptionDefaultValue` branch assigned
`column.DefaultIsNull = exprIsNullLiteral(option.Expr)` only in the NULL
arm — actually it set `DefaultIsNull=true` when the current expression was
SQL NULL but never reassigned it for later non-NULL `DEFAULT` options.
Since the TiDB AST keeps repeated `DEFAULT` options in order and the
extractor applies last-writer-wins for `DefaultValue`, a repeated
`DEFAULT NULL DEFAULT 'NULL'` left `DefaultIsNull` describing the earlier
expression while `DefaultValue` described the later one. R1 recomputes the
flag on every option:

```go
column.HasDefault = true
column.DefaultIsNull = exprIsNullLiteral(option.Expr)
if column.DefaultIsNull {
	column.DefaultValue = "NULL"
} else {
	column.DefaultValue = normalizedExprText(option.Expr)
}
column.DefaultIsCurrentTimestamp = exprIsCurrentTimestamp(option.Expr)
```

`exprIsNullLiteral` (param-marker exclusion, typed `ast.ValueExpr`
nil-datum check), `normalizedExprText`, `exprIsCurrentTimestamp`, COMMENT
extraction, the provider, all rules, the A6 conservative drop boundary,
public `spec`, and transports are unchanged. Production diff:
`internal/infrastructure/parser/tidb/extractor.go` only (+3/-1).

## Proven downstream effect

`dropOtherColumnMayReference` consumes `DefaultIsNull`. With the flag
recomputed per option, `keep_c VARCHAR(8) DEFAULT NULL DEFAULT 'NULL'`
extracts as `{HasDefault:true DefaultValue:"'NULL'" DefaultIsNull:false}`
and the batch keeps the A6 conservative boundary: statement 3 emits zero
findings and exactly one `ddl.create_index.columns.exists.require` gap
with `reason_code=unknown_table_state` and
`required_facts=[target_table.columns,target_table.existence]`; aggregate
is `review`/`unverified`; the provider snapshot is not re-read. The
`DEFAULT NULL DEFAULT ... DEFAULT NULL` ending keeps the precise path
(`complete`/`pass`, typed NULL preserved through the post-state).

## Gates (all rc=0, captured in `gates/`)

`focused-t06a3r1` (2 pkgs `-run T06A3R1 -v`), `focused-t06a3-regression`
(4 pkgs `-run 'T06A1|T06A2|T06A3' -v` incl. SDK/HTTP representative
regressions), `ddl-golden-validator-test` (272 contract cases, 0
failures), `golden-run` (four anchors, 72/72 cases, 1524 assertions),
`validate` (`artifact valid`), `make-test` (`GOFLAGS=-count=1`, 37 pkgs),
`pg-unit-test-gates`, `sql-corpus-gates`, `ddl-inventory-gate`,
`ddl-coverage-catalog-test`, `docs-example-gates`,
`decision-record-gate`, `gofmt` (no files listed), `task-diff-stat`,
`source-identity`.

## Boundaries honestly recorded

- The task does not claim native duplicate-`DEFAULT` semantics were
  re-verified against databases; the golden run re-executes the frozen
  72-case contract, which contains no repeated-`DEFAULT` case. The R1
  guarantee is narrower and exactly what was asked: for options the parser
  already accepts, `DefaultValue` and `DefaultIsNull` now describe the
  same (final) expression.
- No rule judges repeated-`DEFAULT` legality; none was added. The A6
  conservative boundary for non-NULL literal defaults is preserved, not
  relaxed.
- `bin/` build output, credentials, and container internals are not
  committed; `connect.password_env` records the env name only.
- Temporary originals (`/tmp/ds-t06-a3r1`, `/tmp/ds-t06-a3r1-golden.OgaFzC`)
  exist only in the author's environment; this directory is the committed
  copy.
