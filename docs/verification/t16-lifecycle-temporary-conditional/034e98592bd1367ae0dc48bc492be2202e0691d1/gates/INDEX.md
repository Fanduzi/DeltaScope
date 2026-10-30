# T16-R1 gates index — evidence bundle @034e9859

Gate tuples (`.command.txt` / `.rc.txt` / `.stdout.txt` / `.stderr.txt`)
recorded on the milestone branch at `034e98592bd1367ae0dc48bc492be2202e0691d1`
(the R1 product commit itself — golden artifact's `head_sha` and
`cli.build.head_sha` both equal it). Every tuple is an exact command +
return code + captured stdout/stderr — nothing was retyped.

| Gate | Command | rc |
|---|---|---|
| go-test | `go test ./... -count=1` | 0 |
| go-vet | `go vet ./...` | 0 |
| go-race | `env CGO_ENABLED=1 go test -race -count=1 ./internal/...` | 0 |
| sql-corpus-gates | `make sql-corpus-gates` | 0 |
| ddl-inventory-gate | `make ddl-inventory-gate` | 0 |
| ddl-golden-validator-test | `python3 scripts/test_ddl_golden.py` | 0 |
| ddl-coverage-catalog-test | `make ddl-coverage-catalog-test` | 0 |
| parser-error-unsupported-contract-test | `make parser-error-unsupported-contract-test` | 0 |
| unsupported-diagnostics-evidence-test | `make unsupported-diagnostics-evidence-test` | 0 |
| decision-record-gate | `make decision-record-gate` | 0 |
| docs-example-gates | `make docs-example-gates` | 0 |
| pg-unit-test-gates | `make pg-unit-test-gates` | 0 |
| pg-confidence-gates | `make pg-confidence-gates` | 0 |
| three-level-doc (worktree) | `bash /Users/fan/.agents/skills/check-three-level-doc/scripts/check_three_level_doc.sh` | 0 |
| l2-committed-range | `bash …/gates/l2_committed_range_check.sh 882f5171..034e9859` | 0 |
| lint-scoped | `golangci-lint run ./internal/application/audit/... ./internal/domain/spec/... ./internal/domain/rule/ddl/... ./internal/infrastructure/metadata/mysql/...` | 1 — 15 findings, all pre-existing T16 baseline (identical signatures existed at `7985c774`); R1 adds zero |
| lint-fullrepo | `golangci-lint run ./...` | 1 — 39 findings, all baseline; none in R1-touched files beyond the three T16 `unparam`/`unused` items |

The committed-range L2 check covers the actual R1 diff (16 files across
`882f5171..034e9859`, including every touched module README):
`[three-level-doc] OK`. The worktree check also reports an L1 reminder —
module internals changed only; no user-facing surface (rule IDs, flags,
output shape) moved, so the root README needs no update.

## Golden runs

| Suite | Command | Result |
|---|---|---|
| T16 | `T16_MYSQL_PASSWORD=root python3 ./scripts/ddl_golden.py run --task T16 --artifact-dir artifacts/t16r1` | 60/60 cases, 832 assertions, PASS — `golden/T16-run.log`, artifact `golden/T16.artifact.json` (`head_sha == cli.build.head_sha == 034e9859`) |

Denominator note: the original 55/739 artifact at
`../7985c774f8c83e5a93facb4106923a5581495062/golden/T16.artifact.json` is
preserved untouched. This run executes the five new R1 cases plus the two
re-expected F4 cells (`truncate-suite`, `drop-duplicate` provider-loaded
TRUNCATE now owing `target_table.inbound_fk`).

Anchor-image note: `mysql:8.0.46` was tagged locally from the pre-existing
`mysql:8.0` image after `mysqld --version` verified the server is exactly
8.0.46 — the pinned tag name differs but the server bytes are the same
patch release. `mysql:5.7.44` (`--platform linux/amd64`), `mysql:8.4.10`,
and `pingcap/tidb:v8.5.0` were pulled fresh because a `docker-credential-
desktop` helper hang had stalled the default `docker pull` path; the
pulls used an isolated `DOCKER_CONFIG` with no credential store.
