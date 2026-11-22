# PostgreSQL DROP VIEW row-type array dependencies

Status Accepted

## Decision

Extend the existing PostgreSQL 17 DROP VIEW target checks to columns that directly use a selected view's generated row-type array. Read the array identity from `pg_type.typarray` and require a normal dependency to a live user column. For a multi-target DROP, exclude dependent views selected in the same set. Existing target failures precede dependency checks.

The finding retains `drop_view_has_row_type_dependency`. Catalog column names and dependent identities stay private. The native CASCADE case verifies that PostgreSQL removes the array-typed column while the containing table and row remain. General dependency closure and execution prerequisites remain outside this check.

## Verification

- [T40 row-type array controls](../../testdata/ddl-golden/T40.json)
- [PostgreSQL 17 DROP VIEW](https://www.postgresql.org/docs/17/sql-dropview.html)

T40 passed 187 cases and 5,650 assertions in the row-type preview. Eight selectors bind the two PostgreSQL 17 DROP VIEW cells.
