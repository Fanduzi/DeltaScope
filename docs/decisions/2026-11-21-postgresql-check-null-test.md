# PostgreSQL ADD CHECK column null tests

Status Accepted

## Decision

Extend `ddl.pg.alter.add_check.name.available` to check `column IS NULL`
and `column IS NOT NULL` on a fresh, exact PostgreSQL 17 ordinary table.
The rule also checks an exact `table.column` reference. It rejects a
different relation qualifier, a proved absent column or a resolved system
column that PostgreSQL forbids in CHECK. A single identifier matching the
relation can resolve as a whole-row reference. `tableoid` is allowed.

The new checks run after the table guard and before the existing name check.
The rule reuses observed column membership and keeps unknown membership as
an evidence gap. It does not derive a failure from a NULL test on an arbitrary
expression. Whole-row star, three-part references, typed tables, inheritance
and expressions that need normal type and function resolution are outside the
new check. `NOT VALID` still permits registering over old violations while
validating the predicate at creation time.

Findings retain their existing shape. Resolved column identifiers remain
internal.

## Verification

- [T37 native controls](../../testdata/ddl-golden/T37.json)
- [Missing column corpus](../../testdata/sql-corpus/postgresql/ddl/findings/check_null_test_missing_column.expected.yaml)
- [Exact normalized identifiers](../../internal/infrastructure/parser/postgresql/parser_check_null_test_postgresql_tag_test.go)
- [PostgreSQL 17.7 column resolution](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_relation.c)

T37 passed 613 cases and 18,182 assertions in the D51 preview. Fourteen selectors bind the PostgreSQL 17 ADD CHECK and ADD CHECK NOT VALID cells.
