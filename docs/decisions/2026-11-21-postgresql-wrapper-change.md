# PostgreSQL ALTER FOREIGN DATA WRAPPER declarations

Status Accepted

## Decision

Enable `ddl.pg.alter_foreign_data_wrapper.target.valid` for PostgreSQL 17.
Bind the exact wrapper and destination name. For OWNER TO, resolve the role
first and check that the requested role is a superuser. For routine changes,
resolve qualified zero-argument handlers and `(text[], oid)` validators, and
check that handlers return `fdw_handler`. Preserve option actions in authored
order. ADD requires absence, while SET and DROP require presence.

The catalog reader retrieves option names and routine eligibility. Findings
expose the authored wrapper name and reason code. Option values and resolved
catalog identities stay internal. A validator can reject an otherwise valid
option set, so that decision remains an evidence gap. Unqualified routine
resolution, caller authorization and actual data access need separate evidence.
The native fixtures use locally created wrappers and perform no remote access.

## Verification

- [T44 wrapper controls](../../testdata/ddl-golden/T44.json)
- [Missing option corpus](../../testdata/sql-corpus/postgresql/ddl/findings/alter_wrapper_drop_absent_option.expected.yaml)
- [PostgreSQL 17.7 foreign-data wrapper commands](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/foreigncmds.c)

T44 preview passed 64 cases and 1,280 assertions. Seventeen references bind the three exact PostgreSQL 17 wrapper cells.
