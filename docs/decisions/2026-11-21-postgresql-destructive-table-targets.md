# PostgreSQL 17 destructive table target binding

Status Accepted

The existing DROP and TRUNCATE target rules use the ordered AST targets on PostgreSQL 17. The model preserves exact decoded identifiers, duplicates, database qualification, the DROP guard, effective CASCADE and RESTRICT, per-target ONLY, and the requested identity behavior. Each qualified target binds to a fresh schema-local relation observation. DROP IF EXISTS skips an observed absence. A wrong relation kind still refuses, and a later unguarded absence remains visible. A request with catalog-qualified, temporary, unknown or stale targets reports an attributable evidence gap.

TRUNCATE checks relation eligibility and delegates the ordered partitioned ONLY premise to the target membership observation. The first selection determines whether a later duplicate needs traversal. Foreign-table TRUNCATE requires FDW evidence. Sequence ownership, permission checks and lock availability require separate facts. The audit records the requested identity mode. The native fixtures independently verify the owned sequence values after RESTART and CONTINUE.

The rebased T38 preview at `24389415` passed 62 cases and 1,761 assertions. Its 44 added native commands contain 21 successes and 23 refusals. All 17 original case IDs remain. Two inventory rows move from generic notice to mixed coverage, based on the PostgreSQL 17 target and dependency evidence. The wider CASCADE effect set, temporary-session resolution and FDW behavior still require evidence.

- [PostgreSQL 17 DROP TABLE](https://www.postgresql.org/docs/17/sql-droptable.html)
- [PostgreSQL 17 TRUNCATE](https://www.postgresql.org/docs/17/sql-truncate.html)
- [Native guard and target-kind fixtures](../../testdata/ddl-golden/T38.json)
