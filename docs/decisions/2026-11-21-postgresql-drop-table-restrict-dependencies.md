# F10 PostgreSQL 17 direct DROP TABLE dependencies under RESTRICT

Status Accepted

The existing DROP target check can establish a separate RESTRICT dependency
premise after it resolves every exact authored member. The bounded scope is
an observed PostgreSQL 17 set of qualified persistent ordinary tables without
inheritance, with an optional IF EXISTS guard for proved absence. A targeted
catalog statement observes two dependency classes. A foreign key blocks when
its referencing table is outside the set. An ordinary view or materialized
view blocks when a normal rewrite edge references the relation or one of its
columns. A self-reference and a foreign key from a table included in the same
DROP list follow PostgreSQL's own removal semantics. CASCADE has its own
traversal. The fact binds an ordered target list and the first blocked
position, with zero for a proved empty blocker set and no value for an
incomplete catalog read. Public output identifies the authored target and a
bounded refusal reason. Dependent names and OIDs stay private.

Ten PostgreSQL 17.7 native transactions compared external, self and included
foreign-key references, whole-row and column view references, materialized
views, guarded absence, CASCADE and repeated targets. The four direct
refusals returned `2BP01`. A missing unguarded target returned `42P01`
before native dependency checking. Five controls succeeded. The fixture
verified catalog identities, rows and sequence state after rollback.
The exact query independently returned the expected position for ten target
sets. The current rule addresses direct observed classes. Wider dependency
closure, other relation kinds, locking and privileges require separate facts.

- `/tmp/ds-pg-f10-drop-dependencies-36ef07e4/evidence.json`
- `/tmp/ds-pg-f10-drop-reference-query-441e6b34/catalog-read.json`
- [PostgreSQL 17 DROP TABLE](https://www.postgresql.org/docs/17/sql-droptable.html)
- [PostgreSQL 17.7 dependency traversal](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/dependency.c)

T38 passed all 46 preview cases with 1,178 assertions, including seven new direct-dependency controls. The selectors bind the two DROP TABLE PostgreSQL 17 subactions.
