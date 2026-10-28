# T14 gate index — product SHA b5b2ac70

Every tuple stores `<name>.command.txt` (the real argv),
`<name>.stdout.txt`, `<name>.stderr.txt`, `<name>.rc.txt` (saved
immediately after the command exits — no `$?` reuse across `echo`).
All gates ran on the final product commit
`b5b2ac7007121c9319e1ab55f568b462a49f9af3`
(`feat(audit): partition topology and maintenance semantics (T14/#93)`),
after the post-implementation lint cleanup was folded in.

## Required gates

| Gate | Command actually run | rc | Result |
|------|----------------------|----|--------|
| go_test | `go test ./... -count=1` | 0 | whole repo, no cache |
| go_vet | `go vet ./...` | 0 | clean |
| gofmt | `gofmt -l ./cmd ./internal ./pkg` | 0 | only the two accepted T07 baseline files listed (`batch_state_t07_matrix_test.go`, `batch_state_t07_test.go`) — no T14 file listed |
| lint-scoped | `golangci-lint run ./internal/application/audit/... ./internal/domain/spec/... ./internal/domain/rule/ddl/... ./internal/infrastructure/metadata/mysql/...` | 1 | exactly the 13 accepted baseline issues — stdout is identical to the T13-R3 bundle's `lint-scoped.stdout.txt` modulo line numbers (verified by normalized diff) |
| go_race | `env CGO_ENABLED=1 go test -race -count=1 ./internal/...` | 0 | full internal tree |
| ddl-coverage-catalog-test | `make ddl-coverage-catalog-test` | 0 | postgresql-tagged test executed via the canonical make entry |
| ddl-coverage-catalog-test-verbose | `go test ./internal/application/audit -count=1 -tags postgresql -run '^TestDDLCoverageCatalog$' -v` | 0 | `--- PASS: TestDDLCoverageCatalog` — proves non-zero execution |
| ddl-golden-validator-test | `make ddl-golden-validator-test` | 0 | pass |
| ddl-inventory-gate | `make ddl-inventory-gate` | 0 | pass |
| decision-record-gate | `make decision-record-gate` | 0 | pass |
| docs-example-gates | `make docs-example-gates` | 0 | pass |
| parser-error-unsupported-contract-test | `make parser-error-unsupported-contract-test` | 0 | pass |
| unsupported-diagnostics-evidence-test | `make unsupported-diagnostics-evidence-test` | 0 | pass |
| pg-confidence-gates | `make pg-confidence-gates` | 0 | pass |
| pg-unit-test-gates | `make pg-unit-test-gates` | 0 | pass |
| query-access-corpus-gates | `make query-access-corpus-gates` | 0 | pass |
| sql-corpus-gates | `make sql-corpus-gates` | 0 | pass |
| release-dialect-hygiene-gates | `make release-dialect-hygiene-gates` | 0 | pass |
| three-level-doc | `bash ~/.agents/skills/check-three-level-doc/scripts/check_three_level_doc.sh` | 0 | L2 README sync check; retired L3 headers are not re-checked |

**Required-set count: 19 named gates**, all green or matching their
accepted baseline. The set maps one-to-one onto the T13-R3 required
list plus `pg-unit-test-gates`, which T13-R3 archived under the same
name.

## Extra diagnostic beyond the required set

| Gate | Command actually run | rc | Result |
|------|----------------------|----|--------|
| lint-fullrepo | `golangci-lint run ./...` | 1 | 37 issues across the broader tree — identical count to the T13-R3 full-repo diagnostic; NOT the accepted gate scope and its count is not the baseline |

## Six-suite golden (serial, same product SHA, same CLI fingerprint)

Each suite ran `python3 ./scripts/ddl_golden.py run --task <T>`; the
runner builds its own CLI from the checkout
(`CGO_ENABLED=0 go build -o <artifact>/deltascope ./cmd/deltascope`)
and records `head_sha` + binary sha256 in `artifact.json`. All six
report `head=b5b2ac700712` — the same product SHA these gates ran on.
Artifacts are archived verbatim under `../golden/<T>/<T>/`.

| Task | cases | assertions | result |
|------|-------|------------|--------|
| T09 | 125/125 | 3442 | PASS |
| T10 | 126/126 | 3175 | PASS |
| T11 | 40/40 | 506 | PASS |
| T12 | 25/25 | 282 | PASS |
| T13 | 47/47 | 697 | PASS |
| T14 | 59/59 | 917 | PASS |

The required/executed case sets are equal inside each artifact
(`required_case_ids` vs executed cases); no zero-case suite counts
toward the result. Golden container cleanup is recorded inside each
`artifact.json` under `cleanup`.
