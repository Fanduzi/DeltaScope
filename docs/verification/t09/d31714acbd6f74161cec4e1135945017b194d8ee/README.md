# T09-R2 verification archive — d31714ac

Bound code SHA: `d31714acbd6f74161cec4e1135945017b194d8ee`
(`fix(ddl): report member-comparability gaps and honor provider string defaults on PK conversion`)
Contract/doc SHA: `b8215e600afd754f2d25b2de253a61e013bbb160`
(`docs(t09): amend contract for R2 member-comparability gap and PK default boundary`)
Base: `4c62b47aaf0470f6fd77f5e32b484c39e1a42b3a` (R1 evidence archive)

Scope: T09-R1 acceptance rework — two bounded defects, no T10:

- **R2-A** — a vouched existing index set containing a non-plain member
  that a lifecycle rule had to skip produced zero findings AND zero
  evidence gaps: complete coverage + pass verdict for a comparison that
  never completed. `lifecycleIndexGap` and `alterAddedIndexRule.EvidenceGaps`
  now emit one attributable `unknown_table_state` gap requiring
  `target_table.indexes.key_parts` (new minimal fact) when a non-plain
  existing member could have paired with a declared index under the rule's
  shape (same kind+signature for duplicate, strict left-prefix reach for
  left_prefix, equal columns for unique_overlap). Scoped only to
  affected pairings — unrelated non-plain members emit nothing.
- **R2-B** — `applyAddIndex`'s PK branch cleared a member default whenever
  `DefaultValue` displayed as `null` — erasing provider-loaded string
  defaults `'null'`/`'NULL'`/`'NuLl'`/`' null '`. Clearing now keys on
  `DefaultIsNull` alone (the AST-only SQL-NULL fact); provider `HasDefault`
  with a string `DefaultLiteral` survives, matching the native catalog on
  all four anchors.

## Layout

- `gates/red-before-fix.log` — real red run captured before the fix:
  24 failing subtests (rule-level gap cells + audit-level gap/coverage/
  verdict cells + provider string-default cells).
- `gates/r2-focused-tests.log` — the same tests green at the bound SHA.
- `gates/make-test.log` — `GOFLAGS=-count=1 make test` (rc=0).
- `gates/{pg-unit-test-gates,sql-corpus-gates,ddl-inventory-gate,
  ddl-coverage-catalog-test,docs-example-gates,decision-record-gate,
  ddl-golden-validator-test,three-level-doc}.log` — all rc=0.
- `gates/race.log` — `go test -race` on audit + ddl rules + mysql
  provider (rc=0).
- `gates/gofmt.log` — same two pre-existing unformatted T07 test files
  as the R1 archive (outside this diff); all R2 files clean.
- `golden-run.log` + `golden/<T>/` — six golden artifacts head-bound to
  d31714ac, each independently re-validated; run history records the one
  environment failure (leftover probe containers) honestly.
- `native-probe/` — R2 bounded cell: `t09r2_probe.py`,
  `native-matrix-r2.json`, `native-matrix-r2.txt`, `probe-run.log`.
- `identity/` — work-CLI + golden-CLI identities (the `+dirty` module
  marker reflects only the preserved external `.mcp.json` modification).

## Prior archives

`docs/verification/t09/51c8eec3.../` (original T09) and
`docs/verification/t09/a986eaba.../` (R1) keep their own SHA bindings.
