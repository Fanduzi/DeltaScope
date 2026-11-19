# Decision: PostgreSQL ordinary-table access method destination

Date: 2026-11-18
Status: Accepted
Milestone: F09 / `milestone/pg-alter-semantics` (D12)
Product baseline: `42259221dd283c5a46f87934a9806716ad44fae2`

## Decision

Enable `ddl.pg.alter.access_method.target.valid` as a blocker without
parameters. It checks a single `ALTER TABLE … SET ACCESS METHOD` on an
observed PostgreSQL 17 server with a fresh, exact schema-qualified ordinary
persistent non-typed table. Shared and mapped relations are excluded at the
catalog read. Earlier DDL or parse failure, unknown or misbound facts, multi-
action ALTERs, unqualified and three-part bindings and other relation kinds
leave `unproven_table_access_method`. A proven qualified absent target
protected by `IF EXISTS` has no destination premise.

The AST already distinguishes a named method from the bare `DEFAULT` token.
Resolve a named method exactly, preserving spaces and case. Bare `DEFAULT`
uses the current effective `default_table_access_method` from the same query.
The `pg_am` row proves the method's kind. An absent method or a method proven
to be an index AM is a refusal. The same-method comparison uses the table's
actual catalog method and the observed destination. The existing warning
retains its level and metadata, with no-rewrite wording when sameness is
proved for the observed configuration.

Catalog observations stay private. New findings expose only action, table
identity and a bounded reason. Query method metadata without invoking or
installing a handler. Partition root semantics, plugin behavior, permission,
locking, rewrite cost and actual execution remain separate dimensions.

## Baseline evidence

At `34ec33f6`, eight PG17.7 native shapes and 16 real offline and metadata
CLI audits showed legal and refused forms sharing the legacy warning. Named
heap and bare `DEFAULT` did not rewrite the ordinary heap. The missing,
quoted `"Heap"`, and quoted `"default"` names returned `42704`. Using the
index method `btree` returned `55000`. Partition root selection changed root
metadata while existing partition file identity stayed unchanged. The driver
checked catalog, file identity, rows and a sequence guard before and after the
audit and native rollback. See
`/tmp/ds-pg-f09-access-method-cli-34ec33f6/evidence.json`.

## Frozen T37 additions

- `T37.cli.table-access-method-offline`
- `T37.cli.table-access-method-disabled`
- `T37.meta.pg-table-access-method-missing`
- `T37.meta.pg-table-access-method-index-kind`
- `T37.meta.pg-table-access-method-same`
- `T37.meta.pg-table-access-method-default`
- `T37.meta.pg-table-access-method-quoted-default`
- `T37.meta.pg-table-access-method-scope`
- `T37.meta.pg-table-access-method-freshness`
- `T37.meta.pg-table-access-method-guard-absent`

Retain all existing cases and profiles. Bind only the original `SET ACCESS
METHOD` inventory evidence cell. Each real CLI audit has independent catalog,
file, data and sequence checks before the native leg.

## Evidence bindings

`e64fd4bb` binds private catalog facts. `683ffd81` adds the shared blocker and
same-method wording to the existing warning. The focused candidate passed T37
223/223 with 4943 assertions. All 213 original case objects and profiles remain
exact. Ten additions under an isolated profile include eleven native steps,
with seven successes and four exact SQLSTATE refusals. Catalog, file, payload
and sequence checks prove the audit leaves the fixture intact. The fixed-SHA
full run is required for delivery.

Eleven selectors bind the original `SET ACCESS METHOD` cell. The generated
matrix preserves the row and subaction classifications and denominators.

## Sources

- [PostgreSQL 17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PostgreSQL 17 access method catalog](https://www.postgresql.org/docs/17/catalog-pg-am.html)
- [PostgreSQL 17 client defaults](https://www.postgresql.org/docs/17/runtime-config-client.html)
- [PostgreSQL 17.7 method selection](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [PostgreSQL 17.7 method lookup](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/amcmds.c)
