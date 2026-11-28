# PostgreSQL 17 named ADD CHECK constraint collisions

Date 2026-11-19

Status Accepted

F09 D19 on `milestone/pg-alter-semantics`.

## Decision

Enable `ddl.pg.alter.add_check.name.available` as a blocker without parameters
for a single explicitly named `ADD CHECK` or `ADD CHECK NOT VALID` on a
fresh, exact, schema-qualified ordinary persistent non-typed table in
observed PostgreSQL 17. An existing exact name of any supported catalog
constraint kind blocks. Unnamed CHECK delegates naming to PostgreSQL. A
proven absent qualified table with `IF EXISTS` skips the name premise.
Unknown version, directory, table scope or statement order yields
`unproven_check_constraint_name`. The existing CHECK NOT VALID policy and
batch validation rule each retain their own scope. Within the same PG17
table scope, omit the shared add-index name check for a single named CHECK
declaration. A CHECK does not create an index, and a case-folded primary
key name can otherwise cause a false blocker even when the exact
constraint name is free.

Use the private `pg_constraint` directory. Bind its relation and namespace
comparisons to `pg_catalog` operators so a session operator cannot substitute
another table's constraints. Compare decoded names exactly
and retain unknown/known-empty and ambiguity distinctions. Restrict new
findings to table, schema, constraint identity, action and bounded reason.
A matching catalog entry proves name occupation. Expression resolution,
validation of existing rows, recursive and inherited merge behavior,
permissions and other DDL effects remain separate.

## Research evidence

The baseline at `36b6957a` with pending D18 work paired ten native PG17.7
statements with 20 real CLI audits. Four native statements succeeded and
six refused: three `42710`, one `23514`, one `42703` and one `42804`.
The duplicate-check premise is visible independently of the existing NOT
VALID advisory. A same-expression CHECK with an unused or generated name
can succeed. The fixture's catalog, files, data and independent sequence
were checked after audit and rollback.

- `/tmp/ds-pg-f09-add-check-36b6957a/evidence.json`
- `/tmp/ds-pg-f09-check-name-readonly-summary.md`

A second five-case native/CLI comparison at `1bc8709c` showed that CHECK may
share an exact index-only name. It may also differ from a primary constraint
only in case. The live CLI incorrectly blocked the latter via
`ddl.alter.add_index.exists.forbid`. The exact primary name returns `42710`.

- `/tmp/ds-pg-f09-check-index-namespace-1bc8709c/evidence.json`
- `/tmp/ds-pg-d19-constraint-source/qualified.log`

## Frozen T37 additions

- `T37.cli.add-check-name-offline`
- `T37.cli.add-check-name-disabled`
- `T37.meta.pg-add-check-name-duplicate`
- `T37.meta.pg-add-check-name-duplicate-not-valid`
- `T37.meta.pg-add-check-name-other-kind`
- `T37.meta.pg-add-check-name-exact-control`
- `T37.meta.pg-add-check-name-unnamed`
- `T37.meta.pg-add-check-name-inherited`
- `T37.meta.pg-add-check-name-guarded-absent`
- `T37.meta.pg-add-check-name-scope-order`

Keep all previous cases and profiles. Bind exact evidence only to ADD CHECK
and ADD CHECK NOT VALID at the PG17 anchor. Native controls and audit
oracles use owned relations and no execution through the audit path.

## Evidence bindings

`99162180` binds the constraint directory comparisons to PostgreSQL system
operators. `9055f9fd` adds the approved blocker and corrects the overlapping
index rule. Five live CLI comparisons at `99162180` with the rule worktree
matched the native name outcomes across both namespaces. The focused T37
candidate passed 309/309 cases and 7,870 assertions, with all 299 earlier
cases and profiles preserved. Ten new native commands produced six successes
and four exact `42710` refusals. The validator passed. Six selectors attach
to ADD CHECK and six to ADD CHECK NOT VALID. Inventory classifications and
the original denominator are preserved.

## Delivered evidence

`64a1fd4e77556f54dce08f6f84ce57ba73ab92cd` passed 50 canonical tasks with 3,641/3,641 cases and 66,909
assertions. T37 passed 309/309 with 7,870 assertions. The validator,
independent review and merged-root gates passed. The milestone advanced to
`main` by fast-forward and normal push. [Go, PostgreSQL and lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44652137227),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44652137340) and
[Pages](https://github.com/Fanduzi/DeltaScope/runs/136009522204) passed for that SHA.
The aggregate is `/tmp/ddl-golden-pg-check-constraint-name-final-64a1fd4e/all/artifact.json`.

The full worktree status and protected release worktree matched. Golden's
cleanup receipts were scoped to its own Compose project. Docker destroy
events put the removal of eleven `binlog-h1run21` containers more than two
hours after Golden finished. The five other baseline container identities
matched. The event record is `/tmp/ds-pg-d19-external-container-events.json`.

## Unnamed TRUE checks on partition roots

For one unnamed `ADD CHECK (TRUE)` on a fresh, schema-qualified persistent PostgreSQL 17 partition root, the existing blocker rejects `NO INHERIT`, even without children, and `ONLY` when a direct partition is attached. `NOT VALID` does not bypass either precondition. PostgreSQL 17.7 returns `42P16` for both. Named constraints and other predicates are outside the extension: their analysis or a name collision can fail before the traversal check.

The parser and provider retain only private Boolean facts. The rule does not evaluate expressions, scan rows, or predict child changes. The `pg-partition-check-*` [T37 controls](../../testdata/ddl-golden/T37.json) compare the refusals with native success on a childless root, a leaf and a recursive root addition; the [corpus](../../testdata/sql-corpus/postgresql/ddl/findings/partition_check_true.expected.yaml) binds both affected subactions. Inventory classifications and denominators are unchanged.

## Sources

- [PostgreSQL 17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PostgreSQL 17.7 ADD CHECK](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [PostgreSQL 17.7 constraint name checks](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/heap.c)
