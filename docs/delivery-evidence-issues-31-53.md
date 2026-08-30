# Delivery evidence for GitHub issues #31–#53

Verified on 2026-08-30 for [issues #31 through #53](https://github.com/Fanduzi/DeltaScope/issues).

This file is the tracked delivery record. It does not contain secrets. L3 file headers do not apply to this Markdown evidence file. No module source membership or public architecture map changed, so L1 `README.md` Architecture/Modules and L2 package READMEs were left unchanged.

## Revisions and delivery

- Base / pre-delivery `origin/main`: `14b6812d0e34224780e411ab0b12c30beb87dcf3`
- Original candidate HEAD on `milestone/bar70-20260829`: `afe0004f547ebfaaf79278ca58179d3fff233a0c`
- Delivery HEAD after the Standards P2 docs fix: `b06f9697e58f480f51d2bee4423a0a932d465d6c`
- Merge type: fast-forward only (`git merge --ff-only milestone/bar70-20260829`) in `/Users/fan/GolangProjects/DeltaScope` on local `main`
- First code push range: `14b6812d0e34224780e411ab0b12c30beb87dcf3..b06f9697e58f480f51d2bee4423a0a932d465d6c`
- First-push `origin/main`: `b06f9697e58f480f51d2bee4423a0a932d465d6c`
- Range versus base at that SHA: 91 commits, 316 files, +10107/-743
- Remote Git SSH `git fetch`/`ls-remote` to `git@github.com:Fanduzi/DeltaScope.git` timed out (`Connection timed out during banner exchange` to `198.18.0.74:22`). Remote `main` was read and later pushed over HTTPS (`gh api` and `git -c url."https://github.com/".insteadOf="git@github.com:"`). GitHub `commits/main` after the first push was `b06f9697e58f480f51d2bee4423a0a932d465d6c`.

This evidence file is committed on top of `b06f9697e58f480f51d2bee4423a0a932d465d6c`. The evidence commit SHA is the Git object that contains this file.

## Review

Independent Grok `general-purpose` subagents reviewed candidate `afe0004f547ebfaaf79278ca58179d3fff233a0c` against base `14b6812d0e34224780e411ab0b12c30beb87dcf3`. Claude, pi, opencode, and qoder were not used.

- Standards subagent `01a05019-08db-7e30-b127-c77098d98da7`: actual verdict **Standards FAIL**, unresolved P1 = 0, unresolved P2 = 1. The P2 was public-contract documentation drift for mixed parser-error results and optional diagnostic `line`/`column` in `docs/reference/cli.md`, `docs/reference/cli.zh-CN.md`, `docs/reference/http-api.md`, `docs/reference/http-api.zh-CN.md`, `docs/reference/library.md`, and `docs/reference/library.zh-CN.md`.
- Spec subagent `01a05019-08db-7e30-b127-c7821116a83c`: actual verdict **Spec PASS**, unresolved P1 = 0, unresolved P2 = 0. Issues #31–#53 were each reported PASS.

The P2 was resolved by commit `b06f9697e58f480f51d2bee4423a0a932d465d6c` (`docs: document mixed parser-error public contract`) on `milestone/bar70-20260829` before the fast-forward. Residual unresolved P1 = 0, residual unresolved P2 = 0.

## Candidate gates

All commands ran from `/Users/fan/GolangProjects/DeltaScope/.worktrees/milestone-bar70-20260829` at SHA `afe0004f547ebfaaf79278ca58179d3fff233a0c` except the docs-example re-run after the P2 fix, which ran in the same worktree at `b06f9697e58f480f51d2bee4423a0a932d465d6c`. Docker-backed suites ran strictly serially.

| Command | Result |
| --- | --- |
| `make test` | PASS |
| `make lint` | PASS; `golangci-lint` reported 0 issues |
| `make build` | PASS; built `bin/deltascope`, `bin/deltascope-server`, `bin/deltascope-mcp` |
| `make sql-corpus-gates` | PASS |
| `make query-access-corpus-gates` | PASS |
| `make pg-unit-test-gates` | PASS |
| `npm test --prefix packages/deltascope-mcp` | PASS; 15 tests, 15 passed, 0 failed, 0 skipped |
| `git diff --check origin/main...HEAD` | PASS |
| `make docs-example-gates` | PASS (`docs-examples: PASS`); re-run PASS at `b06f9697e58f480f51d2bee4423a0a932d465d6c` |
| `make lint-landing` | PASS (`landing page JS syntax OK`) |
| `make test-e2e-cli-tls` | PASS; 12 total, 12 passed, 0 failed, 0 skipped; serving CWD `/Users/fan/GolangProjects/DeltaScope/.worktrees/milestone-bar70-20260829`, serving SHA `afe0004f547ebfaaf79278ca58179d3fff233a0c` |
| `make test-e2e-cli` | PASS (MySQL then TiDB metadata CLI harnesses, serial); 0 failed, 0 skipped; same CWD/SHA |
| `make test-e2e-http-mysql` | PASS; `TestRunServesMetadataAwareAuditOverRealMySQL` 1 passed, 0 failed, 0 skipped; same CWD/SHA |
| `make pg-e2e-gates` | PASS (`test-e2e-cli-postgresql`, `test-e2e-http-postgresql`, `test-e2e-mcp-postgresql` serial); 0 failed, 0 skipped; same CWD/SHA |

Candidate gate total: 14 commands PASS, 0 FAIL.

## Merged-root gates

All commands ran from `/Users/fan/GolangProjects/DeltaScope` on branch `main` at SHA `b06f9697e58f480f51d2bee4423a0a932d465d6c`. Docker-backed suites ran strictly serially.

| Command | Result |
| --- | --- |
| `make test` | PASS |
| `make lint` | PASS; `golangci-lint` reported 0 issues |
| `make build` | PASS |
| `make sql-corpus-gates` | PASS |
| `make query-access-corpus-gates` | PASS |
| `make pg-unit-test-gates` | PASS |
| `npm test --prefix packages/deltascope-mcp` | PASS; 15 tests, 15 passed, 0 failed, 0 skipped |
| `git diff --check origin/main...HEAD` | PASS |
| `make docs-example-gates` | PASS |
| `make lint-landing` | PASS |
| `make test-e2e-cli-tls` | PASS; 12 total, 12 passed, 0 failed, 0 skipped; serving CWD `/Users/fan/GolangProjects/DeltaScope`, serving SHA `b06f9697e58f480f51d2bee4423a0a932d465d6c` |
| `make test-e2e-cli` | PASS; 0 failed, 0 skipped; same CWD/SHA |
| `make test-e2e-http-mysql` | PASS; 1 passed, 0 failed, 0 skipped; same CWD/SHA |
| `make pg-e2e-gates` | PASS; 0 failed, 0 skipped; same CWD/SHA |

Merged-root gate total: 14 commands PASS, 0 FAIL.

## First-push GitHub CI at `b06f9697e58f480f51d2bee4423a0a932d465d6c`

Required push workflows on `main` are Lint, CLI TLS E2E, and Landing Page (path filter matched `docs/landing/index.html`). `release` is tag-only (`v*`). `release-smoke` and `release-recover` are `workflow_dispatch` and were not run.

| Workflow | Run | Job | Conclusion |
| --- | --- | --- | --- |
| Lint | [33284304447](https://github.com/Fanduzi/DeltaScope/actions/runs/33284304447) | `golangci-lint` | success |
| CLI TLS E2E | [33284304435](https://github.com/Fanduzi/DeltaScope/actions/runs/33284304435) | `CLI TLS E2E (MySQL 8.4 + PostgreSQL 17)` | success |
| Landing Page | [33284304441](https://github.com/Fanduzi/DeltaScope/actions/runs/33284304441) | `Inline JS syntax check` | success |

CI TLS E2E command: `make test-e2e-cli-tls`. Log: 12 total, 12 passed, 0 failed, 0 skipped. Checkout `git log -1 --format=%H` printed `b06f9697e58f480f51d2bee4423a0a932d465d6c`. Runner workdir `/home/runner/work/DeltaScope/DeltaScope`; fixture workspace `/tmp/deltascope-cli-tls-e2e.a9hprr`.

The evidence commit is a documentation-only follow-up. After it is pushed, required CI for the final `origin/main` HEAD is the same three workflows. Those final-run URLs are not written here because they do not exist until that push.

## Preservation and cleanup

Allowed user-owned dirty-path whitelist: `.agents/`, `.debug-journal.md`, `.opencode/`, `.pi-subagents/`, `.qoder/`, `docs/quality/`. Compared against the candidate diff before merge: no overlap. After merge and merged-root gates they remained untracked and untouched.

No stash, reset, clean, relocate, overwrite, or delete of whitelist paths. Candidate branch `milestone/bar70-20260829` and worktree `/Users/fan/GolangProjects/DeltaScope/.worktrees/milestone-bar70-20260829` were intentionally preserved. Unrelated worktrees and branches were not deleted. Unrelated Docker containers `controlhub-query-e2e-mysql` and `mac-connector` were not removed.

A new decision record was not required for this evidence file. The P2 docs commit documents the already-accepted mixed parser-error contract in `docs/decisions/2026-08-30-partial-parser-error-recovery.md`.
