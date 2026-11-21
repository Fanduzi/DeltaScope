# PostgreSQL 17 ordinary-view column defaults

Status Accepted

`ddl.pg.alter_view.column_default.valid` checks single-action SET DEFAULT and DROP DEFAULT on an exactly qualified ordinary view. The catalog read binds the source relation kind and the exact, non-dropped user column. A proved absent view is skipped under IF EXISTS. A present relation of the wrong kind or a missing column blocks before default expression checks. SET DEFAULT reuses the existing PostgreSQL AST fact for column references and subqueries, including the distinction between expression tokens and type modifiers. The rule keeps the expression and the catalog facts out of findings.

Nine paired PostgreSQL 17.7 and CLI controls cover successful SET and DROP, quoted column mismatch, guarded absence, wrong kind, a system attribute and both disallowed references. PostgreSQL accepts the tested default for a constant view. The native fixture verifies a stored default is added or removed while the audit leaves catalog identities, row data and the guard sequence intact. Default type coercion, function resolution and future writes through the view require separate facts. Materialized-view column defaults and multi-action view changes retain their adapter boundaries.

- [PostgreSQL 17 ALTER VIEW](https://www.postgresql.org/docs/17/sql-alterview.html)
- [Native and audit controls](../../testdata/ddl-golden/T40.json)

The joint T40 preview passed 119 cases and 3,319 assertions. Eleven selectors bind the ALTER COLUMN DEFAULT PostgreSQL 17 cell.
