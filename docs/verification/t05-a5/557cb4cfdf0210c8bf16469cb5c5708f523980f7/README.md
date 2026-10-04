# T05-A5 verification evidence — 557cb4cf

## Identity and scope

- Task base: `fe073f56ed7d378336d5e2af17dc6735fbc670ad` (`origin/workfree` at the start of this slice).
- Tested code: `557cb4cfdf0210c8bf16469cb5c5708f523980f7`.
- Branch: `milestone/mysql-tidb-ddl-completion`. Delivery target is `origin/workfree`.
- Local `origin/main` observed as `9a28cc99c38770e6e40738515994f864166419f9`. No main merge.
- Local `origin/milestone/mysql-tidb-ddl-completion` observed as `1bf503ea1f80a1d156ea00270e6390e6bd3f25f3`. That ref was not pushed.
- Task diff `fe073f56..557cb4cf`: 39 files, +8000/-189. Names are in `identity/task-diff-names.txt`. This is not a `main...HEAD` milestone-completion claim.
- The evidence commit that adds this directory is separate from the code commit. Its SHA is reported with the push, not written back into this file.
- A decision record was required and is in the code commit: `docs/decisions/2026-10-21-ddl-change-rename-column-identity.md`. The A4 deferred-scope paragraph now points at that limited CHANGE and RENAME implementation.
- A5 is not accepted. A1 through A4, #82, and #83 stay accepted. #79 and #84 stay open. No PR, tag, release, or issue close.
- The detached worktree `/private/tmp/ds-t05-a3-final.2K4NKO/doc-check` stayed at `49a0f5bce25a72f71c1171ac0bee9bb5d86d963b`.

A precise single CHANGE or RENAME migrates the source column in place when the destination name is free. CHANGE reuses the ordinary definition replacement. RENAME copies the identity and keeps unknown fields. A known incompatible RENAME version is one complete blocker. A missing or out-of-range version stays a gap and does not publish.

## What was executed

| Stage | Tree | Result |
|---|---|---|
| Red first path | working tree before identity migration, parent `fe073f56` | `TestAuditSQLT05A5ChangeFirstPath` failed, rc=1. Both dialects left statement 2 unverified with the three unknown-table gaps. Log: `red/stdout.txt`. |
| Code-SHA golden | `557cb4cf` | `make ddl-golden TASK=T05 ARTIFACT_DIR=/tmp/ds-t05-a5-golden-557cb4cfdf0210c8bf16469cb5c5708f523980f7`: 231/231, 4484 assertions, PASS. Recorded head `557cb4cfdf0210c8bf16469cb5c5708f523980f7`. |

`gates/golden.log` is that run. `python3 scripts/ddl_golden.py validate --artifact .../T05/artifact.json` printed `artifact valid` (`gates/validate.log`).

## Red, as saved before the publisher

`red/stdout.txt` is the real parser and AuditSQL CHANGE path while a single CHANGE still invalidated the table. Statement index 2 (MODIFY) was unverified on MySQL and TiDB. The assertion was not weakened. The same test passes in the code-SHA focus run.

## Plan-stage records

`plan/` is the contents of `/tmp/ds-t05-a5-plan/` except the 31.6 MB CLI binary. That binary's sha256 is `identity/plan-binary.sha256`: `679b67e76658a1fd4fbd18871e1c34848d2df94ddc407151e500a0311dfb753d`. These records were not treated as acceptance. `plan/schema-before.txt` and `plan/schema-after.txt` are empty. Table absence in that experiment is `plan/_count.out` (`COUNT(*)` = 0). The code-SHA golden run is the database proof.

## Gates at 557cb4cf

Commands ran in the root checkout at `557cb4cf`. `gates/sha-gates.log` is the combined stdout.

| Gate | Command | Result |
|---|---|---|
| whitespace | `git diff --check fe073f56..557cb4cf` | clean |
| gofmt | `gofmt -l` on the changed Go files | clean |
| golden | `make ddl-golden TASK=T05 ARTIFACT_DIR=/tmp/ds-t05-a5-golden-557cb4cfdf0210c8bf16469cb5c5708f523980f7` | 231/231, 4484 assertions, PASS |
| validate | `python3 scripts/ddl_golden.py validate --artifact .../T05/artifact.json` | `artifact valid` |
| validator | `make ddl-golden-validator-test` | `contract cases=176 failures=0` |
| unit | `GOFLAGS=-count=1 make test` | PASS |
| pg unit | `make pg-unit-test-gates` | PASS |
| corpus | `make sql-corpus-gates` | PASS |
| inventory | `make ddl-inventory-gate` | PASS |
| coverage catalog | `make ddl-coverage-catalog-test` | PASS |
| docs examples | `make docs-example-gates` | `docs-examples: PASS` |
| decision, task range | `./scripts/check_decision_record.sh fe073f56..HEAD` | decision file present, PASS |
| decision, makefile | `make decision-record-gate` (`main...HEAD`) | PASS, no trigger keywords in that range |
| focus | A5 AuditSQL, batch-state, CLI, HTTP, MCP, and SDK filters, plus the PostgreSQL-tagged three-shape test | PASS |

`vcs.modified=true` on the code-SHA binary (`identity/binary-version.txt`, `identity/binary-version-m.txt`). The tracked diff was empty when that binary was built. The six preexisting untracked paths are listed in `identity/status-after-code.txt`. The dirty bit is not evidence of an uncommitted code edit.

The module line recorded from `go version -m` is `github.com/Fanduzi/DeltaScope v0.511.2-0.20261004102258-557cb4cfdf02+dirty`. The code-SHA binary sha256 recorded here is `8c60158acf6ceb73a35371ba15f1fcb5abc79169e12a688f32085ef1d1f4e214`. These hashes are the outputs of this checkout. They are not a claim that a reviewer recomputed them.

## Policy generation for older tasks

T02, T03, and T04 databases were not rerun. `gates/policy-diff.txt` shows the live catalog at 380 rules. The three new rule IDs are enabled only by `t05-a5-column-identity-isolated` (11 enabled). `t05-a4-modify-isolated` stays at 4 enabled, `t05-drop-recreate-isolated` at 5, `t05-first-path-isolated` at 4, and the T04 profiles at 1. The T02, T03, and T04 manifests do not name the new IDs in any enable map, so generated policies keep them disabled. `t05_a5_manifest_failures` returns immediately unless `task_id` is T05. Case execution for the older tasks was not changed.

## Not claimed

No hosted CI result. No second reviewer pass. A5 is authorized implementation with this evidence, not acceptance. #79, #84, and the milestone stay open.
