# PostgreSQL 17 default operator classes for constraint attachment

Status Accepted

D31 extends the existing `ddl.pg.alter.constraint_using_index.valid` blocker for a fresh, exact ordinary-table index used by `PRIMARY KEY USING INDEX` or `UNIQUE USING INDEX`. PostgreSQL requires the attached key slots to match the operator class it would select by default. The catalog read compares key slots only, excludes INCLUDE, and keeps class names and OIDs private. A class whose `opcdefault` flag is false supplies a rejection. Where a single exact-type default is observed, compare the installed class to it. Domains and default selection through binary compatibility or polymorphic classes need additional evidence. The scoped rule reports `index.constraint_default_opclasses` when the default cannot be established.

PostgreSQL 17.7 and the candidate CLI agreed on nondefault `text_pattern_ops`, an exact `text_ops` control, collation and sort controls, and the previous invalid-index and NULLS NOT DISTINCT cases. PostgreSQL accepted the varchar control through binary compatibility. The candidate reported the exact evidence gap. The audit leaves index and constraint installation to PostgreSQL and keeps catalog state private.

- `/tmp/ds-pg-f09-opclass-cli-b6d73f04/evidence.json`
- [PostgreSQL 17.7 USING INDEX transformation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_utilcmd.c)
- [PostgreSQL 17.7 default operator-class selection](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/indexcmds.c)

The combined T37 preview passed 447 cases and 12,506 assertions.
