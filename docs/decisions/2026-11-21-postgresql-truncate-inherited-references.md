# PostgreSQL 17 TRUNCATE target order and inherited foreign-key references

Status Accepted

F10 extends `ddl.table.truncate.inbound_fk` for an exact qualified PostgreSQL
17 RESTRICT request. The provider expands the selected persistent table set
through attached inheritance edges. The first selection of a relation controls
whether the explicit target recurses. A partitioned ONLY target refuses unless
an earlier target already selected it. This eligibility also applies to
CASCADE. A referencing table outside the selected set can block even when
the authored target is a partition leaf. The provider follows inherited
foreign-key constraint parents to find that reference. Findings identify the
authored member and keep the discovered table and foreign-key names private.

Sixteen native PostgreSQL 17.7 transactions and matching CLI audits cover
ordinary and partitioned targets, external references, explicit target order,
CASCADE controls and RESTART IDENTITY. The fixture compares descendant rows,
catalog and file identity, owned sequence state and an independent guard
sequence before and after audit and after the native rollback.

CASCADE finds further dependencies at execution time. Sequence ownership
permissions, locking, foreign-table behavior and session-dependent temporary
relations require their own evidence. The audit reads catalog relations.

- `/tmp/ds-pg-f10-truncate-order-cli-root.txt` points to the local comparison receipt.
- [PostgreSQL 17.7 TRUNCATE target handling](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [PostgreSQL 17.7 foreign-key traversal](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/heap/heap.c)

T38 passed all 62 preview cases with 1,761 assertions. Sixteen added selectors bind the three PostgreSQL 17 TRUNCATE subactions.
