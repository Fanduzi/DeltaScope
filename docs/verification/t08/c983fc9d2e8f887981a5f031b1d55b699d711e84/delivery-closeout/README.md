# T08 delivery closeout — verification reconciliation

Closeout scope: delivery of the already-reviewed T08 candidate to
`origin/workfree` plus reconciliation of every verification the original
long-task work order required. No production code, tests, manifests,
profiles, oracle, or denominator was changed by this closeout.

- Tested code: `c983fc9d2e8f887981a5f031b1d55b699d711e84`
- Evidence commit delivered: `18f34abb32c411329c0be36c4eb61a6a4ce3f70b`
- This closeout index: evidence-only supplement under the same tested-SHA
  archive; it does not retroactively become tested code.

Push transaction and identity chain: `preflight.txt`.

## 1. Gate reconciliation table

Classification vocabulary per the work order: `有效可复用` (valid original at
the right SHA) / `本轮补跑` (rerun this closeout) / `缺失` (missing) /
`归属不清` (unclear attribution) / `真实失败` (real failure) /
`待独立裁定` (frozen-oracle mismatch needing adjudication).

### A. T08 contract surface

| Gate | Command | Tested SHA | rc | Original | Class |
|---|---|---|---|---|---|
| T08 golden | `python3 ./scripts/ddl_golden.py run --task T08 --artifact-dir /private/tmp/ddl-golden-t08-final3` | c983fc9d | 0 | `../golden/T08/artifact.json` — 172/172, 4240 assertions, binary check on | 有效可复用 |
| T08 contract validator | `python3 scripts/test_t08_contract.py` | c983fc9d | 0 | `../gates/t08-contract-validator.log` — 25/25 | 有效可复用 |
| Golden manifest validator | `python3 scripts/test_ddl_golden.py` | c983fc9d | 0 | `../gates/ddl-golden-validator-test.log` | 有效可复用 |
| Denominator/ids digest | embedded in validator + `testdata/ddl-golden/T08.json` | c983fc9d | — | 172 cases, ids sha256 `fb472107…31b18c` | 有效可复用 |
| Denominator provenance | `docs/work-plans/t08-column-governance/EXPECTED-UPGRADES.md` §Admission reconciliation + Post-review amendments | — | — | 162 roles → 163 (A-tidb-ai-drop) → manifest 171 → 172 | documented |

Role attribution inside the 172: 8 baseline db cases (`T08.db.<anchor>.ddl` /
`.syntax_negative`), 39 cli (incl. parser-negative `err-syntax` and
`unsupported-aspect`), 3 clierr pre-result (`err-bad-dialect`,
`err-policy-invalid`, `err-provider-failure`), 122 metadata cases. The
`err.cancel` role is unit-admitted (`internal/application/audit` T08 test
file), counted in the 163-role ledger but not in the 172-case manifest.

### B. Old-task database regressions at c983fc9d

All reruns executed this closeout on a detached worktree pinned to
`c983fc9d2e8f` (`/tmp/ds-t08-closeout`), serial order, per-task freshly
built CLI sha256 `42c468d5…0798a4d` (go1.26.1). Full per-task provenance in
`regressions/<T>/run.log`; complete originals in
`regressions/<T>/artifact.json`.

| Task | Cases | Passed | Result | Class |
|---|---|---|---|---|
| T02 | 10 | 10 | reused T07-era original at aba44783 (see `regressions/T02/REUSE.md`) | 有效可复用 |
| T03 | 118 | 115 | 3 fails | 待独立裁定 (frozen-oracle drift, upgrade class U-pos) |
| T04 | 163 | 160 | 3 fails | 待独立裁定 (U-pos) |
| T05 | 247 | 242 | 5 fails | 待独立裁定 (U-pos ×3, U-compat-default ×2) |
| T06 | 496 | 496 | PASS | 本轮补跑 green |
| T07 | 207 | 207 | PASS | 本轮补跑 green |

#### Failing-cell attribution (all 11 are frozen-oracle drift)

Old manifests were intentionally not migrated during T08
(`EXPECTED-UPGRADES.md` bound upgrades to unit tests, one T07 golden case,
and the corpus only). This closeout does not edit expected files; each cell
is recorded for independent adjudication.

**Class 1 — column_position promotion (9 cells).**
Frozen expectation: `exit=1`, `verdict=review`, `coverage=incomplete`,
`diagnostics=1`, `unsupported=1` with feature
`alter_table.<action>.column_position`; the T04/T05 modify cells also
declared `evidence_gaps=1` (`missing_source_column`).
Actual at c983fc9d: `exit=0`, `verdict=pass`, `coverage=complete`,
`unsupported=[]`, `diagnostics=[]` for the six ADD cells and the T03
MODIFY cell; `exit=0`, `verdict=review`, `coverage=unverified` with the
`missing_source_column` gap preserved for the T04/T05 MODIFY cells.
Producing change: `spec.AlterPosition` + ordered successor state +
`ddl.alter.column_position.exists.require` (decision record
`docs/decisions/2026-10-28-ddl-column-position-and-multi-action-state.md`).

| Case id | Statement | Actual at c983fc9d |
|---|---|---|
| `T03.cli.mysql-alter-add-column-position` | `ALTER TABLE t ADD COLUMN c INT FIRST;` | pass / complete |
| `T03.cli.tidb-alter-add-column-position` | `ALTER TABLE t ADD COLUMN c INT AFTER id;` | pass / complete |
| `T03.cli.mysql-alter-modify-column-position` | `ALTER TABLE t MODIFY COLUMN c INT AFTER id;` | pass / complete (all-rules-disabled profile — no compat gap) |
| `T04.cli.mysql-alter-add-column-position` | `ALTER TABLE t ADD COLUMN c INT FIRST;` | pass / complete |
| `T04.cli.tidb-alter-add-column-position` | `ALTER TABLE t ADD COLUMN c INT AFTER id;` | pass / complete |
| `T04.cli.mysql-alter-modify-column-position` | `ALTER TABLE t MODIFY COLUMN c INT AFTER id;` | review / unverified + `missing_source_column` gap |
| `T05.cli.mysql-alter-add-column-position` | `ALTER TABLE t ADD COLUMN c INT FIRST;` | pass / complete |
| `T05.cli.tidb-alter-add-column-position` | `ALTER TABLE t ADD COLUMN c INT AFTER id;` | pass / complete |
| `T05.cli.mysql-alter-modify-column-position` | `ALTER TABLE t MODIFY COLUMN c INT AFTER id;` | review / unverified + `missing_source_column` gap |

**Class 2 — §5.4 effective-default transition (2 cells).**
`T05.meta.t05-a4-{mysql84,tidb85}-omit-integer`, statement:
`ALTER TABLE t MODIFY COLUMN c INT;` on a column with stored `DEFAULT 1`.
Frozen expectation: 2 findings (unsigned-drop at stmt 1, nullability at
stmt 2). Actual: 3 findings — the additional stmt-1 finding is
`ddl.alter.modify_column.compatibility.require` with
`source_default=literal → target_default=none` ("loses its default value").
Producing change: explicit-default semantics applied to MODIFY/CHANGE
(`publishColumnDefault` + §5.4 compare). Native evidence on all four
anchors confirmed an omitted `DEFAULT` clause clears the stored default —
the new finding is correct per the shipped contract and per the server.

No failure is an environment error (all anchors healthy, credential envs
present, `compose_down_rc=0`) and none is a product crash or wrong verdict
— each actual result is the T08-promoted audited behavior against the
pre-T08 frozen cell.

### C. Offline & shared gates (archived originals at c983fc9d)

| Gate | Command | rc | Original |
|---|---|---|---|
| unit suite | `env GOFLAGS=-count=1 make test` | 0 | `../gates/make-test.log` |
| pg unit gates | `make pg-unit-test-gates` | 0 | `../gates/pg-unit-test-gates.log` |
| corpus gates | `make sql-corpus-gates` | 0 | `../gates/sql-corpus-gates.log` |
| inventory | `make ddl-inventory-gate` | 0 | `../gates/ddl-inventory-gate.log` |
| coverage catalog | `make ddl-coverage-catalog-test` | 0 | `../gates/ddl-coverage-catalog-test.log` |
| docs examples | `make docs-example-gates` | 0 | `../gates/docs-example-gates.log` |
| decision records | `make decision-record-gate` | 0 | `../gates/decision-record-gate.log` |
| golden validator | `python3 scripts/test_ddl_golden.py` | 0 | `../gates/ddl-golden-validator-test.log` |
| three-level doc | `check_three_level_doc.sh` (skill path) | 0 | `../gates/three-level-doc.log` |
| gofmt | `gofmt -l` over changed Go files | 0 | `../gates/gofmt.meta` |

Race / request isolation: this closeout carried **no `-race` evidence** —
the paragraph below previously presented non-race coverage as a stand-in;
that wording was corrected during the oracle-closure round (see §5
Errata). The affected surfaces (ordered state, position, multi-action) are
exercised by `T07R1RequestIsolation` tests and the T08 multi-action/state
tests inside `make test` (`-count=1`, no cache), but those are ordinary
runs, not race-detector evidence. Real `go test -race` originals were
produced in the follow-up oracle-closure round and live under
`docs/verification/t08/<oracle-closure proof sha>/race/`. SDK/CLI/HTTP/MCP
representative checks are the `make test` package suite plus the CLI path
exercised by every golden cli/metadata case above; the HTTP/MCP adapters
do not consume the changed state layer (no diff under
`internal/interfaces/` in the task range).

### D. Protected scope

| Requirement | Evidence |
|---|---|
| T07 eval-gap & state withholding not bypassed | T07 golden 207/207 at c983fc9d (`regressions/T07/artifact.json`); blocker+gap coexistence pinned by `TestAuditSQLT08EvalGapCoexistsWithBlocker` |
| T06 fidelity & ordinary-declaration regression | T06 golden 496/496, 11,772 assertions (`regressions/T06/artifact.json`) |
| PG paths inert | `make pg-unit-test-gates` rc=0 at c983fc9d; T08 PG control test under `postgresql` tag |
| T05 single-action & statistics cleanup bounds | all non-drift T05 cells pass (242/247; the 5 fails are the two documented upgrade classes only) |

## 2. Historical evidence reachability

| Item | Status |
|---|---|
| T08 archive @ `6fb8e438` | reachable via `git show 6fb8e438:docs/verification/t08/6fb8e438…/` (deleted from tree at 18f34abb) |
| T08 archive @ `9c4fe175` | reachable via `git show 4762fb44:docs/verification/t08/9c4fe175…/` |
| First failed live run (129/170 @ 6fb8e438 era) | not archived; superseded by the recorded fix trail in EXPECTED-UPGRADES + commit messages — noted as a genuine missing original |
| T02 last accepted run | `docs/verification/t07/aba44783…/regressions/T02/artifact.json` (10/10) |
| Old-task artifacts at earlier SHAs | `docs/verification/t07/aba44783…/regressions/` |

## 3. Blockers and unexecuted items

1. **Frozen-oracle adjudication** — 11 cells across T03/T04/T05 manifest
   assertions encoded pre-T08 expectations against intentionally promoted
   semantics. At this closeout each cell was precisely attributed and left
   pending an owner decision: re-freeze old manifests at the new contract,
   or treat the cells as retired-by-upgrade. **Resolved:** the owner chose
   precise oracle migration (not retirement); the eleven cells were
   re-frozen at the T08 contract in the follow-up round — ledger in
   `docs/work-plans/t08-column-governance/EXPECTED-UPGRADES.md` §"Old-golden
   oracle migration".
2. **CI status** — no authoritative CI run state was established for
   `18f34abb`; not claimed anywhere in this evidence.
3. **Missing original** — the first live T08 golden run (129/170) was not
   archived before supersession; its failure trail exists only in commit
   history and the upgrade ledger.
4. **RC capture caveat** — the five regression runs captured runner stdout
   through `tail`; the runner rc recorded is derived from
   `validate_artifact` failure lists plus runner source semantics
   (`scripts/ddl_golden.py` returns 1 iff failures), not from a directly
   captured `$?`. See each `regressions/<T>/run.log`.

## 4. Role separation

Standards/Spec reviews recorded in the T08 trail are internal self-reviews,
not external reviewer acceptance. This closeout's verifier pass is
read-only and does not constitute milestone sign-off. `.mcp.json` and the
untracked tool/residue paths in the root worktree are external/user-owned
and were never staged, read for secrets, or committed.

## 5. Errata (corrected during T08-REGRESSION-ORACLE-CLOSURE)

This index text was corrected in the follow-up oracle-closure round; the
original run artifacts, `run.log` files, policy files, and binary-hash
records under `regressions/` are unchanged and remain the byte-exact
originals of the c983fc9d reruns. Corrections made:

- **Old verdict/coverage wording.** The frozen position expectations were
  described as `verdict=reject` / `coverage=unverified`; the real frozen
  cells bound `verdict=review` / `coverage=incomplete` (+1 diagnostic, +1
  unsupported entry; the T04/T05 modify cells also carried the
  `missing_source_column` evidence gap).
- **Case naming / SQL.** The T03 MySQL add cell was transcribed as
  `… AFTER b`; the frozen SQL is `ALTER TABLE t ADD COLUMN c INT FIRST;`.
  Each task's tidb add cell binds `… AFTER id;` (not "same" as the mysql
  cell's FIRST). There are no `*-change-column-position` cells among the
  drifted eleven.
- **Actual for T04/T05 modify cells.** Previously summarized as
  `complete`; the recorded actual is `review`/`unverified` with the
  `missing_source_column` gap preserved — the compat profile still cannot
  observe the provider-side source column offline.
- **Race wording.** The earlier paragraph presented ordinary `make test`
  coverage as race-detector coverage. No `-race` run was performed in this
  closeout; genuine race evidence was produced in the follow-up round and
  is indexed from `docs/verification/t08/<oracle-closure proof sha>/race/`.
