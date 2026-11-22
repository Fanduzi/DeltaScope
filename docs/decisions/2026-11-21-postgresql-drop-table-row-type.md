# PostgreSQL DROP TABLE row-type column dependencies

Status Accepted

## Decision

Extend the PostgreSQL 17 `ddl.table.drop.exists.require` check for an exact set of ordinary persistent DROP TABLE targets. A separate relation's live user column can depend on a selected table's row type or its generated array type. Under RESTRICT, a normal catalog dependency of either kind blocks the drop. A relation selected in the same DROP statement does not count as an external dependent. Previously dropped attributes do not count. Existing target refusals precede dependency evaluation.

The rule uses the existing `drop_table_has_direct_dependency` reason and reports the authored table identity. Column names and referenced catalog identities stay private. CASCADE is covered by a native effect control. General dependency closure, domain and routine dependents, partition or inheritance sets, and execution privileges retain their boundaries.

## Verification

- [T38 native row-type DROP controls](../../testdata/ddl-golden/T38.json)
- [DROP TABLE dependency corpus](../../testdata/sql-corpus/postgresql/ddl/findings/drop_table_direct_references.expected.yaml)
- [PostgreSQL 17 dependency tracking](https://www.postgresql.org/docs/17/ddl-depend.html)

T38 passed 70 cases and 2,046 assertions in the row-type preview. Eight selectors bind the two PostgreSQL 17 DROP TABLE cells.
