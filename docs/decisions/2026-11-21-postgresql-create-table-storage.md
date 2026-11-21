# PostgreSQL CREATE TABLE storage declarations

Status Accepted

## Decision

Enable `ddl.pg.create_table.storage.valid` as a blocker for qualified persistent
PostgreSQL 17 CREATE TABLE declarations with explicit storage arguments. Use
catalog facts to check the destination schema and `IF NOT EXISTS` guard before
checking the tablespace. The rule rejects a missing tablespace or `pg_global`,
checks supported storage values, then verifies the named access method exists
and has table kind. The rule shares `fillfactor` and `autovacuum_enabled`
validation with ALTER TABLE.

The rule keeps unresolved namespace selection, partitioned or inherited
storage, typed tables, LIKE copies, and uncovered reloption values outside
this check. Quoted numeric values and settings in other reloption namespaces
retain an evidence gap. ACLs, configured defaults and custom access-method
execution require separate evidence. Findings include the authored creation
target, a reason code and a bounded option name where applicable.

## Verification

- [Native CREATE TABLE controls](../../testdata/ddl-golden/T36.json)
- [Index access method refusal](../../testdata/sql-corpus/postgresql/ddl/findings/create_table_index_access_method.expected.yaml)
- [Storage precedence tests](../../internal/domain/rule/ddl/postgresql_create_storage_rules_test.go)
- [PostgreSQL 17.7 DefineRelation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)

The joint T36 preview passed 126 cases and 2,903 assertions. 26 references bind the four exact PostgreSQL 17 storage and guard cells.
