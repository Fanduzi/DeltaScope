# T10 verification archive — 5445a481

Bound code SHA: `5445a4815c360fed0be6850c85deff281d243d59`
(`test(spec): T10 advanced-index assessment decision table`)
Contract/oracle SHA: `94deda7f8698a1d89cd562710618bd36864068da`
(`test(ddl-golden): T10 frozen advanced-index contract oracle`)
Oracle amendment SHA: `5c47f434`
(`fix(ddl-golden): retire the option.(in)visible unaudited aspects in T04/T05 oracles`)
Base: `fbd411308e28332b5e8408b8074dd1c1fa501576` (T09-R3 accepted)

Scope: T10 advanced indexes and visibility (issue #89) —
`ALTER INDEX VISIBLE|INVISIBLE` as an audited ordered-state action,
declared index visibility modeled on every index surface, expression /
prefix / descending / multi-valued member marks, SPATIAL+FULLTEXT
provider kind fidelity, `STATISTICS.IS_VISIBLE` provider capability
probing, three new rules
(`ddl.index.invisible.forbid` default-warning,
`ddl.alter.alter_index.exists.require` blocker,
`ddl.index.advanced.compatibility.require` blocker), and coverage that
audits `alter_index` while bounding `multi_valued` member/query
semantics as unsupported evidence.

## Layout

- `gates/t10-focused-tests.log` — audit-level T10 suite at the bound
  SHA: first path, direction silence, missing index, unknown set, batch
  chain, multi-action tombstone, provider-invisible-not-intent,
  post-state member, declared invisible paths, declared PK invisibility,
  ALTER INDEX PRIMARY on both dialects, 5.7 version gate, missing
  version, per-form gates, TiDB descending, feature isolation, and the
  rules-off control (rc=0).
- `gates/t10-focused-tests-other-pkgs.log` — extractor, provider
  classification/probing (incl. SPATIAL), and catalog tests (rc=0).
- `gates/make-test-postcommit.log` — `make test` at the bound SHA:
  39 `ok`, 5 `no test files`, 0 `FAIL` (rc=0).
- `gates/make-test-run2.log` — the same gate at the pre-test-commit SHA
  `94deda7f` (rc=0); kept because the evidence-collection pass added one
  test file afterward.
- `gates/race-run2.log` — `go test -race` on audit, spec, ddl rules,
  mysql provider, and tidb parser (rc=0).
- `gates/{sql-corpus-gates,ddl-coverage-catalog-test,ddl-inventory-gate,
  ddl-parser-error-feasibility-report,parser-error-unsupported-contract-
  test,unsupported-diagnostics-evidence-test,ddl-census-report,
  docs-example-gates,adr-run2,t10-contract,three-level-doc,
  ddl-golden-validator-test}.log` — rc=0 each.
- `gates/gofmt.log` — the same two pre-existing unformatted T07 test
  files as the T09 archives (committed in d2bbbfa6, outside this diff);
  all T10 files clean.
- `gates/t10-contract.log` — `scripts/test_t10_contract.py` adversarial
  drift suite: 33/33 constructed-control checks passed.
- `golden/T10/artifact.json` + `golden/T10-run.log` — 81/81 cases,
  1,910 assertions at head=5445a481; includes the materialized
  `t10-<anchor>-unknown-set` tombstone-chain cases (1 existence+indexes
  gap pair, zero findings under the isolated exists policy, on
  mysql80/mysql84/tidb85).
- `golden/{T04..T09}/artifact.json` + `golden/*-run.log` +
  `golden/regression-all.log` — regression goldens at head=5445a481:
  T04 163/163 (1,884), T05 247/247 (5,104), T06 496/496 (11,772),
  T07 207/207 (6,190), T08 172/172 (4,240), T09 125/125 (3,442).
  The T04/T05 first pass failed on the retired unaudited aspects the
  T10 ledger already records; the amended oracles rerun clean in the
  same directory (assertion counts drop by the retired-entry checks).
- `native-probe/round1-ambiguous-fixtures.log` — recorded honestly: a
  PK-less table makes MySQL answer `ALTER INDEX PRIMARY` with 1176
  (absence, not refusal), and a single-column table makes declared PK
  INVISIBLE answer 4028 (visible-column rule fires first); 5.7 also
  proves the DESC collation cell is NULL (not 'A').
- `native-probe/round2-primary-visibility-pins.log` — corrected
  fixtures: MySQL 8.0/8.4 `ALTER INDEX PRIMARY INVISIBLE` → 3522 and
  declared `PRIMARY KEY(id) INVISIBLE` → 3522 while `... VISIBLE` is
  accepted; TiDB `ALTER INDEX PRIMARY` → 1176 (PRIMARY is not an index)
  yet declared PK INVISIBLE → 3522; TiDB expression CONCAT → 8200
  (function whitelist); multi-valued catalogs as BTREE on both
  products; TiDB has no GEOMETRY (1064).
- `identity/` — golden artifact binary: `5445a4815c36` embedded
  version, sha256 `eb03e6ad…19ef`, go1.26.1 darwin/arm64.

## Notes

- Golden `+dirty` in the embedded version comes from the unrelated
  user-owned `.mcp.json` working-tree change; every T10 source file is
  committed.
- The unknown-set role in `roles.json` is materialized in the frozen
  manifest (`t10-<anchor>-unknown-set`, 81-case denominator, ID SHA
  `4ec9ba71…d8a`): a multi-action `ADD COLUMN + ALTER INDEX` statement
  exceeds the bounded successor template and tombstones the entry, so
  the following `ALTER INDEX` reads the `existence`+`indexes` gap pair
  instead of a fabricated member.
- Evidence for the two-axis independent review (Standards + Spec since
  `fbd41130`) lives in the review findings handled during this round;
  every blocking item was closed (default warning alignment, missing-
  index finding independence prose, TiDB-desc unconditional, PRIMARY
  VISIBLE silence, unvouched-PRIMARY gap, `VisibilityExplicit` post-
  state clearing, SPATIAL unit assertion, unknown-set materialization).
