# Decision: PostgreSQL column tuning declarations and guarded drops

Date: 2026-11-18
Status: Accepted
Related milestone: F09 / `milestone/pg-alter-semantics` (D6)
Related commits: D5 shipped `26dde9ad`; D5 history closeout/research baseline `4eac7c11`; D6 implementation not yet delivered

## Context

A 72-shape PostgreSQL 17.7 study with 144 offline/metadata-aware CLI calls at
`4eac7c11` found precise remaining gaps. Eight normalized-spec counterexamples
collapse distinct behavior: STATISTICS valid/invalid/clamped values, column
option values/names, compression methods, table reloption values and DROP
COLUMN guards/CASCADE. No audit or transactional native driver rollback
changed the baseline catalog, rows, file identity or sentinel sequence.

`DROP COLUMN IF EXISTS ghost` is a native no-op but the metadata-aware audit
emits `ddl.alter.drop_column.exists.require` plus deletion advice and rejects.
STATISTICS -2 and n_distinct=-1.1 pass despite native 22023; STATISTICS 10001
succeeds with a warning and records 10000. RESET of unknown option names can
succeed as a no-op, unlike SET. Broader storage/compression/RID/schema/owner/
dependency gaps were measured too; they are not covered by this D6 decision.

The operator explicitly approved the three new defaults, existing-ID guard
correction, conditional advice and stage boundaries below. Approval is not
whole-F09 completion or authorization to analyze permissions, expressions,
business data, extend the parser or simulate PostgreSQL execution.

## Decision

Preserve AST-owned STATISTICS DEFAULT/value separately, and ordered column
option keys, namespaces, numeric values and value kinds. Keep duplicate
members visible; never infer their values from SQL text or a count-only map.
Quoted, nonfinite, overflowed or otherwise unmodeled values retain uncertainty;
raw quoted payloads/expressions are not stored. DROP COLUMN preserves parsed
IF EXISTS and CASCADE with explicit knownness.

Reuse D5's observed-PG17, exact-name ordinary-table directory and conservative
freshness contract. Existing DROP existence rule and deletion advice share one
assessment so findings, gaps and no-op suppression cannot disagree. Do not
change MySQL/TiDB rules or remove independent user-enabled forbids.

## Rationale

One parser-owned fact seam fixes the actual loss rather than adding lexical
fallbacks or transport-specific checks. Keeping the DROP rule ID avoids
unnecessary public churn. The native backend clamps statistics above 10000;
turning this into a blocker would reject valid SQL. Column option SET and RESET
are different operations: an unknown RESET name does not establish an error.
Extension option registration is not observable from the current catalog, so
unknown keys remain gaps rather than an invented closed-world allowlist.

## Public Contract

Enabled by default, parameterless, observed PG17:

| Rule | Default | Bounded predicate |
|---|---|---|
| `ddl.pg.alter.column.statistics.target.valid` | blocker | SET STATISTICS below -1, or a proven missing target column. DEFAULT/-1 and 0..10000 are valid; >10000 is not a blocker |
| `ddl.pg.alter.column.statistics.clamp.warn` | warning | Applicable explicit value >10000 is natively lowered to 10000; normal/default values are silent |
| `ddl.pg.alter.column.options.valid` | blocker | Known builtin SET n_distinct/n_distinct_inherited numeric range [-1, DBL_MAX], repeated known keys, and proven absent SET/RESET column targets |

The existing `ddl.alter.drop_column.exists.require` retains its ID/default.
A uniquely proven missing PG column with parsed IF EXISTS produces no missing
blocker, and its proven no-op deletion advisory is silent. A missing unguarded
column still blocks; an existing column retains independent deletion risk.
Unknown guard/version/scope/ambiguous/stale/misbound facts produce attributable
unverified/review gaps, not guessed absence or no-op. Related preceding DDL and
earlier ALTER subcommands withhold old facts rather than predicting a post-state.
CASCADE is retained, not dependency/execution proof.

New checks use real observed PG17 facts and D5's ordinary persistent,
non-inheritance/non-partition stage. Intrinsic parameter refusals do not
fabricate a catalog; applicable target-dependent conclusions require complete
fresh binding. Unknown/extension SET keys, quoted/nonfinite/unmodeled values
remain explicit gaps. RESET unknown keys are not rejected merely because SET
would fail on the anchor. No raw numeric/expression/provider payload enters
new finding metadata: only bounded action, identifiers, parameter and reason.

Existing fillfactor/access-method advice is corrected without another default
predicate: SET fillfactor does not itself immediately rewrite the table;
same-AM and partition cases cannot unconditionally claim a rewrite. Old
blanket forbids, D2/D5 rules and MySQL/TiDB behavior remain independent.

## Deferred / Out Of Scope

D6 does not certify data/expressions/permissions, physical compression/RID index
eligibility, table reloption/AM/tablespace/owner/schema semantics, inheritance/
partition behavior or general sequential state. These remain in the full F09
queue, not permanent deferrals or completed rows. Unknown field/value/extension
cases use the operator-accepted conservative stage gap; no default-off substitute
or parser expansion is authorized.

## Verification Evidence

Historical research (not D6 shipping proof):
`/tmp/ds-pg-f09-remaining-scope-4eac7c11/evidence.json` (72 native forms/144 CLI),
`/tmp/ds-pg-f09-remaining-normalized.log` (eight collapsed pairs),
`/tmp/ds-pg-d6-ast.log` (direct AST shapes, including quoted/overflow options).
Native results include 38 successes and 34 refusals. STATISTICS 10001 stores
10000 with native warning; duplicate n_distinct/fillfactor is 22023; unknown
RESET succeeds. Native type-change dependency/function refusals are not
misreported as a proved data-conversion failure.

Freeze these **18 additive T37 cases before implementation**; all old 118 case
IDs/profiles stay retained (final expected count 136):

- `T37.cli.column-tuning-statistics-offline`
- `T37.cli.column-tuning-default-offline`
- `T37.cli.column-tuning-options-offline`
- `T37.cli.column-tuning-reset-offline`
- `T37.cli.column-tuning-drop-guard-offline`
- `T37.cli.column-tuning-drop-cascade-offline`
- `T37.cli.column-tuning-disabled`
- `T37.cli.column-tuning-mysql`
- `T37.cli.column-tuning-tidb`
- `T37.cli.column-tuning-parser-retention`
- `T37.meta.pg-column-tuning-statistics`
- `T37.meta.pg-column-tuning-options`
- `T37.meta.pg-column-tuning-drop-guards`
- `T37.meta.pg-column-tuning-prior-ddl`
- `T37.meta.pg-column-tuning-prior-parser`
- `T37.meta.pg-column-tuning-unknown-values`
- `T37.meta.pg-column-tuning-scope`
- `T37.meta.pg-column-tuning-no-rewrite-advice`

Unit/application/SDK/HTTP/MCP/corpus checks, independent native catalog/refusal/
no-execution/owned-cleanup oracles and exact inventory bindings precede final
same-HEAD aggregate, independent read-only review, FF-only merge, normal push
and exact-SHA CI. None is replaced by the historical research above.

### Fact seam before rules

`PGColumnTuning` retains DEFAULT/literal and ordered key/namespace/value-kind
facts; only finite numeric AST values are carried, never quoted payloads.
DROP COLUMN publishes explicit parsed IF EXISTS/CASCADE known bits. Legacy
statistics/option count projections remain unchanged. Model/parser privacy,
default/zero/bounds/duplicates/opaque/overflow/RESET/guard tests pass, as do
build/default+PG tests/contracts/default+PG lint/format and unchanged T37
118/118 (1661 assertions) in focused candidate execution. These are fact-seam
checks, not D6 rule or final aggregate delivery.

## Consequences

Value ownership, SET versus RESET and guarded absence must remain distinct.
A future broader option/extension/table-kind domain needs new evidence and
approval. Preserve original denominators and honest mixed cells; correcting a
false blocker or adding a tuning check does not close F09 by association.

## Links

- [PG17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PG17.7 reloptions range and duplicate handling](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/access/common/reloptions.c)
- [D5 directory/freshness contract](2026-11-18-postgresql-column-generation-transitions.md)
- [Original program ledger](../plans/2026-11-17-rule-backlog-ledger.md)
