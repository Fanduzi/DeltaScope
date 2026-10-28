# T14-R2-tail2 gate index — product SHA 2119f230

Every tuple stores `<name>.command.txt` (the real argv),
`<name>.stdout.txt`, `<name>.stderr.txt`, `<name>.rc.txt` (saved
immediately after the command exits). All gates ran on the final
product commit `2119f2305b92ea10b66f8c7c3cf79f714c198b8b`
(`fix(audit): MAXVALUE-then-undecidable pair refusal in
declaredSchemeOutcome — T14-R2-tail2 (#93)`), whose parent is the
T14-R2-tail evidence commit `50393d40…`.

## Required gates

| Gate | Command actually run | rc | Result |
|------|----------------------|----|--------|
| go_test | `go test ./... -count=1` | 0 | whole repo, no cache — includes `batch_state_t14r2tail2_test.go`, none skipped |
| go_vet | `go vet ./...` | 0 | clean |
| gofmt | `gofmt -l ./cmd ./internal ./pkg` | 0 | only the two accepted T07 baseline files listed — no T14-R2-tail2 file listed |
| lint-scoped | `golangci-lint run ./internal/application/audit/... ./internal/domain/spec/... ./internal/domain/rule/ddl/... ./internal/infrastructure/metadata/mysql/...` | 1 | exactly the 13 accepted baseline issues — stdout byte-identical to the ce9a73dc R2 record (verified by diff); zero new findings |
| go_race | `env CGO_ENABLED=1 go test -race -count=1 ./internal/...` | 0 | full internal tree |
| ddl-coverage-catalog-test | `make ddl-coverage-catalog-test` | 0 | postgresql-tagged test via the canonical make entry |
| ddl-coverage-catalog-test-verbose | `go test ./internal/application/audit -count=1 -tags postgresql -run '^TestDDLCoverageCatalog$' -v` | 0 | `--- PASS: TestDDLCoverageCatalog` — non-zero execution proven |
| ddl-golden-validator-test | `make ddl-golden-validator-test` | 0 | pass |
| ddl-inventory-gate | `make ddl-inventory-gate` | 0 | pass |
| decision-record-gate | `make decision-record-gate` | 0 | pass — R2 ADR updated with the tail2 control-flow note |
| docs-example-gates | `make docs-example-gates` | 0 | pass |
| parser-error-unsupported-contract-test | `make parser-error-unsupported-contract-test` | 0 | pass |
| unsupported-diagnostics-evidence-test | `make unsupported-diagnostics-evidence-test` | 0 | pass |
| pg-confidence-gates | `make pg-confidence-gates` | 0 | pass |
| pg-unit-test-gates | `make pg-unit-test-gates` | 0 | pass |
| query-access-corpus-gates | `make query-access-corpus-gates` | 0 | pass |
| sql-corpus-gates | `make sql-corpus-gates` | 0 | pass |
| release-dialect-hygiene-gates | `make release-dialect-hygiene-gates` | 0 | pass |
| three-level-doc | `bash /Users/fan/.agents/skills/check-three-level-doc/scripts/check_three_level_doc.sh` | 0 | L2 README sync over the working tree (ran green pre-evidence-commit; the post-commit clean-tree record is `l2-final-tree`) |

**Required-set count: 19 named gates**, all green or matching their
accepted baseline.

## Committed-diff L2 checks

`l2_committed_range_check.sh` is archived verbatim (unchanged from the
R2 bundle; same L2 semantics, diff source `git diff --name-only
<range>`):

| Check | Range | rc | Result |
|-------|-------|----|--------|
| l2-committed-t14r2tail2 | `85b6d01a..2119f230` (tail2 product + doc corrections, 645 files) | 0 | clean |
| l2-committed-t14-cumulative | `b54a0544..2119f230` (cumulative T14 incl. evidence commits, 2553 files) | 0 | clean |
| l2-final-tree | worktree check on the final tree | 0 | `[three-level-doc] no changed files` — honest clean-tree record, not a substituted pass |

## Extra diagnostic beyond the required set

| Gate | Command actually run | rc | Result |
|------|----------------------|----|--------|
| lint-fullrepo | `golangci-lint run ./...` | 1 | 37 diagnostics across the broader tree — identical count to the accepted R2 record; NOT the accepted gate scope |

## Six-suite golden (serial, same product SHA, same CLI fingerprint)

Each suite ran
`python3 ./scripts/ddl_golden.py run --task <T> --artifact-dir <bundle>/golden`;
the runner builds its own CLI from the checkout and records `head_sha`
plus binary sha256 in `artifact.json`. All six report
`head=2119f2305b92` and share one CLI sha256 (`d5d6b33e4f53…`).
Credential env vars were set per each manifest's
`password_env`/`exec_password_env` contract (`T09_MYSQL_PASSWORD`,
`T10_MYSQL_PASSWORD`/`DS_T10_GOLDEN_PW`, `T11_*`, `T12_*`, `T13_*`,
`T14_MYSQL_PASSWORD`/`DS_T14_GOLDEN_PW` — all `root`); no secret is
stored in the artifacts.

| Task | cases | assertions | result |
|------|-------|------------|--------|
| T09 | 125/125 | 3442 | PASS |
| T10 | 126/126 | 3175 | PASS |
| T11 | 40/40 | 506 | PASS |
| T12 | 25/25 | 282 | PASS |
| T13 | 47/47 | 697 | PASS |
| T14 | 69/69 | 1087 | PASS |

T09–T13 denominators frozen; T14 keeps all 69 required IDs unchanged —
the tail2 regression is pinned by the public test matrix, no golden
cell was added or weakened. Each artifact's required/executed case
sets are equal, no duplicate IDs, `compose_down_rc=0`,
`residual_containers=[]`.
