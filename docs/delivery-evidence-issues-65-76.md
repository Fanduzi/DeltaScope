# Delivery evidence for GitHub issues #65–#76

Verified on 2026-09-05 for [issues #65 through #76](https://github.com/Fanduzi/DeltaScope/issues) and parent spec [#77](https://github.com/Fanduzi/DeltaScope/issues/77). Pull request [#78](https://github.com/Fanduzi/DeltaScope/pull/78).

This file is the tracked delivery record. It does not contain secrets. L3 file headers do not apply to this Markdown evidence file. No module source membership or public architecture map changed in this evidence commit, so L1 `README.md` Architecture/Modules and L2 package READMEs were left unchanged.

## Revisions and delivery

- Base / pre-delivery `origin/main`: `a2bc7a79e48b89b0950e183a23651eefc6c93e07`
- Candidate / merged SHA: `7d0b202d602d827d04450bd621fe9023a1ef260e`
- Merge type: fast-forward only (`git merge --ff-only fix/issues-65-76-named-signals`) in `/Users/fan/GolangProjects/DeltaScope` on local `main`
- Push range: `a2bc7a79e48b89b0950e183a23651eefc6c93e07..7d0b202d602d827d04450bd621fe9023a1ef260e` to `origin/main`
- Post-push `origin/main`: `7d0b202d602d827d04450bd621fe9023a1ef260e`
- Local `HEAD` after push: `7d0b202d602d827d04450bd621fe9023a1ef260e`
- PR: https://github.com/Fanduzi/DeltaScope/pull/78 state MERGED, mergeCommit `7d0b202d602d827d04450bd621fe9023a1ef260e`, mergedAt 2026-09-05T09:44:45Z
- Range versus base: 15 commits, 95 files, +3654/-355

This evidence file is committed on top of `7d0b202d602d827d04450bd621fe9023a1ef260e`. The evidence commit SHA is the Git object that contains this file.

## Review

Independent Grok `general-purpose` subagents reviewed `main...HEAD` at `1231b78d18c3c3b049f951d78156d72617e6a2eb` (before the two follow-up commits).

- Standards subagent `01a06eb1-17ec-7f21-9b25-ac3e86a9543b`: 5 documented-standard findings, 2 judgement smells, unresolved P1 = 0. Documented findings: Loaded glossary over-collapsed Default Policy; `fk_forbid` wired through skip vocabulary; umbrella decision record still listed #69/#72 as unimplemented in Verification Evidence; two L3 headers stale; npm launcher README missed new exports.
- Spec subagent `01a06eb1-17ec-7f21-9b25-ac4a05ca9652`: actual verdict **Spec PASS**, unresolved P1 = 0, unresolved P2 = 0.

Follow-up `7b236c09c1bdee92fce3a8265efc2efd6abe9527` closed the five documented Standards findings. Follow-up `7d0b202d602d827d04450bd621fe9023a1ef260e` closed the `unparam` finding on `shippedRulePolicy`. Remaining judgement smells (duplicated `shippedRulePolicy` helpers, repeated Version/ReportedVersion fallback) were not treated as merge blockers. Unresolved P1 = 0, unresolved P2 = 0 after those commits.

## Candidate gates

All commands ran from `/Users/fan/GolangProjects/DeltaScope` on `fix/issues-65-76-named-signals` at `7b236c09c1bdee92fce3a8265efc2efd6abe9527` unless noted. The unparam-only follow-up `7d0b202d602d827d04450bd621fe9023a1ef260e` was rechecked with `go test ./internal/application/configstatus` and `golangci-lint run --new-from-rev=origin/main` (0 issues).

| Command | Result |
| --- | --- |
| `make test` | PASS at `7b236c09c1bdee92fce3a8265efc2efd6abe9527` |
| `make build` | PASS; built `bin/deltascope`, `bin/deltascope-server`, `bin/deltascope-mcp` |
| `make pg-unit-test-gates` | PASS |
| `make sql-corpus-gates` | PASS |
| `python3 scripts/test_pr_workflow_contract.py` | PASS; 11 tests, 0 failed |
| `npm test --prefix packages/deltascope-mcp` | PASS; 20 tests, 20 passed, 0 failed, 0 skipped |
| `bash scripts/test_install.sh` | PASS; 43 passed, 0 failed |
| `VERSION=v0.510.3 python3 scripts/verify_docs_examples.py` | PASS (`docs-examples: PASS`) |
| `python3 scripts/test_verify_docs_examples.py` | PASS; 96 tests |
| `make decision-record-gate` | PASS |
| `git diff --check origin/main...HEAD` | PASS |
| `golangci-lint run --new-from-rev=origin/main` | PASS; 0 issues at `7d0b202d602d827d04450bd621fe9023a1ef260e` |
| `go test ./internal/application/configstatus -count=1` | PASS at `7d0b202d602d827d04450bd621fe9023a1ef260e` |

Local `make lint` (`golangci-lint run ./...`) is not a merge blocker: it scans nested `.worktrees/` checkouts that are absent from GitHub CI. Required lint is the GitHub `golangci-lint` job.

Candidate gate total: 13 commands PASS, 0 FAIL.

## Merged-root gates

Commands ran from `/Users/fan/GolangProjects/DeltaScope` on branch `main` at SHA `7d0b202d602d827d04450bd621fe9023a1ef260e`.

| Command | Result |
| --- | --- |
| `make test` | PASS |
| `git rev-parse HEAD` equals `origin/main` | PASS; both `7d0b202d602d827d04450bd621fe9023a1ef260e` |

## GitHub CI at `7d0b202d602d827d04450bd621fe9023a1ef260e`

Required push workflows on `main` are Lint and CLI TLS E2E. Landing Page did not run because its path filter is `docs/landing/**` and this range did not change those files. `release` is tag-only (`v*`). `release-smoke` and `release-recover` are `workflow_dispatch` and were not run.

| Workflow | Run | Job | Conclusion |
| --- | --- | --- | --- |
| Lint | [33958805533](https://github.com/Fanduzi/DeltaScope/actions/runs/33958805533) | `go test` | success |
| Lint | [33958805533](https://github.com/Fanduzi/DeltaScope/actions/runs/33958805533) | `PostgreSQL unit tests` | success |
| Lint | [33958805533](https://github.com/Fanduzi/DeltaScope/actions/runs/33958805533) | `golangci-lint` | success |
| CLI TLS E2E | [33958805514](https://github.com/Fanduzi/DeltaScope/actions/runs/33958805514) | `CLI TLS E2E (MySQL 8.4 + PostgreSQL 17)` | success |

CI conclusion: required jobs succeeded. 0 required jobs failed. 0 required jobs skipped.

E2E: GitHub Actions job `CLI TLS E2E (MySQL 8.4 + PostgreSQL 17)` on run 33958805514, event `push`, branch `main`, SHA `7d0b202d602d827d04450bd621fe9023a1ef260e`, conclusion success, failed count 0, skipped count 0.

## Tickets

GitHub closed via PR #78 `Closes` lines at merge 2026-09-05T09:44:45Z:

- https://github.com/Fanduzi/DeltaScope/issues/65 CLOSED
- https://github.com/Fanduzi/DeltaScope/issues/66 CLOSED
- https://github.com/Fanduzi/DeltaScope/issues/67 CLOSED
- https://github.com/Fanduzi/DeltaScope/issues/68 CLOSED
- https://github.com/Fanduzi/DeltaScope/issues/69 CLOSED
- https://github.com/Fanduzi/DeltaScope/issues/70 CLOSED
- https://github.com/Fanduzi/DeltaScope/issues/71 CLOSED
- https://github.com/Fanduzi/DeltaScope/issues/72 CLOSED
- https://github.com/Fanduzi/DeltaScope/issues/73 CLOSED
- https://github.com/Fanduzi/DeltaScope/issues/74 CLOSED
- https://github.com/Fanduzi/DeltaScope/issues/75 CLOSED
- https://github.com/Fanduzi/DeltaScope/issues/76 CLOSED
- https://github.com/Fanduzi/DeltaScope/issues/77 CLOSED

## Root dirty-path whitelist

Preserved untracked paths, not part of the delivery:

- `.agents/`
- `.debug-journal.md`
- `.opencode/`
- `.pi-subagents/`
- `.qoder/`
- `docs/quality/`

No stash, reset, or clean of those paths. Unrelated worktrees under `.worktrees/` and `DeltaScope-release-v0.480.0-prep` were left in place.

## Cleanup

Feature branch `fix/issues-65-76-named-signals` was kept after push. It was not deleted.
