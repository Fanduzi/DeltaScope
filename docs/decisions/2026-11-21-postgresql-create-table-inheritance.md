# PostgreSQL CREATE TABLE INHERITS parents

Status Accepted

## Decision

Enable `ddl.pg.create_table.inherits.valid` as a blocker for qualified
persistent PostgreSQL 17 creation targets. Resolve the destination and the
ordered parent list. An existing destination under `IF NOT EXISTS` skips the
parent checks. PostgreSQL resolves each parent identity and rejects duplicates
before checking relation kinds. Ordinary and foreign tables are eligible kinds.
Views, partitioned roots and partition children are refused. The absence of a
partition fact leaves that check open.

The catalog lookup shares destination and ordered source binding with CREATE
TABLE LIKE. Findings expose the authored destination and the refusal reason.
Parent identities and catalog facts are kept internal. Session-dependent
names and snapshot freshness require evidence. Parent attribute merging,
defaults, constraints, permissions and execution still require their own
checks. The successful parent controls report that the column merge is
unverified and use native catalog witnesses for the actual merge.

## Verification

- [T36 INHERITS controls](../../testdata/ddl-golden/T36.json)
- [Absent parent corpus](../../testdata/sql-corpus/postgresql/ddl/findings/create_inheritance_absent_parent.expected.yaml)
- [Parent precedence unit tests](../../internal/domain/rule/ddl/postgresql_create_inheritance_rules_test.go)
- [PostgreSQL 17.7 relation creation and attribute merge](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)

The joint T36 preview passed 198 cases and 5,362 assertions. 16 references bind the exact PostgreSQL 17 INHERITS cell.
