# PostgreSQL 17 ALTER TABLE CHECK predicate subqueries

Status Accepted

F09 D46 extends `ddl.pg.alter.add_check.name.available` for a single
ADD CHECK on an exactly bound, fresh PostgreSQL 17 ordinary table. The
existing parser records a private flag for a raw subquery in the CHECK
predicate. PostgreSQL refuses these predicates in named and unnamed CHECK
constraints, including NOT VALID. Predicate transformation runs before the
name-collision check. A proved absent target with IF EXISTS skips the action.
The finding identifies the authored constraint and a bounded reason.

The PostgreSQL 17.7 and CLI comparison passed seven controls. Four statements
returned `0A000`. The controls with a local column predicate, a string literal,
and a guarded absent table succeeded. The catalog, rows and guard sequence
matched before and after audit and native rollback. Evaluating the predicate,
resolving functions and proving existing row validity require separate facts.

- `/tmp/ds-pg-f09-check-expression-cli-24389415/evidence.json`
- [PostgreSQL 17.7 CHECK expression transformation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/heap.c)
- [PostgreSQL 17.7 subquery restrictions](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_expr.c)

The joint preview passed all 560 T37 cases with 16,507 assertions.
