# Decision: PostgreSQL ordinary-table persistence transitions

Date: 2026-11-18
Status: Accepted
Milestone: F09 / `milestone/pg-alter-semantics` (D13)
Product baseline: `28de6dc545bb007246513f606e9ed802903515f4`

## Decision

Enable `ddl.pg.alter.persistence.transition.valid` as a blocker without
parameters. Check a single `SET LOGGED` or `SET UNLOGGED` on an observed
PostgreSQL 17 server with a fresh, exact schema-qualified ordinary non-typed
table of permanent or unlogged persistence. Exclude shared and mapped
relations, inheritance and partition membership. A proven qualified absent
relation protected by `IF EXISTS` has no transition premise. Unknown scope,
multiple actions, prior DDL or parser failures retain an
`unproven_table_persistence` evidence gap.

A request for the current persistence settles the transition check and the
existing notice explains that table files stay in place. For a change to
UNLOGGED, check direct `pg_publication_rel` membership and inbound foreign keys
from other permanent tables. For a change to LOGGED, check outbound foreign
keys to nonpermanent tables. Self-references are excluded in both directions.
Membership through FOR ALL TABLES or FOR TABLES IN SCHEMA alone does not trigger
the publication refusal. A missing foreign-key counterpart does not prove
absence of a conflict. Findings expose only the action, affected table identity
and bounded reason. Publication and constraint names, counterpart identities,
provider errors and catalog details stay private.

The old notices retain their levels and metadata shape, with conditional wording
for proposed changes and no-rewrite wording for observed same-persistence
requests. This scope covers the specified native preconditions. It does not
certify privileges, locks, runtime replication or execution. Temporary tables,
partition and inheritance behavior remain separate dimensions.

## Baseline evidence

At `28de6dc5`, 11 real PG17.7 native shapes paired with 22 offline/live CLI
audits gave eight native successes, a direct-publication `55000` refusal and
two directional-FK `42P16` refusals. All audits passed with the legacy notice.
An independent oracle checked exact catalogs, file identity, data and sequence
state after audit and native rollback. Additional native controls show both
same-session temporary-table requests refusing `42P16` and a partitioned root
accepting a request without a local persistence or file change. Evidence is at
`/tmp/ds-pg-f09-persistence-cli-28de6dc5/evidence.json` and
`/tmp/ds-pg-f09-persistence-e64fd4bb/evidence.json`.

## Frozen T37 additions

- `T37.cli.table-persistence-offline`
- `T37.cli.table-persistence-disabled`
- `T37.meta.pg-table-persistence-same-logged`
- `T37.meta.pg-table-persistence-same-unlogged`
- `T37.meta.pg-table-persistence-change`
- `T37.meta.pg-table-persistence-direct-publication`
- `T37.meta.pg-table-persistence-all-schema-publication`
- `T37.meta.pg-table-persistence-inbound-fk`
- `T37.meta.pg-table-persistence-outbound-fk`
- `T37.meta.pg-table-persistence-self-fk`
- `T37.meta.pg-table-persistence-scope`
- `T37.meta.pg-table-persistence-freshness`
- `T37.meta.pg-table-persistence-guard-absent`

Keep every earlier case and profile. Bind only the original `SET LOGGED/UNLOGGED`
inventory evidence cell. Each canonical live case checks the catalog, relation
file identity, data and an unrelated sequence before independent native steps.

## Evidence bindings

`a8767e9c` binds private catalog facts. `dc2a3bbb` implements the shared blocker
and gives same-persistence requests precise no-rewrite wording. The focused
candidate passed T37 236/236 with 5333 assertions. All 223 earlier cases and
profiles are preserved. Thirteen new cases include fourteen native steps
with ten successes and four precise refusals. The catalog, file, payload and
sequence oracles run independently before native execution. The full run at
the fixed candidate SHA is required for delivery.

Fourteen evidence selectors bind the original SET LOGGED/UNLOGGED anchor.
Existing row and subaction classifications and denominators are preserved.

## Sources

- [PostgreSQL 17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PostgreSQL 17 relation catalog](https://www.postgresql.org/docs/17/catalog-pg-class.html)
- [PostgreSQL 17 constraint catalog](https://www.postgresql.org/docs/17/catalog-pg-constraint.html)
- [PostgreSQL 17.7 persistence preconditions](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [PostgreSQL 17.7 direct publication membership](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/pg_publication.c)
