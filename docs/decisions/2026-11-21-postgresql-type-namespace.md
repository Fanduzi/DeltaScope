# PostgreSQL ALTER TYPE rename and schema destinations

Status Accepted

## Decision

Enable `ddl.pg.alter_type.change.target.valid` for qualified PostgreSQL 17 type
rename and schema move. The rule binds the exact source type, its backing
relation and the declared destination. It distinguishes undefined shells,
table row types and direct array changes from standalone type changes.

Rename checks composite relation names before type names. PostgreSQL can move
an automatic array out of a rename destination. A shell occupies a rename
destination. Schema moves check the destination schema, TOAST or temporary
namespace restrictions, exact type and composite relation occupancy and an
associated array name. Moving a standalone source to its current schema
succeeds. Renaming to the current name can fail.

OWNER TO resolves the authored role before source type eligibility. It accepts
shell types and rejects table row types and arrays. PUBLIC receives its observed
role refusal. CURRENT_USER, CURRENT_ROLE and SESSION_USER use observed session
roles. Broader type dependency moves, session namespaces and execution
permissions require further facts. Findings expose the authored type and reason
code. Catalog roles and associated type names stay internal.

## Verification

- [T41 native type change controls](../../testdata/ddl-golden/T41.json)
- [Occupied composite rename corpus](../../testdata/sql-corpus/postgresql/ddl/findings/alter_type_rename_occupied.expected.yaml)
- [Exact target extraction](../../internal/infrastructure/parser/postgresql/parser_type_change_postgresql_tag_test.go)
- [PostgreSQL 17.7 type changes](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/typecmds.c)

T41 preview passed 160 cases and 3,914 assertions. The namespace and owner cell has 33 exact references.
