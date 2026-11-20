# PostgreSQL 17 constraint attachment using an index

Status Accepted

F09 D21 on `milestone/pg-alter-semantics`, following `5799cd4d`.

## Decision

`ddl.pg.alter.constraint_using_index.valid` defaults to blocker without parameters for one `ADD PRIMARY KEY` or `ADD UNIQUE` action using an existing index. The check requires observed PostgreSQL 17, a fresh exact schema-qualified ordinary persistent non-typed table, and a matching index observation. The parser preserves the decoded index name privately alongside the existing `using_index` flag. A proven qualified absent table with `IF EXISTS` skips index assessment.

The PostgreSQL provider reads a candidate relation in the table's exact namespace. It checks index kind, ownership by the target table, validity, uniqueness, built-in btree, expression and predicate presence, and sort flags, column collation and options for key slots only. An internal dependency from the index to a constraint proves the index is constraint-owned. A foreign key may reference a standalone unique index without owning it. `NULLS NOT DISTINCT` is refused for PRIMARY KEY and permitted by this premise for UNIQUE. Unknown binding, freshness or needed key attributes yields `unproven_constraint_using_index`.

The output contains the table, constraint kind, action and bounded reason. When a fresh observation binds the selected index to the table and the explicit constraint name matches the index name, the existing add-index name rule skips its collision check. PostgreSQL attaches under that name without renaming the index. A different constraint name still reaches the existing check. The private index name and catalog fields remain excluded from JSON. Default operator-class resolution, nullable primary-key rows, permissions, locking, other relation kinds, name conflicts for the newly declared constraint, and execution remain separate premises. Existing D3 migration advice and replica identity policy keep their scope.

## Evidence

At `1bc8709c`, 14 native PG17.7 steps and 28 real CLI audits established exact names, table ownership, invalid kinds and shapes, owned indexes and nullable-key behavior. At `5799cd4d`, eight native steps and sixteen CLI audits established collation, ordering, opclass, invalid builds, foreign-key references and `NULLS NOT DISTINCT`. Independent checks compared catalog and file identity, data, and a guard sequence around audit and native rollback. The existing CLI passed these attachment probes before the new rule.

- `/tmp/ds-pg-f09-constraint-using-index-1bc8709c/evidence.json`
- `/tmp/ds-pg-d21-constraint-index-edges-5799cd4d/evidence.json`

## Frozen T37 additions

- `T37.cli.constraint-index-offline`
- `T37.cli.constraint-index-disabled`
- `T37.meta.pg-constraint-index-pk-eligible`
- `T37.meta.pg-constraint-index-unique-referenced`
- `T37.meta.pg-constraint-index-rename-conflict`
- `T37.meta.pg-constraint-index-absent`
- `T37.meta.pg-constraint-index-wrong-kind`
- `T37.meta.pg-constraint-index-wrong-table`
- `T37.meta.pg-constraint-index-owned`
- `T37.meta.pg-constraint-index-invalid`
- `T37.meta.pg-constraint-index-nonunique`
- `T37.meta.pg-constraint-index-expression`
- `T37.meta.pg-constraint-index-partial`
- `T37.meta.pg-constraint-index-key-defaults`
- `T37.meta.pg-constraint-index-collation`
- `T37.meta.pg-constraint-index-null-distinct`
- `T37.meta.pg-constraint-index-null-distinct-unique`
- `T37.meta.pg-constraint-index-freshness`
- `T37.meta.pg-constraint-index-guarded-absent`

Keep the prior 317 cases and policy profiles. Add evidence only to the original ADD PRIMARY KEY and ADD UNIQUE cells. Native controls require exact SQLSTATE, independent catalog readback, and no execution through audit.

## Evidence bindings

`4f91cc2f` captures the private selection and catalog observation. `a47025da`
adds the default blocker and the same-name attachment correction. In a
separate real CLI comparison at the facts commit with the rule in the
worktree, three native same-name or unnamed attachments passed. An occupied
rename destination returned `42P07` and retained the existing blocker.
See `/tmp/ds-pg-d21-existing-name-fixed-4f91cc2f/evidence.json`.

The first focused T37 run passed 333 of 335 cases. The invalid-index setup
used a psql input mode unsupported by the setup runner, and the existing
named tablespace fixture failed with a broken pipe. The second run used the
established concurrent build setup and passed the earlier tablespace case
with its original assertions. It passed 336/336 cases and 8,751 assertions
with the same-name and rename controls. Eighteen new native commands produced
six successes and twelve precise refusals. The invalid-index setup separately
proves a failed build left a live invalid index. Five selectors attach to
ADD PRIMARY KEY and fifteen to ADD UNIQUE. All 317 earlier cases and policy
profiles, and the inventory classifications and denominators, were preserved.

## Delivered evidence

`53cc0592051ceca225b1463d69b1d8b0ce1ebf3c` passed all 50 canonical tasks with 3,668/3,668 cases and
67,790 assertions. T37 passed 336/336 cases and 8,751 assertions. The validator,
independent review and merged-root gates passed. The milestone advanced to
`main` by fast-forward and normal push. [Go, PostgreSQL and lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44685529669),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44685529712) and
[Pages](https://github.com/Fanduzi/DeltaScope/runs/136118934239) passed for this SHA.
The aggregate is `/tmp/ddl-golden-pg-constraint-using-index-final-53cc0592/all/artifact.json`.

The original worktree status, protected release worktree and sixteen protected
Docker identities matched. One additional container in the starting snapshot
was absent at review. Its lifecycle could not be established. All fifty
cleanup receipts refer to the dedicated Golden project.

## Sources

- [PG17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PG17.7 transformIndexConstraint](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_utilcmd.c)
- [PG17.7 get_index_constraint](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/pg_depend.c)
- [PG17.7 primary index checks](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/index.c)
