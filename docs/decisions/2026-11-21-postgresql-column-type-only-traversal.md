# PostgreSQL 17 ALTER COLUMN TYPE ONLY traversal

Status Accepted

F09 D43 uses the existing `ddl.pg.alter.column_type.destination.valid`
blocker for a fresh exact PostgreSQL 17 parent with attached children.
The observed user column must be locally eligible. For TYPE without USING,
the rule first checks the exact destination and bounded scalar assignment.
An eligible ONLY request then blocks if PostgreSQL requires changing the
children. The known destination failure precedes the traversal failure.
The provider rechecks attachment and omits detach-pending children.

Seven matching native and CLI controls exercise ordinary inheritance,
partitioning, missing destination, incompatible assignment, a childless
parent, the recursive action and USING. Five native operations refuse, and
two succeed. The USING and recursive nonordinary cases retain evidence gaps
pending conversion and descendant facts. Row conversion results, dependent
types, privileges, locks and effects on children require separate facts.
The output keeps child identities private.

- `/tmp/ds-pg-f09-type-only-cli-v2-683d84ac/evidence.json`
- [PostgreSQL 17.7 ATPrepAlterColumnType](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)

The joint T37 preview passed 535 cases with 15,589 assertions.
