# T10-R2 verification archive — d4d6d5a8

Bound SHA: `d4d6d5a8e5e35b3a21efc39b6d535cabd1255afe`
(`style(ddl): drop dead params and lint nits from T10 member work` —
behavior-neutral cleanup on top of the R2 product/contract commits)

Product-code commits: `5a918ac2` (four-state primary role, kind-aware
member legality, coverage graduation, provider literal-NULL triage,
TiDB handle tri-state, unresolved publication gate), `d4d6d5a8`
(style-only: dead params, tagged switch, De Morgan, unconvert).
Contract/oracle commits: `8cba6c1c` (126-case oracle, CONTRACT/ADR/L2
sync), `1135e3f5` (T03–T05 shared-corpus expects graduated for
expr/prefix/desc/TiDB GLOBAL).
Base: `8d75e6b2212f4696e06a9acb2c039414ec147a2b` (milestone base, T10
R0 delivered; this round finishes the deferred #89 re-audit scope)

Scope: T10-R2 acceptance restoration for issue #89 — a four-state
`PrimaryRoleResolution` (explicit / holder / no-candidate / unresolved)
with member-pointer holder identity, `UnresolvedTransition` tombstoning
of unprovable transitions in ordered prospective state, kind-aware
member legality (FULLTEXT/SPATIAL/prefix/JSON/expression/TiDB GLOBAL
matrices), the TiDB `PrimaryKeyIsHandle` + `PrimaryKeyHandleUnknown`
tri-state driven by observed `tidb_enable_clustered_index`, provider
literal-`NULL` member triage (SQL NULL / expression text / real ``NULL``
column / unmodeled), multi-valued cast facts, and graduation of
expression, prefix, descending, multi-valued, and TiDB GLOBAL member
forms out of blanket-unsupported coverage.

## Layout

- `gates/` — offline gates at the bound SHA: `make test`,
  `pg-unit-test-gates`, `pg-confidence-gates` (unit + e2e CLI/HTTP/MCP),
  `sql-corpus-gates`, `query-access-corpus-gates`, `ddl-inventory-gate`,
  corpus/catalog/parser-error/unsupported-diagnostics/docs-examples,
  `decision-record-gate`, `release-dialect-hygiene-gates`, `go vet`,
  race, three-level-doc, T10 contract sensitivity (33 controls),
  golden validator suite, focused T10-R2 tests, `release-gofmt-gate`.
- `golden/` — `ddl_golden.py` artifacts per task at the bound SHA plus
  the consolidated run log (`regression-all.log`).
- `native-probe/r2-consolidated-pins.log` — live-product pins from the
  R1 archive carried byte-identical (git blob
  `c83362698c9b91664f5f7e6cd2a90890e38f4f72`) against MySQL/TiDB
  anchors. **Corrected in T10-R3**: the file actually contains the
  implicit-PK promotion matrix (first/second candidate, nullable,
  declaration order, prefix-unique, expression member, create/add
  entries), the expression-unique promotion dialect split (MySQL 3522
  vs TiDB allow), the TiDB PRIMARY-object truth table (NONCLUSTERED and
  non-integer clustered = real object; single-int clustered PKIsHandle
  = 1176 on `ALTER INDEX PRIMARY`), `TIDB_INDEXES.CLUSTERED`/
  `TIDB_PK_TYPE` catalog shapes, the member-legality error table
  (1170/3152/1283/1687/1252/1054/8200), and TiDB
  descending-accepted-stored-ascending (`desc_dir`/`desc_collation`
  rows). The FT(5)/FT(0), multi-valued DECIMAL/DOUBLE dialect,
  clustered default-mode, and explicit-PK-then-UNIQUE probes listed
  here before were not preserved in this file and no probe scripts were
  archived — bounded re-verification of exactly those matrices is
  recorded in the T10-R3 archive.
- `identity/` — bound HEAD, CLI sha256, Go toolchain, working-tree
  status at capture time.

## Golden results (T03–T10, 1654 cases, all pass)

| task | cases | passed | head bound |
|------|-------|--------|------------|
| T03  | 118   | 118    | 1135e3f5   |
| T04  | 163   | 163    | 1135e3f5   |
| T05  | 247   | 247    | 1135e3f5   |
| T06  | 496   | 496    | d4d6d5a8   |
| T07  | 207   | 207    | d4d6d5a8   |
| T08  | 172   | 172    | d4d6d5a8   |
| T09  | 125   | 125    | d4d6d5a8   |
| T10  | 126   | 126    | d4d6d5a8   |

## Notes

- The golden binary identity lives inside each `artifact.json`
  (`cli.sha256`); `identity/golden-cli-sha256.txt` is the independently
  rebuilt same-tree binary at `d4d6d5a8`. T03–T05 artifacts record head
  `1135e3f5` because they ran before the style-only commit; `git diff
  1135e3f5..d4d6d5a8` touches only dead-parameter removal and lint nits
  in `advanced_indexes.go`/`extractor.go` — behavior-identical, so the
  archive binds the union under the final SHA.
- `worktree-status.txt` intentionally shows `.mcp.json` modified — an
  unrelated user-owned change preserved per delivery rules.
- The T07–T10 manifests require `T07/T08/T09/T10_MYSQL_PASSWORD`
  exported (fixture-credential contract: the password travels through
  `MYSQL_PWD` env, never argv or recorded commands). The r5 run without
  them is kept out of this archive; `regression-all.log` preserves the
  r4 (T03–T06) and r6/r7 (T07–T10) passes.
- During this round the Docker VM exhausted disk: `docker rm -f`
  between tasks leaked 121 dangling anonymous volumes (~17.5 GB), which
  surfaced as `InnoDB: No space left on device`, `tidb85 exited (0)` on
  compose up, and MySQL-anchor `database not reachable` cascades. The
  failure was environmental, not product; removal of the dangling
  volumes restored clean runs. Named project volumes were untouched.
- `release-gofmt-gate` reports five violations, all baseline files
  untouched by this round (`git blame` pre-milestone commits):
  `batch_state_t07_{matrix,}_test.go` and three
  `docs/verification/t06-*/probe/main.go` scratch fixtures.
- Prior-round evidence under `5445a481…/` and `3a92a02a…/` stays
  untouched; this round's scope restoration is archived separately.
