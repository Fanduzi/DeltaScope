# Decision: Conditional post-state for ordinary single-column MODIFY

Date: 2026-10-04
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (#79, T05-A4 under #84)
Related decisions: `2026-10-02-ddl-ordered-schema-state-first-path.md`, `2026-10-03-ddl-drop-recreate-state.md`, `2026-10-20-ddl-rename-table-identity-migration.md`
Related tests: `internal/application/audit/batch_state_a4_test.go`, `internal/application/audit/batch_state_a4_matrix_test.go`, `internal/application/audit/batch_state_a4_r1_test.go`, `internal/application/audit/batch_state_modify.go`, `testdata/ddl-golden/T05.json`, `scripts/test_ddl_golden.py`
Related docs: `internal/application/audit/README.md`

## Context

An ordinary `ALTER TABLE ... MODIFY COLUMN` was already fully audited, then
`applyAlterTable` invalidated the table because the action was not `add_columns`.
The next statement therefore saw an unknown table and could not use the length
the previous MODIFY had declared. The accepted compatibility rule already emits
a shrink blocker when both lengths are known. This slice supplies that state.
It does not change the rule's meaning, reason codes, or gap thresholds.

## Decision

Only MySQL and TiDB publish a post-state, and only for one ordinary
`modify_column` whose target identity matches the statement. The new column
definition replaces that column in place. Other ordinary columns, their order,
and same-name ordinary indexes stay. Unknown is not restored to present, and a
prior invalidation is not reread from the provider.

| Pre-state | Post-state |
|---|---|
| Table present, column set known, target column present | Replace that column on a deep copy |
| Same, but the old type or length is unknown | Keep this statement's gap; a sufficient new definition is visible to the next statement |
| Column set known, target column confirmed absent | Target becomes unknown; this statement's findings and gaps stay |
| Table present, column set withheld | Existence stays; do not invent a one-column table |
| Table absent, unknown, or batch-contaminated | Absent becomes unknown; unknown and contamination stay |
| Multiple actions, conditional modifiers, position clauses, or an unaudited aspect | Conservative invalidation, with no partial success |
| Policy blocker only, structural premise holds | Still publish the conditional definition |

Omitted attributes are not copied from the old column. `HasDefault=false` still
means the declaration has no DEFAULT clause. An existing primary key, whether
recorded on `PrimaryKey.Columns` or as a `primary_key`/`primary` constraint,
keeps `NotNull=true` when the new declaration does not use
`AlterColumn.Change.TouchesNullability`. Explicit `NOT NULL` stays true.
Explicit `NULL` on a known primary key does not publish a nullable key. Charset
and collation are taken from the new declaration. Table defaults apply only
when both are omitted and the table options agree; a one-sided declaration does
not borrow the other side, the old column, or a server default.

Loaded dependents that name the column and cannot be recomputed — foreign keys,
checks, generated or identity columns, and prefix or special indexes — are
invalidated together with the target before publication. Ordinary primary-key
and secondary index names stay, but their cardinalities become nil. Storage and
auto-increment counters are dropped. `table_rows` keeps its previous estimate
and is not promoted to an exact count. Only entries already loaded in this
request are scanned.

The precise template is ordinary same-name `VARCHAR` and integer conversions
whose new definition is complete. Cross-family changes, `DECIMAL`, temporal
defaults, generated columns, and TiDB-only reorganization limits stay on the
existing conservative path.

## Rationale

Copying the ADD rule's unknown-to-present derivation would invent a table the
batch has not established. Keeping an old generated type beside an unknown
constraint would publish a fact the model cannot maintain. Primary-key nullability
is a structural requirement of the key, not a leftover column flag, and the
parser already records whether nullability was written. Publishing the new
definition after a policy blocker matches the accepted conditional-state
contract: the finding describes this statement's pre-state, and a later
statement may use the modeled successor.

## Public Contract

- `VARCHAR(10)` then `VARCHAR(20)` then `VARCHAR(15)`: the third statement has
  exactly one `ddl.alter.modify_column.compatibility.require` blocker with
  `source_length=20` and `target_length=15`. Aggregate coverage is complete and
  the verdict is reject. The provider reads the absent table once.
- `VARCHAR(10)` then `VARCHAR(20)` then `VARCHAR(30)`: three complete statements,
  no findings or gaps, verdict pass.
- A later `VARCHAR(18)` after the shrink still has only that one blocker and
  reads length 15. A later `VARCHAR(25)` after the widen reports 30 to 25.
- A policy blocker does not erase the published definition. Disabling the
  compatibility rule, or setting `required: false`, removes that rule's gap
  and does not block a sufficient transfer.
- PostgreSQL `ALTER COLUMN ... TYPE` keeps its previous result shape.
- SDK gap-only and reject results return nil error. HTTP returns 200 for those
  results and 400 for empty SQL. MCP `isError` is false for the reject and the
  gap, and true for an unknown dialect. Offline CLI `--fail-on blocker` exits 1
  for the shrink and 0 for the widen.

## Verification

Application tests assert the second statement's pre-state length is 10 and the
third statement's pre-state length is 20 directly. Public SDK and representative
CLI, HTTP, and MCP checks pin the lengths they can see. The T05 manifest keeps
its original 185 cases and adds 18 under `t05-a4-modify-isolated`: four anchors
for the narrow and wide sequences, mysql84 and tidb85 continuations, integer
attribute replacement, primary-key nullability, and one offline narrow plus one
offline wide case. Each metadata case confirms `t` is absent before and after
the product audit, then has an independent driver query the column after every
MODIFY. The validator recomputes those oracles from the manifest and raw stdout.

## Amendment (T05-A4-R1)

Three boundaries of the original decision were not met by the first publication.

The affected set is collected from the immutable pre-state before the statement chooses replacement or invalidation. Every non-precise branch, including a conditional, positional, or multi-action MODIFY, drops already-loaded dependents that reference the named column. A withheld column set still keeps the target's other known members. A self-foreign-key is unsafe when either its local columns or its referenced columns involve the column. `UnmodeledParts` and `UnmodeledReferencedParts` keep a non-empty name list from proving that a constraint is unrelated. A complete list that excludes the column, and a same-named table in another schema, stay. Unloaded tables are not discovered.

An inline `PRIMARY KEY` is `AlterColumnChange.DeclaresPrimaryKey`, set by the parser when the new definition writes that option. `PRIMARY KEY`, `PRIMARY KEY NOT NULL`, `NOT NULL PRIMARY KEY`, and `PRIMARY KEY NULL` are outside the ordinary template. The post-state is conservative invalidation, not a known-absent primary key, and this slice still does not implement add-primary-key. Ordinary `NOT NULL` without `PRIMARY KEY` stays inside the template. The fact is `json:"-"` and does not change the public result schema.

On TiDB 8.5, a known integer primary key whose signedness changes is not a precise successor. The signedness finding still describes the pre-state. `INT PRIMARY KEY` to `BIGINT` remains a published widening, as does the same signedness change on MySQL and on a TiDB column that is not a known primary key. This follows the release-8.5 MODIFY compatibility note (verified 2026-10-04): `INT PRIMARY KEY` to `INT UNSIGNED` is `ERROR 8200`, and `INT PRIMARY KEY` to `BIGINT` is allowed. No reorg implementation is added. The original 18 manifest cases stay. `t05-a4-tidb85-pk-unsigned` adds the native refusal on the existing tidb85 anchor: the driver records `ERROR 8200` for the signedness statement and does not send the following statement to the server.

## Deferred Scope

No `CHANGE COLUMN`, `RENAME COLUMN`, `DROP COLUMN`, multi-action ALTER,
cross-family conversion, decimal precision, temporal implicit defaults,
generated or identity columns, or prefix-length rewriting. This slice does not
complete #84 or the milestone. A1, A2, and A3 stay accepted.

## References

- Parent contract: https://github.com/Fanduzi/DeltaScope/issues/79
- Ordered state: https://github.com/Fanduzi/DeltaScope/issues/84
- MySQL 8.4 ALTER TABLE: https://dev.mysql.com/doc/refman/8.4/en/alter-table.html
- MySQL column charset: https://dev.mysql.com/doc/refman/8.4/en/charset-column.html
- TiDB 8.5 MODIFY: https://raw.githubusercontent.com/pingcap/docs/release-8.5/sql-statements/sql-statement-modify-column.md
