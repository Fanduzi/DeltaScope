# T05-A4-R2 verification evidence — 0cb6b7e

## Identity and scope

- Evidence tip before this rework: `6a852d4f243a097454547a3ff853d3648c838b84`.
- Reviewed code that this rework replaces: `77624ee71fbef56dbcd4f635ba1a9c33eb5d0ab8`.
- Tested code: `0cb6b7e38fc0063bb7f78dc2eb4db60e31528070`.
- Branch: `milestone/mysql-tidb-ddl-completion`. Delivery target is `origin/workfree`.
- Local `origin/main` observed as `9a28cc99c38770e6e40738515994f864166419f9`. No main merge.
- Local `origin/milestone/mysql-tidb-ddl-completion` observed as `1bf503ea1f80a1d156ea00270e6390e6bd3f25f3`. That ref was not pushed.
- Task diff `6a852d4f...0cb6b7e3`: 7 files, +277/-43. Names are in `identity/task-diff-names.txt`. This is not a `main...HEAD` milestone-completion claim.
- The evidence commit that adds this directory is separate from the code commit. Its SHA is reported with the push, not written back into this file.
- A4 is not accepted. A1, A2, A3, #82, and #83 stay accepted. #79 and #84 stay open. No PR, tag, release, or issue close.
- The detached worktree `/private/tmp/ds-t05-a3-final.2K4NKO/doc-check` stayed at `49a0f5bce25a72f71c1171ac0bee9bb5d86d963b`.

A non-precise `MODIFY` tombstones every named table identity and the already-loaded dependents of those identities. A precise replacement stays only when no loaded foreign key needs recomputation. Inline `PRIMARY KEY` and the TiDB signedness gate were left as closed in T05-A4-R1.

## What was executed

| Stage | Tree | Result |
|---|---|---|
| Red regressions | `6a852d4f`, before the production edit | `go test ./internal/application/audit/ -count=1 -run 'TestAuditSQLT05A4R2\|TestBatchStateA4R1AffectedSet'` failed, rc=1. Log: `red/r2-before-fix.stdout`. Head: `red/head.txt`. |
| Code-SHA golden | `0cb6b7e3` | `make ddl-golden TASK=T05 ARTIFACT_DIR=/tmp/ds-t05-a4-r2-golden-0cb6b7e38fc0063bb7f78dc2eb4db60e31528070`: 204/204, 3486 assertions, PASS. `python3 scripts/ddl_golden.py validate` printed `artifact valid`. |

T02, T03, and T04 were not rerun. This commit does not change the parser, the coverage catalog, or the golden runner. The prior T03 record remains `docs/verification/t05-a4/77624ee71fbef56dbcd4f635ba1a9c33eb5d0ab8/gates/t03-golden.log` (118/118, 1395 assertions). The new counterexamples stay on the real parser and AuditSQL path. They were not copied onto a database matrix.

## Red, as run at 6a852d4

`red/r2-before-fix.stdout` is the real parser and AuditSQL run before `prepareModifyColumn` published from the whole statement. These failed, and `TestAuditSQLT05A4R2UnrelatedReferenceStillPublishes` plus the complete-unrelated subtest did not:

- Cached-absent `dst.u` and the not-yet-loaded `dst.u` each produced `ddl.table.exists.alter.require` “table u does not exist”, `exists:false`.
- A loaded child of the RENAME destination, and a loaded source child whose foreign key references `id`, stayed known, so the later statement had no `unknown_table_state` gap.
- `MODIFY c, DROP COLUMN d` left the child that references `d` known.
- A precise `MODIFY` of `c` with a loaded child referencing `c` still published a present parent. The same blind spot failed for an empty referenced list and for unmodeled referenced parts.

The same filter passed after the production edit and is included in `GOFLAGS=-count=1 make test`.

## Gates at 0cb6b7e3

Commands ran in the root checkout at `0cb6b7e3`. `gates/sha-gates.log` is the combined stdout. `gates/sha-golden.log` is the T05 golden section.

| Gate | Command | Result |
|---|---|---|
| whitespace | `git diff --check 6a852d4f...0cb6b7e3` | clean |
| gofmt | `gofmt -l` on the four changed Go files | clean |
| golden | `make ddl-golden TASK=T05 ARTIFACT_DIR=/tmp/ds-t05-a4-r2-golden-0cb6b7e38fc0063bb7f78dc2eb4db60e31528070` | 204/204, 3486 assertions, PASS. Recorded head `0cb6b7e38fc0` |
| validate | `python3 scripts/ddl_golden.py validate --artifact .../T05/artifact.json` | `artifact valid` |
| validator | `make ddl-golden-validator-test` | `contract cases=155 failures=0` |
| unit | `GOFLAGS=-count=1 make test` | PASS |
| pg unit | `make pg-unit-test-gates` | PASS |
| corpus | `make sql-corpus-gates` | PASS |
| inventory | `make ddl-inventory-gate` | PASS |
| coverage catalog | `make ddl-coverage-catalog-test` | PASS |
| docs examples | `make docs-example-gates` | `docs-examples: PASS` |
| decision, task range | `./scripts/check_decision_record.sh 6a852d4f...0cb6b7e3` | decision file present, PASS |
| decision, makefile | `make decision-record-gate` (`main...HEAD`) | exit 0; the milestone diff still prints the keyword note |
| three-level | unstaged task diff immediately before the code commit | OK, L1 reminder, root READMEs unchanged |

`vcs.modified=true` is the built binary's own stamp (`identity/binary-version.txt`). The tracked diff was empty when that binary was built. The six preexisting untracked paths are listed in `identity/status-after-code.txt`. The dirty bit is not evidence of an uncommitted code edit.

The module line recorded from `go version -m` is `github.com/Fanduzi/DeltaScope v0.511.2-0.20261004075901-0cb6b7e38fc0+dirty`.

## Not run

No SDK, HTTP, or MCP live-server pass beyond the unit packages already inside `make test` and `make pg-unit-test-gates`. No second golden. No database execution of the new foreign-key counterexamples. This file does not claim reviewer acceptance.
