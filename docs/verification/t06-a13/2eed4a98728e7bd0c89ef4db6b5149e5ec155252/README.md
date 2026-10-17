# T06-A13: five CREATE TABLE type-family forbid policies proof

Code under test: `2eed4a98728e7bd0c89ef4db6b5149e5ec155252`
(`test(ddl): T06-A13 type-family forbid proof (isolation, attribution, switches, four-anchor structure)`)

Zero production-code changes. The slice extends the frozen T06 golden
contract from 360 to **412 cases** — the A1–A12 contract (case
identities, SQL, profiles, expectations) is preserved (every pre-A13
case is contract-identical under normalized field comparison; the file
keeps its indent/UTF-8/key-order conventions), while this run's 412
records were all executed fresh and belong to `head_sha` `2eed4a98`;
52 new records are added under ten new isolated profiles plus the
reused `all-rules-disabled` control:

- `t06-a13-<family>-on-isolated` — only the family rule at `blocker`,
  `params.forbid=true`, for `<family>` in `blob-text`, `json`, `bit`,
  `float-double`, `timestamp`
- `t06-a13-<family>-false-isolated` — same rule at `blocker`,
  `params.forbid=false` (loaded but inapplicable — never conflated with
  disabled)

## What is proven

**Product (real parser + AuditSQL + CLI, both dialects):** 16 frozen
roles × 2 dialects = 32 offline CLI runs over two fixed inputs —
mixed `M` (`CREATE TABLE t (c_blob BLOB, c_text TEXT, c_json JSON,
c_bit BIT(1), c_float FLOAT, c_double DOUBLE, c_timestamp TIMESTAMP
NULL, c_datetime DATETIME NULL, c_int INT);`) and safe `S`
(`CREATE TABLE t (c_int INT, c_string VARCHAR(8), c_datetime DATETIME
NULL);`):

| Role | Input × profile | Expected/actual |
| --- | --- | --- |
| `<family>-mixed-hit` | M × on-isolated | reject rc=1: blob-text→2 blockers (`c_blob` `blob binary`, `c_text`; `c_json` excluded), json→1 (`c_json`), bit→1 (`c_bit` `bit(1)`), float-double→2 (`c_float`,`c_double`), timestamp→1 (`c_timestamp`; `c_datetime` excluded) |
| `<family>-safe` | S × on-isolated | pass rc=0, 0 findings, `rule_summary` loaded=1 applicable=1 (applicable, no matching column) |
| `<family>-forbid-false` | M × false-isolated | pass rc=0, 0 findings, `rule_summary` loaded=1 applicable=0 (`forbid=false`) |
| `mixed-off` | M × all-rules-disabled | pass rc=0, `rule_summary` key absent per the serialization contract |

Generic rules pin `column "<name>" must not use the <family> type
family` with `{table,column,type,family,charset:""}` metadata; the
independent float/double rule pins `column "c_float" should not use
float` / `column "c_double" should not use double` with
`{table,column,type}` metadata only. Complete finding entries are
matched as a multiset — collapse, duplication, and message/metadata
cross-pairing all fail validation. `TIMESTAMP NULL` is a fixed input
fact only; no implicit-default or runtime claim follows.

**Native (four anchors: MySQL 5.7.44 / 8.0.46 / 8.4.10 / TiDB 8.5.0):**

- 20 anchored metadata cases (five `mixed-hit` roles × 4 anchors).
- Per case: setup absence check → product audit (reject) → post-audit
  absence check (the product never executes the audited DDL) → driver
  applies the same CREATE with `rc=0` → structure oracles → teardown +
  residual count.
- Structure oracle per case: one user table, exactly nine columns, the
  ordered `schema:table:column:DATA_TYPE:ordinal` GROUP_CONCAT set
  `golden:t:c_blob:blob:1|…|golden:t:c_int:int:9`, zero PK constraints,
  zero primary index parts, no secondary indexes, InnoDB on MySQL
  anchors (TiDB not claimed as InnoDB). GIPK instance facts recorded on
  MySQL 8.0/8.4.

## Layout

- `artifact.json` + `cases/` — the 412-case artifact and per-case raw
  records (argv/stdout/stderr/rc, parsed output, driver steps,
  structure/teardown queries)
- `policies/` — the generated policy files used by this run (built from
  the live `rules list` catalog; catalog size never hardcoded)
- `T06-manifest.json` / `anchors.json` — the manifest and fixed anchors
- `gates/` — argv+stdout/stderr+rc for every mandated gate
- `identity/hashes.txt` — sha256 of every committed artifact file
- `build-identity.txt`, `binary-version.txt`, `binary-sha256.txt`,
  `head-sha.txt`, `worktree-status.txt`, `go-version.txt` — source and
  build identity (the runner binary stays outside the repo at
  `/tmp/ds-t06-a13-proof/T06/bin/deltascope`, hash recorded)
- `baseline/` — the pre-implementation baseline evidence
  (`/tmp/ds-t06-a13-baseline/` originals: 32-run matrix, 11 hand-built
  policies, rules-list catalog, M.sql/S.sql, real-AST probe) collected
  at `84e37e32` and reused, not rerun
- `task-range-diff.txt` — `git diff 84e37e32..2eed4a98` (the code
  commit)

## Gates (all at `2eed4a98`, real rc recorded under `gates/`)

- `go test -count=1 ./internal/infrastructure/parser/tidb ./internal/domain/rule/ddl ./internal/application/audit -run T06A13 -v` — rc=0 (real `TestT06A13*` matches, non-empty)
- `go test -count=1 ./pkg/deltascope ./internal/interfaces/http -run T06A13 -v` — rc=0 (SDK/HTTP representatives)
- `make ddl-golden-validator-test` — rc=0, 664 contract cases, 0 failures (42 A13 controls: missed/forged findings, absorbed c_json/c_datetime, collapsed/duplicated/cross-paired findings, metadata-key mutations, same-SQL family swaps, switch-state laundering, weakened profiles, column-set corruptions, skipped/failed driver replay, denominator shrinkage)
- `make ddl-golden TASK=T06` — rc=0, 412/412 on the four frozen anchors, cleanup rc=0
- `python3 scripts/ddl_golden.py validate --artifact .../artifact.json` — rc=0, binary check on
- `GOFLAGS=-count=1 make test` — rc=0
- `make pg-unit-test-gates` / `sql-corpus-gates` / `ddl-inventory-gate` / `ddl-coverage-catalog-test` / `docs-example-gates` / `decision-record-gate` — all rc=0

## Boundaries recorded, not widened

- The blob/text forbid predicate is the registered
  `isBlobTextLike && !json` pair — the A12 `isBlobTextLike` exemption
  set still contains `json`; the two sets are not interchangeable.
- The timestamp forbid matches `baseType=="timestamp"` only; `DATETIME`
  is never absorbed.
- Size variants (TINYBLOB/LONGTEXT etc.), BIT lengths, FLOAT precision /
  REAL aliases / sql_mode, JSON document/default/index semantics,
  TIMESTAMP implicit defaults / ranges / time zones, DECIMAL
  precision/scale, ALTER-side type governance, and runtime values stay
  unproven or deferred.
- `.mcp.json` carries an external tool modification that predates this
  slice — recorded in `worktree-status.txt`, never staged, never
  committed; `+dirty` in the build version attributes to that
  modification plus the pre-existing untracked tool directories, not to
  this slice's files (all committed).
