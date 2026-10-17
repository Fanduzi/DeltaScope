# T06-A14: ordinary CREATE TABLE table-option allowlist proof

Code under test: `ded0ed51bcbf4e34952438e53f6b08073425b800`
(`test(ddl): T06-A14 table-option allowlist proof (require_explicit matrix, per-product catalog oracle)`)

Zero production-code changes. The slice extends the frozen T06 golden
contract from 412 to **472 cases** — the A1–A13 contract (case
identities, SQL, profiles, expectations) is preserved (every pre-A14
case is contract-identical under normalized field comparison; the file
keeps its indent/UTF-8/key-order conventions), while this run's 472
records were all executed fresh and belong to `head_sha` `ded0ed51`;
60 new records are added under six new isolated profiles plus the
reused `all-rules-disabled` control:

- `t06-a14-<family>-strict-isolated` — only the family rule at
  `blocker`, `params.values=[<value>]`, `require_explicit=true`, for
  `<family>` in `engine`, `charset`, `row-format`
- `t06-a14-<family>-optional-isolated` — same rule and values at
  `blocker`, `require_explicit=false` (omission exempted; explicit
  disallowed values still reject — never a satisfied-state conflation)

## What is proven

**Product (real parser + AuditSQL + CLI, both dialects):** 6 roles ×
3 families × 2 dialects = 36 offline CLI runs over seven fixed inputs:

| Role | Input × profile | Expected/actual |
| --- | --- | --- |
| `<family>-allowed-strict` | allowed SQL × strict | pass rc=0, 0 findings, `rule_summary` loaded=1 applicable=1 |
| `<family>-denied-strict` | denied SQL × strict | reject rc=1: exactly 1 blocker |
| `<family>-omitted-strict` | `CREATE TABLE t (c INT);` × strict | reject rc=1: 1 blocker, `metadata.actual=""` — an absent `Options` key, never an observed server default |
| `<family>-omitted-optional` | same SQL × optional | pass rc=0, 0 findings, loaded=1 applicable=1 — never the `forbid=false` 1/0 state |
| `<family>-denied-optional` | denied SQL × optional | reject rc=1: `require_explicit=false` exempts omission only |
| `<family>-denied-off` | denied SQL × all-rules-disabled | pass rc=0, `rule_summary` key absent per the serialization contract |

Frozen messages `table engine must be one of [InnoDB]` /
`table charset must be one of [utf8mb4]` /
`table row format must be one of [DYNAMIC]`; metadata
`{table:t, option:<key>, actual:<declared|"">, allowed:[<value>]}` with
the declared case preserved (`MyISAM`, `latin1`, `COMPACT`); complete
finding entries matched via `finding_entries_full`. A column-level
`CHARACTER SET utf8mb4` lands `Column.Charset` without fabricating
`Options["charset"]` — a column declaration cannot satisfy the table
charset requirement (Go control + extractor probe).

**Native (four anchors: MySQL 5.7.44 / 8.0.46 / 8.4.10 / TiDB 8.5.0):**

- 24 anchored metadata cases (allowed-strict + denied-strict ×
  3 families × 4 anchors).
- Per case: setup absence check → product audit → post-audit absence
  check (the product never executes the audited DDL) → driver applies
  the same CREATE with `rc=0` → structure oracles → teardown +
  residual count. Policy rejects and native CREATE success are
  recorded side by side, never conflated.
- Shared fixture shape per case: exactly one user table, exactly one
  nullable column `c:int:1:YES`, zero PK constraints, zero primary
  index parts, no secondary indexes. GIPK instance facts recorded on
  MySQL 8.0/8.4.
- Per-product catalog oracle (real records in `cases/`):
  - engine: MySQL `golden:t:InnoDB`/`golden:t:MyISAM` by declared
    value; TiDB `golden:t:InnoDB` on both roles — MySQL-compatible
    catalog display, never a physical-storage claim.
  - row_format: MySQL `golden:t:InnoDB:Dynamic` /
    `golden:t:InnoDB:Compact` (ENGINE=InnoDB pins the fixture's
    default-engine premise only); TiDB `golden:t:InnoDB:Compact` on
    both roles — TiDB parses but ignores ROW_FORMAT.
  - charset: `golden:t:utf8mb4` / `golden:t:latin1` through the real
    `TABLE_COLLATION`→`COLLATION_CHARACTER_SET_APPLICABILITY`
    association with an exactly-one-row check on all four anchors —
    never a column's charset, never a collation-name prefix, never a
    hardcoded default collation.

## Baseline reuse (old-HEAD evidence, not re-executed)

`baseline/` carries the publishable T06-A14-BASELINE originals
collected on evidence HEAD `a2ea0b6c` (36 offline CLI
argv/stdout/stderr/rc records, seven isolated policies, rules catalog,
parser AST→Options probe output and source, fixed SQL files).
`baseline/binary-sha256.txt` records the baseline binary hash; the
binary itself stays at `/tmp/ds-t06-a14-baseline/deltascope`
(33,174,818 bytes) outside the repo. The baseline established the
source-expected semantics this slice now proves permanently; it was
not re-run here.

## Gates (argv/stdout/stderr recorded under `gates/`, all rc=0)

| Command | Result |
| --- | --- |
| `go test -count=1 ./internal/infrastructure/parser/tidb ./internal/domain/rule/ddl ./internal/application/audit -run T06A14 -v` | PASS — extraction, domain matrix/params, audit roles, derived pre-state |
| `go test -count=1 ./pkg/deltascope ./internal/interfaces/http -run T06A14 -v` | PASS — SDK/HTTP representatives |
| `make ddl-golden-validator-test` | PASS — 711 contract cases, 0 failures (incl. the A14 mutation group) |
| `make ddl-golden TASK=T06 ARTIFACT_DIR=/tmp/ds-t06-a14-proof` | PASS — 472/472, 11392 assertions, four anchors at head ded0ed51 |
| `python3 scripts/ddl_golden.py validate --artifact …/T06/artifact.json` | PASS — `[ddl-golden] artifact valid`, binary check on |
| `GOFLAGS=-count=1 make test` | PASS — full repository suite |
| `make pg-unit-test-gates sql-corpus-gates ddl-inventory-gate ddl-coverage-catalog-test docs-example-gates decision-record-gate` | all PASS (06-misc-gates.log) |

## Layout

- `artifact.json`, `cases/` — the full 472-case run at `ded0ed51`
- `T06-manifest.json`, `anchors.json` — the frozen contract and pinned
  anchor identities the run executed against
- `policies/` — the 41 generated policy files the run actually used
  (incl. the six `t06-a14-*-isolated` profiles), sha256-bound in the
  artifact
- `gates/` — gate argv/stdout/stderr/rc records
- `version-refs/` — retrieval-dated official-doc excerpts that fix the
  per-product catalog expectations (TiDB "parsed but ignored",
  InnoDB/Compact display; MySQL ENGINE/ROW_FORMAT/TABLE_COLLATION
  semantics; table charset/clause rules)
- `baseline/` — old-HEAD baseline originals (see above)
- `binary-sha256.txt`, `binary-version.txt`, `build-identity.txt`,
  `go-version.txt`, `head-sha.txt` — source/build identity; the proof
  binary stays at `/tmp/ds-t06-a14-proof/T06/bin/deltascope`
  (`v0.511.2-0.20261017155122-ded0ed51bcbf+dirty`, sha256
  `b5ec6a68…824c`) outside the repo — `+dirty` reflects external
  worktree state (`.mcp.json` modification, tool dirs), not code drift
- `identity/hashes.txt` — artifact/manifest/binary hashes
- `task-range-diff.txt` — `a2ea0b6c..ded0ed51` diffstat + name list
- `worktree-status.txt` — tree state at evidence time

Self-check is not an independent reviewer pass.
