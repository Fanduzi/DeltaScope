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

## Sources

- [PG17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PG17.7 referenced-key selection and arity](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [PG17.7 live-index enumeration](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/utils/cache/relcache.c)
