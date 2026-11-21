# PostgreSQL 17 partition keys in primary and unique constraints

Status Accepted

Extend `ddl.pg.alter.constraint_columns.valid` to a single direct ADD PRIMARY KEY or ADD UNIQUE on a qualified PostgreSQL 17 partitioned root. Check the authored key and INCLUDE members before comparing the partition key. Every partition column must be part of the constraint key. A column present only in INCLUDE does not satisfy that requirement. An expression partition key also prevents this constraint. The check compares the partition column's collation and equality operator to the collation and exact default B-tree operator class that the constraint would use.

The provider reads one catalog snapshot and returns column identities and comparison facts. The finding keeps the partition-column identities, collation and operator IDs private. The audit reports an evidence gap when it cannot establish the default equality operator. Partition subtrees, fallback operator class resolution and existing row validation require separate checks.

- [PostgreSQL 17 partitioning limitations](https://www.postgresql.org/docs/17/ddl-partitioning.html#DDL-PARTITIONING-DECLARATIVE-LIMITATIONS)
- [PostgreSQL 17.7 index definition checks](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/indexcmds.c)
- [T37 controls](../../testdata/ddl-golden/T37.json)

T37 passed 589 cases and 17,444 assertions in the joint D48 and D49 preview. Fourteen selectors bind the two PostgreSQL 17 constraint cells.
