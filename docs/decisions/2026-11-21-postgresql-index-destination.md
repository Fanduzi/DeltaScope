# PostgreSQL 17 ALTER INDEX rename and tablespace destinations

Status Accepted

`ddl.pg.alter_index.destination.valid` binds a qualified source and the exact destination of `RENAME TO` or a single `SET TABLESPACE`. A proved missing source is skipped under `IF EXISTS`. A present source still goes through the action-specific checks.

PostgreSQL allows `ALTER INDEX … RENAME` to rename some non-index relations. The audit rejects composite types and checks occupancy in the relation namespace, including a rename to the current name. An associated row type or constraint leaves a compatibility gap after an available relation slot. `SET TABLESPACE` requires an ordinary or partitioned index. The audit checks destination existence, then accepts the current tablespace and checks mapped storage and the global tablespace. Ownership and storage outcomes require separate evidence.

The 15 native and CLI comparisons observe quoted source names, a collision with a table, native table rename through index syntax, guarded source absence, and missing or inadmissible tablespaces. The audit keeps companion identities and catalog identifiers private.

- [PostgreSQL 17 ALTER INDEX](https://www.postgresql.org/docs/17/sql-alterindex.html)
- [PostgreSQL 17.7 rename and tablespace checks](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [T39 native and audit controls](../../testdata/ddl-golden/T39.json)

T39 passed 162 cases and 4,005 assertions in the joint preview. Seventeen evidence selectors bind three ALTER INDEX cells for PostgreSQL 17.
