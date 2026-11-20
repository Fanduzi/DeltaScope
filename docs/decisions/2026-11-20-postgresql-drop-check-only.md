# PostgreSQL 17 DROP CHECK with ONLY on partitioned parents

Status Accepted

F09 D26 on `milestone/pg-alter-semantics`. The user delegated future decisions about default rule behavior. Run this slice alongside D25, with separate reasons and evidence.

## Decision

Extend the existing `ddl.pg.alter.drop_constraint.target.require` at blocker level for a single exact schema-qualified `ALTER TABLE ONLY ... DROP CONSTRAINT` on observed PostgreSQL 17. A bound CHECK on a persistent, non-typed partitioned table with attached direct children refuses when that CHECK participates in inheritance. The audit retains the traversal flag from the PostgreSQL AST and reads `pg_catalog.pg_inherits` and `connoinherit` in the exact constraint directory. PostgreSQL skips visible detach-pending children in this traversal. The catalog query excludes those children, so a parent with only a detach-pending child follows the observed empty-child path. A missing observation yields `constraint.pg_partition_children` under the existing target gap. The finding uses `constraint_partition_parent_only` and keeps partition names and catalog IDs private.

Give a proved inherited CHECK the D25 reason before considering D26. `IF EXISTS` still guards exact absence, and CASCADE still permits the existing dependency policy to run independently. Recursive DROP, an ordinary inheritance parent with ONLY, and non-CHECK constraints do not invoke this specific premise. Locking, permissions, underlying DDL ordering and dependency closure have separate prerequisites.

## Native evidence

Ten native PostgreSQL 17.7 commands at `a9b8065d` distinguish a partitioned parent with children from an empty partitioned parent, an ordinary inheritance parent and the non-CHECK case. Five succeeded and five returned `42P16`. Catalog and file identities, payload rows and a sequence guard matched after each rollback. A temporary CLI overlay reproduced native results for D25 and D26, including their distinct rejection reasons. No candidate repository source was modified during D24 validation.

- `/tmp/ds-pg-d26-only-check-a9b8065d/evidence.json`
- `/tmp/ds-pg-d25-d26-overlay-cli-a9b8065d/evidence.json`
An additional PostgreSQL 17.7 comparison used two partitioned parents with the same CHECK. One had an attached child. For the other, a separate session held an `ACCESS SHARE` lock on the parent while `DETACH PARTITION CONCURRENTLY` committed the pending catalog state. The fixture canceled that command while it waited and released the reader. It verified `inhdetachpending`, then ran each `ONLY DROP CONSTRAINT` in a fresh rollback transaction. The detach-pending case succeeded. The attached case returned `42P16`. The original CLI reported `constraint_partition_parent_only` for both. The corrected CLI reported it only for the attached child. Catalog observations, the child row, and the guard sequence matched after the checks. The provider unit test pins the detach-pending filter.

- `/tmp/ds-d26-detach-pending-cli-v3/evidence.json`
- [PG17.7 ATExecDropConstraint](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [PG17.7 find_inheritance_children](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/pg_inherits.c)

## T37 cases

- `T37.meta.pg-drop-check-only-partition-root`
- `T37.meta.pg-drop-check-only-guarded`
- `T37.meta.pg-drop-check-only-cascade`
- `T37.meta.pg-drop-check-only-empty-root`
- `T37.meta.pg-drop-check-only-recursive`
- `T37.meta.pg-drop-check-only-inheritance-parent`
- `T37.meta.pg-drop-check-only-missing-guarded`
- `T37.meta.pg-drop-check-only-primary-control`
