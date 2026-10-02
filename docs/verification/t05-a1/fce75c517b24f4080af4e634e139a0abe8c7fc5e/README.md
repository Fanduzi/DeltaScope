# T05-A1-R2 verification evidence — fce75c5

Evidence-only commit for the T05-A1-R2 review. Code head under review:
`fce75c517b24f4080af4e634e139a0abe8c7fc5e` on `origin/workfree`
(fast-forwarded from R1 head `ef8cdade4a3d7ee96c46330aa38c20a0adfb79ee`).

## Layout

| Path | Content |
|---|---|
| `artifact.json` | T05 golden artifact produced by the post-commit run (`head_sha=fce75c5`) |
| `cases/` | All 155 per-case raw results (command, rc, stdout, stderr, metadata phases) |
| `policies/` | Effective golden policy YAMLs used by the run (sha-bound inside artifact) |
| `gates/` | Gate logs re-run at `fce75c5` unless noted below |
| `red/` | R2 red→green evidence (see "Red evidence" below) |
| `identity/source-identity.txt` | Code/review base SHAs, worktree status, diff scope, manifest+script digests, toolchain |

## Gate results at fce75c5

| Gate | Command | Result | Log |
|---|---|---|---|
| T05 golden | `make ddl-golden TASK=T05` (runner `scripts/ddl_golden.py`) | 155/155 cases, 1944 assertions, PASS | `gates/golden-run.log` |
| Artifact validator | `ddl_golden.py validate --task T05 --artifact …` | artifact valid | `gates/validate.log` |
| Validator contract | `python3 scripts/test_ddl_golden.py` | 88 cases, 0 failures | `gates/validator-contract.log` |
| Unit tests | `go test ./...` | all packages pass | `gates/go-test-all.log` |
| PG-tagged audit tests | `go test -tags postgresql ./internal/application/audit` | ok | `gates/pg-tag-audit.log` |
| PG gates | `make pg-unit-test-gates` | pass | `gates/pg-unit-test-gates.log` |
| Corpus gates | `make sql-corpus-gates` | pass | `gates/sql-corpus-gates.log` |
| Decision record | `make decision-record-gate` | pass | `gates/decision-record-gate.log` |
| Three-level doc | skill `check-three-level-doc` script | OK (L1 reminder only; root README unchanged — module README edits are internal-contract wording) | `gates/three-level-doc.log` |
| gofmt | `gofmt -l internal/ pkg/ cmd/` | clean (empty output) | `gates/gofmt.log` |

## First-path oracle evidence (G1/G2)

Per-anchor six-phase proofs live in `cases/`:

- `T05.meta.t05-mysql57-first-path.json`
- `T05.meta.t05-mysql80-first-path.json`
- `T05.meta.t05-mysql84-first-path.json`
- `T05.meta.t05-tidb85-first-path.json`

Each carries setup (table absent) → product audit → post_verify (product did not
execute) → execute (test driver applies the three audited statements) →
structure (`c` int, `idx_c(c)`, `PRIMARY(id)`) → teardown + residual check.

## Red evidence

`red/r2-member-gaps-red.log` and `red/r2-pg-gaps-red.log` were produced by
checking out the pre-fix commit `ef8cdad` in a detached worktree and running the
new R2 tests against it — genuine failures (`got []` gaps / PG gap leak), not
reconstructions. `red/green-*.log` are the same tests at `fce75c5` (all pass).

## Attribution notes

- `gates/three-level-doc.log` was produced by replaying the `ef8cdad..fce75c5`
  diff as working-tree changes in a detached worktree, because the checker only
  inspects uncommitted diffs and a clean tree is vacuous.
- The audit binary is not committed (31 MB); its identity is
  `cli.sha256 = f916a63dc87b503accfc479cea59d5cf48d9b7cd3de21c94b36752ea6a847a70`
  in `artifact.json`, built by the runner at `fce75c5` with `go1.26.1`, `CGO_ENABLED=0`.
- `vcs.modified` attribution from earlier rounds: the only untracked paths at
  evidence assembly time are listed in `identity/source-identity.txt`
  (environment directories and one pre-existing review HTML file).
