# T05-A5-R1 verification evidence — bcc8b65

## Identity and scope

- Task base: `ed022b60bc6ace836e41c55674d79e1311892ded` (`origin/workfree` when this rework started).
- Tested code: `bcc8b65b4b86a33887a2df334fdd2044f25c8df7`.
- Branch: `milestone/mysql-tidb-ddl-completion`. Delivery target is `origin/workfree`.
- `origin/main` observed as `9a28cc99c38770e6e40738515994f864166419f9`. No main merge.
- Task diff `ed022b60..bcc8b65`: 7 files, +512/-14. Names are in `identity/task-diff-names.txt`. This is not a `main...HEAD` milestone-completion claim.
- The evidence commit that adds this directory is separate. Its SHA is reported with the push, not written back into this file.
- The existing decision `docs/decisions/2026-10-21-ddl-change-rename-column-identity.md` was amended in the code commit. A new decision file was not added.
- A5 is not accepted. A1 through A4, #82, and #83 stay accepted. #79 and #84 stay open. No PR, tag, release, or issue close.
- The detached worktree `/private/tmp/ds-t05-a3-final.2K4NKO/doc-check` stayed at `49a0f5bce25a72f71c1171ac0bee9bb5d86d963b`.

## What changed

A nil index or constraint collection with `Unknown=false` stays a loaded empty set after CHANGE or RENAME. A CHANGE declaration that `modifyDeclaresExtraMember` already rejects tombstones the target before that partial snapshot is kept. The same column identity emits no destination-conflict finding and no destination-conflict gap. Source existence, compatibility, and the RENAME version check stay on their own contracts.

## Red and green

`red/` is `go test ./internal/application/audit/ ./internal/domain/rule/ddl/ -count=1 -run 'TestAuditSQLT05A5R1|TestColumnTargetExistsSameIdentityHasNoGap'` on `ed022b60` before the fix. rc=1. The failures are the nil known-empty scrub, the PRIMARY KEY and AUTO_INCREMENT early return, and the same-identity destination gap. Empty-slice collections, ordinary NOT NULL, an already-unknown collection, an affected member beside an unrelated member, a different new name, a real duplicate, a confirmed-absent table, and disabled or inapplicable statements were already green in that run.

`green/` is the same command with `-v` after the code commit, HEAD `bcc8b65b4b86a33887a2df334fdd2044f25c8df7`, rc=0.

## Gates at bcc8b65

Commands ran in the root checkout at `bcc8b65`. Each command's rc is the matching `*.rc` file.

| Gate | Command | rc |
|---|---|---|
| golden | `make ddl-golden TASK=T05 ARTIFACT_DIR=/tmp/ds-t05-a5-r1-golden-bcc8b65b4b86a33887a2df334fdd2044f25c8df7` | 0 |
| validate | `python3 scripts/ddl_golden.py validate --artifact <that>/T05/artifact.json` | 0 |
| validator | `make ddl-golden-validator-test` | 0 |
| unit | `GOFLAGS=-count=1 make test` | 0 |
| pg unit | `make pg-unit-test-gates` | 0 |
| corpus | `make sql-corpus-gates` | 0 |
| inventory | `make ddl-inventory-gate` | 0 |
| coverage catalog | `make ddl-coverage-catalog-test` | 0 |
| docs examples | `make docs-example-gates` | 0 |
| decision, task range | `./scripts/check_decision_record.sh ed022b60..HEAD` | 0 |
| decision, makefile | `make decision-record-gate` | 0 |
| whitespace | `git diff --check ed022b60..HEAD` | 0 |
| gofmt | `gofmt -l` on the four changed Go files | 0 |
| focus | A4/A5 AuditSQL, rule, SDK, CLI, HTTP, and MCP filters, plus the PostgreSQL-tagged A4/A5 legacy tests | 0 |

Golden printed `cases=231 passed=231 assertions=4484` and `PASS`. `artifact["head_sha"]` is `bcc8b65b4b86a33887a2df334fdd2044f25c8df7`. Validate printed `artifact valid`. The validator printed `contract cases=176 failures=0`.

The golden CLI sha256 is `20ba44df7c416bfd0dcbf79db32fcb23860049180f60bdcd1dd78454d50e61b8`. `go version -m` records `vcs.revision=bcc8b65b4b86a33887a2df334fdd2044f25c8df7` and `vcs.modified=true`. The tracked tree was clean. The dirty bit is the six preexisting untracked paths in `identity/status-after-code.txt`. The binary itself is not in git.

`gates/short/three-level.stdout` says `no changed files` because that checker reads the working tree after the code commit. The same checker passed on the uncommitted patch, and the committed Go files contain `input`, `output`, `pos`, `note`, and a package comment. Root `README.md` was left unchanged: the module map did not change.

## Policy generation for older tasks

T02, T03, and T04 databases were not rerun. `gates/policy-diff.txt` is the code-SHA `rules list` catalog, 380 rules, joined to the manifest enable maps. The three A5 rule IDs stay enabled only in `t05-a5-column-identity-isolated` (11 enabled). The other T05 profiles and the T02, T03, and T04 enable maps leave them disabled.

## Not claimed

No hosted CI result. No second reviewer pass. A5 is not accepted. #79, #84, and the milestone stay open.
