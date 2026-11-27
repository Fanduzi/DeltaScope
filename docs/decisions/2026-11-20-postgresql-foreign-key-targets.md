# PostgreSQL 17 ADD FOREIGN KEY relation and column identity

Status Accepted

F09 D22 on `milestone/pg-alter-semantics`, following `53cc0592`.

## Decision

`ddl.pg.alter.add_foreign_key.targets.valid` defaults to blocker without parameters. It checks one table-level ADD FOREIGN KEY or ADD FOREIGN KEY NOT VALID action on observed PostgreSQL 17 and a fresh exact schema-qualified ordinary persistent non-typed source table. The parser preserves the decoded referencing column list, referenced table identity, database qualification, and explicit referenced column list in authored order. These facts remain private.

Use fresh exact schema-qualified relation and column evidence. Resolve the referenced relation before assessing column names. Known missing targets and ineligible relation kinds produce bounded findings. An ordinary or partitioned table kind establishes the relation premise. This slice uses only the ordinary persistent target column directory and reports a gap for other table shapes or persistence. An exact miss or observed system column on either side blocks. An omitted reference column list keeps the source check and requires separate referenced primary-key evidence. Missing or stale observations, unmapped or unqualified names, and three-part names remain gaps. Proven qualified guarded source absence skips this check.

Finding fields identify the action, source table, reason and affected column where needed. Existing FK forbid, NOT VALID and VALIDATE policies retain their scope. Scope excludes candidate-key eligibility, arity, casts or equality operators, grants, inheritance effects, validation of existing rows and execution simulation.

## Evidence

The baseline at `1bc8709c` paired 15 native statements with 30 real CLI audits. The follow-up at `53cc0592` paired 11 native statements with 22 real CLI audits. The second run verified exact comma-containing identifiers, system columns, relation lookup precedence, omitted lists, and unqualified, cross-database, partitioned and persistence boundaries. Each case used independent catalog, file, row and sequence oracles around audit and native rollback.

- `/tmp/ds-pg-f09-add-fk-targets-1bc8709c/evidence.json`
- `/tmp/ds-pg-d22-fk-identity-edges-53cc0592/evidence.json`

## Evidence bindings

Preserve all previous T37 cases and policy profiles. Add dedicated cases for offline and disabled policy, exact names (including a comma), missing and wrong-kind references, exact source and reference misses, system targets, omitted reference list, source guard, stale catalog, catalog-qualified, unqualified and partitioned reference gaps. Bind only the original ADD FOREIGN KEY cell at the PG17 anchor.

- `T37.cli.fk-targets-offline`
- `T37.cli.fk-targets-disabled`
- `T37.meta.pg-fk-targets-exact`
- `T37.meta.pg-fk-targets-reference-absent`
- `T37.meta.pg-fk-targets-reference-kind`
- `T37.meta.pg-fk-targets-source-column`
- `T37.meta.pg-fk-targets-reference-column`
- `T37.meta.pg-fk-targets-source-system`
- `T37.meta.pg-fk-targets-reference-system`
- `T37.meta.pg-fk-targets-omitted`
- `T37.meta.pg-fk-targets-omitted-source-missing`
- `T37.meta.pg-fk-targets-unqualified`
- `T37.meta.pg-fk-targets-catalog-qualified`
- `T37.meta.pg-fk-targets-partitioned-reference`
- `T37.meta.pg-fk-targets-unlogged-reference`
- `T37.meta.pg-fk-targets-stale`
- `T37.meta.pg-fk-targets-guarded-absent`
- `T37.meta.pg-fk-targets-validating`

`d4ccb0c0` retains the exact AST identities and binds private catalog facts.
`d0501883` adds the default blocker. The candidate real CLI comparison at
`d4ccb0c0` with the rule in the worktree reproduced 11 native outcomes and
checked all 22 audits before the native leg. Five cases exercised the new
blocker and five exercised the specific live scope gaps. The successful
comma-containing reference kept both identifiers intact.

Focused T37 passed 354/354 cases and 9,344 assertions, retaining the 336
existing cases and all prior profiles. Eighteen new cases include seventeen
native commands, with nine successes and eight exact refusals. The validator
passed. Nineteen selectors were added to ADD FOREIGN KEY at the PG17 anchor.
The inventory classifications and denominators were retained.

## Delivered evidence

`529d10b1b548a277ca74eb1200b2b1380ae8f721` passed 50 canonical tasks with 3,686/3,686 cases and 68,383
assertions. T37 passed 354/354 cases and 9,344 assertions. The validator,
read-only review and merged-root gates passed. The milestone advanced to
`main` by fast-forward and normal push. [Go, PostgreSQL and lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44694809416),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44694809506) and
[Pages](https://github.com/Fanduzi/DeltaScope/runs/136149005473) passed for this SHA.
The aggregate is `/tmp/ddl-golden-pg-foreign-key-targets-final-529d10b1/all/artifact.json`.

The original worktree status, protected release worktree and 16 previously
protected container identities matched. An additional container from the
starting snapshot was absent at review, and the cause remains unknown. All
50 cleanup receipts refer to the dedicated Golden project.

## Partitioned source with NOT VALID

PostgreSQL 17.7 rejects a single `ADD FOREIGN KEY … NOT VALID` on a
partitioned source table (`42809`). The existing blocker now reports
`partitioned_foreign_key_not_valid` when a fresh catalog observation binds
an explicitly schema-qualified source to `relkind = p`. This refusal does
not need a referenced-key lookup. The finding includes only the authored
source identity, action and reason. It does not expose catalog identifiers.

Native controls distinguish a partitioned source from an ordinary leaf
partition and distinguish `NOT VALID` from an initially validated foreign
key. The ordinary source and the qualified `IF EXISTS` miss retain their
existing behavior. General foreign-key checks on partitioned sources remain
unverified. The [T37 cases](../../testdata/ddl-golden/T37.json) bind only the
PostgreSQL 17 `ADD FOREIGN KEY` cell; no inventory classification changes.

## Sources

- [PostgreSQL 17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PostgreSQL 17.7 foreign-key identity resolution](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
