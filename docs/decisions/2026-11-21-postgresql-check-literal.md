# PostgreSQL 17 literal Boolean coercion in ADD CHECK

Status Accepted

The existing `ddl.pg.alter.add_check.name.available` rule checks two additional predicate shapes in single-action ADD CHECK on a freshly bound PostgreSQL 17 ordinary table. A malformed untyped string literal is refused using PostgreSQL's Boolean input syntax, including its ASCII whitespace handling. A plain int4 literal is refused when the observed catalog has no implicit or assignment cast to Boolean. The presence of such a cast leaves a result evidence gap. Larger integers, numeric forms, bare columns and other expressions require separate type or evaluation facts.

Target binding and the `IF EXISTS` guard precede predicate analysis. The bounded native comparison verifies that predicate coercion can fail before an occupied constraint name and that `NOT VALID` still checks predicate type. The parser retains a classification and the provider returns one cast eligibility fact. Findings keep the literal and catalog facts private and preserve the original SQL contract.

- [PostgreSQL 17 CHECK constraints](https://www.postgresql.org/docs/17/ddl-constraints.html)
- [PostgreSQL 17.7 assignment coercion](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_coerce.c#L1161)
- [PostgreSQL 17.7 Boolean input](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/utils/adt/bool.c#L123)
- [T37 controls](../../testdata/ddl-golden/T37.json)

T37 passed 587 cases and 17,350 assertions in the D50 preview. Twelve selectors bind the PostgreSQL 17 ADD CHECK and ADD CHECK NOT VALID cells.
