# T05-A4-R1 verification evidence — 77624ee

## Identity and scope

- Evidence tip before this rework: `c33589688584ae11c12d9c34c85e05401d899d38`.
- Reviewed code that this rework replaces: `ce93dbf8bec796a3b16a3f3a5f0e7fc2ed11a109`.
- Tested code: `77624ee71fbef56dbcd4f635ba1a9c33eb5d0ab8`.
- Branch: `milestone/mysql-tidb-ddl-completion`. Delivery target is `origin/workfree`.
- Local `origin/main` observed as `9a28cc99c38770e6e40738515994f864166419f9`. No main merge.
- Task diff `c3358968...77624ee`: 15 files, +854/-76. Names are in `identity/task-diff-names.txt`. This is not a `main...HEAD` milestone-completion claim.
- The evidence commit that adds this directory is separate from the code commit. Its SHA is reported with the push, not written back into this file.
- A4 is not accepted. A1, A2, A3, #82, and #83 stay accepted. #79 and #84 stay open. No PR, tag, release, or issue close.

The rework keeps one affected-set publication for ordinary `MODIFY COLUMN`. A non-precise statement still drops already-loaded dependents of the named column. Inline `PRIMARY KEY` is parser-owned presence. A TiDB known integer primary key whose signedness changes is not published as a successor.

## What was executed

| Stage | Tree | Result |
|---|---|---|
| Red regressions | `c3358968`, before the production edit | `go test ./internal/application/audit/ -count=1 -run 'TestAuditSQLT05A4R1\|TestBatchStateA4R1'` failed. Log: `red/r1-before-fix.stdout`. Head: `red/head.txt`. |
| Code-SHA golden | `77624ee` | `make ddl-golden TASK=T05 ARTIFACT_DIR=/tmp/ds-t05-a4-r1-golden-77624ee`: 204/204, 3486 assertions, PASS. `python3 scripts/ddl_golden.py validate` printed `artifact valid`. |
| Parser golden | `77624ee` | `make ddl-golden TASK=T03 ARTIFACT_DIR=/tmp/ds-t05-a4-r1-t03-77624ee`: 118/118, 1395 assertions, PASS. Validate printed `artifact valid`. Log: `gates/t03-golden.log`. |

T02 and T04 were not rerun. `t05_a4_manifest_failures` returns immediately unless `task_id` is `T05`. The runner's case-execution path was not changed. The parser change is the new `DeclaresPrimaryKey` presence fact, so T03 was the golden that had to be repeated. Their previously accepted artifacts are not recopied here.

## Frozen oracle, as run at 77624ee

The original 185 T05 cases and the 18 ordinary MODIFY cases stayed in the same 204-case run. `t05-a4-tidb85-pk-unsigned` is the added case.

Metadata runs use a live anchor, `--fail-on blocker`, and the four-rule profile `t05-a4-modify-isolated` (`rule_summary.loaded=4`). The product audit does not execute the user's SQL. The driver executes it and queries the column definition after every statement it sends.

The new tidb85 case audits three statements and sends only the first two to the server:

```sql
CREATE TABLE t (id INT PRIMARY KEY);
ALTER TABLE t MODIFY COLUMN id INT UNSIGNED;
ALTER TABLE t MODIFY COLUMN id BIGINT UNSIGNED;
```

Observed product result on `77624ee`, banner `8.0.11-TiDB-v8.5.0`:

- Statement 0: complete, no findings, no gaps.
- Statement 1: complete, one `ddl.alter.modify_column.compatibility.require` blocker, `source_unsigned=false`, `target_unsigned=true`, column `id`, no gaps.
- Statement 2: unverified, no findings, three gaps in this order: compatibility `missing_source_column` (`source_column.definition`), `ddl.table.exists.alter.require` `unknown_table_state` (`target_table.existence`), `ddl.alter.modify_column.exists.require` `unknown_table_state` (`target_table.columns`, `target_table.existence`).
- Aggregate: exit 1, verdict reject, coverage unverified.

Driver statement 1 returned rc 1. Stderr was `ERROR 8200 (HY000) at line 1: Unsupported modify column: this column has primary key flag`. The following statement was not sent. After that refusal the table still had `id` as signed `int` `NOT NULL` and primary key `id`.

The existing `t05-a4-tidb85-pk-not-null` case (`INT PRIMARY KEY` to `BIGINT` to `BIGINT NOT NULL`) stayed exit 0, complete/pass. MySQL primary-key signedness stays on the ordinary integer path in the shared AuditSQL tests. Those tests are not a second copy of this golden case.

## Gates at 77624ee

Commands ran in the root checkout at `77624ee` unless noted. `gates/sha-gates.log` is the combined local stdout. `gates/sha-golden.log` is the T05 golden stdout.

| Gate | Command | Result |
|---|---|---|
| golden | `make ddl-golden TASK=T05 ARTIFACT_DIR=/tmp/ds-t05-a4-r1-golden-77624ee` | 204/204, 3486 assertions, PASS |
| validate | `python3 scripts/ddl_golden.py validate --artifact .../T05/artifact.json` | `artifact valid` |
| T03 golden | `make ddl-golden TASK=T03 ARTIFACT_DIR=/tmp/ds-t05-a4-r1-t03-77624ee` | 118/118, 1395 assertions, PASS |
| T03 validate | `python3 scripts/ddl_golden.py validate --artifact .../T03/artifact.json` | `artifact valid` |
| unit | `GOFLAGS=-count=1 make test` | PASS |
| pg unit | `make pg-unit-test-gates` | PASS |
| corpus | `make sql-corpus-gates` | PASS |
| inventory | `make ddl-inventory-gate` | PASS |
| coverage catalog | `make ddl-coverage-catalog-test` | PASS |
| validator | `make ddl-golden-validator-test` | `contract cases=155 failures=0` |
| docs examples | `make docs-example-gates` | PASS |
| decision, task range | `./scripts/check_decision_record.sh c3358968...77624ee` | decision file present, PASS |
| decision, makefile | `make decision-record-gate` (`main...HEAD`) | exit 0; the milestone diff still prints the keyword note |
| three-level | staged replay of the 15-file diff | OK, L1 reminder, root READMEs unchanged |
| whitespace | `git diff --check c3358968...77624ee` | clean |

`vcs.modified=true` is the built binary's own stamp (`identity/binary-version.txt`). The tracked diff was empty when that binary was built. The six preexisting untracked paths are listed in `identity/status-after-code.txt`. The dirty bit is not evidence of an uncommitted code edit.

The module line recorded from `go version -m` is `github.com/Fanduzi/DeltaScope v0.511.2-0.20261004063924-77624ee71fbe+dirty`.

## Red

`red/r1-before-fix.stdout` is the real parser and AuditSQL run on `c3358968` before `prepareModifyColumn` collected dependents first. Four tests failed: the TiDB primary-key signedness successor, the cross-family child, the affected-set matrix, and the inline `PRIMARY KEY` forms. The same filter passed after the production edit and is included in `GOFLAGS=-count=1 make test`.

## Standards and spec

See `reviews/standards-spec.txt`. That file is the implementer's checklist. It is not acceptance.

A decision record was required. The amendment is in `docs/decisions/2026-10-04-ddl-modify-column-state.md` and is part of `77624ee`.
