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
  the altered subject. `Targets` carries as-written object identities: an
  unqualified rename destination stays unqualified because MySQL/TiDB resolve
  it to the current database, not the source table's schema. `Alter[].Options`
  keeps the as-written `new_table`/`new_schema` clause facts.
- The denylist iterates `TableTargets()` and resolves each target's schema as
  explicit qualifier first, then `Metadata.Schema`. Findings deduplicate by the
  resolved `(schema, table)` tuple — not a flattened `schema.table` string, which
  would merge distinct objects like `"a.b"."c"` and `"a"."b.c"` — so one
  protected object yields one finding even when it matches `tables`, `schemas`,
  and `qualified_tables` selectors or appears twice in the statement. Finding
  order follows source order.
- `auditmeta.statementTargets` feeds every drop/truncate/create target into
  session-schema inference but only the subject of `alter_table`: rename
  destinations are new names, not existing objects, so an unqualified
  destination resolves to the session schema being inferred and a qualified
  one points at the future location; neither contributes existence evidence.

## Correction (2026-09-22, same-day rework)

The original version of this record claimed an unqualified rename destination
"inherits the source schema". That is wrong. Under MySQL/TiDB semantics an
unqualified destination resolves to the **current database**, independent of
the source qualifier:

- MySQL 8.4 `PT_alter_table_rename::do_contextualize`
  (`sql/parse_tree_nodes.cc`): when `RENAME TO` carries no db, the new name's
  `new_db_name` is filled by `LEX::copy_db_to`, i.e. the session's current
  database — never the altered table's schema.
- MySQL `LEX::copy_db_to` (`sql/sql_lex.cc`) resolves to `thd->copy_db_to`,
  the default database of the statement context.
- TiDB 8.5 `handleTableName` (`pkg/planner/core/preprocess.go`): any table
  name with empty `Schema` — including rename destinations — is set to
  `GetSessionVars().CurrentDB`; an empty current database errors with
  `ErrNoDB`.
- MySQL bug [#11493](https://bugs.mysql.com/bug.php?id=11493): fixed in
  4.1.15/5.0.12 so `ALTER TABLE db2.t RENAME t` moves `t` to the default
  database, matching `RENAME TABLE db2.t TO t`.

The implementation was corrected so `Targets` records the as-written
qualifier and the denylist resolves it against `Metadata.Schema` (the request
schema). The earlier claim survives nowhere in code, tests, fixtures, probes,
or docs.

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
- `rename_table` statements never participated in session-schema inference
  (`auditmeta` only reads create/alter/drop/truncate targets); that pre-existing
  boundary is unchanged. In metadata-aware mode without an explicit schema, an
  unqualified `RENAME TABLE` destination therefore stays unknown, consistent
  with the offline boundary above.

## Verification

Issue #80 Golden Path (`/tmp/t01_golden.sh`, verbatim from the issue body):
`cases=16 failures=0` covering first/middle/last DROP positions, RENAME source
and destination pairs, and all-allowed controls for `mysql` and `tidb`. Focused
coverage added at the spec accessor, TiDB extractor, PostgreSQL extractor
(tagged), denylist rule, auditmeta inference, shared audit path, SDK, CLI, HTTP,
and MCP layers. The rework's SDK schema matrix
(`TestAuditRenameDestinationResolvesRequestSchema`) proves unqualified
destinations resolve to `Request.Schema`, explicit destinations win, the
source schema never leaks, and the unknown-schema boundary holds — for both
dialects and both rename syntaxes. `make sql-corpus-gates`, `make test`, and
`make pg-unit-test-gates` pass at the task HEAD.

## Consequences

`ALTER TABLE x RENAME TO sensitive` and multi-target DROP/RENAME forms now emit
the same blocker as their single-target equivalents; SDK, CLI, HTTP, and MCP
share the outcome because the semantics live in the domain/application seam.
An unqualified rename destination is evaluated against the request schema:
`qualified_tables` policies protecting `request_schema.name` now catch
rename-into-protected-name attempts that a source-schema copy would have
missed, and policies protecting `source_schema.name` no longer fire on the
destination. Cross-schema DROP lists can now surface a legitimate "resolved
multiple schemas" inference error where previously only the first target
informed inference.
