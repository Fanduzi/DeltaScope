# Multi-Target DDL Denylist Coverage

Date: 2026-09-22
Status: Accepted — implemented with issue #80

Task: [GitHub issue #80](https://github.com/Fanduzi/DeltaScope/issues/80) under the
[MySQL/TiDB DDL completion milestone](2026-09-21-mysql-tidb-ddl-completion-scope.md).

## Context

`ddl.table.denylist.forbid` historically evaluated only `spec.DDL.Table`, the
first extracted target. Extraction dropped every later target: multi-name
`DROP TABLE` kept only `Tables[0]`, and `RENAME TABLE` pairs lived only inside
`Alter[].Options` strings that the rule never read. A protected object in any
non-first position bypassed the denylist, making the verdict depend on target
order.

## Decisions

- `spec.DDL` gains `Targets []Table`, preserving every table-level identity a
  statement names in source order. `DDL.Table` stays the primary/first target
  for compatibility; `(*DDL).TableTargets()` returns `Targets` when populated
  and otherwise falls back to `Table`, mirroring `DML.MutationTargetTables()`.
- Extractors populate `Targets` for every `DROP TABLE`/`DROP VIEW` list member,
  every `RENAME TABLE` source and destination pair, every PostgreSQL `DROP
  TABLE`/`TRUNCATE` relation, and `ALTER TABLE ... RENAME TO` destinations after
  the altered subject. `Targets` carries effective object identities: an
  unqualified rename destination is recorded with its source's schema because
  that is where the object lands under MySQL/TiDB semantics. `Alter[].Options`
  keeps the as-written `new_table`/`new_schema` clause facts.
- The denylist iterates `TableTargets()` and resolves each target's schema as
  explicit qualifier first, then `Metadata.Schema`. Findings deduplicate by the
  resolved `(schema, table)` tuple — not a flattened `schema.table` string, which
  would merge distinct objects like `"a.b"."c"` and `"a"."b.c"` — so one
  protected object yields one finding even when it matches `tables`, `schemas`,
  and `qualified_tables` selectors or appears twice in the statement. Finding
  order follows source order.
- `auditmeta.statementTargets` feeds every drop/truncate/create target into
  session-schema inference but only the subject of `alter_table`: an unqualified
  rename destination inherits the source schema and a qualified one is already
  explicit, so destinations neither require an existing lookup nor contribute
  their own resolved schema.

## Boundaries

- An unqualified target with no request schema cannot match `qualified_tables`;
  that unknown-schema boundary is unchanged.
- `qualified_tables` entries are flat `schema.table` strings, so a selector like
  `a.b.c` inherently matches both `"a.b"."c"` and `"a"."b.c"`. Disambiguating
  that selector format is a policy-schema question deferred beyond this task;
  statement-side findings still carry the resolved `(schema, table)` pair.
- Rename destinations join the denylist only as named identities. No rule checks
  whether the destination object already exists.
- PostgreSQL `RENAME` uses the `alter_table` rename action path, not the MySQL
  `RENAME TABLE` statement form; no behavior change there beyond the shared rule.
- DML denylist semantics are unchanged: it already iterates `MutationTargetTables()`
  and still resolves schema only through `Metadata.Schema`.
- `Metadata.TargetTable` enrichment still resolves the primary `DDL.Table` only;
  per-target metadata snapshots for existence/shape rules are deferred to the
  later ordered-state and metadata work in this milestone. The denylist is
  offline and unaffected by that boundary.

## Verification

Issue #80 Golden Path (`/tmp/t01_golden.sh`, verbatim from the issue body):
`cases=16 failures=0` covering first/middle/last DROP positions, RENAME source
and destination pairs, and all-allowed controls for `mysql` and `tidb`. Focused
coverage added at the spec accessor, TiDB extractor, PostgreSQL extractor
(tagged), denylist rule, auditmeta inference, shared audit path, SDK, CLI, HTTP,
and MCP layers. `make sql-corpus-gates`, `make test`, and
`make pg-unit-test-gates` pass at the task HEAD.

## Consequences

`ALTER TABLE x RENAME TO sensitive` and multi-target DROP/RENAME forms now emit
the same blocker as their single-target equivalents; SDK, CLI, HTTP, and MCP
share the outcome because the semantics live in the domain/application seam.
Cross-schema DROP lists can now surface a legitimate "resolved multiple schemas"
inference error where previously only the first target informed inference.
