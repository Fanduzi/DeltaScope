# PostgreSQL 17 CREATE VIEW options

Status Accepted

F12 adds `ddl.pg.create_view.options.valid`, a default blocker for an observed PostgreSQL 17 target. It checks the authored option list for unknown names, namespaces, duplicate declarations and bounded values for `security_barrier`, `security_invoker` and `check_option`. A separate CHECK OPTION clause and a `check_option` entry can conflict. The parser preserves authored order and keeps option observations out of public JSON. Version or value facts that are unavailable produce an evidence gap. The CREATE VIEW forbid and the existing CHECK OPTION notice remain independently configurable.

Ten native PostgreSQL 17.7 and CLI controls cover five `22023` refusals, four accepted declarations and one updatability refusal. The latter proves a separate `0A000` condition outside declaration validity. The native rollback checks preserve the relation catalog, payload and sequence state. The combined T40 preview passed 58 cases with 1,152 assertions, and the SDK, HTTP and MCP tests verify the specific blocker. Fourteen selectors identify the CREATE VIEW WITH options and CHECK OPTION evidence. The WITH options cell records mixed coverage. View updatability, downstream dependencies and ALTER VIEW SET or RESET require separate facts.

The separate ALTER VIEW notices now use the exact decoded action. RENAME emits the rename notice and SET SCHEMA emits the schema notice. Four prior T40 rename cases retain their SQL and verification steps and now expect one notice. Both corpus cases assert the expected notice and exclude the unrelated one.

- [T40 native and audit controls](../../testdata/ddl-golden/T40.json)
- [PostgreSQL 17 CREATE VIEW](https://www.postgresql.org/docs/17/sql-createview.html)
- [PostgreSQL 17.7 relation-option parsing](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/access/common/reloptions.c)
