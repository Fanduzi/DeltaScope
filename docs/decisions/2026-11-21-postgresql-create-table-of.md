# PostgreSQL CREATE TABLE OF source types

Status Accepted

## Decision

Enable `ddl.pg.create_table.of.valid` as a blocker for PostgreSQL 17. The rule
binds a qualified persistent creation target to a qualified standalone composite
type. It checks destination schema presence and the `IF NOT EXISTS` guard before
checking the type namespace, existence, shell status and relation kind. Column
options must name live attributes by exact identifier. Repeated options for a
single attribute are refused.

PostgreSQL looks up the source type only after the destination guard. The source
must have `pg_type.typtype = 'c'` and a backing `pg_class` row with `relkind = 'c'`.
This separates a standalone composite type from the row type of a table or view.
The lookup reads only catalog facts. Findings carry the authored destination and
reason code. Resolved type identities and attribute names stay internal.

A temporary target, an unqualified or session-dependent namespace, stale facts,
and missing catalog evidence produce an evidence gap. The rule checks column
option names. Default expressions, constraints, permissions and execution locks
require their own evidence. The array suffix in `CREATE TABLE d.t OF d.c[]`
returns SQLSTATE `42601` in PostgreSQL 17.7 and a parser diagnostic in DeltaScope.

## Verification

- [T36 CREATE TABLE controls](../../testdata/ddl-golden/T36.json)
- [Table row type refusal](../../testdata/sql-corpus/postgresql/ddl/findings/create_of_table_row_type.expected.yaml)
- [Exact AST names and option order](../../internal/infrastructure/parser/postgresql/parser_create_typed_postgresql_tag_test.go)
- [Rule assessment](../../internal/domain/rule/ddl/postgresql_create_typed_rules_test.go)
- [PostgreSQL 17.7 creation transform](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_utilcmd.c)
- [PostgreSQL 17.7 type and attribute checks](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)

The joint T36 preview passed 107 cases and 2,256 assertions.
