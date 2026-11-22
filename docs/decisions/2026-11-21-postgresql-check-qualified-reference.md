# PostgreSQL CHECK null tests on qualified references

Status Accepted

## Decision

Extend `ddl.pg.alter.add_check.name.available` to simple `IS NULL` and `IS NOT NULL` predicates with a qualified column or whole-row `*`. The parser preserves each exact identifier. The rule checks the qualified schema and relation against the bound ordinary table. An authored database component requires an observed current database. `tableoid` remains valid and other system columns retain their native refusal.

PostgreSQL can interpret a missing qualified field as a function call or cast on the entire row. The catalog query checks whether any routine or type has that exact name. If neither exists, the rule can report the missing column. A possible fallback or missing catalog witness leaves an evidence gap. The query does not evaluate routines, casts, search-path precedence or row values. Catalog facts and predicate identifiers stay outside the finding payload.

The scope remains single-action ALTER TABLE ADD CHECK on a qualified, fresh ordinary table. Built-in scalar coercion, arbitrary expressions, inheritance, and data checks retain their existing contracts. `NOT VALID` still checks the declaration before a constraint-name collision.

## Verification

- [T37 qualified CHECK controls](../../testdata/ddl-golden/T37.json)
- [Wrong schema corpus](../../testdata/sql-corpus/postgresql/ddl/findings/check_null_test_wrong_schema.expected.yaml)
- [PostgreSQL 17.7 column-reference transformation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_expr.c#L675-L800)
- [PostgreSQL 17.7 function and cast resolution](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_func.c)

The joint T37 preview passed 650 cases and 19,523 assertions, including all 18 controls for this decision.
