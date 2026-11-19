# PostgreSQL 17 bulk table tablespace targets

Date 2026-11-19

Status Accepted

Milestone F09 D17 on `milestone/pg-alter-semantics`, following `1a03cd0d`.

## Decision

`ddl.pg.alter.move_all_tablespace.target.valid` is a default blocker without
parameters for `ALTER TABLE ALL IN TABLESPACE` on observed PostgreSQL 17.
It checks the ordered `OWNED BY` role selections before checking source
existence, destination existence, and the shared `pg_global` restriction.
Definite rejections depend on matching catalog evidence. A missing or stale
observation yields `unproven_bulk_tablespace_targets`.

The parser preserves decoded endpoints, ordered roles and `NOWAIT`. It uses
the same RoleSpec kinds as `OWNER TO`. Bare session aliases refer to the
observed session, and quoted aliases refer to exact literal names. `PUBLIC`
is an inapplicable pseudo-role. Repetitions retain statement order. A single
read binds the named endpoints and role existence in the observed connection.
Prior unapplied DDL, unrecognized statements or an earlier parser failure
invalidate this catalog evidence.

The existing notice describes the transactional request and can report the
same observed endpoint after the tracked checks pass. Finding metadata omits
role names and catalog identifiers. Existing endpoint fields remain available.
The result leaves permission, membership and relation selection, storage
capacity, physical movement and current lock availability to separate checks.
It does not extend single-table SET TABLESPACE, ALTER INDEX or ALTER
MATERIALIZED VIEW behavior.

## Evidence plan

T37 preserves the existing 270 cases and their policy profiles. The added
native and real CLI cases bind only the original ALL IN TABLESPACE SET
TABLESPACE inventory cell. They retain isolated policy profiles, exact
catalog and file identity and independent payload and sequence assertions.

- `T37.cli.bulk-tablespace-offline`
- `T37.cli.bulk-tablespace-disabled`
- `T37.meta.pg-bulk-tablespace-missing-source`
- `T37.meta.pg-bulk-tablespace-missing-destination`
- `T37.meta.pg-bulk-tablespace-global`
- `T37.meta.pg-bulk-tablespace-global-destination`
- `T37.meta.pg-bulk-tablespace-same`
- `T37.meta.pg-bulk-tablespace-role-precedence`
- `T37.meta.pg-bulk-tablespace-public`
- `T37.meta.pg-bulk-tablespace-role-aliases`
- `T37.meta.pg-bulk-tablespace-owned-nowait`
- `T37.meta.pg-bulk-tablespace-freshness`

At `1a03cd0d`, twelve owned PostgreSQL 17.7 native probes paired with 24
real offline/live CLI audits. Native PostgreSQL returned five successes,
four `42704` and three `22023` refusals. All existing audits passed with the
bulk notice. Role validation preceded the same-space short circuit. A
separate session verified `55006` for a conflicting `NOWAIT` lock, followed
by success when the lock was released. See
`/tmp/ds-pg-f09-bulk-tablespace-edges-1a03cd0d/evidence.json` and
`/tmp/ds-pg-f09-tablespace-readonly-summary.md`.

## Sources

- [PostgreSQL 17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PostgreSQL 17.7 implementation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
