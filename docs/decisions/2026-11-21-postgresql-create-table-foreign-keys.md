# PostgreSQL CREATE TABLE foreign-key declarations

Status Accepted

## Decision

Enable `ddl.pg.create_table.foreign_key.valid` for PostgreSQL 17. The rule
checks direct foreign keys in a qualified persistent CREATE TABLE. It
preserves inline and table-level declarations in AST order, including explicit
referenced and `ON DELETE SET` column names.

After the destination namespace and guard are established, the auditor reuses
the existing foreign-key checks for the referenced relation, exact local and
remote member names, eligible primary or unique keys and key width. It checks
`ON DELETE SET NULL` and `SET DEFAULT` column membership. A partial unique
index does not establish a usable referenced key. A protected destination that
already exists skips reference lookup.

Creation-time equality and coercion depend on resolving new column types.
Self-references depend on prospective key facts. These cases report evidence
gaps. The rule also requires separate evidence for session namespaces, typed
or inherited columns, partitioning and authorization. Findings carry the
authored destination and a reason code. Catalog key and member names remain
internal.

## Verification

- [T36 foreign-key controls](../../testdata/ddl-golden/T36.json)
- [Absent reference corpus](../../testdata/sql-corpus/postgresql/ddl/findings/create_foreign_key_missing_reference.expected.yaml)
- [PostgreSQL 17.7 foreign-key checks](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)

The joint T36 preview passed 183 cases and 4,845 assertions. 20 references bind two exact PostgreSQL 17 cells.
