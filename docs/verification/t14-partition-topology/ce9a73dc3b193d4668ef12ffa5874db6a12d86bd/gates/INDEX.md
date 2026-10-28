# T14-R2 gate index — product SHA ce9a73dc

Every tuple stores `<name>.command.txt` (the real argv),
`<name>.stdout.txt`, `<name>.stderr.txt`, `<name>.rc.txt` (saved
immediately after the command exits). All gates ran on the final
product commit `ce9a73dc3b193d4668ef12ffa5874db6a12d86bd`
(`fix(audit): partition R2 remediation — ADD bound ownership,
undecidable disclosure, bounded state admission (T14-R2/#93)`), whose
parent is the T14-R1 evidence commit `17539dbbaddb…`.

## Required gates

| Gate | Command actually run | rc | Result |
|------|----------------------|----|--------|
| go_test | `go test ./... -count=1` | 0 | whole repo, no cache — includes the `batch_state_t14r2_test.go` matrix and the six ownership-corrected R1 expectations, none skipped |
| go_vet | `go vet ./...` | 0 | clean |
| gofmt | `gofmt -l ./cmd ./internal ./pkg` | 0 | only the two accepted T07 baseline files listed — no T14-R2 file listed |
| lint-scoped | `golangci-lint run ./internal/application/audit/... ./internal/domain/spec/... ./internal/domain/rule/ddl/... ./internal/infrastructure/metadata/mysql/...` | 1 | exactly the 13 accepted baseline issues — file-list diff against the R1 bundle record is identical (two self-introduced findings — `typeDefFirst`, unused `listBoundsShareValue` — were fixed before the final SHA; zero new findings) |
| go_race | `env CGO_ENABLED=1 go test -race -count=1 ./internal/...` | 0 | full internal tree |
| ddl-coverage-catalog-test | `make ddl-coverage-catalog-test` | 0 | postgresql-tagged test via the canonical make entry |
| ddl-coverage-catalog-test-verbose | `go test ./internal/application/audit -count=1 -tags postgresql -run '^TestDDLCoverageCatalog$' -v` | 0 | `--- PASS: TestDDLCoverageCatalog` — non-zero execution proven |
| ddl-golden-validator-test | `make ddl-golden-validator-test` | 0 | pass — includes the two R2 ownership-corrected manifest cells |
| ddl-inventory-gate | `make ddl-inventory-gate` | 0 | pass |
| decision-record-gate | `make decision-record-gate` | 0 | pass — `2026-10-28-ddl-partition-topology-r2-remediation.md` present in the diff |
| docs-example-gates | `make docs-example-gates` | 0 | pass |
| parser-error-unsupported-contract-test | `make parser-error-unsupported-contract-test` | 0 | pass |
| unsupported-diagnostics-evidence-test | `make unsupported-diagnostics-evidence-test` | 0 | pass |
| pg-confidence-gates | `make pg-confidence-gates` | 0 | pass |
| pg-unit-test-gates | `make pg-unit-test-gates` | 0 | pass |
| query-access-corpus-gates | `make query-access-corpus-gates` | 0 | pass |
| sql-corpus-gates | `make sql-corpus-gates` | 0 | pass |
| release-dialect-hygiene-gates | `make release-dialect-hygiene-gates` | 0 | pass |
| three-level-doc | `bash ~/.agents/skills/check-three-level-doc/scripts/check_three_level_doc.sh` | 0 | L2 README sync over the working-tree diff (ran green pre-commit; rerun post-commit reports a clean tree) |

**Required-set count: 19 named gates**, all green or matching their
accepted baseline.

## Committed-diff L2 checks (E1)

The R1 committed-diff tuples recorded `git diff --name-only` as their
command while stdout came from a bespoke checker whose script was not
archived — that checker is recorded as **missing**. The current-rule
committed-range variant `l2_committed_range_check.sh` (same L2
semantics, diff source switched to `git diff --name-only <range>`) is
archived here with its tuples:

| Check | Range | rc | Result |
|-------|-------|----|--------|
| l2-committed-t14r2 | `17539dbb..ce9a73dc` (R2 product, 12 files) | 0 | clean — every impacted module ships its synced README.md in the diff |
| l2-committed-t14-cumulative | `b54a0544..ce9a73dc` (cumulative T14 product, 1280 files incl. evidence commits) | 0 | clean — exercises the probes/README and mcp/README closures restored in R1 |
| l2-final-tree | worktree check on the final tree | 0 | `[three-level-doc] no changed files` — honest clean-tree record, not a substituted pass |

The two R1 failure tuples (`l2-committed-b5b2ac70`, `l2-committed-c29ebc38`,
both rc=1) remain archived verbatim under the c29ebc38 bundle — not
rewritten.

## Extra diagnostic beyond the required set

| Gate | Command actually run | rc | Result |
|------|----------------------|----|--------|
| lint-fullrepo | `golangci-lint run ./...` | 1 | 37 diagnostics across the broader tree — identical count to the accepted R1 record; NOT the accepted gate scope |

## Six-suite golden (serial, same product SHA, same CLI fingerprint)

Each suite ran
`python3 ./scripts/ddl_golden.py run --task <T> --artifact-dir <bundle>/golden`;
the runner builds its own CLI from the checkout and records `head_sha`
plus binary sha256 in `artifact.json`. All six report
`head=ce9a73dc3b19` and share one CLI sha256 (`a7a1c1da9c0d…`).
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
| T14 | 68/68 | 1066 | PASS |

T09–T13 inputs/expectations/denominators are frozen and unchanged.
T14 keeps all 68 required case IDs; two carry the approved R2
ownership corrections (`t14r1-mysql80-string-domain-unproven` now
expects one `unknown_table_state` gap owned by `add_partition.target`
under profile `t14r2-add-truncate-target`;
`t14r1-mysql80-rcol-decreasing-tuple` now expects the 1493 finding
from `add_partition.target` under `t14-add-target`). The assertion
count rose 1065 → 1066 by the corrected gap expectation. Each
artifact's required/executed case sets are equal, no duplicate IDs,
`compose_down_rc=0`, `residual_containers=[]`.
