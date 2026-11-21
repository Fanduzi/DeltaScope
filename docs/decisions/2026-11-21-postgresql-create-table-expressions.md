# PostgreSQL CREATE TABLE expression refusals

Status Accepted

## Decision

Enable `ddl.pg.create_table.expression.valid` for bounded PostgreSQL 17 AST
refusals in an exact persistent CREATE TABLE. The rule checks column
references and subqueries in defaults, subqueries in CHECK and generation
expressions, and a generated expression that directly references a generated
column. It keeps ordinary column references in generated expressions, literal
composite field selection and array subscripts out of the refusal path.

A shared catalog lookup establishes the destination schema and relation
occupancy. PostgreSQL's `IF NOT EXISTS` guard precedes expression checks.
Missing or stale catalog facts produce an evidence gap for refusal candidates.
Temporary, inherited, typed and LIKE tables need additional identity evidence.
Earlier DDL can invalidate the destination catalog evidence.

Function resolution and volatility, CHECK coercion and column resolution,
and deeper generated-column dependencies remain separate evidence boundaries.
T36 records native refusals in those cases and the absence of a finding from
this rule. Findings expose the authored table target and a reason code.

## Verification

- [T36 expression controls](../../testdata/ddl-golden/T36.json)
- [Default subquery corpus](../../testdata/sql-corpus/postgresql/ddl/findings/create_default_subquery.expected.yaml)
- [AST tests](../../internal/infrastructure/parser/postgresql/parser_create_expression_postgresql_tag_test.go)
- [PostgreSQL 17.7 default and generation checks](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/heap.c)

The joint T36 preview passed 166 cases and 4,261 assertions. 21 references bind four exact PostgreSQL 17 cells.
