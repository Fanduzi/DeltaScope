# PostgreSQL 17 ALTER COLUMN TYPE destination identity

Date 2026-11-19

Status Accepted

Milestone F09 D18 on `milestone/pg-alter-semantics`, after `6b394bd8`.

## Decision

`ddl.pg.alter.column_type.destination.valid` is a default blocker without
parameters for one TYPE or TYPE USING action on observed PostgreSQL 17. It
reuses D14's fresh, exact, schema-qualified ordinary persistent non-typed
table and source column precondition. Proven earlier source failure or
qualified guarded table absence suppresses destination assessment. Unknown
source, version or destination facts yield `unproven_column_type_destination`.

Preserve the exact two-part destination from the parser and whether an array
was requested. A single read in the observed connection resolves the schema,
base name and the actual array OID when requested. Verified missing schema,
missing type, missing associated array, unresolved shell and direct pseudo
type produce distinct reason codes. A true array's immediate pseudo-type
element is also in scope. Existing valid-looking destinations are silent for
this premise. The new findings omit authored type names, type modifiers,
expressions and catalog identifiers. The private observations stay out of JSON.

Unqualified names, three-part names and temporary schemas remain evidence
gaps. The parser still rejects TYPE references using `%TYPE` in this
statement. Type modifiers, collations, USING results, casts, privileges,
recursive type legality, dependencies, rows and physical rewrites have their
own premises. Preserve D14 source checks, existing type policy and rewrite
advice.

## Evidence

The real PG17.7/CLI baseline paired 20 native statements with 40 audits at
`6b394bd8`. Native outcomes were seven successes, five `42704`, three
`42P16`, two `3F000`, two `0A000` and one `42601`. Exact spaced/cased domain
names and arrays succeeded. The CLI treated destination refusals as migration
review, without destination-specific evidence. Audit and native rollback
preserved catalog and file identities, payload and an independent sequence.
See `/tmp/ds-pg-f09-type-destination-v2-6b394bd8/evidence.json` and the AST
probe `/tmp/ds-pg-d18-destination-ast.log`.

## Frozen T37 additions

- `T37.cli.column-type-destination-offline`
- `T37.cli.column-type-destination-disabled`
- `T37.meta.pg-column-type-destination-missing-schema`
- `T37.meta.pg-column-type-destination-missing-type`
- `T37.meta.pg-column-type-destination-shell`
- `T37.meta.pg-column-type-destination-missing-array`
- `T37.meta.pg-column-type-destination-pseudo`
- `T37.meta.pg-column-type-destination-pseudo-array`
- `T37.meta.pg-column-type-destination-exact`
- `T37.meta.pg-column-type-destination-using`
- `T37.meta.pg-column-type-destination-using-missing`
- `T37.meta.pg-column-type-destination-scope`
- `T37.meta.pg-column-type-destination-three-part`
- `T37.meta.pg-column-type-destination-temp-alias`
- `T37.meta.pg-column-type-destination-freshness`
- `T37.meta.pg-column-type-destination-source-missing`
- `T37.meta.pg-column-type-destination-guard-absent`

Keep all prior T37 cases and profiles. Bind exact evidence only to ALTER
COLUMN TYPE and ALTER COLUMN TYPE USING at the PG17 anchor. Keep independent
catalog, file, payload and sequence oracles and separate native execution.

## Evidence bindings

`36b6957a` binds private exact destinations and shares the D14 source-column
premise. `e124c553` adds the default blocker across SDK, CLI, HTTP and MCP. A
real CLI run with those rule changes in the worktree at `36b6957a` matched all
nine approved destination refusals. See
`/tmp/ds-pg-d18-candidate-cli-36b6957a/evidence.json`.

The first focused T37 iteration passed 294/297 cases. Three new assertions
used the source-rule gap code or placed a target-scope test after a prior DDL.
After correcting the gap code and isolating unqualified, three-part and
`pg_temp` cases, T37 passed 299/299 cases and 7,604 assertions. Eighteen native
commands produced six successes and twelve exact refusals. The 282 previous
cases and profiles were preserved. Fifteen selectors were added to the
original TYPE cell and three to TYPE USING. Classifications were preserved.

## Sources

- [PostgreSQL 17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PostgreSQL 17.7 type resolution](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_type.c)
- [PostgreSQL 17.7 ALTER type preparation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [PostgreSQL 17.7 attribute type checks](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/heap.c)
