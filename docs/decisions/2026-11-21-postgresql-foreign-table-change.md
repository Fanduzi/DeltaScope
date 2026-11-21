# PostgreSQL 17 ALTER FOREIGN TABLE target checks

Status Accepted

## Decision

Enable `ddl.pg.alter_foreign_table.target.valid` for qualified PostgreSQL 17 foreign-table targets. The rule checks the relation kind, honors the relation guard and evaluates one supported action at a time. Column actions check exact names, guarded ADD and DROP, system columns and rename collisions. Table rename and schema moves check the destination relation and type names. OWNER TO checks the requested role after resolving the table.

Table OPTIONS use the same ordered ADD, SET and DROP checks as wrapper OPTIONS. The catalog query reads only option names. Validator decisions, privileges, dependencies beyond the names checked here, and remote effects require separate evidence. Multi-action statements and session-dependent targets leave an evidence gap. Native fixtures create local catalog objects without accessing remote data.

## Verification

- [T44 foreign table controls](../../testdata/ddl-golden/T44.json)
- [Missing column corpus](../../testdata/sql-corpus/postgresql/ddl/findings/alter_foreign_table_drop_absent_column.expected.yaml)
- [PostgreSQL 17.7 table commands](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [PostgreSQL 17.7 foreign option handling](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/foreigncmds.c)

T44 preview passed 89 cases and 2,825 assertions. Twenty-six references bind the three PostgreSQL 17 foreign-table cells.
