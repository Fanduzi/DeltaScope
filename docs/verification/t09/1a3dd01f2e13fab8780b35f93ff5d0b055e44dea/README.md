# T09-R3 verification archive — 1a3dd01f

Bound code SHA: `1a3dd01f2e13fab8780b35f93ff5d0b055e44dea`
(`fix(ddl): treat expression-bearing index Columns as a truncated projection in left-prefix pairing`)
Contract/doc SHA: `8e68b44c4eddb4cb654486a42c144dccf81dc270`
(`docs(t09): amend contract for R3 truncated-projection pairing and discriminated default evidence`)
Base: `b074aee0187938363789569e4a07f2483746d83a` (R2 evidence archive)

Scope: T09-R2 acceptance rework — one residual code defect plus one
evidence strengthening, no T10:

- **R3-A** — an expression-bearing existing member's `Columns` is a
  truncated projection (the provider marks expression parts via
  `HasExpressionKeys`/`ExpressionCount` without entering them into
  `Columns`). `leftPrefixPairing`'s strict length check could therefore
  disprove a pairing the real key sequence still admits (`(a, <expr>)`
  vs declared `(a)`) — zero findings, zero gaps, complete coverage, pass.
  The pairing now keys on projection completeness: expression-bearing
  members pair on a non-strict named-prefix (`hasKeyPartPrefix`),
  complete projections keep the strict proof. Provably unrelated members
  still emit no gap.
- **R3-B** — evidence only: the R2 native matrix could not separate the
  string `'NULL'` from SQL `NULL` through `--raw` display. A bounded
  follow-up on the same four anchors records `COLUMN_DEFAULT IS NULL`,
  `HEX(COLUMN_DEFAULT)`, and `SHOW CREATE TABLE`. The R2-B product fix
  (`DefaultIsNull` only) is unchanged.

## Layout

- `gates/red-before-fix.log` — real red run captured before the fix:
  7 leaf-level failing subtests (3 rule-level pairing cells across both
  lifecycle entries, 4 audit-level entry×dialect cells) plus parent
  `FAIL` lines; every scope control already passed pre-fix.
- `gates/r3-focused-tests.log` — the same tests green at the bound SHA
  (2 packages `ok`, 0 FAIL lines).
- `gates/make-test-postcommit.log` — `make test` at the bound SHA:
  39 `ok`, 5 `no test files`, 0 `FAIL` (rc=0).
- `gates/{pg-unit-gates,sql-corpus-gates,ddl-inventory-gate,
  ddl-coverage-catalog-test,docs-example-gates,decision-record-gate,
  ddl-golden-validator-test,t09-contract,three-level-doc}.log` — rc=0.
- `gates/race.log` — `go test -race` on audit + ddl rules + metadata
  providers (rc=0).
- `gates/gofmt.log` — the same two pre-existing unformatted T07 test
  files as the R1/R2 archives (committed in d2bbbfa6, outside this
  diff); all R3 files clean.
- `golden-run.log` + `golden/<T>/` — six golden artifacts head-bound to
  1a3dd01f, each independently re-validated; run history records the
  T07/T08 credential-env miss and clean rerun honestly.
- `native-probe/` — R3 bounded cell: `t09r3_probe.py`,
  `native-matrix-r3.json`, `native-matrix-r3.txt`, `probe-run.log`.
- `identity/` — work-CLI + golden-CLI identities (the `+dirty` module
  marker reflects only the preserved external `.mcp.json` modification).

## Prior archives

`docs/verification/t09/51c8eec3.../` (original T09),
`a986eaba.../` (R1), and `d31714ac.../` (R2) keep their own SHA bindings;
R2's archive is untouched by this round.
