# T05-A4 verification evidence — ce93dbf

## Identity and scope

- Task base: `200163813a330d775a0d6e2e526c0650318a2035` (`workfree` before this slice).
- Accepted code baseline under that base: `c82d237e543f9bef401d07f9461d9094e9405780`.
- Tested code: `ce93dbf8bec796a3b16a3f3a5f0e7fc2ed11a109`.
- Branch: `milestone/mysql-tidb-ddl-completion`. Delivery target is `origin/workfree`.
- Local `origin/main` observed as `9a28cc99c38770e6e40738515994f864166419f9`. No main merge.
- Task diff `20016381...ce93dbf`: 21 files, +5111/-31. Names are in `identity/task-diff-names.txt`. This is not a `main...HEAD` milestone-completion claim.
- The evidence commit that adds this directory is separate from the code commit. Its SHA is reported with the push, not written back into this file.
- A4 is not accepted. A1, A2, A3, #82, and #83 stay accepted. #79 and #84 stay open. No PR, tag, release, or issue close.

The slice publishes conditional post-state for one ordinary same-name `MODIFY COLUMN` of a VARCHAR or integer column. It does not change `ddl.alter.modify_column.compatibility.require`.

## What was executed

| Stage | Tree | Result |
|---|---|---|
| Plan experiment | `20016381`, `vcs.modified=true` | MySQL 8.4.10 only. Product audit of `10 → 20 → 15` was review/unverified: statement 2 had three gaps and no shrink finding. The driver then saw `varchar(10)`, `varchar(20)`, `varchar(15)`, all `utf8mb4` / `utf8mb4_bin` / `NOT NULL`, and the table was absent before and after the audit. Records are in `plan-baseline/`. |
| Pre-commit golden | dirty tree, `git rev-parse HEAD` still `20016381` | `make ddl-golden TASK=T05 ARTIFACT_DIR=/tmp/ds-t05-a4-golden`: 203/203, 3448 assertions, PASS. Log: `gates/precommit-golden.log`. Artifact: `precommit/artifact.json`. |
| Code-SHA golden | `ce93dbf` | `make ddl-golden TASK=T05 ARTIFACT_DIR=/tmp/ds-t05-a4-golden-ce93dbf`: 203/203, 3448 assertions, PASS. `python3 scripts/ddl_golden.py validate` printed `artifact valid`. |

An earlier `make ddl-golden` on the dirty tree executed 0 cases. Compose tried to create `deltascope-ddl-golden-mysql84` while a container of that name already existed under compose project `docker` (started from `docker/` during the plan round). That run is an environment conflict, not a product result. The container was removed and the pre-commit golden above is the run that executed the cases.

T02, T03, and T04 were not rerun. `t05_a4_manifest_failures` returns immediately unless `task_id` is `T05`, and the case-execution path was not changed. Their previously accepted artifacts are not recopied here.

## Frozen oracle, as run at ce93dbf

All 18 new cases passed. The original 185 T05 cases stayed in the same 203-case run.

Metadata runs use a live anchor, `--fail-on blocker`, and the four-rule profile `t05-a4-modify-isolated` (`rule_summary.loaded=4`). The product audit does not execute the user's SQL. The driver executes it and queries the column definition after every statement.

| Case shape | Anchors | Product result |
|---|---|---|
| `10 → 20 → 15` | mysql57, mysql80, mysql84, tidb85 | exit 1, complete/reject, one blocker on statement 2, `source_length=20`, `target_length=15`, no gaps |
| `10 → 20 → 30` | same four | exit 0, complete/pass, no findings or gaps |
| narrow, then 18 | mysql84, tidb85 | still one blocker, statement 2, 20→15 |
| wide, then 25 | mysql84, tidb85 | one blocker, statement 3, 30→25 |
| integer omit attributes | mysql84, tidb85 | two blockers: statement 1 drops UNSIGNED, statement 2 adds NOT NULL |
| primary-key `INT` then `BIGINT` then `BIGINT NOT NULL` | mysql84, tidb85 | exit 0, complete/pass, no findings |
| offline narrow | mysql | exit 1, reject/unverified, the CREATE gap stays, and statement 2 still reports 20→15 |
| offline wide | tidb | exit 0, review/unverified, the CREATE gap stays, no findings |

Shared tests, not the golden JSON, pin statement pre-state lengths 10 and 20 directly. Public statement JSON has no `TargetTable`.

Native empty-table changes and policy findings are different facts. On the plan-round MySQL 8.4 driver, `c` really became `varchar(10)`, then `varchar(20)`, then `varchar(15)`, with the primary key kept. The same SQL, audited by the pre-fix binary, produced no shrink finding because statement 3 no longer had a source length. After this slice the product reports the shrink while the driver still applies the length on an empty table. A policy reject does not mean the server refused the statement.

## Gates at ce93dbf

Commands ran in the root checkout at `ce93dbf` unless noted. `gates/sha-gates.log` is the combined stdout. `gates/sha-golden.log` is the golden stdout.

| Gate | Command | Result |
|---|---|---|
| golden | `make ddl-golden TASK=T05 ARTIFACT_DIR=/tmp/ds-t05-a4-golden-ce93dbf` | 203/203, 3448 assertions, PASS |
| validate | `python3 scripts/ddl_golden.py validate --artifact .../T05/artifact.json` | `artifact valid` |
| focused Go | application `TestAuditSQLT05A4\|TestBatchStateA4`; PostgreSQL tag `TestBatchStateA4PostgreSQLModifyKeepsLegacyContract`; SDK/CLI/HTTP/MCP `T05A4` | PASS |
| unit | `GOFLAGS=-count=1 make test` | PASS |
| pg unit | `make pg-unit-test-gates` | PASS |
| corpus | `make sql-corpus-gates` | PASS |
| inventory | `make ddl-inventory-gate` | PASS |
| coverage catalog | `make ddl-coverage-catalog-test` | PASS |
| validator | `make ddl-golden-validator-test` | `contract cases=152 failures=0` |
| docs examples | `make docs-example-gates` | PASS |
| decision, task range | `./scripts/check_decision_record.sh 20016381...ce93dbf` | decision file present, PASS |
| decision, makefile | `make decision-record-gate` | exit 0, see the note in `reviews/standards-spec.txt` |
| three-level | staged replay of the 21-file diff | OK, L1 reminder, root READMEs unchanged |
| whitespace | `git diff --check 20016381...ce93dbf` | clean for the code commit |

`vcs.modified=true` is the built binary's own stamp (`identity/binary-version.txt`). The tracked diff was empty when that binary was built. The six preexisting untracked paths are listed in `identity/status-after-code.txt`. The dirty bit is not evidence of an uncommitted code edit.

## Red, and what was left out

`red/first-path-before-post-state.json` is the go test log from before `applyModifyColumn`. `TestAuditSQLT05A4ModifyNarrowThirdStatement` failed because statement 2 was unverified, with `missing_source_column` plus two `unknown_table_state` gaps, and no finding. `plan-baseline/summary.json` is the same pre-fix shape from the MySQL 8.4 CLI.

`plan-baseline/` does not contain the 32MB `deltascope` binary or the two upstream manual HTML copies. Their SHA-256 values are in `identity/omitted-plan-files.txt`. Column-probe stdout files keep the server's trailing whitespace. `git diff --check` on this evidence directory flags those lines; the bytes were not trimmed.

PostgreSQL reads `public.t` once per request and reuses that snapshot. The guard asserts the second statement's pre-state length is still 10. It does not expect a second provider read.

## Standards and spec

See `reviews/standards-spec.txt`. That file is the implementer's checklist. It is not acceptance.

Out of the precise template, and still conservative: `CHANGE`/`RENAME COLUMN`, `DROP COLUMN`, multi-action `ALTER`, cross-family changes, `DECIMAL`, temporal implicit defaults, generated or identity columns, and TiDB primary-key reorganization limits.
