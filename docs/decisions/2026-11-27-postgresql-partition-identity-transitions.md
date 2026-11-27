# PostgreSQL 17 identity changes on partition targets

Status: Accepted

Reuse `ddl.pg.alter.column.generation.transition.valid` for single-action `ADD IDENTITY`, `DROP IDENTITY`, `SET GENERATED`, and decoded identity sequence options and `RESTART`. The rule emits `identity_partition_target_inapplicable` only after observing a persistent schema-qualified ordinary leaf partition or an explicit `ONLY` on a persistent partition root. PostgreSQL 17.7 refuses those targets with `42P16` before the column lookup: a root need not have children, and `DROP IDENTITY IF EXISTS` does not suppress the relation-level refusal.

Bind a private boolean in the existing column traversal lookup; do not infer identity sequence ownership, issuance state, or descendant effects. A proved refusal suppresses dependent options gaps and restart/cycle advice, rather than claiming those checks succeeded. Recursive changes on roots, unproven targets and other relation shapes retain their previous behavior. No new rule, default, or inventory classification is introduced.

The [T37 controls](../../testdata/ddl-golden/T37.json) exercise both relation shapes, guarded DROP precedence, native root successes and an absent-table guard. They check that audit does not mutate identity attributes, sequence state or witness rows. The [corpus fixture](../../testdata/sql-corpus/postgresql/ddl/findings/partition_identity_transition.expected.yaml) covers the four PostgreSQL 17 inventory cells. See `ATExecAddIdentity`, `ATExecSetIdentity` and `ATExecDropIdentity` in [PostgreSQL 17.7 source](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c).
