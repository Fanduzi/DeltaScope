# T17 gates index — evidence bundle @adc1f732

Gate tuples (`.command.txt` / `.rc.txt` / `.stdout.txt` / `.stderr.txt`)
recorded on the milestone branch at `adc1f7322e1c70524435ed687fe27b67b931abb5`
(the T17 product commit itself — the golden artifact's `head_sha` and
`cli.build.head_sha` both equal it). Every tuple is an exact command +
return code + captured stdout/stderr — nothing was retyped.

| Gate | Command | rc |
|---|---|---|
| go-test | `go test ./... -count=1` | 0 |
| go-vet | `go vet ./...` | 0 |
| go-race | `env CGO_ENABLED=1 go test -race -count=1 ./internal/...` | 0 |
| sql-corpus-gates | `make sql-corpus-gates` | 0 |
| ddl-inventory-gate | `make ddl-inventory-gate` | 0 |
| ddl-golden-validator-test | `python3 scripts/test_ddl_golden.py` | 0 — contract cases=758 failures=0 |
| ddl-coverage-catalog-test | `make ddl-coverage-catalog-test` | 0 |
| parser-error-unsupported-contract-test | `make parser-error-unsupported-contract-test` | 0 |
| unsupported-diagnostics-evidence-test | `make unsupported-diagnostics-evidence-test` | 0 |
| decision-record-gate | `make decision-record-gate` | 0 |
| docs-example-gates | `make docs-example-gates` | 0 |
| pg-unit-test-gates | `make pg-unit-test-gates` | 0 |
| pg-confidence-gates | `make pg-confidence-gates` | 0 |
| three-level-doc (worktree) | `bash /Users/fan/.agents/skills/check-three-level-doc/scripts/check_three_level_doc.sh` | 0 |
| l2-committed-range | `bash …/gates/l2_committed_range_check.sh 8784346d..adc1f732` | 0 — 42 changed files, all module READMEs in sync |
| lint-scoped | `golangci-lint run` over the eight T17-touched package trees | 1 — 22 findings, all pre-existing baseline (T07/T16 files and untouched helpers); the single T17-file item (`batch_state_schema.go` QF1002) was fixed in-flight, so the committed tree adds **zero** new findings |
| lint-fullrepo | `golangci-lint run ./...` | 1 — 39 findings, identical to the T16 baseline count; none in T17-touched code |

The committed-range L2 check covers the actual T17 diff (42 files across
`8784346d..adc1f732`, including every touched module README):
`[three-level-doc] OK`. The worktree check also reports an L1 reminder —
module internals changed only; the user-facing surface additions (the six
rule IDs) are documented in `configs/deltascope.example.yaml` and
`docs/reference/audit-capability-matrix.md`, which ship in the same commit.

## Golden runs

| Suite | Command | Result |
|---|---|---|
| T17 | `T17_MYSQL_PASSWORD=root python3 ./scripts/ddl_golden.py run --task T17 --artifact-dir /tmp/ddl-golden-t17-final` | 47/47 cases, 721 assertions, PASS — `golden/T17-run.log`, artifact `golden/T17.artifact.json` (`head_sha == cli.build.head_sha == adc1f732`, required == executed, `compose_down_rc=0`, zero residual containers) |

Anchor images are the same pinned fixtures the T16 run used
(`mysql:5.7.44` linux/amd64, `mysql:8.0.46`, `mysql:8.4.10`,
`pingcap/tidb:v8.5.0`); per-case image digests are recorded inside the
artifact. The MySQL 5.7 fixture was rebuilt after a probe proved
`DROP DATABASE mysql` succeeds there (native fact, now encoded in the
committed probe script as a skip-repeat).
