# T06-A11: single-member primary-key BIGINT/UNSIGNED policy proof

Code under test: `4dc51cbd424d2e1f13a9658622820c64685da340`
(`test(ddl): prove single-member primary-key BIGINT/UNSIGNED policies (T06-A11)`)

Zero production-code changes. The slice extends the frozen T06 golden
contract from 292 to **324 cases** — the A1–A10 contract (case
identities, SQL, profiles, expectations) is preserved unchanged, while
this run's 324 records were all executed fresh and belong to
`head_sha` `4dc51cbd` (the historical run records stay in their original
evidence directories under their own SHAs); 32 new records are added
under two new isolated profiles:

- `t06-a11-pk-bigint-isolated` — only
  `ddl.table.primary_key.bigint.require` at `blocker`,
  `params.required=true`
- `t06-a11-pk-unsigned-isolated` — only
  `ddl.table.primary_key.unsigned.require` at `blocker`,
  `params.required=true`

## What is proven

**Product (real parser + AuditSQL + CLI, both dialects):** 8 frozen
roles × 2 dialects = 16 offline CLI runs:

| Role | Input | Expected/actual |
| --- | --- | --- |
| bigint-pass | `CREATE TABLE t (id BIGINT PRIMARY KEY);` | pass, 0 findings, exit 0 |
| bigint-reject | `CREATE TABLE t (id INT PRIMARY KEY);` | exactly 1 blocker `ddl.table.primary_key.bigint.require`, `primary key column "id" must use bigint`, metadata `{table:t,column:id,type:int(11)}`, exit 1, `fail_on_triggered` |
| bigint-composite-skip | `CREATE TABLE t (id INT, tenant_id INT, PRIMARY KEY(id,tenant_id));` | pass — member count ≠ 1 skips the check; does not prove the composite satisfies the policy |
| bigint-off | same as bigint-reject under `all-rules-disabled` | pass, 0 findings |
| unsigned-pass | `CREATE TABLE t (id BIGINT UNSIGNED PRIMARY KEY);` | pass, 0 findings |
| unsigned-reject | `CREATE TABLE t (id BIGINT PRIMARY KEY);` | exactly 1 blocker `ddl.table.primary_key.unsigned.require`, `primary key column "id" must be unsigned`, metadata `{table:t,column:id,type:bigint(20)}`, exit 1 |
| unsigned-composite-skip | `CREATE TABLE t (id BIGINT, tenant_id INT, PRIMARY KEY(id,tenant_id));` | pass — same skip semantics |
| unsigned-off | same as unsigned-reject under `all-rules-disabled` | pass, 0 findings |

`int(11)`/`bigint(20)` are parser-rendered forms (`Tp.String()`), not
declared widths and not numeric ranges. Roles bigint-pass and
unsigned-reject share the same SQL but are bound by SQL + profile +
verdict — no cross-borrowing.

**Native (four anchors: MySQL 5.7.44 / 8.0.46 / 8.4.10 / TiDB 8.5.0):**

- 16 anchored metadata cases (bigint-pass / bigint-reject / unsigned-pass
  / unsigned-reject × 4 anchors).
- Per case: setup absence check → product audit → post-audit absence
  check (the product never executes the audited DDL) → driver applies
  the same CREATE with `rc=0` — including after policy rejects →
  structure oracles → teardown + residual count.
- Structure oracle per case: one table, one column `id:1:NO`,
  `DATA_TYPE` exact (`bigint`/`int`), finite-whitelist complete-form
  `COLUMN_TYPE` via `CAST(LOWER(COLUMN_TYPE) AS BINARY)` comparison,
  byte-empty `EXTRA`, one PK constraint with member `id:1`, no secondary
  indexes, InnoDB on MySQL anchors (TiDB not claimed as InnoDB).
- Raw nine-field column observation
  (`type|schema|table|column|DATA_TYPE|ordinal|nullable|HEX(COLUMN_TYPE)|HEX(EXTRA)`)
  bound to `golden.t.id`, e.g. mysql57 unsigned-pass recorded
  `type|golden|t|id|bigint|1|NO|626967696E742832302920756E7369676E6564|`
  → `bigint(20) unsigned`, `EXTRA` byte-empty.

## Directory layout

| Path | Contents |
| --- | --- |
| `artifact.json` | Complete 324-case run artifact (`head_sha` = tested SHA). |
| `cases/` | All 324 per-case records of this run (the A1–A10 cases re-executed under their unchanged contracts, plus 32 new A11 records) — all attributed to `head_sha` `4dc51cbd`. |
| `policies/` | The 22 generated policy files actually used (sha256-recorded in the artifact). |
| `T06-manifest.json` | The frozen manifest as run. |
| `anchors.json` | Anchor image/digest/version/reachable summary. |
| `build-identity.txt` | Source/build identity, binary sha256, worktree state, `+dirty` attribution. |
| `identity/hashes.txt` | sha256/git blob ids of the evidence-bearing source files + artifact + binary. |
| `gates/` | argv/stdout/stderr/rc for every gate run at the tested SHA. |

## Gate summary (all at `4dc51cbd`)

| Gate | Result |
| --- | --- |
| `go test -count=1 … -run T06A11 -v` (parser + rule + audit) | rc=0 — all T06A11 tests/subtests pass; `-run` verified non-empty |
| `go test -count=1 ./pkg/deltascope ./internal/interfaces/http -run T06A11 -v` | rc=0 — SDK/HTTP representatives pass |
| `make ddl-golden-validator-test` | rc=0 — 507 contract cases, 0 failures (39 A11 mutation groups all reject) |
| `make ddl-golden TASK=T06` | rc=0 — **324/324 cases, 7944 assertions**, four anchors |
| `ddl_golden.py validate --artifact` (binary check enabled) | rc=0 — `artifact valid` |
| `GOFLAGS=-count=1 make test` | rc=0 |
| `pg-unit-test-gates` / `sql-corpus-gates` / `ddl-inventory-gate` / `ddl-coverage-catalog-test` / `docs-example-gates` / `decision-record-gate` | rc=0 each |

## Scope honesty

- The BIGINT and UNSIGNED policies are independent: signed BIGINT
  satisfies the bigint rule, `INT UNSIGNED` satisfies the unsigned rule
  but not the bigint rule — proven by shared-layer controls, not extra
  DB cases.
- `required=false` and whole-rule `disabled` remain distinct controls;
  no `satisfied`/`skip_reason`/public proof fields were added.
- ZEROFILL is a parser-only observation (implied unsigned flag) covered
  by an extraction regression test — no Golden role, no version/legality
  claim, no recommendation.
- The rules' existing ALTER applicability seam is unchanged; this slice
  proves ordinary `CREATE TABLE` only and does not claim ALTER behavior.
- The decision-record gate's large-diff path is heuristic (`SIGPIPE`
  shortcut prints "no trigger keywords, PASS"); the ADR itself is
  committed at `docs/decisions/2026-10-14-ddl-primary-key-bigint-unsigned-proof.md`.
- Baseline evidence from the pre-implementation round is preserved
  outside the repo at `/tmp/ds-t06-a11-baseline/` and attributed to the
  old HEAD `5330912f`; this directory is the post-commit proof.

## Not executed / out of scope

No INSERT/overflow/numeric-range checks, no integer display-width
version governance, no alias exhaustiveness, no ALTER cases, no
PostgreSQL changes, no mock substitution — all 324 records come from the
single real run at the tested SHA.
