# PostgreSQL 17 CREATE INDEX INCLUDE and NULLS NOT DISTINCT

Status Accepted

`ddl.pg.create_index.include.valid` checks qualified PostgreSQL 17 relations and the exact user-column names in an INCLUDE list. It rejects an absent or wrong-kind target, included columns missing from the observed directory, and observed system attributes. Native controls confirm that B-tree accepts an included column that also appears in the key and accepts repeated included columns. Hash and GIN reject INCLUDE. A partitioned relation requires partition-key evidence before this rule can reach a conclusion.

The parser preserves the `NULLS NOT DISTINCT` flag on unique and nonunique PostgreSQL index declarations. PostgreSQL 17.7 stores the flag in both cases. Plain-key comparisons exclude declarations with the flag because the existing index model does not establish equivalent null semantics. The INCLUDE check leaves expression evaluation, predicate behavior, partitioned-index rules and access methods beyond the tested set open. It keeps catalog facts private.

- [PostgreSQL 17 CREATE INDEX](https://www.postgresql.org/docs/17/sql-createindex.html)
- [PostgreSQL 17.7 index definition checks](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/indexcmds.c)
- [T39 cases](../../testdata/ddl-golden/T39.json)

The joint T39 preview passed all 130 cases and 2,948 assertions at base `d4cf7b6c`.
