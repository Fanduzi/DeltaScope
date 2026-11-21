# PostgreSQL 17 primary and unique constraint columns

Status Accepted

D48 checks one direct `ALTER TABLE ... ADD PRIMARY KEY` or `ADD UNIQUE` on a fresh, exactly bound PostgreSQL 17 ordinary table. The parser keeps the key and INCLUDE column lists separately, preserving decoded identifiers, order and repetitions. A duplicate key, a proved missing member, an observed system column or a second primary key produces a specific blocker. The check honors a proved absent target under IF EXISTS. The PostgreSQL primary-key column-count rule uses the exact key list when available, so a comma inside a quoted identifier counts as part of the name. The existing migration warning and USING INDEX checks remain independently configurable.

PostgreSQL 17.7 accepts repeated INCLUDE members and overlap with the key. The native index catalog records the exact slots in both cases. Missing, duplicate and system members can fail before a second-primary-key refusal. The original native comparison covers fifteen cases. Six additional native and CLI controls verify precedence around an existing primary key. The audit does not read row values or predict uniqueness validation. Partition key coverage, constraints in the same ALTER batch, locks and permission checks require separate facts.

- [PostgreSQL 17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PostgreSQL 17.7 constraint transformation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_utilcmd.c)
- [T37 native and audit controls](../../testdata/ddl-golden/T37.json)

The joint T37 preview passed all 576 cases with 17,015 assertions. Fifteen selectors bind the PostgreSQL 17 ADD PRIMARY KEY and ADD UNIQUE cells.
