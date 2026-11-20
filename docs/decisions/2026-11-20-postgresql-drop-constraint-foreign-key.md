# PostgreSQL 17 direct foreign-key blockers for DROP CONSTRAINT

Status Accepted

F09 D24 on `milestone/pg-alter-semantics`, following delivered D23 `c34c6e8a`.

## Decision

Extend `ddl.pg.alter.drop_constraint.target.require` at its existing blocker level. For one `DROP CONSTRAINT` with default or explicit RESTRICT on a fresh, exact, schema-qualified ordinary persistent non-typed table on observed PostgreSQL 17, check a bound primary or unique constraint for direct foreign-key dependencies. Use the supporting index's internal dependency on the selected constraint and the foreign key's normal dependency on that exact index. Self-referential and unvalidated foreign keys count. Alternate unique indexes on the same columns do not answer for the selected index.

An observed dependency reports `constraint_referenced_by_foreign_key`. A missing or mismatched dependency observation for a bound key reports `constraint.pg_foreign_key_references` under `unproven_constraint_target`. The check uses no dependent names or catalog identifiers in findings. CASCADE and guarded absence retain their existing semantics. Inheritance, partition handling, dependency closure, permissions, and locks remain separate premises.

## Native evidence

At `c34c6e8a`, nine native PostgreSQL 17.7 commands and eighteen real CLI audits distinguished default and explicit RESTRICT, CASCADE, self-reference, alternate indexes, and guarded exact-name absence. Six native commands succeeded and three returned `2BP01`. Catalog identities, payloads, files and an independent sequence matched after audit and native rollback. The provider probe separately checked exact names, supporting-index identity and self-reference.

- `/tmp/ds-pg-d24-drop-key-dependency-v2-c34c6e8a/evidence.json`
- `/tmp/ds-pg-d24-actual-provider.log`
- [PostgreSQL 17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PG17.7 constraint dependency recording](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/pg_constraint.c)
- [PG17.7 constraint index ownership](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/index.c)

## Evidence bindings

`508d69e3` observes the exact supporting index and its direct foreign-key
reference. `cfd0b1ab` filters RESTRICT results through the existing blocker
and evidence gap. Focused T37 passed 377/377 with 10,087 assertions.
The nine additions exercised eight native commands, with four successes
and four `2BP01` refusals. Ten selectors attach only to the PostgreSQL 17
DROP CONSTRAINT cell. Inventory classifications and denominators are
preserved.

The first run exposed a duplicate test privacy marker, a broken COPY pipe
in a legacy tablespace fixture, and two expected freshness gaps missing
from the legacy DROP batch. The verified manifest uses distinct privacy
markers, runs the existing directory-creation commands without sending COPY
rows, and asserts both freshness gaps and their exact source positions.
All 368 prior case IDs, profiles, SQL inputs and native oracles are retained.

Real CLI comparison: `/tmp/ds-pg-d24-candidate-cli-508d69e3/evidence.json`.

## T37 cases

- `T37.cli.drop-fk-reference-offline`
- `T37.cli.drop-fk-reference-disabled`
- `T37.meta.pg-drop-fk-reference-primary-restrict`
- `T37.meta.pg-drop-fk-reference-unique-restrict`
- `T37.meta.pg-drop-fk-reference-self-reference`
- `T37.meta.pg-drop-fk-reference-cascade`
- `T37.meta.pg-drop-fk-reference-other-index`
- `T37.meta.pg-drop-fk-reference-guarded-absence`
- `T37.meta.pg-drop-fk-reference-stale`
