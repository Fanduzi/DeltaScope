# PostgreSQL 17 foreign-key referenced keys

Status Accepted

F09 D23 on `milestone/pg-alter-semantics`, based on delivered D22.

## Decision

Extend the existing `ddl.pg.alter.add_foreign_key.targets.valid` blocker after
its relation and column identity checks. In the same observed PG17 fresh
ordinary-table scope, explicit referenced columns must resolve to a live,
valid, immediate, unique index with no predicate or expression. Compare the
first `indnkeyatts` slots as an exact set. Preserve the original SQL order for
identity assessment. INCLUDE columns never contribute to the key. A qualifying
immediate index wins over an otherwise matching deferred candidate. Repeated
referenced columns block. Repeated source columns still count toward arity.

If REFERENCES omits its columns, use the observed live valid primary index.
A standalone UNIQUE index alone is insufficient. A deferrable primary key
blocks. Once the referenced key resolves, compare its number of key columns
with the number of authored source columns. Allow `NULLS NOT DISTINCT` on a
qualifying referenced unique index. A missing or incomplete key directory
remains an evidence gap. A complete empty directory can prove absence.

Use a targeted parameterized `pg_catalog` query for one exact reference,
with key properties and ordered decoded attribute names kept private. Output
adds bounded reasons and retains the existing finding shape. Casts,
operator-class/equality compatibility, referential action semantics, grants,
partition and inheritance effects, existing data, and execution remain
separate premises. Keep independent FK forbid and NOT VALID policies.

## Evidence

The 12 additional PG17.7 native cases at `d4ccb0c0` paired with 24 real CLI
audits. Five passed. Four refused with `42830`, two with `55000`, and one with
`42704`. Independent fixture state was verified after audit and native
rollback. Earlier D22 research also established arity and reordered matching.

- `/tmp/ds-pg-d23-fk-key-edges-d4ccb0c0/evidence.json`
- `/tmp/ds-pg-f09-add-fk-targets-1bc8709c/evidence.json`

Preserve all 354 prior T37 cases and profiles. The previous D22 omitted-list
case now needs an expectation for the proved primary key. Retain its exact
SQL, fixture, case ID and native checks. New isolated cases bind only to ADD
FOREIGN KEY at the PG17 anchor.

## New T37 cases

- `T37.cli.fk-keys-offline`
- `T37.cli.fk-keys-disabled`
- `T37.meta.pg-fk-keys-include`
- `T37.meta.pg-fk-keys-include-is-not-key`
- `T37.meta.pg-fk-keys-implicit-no-primary`
- `T37.meta.pg-fk-keys-implicit-deferred-primary`
- `T37.meta.pg-fk-keys-explicit-deferred`
- `T37.meta.pg-fk-keys-immediate-over-deferred`
- `T37.meta.pg-fk-keys-nulls-not-distinct`
- `T37.meta.pg-fk-keys-invalid`
- `T37.meta.pg-fk-keys-partial-and-expression`
- `T37.meta.pg-fk-keys-duplicate-reference`
- `T37.meta.pg-fk-keys-reordered-repeated-source`
- `T37.meta.pg-fk-keys-arity`

## Evidence bindings

`83577403` adds the private key directory. `75319194` applies it in the
existing blocker. A real PG17.7 provider probe confirmed ordered key slots,
known empty directories and exclusion of live invalid indexes. The CLI
comparison at `83577403` with the rule in the worktree matched 12 native
outcomes, including seven specific refusals and five accepted key premises.
See `/tmp/ds-pg-d23-key-candidate-cli-83577403/evidence.json`.

Focused T37 passed 368/368 cases and 9,807 assertions. Fourteen additional
cases include twelve native commands, with four successes and eight exact
refusals. The existing omitted-list case now proves the actual referenced
primary key with its original SQL, ID, fixture and native oracles. All 354
previous IDs and profiles remain. Fifteen selectors append to ADD FOREIGN KEY
at the PG17 anchor. Inventory classifications and denominators are preserved.

## Sources

- [PG17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PG17.7 referenced-key selection and arity](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [PG17.7 live-index enumeration](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/utils/cache/relcache.c)
