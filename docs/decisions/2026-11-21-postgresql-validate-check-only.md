# PostgreSQL 17 parent-only CHECK validation

Status Accepted

D38 extends the existing `ddl.pg.alter.validate_constraint.target.require`
blocker. For a single `ALTER TABLE ONLY ... VALIDATE CONSTRAINT` on a fresh
exact PostgreSQL 17 ordinary or partitioned table, an unvalidated CHECK that
propagates to attached children requires validation through the parent and
children. The provider binds the exact constraint, `connoinherit`, validation
state and attached child presence. Detach-pending entries are excluded. An
unknown applicable observation yields `constraint.pg_validation_children`.
Already validated CHECKs, NO INHERIT CHECKs and foreign keys retain their
respective native paths. The finding keeps descendant identities private.

Nine native PostgreSQL 17.7 transactions and actual CLI audits distinguish
ordinary and partitioned parent-only refusals, valid recursion, a parent
without children, an independently validated child, NO INHERIT and the FK
path. Six native transactions succeeded and three returned `42P16`. The
child-first batch keeps the parent prerequisite unresolved in the audit
because the prior DDL invalidates the catalog snapshot. Catalog and file
identity, fixture rows and the guard sequence matched after rollback.

This premise describes validation traversal. Verifying all existing rows,
execution permissions and locks requires separate observations.

- `/tmp/ds-pg-f09-validate-only-cli-b49be2d4/evidence.json`
- [PostgreSQL 17.7 CHECK validation traversal](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)

The combined preview passed all 497 T37 cases with 14,257 assertions. The inventory records nine exact PostgreSQL 17 selectors for this subaction.
