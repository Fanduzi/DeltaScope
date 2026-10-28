# T13-R3 gate index — product SHA 01525861

Every tuple stores `<name>.command.txt` (the real argv),
`<name>.stdout.txt`, `<name>.stderr.txt`, `<name>.rc.txt` (saved
immediately after the command exits). The original R3 runner printed
each name's rc to the console by reading `$?` AFTER an intervening
`echo`, so its console table showed `0` for every gate — the recorded
`.rc.txt` values were always correct and are the authoritative record;
the runner was corrected to save `rc` immediately (no `$?` reuse
across `echo`).

## Required gates (mapped by name from the R2 bundle set)

| Gate | Command actually run | rc | Result |
|------|----------------------|----|--------|
| go_test | `go test ./... -count=1` | 0 | whole repo, no cache |
| go_vet | `go vet ./...` | 0 | clean |
| gofmt | `gofmt -l ./cmd ./internal ./pkg` with an assertion that the list is exactly the two accepted T07 baseline files | 0 | only `batch_state_t07_test.go` + `batch_state_t07_matrix_test.go` listed |
| lint | `golangci-lint run ./internal/application/audit/... ./internal/domain/spec/... ./internal/domain/rule/ddl/... ./internal/infrastructure/metadata/mysql/...` (`lint-scoped.*`) | 1 | exactly the 13 accepted baseline issues — byte-identical to the R2 bundle's list (`lint-scoped-vs-r2.diff` is empty) |
| go_race | `env CGO_ENABLED=1 go test -race -count=1 ./internal/...` (`go_race.*`) | 0 | full internal tree |
| ddl-coverage-catalog-test | `make ddl-coverage-catalog-test` → `go test ./internal/application/audit -count=1 -tags postgresql -run TestDDLCoverageCatalog` (`ddl-coverage-catalog-test-rerun.*`) | 0 | test actually ran — see verbose record |
| ddl-coverage-catalog-test (verbose proof) | `go test ./internal/application/audit -count=1 -tags postgresql -run '^TestDDLCoverageCatalog$' -v` (`ddl-coverage-catalog-test-verbose.*`) | 0 | `--- PASS: TestDDLCoverageCatalog` |
| ddl-golden-validator-test | `make ddl-golden-validator-test` | 0 | pass |
| ddl-inventory-gate | `make ddl-inventory-gate` | 0 | pass |
| decision-record-gate | `make decision-record-gate` | 0 | pass |
| docs-example-gates | `make docs-example-gates` | 0 | pass |
| parser-error-unsupported-contract-test | `make parser-error-unsupported-contract-test` | 0 | pass |
| unsupported-diagnostics-evidence-test | `make unsupported-diagnostics-evidence-test` | 0 | pass |
| pg-confidence-gates | `make pg-confidence-gates` | 0 | pass |
| query-access-corpus-gates | `make query-access-corpus-gates` | 0 | pass |
| sql-corpus-gates | `make sql-corpus-gates` | 0 | pass |
| release-dialect-hygiene-gates | `make release-dialect-hygiene-gates` | 0 | pass |
| three-level-doc | `bash ~/.agents/skills/check-three-level-doc/scripts/check_three_level_doc.sh` | 0 | working tree clean — `no changed files` (L2 sync check; retired L3 headers are not re-checked) |
| three-level-doc-diff | `bash /tmp/t13r2_docdiff.sh` — every `internal/**.go` file changed in `1f10ea99..01525861` must have its package `README.md` changed in the same range | 0 | all 4 changed Go files SYNCED → `RESULT: PASS` |

**Required-set count: 18 named gates**, all green or matching their
accepted baseline. `go test` and `gofmt` ran slightly stricter
equivalents of the R2 commands (`-count=1`; count-asserted list) —
recorded verbatim.

## Miscalled records (kept, annotated, superseded)

- `ddl-coverage-catalog-test.*` — ran `go test ./internal/application/audit
  -run TestDDLCoverageCatalog -count=1` **without** `-tags postgresql`;
  the test is postgresql-tagged so stdout shows `[no tests to run]`.
  Superseded by the `make` rerun + `-v` record above. The same test was
  already executed inside `pg-unit-test-gates` — a record-scope
  correction, not a new product defect. See `…note.txt`.
- `go-race.*` — partial-scope race
  (`./internal/application/audit/ ./internal/domain/rule/ddl/`,
  two packages, no `CGO_ENABLED`) — kept as a focused diagnostic; the
  required-scope record is `go_race.*`. See `…note.txt`.
- `lint.*` — **full-repo diagnostic** `golangci-lint run ./...`:
  **37 issues** across all surfaces (the broader tree beyond the
  accepted four-package scope — older domain/interface/pkg files).
  Recorded verbatim; it is NOT the accepted gate scope and its count
  is not the baseline. The accepted-scope run is `lint-scoped.*`
  (13 issues, identical to the R2 baseline). See `…note.txt`.

## Extra diagnostics beyond the required set

- `go-build.*` — `go build ./...` (compile sanity, not an R2-named
  gate) — rc=0.
- `pg-unit-test-gates.*` — `make pg-unit-test-gates` — rc=0; includes
  `TestDDLCoverageCatalog` under the postgresql tag (the reason the
  miscall above is not a product defect).
- `go-race.*` (the two-package run) — counted as an extra diagnostic.
- `lint.*` (full-repo 37-issue run) — counted as an extra diagnostic.
