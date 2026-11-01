# Lead-Authored T18 Final Gate Recipe

This is a recipe, not execution evidence. Run only after lead review has no
unresolved findings, the full manifest has passed a WIP preflight, and lead
has committed/frozen the product. Every record must identify the same full
product SHA and CWD. Do not author substitute queries or weaken expectations.

## Identity And Resource Preconditions

- Repository: /Users/fan/GolangProjects/DeltaScope.
- Task base: 7f67c5de42fb6096d9c80f51e2d0f217eb34e528.
- Record HEAD, branch, tracked/staged diff, owned paths, foreign untracked
  preservation, worktrees, Go/Python/lint versions and Docker identities.
- Product tests run before evidence files are materialized in the worktree.
- Preserve all historical artifacts and all failed new attempts. Use unique
  independent artifact directories, never canonical old artifact path reuse.
- Check protected 17 containers against the approved cleanup after.json
  (a JSON list, not an object), including IDs, running state and mounts.
- Canonical DDL Golden owns only its five services. PG confidence owns
  deltascope-pg-e2e; its name is absent from the frozen protected list.
  Check live state before startup; a foreign collision is a blocker.
- No global prune, system-schema probing scripts, ad-hoc account/grant changes,
  unrelated worktree changes or ControlHub operations. Existing canonical PG
  fixture-role setup is confined to its disposable container, never real users.

## One Final Gate Pass

Record actual command/stdout/stderr/rc independently for each entry. Reuse the
existing recording conventions, not another generic test framework.

1. `go test ./... -count=1`
2. `go vet ./...`
3. `env CGO_ENABLED=1 go test -race ./internal/... -count=1`
4. `make pg-confidence-gates` (already includes canonical PG unit gates;
   do not duplicate pg-unit-test-gates immediately before it).
5. `make sql-corpus-gates`
6. `make query-access-corpus-gates`
7. `make ddl-inventory-gate`
8. `make ddl-coverage-catalog-test`
9. `make ddl-golden-validator-test`
10. `make parser-error-unsupported-contract-test`
11. `make unsupported-diagnostics-evidence-test`
12. `go test ./internal/interfaces/http ./internal/interfaces/mcp -run '^TestT18' -count=1 -v`
13. `make docs-example-gates`
14. `make decision-record-gate`
15. `./scripts/check_decision_record.sh <task-base>..<product-sha>`
16. `/opt/homebrew/bin/bash docs/verification/t17-database-schema-lifecycle/adc1f7322e1c70524435ed687fe27b67b931abb5/gates/l2_committed_range_check.sh <task-base>..<product-sha>`
17. Scoped formatting check over changed Go paths, plus `git diff --check`.
18. Exact accepted eight-tree scoped lint command in the next section.
19. `golangci-lint run ./...`
20. `python3 /tmp/t18-evidence/compare_lint.py <current-receipts-dir>`
21. `make ddl-golden TASK=T16 ARTIFACT_DIR=<unique-final-T16-dir>`
22. `make ddl-golden TASK=T17 ARTIFACT_DIR=<unique-final-T17-dir>`
23. `make ddl-golden TASK=T18 ARTIFACT_DIR=<unique-final-T18-dir>`

Golden T17's original disposable-fixture system-schema cases are the existing
user-authorized exception, not permission for new system-schema experiments.
T09-T15 artifacts stay at their original SHAs. No shared parser upgrade occurred;
T16/T17, Query Access and the shared PG confidence paths are the required scope.
Report actual case/assertion counts from immutable artifacts; companion Go
proofs are a separate layer, never added to those counts.

## Exact Scoped Lint Identity

`golangci-lint run ./internal/application/audit/... ./internal/application/auditmeta/... ./internal/domain/spec/... ./internal/domain/rule/ddl/... ./internal/domain/policy/... ./internal/domain/rule/catalog/... ./internal/infrastructure/metadata/mysql/... ./internal/infrastructure/parser/tidb/...`

Use stems lint-scoped and lint-fullrepo with raw stdout/stderr/rc files.
Accepted baseline is the 7f67c5de evidence at the c6eebc09 product directory.
The comparator checks full diagnostic identity and multiplicity, ignoring
only shifted line/column coordinates. Expected inherited baselines are 22/39;
rc1 is not a green-lint claim. No new or removed identities are accepted.

## Closure

Lead checks the output artifacts, current-statement and successor proofs,
required/executed identity sets, CLI fingerprint, manifest bytes, cleanup and
protected resources. Product and evidence are separate appended commits.
Only the lead commits and executes the authorized normal fast-forward push
`git push origin HEAD:refs/heads/workfree` after live ref/ancestry preflight.
Milestone ref stays d0c7cf3373167db7065d3027088a43c56ed6dbf5.
No merge, issue closure, PR, release, deployment, T19 or CI-success inference.
