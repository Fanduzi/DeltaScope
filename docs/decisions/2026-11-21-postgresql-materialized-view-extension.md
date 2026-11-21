# PostgreSQL 17 materialized-view extension dependencies

Status Accepted

`ddl.pg.alter_materialized_view.extension.valid` checks the exact qualified source and installed extension for DEPENDS ON EXTENSION and NO DEPENDS ON EXTENSION. The source is checked first. An absent source, a present relation of the wrong kind or an absent extension produces its own reason code. The AST preserves the extension's exact spelling and the add or remove action. The rule needs only existence because PostgreSQL accepts an already attached dependency on add and an already absent dependency on remove. Catalog identities remain private.

Eight PostgreSQL 17.7 native comparisons cover add, repeated add, remove, absent attachment, case-distinct missing extension and source precedence. The native fixtures inspect the exact automatic dependency and check that audit leaves the rows, catalog and guard sequence unchanged. Authorization, locks and the effects of a later DROP EXTENSION require separate evidence. An unqualified source requires session resolution facts.

- [PostgreSQL 17 ALTER MATERIALIZED VIEW](https://www.postgresql.org/docs/17/sql-altermaterializedview.html)
- [PostgreSQL 17.7 automatic extension dependency implementation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/alter.c#L457)
- [T40 native controls](../../testdata/ddl-golden/T40.json)

The joint T40 preview passed 147 cases and 4,224 assertions.
