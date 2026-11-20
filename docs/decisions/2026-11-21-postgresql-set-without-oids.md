# PostgreSQL 17 SET WITHOUT OIDS target eligibility

Status Accepted

F09 D29 keeps the existing `ddl.table.exists.alter.require` blocker for the
obsolete `SET WITHOUT OIDS` clause. PostgreSQL 17 still checks the relation
kind in `ATPrepCmd`. The execution pass has no action for `AT_DropOids`.
Audit one action on a fresh exact schema-qualified relation on observed
PostgreSQL 17. A table, partitioned table or foreign table satisfies the
kind premise. A proved missing relation is guarded by `IF EXISTS`. An
observed view or sequence fails the kind check even with the guard. Missing
version, identity or catalog evidence produces
`unproven_alter_table_relation`. Removed `SET WITH OIDS` syntax keeps its
parser-error classification. No data or catalog modification takes place
in the audit. Execution permission and lock availability are separate.

The PostgreSQL 17.7 native and CLI comparison checked eight forms. Ordinary
and partitioned tables and guarded absence succeeded. Missing unguarded
relation returned `42P01`. View, guarded view and sequence returned `42809`.
Removed syntax returned `42601`. The native fixtures compared exact catalog
and file identities, row contents and a sequence before and after audit.

- `/tmp/ds-pg-f09-oid-target-cli-b6d73f04/evidence.json`
- [PostgreSQL 17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PostgreSQL 17.7 AT_DropOids preparation and execution](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
