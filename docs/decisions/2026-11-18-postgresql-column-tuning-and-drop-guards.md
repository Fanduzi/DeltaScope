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

### Shared rules and compatibility evidence (candidate, not delivery)

The shared assessment implements the approved defaults and old DROP ID;
conditional deletion/no-rewrite copy and config/catalog/module/bilingual docs
are synchronized. Unit and real parser/application/SDK/HTTP/MCP/corpus checks
cover validity, warning, silence, unknown/version/freshness/guard/opaque and
privacy branches. Build, default/PG tests, inventory/corpus/catalog contracts,
default/PG lint and formatting pass. The minimized guarded-absence regression
failed on both the old blocker and unconditional advisory before the common fix.

An 80-shape/160-CLI candidate replay at base `2299f216` plus the rule worktree
records the binary/diff hashes in
`/tmp/ds-pg-d6-rules-scope-2299f216/evidence.json`: 42 native successes/38
refusals, all original 72 SQLSTATE controls unchanged, catalog/data/file/sentinel
unchanged after audits and native rollback. Quoted numeric succeeds natively;
quoted NaN/Infinity and float overflow refuse 22023 but correctly remain
unverified rather than invented blockers. RESET duplicates succeed. Int32-max
statistics clamp; out-of-grammar int32 overflow retains parser-error/42601.

The first old-case replay was 115/118 because three offline DROP cases still
claimed complete coverage despite the newly approved required proof. Retain
all 118 IDs, profiles, SQL, verdicts, finding counts/entries and checks; correct
only their aggregate/per-statement coverage to unverified and **add exact gap
count/attribution/required-fact/no-finding-leak assertions**. Mechanical JSON
comparison pins those as the sole three object changes. The corrected replay
passes 118/118, 1673 assertions (12 stronger assertions), at the same base plus
candidate worktree; neither replay is the final D6 aggregate. The 18 frozen
additions, exact inventory bindings and fixed-HEAD delivery still follow.

Implementation seams: [shared rules](../../internal/domain/rule/ddl/postgresql_column_tuning_rules.go),
[unit](../../internal/domain/rule/ddl/postgresql_column_tuning_rules_test.go),
[pipeline](../../internal/application/audit/column_tuning_postgresql_tag_test.go),
[corpus](../../testdata/sql-corpus/postgresql/ddl/findings/column_tuning_invalid.expected.yaml)
and [T37](../../testdata/ddl-golden/T37.json).

### Frozen canonical and inventory evidence

All 18 frozen additions are present: ten CLI and eight owned metadata cases,
**136/136 cases, 2168 assertions** in focused candidate execution (495 additive
assertions over the corrected old 118/1673). Mechanical comparison preserves
all 118 old case objects, required IDs and independent profiles. Fifty-one
native driver actions include 36 successes and 15 exact refusals (22023,
42703, 2BP01 and 42601), with independent attribute/file/data/guard receipts,
post-audit non-execution checks and ownership-limited absent/cleanup proofs.
The artifact at
`/tmp/ddl-golden-pg-column-tuning-canonical-v3/T37/artifact.json` is a
`65387b5f` plus candidate-worktree check, not the final D6 same-HEAD aggregate.

The first additive replay was 133/136. Its fixture assumptions were corrected,
not the product: all-off omits rule_summary; preceding DROP conservatively
revokes global target directories, even across different named tables. Fresh
missing-case collision, guarded CASCADE no-op, quoted present target and
known-empty no-op controls now occur before their request's first DROP;
subsequent targets explicitly assert every owed gap. No generic PostgreSQL
prospective-state engine or freshness shortcut was added to turn this green.
Native quoted/default/RESET/refusal controls remain independent. PG17's
`attstattarget` default is **NULL**, including SET STATISTICS -1; explicit
zero remains zero. A separate native probe and the PG17 catalog manual verify
this representation rather than inventing a -1 catalog fact.

Exact inventory binding touches seven original subactions across three mixed
F09 rows: DROP, guarded CASCADE and multi-action; STATISTICS and n_distinct/
RESET; and two existing layout-advice cells. The only cell status changes
make multi-action and n_distinct/RESET **mixed**, honestly retaining unapplied
state and opaque/extension/value gaps. Other existing audited cells receive
bounded evidence, not full execution/dependency/permission certification.
Layout evidence is copy-only, not a new eligibility validator. All original
subaction sets, 301 rows, row statuses and 19-family/63-row denominators stay
unchanged. The matrix is generated from YAML. Final fixed-HEAD all-task gates,
read-only review, merge/push and exact-SHA CI remain required before delivery.

### Read-only review correction: native floating underflow

The first fixed candidate `f97c73988098ce4d386dc7640c8e081f9c184417`
completed all 50 tasks, 3468/3468 cases and 61202 assertions, and validate
passed. It was **not merged or delivered**: independent read-only review found
that Go ParseFloat returns nil error for nonzero literals rounded to zero or
subnormal values, while PG17 parse_real/strtod rejects ERANGE (22023).
The exact parser/application regression was red; a temporary overlay verified
the proposed correction without changing source/HEAD during the active all run.
The owned network-none PG17.7 review container was removed independently of
all-task fixtures. Old green artifacts remain historical, never final proof
for the corrected HEAD.

Correct the fact seam, not the validity rule: Float values at/below the
minimum-normal rounding boundary remain unmodeled unless the AST numeric
coefficient proves genuine zero. Normal finite values above that boundary
remain numeric. A rounded nonzero value must never masquerade as authored
zero. Inspect only the numeric AST literal, not surrounding SQL; keep the
existing approved column.options evidence gap rather than inventing a new
blocker. Exact minimum-normal input can succeed natively yet still retain a
conservative gap because its rounded value alone cannot prove strtod behavior
for every authored coefficient. This is a representation boundary, not a
promise to validate every finite numeric spelling or a permanent deferral.

Parser/application/SDK/HTTP/MCP and corpus controls cover the regression.
The original 136 IDs/profiles stay unchanged; only the opaque-value canonical
fixture gains eight controls: positive/negative rounded-zero, subnormal and
minimum-subnormal refusals, minimum-normal boundary, real zero/zero exponent
and small normal success. Corrected focused execution is **136/136, 2186
assertions**, with **59 native steps (40 successes, 19 refusals)**. The candidate
artifact `/tmp/ddl-golden-pg-column-tuning-underflow/T37/artifact.json` records
base f97 plus the corrective worktree, not final shipping evidence. No old
case, profile or native refusal is dropped or weakened; new exact gap and
catalog/data/file/guard checks supplement them. The inventory's existing mixed
option cell and source-generated matrix include underflow explicitly without
changing any row or declared dimension. A new fixed-HEAD aggregate/review and
the complete merge/push/exact-SHA CI chain are still required.

## Consequences

Value ownership, SET versus RESET and guarded absence must remain distinct.
A future broader option/extension/table-kind domain needs new evidence and
approval. Preserve original denominators and honest mixed cells; correcting a
false blocker or adding a tuning check does not close F09 by association.

## Links

- [PG17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PG17 pg_attribute: NULL default statistics target](https://www.postgresql.org/docs/17/catalog-pg-attribute.html)
- [PG17.7 reloptions range and duplicate handling](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/access/common/reloptions.c)
- [PG17.7 parse_real rejects strtod ERANGE](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/utils/misc/guc.c)
- [D5 directory/freshness contract](2026-11-18-postgresql-column-generation-transitions.md)
- [Original program ledger](../plans/2026-11-17-rule-backlog-ledger.md)
