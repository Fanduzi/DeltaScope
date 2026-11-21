# PostgreSQL 17 operator family namespace and owner changes

Status Accepted

`ddl.pg.alter_operator_family.change.target.valid` binds an exact qualified operator family and index access method. It checks whether a rename would collide with an existing family for that same method. A schema move uses the exact destination schema, rejects temporary and TOAST namespaces, and accepts a move to the current schema before checking destination occupancy.

PostgreSQL resolves the requested role before the source family for `OWNER TO`. The audit reports missing literal roles and PUBLIC, and binds session-role selectors through the observed connection. The rule keeps catalog IDs and resolved role identities private. Authorization, unqualified name resolution and the session's `pg_temp` alias need separate evidence.

The native and CLI controls verify collisions within one method, name reuse across methods, source and destination precedence, missing or inapplicable methods, and role resolution before source lookup.

- [PostgreSQL 17 ALTER OPERATOR FAMILY](https://www.postgresql.org/docs/17/sql-alteropfamily.html)
- [PostgreSQL 17.7 owner and namespace operations](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/alter.c)
- [T39 native and audit controls](../../testdata/ddl-golden/T39.json)

T39 passed 162 cases and 4,005 assertions in the joint preview. Seventeen evidence selectors bind the RENAME/OWNER/SET SCHEMA cell for PostgreSQL 17.
