# PostgreSQL 17 index partition attachment

Status Accepted

`ddl.pg.alter_index.attach_partition.valid` checks qualified parent and child names for one ALTER INDEX ATTACH PARTITION. PostgreSQL checks the parent first, then the child. A parent must be a partitioned index. A child can be an ordinary or partitioned index. The audit is silent for an already observed attachment to the same parent. It reports an occupied slot for the child's table, attachment to another parent, or a child table outside the parent's direct partition set. A detach-pending edge requires visibility evidence.

Twenty-two PostgreSQL 17.7 paired controls cover the source and attachment checks, including the guarded spelling rejected with `42601`. For indexes without expressions or predicates, compare key and INCLUDE slots by their exact owning column names. The definitions must agree on access method, uniqueness, null treatment, key count, operator families and key collations. A constraint-backed parent also needs a constraint on the child index. The native controls include a matching index whose table has different attribute numbers. When both indexes use expressions or both have predicates, the audit reports missing compatibility evidence. Findings carry authored identities and reason codes.

- [PostgreSQL 17 ALTER INDEX](https://www.postgresql.org/docs/17/sql-alterindex.html)
- [PostgreSQL 17.7 attachment prerequisites](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c#L19863)
- [PostgreSQL 17.7 index definition comparison](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/index.c#L2511)

T39 passed 79 cases and 1,339 assertions at preview base `d4cf7b6c`. Twenty-three selectors bind the PostgreSQL 17 ATTACH PARTITION cell.

- [T39 native and audit controls](../../testdata/ddl-golden/T39.json)
