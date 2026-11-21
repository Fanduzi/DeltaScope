# PostgreSQL 17 collation namespace and owner checks

Status Accepted

`ddl.pg.alter_collation.change.target.valid` binds a qualified collation in the database encoding then assessing a rename or schema move. `OWNER TO` resolves the requested role first. A rename rejects an occupied name, including the source's current name. A move to the current schema succeeds before the destination collision check. A move to a different schema checks exact existence, rejects temporary and TOAST namespaces, and checks the source name against destination entries for the current encoding and any encoding.

Owner checks distinguish literal and session-role selectors. They report missing literal roles or PUBLIC before evaluating source existence. Findings retain the authored collation identity and a reason code. Resolved role names and catalog identifiers stay private. The `pg_temp` alias, unqualified names, unsupported ICU encoding resolution, role membership and execution permissions need separate evidence.

- [PostgreSQL 17 ALTER COLLATION](https://www.postgresql.org/docs/17/sql-altercollation.html)
- [PostgreSQL 17.7 destination collision check](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/collationcmds.c#L398)
- [T35 native and audit controls](../../testdata/ddl-golden/T35.json)

T35 passed 77 cases and 1,608 assertions in the joint preview. Sixteen evidence selectors bind the PostgreSQL 17 RENAME/OWNER/SET SCHEMA cell.
