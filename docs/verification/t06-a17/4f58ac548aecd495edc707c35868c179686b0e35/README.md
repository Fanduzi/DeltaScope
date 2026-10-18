# T06-A17: declared column type-parameter fidelity

Code under test: `4f58ac548aecd495edc707c35868c179686b0e35`
(`fix(ddl): T06-A17 declared column type-parameter fidelity`)

Buggy base (red-state): `a08190969237404b280cfcc8d2cec8c01a082316`
Evidence parent: same `a0819096` (code commit is its direct child).
Frozen T06 denominator: **496 cases — unchanged** (no manifest/profile/case
edits; validator remains 728 checks, 0 failures).

## The two root causes (one source-fact slice, two loss layers)

- **Extractor** (`internal/infrastructure/parser/tidb/extractor.go`):
  `strings.ToLower(col.Tp.String())` lowercased ENUM/SET member literal
  bytes (merging `enum('Ready','Closed')` with `enum('ready','closed')`),
  while the formatted layer substituted display defaults for unspecified
  sentinels (`DECIMAL` → `decimal(10,0)`, `DECIMAL(12)` → `decimal(12,0)`)
  and elided explicit `(M,0)` scale on FLOAT/DOUBLE.
- **Provider** (`internal/infrastructure/metadata/mysql/provider.go`):
  `parseColumnType` truncated `COLUMN_TYPE` at the first `(` and parsed
  only the first integer — all scale, all second parameters, and all
  member literals were dropped, and an ` unsigned` substring inside a
  member literal could set `Unsigned` falsely.

## The fix (no new fields, no new rules)

`spec.Column.Type` stays the single representation. The five target
types (DECIMAL, FLOAT, DOUBLE, ENUM, SET) get faithful text at each
source; every other type keeps its established rendering.

- Extractor: `declaredColumnType` routes the five AST bases through
  `FieldType.Restore` (lowercase keyword + single-quote + backslash
  escape flags). Sentinel-faithful: `decimal` / `decimal(12)` /
  `decimal(12,0)` stay three distinct strings; `float(10,0)` survives;
  members are re-quoted byte-exact. All other types keep
  `ToLower(String())` (`int(11)`, `bigint(20) unsigned`).
- Provider: `parseColumnType` keeps the verbatim parenthesized span for
  the five bases via `typeParameterSpan` (quote-aware: member commas,
  `)`, `''` never end the span) and detects `unsigned` on quote-stripped
  text. `Length` keeps its first-integer meaning; non-target bases keep
  the bare-keyword contract (`varchar(8)` → `varchar` + length 8).
- Declaration vs catalog stay distinct: declared `DECIMAL(12)` yields
  `decimal(12)`; a catalog `decimal(12,0)` row yields `decimal(12,0)` —
  catalog normalization is never laundered into declaration provenance.
- `FLOAT(p)`: the locked parser itself normalizes `FLOAT(10)`→`float`,
  `FLOAT(30)`→`double`, discarding `p`; the extractor reports that AST
  fact and never rebuilds the parameter.

## What is proven

- **Red state on the buggy base** (`red-green/red-run.log`): the final
  five A17 test files dropped onto an unmodified `a0819096` worktree
  compile cleanly and produce 18 `--- FAIL` subtests — extraction folds
  member case / substitutes defaults / elides explicit zero, provider
  truncates to base keyword, derived pre-state and `source_type` echo
  carry the losses. `TestT06A17TypeFactsSDKTransport` passes on both
  sides by design (transport regression, not a fidelity assertion).
- **Green state** (`red-green/green-run.log`): the same 14 tests at
  `4f58ac54` — all pass, both dialects.
- **Declaration matrix** (`declaration-matrix.md`): every frozen N1/N2/
  L1/L2/C spelling → `Column.Type`/`Length`/`Unsigned`, including the
  `FLOAT(p)` normalization controls and the `a\\b` backslash member.
- **Controlled-driver provider matrix**: `provider_t06a17_test.go` feeds
  constructed `COLUMN_TYPE` rows through the real
  `Provider.LoadTableSnapshot` — verbatim spans, `Length` intact,
  member-internal ` unsigned`/`zerofill` never set `Unsigned`.
- **Ordered pre-state**: `batch_state_t06a17_test.go` proves a
  CREATE-declared `decimal(12,2)`/`float(10,0)`/`enum('Ready','Closed')`/
  `set('Write','Read')` survives `batchState` into the follower ALTER's
  pre-state under both dialects; `audit_t06a17_test.go` proves a real
  `mysqlmeta.Provider` source carries verbatim text into the
  modify-compatibility finding's `source_type` echo (one audit per
  column — a cross-family MODIFY tombstones the next pre-state by
  design).
- **Four-anchor catalog→provider proof** (`anchor-catalog-provider.md`,
  `anchor-probe/`): one disposable `golden.t06_a17_type_facts` (24
  columns = N1+N2+L1+L2+C union) per anchor — MySQL 5.7.44, 8.0.46,
  8.4.10, TiDB 8.5.0 pinned images. Per anchor: pre COUNT=0 → CREATE →
  ordered `information_schema.COLUMNS` incl. `HEX(COLUMN_TYPE)`,
  `NUMERIC_PRECISION/SCALE`, charset/collation → real
  `Provider.LoadTableSnapshot` through a query-logging driver wrapper
  (per-query ledger) → DROP → post COUNT=0. All 21 target columns match
  catalog byte-for-byte on all four anchors; server normalization
  observed and kept (TiDB `float(10,0)`→`float`, all-anchor
  `DECIMAL`→`decimal(10,0)`); non-target columns keep the bare-keyword
  contract. Probe source: `anchor-probe/main.go.txt` (temporary
  in-module package, removed before commit; credentials via env only).
- **Golden**: `make ddl-golden TASK=T06` → **496/496 passed, 11772
  assertions** (`artifact.json`, `cases/`); `validate --artifact` →
  `artifact valid`. No frozen expectation in the 496 cases embeds
  five-type parameter text, so the upgrade touches nothing in the
  manifest — verified by `EXPECTED-UPGRADES.md` and the validator's
  own identity checks.
- **Gates** (`gates/`): `make test` 37 packages ok; pg-unit-test-gates,
  sql-corpus-gates, ddl-inventory-gate, ddl-coverage-catalog-test,
  docs-example-gates, decision-record-gate all rc=0;
  `ddl-golden-validator-test` → `contract cases=728 failures=0`.
- **EXPECTED-UPGRADES.md**: the complete upgrade surface — two
  production sites, one same-root test expectation
  (`provider_test.go` `decimal(10,2)`), and the finding-text echoes
  (`source_type`/`target_type`) that now carry faithful text. Rule
  applicability, finding counts, levels, coverage, and gaps are
  unchanged.

## Execution provenance

- Real parser/AuditSQL: extractor tests (`t06a2Extract` real parse→extract
  under both dialects), batch-state test (real `Parse`/`Extract`/
  `batchState`), auditmeta test (real `AuditSQL` + real provider over
  constructed driver rows), SDK test (public `Audit`).
- Constructed catalog rows: `provider_t06a17_test.go` and
  `audit_t06a17_test.go` mark their `information_schema` rows as test
  fixtures — not live anchors.
- Actual four-database runs: `anchor-probe/` — the only live-server
  evidence; everything recorded including catalog-side normalization.
- Reused: the T06-A17 baseline originals under `baseline-a0819096/`
  (archive-built probe at the old HEAD; the 25 MB probe binary is not
  committed — build identity and hash are in its README/build files).
- Not executed: FLOAT(M,D) version matrix beyond the four fixed forms,
  sql_mode matrix, live-server ALTERs on the probe table, PG provider.

## Commands

- `go test -count=1 ./internal/infrastructure/parser/tidb ./internal/infrastructure/metadata/mysql ./internal/application/audit -run T06A17 -v` (plus auditmeta + pkg/deltascope `-run T06A17`)
- `GOFLAGS=-count=1 make test`
- `make pg-unit-test-gates sql-corpus-gates ddl-inventory-gate ddl-coverage-catalog-test docs-example-gates decision-record-gate ddl-golden-validator-test`
- `make ddl-golden TASK=T06 ARTIFACT_DIR=/tmp/ds-a17-proof/golden`
- `python3 scripts/ddl_golden.py validate --artifact /tmp/ds-a17-proof/golden/T06/artifact.json`
- Four-anchor probe: `/tmp/ds-a17-proof/a17probe <out> <anchor> <port> <dialect>` per anchor (rc=0 ×4; logs under `anchor-probe/`)

One recorded-and-kept failure: the first `make ddl-golden` attempt hit a
container-name conflict because this session had pre-started the four
anchors for the §7 proof (`gates/ddl-golden-T06-failed-run-1.log`); the
containers were removed and the rerun passed cleanly — no runner,
manifest, or artifact-path changes.

## Working tree honesty

`.mcp.json` carries an unrelated external modification and was never
staged. Untracked tool/recovery directories and the stray
`docs/verification/t06-a12/…/T06/` leftover belong to other
tooling/tasks and are untouched. The temporary `a17probe/` package was
deleted before the code commit (source preserved in evidence). Committed
tree is clean apart from this task's files; the whole working tree is
not clean.

## Identity

`head-sha.txt`, `go-version.txt`, `binary-sha256.txt`,
`binary-version.txt`, `build-identity.txt`, `worktree-status.txt`,
`task-range-diff.txt`, `identity/hashes.txt` — the directory name is the
tested code SHA; this evidence lives in a separate evidence-only commit
that is its child. Self-verification is not reviewer acceptance.
