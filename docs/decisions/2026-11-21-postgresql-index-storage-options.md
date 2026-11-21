# PostgreSQL 17 index storage options

Status Accepted

`ddl.pg.alter_index.options.valid` binds one qualified `ALTER INDEX SET` or `RESET` target before checking declarations. PostgreSQL skips proved absence under `IF EXISTS` and refuses a present target of the wrong kind, including a partitioned index. A RESET entry with a value is refused. PostgreSQL accepts duplicate or unknown RESET names without values.

For observed built-in B-tree and hash indexes, the audit checks integer fillfactor in the inclusive range 10 to 100. It checks PostgreSQL Boolean syntax for B-tree `deduplicate_items`, rejects that parameter on hash indexes and detects duplicate modeled SET declarations. PostgreSQL ignores the `toast` namespace on ordinary indexes and refuses unknown namespaces on SET. Quoted numeric forms, additional parameters, other access methods and ordered multi-action changes need independent evidence. Findings retain the authored target and hide catalog identities and parameter values.

- [PostgreSQL 17 ALTER INDEX](https://www.postgresql.org/docs/17/sql-alterindex.html)
- [PostgreSQL 17.7 relation options](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/access/common/reloptions.c)
- [T39 native and audit controls](../../testdata/ddl-golden/T39.json)

The joint T39 preview passed all 98 cases and 1,938 assertions. Twenty evidence selectors bind the PostgreSQL 17 SET/RESET parameters cell.
