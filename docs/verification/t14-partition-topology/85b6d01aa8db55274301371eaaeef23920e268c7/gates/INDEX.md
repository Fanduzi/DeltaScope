# T14-R2-tail gate index — product SHA 85b6d01a

Every tuple stores `<name>.command.txt` (the real argv),
`<name>.stdout.txt`, `<name>.stderr.txt`, `<name>.rc.txt` (saved
immediately after the command exits). All gates ran on the final
product commit `85b6d01aa8db55274301371eaaeef23920e268c7`
(`fix(audit): single-member undecidable bound premise in
declaredSchemeOutcome — T14-R2-tail (#93)`), whose parent is the
T14-R2 evidence commit `f95dc949…`.

## Required gates

| Gate | Command actually run | rc | Result |
|------|----------------------|----|--------|
| go_test | `go test ./... -count=1` | 0 | whole repo, no cache — includes `batch_state_t14r2tail_test.go`, none skipped |
| go_vet | `go vet ./...` | 0 | clean |
| gofmt | `gofmt -l ./cmd ./internal ./pkg` | 0 | only the two accepted T07 baseline files listed — no T14-R2-tail file listed |
| lint-scoped | `golangci-lint run ./internal/application/audit/... ./internal/domain/spec/... ./internal/domain/rule/ddl/... ./internal/infrastructure/metadata/mysql/...` | 1 | exactly the 13 accepted baseline issues — stdout byte-identical to the ce9a73dc R2 record (verified by diff); zero new findings |
| go_race | `env CGO_ENABLED=1 go test -race -count=1 ./internal/...` | 0 | full internal tree |
| ddl-coverage-catalog-test | `make ddl-coverage-catalog-test` | 0 | postgresql-tagged test via the canonical make entry |
| ddl-coverage-catalog-test-verbose | `go test ./internal/application/audit -count=1 -tags postgresql -run '^TestDDLCoverageCatalog$' -v` | 0 | `--- PASS: TestDDLCoverageCatalog` — non-zero execution proven |
| ddl-golden-validator-test | `make ddl-golden-validator-test` | 0 | pass — includes the new tail manifest cell |
| ddl-inventory-gate | `make ddl-inventory-gate` | 0 | pass |
| decision-record-gate | `make decision-record-gate` | 0 | pass — R2 ADR updated with the tail decision note |
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
| l2-committed-t14r2tail | `ce9a73dc..85b6d01a` (tail product + doc corrections, 641 files) | 0 | clean |
| l2-committed-t14-cumulative | `b54a0544..85b6d01a` (cumulative T14 incl. evidence commits, 1913 files) | 0 | clean |
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
`head=85b6d01aa8db` and share one CLI sha256 (`6bf916bfc578…`).
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

T09–T13 inputs/expectations/denominators are frozen and unchanged.
T14 keeps all 68 previously-required case IDs (including the two R2
ownership-corrected cells) and adds
`t14r2tail-mysql80-pby-single-expr-member` — the single-member
`PARTITION BY RANGE ... (1+0)` → `TRUNCATE PARTITION fresh` regression
pinning unproven-not-combinable on a live anchor; assertions rose
1066 → 1087. Each artifact's required/executed case sets are equal, no
duplicate IDs, `compose_down_rc=0`, `residual_containers=[]`.
