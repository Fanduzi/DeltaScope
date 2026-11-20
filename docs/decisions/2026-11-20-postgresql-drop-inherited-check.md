# PostgreSQL 17 direct DROP of inherited CHECK constraints

Status Accepted

F09 D25 on `milestone/pg-alter-semantics`, following D24. The user delegated decisions about default behavior. The implementation uses exact PostgreSQL 17.7 native and catalog evidence.

## Decision

Extend `ddl.pg.alter.drop_constraint.target.require` at its existing blocker level for a single exact schema-qualified `DROP CONSTRAINT` on a persistent ordinary or partitioned table on observed PostgreSQL 17. A CHECK target observed with `pg_constraint.coninhcount > 0` directly refuses. This includes a locally merged CHECK with `conislocal = true`, and the same refusal applies with `IF EXISTS` or `CASCADE`. A known zero inheritance count leaves this premise silent. Missing or stale inheritance proof reports `constraint.pg_inheritance` under `unproven_constraint_target` for a bound candidate in scope.

Reuse the constraint directory with the inheritance observation private and qualified to persistent ordinary or partitioned non-typed tables in the catalog query. Preserve the existing target check, RESTRICT dependency check and DROP advisory. Do not expose inheritance counters or dependent names in the finding. The same fix does not decide parent or ONLY behavior, child recursion, dependency closure, permissions, locks or actual execution.

## Native evidence

The PG17.7 sandbox at `508d69e3` paired ten native commands with twenty real CLI audits. Direct inherited DROP returned `42P16` on ordinary and partition children, with and without modifiers. The observed locally merged CHECK also refused. A local-only child CHECK and recursive DROP at the parent succeeded. Ordinary parent ONLY and partition parent ONLY have different native outcomes and remain separate. Catalog, files, rows and an independent sequence were verified after audit and native rollback.

- `/tmp/ds-pg-d25-drop-check-inheritance-508d69e3/evidence.json`
- [PG17.7 ATExecDropConstraint](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)

## T37 cases

- `T37.cli.drop-inherited-check-offline`
- `T37.cli.drop-inherited-check-disabled`
- `T37.meta.pg-drop-inherited-check-child`
- `T37.meta.pg-drop-inherited-check-guarded`
- `T37.meta.pg-drop-inherited-check-cascade`
- `T37.meta.pg-drop-inherited-check-merged`
- `T37.meta.pg-drop-inherited-check-partition`
- `T37.meta.pg-drop-inherited-check-local`
- `T37.meta.pg-drop-inherited-check-parent-recursive`
- `T37.meta.pg-drop-inherited-check-stale`

Keep all earlier case IDs, profiles, SQL and native checks. Add D25 references to DROP CONSTRAINT and DROP CONSTRAINT CASCADE only for the tested scope. Explain and assert any legacy batch expectation change.
