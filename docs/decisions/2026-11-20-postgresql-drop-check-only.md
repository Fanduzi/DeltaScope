# PostgreSQL 17 DROP CHECK with ONLY on partitioned parents

Status Accepted

F09 D26 on `milestone/pg-alter-semantics`. The user delegated future decisions about default rule behavior. Run this slice alongside D25, with separate reasons and evidence.

## Decision

Extend the existing `ddl.pg.alter.drop_constraint.target.require` at blocker level for a single exact schema-qualified `ALTER TABLE ONLY ... DROP CONSTRAINT` on observed PostgreSQL 17. A bound CHECK on a persistent, non-typed partitioned table with direct children refuses when that CHECK participates in inheritance. Retain the traversal flag from the PostgreSQL AST. Read the child-existence and CHECK propagation premise in the exact constraint directory through `pg_catalog.pg_inherits` and `connoinherit`. A known empty child set leaves this premise silent. A missing observation yields `constraint.pg_partition_children` under the existing target gap. The finding uses `constraint_partition_parent_only` without exposing partition names or catalog IDs.

Give a proved inherited CHECK the D25 reason before considering D26. `IF EXISTS` still guards exact absence, and CASCADE still permits the existing dependency policy to run independently. Recursive DROP, an ordinary inheritance parent with ONLY, and non-CHECK constraints do not invoke this specific premise. Locking, permissions, underlying DDL ordering and dependency closure have separate prerequisites.

## Native evidence

Ten native PostgreSQL 17.7 commands at `a9b8065d` distinguish a partitioned parent with children from an empty partitioned parent, an ordinary inheritance parent and the non-CHECK case. Five succeeded and five returned `42P16`. Catalog and file identities, payload rows and a sequence guard matched after each rollback. A temporary CLI overlay reproduced native results for D25 and D26, including their distinct rejection reasons. No candidate repository source was modified during D24 validation.

- `/tmp/ds-pg-d26-only-check-a9b8065d/evidence.json`
- `/tmp/ds-pg-d25-d26-overlay-cli-a9b8065d/evidence.json`
- [PG17.7 ATExecDropConstraint](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)

## T37 cases

- `T37.meta.pg-drop-check-only-partition-root`
- `T37.meta.pg-drop-check-only-guarded`
- `T37.meta.pg-drop-check-only-cascade`
- `T37.meta.pg-drop-check-only-empty-root`
- `T37.meta.pg-drop-check-only-recursive`
- `T37.meta.pg-drop-check-only-inheritance-parent`
- `T37.meta.pg-drop-check-only-missing-guarded`
- `T37.meta.pg-drop-check-only-primary-control`
