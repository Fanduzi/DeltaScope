# Decision: Dependency-free single-column DROP COLUMN post-state

Date: 2026-10-04
Status: Accepted for the bounded subset below. This record does not accept T05-A6, close #84, or close the milestone.
Related milestone/version: mysql-tidb-ddl-completion (#79, T05-A6 under #84)
Related decisions: `2026-10-04-ddl-modify-column-state.md`, `2026-10-21-ddl-change-rename-column-identity.md`, `2026-10-02-ddl-ordered-schema-state-first-path.md`
Related tests: `internal/application/audit/batch_state_drop_column.go`, `internal/application/audit/batch_state_a6_test.go`, `internal/application/audit/batch_state_a6_matrix_test.go`, `internal/application/audit/batch_state_a6_postgresql_tag_test.go`, `testdata/ddl-golden/T05.json`, `scripts/ddl_golden.py`, `scripts/test_ddl_golden.py`
Related docs: `docs/reference/rules.md`, `docs/reference/rules.zh-CN.md`

## Context

`DROP COLUMN` was already extracted as `drop_column`, with `Alter.Name` and `Column.OldName` set and no new column definition. Ordered state still invalidated the whole table, so the next statement lost the columns that remained. MySQL documents automatic removal of index key parts, and TiDB 8.5 documents primary-key and composite-index restrictions. Those native outcomes are not this slice's post-state.

Enrichment still saves each statement's pre-state and derives the post-state before `EvaluateStatements`. The transition does not read the verdict.

## Decision

MySQL and TiDB publish one precise post-state for a fully audited, single-table, single-action ordinary `DROP COLUMN`. The table is known-present, the column set is non-nil, the name matches exactly one column, and at least one column remains. The target is not `AUTO_INCREMENT`, `AUTO_RANDOM`, identity, generated, or an unmodeled declaration. `PrimaryKeyUnknown`, `IndexesUnknown`, or `ConstraintsUnknown` refuses the template. A nil or empty index or constraint collection with the matching Unknown flag false is a known empty set.

The proof that a member is unrelated is stricter than MODIFY. An ordinary, unique, or primary index that names the column in `Columns` or `IncludedColumns` refuses the precise drop. So do a primary key, foreign key, CHECK, or sibling expression that cannot be proved unrelated. The precise path does not shrink an index, delete a single-column index, or rewrite a constraint. Unrelated known members stay, including `PRIMARY(id)`.

| Premise | Post-state |
|---|---|
| Present, columns known, target exists, one column remains, no related dependency | Present. Remove only the target. Relative order stays |
| Source absent, last column, table absent, or columns withheld | Target unknown. A finding already determined stays |
| Table unknown or already contaminated | Stay unknown or contaminated. Do not reread the provider |
| A related or unprovable member, or a loaded dependent that references the column | The target and those dependents become unknown together |
| Multi-action, conditional modifier, or not fully audited | Tombstone every named table identity and already-loaded identity-level dependents. An unloaded endpoint gets a tombstone |
| Only a policy blocker, and the structural gates pass | Publish the conditional successor |
| Cancellation or a provider error | Error channel. No partial post-state |

A successful drop clears the accepted MODIFY counters: primary-key and index cardinality become nil, and `data_length`, `index_length`, `avg_row_length`, and `auto_increment` are removed. `table_rows` may keep its estimate. A later same-name `ADD` uses the new definition and does not restore those estimates.

The golden profile `t05-a6-drop-column-isolated` enables six existing blockers and disables every other catalog rule, including `ddl.alter.drop_column.forbid`. No new rule, default level, or gap token is added. CLI `--schema` remains a metadata-mode switch. Offline CLI golden cases therefore record an empty request schema. Online cases, the SDK, and HTTP pass schema `golden`.

PostgreSQL stays on the legacy per-statement snapshot for the no-provider, schema-only, and provider-backed shapes.

## Rationale

MODIFY may leave an ordinary index pointing at the same column. DROP cannot, because the column will be gone and this slice does not implement the index or constraint rewrite MySQL performs. Treating "no reference found" as proof would publish a precise column list while a loaded child still names the column. Publishing that list and only marking the child unknown would leave the next statement with a false complete shape.

## Public Contract

Consumers can rely on the six existing rule IDs and their current defaults. After this subset, a later statement in the same MySQL or TiDB request sees the column removed, the remaining order unchanged, and unrelated primary keys, indexes, and constraints kept. The old-column `CREATE INDEX` blocker stays `ddl.create_index.columns.exists.require` with `schema`, `table`, `index`, `column`, and `exists: false`. CLI, HTTP, MCP, and the SDK share the same request-local state. PostgreSQL output for this statement is unchanged.

## Deferred / Out Of Scope

Related drop-column member consequences, index shrinkage, single-column index deletion, constraint rewrite, multi-action success, and resource limits stay later. Moving one column to another schema and scanning unloaded objects are not required features of this slice or of the parent contract. Cross-schema table identity protection stays in force. Storage-definition isolation is separate from semantic audit inside a storage body. This record does not complete #84 or the milestone. A1 through A5 stay accepted.

## Verification Evidence

Application tests pin the four-statement first path, the removed-column index, repeated and last-column drops, absent and withheld columns, nil versus empty members, the dependency refusals, loaded children, multi-action tombstones, same-name re-add, statistic clearing, cancellation, and provider errors. Representative SDK, CLI, HTTP, and MCP checks cover the pass and reject results. The PostgreSQL build tag keeps the three legacy request shapes. The T05 manifest keeps its previous 231 cases and adds 16 under `t05-a6-drop-column-isolated` (247 total). The four fixed anchors remain MySQL 5.7.44, 8.0.46, 8.4.10, and TiDB 8.5.0. Their raw logs live with this slice's evidence commit.

## Consequences

A non-precise DROP COLUMN, including one mixed with `RENAME` or another column action, tombstones every named identity and the loaded dependents of those identities. A precise drop does not reuse MODIFY's index-safety boolean. Disabling `ddl.alter.drop_column.exists.require` does not by itself authorize the post-state when the structural gates fail.

## Links

- Parent contract: https://github.com/Fanduzi/DeltaScope/issues/79
- Ordered state: https://github.com/Fanduzi/DeltaScope/issues/84
- MySQL 5.7 ALTER TABLE: https://docs.oracle.com/cd/E17952_01/mysql-5.7-en/alter-table.html
- MySQL 8.0 ALTER TABLE: https://docs.oracle.com/cd/E17952_01/mysql-8.0-en/alter-table.html
- MySQL 8.4 ALTER TABLE: https://docs.oracle.com/cd/E17952_01/mysql-8.4-en/alter-table.html
- TiDB release-8.5 DROP COLUMN: https://github.com/pingcap/docs/blob/release-8.5/sql-statements/sql-statement-drop-column.md
