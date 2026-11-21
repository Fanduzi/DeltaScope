# PostgreSQL 17 collation version refresh

Status Accepted

`ddl.pg.alter_collation.refresh.target.valid` checks a qualified `ALTER COLLATION ... REFRESH VERSION`. The provider gives precedence to the database encoding's exact catalog entry and then an applicable any-encoding entry. The audit blocks proved absence and identifies the database-default collation by catalog identity. A user collation named `default` remains eligible for its own version check.

For libc C and POSIX and built-in C and C.UTF-8, the provider compares whether the recorded and actual versions are present. PostgreSQL rejects a change between a tracked version and no version; it accepts changes between two tracked versions. The provider returns Boolean facts and keeps locale strings, version strings and catalog identifiers private. For other provider locales and unresolved encoding eligibility the audit reports an evidence gap. Authorization, dependent index rebuilds and unqualified session resolution require separate facts.

- [PostgreSQL 17 ALTER COLLATION](https://www.postgresql.org/docs/17/sql-altercollation.html)
- [PostgreSQL 17.7 refresh contract](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/collationcmds.c#L428)
- [T35 native and audit controls](../../testdata/ddl-golden/T35.json)

The joint T35 preview passed 62 cases and 1,093 assertions on `919f1d17`. The inventory binds 21 evidence selectors across three PostgreSQL 17 cells.
