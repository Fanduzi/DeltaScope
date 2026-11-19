# Decision: PostgreSQL single-table tablespace destination

Date: 2026-11-18
Status: Accepted
Milestone: F09 / `milestone/pg-alter-semantics` (D11)
Product baseline: `61cb93b1c69840281d75f5ee4f58e0ddf1f996b0`

## Decision

Enable `ddl.pg.alter.tablespace.target.valid` at blocker level without
parameters. Check `ALTER TABLE … SET TABLESPACE` for an observed PostgreSQL
17 server and a fresh, exact schema-qualified ordinary persistent non-typed,
nonshared and nonmapped table. A qualified absent table protected by
`IF EXISTS` has no move premise. Earlier DDL or parser failures, unqualified
or three-part bindings, multiple actions and other table kinds stay unverified.

A single exact catalog lookup observes the named destination and current
location of the bound table in this database. `pg_class.reltablespace = 0`
resolves through the actual database default. An observed absent destination
or a nonshared table's observed `pg_global` destination rejects. The
`unproven_table_tablespace` gap reports missing attachment facts. A confirmed
same location retains the existing notice with explicit no-copy wording.
Other notices describe the request and conditional physical effects.

The lookup and comparison stay private. New findings carry bounded reasons
and the affected table identity. Existing tablespace notice metadata retains
its public shape. This scope covers destination eligibility and same-space
physical behavior. Privileges, locking, disk capacity, actual execution and
`ALTER TABLE ALL IN TABLESPACE` remain separate dimensions.

## Baseline evidence

The PG17.7 paired probe at `6efd1dce` ran 13 native statements and 26 CLI
audits. Five native refusals still received the existing notices and audit
pass. Native single-table controls distinguish missing destination `42704`,
`pg_global` `22023`, same-space physical no-op, actual file relocation and
partition-root catalog changes. Catalog, file, row and guard checks are
independent of audit. The bulk owner, missing endpoint and NOWAIT results
remain a separate history at `/tmp/ds-pg-f09-tablespace-readonly-summary.md`.
The paired evidence is `/tmp/ds-pg-f09-tablespace-cli-6efd1dce/evidence.json`.

## Frozen T37 additions

- `T37.cli.table-tablespace-offline`
- `T37.cli.table-tablespace-disabled`
- `T37.meta.pg-table-tablespace-missing`
- `T37.meta.pg-table-tablespace-global`
- `T37.meta.pg-table-tablespace-same-default`
- `T37.meta.pg-table-tablespace-same-named`
- `T37.meta.pg-table-tablespace-move`
- `T37.meta.pg-table-tablespace-scope`
- `T37.meta.pg-table-tablespace-freshness`
- `T37.meta.pg-table-tablespace-guard-absent`

Keep existing cases and profiles. Real CLI audit must preserve rows, files,
current tablespace and an independent sequence before the native leg.
The matrix binds only the original `SET TABLESPACE` evidence cell.

## Evidence bindings

`34ec33f6` binds the private observation and `c319179e` adds the shared rule.
T37 retains all 203 prior cases and profiles. Ten additions under an isolated
profile bring the focused run to 213/213 and 4655 assertions. The eight live
cases include eleven native controls (eight successes and three precise
refusals). The first replay found two unsubstituted expected space names.
Correcting the expected fixture names preserved every native check.

An independent real CLI and native probe used a database created with a custom
default tablespace. Its heap had zero stored `reltablespace`; selecting the
actual database default produced the same-space notice and preserved the
original file, payload and sequence before and after native execution.
See `/tmp/ds-pg-d11-custom-db-default-c319179e/evidence.json`.

Eleven selectors bind the original `SET TABLESPACE` cell. The generated
matrix retains all row and subaction classifications and denominators.

## Sources

- [PostgreSQL 17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PostgreSQL 17 pg_class](https://www.postgresql.org/docs/17/catalog-pg-class.html)
- [PostgreSQL 17 pg_database](https://www.postgresql.org/docs/17/catalog-pg-database.html)
- [PostgreSQL 17 pg_tablespace](https://www.postgresql.org/docs/17/catalog-pg-tablespace.html)
- [PostgreSQL 17.7 table move checks](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
