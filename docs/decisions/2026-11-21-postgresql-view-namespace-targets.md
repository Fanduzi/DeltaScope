# PostgreSQL 17 view rename and schema-move targets

Status Accepted

`ddl.pg.alter_view.namespace.valid` checks qualified RENAME TO and SET SCHEMA for ordinary and materialized views on an observed PostgreSQL 17 server. The private AST fields preserve the source schema, exact name, catalog qualification and IF EXISTS guard. Source absence and relation kind are assessed before destination collisions. For a matching source, the rule uses the catalog checks already used for table namespace changes, including the destination schema, relation and row-type names, and associated array types or attached indexes when relevant. The rule reports the authored source and a bounded reason. It keeps catalog identities of the conflicting objects private.

Eleven PostgreSQL 17.7 and CLI comparisons distinguish row-type and relation collisions, same-schema moves, unavailable schemas, attached index collisions and source-guard precedence. The unqualified control proves a search-path-dependent native outcome and retains an audit gap. Earlier DDL can invalidate a source observation. Role membership, privileges, locks and downstream query effects require separate evidence.

- [PostgreSQL 17 ALTER VIEW](https://www.postgresql.org/docs/17/sql-alterview.html)
- [PostgreSQL 17 ALTER MATERIALIZED VIEW](https://www.postgresql.org/docs/17/sql-altermaterializedview.html)
- [PostgreSQL 17.7 relation namespace implementation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [T40 native and audit controls](../../testdata/ddl-golden/T40.json)

Three additional native and CLI pairs verify the associated array type. An array name collision returns `42710` and precedes an attached-index collision when both names are occupied. The combined T40 preview passed 83 cases with 2,139 assertions. Sixteen selectors bind the applicable PostgreSQL 17 cells.
