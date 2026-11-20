# PostgreSQL 17 partition NOT NULL traversal

Status Accepted

F09 D39 extends `ddl.pg.alter.column.generation.transition.valid` for a
single observed, exact schema-qualified PostgreSQL 17 partition action. An
attached partition child cannot drop the flag if the exact parent column
requires NOT NULL. A partitioned parent with attached partitions cannot
use ONLY to drop NOT NULL, including when the parent column is already
nullable. An ONLY SET NOT NULL on a currently nullable partitioned root
requires all attached descendant attributes to carry NOT NULL. One observed
nullable descendant proves that refusal. Ordinary inheritance children
have different DROP NOT NULL behavior. The rule keeps names of ancestors
and descendants private and preserves evidence gaps for data and remaining
column premises. It ignores detach-pending entries for these observations.

Ten native PostgreSQL 17.7 controls and corresponding CLI audits show five
successes and five refusals. Four target or traversal refusals return
`42P16`; the remaining `23502` is the independent existing-null case. The
native positive controls include a direct child-local DROP and an ONLY SET
whose child was already marked NOT NULL. Each rollback keeps catalog and
file identities, payload and an independent sequence intact.

- `/tmp/ds-pg-f09-partition-nullability-cli-v2-441e6b34/evidence.json`
- [PostgreSQL 17.7 ALTER TABLE nullability implementation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)

The combined preview passed 505/505 T37 cases with 14,539 assertions. Nine selectors attach to the PostgreSQL 17 SET/DROP NOT NULL cell.
