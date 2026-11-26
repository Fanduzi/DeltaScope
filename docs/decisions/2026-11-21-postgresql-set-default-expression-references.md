# PostgreSQL 17 SET DEFAULT expression references

Status Accepted

F09 D40 extends `ddl.pg.alter.column.generation.transition.valid` when an
observed PostgreSQL 17 ordinary column is eligible for SET DEFAULT. PostgreSQL
refuses a raw column reference or a subquery in this expression. The extractor
keeps a private boolean and walks the existing parser AST. A type-name subtree
can contain a legal type modifier written as an identifier, so the walker skips
that subtree. An absent, system, identity or generated column keeps its prior
target assessment. A missing column catalog keeps the existing evidence gap.

Eleven PostgreSQL 17.7 native transactions and CLI audits reproduce four
reference refusals (`0A000`), two generation conflicts (`42601`), one missing
column (`42703`) and four accepted controls. The accepted controls include a
string literal, a schema-qualified scalar function, a quoted numeric type
modifier and DROP DEFAULT. The audit preserves catalog, row and sequence state.
The rule reports the authored target and a reason code. The expression's type
assignment, callable identity, privileges and runtime evaluation require other
evidence.

- `/tmp/ds-pg-f09-set-default-cli-v2-683d84ac/evidence.json`
- [PostgreSQL 17.7 expression transformation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_expr.c)
- [PostgreSQL 17.7 type modifier handling](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_type.c)

The joint D40 T37 preview passed 535 cases with 15,589 assertions.

## Direct Boolean and int4 literal assignment

On a single SET DEFAULT action for a fresh qualified persistent ordinary non-typed table, the existing default-on blocker also rejects a direct Boolean parser constant assigned to a built-in int4 column, or a direct int4 parser constant assigned to a built-in bool column. Existing target, identity, generation and reference checks take precedence. The parser keeps the literal's kind, not the value; the catalog keeps a Boolean type flag and the existing integer width. The finding adds only `default_literal_type_mismatch` and retains the existing authored column identity. No public catalog OIDs or default text are added.

PostgreSQL 17.7 refused both cross-type assignments with `42804`. `cookDefault` uses `COERCION_ASSIGNMENT`; the built-in casts between bool and int4 are explicit-only. Same-type literals, unknown text accepted by bool input, and an explicit cast are native acceptance controls. This does not validate explicit conversions, text, NULL, other types, domain or function behavior, runtime evaluation, or multi-action ALTER. The existing coverage of those cases is unchanged.

- [PostgreSQL 17.7 `cookDefault`](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/heap.c)
- [PostgreSQL 17.7 built-in cast definitions](https://github.com/postgres/postgres/blob/REL_17_7/src/include/catalog/pg_cast.dat)
- [T37 native and audit controls](../../testdata/ddl-golden/T37.json)

## Positional parameters in SET DEFAULT

PostgreSQL 17.7 cooks an ALTER COLUMN default in a fresh parser state without
parameter bindings. A bare parameter or a direct COALESCE argument fails with
`42P02`; quoted text containing the same characters remains a string literal.
The existing default-on rule adds `default_parameter_inapplicable` only for
those two AST shapes in single-action SET DEFAULT on a fresh qualified ordinary
non-typed table. The pre-existing target, identity, generation and illegal
reference checks retain precedence. The parser retains only a private boolean;
no parameter number or expression is added to the finding. This does not
analyze parameter occurrences in other expressions or broaden relation scope.

- [PostgreSQL 17.7 default cooking](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/catalog/heap.c)
- [PostgreSQL 17.7 parameter transformation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/parser/parse_expr.c)
- [T37 parameter refusals, quoted text, generated and subquery controls](../../testdata/ddl-golden/T37.json)
