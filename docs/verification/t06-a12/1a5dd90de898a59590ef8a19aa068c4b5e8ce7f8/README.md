# T06-A12: ordinary-column NOT NULL policy and per-column attribution proof

Code under test: `1a5dd90de898a59590ef8a19aa068c4b5e8ce7f8`
(`test(ddl): prove ordinary-column NOT NULL policy and attribution (T06-A12)`)

Zero production-code changes. The slice extends the frozen T06 golden
contract from 324 to **360 cases** — the A1–A11 contract (case
identities, SQL, profiles, expectations) is preserved byte-identical,
while this run's 360 records were all executed fresh and belong to
`head_sha` `1a5dd90d`; 36 new records are added under three new isolated
profiles plus the reused `all-rules-disabled` control:

- `t06-a12-not-null-strict-isolated` — only
  `ddl.column.not_null.require` at `blocker`, `required=true`,
  `allow_time_null=false`
- `t06-a12-not-null-time-allow-isolated` — same rule at `blocker`,
  `required=true`, `allow_time_null=true`
- `t06-a12-not-null-not-required-isolated` — same rule at `blocker`,
  `required=false` (loaded but inapplicable — never conflated with
  disabled)

## What is proven

**Product (real parser + AuditSQL + CLI, both dialects):** 8 frozen
roles × 2 dialects = 16 offline CLI runs:

| Role | Input | Profile | Expected/actual |
| --- | --- | --- | --- |
| ordinary-nullable | `CREATE TABLE t (c INT);` | strict | exactly 1 blocker `ddl.column.not_null.require`, `column "c" should be declared NOT NULL`, metadata `{table:t,column:c,type:int(11)}`, exit 1, `fail_on_triggered` |
| ordinary-not-null | `CREATE TABLE t (c INT NOT NULL);` | strict | pass, 0 findings, exit 0 |
| time-exempt | `CREATE TABLE t (c DATETIME NULL);` | time-allow | pass — `allow_time_null` exemption; loaded+applicable |
| time-required | `CREATE TABLE t (c DATETIME NULL);` | strict | exactly 1 blocker, same message shape, metadata `type:datetime` — same SQL as time-exempt, opposite verdict |
| text-exempt | `CREATE TABLE t (payload TEXT);` | strict | pass — BLOB/TEXT exemption independent of `allow_time_null` |
| mixed-attribution | `CREATE TABLE t (ordinary INT, time_c DATETIME NULL, payload TEXT);` | time-allow | exactly 1 blocker bound to `ordinary`/`int(11)`; the exempted columns emit nothing |
| required-false | `CREATE TABLE t (c INT);` | not-required | pass — rule loaded (`loaded=1`) but `applicable=0` |
| disabled | `CREATE TABLE t (c INT);` | all-rules-disabled | pass — `rule_summary` absent per the serialization contract |

`int(11)`/`datetime` are parser-rendered forms (`Tp.String()`), not
declared widths. `time-exempt`/`time-required` and
`ordinary-nullable`/`required-false`/`disabled` share SQL texts but are
bound by (sql, profile, verdict) — no cross-borrowing.

**Native (four anchors: MySQL 5.7.44 / 8.0.46 / 8.4.10 / TiDB 8.5.0):**

- 20 anchored metadata cases (ordinary-nullable / ordinary-not-null /
  time-exempt / time-required / mixed-attribution × 4 anchors).
- Per case: setup absence check → product audit → post-audit absence
  check (the product never executes the audited DDL) → driver applies
  the same CREATE with `rc=0` — including after policy rejects →
  structure oracles → teardown + residual count.
- Structure oracle per case: one user table, exact column count, ordered
  `schema:table:column:DATA_TYPE:ordinal:IS_NULLABLE` GROUP_CONCAT set
  (`golden:t:c:int:1:YES`, `golden:t:c:int:1:NO`,
  `golden:t:c:datetime:1:YES`, and the three-column mixed set), zero PK
  constraints, zero primary index parts, no secondary indexes, InnoDB on
  MySQL anchors (TiDB not claimed as InnoDB). Policy-rejected nullable
  columns record `IS_NULLABLE=YES` — the verdict never rewrites the
  stored declaration. GIPK instance facts recorded on MySQL 8.0/8.4
  (`sql_require_primary_key=OFF`, `sql_generate_invisible_primary_key=OFF`,
  `show_gipk_in_create_table_and_information_schema=ON`).

**Three rule_summary states pinned per role:** loaded+applicable
(isolated roles), loaded+inapplicable (`required=false`), absent
(disabled) — re-parsed from recorded raw stdout by
`t06_a12_artifact_failures`, never trusted from a stored blob.

## Directory layout

- `artifact.json` — the complete 360-case artifact (all 360 records
  executed in this run; none spliced from earlier runs)
- `cases/` — per-case raw records (argv/stdout/stderr/rc, setup,
  post_verify, driver execute, structure, teardown)
- `policies/` — the generated policy files used by this run (built from
  the live `rules list` catalog; catalog size never hardcoded)
- `T06-manifest.json` / `anchors.json` — the manifest and fixed anchors
- `gates/` — argv+stdout/stderr+rc for every mandated gate
- `identity/hashes.txt` — sha256 of every committed artifact file
- `build-identity.txt`, `binary-version.txt`, `binary-sha256.txt`,
  `head-sha.txt`, `worktree-status.txt`, `go-version.txt` — source and
  build identity
- `baseline/` — the pre-implementation baseline evidence
  (`/tmp/ds-t06-a12-baseline/` originals: baseline.txt, rules-list.json,
  the four hand-built probe policies, 9 SQL inputs, 16 run records, and
  the real-AST extractor probe)
- `task-range-diff.txt` — `git diff 907ce213..1a5dd90d` (the code commit)

## Gates (all at `1a5dd90d`, real rc recorded under `gates/`)

- `go test -count=1 ./internal/infrastructure/parser/tidb ./internal/domain/rule/ddl ./internal/application/audit -run T06A12 -v` — rc=0
- `go test -count=1 ./pkg/deltascope ./internal/interfaces/http -run T06A12 -v` — rc=0 (SDK/HTTP representatives)
- `make ddl-golden-validator-test` — rc=0, 587 contract cases, 0 failures (33 A12 controls: missed/forged blockers, re-attributed and cross-paired findings, same-SQL role swaps, required=false/disabled laundering, weakened profile params, IS_NULLABLE forgeries, column-set corruptions, skipped driver replay, cross-anchor/duplicate/denominator shrinkage)
- `make ddl-golden TASK=T06` — rc=0, 360/360 on the four frozen anchors, cleanup rc=0, no residual containers
- `python3 scripts/ddl_golden.py validate --artifact .../artifact.json` — rc=0, binary check on
- `GOFLAGS=-count=1 make test` — rc=0
- `make pg-unit-test-gates` / `sql-corpus-gates` / `ddl-inventory-gate` / `ddl-coverage-catalog-test` / `docs-example-gates` / `decision-record-gate` — all rc=0

## Boundaries recorded, not widened

- Explicit ordinary `NULL` and omitted nullability both normalize to
  `Column.NotNull=false`; the AST distinguishes the option kinds. The
  normalized spec does not preserve the provenance — current behavior,
  not an A12 defect; no provenance field added.
- DATETIME and TEXT are exemption representatives only; the full
  time-like and blob-text-like exemption matrices are not claimed.
- Runtime INSERT NULL behavior, SQL mode, TIMESTAMP implicit defaults,
  ALTER-side nullability, and PG behavior beyond the existing coverage
  corpus are out of scope.
- `.mcp.json` carries an external tool modification that predates this
  slice — recorded in `worktree-status.txt`, never staged, never
  committed; `+dirty` in the build version attributes to that
  modification plus the pre-existing untracked tool directories, not to
  this slice's files (all committed).
