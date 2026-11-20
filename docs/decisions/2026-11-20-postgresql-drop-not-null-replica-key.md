# PostgreSQL 17 DROP NOT NULL replica identity keys

Status Accepted

F09 D20 on `milestone/pg-alter-semantics` after `64a1fd4e`.

## Decision

Extend `ddl.pg.alter.column.generation.transition.valid` for a single
`DROP NOT NULL` on a fresh, exact, schema-qualified ordinary persistent
non-typed table on observed PostgreSQL 17. An observed key slot in a live
index marked as replica identity blocks with
`not_null_required_by_replica_identity`. The existing identity and primary
key refusals retain their reason and precedence. Independent DROP NOT NULL
policies retain their existing behavior.

Read key membership with the exact column directory observation. Use
`pg_index.indrelid`, `indislive`, `indisreplident` and the first
`indnkeyatts` slots of `indkey`. INCLUDE positions remain outside the key.
`RelationGetIndexList` excludes dead indexes. `ATExecDropNotNull` does
not filter out live indexes based on validity or readiness before checking
the relevant markers. FULL mode and unattached unique indexes do not imply
key membership. Unknown membership produces the existing generation target
gap with `column.replica_identity_key` in the supported shape. Known
nonmembership is silent. The observation stays out of public JSON.

The rule covers the column/index prerequisite. Row validation for SET NOT
NULL, runtime replication, PostgreSQL execution effects, permissions,
inheritance and partition behavior retain separate evidence requirements.

## Evidence

At `1bc8709c`, nine PostgreSQL 17.7 native statements paired with eighteen
CLI audits. Five native statements succeeded. The explicit replica key and
the primary key refused with `42P16`, an exact source-name mismatch with
`42703`, and an identity column with `42601`. INCLUDE slots on both index
kinds, unattached unique keys and FULL mode were successful controls. The
probe verified catalog/file identity, nullability, payload and sequence
state after the audits and after native rollback.

- `/tmp/ds-pg-f09-drop-not-null-rid-1bc8709c/evidence.json`
- `/tmp/ds-pg-f09-drop-not-null-rid-1bc8709c/cleanup.json`

## Frozen T37 additions

- `T37.cli.drop-not-null-replica-key-offline`
- `T37.cli.drop-not-null-replica-key-disabled`
- `T37.meta.pg-drop-not-null-replica-key-exact`
- `T37.meta.pg-drop-not-null-replica-key-include`
- `T37.meta.pg-drop-not-null-replica-key-full`
- `T37.meta.pg-drop-not-null-replica-key-unattached`
- `T37.meta.pg-drop-not-null-replica-key-precedence`
- `T37.meta.pg-drop-not-null-replica-key-stale`

Preserve the 309 earlier cases and their profiles. Bind new selectors only
to the PostgreSQL 17 SET/DROP NOT NULL cell. Keep audit and native
execution separate with catalog and file, payload and sequence controls.

## Evidence bindings

`f8695f4d` adds the private column fact. `ef8836ab` consumes it in the existing
blocker. A real PG17.7 provider probe verified six catalog observations. A
separate CLI comparison at `f8695f4d` with the rule in the working tree
reproduced all nine earlier native outcomes, including the independent
policy findings. See `/tmp/ds-pg-d20-actual-provider.log` and
`/tmp/ds-pg-d20-candidate-cli-f8695f4d/evidence.json`.

Focused T37 at that candidate passed 317/317 cases and 8,105 assertions.
Eight new native commands include four successes and four precise refusals.
The new isolated profile tests known membership, known nonmembership and
stale evidence. All 309 earlier cases and profiles were preserved. Nine
selectors attach only to the SET/DROP NOT NULL PG17 inventory cell.

## Sources

- [PostgreSQL 17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PostgreSQL 17.7 DROP NOT NULL](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [PostgreSQL 17.7 live index selection](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/utils/cache/relcache.c)
