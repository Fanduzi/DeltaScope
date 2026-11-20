# PostgreSQL 17 direct DROP COLUMN dependencies

Status Accepted for F09 D27

The existing `ddl.alter.drop_column.exists.require` checks the exact PostgreSQL column and honors IF EXISTS on proved absence. PostgreSQL also refuses RESTRICT when a view, materialized view, stored generated column, or foreign key still depends on an existing column. The default remains the existing blocker. The scope is one fresh, exact schema-qualified DROP COLUMN on a PostgreSQL 17 persistent ordinary non-typed table with no inheritance or partitions.

Use the target column's positive `attnum` in normal `pg_depend` edges. Match view and materialized-view rewrite owners and match a generated expression to its live, distinct stored column. The independent foreign-key branch checks a bound referenced-column normal edge. A self-referential foreign key with the same column on its referencing side has an automatic removal path and supplies no blocker. A self-reference from a different column can block. Dependent names, object IDs and expression text stay private. Missing, unbound or stale evidence for the applicable catalog premise leaves `column.pg_dependencies` under `unproven_column_target`. CASCADE, automatic local index/CHECK deletion, indirect or whole-row dependencies and runtime permission and lock conditions retain separate semantics.

The baseline probes paired eleven clauses with native PostgreSQL 17.7 execution in rolled-back transactions. They distinguish view, materialized-view, generated-column and external-FK SQLSTATE `2BP01` from CASCADE, guarded absent, local CHECK and local index controls. A second set of native probes compared four external and self-referential foreign-key shapes. It observed two refusals and two successes. The fixture receipts checked exact catalog and file identity, unchanged rows and an independent sequence. The actual provider query reproduced the classified edges.

- `/tmp/ds-pg-d27-overlay-cli-v2-a9b8065d/evidence.json`
- `/tmp/ds-pg-d27-foreign-key-query-a9b8065d/query-results.json`
- [PostgreSQL 17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PG17.7 DROP COLUMN](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [PG17.7 dependency reporting](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/dependency.c)

## Canonical evidence

The T37 preview passed 409/409 with 11,137 assertions. Thirteen added
native commands returned six successes and seven `2BP01` refusals.
Fifteen selectors bind these controls and the corpus case only to the
PostgreSQL 17 DROP COLUMN cell. The old tuning batch preserves its SQL
and adds the exact stale dependency gap at statement index 11.
