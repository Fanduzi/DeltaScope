# T17-R1 gate receipts

Product: `83125653d7cfc4343221d6f47e89df4311101efe`.
Parent/evidence baseline: `175ab0e15db942d7b74f252d2359c766783b606e`.
This is an independent-review candidate, not an accepted T17 delivery.

Each stem has actual `.command.txt`, `.stdout.txt`, `.stderr.txt`, and
`.rc.txt` files. Empty stdout/stderr files are retained. No expected result
is substituted for a command's actual exit status.

| Final receipt stem | Command | rc / outcome |
|---|---|---|
| go-test | `go test ./... -count=1` | 0 |
| go-vet | `go vet ./...` | 0 |
| go-race | `env CGO_ENABLED=1 go test -race ./internal/...` | 0 |
| pg-unit-test-gates | `make pg-unit-test-gates` | 0 |
| pg-confidence-gates | `make pg-confidence-gates` | 0, tagged unit + Docker CLI/HTTP/MCP |
| sql-corpus-gates | `make sql-corpus-gates` | 0, actual corpus selectors executed |
| ddl-inventory-gate | `make ddl-inventory-gate` | 0, row statuses/counts unchanged |
| ddl-coverage-catalog-test | `make ddl-coverage-catalog-test` | 0 |
| ddl-golden-validator-test | `make ddl-golden-validator-test` | 0, includes the Python contract suites |
| parser-error-unsupported-contract-test | `make parser-error-unsupported-contract-test` | 0 |
| unsupported-diagnostics-evidence-test | `make unsupported-diagnostics-evidence-test` | 0 |
| docs-example-gates | `make docs-example-gates` | 0 |
| decision-record-gate | `make decision-record-gate` | 0, precommit receipt; final product-head receipt also retained |
| three-level-doc | requested shared L2 worktree checker | 0; no L3 file headers added |
| lint-scoped | eight original T17 package trees, exact command in tuple | 1, 22 inherited diagnostics |
| lint-fullrepo | `golangci-lint run ./...` | 1, 39 inherited diagnostics |
| lint-compare | lead-authored diagnostic-identity comparison | 0, no added/removed identities |
| ddl-golden | canonical `make ddl-golden TASK=T17 ARTIFACT_DIR=/tmp/ddl-golden` | 0, precommit run at 175ab0e1 + product changes |
| ddl-golden-final | same canonical command at product HEAD | 0, 52/52 and 834 assertions, plus projection/error Go proofs |

Final product-head decision-record and committed-range L2 receipts are
separate files rather than overwrites of the earlier worktree receipts.
Their filenames and raw commands are included in the bundle's gate
directory.

## Provenance

The untagged full Go tests, vet, and race ran against the corrected
production source before the product commit. The only subsequent code
change was the PostgreSQL-tagged regression-contract amendment; it is
covered by the fresh PG unit and confidence gates. Both lint commands use
the repository's PostgreSQL build tag and were rerun after that amendment.
The final real Golden CLI build and artifact bind both head fields to the
product SHA. Source/evidence are committed separately.

The original PG-tagged test explicitly expected swallowed resolver errors
and panicked while indexing the now-empty enrichment result. Its actual
failure is preserved in `pg-unit-test-gates-first.*`, not hidden. The
replacement verifies the decided shared B3 error contract through enrich
and public AuditSQL with sentinel/cancel/deadline cases. Genuine
Unavailable-without-error controls remain unchanged.

Earlier `*-first.*`, `*-before-*`, `*-pre-pg-change.*`, and phase1 receipts
are intermediate attempts, not final acceptance gates. The detached
`adc1f732` red receipts intentionally fail from behavioral assertions.

## Lint and formatting

The two nonzero lint commands are inherited baselines, not green lint
claims. `lint-compare.stdout.txt` compares multiplicities of complete
file/message/linter identities with the original T17 receipts:
22 scoped unchanged, 39 full unchanged, zero added and zero removed.
Shifted line/column numbers are not treated as new issue identities.

`gofmt-scoped*` checks only this round's changed Go files; output is empty.
The separate T07 receipts record the pre-existing formatting/blob baseline
without formatting those files. Repository guidance does not require the
retired L3 input/output/pos/note headers; the requested L2 check does not
reintroduce them.

## Resources

`git-*`, `docker-*`, preservation-hash manifests, and cleanup receipts
record the exact refs, worktree status, and task resource lifecycle.
The original prior Golden artifact and the preliminary R1 artifact were
preserved before the final required-path run. Cleanup is scoped to the
test fixtures and the task-created detached red worktree.
