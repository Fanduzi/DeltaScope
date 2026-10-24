# T10-R1 verification archive — 3a92a02a

Bound SHA: `3a92a02a5d0c8bac7c26871b684d8f5aa838e3ab`
(`fix(ddl-golden): correct the post_verify phase expectation on the TiDB expr-unique pin`)

Product-code commits: `e70881d0` (member-level index facts and
primary-role identity), `64e750dc` (MySQL functional-unique role is
unprovable, not disqualified), `2e7b63fb` (TiDB literal-NULL member
classification). Contract commits: `52925171` (123-case freeze) +
`4c725708` (126-case freeze, expression-unique split pins) +
`3a92a02a` (post_verify phase correction).
Base: `8d75e6b2212f4696e06a9acb2c039414ec147a2b` (milestone base, T10
R0 already delivered; this round restores the deferred #89 scope)

Scope: T10-R1 scope restoration for issue #89 — member-level index facts
(`Index.KeyParts`: ordinal/kind/column/prefix-length/descending/expression
identity/multi-valued), the implicit primary-key role refusal shared
across ALTER/CREATE/ADD/CREATE-INDEX surfaces (3522), dialect-aware TiDB
PRIMARY-object resolution (PKIsHandle vs real index object), and the
member-legality feature family (prefix/JSON/FULLTEXT/SPATIAL/GLOBAL)
inside `ddl.index.advanced.compatibility.require` with tombstoned
invalid transitions.

## Layout

- `gates/` — offline gates at the bound SHA: `make test`,
  `pg-unit-test-gates`, validator, inventory, corpus, catalog,
  parser-error/unsupported, diagnostics evidence, docs examples, race,
  three-level-doc, focused T10-R1 tests, T10 contract sensitivity.
- `golden/` — `ddl_golden.py` artifacts per task at the bound SHA plus
  the consolidated run log (`regression-all.log`).
- `native-probe/r2-consolidated-pins.log` — live-product pins recorded
  2026-10 against MySQL 5.7.44/8.0.46/8.4.10 and TiDB v8.5.0 containers:
  implicit-PK promotion matrix (first/second candidate, nullable,
  declaration order, prefix-unique, expression member, create/add
  entries), TiDB PRIMARY-object truth table (NONCLUSTERED and VARCHAR
  clustered = real object; single-int clustered PKIsHandle = 1176),
  TiDB `TIDB_INDEXES.CLUSTERED`/`TABLES.TIDB_PK_TYPE` shape, and the
  member-legality error table (1170/3152/1283/1687/1252/1054/8200).
  The `mysql80|ncp_*` 1146 rows record that MySQL rejects the
  `NONCLUSTERED` attribute outright — the fixture is moot there, not a
  product-behavior claim.
- `identity/` — bound HEAD, CLI sha256, Go toolchain, working-tree
  status at capture time.

## Notes

- The golden binary identity lives inside each `artifact.json`
  (`cli.sha256`); `identity/golden-cli-sha256.txt` is the independently
  rebuilt same-tree binary.
- `worktree-status.txt` intentionally shows `.mcp.json` modified — an
  unrelated user-owned change preserved per delivery rules.
- Prior-round evidence under `5445a481…/` stays untouched; this round's
  scope restoration is archived separately.
