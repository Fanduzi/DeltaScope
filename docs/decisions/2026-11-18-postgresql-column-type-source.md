# Decision: PostgreSQL ALTER COLUMN TYPE source preconditions

Date: 2026-11-18
Status: Accepted
Milestone: F09 / `milestone/pg-alter-semantics` (D14)
Product baseline: `33f87fd76b8b7e8e780d0165f3df77ec1c8ae5f9`

## Decision

Enable `ddl.pg.alter.column_type.source.valid` as a blocker without parameters.
Check one TYPE or TYPE USING action on an observed PostgreSQL 17 server with
fresh exact schema-qualified ordinary persistent non-typed table and column
evidence. The source name is decoded by the parser and compared exactly. Report
an observed absent source, a real system source, or USING on an observed stored
generated source. Presence of USING comes from the existing AST projection.
A proven qualified absent relation protected by IF EXISTS has no source
premise. Unknown version, binding, scope or directory, multiple actions,
and prior DDL or parser failures yield `unproven_column_type_source`.

The existing type-change policies retain their default levels. Rewrite advice
uses conditional wording and describes the remaining lock risk. These checks do
not prove destination type identity, typmod or collation, USING expression,
cast applicability, dependent objects, row values, or whether a table is
rewritten. Audit does not evaluate SQL expressions or inspect user data. New
findings expose a bounded reason and the qualified table and source column.

## Evidence

PostgreSQL 17.7 native research exercised 17 TYPE or TYPE USING shapes, with
eight successes and nine refusals. Seven additional shapes paired native
execution with 14 offline and metadata-aware CLI audits. The paired outcomes
include exact quoted source absence (`42703`), system source (`0A000`), and
USING on a generated source (`42611`). All CLI audits kept the existing review
warnings. Independent checks compared rows, files, catalogs and a sequence
guard after audit and native rollback. Even a same-type request can fail due
to dependencies. A same-type USING request can rewrite files.

- `/tmp/ds-pg-f09-type-using-a8767e9c/evidence.json`
- `/tmp/ds-pg-f09-type-target-cli-v2-a8767e9c/evidence.json`

## Frozen T37 additions

- `T37.cli.column-type-source-offline`
- `T37.cli.column-type-source-disabled`
- `T37.meta.pg-column-type-source-missing`
- `T37.meta.pg-column-type-source-system`
- `T37.meta.pg-column-type-source-generated-using`
- `T37.meta.pg-column-type-source-ordinary`
- `T37.meta.pg-column-type-source-scope`
- `T37.meta.pg-column-type-source-freshness`
- `T37.meta.pg-column-type-source-guard-absent`

Preserve the earlier T37 profiles and case definitions. Bind only the original
ALTER COLUMN TYPE and ALTER COLUMN TYPE USING inventory cells. The live cases
check exact catalogs, file identity, rows and an independent sequence before
running native controls. Existing parser and metadata facts do not require an
extractor or provider change for this slice.

## Evidence bindings

`588f0cd1` implements the shared source blocker and updates the rewrite advice.
The focused candidate passed T37 245/245 with 5589 assertions, preserving all
236 prior cases and policy profiles. Nine additions include nine native steps:
four successes and five exact refusals. The catalog, file, payload and sequence
oracles run independently before native execution. The final fixed-SHA run is
required for delivery.

Evidence references bind the original ALTER COLUMN TYPE and ALTER COLUMN TYPE
USING anchors. Existing row and subaction classifications remain intact.

## Sources

- [PostgreSQL 17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PostgreSQL 17.7 source column preconditions](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
