# PostgreSQL CREATE TABLE columns and exact type targets

Status Accepted

## Decision

Enable `ddl.pg.create_table.columns.valid` for direct PostgreSQL 17 column
lists. It checks duplicate exact names and conflicts with system attributes.
For the observed qualified destination, `IF NOT EXISTS` is applied before
column checks. Quoted spelling is preserved.

The type check reuses the ALTER COLUMN type directory for exact qualified
types, including array lookup via `pg_type.typarray`. Missing schemas and
types, undefined shells and unsupported pseudo-types are concrete refusals.
The commonly used built-in types recognized by PostgreSQL grammar and `text`
keep their existing path. Other qualified types require observed facts. User
catalog names and OIDs stay internal.

Temporary, typed, inherited and LIKE-derived column shapes require their own
facts. Unqualified and session-resolved type names, type modifiers, arbitrary
domain or composite dependencies, privileges and runtime casts are outside
this check. Earlier DDL can make the catalog evidence stale.

## Verification

- [T36 native column and type controls](../../testdata/ddl-golden/T36.json)
- [System attribute conflict corpus](../../testdata/sql-corpus/postgresql/ddl/findings/create_column_system_name.expected.yaml)
- [Exact column type AST targets](../../internal/infrastructure/parser/postgresql/parser_create_columns_postgresql_tag_test.go)
- [PostgreSQL 17.7 attribute validation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/heap.c)

The joint T36 preview passed 214 cases and 5,903 assertions. 18 references bind three exact PostgreSQL 17 column and guard cells.
