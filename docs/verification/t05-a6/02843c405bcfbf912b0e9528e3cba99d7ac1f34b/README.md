# T05-A6 verification evidence — 02843c40

## Identity and scope

- Task base: `903fc01b068388bf89971c055ddb68f3afe30b28` (`origin/workfree` when this slice started).
- Tested code: `02843c405bcfbf912b0e9528e3cba99d7ac1f34b`.
- Branch: `milestone/mysql-tidb-ddl-completion`. Delivery target is `origin/workfree`.
- `main` observed as `9a28cc99c38770e6e40738515994f864166419f9`. No main merge.
- Task diff `903fc01b..02843c40`: 23 files, +5649/-18. Names are in `identity/task-diff-names.txt`. This is not a `main...HEAD` milestone-completion claim.
- The evidence commit that adds this directory is separate. Its SHA is reported with the push, not written back into this file.
- Decision record `docs/decisions/2026-10-04-ddl-drop-column-state.md` is in the code commit. It accepts only the dependency-free single-action subset.
- A6 is not accepted. A1 through A5, #82, and #83 stay accepted. #79 and #84 stay open. No PR, tag, release, or issue close.
- The detached worktree `/private/tmp/ds-t05-a3-final.2K4NKO/doc-check` stayed at `49a0f5bc`. Untracked `.agents/`, `.debug-journal.md`, `.opencode/`, `.pi-subagents/`, `.qoder/`, and `docs/quality/architecture-review-2026-08-11.html` were left untracked.

## What changed

One ordinary, dependency-free `DROP COLUMN` removes that column for later statements in the same MySQL or TiDB request. The remaining order stays, and a known unrelated primary key or index stays, including `PRIMARY(id)`. Ordinary indexes that name the column, unknown member collections, the last column, and multi-action statements tombstone the affected identities. The six existing blockers are unchanged. The T05 manifest keeps its previous 231 required ids and adds 16 (247 total).

## Red and green

`red/stdout.txt` is the go test JSON log of `TestAuditSQLT05A6DropColumnFirstPath` before the post-state existed. rc=1. Statement index 2 on both mysql and tidb was unverified, with `missing_source_column` on modify compatibility and `unknown_table_state` on alter existence and modify existence. The product had not yet kept `id` and `keep_c`.

`green/` is the same two tests, first path and `ix_removed(obsolete)`, with `-v` after the code commit. HEAD `02843c405bcfbf912b0e9528e3cba99d7ac1f34b`, rc=0.

## Gates at 02843c40

Commands ran in the root checkout at `02843c405bcfbf912b0e9528e3cba99d7ac1f34b`. Each command's rc is the matching `gates/*.rc` file.

| Gate | Command | rc |
|---|---|---|
| golden | `make ddl-golden TASK=T05 ARTIFACT_DIR=/tmp/ds-t05-a6-golden-02843c405bcfbf912b0e9528e3cba99d7ac1f34b-r2` | 0 |
| validate | `python3 scripts/ddl_golden.py validate --artifact <that>/T05/artifact.json` | 0 |
| validator | `make ddl-golden-validator-test` | 0 |
| unit | `go test ./... -count=1` | 0 |
| pg unit | `make pg-unit-test-gates` | 0 |
| corpus | `make sql-corpus-gates` | 0 |
| inventory | `make ddl-inventory-gate` | 0 |
| coverage catalog | `make ddl-coverage-catalog-test` | 0 |
| docs examples | `make docs-example-gates` | 0 |
| decision, task range | `./scripts/check_decision_record.sh 903fc01b068388bf89971c055ddb68f3afe30b28..HEAD` | 0 |
| decision, makefile | `make decision-record-gate` | 0 |
| whitespace | `git diff --check 903fc01b068388bf89971c055ddb68f3afe30b28..HEAD` | 0 |
| gofmt | `gofmt -l` on the nine A6 Go files | 0 |
| focus | A3–A6 application, PostgreSQL-tagged A4–A6, and SDK/CLI/HTTP/MCP filters | 0 |

Golden printed `cases=247 passed=247 assertions=5122` and `PASS`. `artifact["head_sha"]` is `02843c405bcfbf912b0e9528e3cba99d7ac1f34b`. Validate printed `artifact valid`. The validator printed `contract cases=204 failures=0`. The sixteen `t05-a6` cases are listed in `cases/a6-digest.txt`.

The first golden attempt failed before any case ran: container name `/deltascope-ddl-golden-mysql84` (`cef9b36a27c6`) was already in use by a leftover compose project named `docker`. That container was removed, and the rerun above is the recorded pass. `gates/golden-conflict.rc` is 2. Cleanup of the successful run removed the four anchors and reported no residual containers.

The golden CLI sha256 is `2c4cd1901a923b0151d7d593725c7b285716f0d4809f5ce4fd89983e36b46956`. `go version -m` records `vcs.revision=02843c405bcfbf912b0e9528e3cba99d7ac1f34b` and `vcs.modified=true`. The tracked tree was clean. The dirty bit is the preexisting untracked paths in `identity/status-after-code.txt`. The binary and the 3.2MB artifact are not in git. `identity/artifact.sha256` records the local artifact. Its commands use `--password-env DS_T05_GOLDEN_PW` and MySQL prints a password warning.

`gates/three-level.stdout` says `no changed files` because that checker reads the working tree after the code commit. The same checker passed on the uncommitted patch. Root `README.md` was left unchanged: the module map did not change.

## Policy

`gates/policy-profiles.txt` is the code-SHA catalog, 380 rules. `ddl.alter.drop_column.exists.require` is enabled only in `t05-a6-drop-column-isolated` (6 enabled). Older T05 profiles keep their previous enable maps. T02, T03, and T04 databases were not rerun.

## Plan originals

`plan/` copies `/tmp/t05-a6-plan` except `bin/` (32MB). The SQL files, the six-rule policy, and the plan-phase gate logs are the originals from before this implementation. They are not a substitute for the gates above.

## Not claimed

No hosted CI result. No second reviewer pass. A6 is not accepted. #79, #84, and the milestone stay open.
