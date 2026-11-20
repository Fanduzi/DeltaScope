# PostgreSQL 17 inherited and partitioned column traversal

Status Accepted

F09 D33 extends the existing DROP COLUMN and RENAME COLUMN source blockers for one exact schema-qualified action on a persistent ordinary inheritance table, partitioned table or partition child. The parser retains ONLY for RENAME. A targeted catalog read binds the positive live user attribute, its `attinhcount`, direct partition-key membership and attached children. The attached-child test excludes detach-pending entries, matching PostgreSQL's visible-child traversal in an active snapshot.

A direct DROP refuses an inherited column, then a direct partition key, then ONLY on a partitioned parent with attached children. The ordinary parent ONLY control has a different native outcome. A direct RENAME refuses ONLY when attached children require traversal, and rejects a column inherited by the selected relation. The facts apply to the exact source. A known refusal is sufficient for the existing blocker. Other target, descendant and dependency premises retain their existing evidence gaps. The provider keeps child names, expressions and catalog identifiers private.

Twelve PostgreSQL 17.7 native rollback cases and corresponding actual CLI audits checked eight `42P16` refusals and four successes. They distinguish ordinary and partition inheritance, locally merged columns, a direct partition key on a childless root, attached and empty ONLY roots, and parent RENAME traversal. Captured catalog, file identity, payload and guard sequence matched after each rollback. Partition-key expressions, descendant name conflicts, multi-parent recursion and privileges require separate evidence.

- `/tmp/ds-pg-f09-column-traversal-cli-36ef07e4/evidence.json`
- [PostgreSQL 17.7 ATExecDropColumn and renameatt_internal](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [PostgreSQL 17.7 find_inheritance_children](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/pg_inherits.c)

The corrected T37 preview passed 449/449 cases and 12,536 assertions. The earlier scope case retains its SQL and native controls and now expects the exact inherited-column refusal. Fourteen inventory selectors bind these cases and corpus controls.
