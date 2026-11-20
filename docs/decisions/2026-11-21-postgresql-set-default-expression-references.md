# PostgreSQL 17 SET DEFAULT expression references

Status Accepted

F09 D40 extends `ddl.pg.alter.column.generation.transition.valid` when an
observed PostgreSQL 17 ordinary column is eligible for SET DEFAULT. PostgreSQL
refuses a raw column reference or a subquery in this expression. The extractor
keeps a private boolean and walks the existing parser AST. A type-name subtree
can contain a legal type modifier written as an identifier, so the walker skips
that subtree. An absent, system, identity or generated column keeps its prior
target assessment. A missing column catalog keeps the existing evidence gap.

Eleven PostgreSQL 17.7 native transactions and CLI audits reproduce four
reference refusals (`0A000`), two generation conflicts (`42601`), one missing
column (`42703`) and four accepted controls. The accepted controls include a
string literal, a schema-qualified scalar function, a quoted numeric type
modifier and DROP DEFAULT. The audit preserves catalog, row and sequence state.
The rule reports the authored target and a reason code. The expression's type
assignment, callable identity, privileges and runtime evaluation require other
evidence.

- `/tmp/ds-pg-f09-set-default-cli-v2-683d84ac/evidence.json`
- [PostgreSQL 17.7 expression transformation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_expr.c)
- [PostgreSQL 17.7 type modifier handling](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_type.c)

The joint T37 preview passed 535 cases with 15,589 assertions.
