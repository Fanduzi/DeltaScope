# PostgreSQL 17 ALTER VIEW option changes

Status Accepted

`ddl.pg.alter_view.options.valid` checks one ordinary-view SET or RESET option action on an observed PostgreSQL 17 server. It binds the exact source and tests its relation kind before assessing declarations. IF EXISTS skips a proved missing target. SET checks names, duplicates and bounded values for the ordinary-view options. The native `toast` namespace is ignored for an ordinary view. RESET permits unknown or repeated names and refuses a value. If SET or RESET leaves CHECK OPTION present, automatic updatability still needs evidence. The catalog read observes the option's presence while keeping catalog values private.

Fourteen PostgreSQL 17.7 native and CLI controls cover target precedence, valid and invalid SET declarations, RESET values, duplicate and unknown RESET names, and positive and negative CHECK OPTION updatability. The no-audit-execution witness compares option arrays, relation and file identities, rows and an independent guard sequence. Materialized-view storage options and multi-action ALTER retain their adapter boundaries. Their native behavior needs its own observation and rule scope.

- [PostgreSQL 17 ALTER VIEW](https://www.postgresql.org/docs/17/sql-alterview.html)
- [PostgreSQL 17.7 option transformations](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/access/common/reloptions.c)
- [PostgreSQL 17.7 view option and updatability checks](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [T40 native and audit controls](../../testdata/ddl-golden/T40.json)

The joint preview passed 109 T40 cases and 3,013 assertions. Sixteen selectors bind the PostgreSQL 17 SET/RESET inventory cell. The established single-SET case keeps its SQL and now verifies the modeled action.
