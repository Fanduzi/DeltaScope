# PostgreSQL 17 ADD COLUMN inheritance and partition traversal

Status Accepted

D36 extends `ddl.alter.add_column.exists.forbid` on observed PostgreSQL 17.
Use an exact live relation, authored column and single action. Direct ADD to a
partition child refuses before the duplicate guard. On an ordinary inheritance
parent, the observed system name still refuses and IF NOT EXISTS can skip a
proved existing user column. A new column on a parent with attached children
requires traversal. Recursively adding an identity column to an ordinary
inheritance parent also refuses. A partitioned root follows its own native
identity semantics. The rule withholds unproved recursive merge, conversion and
execution outcomes. Findings keep dependent object identities private.

Eight PostgreSQL 17.7 native commands and corresponding CLI audits verified two
successes and six refusals. The fixture checked exact catalog and file identities,
unchanged data and an independent sequence after each audit and rollback.

- `/tmp/ds-pg-f09-add-column-traversal-cli-b49be2d4/evidence.json`
- [PostgreSQL 17.7 ATExecAddColumn](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)

The corrected combined T37 preview passed 481/481 with 13,697 assertions. Its 22 new native commands returned five successes and seventeen refusals. The `exists` field on the ADD findings describes the observed column presence.
