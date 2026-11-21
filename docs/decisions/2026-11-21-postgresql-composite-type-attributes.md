# PostgreSQL composite type declaration and attribute names

Status Accepted

## Decision

Enable `ddl.pg.type.composite.valid` for qualified PostgreSQL 17 composite
creation and single attribute actions. CREATE checks namespace and type name
occupancy before checking duplicate exact attribute names. Undefined shells
and movable automatic arrays can share the requested type name before CREATE.
Composite attributes can use names such as `ctid`.

ALTER binds the exact backing relation and requires standalone composite kind.
It checks existence for DROP, ALTER TYPE and RENAME, duplicate names for ADD
and RENAME, and the guarded absence case for DROP ATTRIBUTE. A type with no
backing relation has a distinct refusal. The parser keeps the existing
migration advice for composite declarations. Multi-action ordering requires
separate evidence.

The check covers names and relation eligibility. Type coercion, synthetic
names retained by dropped attributes, dependencies, privileges and mutation
side effects require further facts. Catalog attribute names stay internal.
Earlier DDL can invalidate the observations used by the rule.

## Verification

- [T41 native composite type controls](../../testdata/ddl-golden/T41.json)
- [Missing composite attribute corpus](../../testdata/sql-corpus/postgresql/ddl/findings/alter_composite_missing_attribute.expected.yaml)
- [Exact attribute declaration tests](../../internal/infrastructure/parser/postgresql/parser_composite_premises_postgresql_tag_test.go)
- [PostgreSQL 17.7 type declarations](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/typecmds.c)
- [PostgreSQL 17.7 attribute operations](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)

T41 preview passed 128 cases and 2,512 assertions. Twenty-two references bind two PostgreSQL 17 composite dimensions.
