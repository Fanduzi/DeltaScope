# T14-R1 gate index — product SHA c29ebc38

Every tuple stores `<name>.command.txt` (the real argv),
`<name>.stdout.txt`, `<name>.stderr.txt`, `<name>.rc.txt` (saved
immediately after the command exits — no `$?` reuse across `echo`).
All gates ran on the final product commit
`c29ebc38dc5fd289ca92f23c13f4d925250449df`
(`feat(audit): partition R1 remediation — count fidelity, declaration
validity, bound domains (T14-R1/#93)`), whose parent is the accepted
T14 evidence commit `14850fb23eb5…`.

## Required gates

| Gate | Command actually run | rc | Result |
|------|----------------------|----|--------|
| go_test | `go test ./... -count=1` | 0 | whole repo, no cache — includes the 79-subtest `batch_state_t14r1_test.go` matrix, none skipped |
| go_vet | `go vet ./...` | 0 | clean |
| gofmt | `gofmt -l ./cmd ./internal ./pkg` | 0 | only the two accepted T07 baseline files listed (`batch_state_t07_matrix_test.go`, `batch_state_t07_test.go`) — no T14-R1 file listed |
| lint-scoped | `golangci-lint run ./internal/application/audit/... ./internal/domain/spec/... ./internal/domain/rule/ddl/... ./internal/infrastructure/metadata/mysql/...` | 1 | exactly the 13 accepted baseline issues — same file set as the b5b2ac70 bundle (verified by file-list diff); no R1-introduced finding |
| go_race | `env CGO_ENABLED=1 go test -race -count=1 ./internal/...` | 0 | full internal tree |
| ddl-coverage-catalog-test | `make ddl-coverage-catalog-test` | 0 | postgresql-tagged test executed via the canonical make entry |
| ddl-coverage-catalog-test-verbose | `go test ./internal/application/audit -count=1 -tags postgresql -run '^TestDDLCoverageCatalog$' -v` | 0 | `--- PASS: TestDDLCoverageCatalog` — proves non-zero execution |
| ddl-golden-validator-test | `make ddl-golden-validator-test` | 0 | pass |
| ddl-inventory-gate | `make ddl-inventory-gate` | 0 | pass — includes the five resynced owner:T14 rows |
| decision-record-gate | `make decision-record-gate` | 0 | pass — `2026-10-28-ddl-partition-topology-r1-remediation.md` present in the diff |
| docs-example-gates | `make docs-example-gates` | 0 | pass |
| parser-error-unsupported-contract-test | `make parser-error-unsupported-contract-test` | 0 | pass — the R1 parser-boundary cells (1504/1492) stay diagnostics |
| unsupported-diagnostics-evidence-test | `make unsupported-diagnostics-evidence-test` | 0 | pass |
| pg-confidence-gates | `make pg-confidence-gates` | 0 | pass |
| pg-unit-test-gates | `make pg-unit-test-gates` | 0 | pass |
| query-access-corpus-gates | `make query-access-corpus-gates` | 0 | pass |
| sql-corpus-gates | `make sql-corpus-gates` | 0 | pass |
| release-dialect-hygiene-gates | `make release-dialect-hygiene-gates` | 0 | pass |
| three-level-doc | `bash ~/.agents/skills/check-three-level-doc/scripts/check_three_level_doc.sh` | 0 | L2 README sync check over the working-tree diff; retired L3 headers are not re-checked |

**Required-set count: 19 named gates**, all green or matching their
accepted baseline — the same set the b5b2ac70 bundle ran.

## Committed-diff L2 checks (E5 — new this round)

The stock three-level-doc script only sees the working-tree diff, so
the committed ranges were replayed through the same semantics (every
changed source file's directory must ship a README.md that is itself
in the changed set):

| Check | Range | rc | Result |
|-------|-------|----|--------|
| l2-committed-b5b2ac70 | `b54a0544…b5b2ac70` (original T14 product, 38 files) | 1 | two findings recorded: `probes/README.md` missing and `internal/interfaces/mcp/README.md` not synced with `rule_tools.go` — both closed this round (README added in this evidence commit; mcp README synced inside c29ebc38) |
| l2-committed-c29ebc38 | `14850fb2…c29ebc38` (R1 product, 20 files) | 1 | one finding recorded: `probes/README.md` — added alongside this bundle; re-run semantics now clean |

## Extra diagnostic beyond the required set

| Gate | Command actually run | rc | Result |
|------|----------------------|----|--------|
| lint-fullrepo | `golangci-lint run ./...` | 1 | 37 issues across the broader tree — identical count to the accepted diagnostic; NOT the accepted gate scope |

## Six-suite golden (serial, same product SHA, same CLI fingerprint)

Each suite ran `python3 ./scripts/ddl_golden.py run --task <T>
--artifact-dir ../golden/<T>`; the runner builds its own CLI from the
checkout (`CGO_ENABLED=0 go build -o <artifact>/deltascope
./cmd/deltascope`) and records `head_sha` + binary sha256 in
`artifact.json`. All six report `head=c29ebc38dc5f`. Credential env
vars were set per each manifest's `password_env`/`exec_password_env`
contract (`T09_MYSQL_PASSWORD`, `DS_T1*_GOLDEN_PW`, `T14_MYSQL_PASSWORD`
et al. — all `root`); no secret is stored in the artifacts.

| Task | cases | assertions | result |
|------|-------|------------|--------|
| T09 | 125/125 | 3442 | PASS |
| T10 | 126/126 | 3175 | PASS |
| T11 | 40/40 | 506 | PASS |
| T12 | 25/25 | 282 | PASS |
| T13 | 47/47 | 697 | PASS |
| T14 | 68/68 | 1065 | PASS |

T09–T13 inputs/expectations/denominators are frozen and unchanged;
T14 grew 59 → 68 required cases (the nine R1 cells). The
required/executed case sets are equal inside each artifact; cleanup is
recorded per artifact.
