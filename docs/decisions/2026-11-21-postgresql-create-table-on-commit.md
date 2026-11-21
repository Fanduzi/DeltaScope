# PostgreSQL 17 CREATE TABLE persistence and ON COMMIT

Status Accepted

`ddl.pg.create_table.on_commit.valid` checks CREATE TABLE statements that author an ON COMMIT action or declare TEMPORARY with an explicit schema. In a literal persistent namespace, temporary creation is refused before the `IF NOT EXISTS` skip. An absent schema also blocks before ON COMMIT validation. With an existing destination and a usable persistence declaration, the guard skips ON COMMIT validation. Permanent and unlogged tables cannot otherwise use ON COMMIT in that namespace.

Unqualified TEMP uses the session's temporary namespace. The audit recognizes the declaration and leaves runtime commit effects to the server. `pg_temp`, other session namespace aliases, and unqualified permanent names need separate resolution evidence. PostgreSQL accepts `GLOBAL TEMPORARY` with a deprecation warning. The parser retains effective persistence and never treats GLOBAL as a separate kind of temporary storage.

Native controls check row preservation, row deletion, and relation removal in the same database session after an explicit commit. Earlier DDL can invalidate the catalog evidence used by this rule.

- [PostgreSQL 17 CREATE TABLE](https://www.postgresql.org/docs/17/sql-createtable.html)
- [PostgreSQL 17.7 creation namespace adjustment](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/namespace.c#L846)
- [T36 controls](../../testdata/ddl-golden/T36.json)

The joint T36 preview passed 88 cases and 1,608 assertions on `a83a1710`.
