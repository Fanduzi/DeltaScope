# Decision: PostgreSQL heap table option declarations

Date: 2026-11-18
Status: Accepted
Milestone: F09 / `milestone/pg-alter-semantics` (D10)
Product baseline: `cb2e7c360944a8716a20a8c73c378bc25440ae24`

## Decision

Enable `ddl.pg.alter.reloptions.valid` as a PostgreSQL blocker with no
parameters. Use actual PG17, a fresh exact schema-qualified binding, and an
observed ordinary persistent non-typed heap table. A missing observation,
unqualified or three-part target, different table kind, or unproven execution
order leaves `unproven_table_reloptions`. A proven qualified missing table
under `IF EXISTS` has no option premise.

Preserve ordered exact AST members and argument presence, including duplicate
keys and namespaces. On a bound table, SET checks `fillfactor` with an AST
integer against 10–100 and rejects a missing value for that option. Validate
`autovacuum_enabled` for observed Boolean input forms, including a bare name,
AST integer 0/1 and bounded ASCII Boolean strings or simple parsed names.
A verified `1.0` Boolean input refuses. Other numeric or quoted forms not
established by the model stay unverified. Repeated recognized keys in local
SET refuse. TOAST, extension parameters and unknown SET keys remain gaps.
An unvalued RESET can include unknown or repeated names. RESET with a value
refuses on this scope independently of which key was named.

Only the two known parameter names may be emitted by the new rule. Opaque
keys, arbitrary values, type text, catalog fields and provider errors stay out
of finding metadata. Existing SET and RESET advice remains independent and
describes the requested configuration, including the absence of an immediate
fillfactor rewrite. This does not assess permissions, table data, effective
autovacuum behavior or arbitrary extension parameters.

## Baseline evidence

At `cb2e7c36`, 38 native PG17.7 statements gave 23 successes and 15 exact
refusals. Eight additional native shapes were paired with 16 offline and live
CLI audits. Those audits retained SET advice and passed valued RESET with a
notice despite native `42601`. The AST retains name, namespace and argument
nodes, but the prior model kept only member counts. Native controls establish
that some decimal, rounded, scientific and alternate-base fillfactor forms
succeed, so treating them as invalid integers would overreject. The tests
independently compare heap and TOAST options, row/file state and a separate
sequence guard. Research artifacts include
`/tmp/ds-pg-d10-reloptions-cb2e7c36/evidence.json`,
`/tmp/ds-pg-d10-reloptions-cli-cb2e7c36/evidence.json` and
`/tmp/ds-pg-d10-reloptions-ast-expanded-cb2e7c36.log`.

## Frozen T37 additions

- `T37.cli.table-reloptions-offline`
- `T37.cli.table-reloptions-disabled`
- `T37.meta.pg-table-reloptions-fillfactor-controls`
- `T37.meta.pg-table-reloptions-fillfactor-range`
- `T37.meta.pg-table-reloptions-fillfactor-missing`
- `T37.meta.pg-table-reloptions-bool-controls`
- `T37.meta.pg-table-reloptions-bool-invalid`
- `T37.meta.pg-table-reloptions-duplicate-set`
- `T37.meta.pg-table-reloptions-reset-controls`
- `T37.meta.pg-table-reloptions-reset-valued`
- `T37.meta.pg-table-reloptions-opaque-values`
- `T37.meta.pg-table-reloptions-scope`
- `T37.meta.pg-table-reloptions-freshness`
- `T37.meta.pg-table-reloptions-guard-absent`

Old case IDs, SQL, findings and policy profiles stay in place. The two old
T37 offline SET cases use a profile that excludes the new blocker. The
new isolated offline case covers that blocker and its evidence gap. Canonical live
checks compare exact catalogs, file identity, payload and sequence position
before running independent native steps. Only the existing SET/RESET storage
parameters cell receives new evidence. Other F09 dimensions stay open.

## Evidence bindings

`1d088d18` retains the facts and `6efd1dce` provides the shared rule. The focused
candidate passed T37 203/203 with 4352 assertions. The added 14 cases include
12 owned live cases and 27 native controls, with 16 successes and 11 precise
refusals. All 189 original cases, profiles and their 3922 assertions remain.
Two earlier offline expectations had incorrectly assumed the fixed acceptance
profile enabled the new blocker. They were restored to their original values.
The new isolated profile proves the intended missing-fact behavior.

One early facts-only focused run reached the tool timeout. It left an owned
PostgreSQL container, which was removed with the project cleanup after verifying
it was the only owned residual and preserving all original container identities.
The retry passed 189/189 with 3922 assertions. Neither interrupted nor focused
evidence can replace the final fixed-SHA full run.

The original SET/RESET storage parameters anchor receives 15 additional
selectors. The matrix retains its original row and subaction classifications.

## Sources

- [PostgreSQL 17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PostgreSQL 17 CREATE TABLE storage parameters](https://www.postgresql.org/docs/17/sql-createtable.html)
- [PostgreSQL 17.7 reloption validation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/access/common/reloptions.c)
- [PostgreSQL 17.7 integer parsing](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/utils/misc/guc.c)
- [PostgreSQL 17.7 Boolean parsing](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/utils/adt/bool.c)
