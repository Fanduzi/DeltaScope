# PostgreSQL partition-derived foreign-key deferrability

Status Accepted

## Decision

Extend `ddl.pg.alter.constraint_deferrability.target.require` to single-action ALTER CONSTRAINT on an exact, qualified PostgreSQL 17 target. A persistent partition's foreign key with a nonzero `pg_constraint.conparentid` is a derived constraint. PostgreSQL refuses a direct deferrability change with `XX000`, including requests that repeat the current flags and requests with `ONLY`. Missing and wrong-kind constraints retain their earlier findings.

The provider carries a private boolean from the same constraint lookup. The finding exposes the authored target and `constraint_deferrability_derived_foreign_key`. The parent identity and catalog linkage stay private. A positive derivation fact without fresh qualified relation identity leaves an evidence gap. Local and root foreign keys keep their existing audit behavior. Root propagation, trigger changes, execution permissions, and multiple ALTER actions remain outside this bounded comparison.

## Verification

The PostgreSQL 17.7 native and real CLI controls cover three direct refusals, successful local and root changes, and existing missing and wrong-kind precedence. The native catalog query checks the root, derived and local flags. The audit runs before the native statement and preserves those flags, the witness row and the independent sequence.

- [T37 canonical comparisons](../../testdata/ddl-golden/T37.json)
- [Corpus fixture](../../testdata/sql-corpus/postgresql/ddl/findings/constraint_deferrability_derived_foreign_key.expected.yaml)
- [PostgreSQL 17.7 constraint alteration implementation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c#L11406)

The isolated T37 preview passed 622 cases and 18,503 assertions. Ten selectors bind the PostgreSQL 17 deferrability cell.
