# T05-A2 verification evidence — 6e06062

Evidence-only commit for the T05-A2 single-pair RENAME identity-migration
slice. Code head under review: `6e060624ecdff3133e8e2c98c484f10e21efcb0b` on
`origin/workfree` (fast-forwarded from the A1 evidence head `624b126`).

## Layout

| Path | Content |
|---|---|
| `artifact.json` | T05 golden artifact produced by the post-commit run (`head_sha=6e06062`) |
| `cases/` | All 167 per-case raw results (command, rc, stdout, stderr, metadata phases) |
| `policies/` | Effective golden policy YAMLs used by the run (sha-bound inside artifact) |
| `gates/` | Gate logs re-run at `6e06062` unless noted below |
| `red/` | A2 red→green evidence (see "Red evidence" below) |
| `identity/source-identity.txt` | Code/review base SHAs, worktree status, diff scope, manifest+script digests, toolchain |

## Gate results at 6e06062

| Gate | Command | Result | Log |
|---|---|---|---|
| T05 golden | `make ddl-golden TASK=T05` (runner `scripts/ddl_golden.py`) | 167/167 cases, 2396 assertions, PASS | `gates/golden-run.log` |
| Artifact validator | `ddl_golden.py validate --artifact …` | artifact valid | `gates/validate.log` |
| Validator contract | `python3 scripts/test_ddl_golden.py` (via `make ddl-golden-validator-test`) | 98 cases, 0 failures | `gates/go-and-pg.log` |
| Unit tests | `go test ./...` | all packages pass | `gates/go-test-all.log` |
| PG-tagged audit tests | `go test -tags postgresql ./internal/application/audit` | ok | `gates/go-and-pg.log` |
| PG gates | `make pg-unit-test-gates` | pass | `gates/pg-unit-test-gates.log` |
| Corpus gates | `make sql-corpus-gates` | pass | `gates/sql-corpus-gates.log` |
| Decision record | `make decision-record-gate` | pass | `gates/decision-record-gate.log` |
| Three-level doc | skill `check-three-level-doc` script, diff replayed in a detached worktree at `624b126` (the checker inspects worktree diffs; on a clean tree it is a no-op) | OK (L1 reminder only; root README unchanged — module README edits are internal-contract wording) | `gates/three-level-doc.log` |
| Golden regressions | `make ddl-golden TASK=T02/T03/T04` | T02 10/10, T03 118/118, T04 163/163 — pre-commit run at identical tree content | `gates/golden-T02.log`, `golden-T03.log`, `golden-T04.log` |
| gofmt | `gofmt -l internal/ pkg/ cmd/` | clean (empty output) | `gates/go-and-pg.log` |

## Rename-path oracle evidence (A2)

Twelve new `metadata_cases` — four anchors × three forms — live in `cases/`:

- `T05.meta.t05-<anchor>-rename-qualified` — `RENAME TABLE src.t TO dst.t2`
- `T05.meta.t05-<anchor>-rename-alter` — `ALTER TABLE src.t RENAME TO dst.t2`
- `T05.meta.t05-<anchor>-rename-unqualified` — `RENAME TABLE src.t TO t2`
  (unqualified destination)

Each carries: schema setup → endpoint-absence verify → product audit
(4 statements, all `complete`, verdict `pass`) → post_verify (product did not
mutate anything; unqualified cases additionally pin `SELECT DATABASE()` =
`golden` via `use_database`) → driver-applied statements → structure oracle
(`src.t` gone, `dst.t2`/`golden.t2` owns `id` PK + `c` + `idx_c`; unqualified
cases additionally prove `src.t2` absent) → teardown + residual checks.

## Red evidence

`red/a2-tests-red.log` was produced by checking out the pre-fix commit
`624b126` in a detached worktree and running `go test -run TestBatchStateA2`
with the new test file copied in — genuine failures (post-rename statements
still report `unknown_table_state` gaps, provider ledger missing the
destination read, destination provider error not propagated).
`red/a2-tests-green.log` is the same run at the code head.

## Honesty notes

- `gates/golden-T02/T03/T04.log` ran against the pre-commit worktree; the tree
  content was identical to `6e06062` (the commit contains exactly that diff).
- The binary sha256 inside `artifact.json` (`cli.sha256`) is the runner's own
  record of the binary it built and executed; it was not independently
  recomputed by the reviewer of this evidence.
- No credential values appear in this evidence; `DS_T05_GOLDEN_PW` is an
  environment variable name, and the recorded command argv uses it as such.
