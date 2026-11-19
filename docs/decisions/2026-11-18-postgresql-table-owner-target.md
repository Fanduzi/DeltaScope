# Decision: PostgreSQL ordinary-table OWNER TO target

Date: 2026-11-18
Status: Accepted
Milestone: F09 / `milestone/pg-alter-semantics` (D16)
Product baseline: `a303a8b414b8179b9371cc3a6924019f39464cc8`

## Decision

Enable `ddl.pg.alter.owner.target.valid` as a blocker without parameters for
observed PG17 and fresh exact schema-qualified ordinary persistent non-typed
nonshared and nonmapped tables. Privately preserve the parsed owner kind and
exact literal name. CURRENT_USER and CURRENT_ROLE use the effective current
identity of the observation session. SESSION_USER uses its session identity.
Quoted keywords remain exact literal role names. PUBLIC is a pseudo-role that
PostgreSQL refuses as an owner. Missing named roles reject only on an observed
catalog miss. For guarded proven qualified table absence there is no owner
premise. Unknown target scope, action shape, freshness or role evidence yields
`unproven_table_owner_target`.

Read only `pg_roles` identity and same-owner facts. A true observation proves
sameness for the observed session and must never claim that another session
will execute with that same identity. Preserve the previous owner notice level
and publish no new role names, credentials, grants or membership details. The
existing authored SQL representation remains separate from findings and private
facts. The check makes no claim about permissions, namespace grants, or whether
a change can execute.

## Evidence

At `35157dfe`, ten real PG17.7 native OWNER TO statements paired with twenty
CLI audits. Six succeeded and four refused with `42704`. Quoted whitespace
is significant. Bare CURRENT_USER, CURRENT_ROLE and SESSION_USER succeeded,
while quoted `"current_user"` was an absent literal role. PUBLIC independently
refuses `42704`. The old CLI gave complete results for missing named targets
and incomplete for the special RoleSpec inputs. Independent catalog, file,
payload and sequence checks surrounded all audits and native rollback.
See `/tmp/ds-pg-f09-owner-cli-v3-35157dfe/evidence.json`.

## Frozen T37 additions

- `T37.cli.table-owner-target-offline`
- `T37.cli.table-owner-target-disabled`
- `T37.meta.pg-table-owner-target-missing`
- `T37.meta.pg-table-owner-target-exact`
- `T37.meta.pg-table-owner-target-same`
- `T37.meta.pg-table-owner-target-effective`
- `T37.meta.pg-table-owner-target-session`
- `T37.meta.pg-table-owner-target-quoted-alias`
- `T37.meta.pg-table-owner-target-public`
- `T37.meta.pg-table-owner-target-scope`
- `T37.meta.pg-table-owner-target-freshness`
- `T37.meta.pg-table-owner-target-guard-absent`

Keep prior cases and profiles. Bind only the original OWNER TO inventory
cell. The real CLI and PG17.7 native legs use task-owned roles and relations,
exact owner/file readback, a payload row and an independent sequence guard.

## Sources

- [PostgreSQL 17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PostgreSQL 17 pg_roles](https://www.postgresql.org/docs/17/view-pg-roles.html)
- [PostgreSQL 17.7 role resolution](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/utils/adt/acl.c)
- [PostgreSQL 17.7 ownership transition](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
