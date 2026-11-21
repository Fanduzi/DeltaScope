# PostgreSQL 17 CREATE TABLE LIKE target checks

Status Accepted

`ddl.pg.create_table.like.target.valid` checks qualified CREATE TABLE destinations and the ordered LIKE source list. A proved existing destination under `IF NOT EXISTS` skips the LIKE sources. Without the guard, PostgreSQL first resolves each source and checks whether its relation kind can supply columns. The audit recognizes ordinary and partitioned tables, views, materialized views, composite types and foreign tables. It checks duplicate names across the copied and explicit columns and retains the authored destination and reason code in the finding.

The parser preserves the effective LIKE option bits after ordered INCLUDING and EXCLUDING declarations. The audit reports an evidence gap for copying option effects. Native fixtures verify copied defaults, exclusion precedence and identity attributes. Session-local destinations, unqualified sources, typed or inherited destinations and access privileges require separate facts. Earlier DDL can invalidate the source and destination catalog evidence.

- [PostgreSQL 17 CREATE TABLE](https://www.postgresql.org/docs/17/sql-createtable.html)
- [PostgreSQL 17.7 LIKE source checks](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_utilcmd.c#L980)
- [T36 controls](../../testdata/ddl-golden/T36.json)

The joint T36 preview passed 88 cases and 1,608 assertions on `a83a1710`.
