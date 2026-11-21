# PostgreSQL 17 operator family creation targets

Status Accepted

`ddl.pg.create_operator_family.target.valid` checks the exact namespace, family name and access method of a qualified `CREATE OPERATOR FAMILY` declaration. PostgreSQL rejects a missing schema before checking method or destination. The method must exist and be an index method. A family can use the same name as an existing family for a different access method. Findings use the authored schema and family name and keep catalog identifiers private.

The paired PostgreSQL 17.7 cases exercise successful creation, a distinct method, exact collision, missing schema, missing method and a table method. `CREATE OPERATOR FAMILY ... AS OPERATOR ...` returns `42601` in PostgreSQL and is a parser error. The existing T39 case preserves this spelling. Authorization, unqualified namespace selection and ALTER member checks require separate evidence.

- [PostgreSQL 17 CREATE OPERATOR FAMILY](https://www.postgresql.org/docs/17/sql-createopfamily.html)
- [PostgreSQL 17.7 implementation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/opclasscmds.c#L772)
- [T39 native and audit controls](../../testdata/ddl-golden/T39.json)

The joint T39 preview passed all 130 cases and 2,948 assertions at base `d4cf7b6c`.
