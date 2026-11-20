# PostgreSQL 17 constraint attachment using an index

Status Accepted

F09 D21 on `milestone/pg-alter-semantics`, following `5799cd4d`.

## Decision

`ddl.pg.alter.constraint_using_index.valid` defaults to blocker without parameters for one `ADD PRIMARY KEY` or `ADD UNIQUE` action using an existing index. The check requires observed PostgreSQL 17, a fresh exact schema-qualified ordinary persistent non-typed table, and a matching index observation. The parser preserves the decoded index name privately alongside the existing `using_index` flag. A proven qualified absent table with `IF EXISTS` skips index assessment.

The PostgreSQL provider reads a candidate relation in the table's exact namespace. It checks index kind, ownership by the target table, validity, uniqueness, built-in btree, expression and predicate presence, and sort flags, column collation and options for key slots only. An internal dependency from the index to a constraint proves the index is constraint-owned. A foreign key may reference a standalone unique index without owning it. `NULLS NOT DISTINCT` is refused for PRIMARY KEY and permitted by this premise for UNIQUE. Unknown binding, freshness or needed key attributes yields `unproven_constraint_using_index`.

The output contains the table, constraint kind, action and bounded reason. The private index name and catalog fields remain excluded from JSON. Default operator-class resolution, nullable primary-key rows, permissions, locking, other relation kinds, name conflicts for the newly declared constraint, and execution remain separate premises. Existing D3 migration advice and replica identity policy keep their scope.

## Evidence

At `1bc8709c`, 14 native PG17.7 steps and 28 real CLI audits established exact names, table ownership, invalid kinds and shapes, owned indexes and nullable-key behavior. At `5799cd4d`, eight native steps and sixteen CLI audits established collation, ordering, opclass, invalid builds, foreign-key references and `NULLS NOT DISTINCT`. Independent checks compared catalog and file identity, data, and a guard sequence around audit and native rollback. The existing CLI passed these attachment probes before the new rule.

- `/tmp/ds-pg-f09-constraint-using-index-1bc8709c/evidence.json`
- `/tmp/ds-pg-d21-constraint-index-edges-5799cd4d/evidence.json`

## Frozen T37 additions

- `T37.cli.constraint-index-offline`
- `T37.cli.constraint-index-disabled`
- `T37.meta.pg-constraint-index-pk-eligible`
- `T37.meta.pg-constraint-index-unique-referenced`
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

## Sources

- [PG17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PG17.7 transformIndexConstraint](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_utilcmd.c)
- [PG17.7 get_index_constraint](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/pg_depend.c)
- [PG17.7 primary index checks](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/index.c)
