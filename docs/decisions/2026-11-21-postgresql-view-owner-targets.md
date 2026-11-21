# PostgreSQL 17 view owner targets

Status Accepted

The default blocker `ddl.pg.alter_view.owner.valid` checks single-action OWNER TO on ordinary and materialized views on observed PostgreSQL 17. The parser retains exact identifiers, the source guard and the authored role selection. The catalog read checks the precise source kind and the existence of the selected role. IF EXISTS skips a proved missing source. A present relation of the wrong kind blocks before role resolution. An absent literal role or PUBLIC also blocks. The observed session supplies CURRENT_USER, CURRENT_ROLE and SESSION_USER. The rule keeps role catalog facts private.

Ten PostgreSQL 17.7 and CLI controls reproduce quoted role matching, missing roles, PUBLIC, source-kind precedence and guarded absence. The native rollback controls distinguish session and effective roles. Audit and native witnesses preserve the original relation and file identities, rows, and independent sequence. Role membership, schema privileges and effective execution authorization require separate evidence.

- [PostgreSQL 17 ALTER VIEW](https://www.postgresql.org/docs/17/sql-alterview.html)
- [PostgreSQL 17 ALTER MATERIALIZED VIEW](https://www.postgresql.org/docs/17/sql-altermaterializedview.html)
- [T40 native and audit controls](../../testdata/ddl-golden/T40.json)

The combined T40 preview passed 94 cases and 2,528 assertions. Twelve additional selectors bind the three exact PostgreSQL 17 cells.
