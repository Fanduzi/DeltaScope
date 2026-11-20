# PostgreSQL 17 ALTER COLUMN TYPE direct dependencies

Status Accepted

D34 extends `ddl.pg.alter.column_type.destination.valid` on a fresh exact ordinary non-typed relation and a proved source column. The existing catalog lookup binds the positive source attribute to ordinary view or materialized-view rewrite dependencies and to other stored generated columns. A direct foreign key on the column has its own TYPE semantics and gives no refusal for this premise. A failed or stale applicable read produces `column.pg_type_dependencies`. The audit publishes the authored target and a bounded reason while keeping dependent identities private.

Native PostgreSQL 17.7 and the real CLI agreed on four view, materialized-view and stored generated-column refusals and two successful widenings with a foreign key or a local index. A same-type request can still be refused. The USING clause is preserved, and this blocker describes catalog dependencies. Other dependency classes, cast expressions, defaults and execution behavior require separate facts.

- `/tmp/ds-pg-f09-type-dependency-cli-697dfbd9/evidence.json`

- [PostgreSQL 17.7 ALTER table implementation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)

The corrected combined T37 preview passed 481/481 with 13,697 assertions. Its 22 new native commands returned five successes and seventeen refusals. The `exists` field on the ADD findings describes the observed column presence.
