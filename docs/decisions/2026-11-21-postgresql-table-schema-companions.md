# PostgreSQL 17 SET SCHEMA companion collisions

Status Accepted

F09 D28 follows D15. PostgreSQL moves the table's row type and its associated array type before moving the table's indexes and column-owned sequences. A collision in any of those companion namespaces can prevent the move even when the destination has room for the table and its row type.

The existing namespace-destination blocker applies to one fresh, exact, persistent ordinary non-typed table on observed PostgreSQL 17. The lookup uses the array type recorded by the row type, observes live attached indexes and follows serial or identity ownership to the named table. A default expression can reference an independent sequence without moving it. The rule checks array, index and owned-sequence conflicts in the native order; absent companion facts yield `target_table.pg_namespace_companions`. Findings contain the authored table and a reason code. Companion names and OIDs stay private.

A real PostgreSQL 17.7 fixture paired eleven CLI audits with the native transaction. Native rejected five companion cases that the baseline rule accepted, covering an index, identity and serial sequence ownership, an associated array and a quoted index. The quoted case-only neighbor, same schema, clean schema and reference-only default served as successful controls. Existing table and row-type collision rejections matched. A separate real catalog query returned the expected three boolean observations on eight test pairs. The preview then passed 406/406 T37 cases with 11,083 assertions. Ten added native commands returned three successes and seven refusals. Cases cover collision precedence, stale facts and privacy. The earlier index-collision fixture now expects the blocker that matches its existing native refusal. Twelve references attach only to the PostgreSQL 17 SET SCHEMA cell. The table namespace rule remains an enabled blocker under its current rule ID.

- `/tmp/ds-pg-f09-schema-companions-a9b8065d/evidence.json`
- `/tmp/ds-pg-f09-schema-companions-query-a9b8065d/catalog-query-results.txt`
- [PostgreSQL 17 ALTER TABLE SET SCHEMA](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PG17.7 ALTER table, index and sequence namespaces](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [PG17.7 row and array type namespace change](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/typecmds.c)
