# T15 gate index — product SHA f9febe77

Every tuple stores `<name>.command.txt` (the real argv),
`<name>.stdout.txt`, `<name>.stderr.txt`, `<name>.rc.txt` (saved
immediately after the command exits). Gates ran on the T15 product
commits `c761fa38` + `f9febe77` (the working tree the gates executed
against is exactly `f9febe7771c4faaf184eb133494136f21f6121e0` — the
product commit plus the probes-README doc commit), whose parent is the
T14-R2-tail2 evidence commit `1813f062…`.

## Required gates

| Gate | Command actually run | rc | Result |
|------|----------------------|----|--------|
| go_test | `go test ./... -count=1` | 0 | whole repo, no cache — includes `batch_state_t15_test.go` + `extractor_t15_test.go`, none skipped |
| go_vet | `go vet ./...` | 0 | clean |
| gofmt | `gofmt -l ./cmd ./internal ./pkg` | 0 | only the two accepted T07 baseline files listed — no T15 file listed |
| lint-scoped | `golangci-lint run ./internal/application/audit/... ./internal/domain/spec/... ./internal/domain/rule/ddl/... ./internal/infrastructure/metadata/mysql/...` | 1 | exactly the 13 accepted baseline issues — stdout byte-identical to the T14 record (verified by diff); five transient T15 unparam findings were removed before this record, zero new findings shipped |
| go_race | `env CGO_ENABLED=1 go test -race -count=1 ./internal/...` | 0 | full internal tree |
| ddl-coverage-catalog-test | `make ddl-coverage-catalog-test` | 0 | postgresql-tagged test via the canonical make entry |
| ddl-coverage-catalog-test-verbose | `go test ./internal/application/audit -count=1 -tags postgresql -run '^TestDDLCoverageCatalog$' -v` | 0 | `--- PASS: TestDDLCoverageCatalog` — non-zero execution proven |
| ddl-golden-validator-test | `make ddl-golden-validator-test` | 0 | pass — T15 manifest accepted (42 required cases) |
| ddl-inventory-gate | `make ddl-inventory-gate` | 0 | pass — the four T15 inventory rows bind their frozen dimensions |
| decision-record-gate | `make decision-record-gate` | 0 | pass — new ADR `2026-11-08-ddl-table-options-online-ddl.md` |
| docs-example-gates | `make docs-example-gates` | 0 | pass — `configs/deltascope.example.yaml` gained the nine T15 rules at contract levels |
| parser-error-unsupported-contract-test | `make parser-error-unsupported-contract-test` | 0 | pass |
| unsupported-diagnostics-evidence-test | `make unsupported-diagnostics-evidence-test` | 0 | pass — promoted aspects retired from unsupported expectations in the two index-additions corpus files |
| pg-confidence-gates | `make pg-confidence-gates` | 0 | pass |
| pg-unit-test-gates | `make pg-unit-test-gates` | 0 | pass |
| query-access-corpus-gates | `make query-access-corpus-gates` | 0 | pass |
| sql-corpus-gates | `make sql-corpus-gates` | 0 | pass — corpus supported-rule coverage 100% (694/694 targets incl. all nine T15 rules on their real dialects) |
| release-dialect-hygiene-gates | `make release-dialect-hygiene-gates` | 0 | pass |
| three-level-doc | `bash /Users/fan/.agents/skills/check-three-level-doc/scripts/check_three_level_doc.sh` | 0 | L2 README sync over the working tree — `[three-level-doc] OK` (L1 reminder noted; root README unaffected) |

**Required-set count: 19 named gates**, all green or matching their
accepted baseline.

## Committed-diff L2 checks

`l2_committed_range_check.sh` is archived verbatim (same script the
T14 bundles carry; L2 semantics, diff source `git diff --name-only
<range>`):

| Check | Range | rc | Result |
|-------|-------|----|--------|
| l2-committed-t15 | `1813f062..f9febe77` (T15 product + docs, 37 files) | 0 | clean — one transient miss (`probes/README.md`) was fixed inside the committed range itself |
| l2-final-tree | worktree check on the committed tree | 0 | `[three-level-doc] no changed files` — honest clean-tree record, not a substituted pass |

## Extra diagnostic beyond the required set

| Gate | rc | Result |
|------|----|--------|
| lint-fullrepo (`golangci-lint run ./...`) | 1 | 37 issues — identical set to the T14-accepted diagnostic; the only textual diff is a line-number shift of the pre-existing `countIndexPartKinds` unused finding inside the T15-grown extractor |

## Graph change analysis (pre-commit requirement)

`codegraph sync` reported the index current; `codegraph impact` on the
five T15-owned seams (`OnlineDDLOutcomeFor`, `AlterTableOptionOutcome`,
`IsPartitionAlterAction`, `applyTableOptionEffects`,
`prepareMultiAction`) shows the blast radius confined to the new rules,
the ordered-state dispatch, and the coverage seam — no unintended
consumer outside the T15 change set.
