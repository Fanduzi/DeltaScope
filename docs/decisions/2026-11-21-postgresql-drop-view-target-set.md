# PostgreSQL 17 ordered DROP VIEW targets and row-type dependencies

Status Accepted

`ddl.pg.drop_view.target.valid` checks an ordered list of schema-qualified `DROP VIEW` targets. A single catalog read binds all targets with their order and modifiers. The first missing or wrong-kind target takes precedence over dependency checks. `IF EXISTS` skips proved absence. Repeated names remain in the authored list. For `RESTRICT`, a view or materialized view selected for removal can satisfy a rewrite dependency within the set.

A table or view column outside the selected set can depend on the row type of a view. The audit checks the direct normal dependency via `pg_depend` and a live user-column slot in `pg_attribute`. The finding identifies the authored target and keeps the dependent relation, column, and catalog identifiers private. Native `CASCADE` controls verify removal of the covered external dependents. Wider cascades, dependency classes, unqualified names, and execution authorization require separate evidence.

- [PostgreSQL 17 DROP VIEW](https://www.postgresql.org/docs/17/sql-dropview.html)
- [PostgreSQL 17.7 target selection](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c#L1498)
- [T40 native and audit controls](../../testdata/ddl-golden/T40.json)

The joint T40 preview passed 179 cases and 5,354 assertions.
