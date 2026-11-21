# PostgreSQL 17 schema creation targets and owners

Status Accepted

`ddl.pg.create_schema.target.valid` binds the exact requested schema and the authored `AUTHORIZATION` role. PostgreSQL resolves the role before applying the `IF NOT EXISTS` guard. The audit reports a missing literal role or the PUBLIC pseudo-role even when the named schema already exists. A valid literal or session role can provide the schema name when it is omitted. Only the authored schema name and the reason code appear in findings.

The rule checks a duplicate unguarded schema and the reserved `pg_` prefix against the observed `allow_system_table_mods` setting. It preserves the exact case of quoted identifiers. Database CREATE privilege, permission to adopt the requested role, live execution effects, and embedded schema statements require separate evidence. Ordered analysis withholds schema state when the owner or reserved-name execution premise is unresolved.

- [PostgreSQL 17 CREATE SCHEMA](https://www.postgresql.org/docs/17/sql-createschema.html)
- [PostgreSQL 17.7 creation order](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/schemacmds.c#L52)
- [T35 native and audit controls](../../testdata/ddl-golden/T35.json)

The joint T35 preview passed 62 cases and 1,093 assertions on `919f1d17`. The inventory binds 21 evidence selectors across three PostgreSQL 17 cells.
