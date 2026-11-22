# PostgreSQL DROP CONSTRAINT on a partition-inherited foreign key

Status Accepted

For a single `DROP CONSTRAINT` on a qualified PostgreSQL 17 table, extend `ddl.pg.alter.drop_constraint.target.require` to a positively observed inherited foreign key on a persistent partition. The constraint catalog binds the existing inheritance marker to `pg_constraint.coninhcount > 0`. PostgreSQL 17.7 refuses a direct drop with `42P16`, including requests with `ONLY` and `IF EXISTS … CASCADE`. The audit uses `constraint_inherited` and publishes the authored target and kind. The parent identity, constraint linkage and inheritance count stay private.

The native fixture distinguishes a partition-inherited key from a local key, a root key and an unrelated ordinary key. It also verifies absence under `IF EXISTS`. Native root and root `ONLY` drops both remove the derived key in the fixture. The check covers the directly selected constraint. General cascade effects, permissions, locks, and earlier ALTER actions require separate evidence.

- [T37](../../testdata/ddl-golden/T37.json)
- [Corpus fixture](../../testdata/sql-corpus/postgresql/ddl/findings/drop_constraint_inherited_fk.expected.yaml)
- [PostgreSQL 17.7 constraint deletion](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c#L12556)

The isolated T37 preview passed 621 cases and 18,463 assertions. Nine selectors bind the PostgreSQL 17 DROP CONSTRAINT and CASCADE cells.
