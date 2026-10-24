# T10 advanced indexes and visibility — implementation contract

Status: FROZEN for T10 implementation. The frozen role matrix is in
`roles.json`; `testdata/ddl-golden/T10.json` is its materialized manifest.
Internal freeze is authorized by the task order; it is not external
acceptance. Same-contract implementation/test/collector bugs may be repaired
without reducing the frozen responsibilities or denominator.
Owner: #89 under #79. Task base: T09-R3 accepted head
`fbd411308e28332b5e8408b8074dd1c1fa501576` on `workfree`
(T09-R3 code `1a3dd01f2e13fab8780b35f93ff5d0b055e44dea`, contract
`8e68b44c4eddb4cb654486a42c144dccf81dc270`).

## 1. Scope and semantic seam

Promote `ALTER TABLE … ALTER INDEX name VISIBLE|INVISIBLE` from an
unaudited action to an audited one — normalized target facts, index
existence, an invisibility risk policy, an ordered post-state, and
product/version applicability — and promote the declared index visibility
attributes (`KEY|INDEX … VISIBLE|INVISIBLE`) from bounded unmodeled names to
modeled `spec.Index` facts. Give the remaining official advanced index
forms (prefix lengths, expression key parts, descending key parts,
multi-valued members, FULLTEXT, SPATIAL) a shared product/version
responsibility answer instead of bare parse marks, and repair the provider
catalog for index visibility and SPATIAL kind fidelity.

Out of scope: optimizer-benefit estimation, EXPLAIN/cost, expression
equivalence or evaluation, index recommendation, descending/prefix
ordering semantics beyond the bounded part counts the model already owns,
per-function expression legality inside key parts, a generic audit
framework, and anything belonging to later sibling tickets. Workload
performance and lock-duration claims are never made — catalog and syntax
facts only.

### 1.1 The first input

Pre-state: real isolated table `golden.t(a INT, b INT, KEY idx_c(b))`,
index set known-vouched, `idx_c` catalog-visible, target version proven by
the anchor.

```sql
ALTER TABLE t ALTER INDEX idx_c INVISIBLE;
```

Policy `t10-visibility`: `ddl.index.invisible.forbid` (warning),
`ddl.alter.alter_index.exists.require` (blocker),
`ddl.index.advanced.compatibility.require` (blocker, all features).
Oracle: exactly one warning under `ddl.index.invisible.forbid` bound to
`idx_c`/`t` with `visibility=invisible` metadata; zero evidence gaps;
`coverage=complete` per statement and overall; `verdict=review`.
Counter-cases: `… VISIBLE` emits no invisibility finding and stays
`coverage=complete`; a missing index name emits the exists blocker
alongside the independent invisibility warning (declared invisibility is
a statement-level fact, existence is a metadata fact — the two findings
are independent); an unknown index set emits the
`target_table.indexes` gap instead of a finding; the same action under
`--target-version 5.7.44` (or the mysql57 anchor) emits the
`version_unsupported` compatibility blocker, never a silent pass.

## 2. Reuse map

### 2.1 Existing and reused as-is

- `spec.Alter.Action`/`Name`/`Index`/`AlterIndex{OldName,Definition}` —
  `extractAlterName` already binds `specification.IndexName` to
  `alter.Name` for this action.
- `spec.Column.Visibility`/`VisibilityExplicit` — the same fact pair and
  the same semantics: `""` means unresolved; `Explicit` records the
  authored keyword only (never inferred from catalog normalization).
- `alterIndexTargetName`, `memberExistenceGaps`, `orderedGapDialect`,
  `targetTableSnapshot`, `matchingAlterObjectActions` — the existence-rule
  machinery and the ordered-state gap vocabulary.
- `spec.AssessAdvancedColumns` + `advancedCompatibilityRule` — the
  product/version responsibility pattern (features, issues, gaps,
  `missing_target_version`/`target_version_out_of_validated_range`).
- T09 non-plain member marks (`PrefixParts`, `DescParts`,
  `ExpressionCount`, `HasExpressionKeys`, `UnmodeledKeyParts`) — member
  comparability semantics are unchanged by visibility.
- `applyColumnVisibility` — the visibility-setter post-state template:
  publish only the effective visibility, reset the explicit bit state
  does not claim, shared conservative tombstone on unprovable premises.
- `ddl.table.exists.alter.require` — already covers `alter_index` actions
  for target-table existence.
- `memberExistenceGaps`/`memberCollectionFact` — the
  `target_table.existence`/`target_table.indexes` gap shape.

### 2.2 Upgraded (formerly bounded/unmodeled)

- `ast.AlterTableIndexInvisible` spec now produces
  `spec.AlterIndex{OldName: <name>, Definition: &Index{Visibility: <target>,
  VisibilityExplicit: true}}` in `extractAlterIndex`.
- `IndexOption` `VISIBLE|INVISIBLE` now produces
  `Index.Visibility="visible"|"invisible"` + `VisibilityExplicit=true` on
  every declared index path (create_table `KEY|INDEX|PRIMARY KEY`,
  standalone `CREATE INDEX`, `ADD INDEX|KEY|UNIQUE|PRIMARY KEY`); the
  bounded option names leave `UnmodeledOptions`.
- `information_schema.STATISTICS.IS_VISIBLE` is read when the column
  exists (8.0/8.4/TiDB anchors); provider members carry
  `Visibility="visible"|"invisible"` with `VisibilityExplicit=false`.
  MySQL 5.7 has no such column — provider leaves `Visibility=""` there
  rather than fabricating a fact.
- Provider `classifyIndex` maps `INDEX_TYPE='SPATIAL'` to
  `IndexKindSpatial` (was silently classified secondary/unique).
- `CAST(<expr> AS <type> ARRAY)` key parts are detected via
  `FuncCastExpr.Tp.IsArray()` and counted in the new bounded mark
  `Index.MultiValuedParts` (still also expression members — no member
  leaves `ExpressionCount`).

## 3. Frozen fact/model vocabulary

- `spec.Index.Visibility` — `"visible"`, `"invisible"`, or `""`
  (unresolved). Effective state; never a performance claim.
- `spec.Index.VisibilityExplicit` — authored keyword presence on declared
  paths only; provider and post-state members keep `false` (state claims
  the effective fact, not the authored form).
- `spec.Index.MultiValuedParts` — count of `CAST(… AS … ARRAY)` key parts.
- `alter_index` alter action: `alter.Name`/`Index.OldName` = target name,
  `Index.Definition.Visibility` = requested target, `VisibilityExplicit`
  = true.
- `spec.AssessAdvancedIndexes(statement)` — shared product/version answer:
  features `visibility`, `expression_keys`, `descending_keys`,
  `multi_valued`; issues with bounded `reason_code` and optional
  `minimum_version`; gaps for version facts; `VersionGated` marks the
  version-consulting subset; `InvalidTransition` marks determined-negative
  issues that must never publish post-state.

## 4. Native-observed responsibility matrix (T10 anchors)

| form | mysql 5.7.44 | mysql 8.0.46 | mysql 8.4.10 | tidb v8.5.0 |
|------|--------------|--------------|--------------|--------------|
| `ALTER INDEX … VISIBLE/INVISIBLE` | ERROR 1064 (syntax) | works; `IS_VISIBLE` flips | works | works; `IS_VISIBLE` flips |
| `ALTER INDEX PRIMARY` either dir | ERROR 1064 | ERROR 3522 (pk cannot be invisible) | 3522 | ERROR 1176 (PRIMARY not an index) |
| `ALTER INDEX <missing>` | ERROR 1064 | ERROR 1176 | ERROR 1176 | ERROR 1176 |
| `STATISTICS.IS_VISIBLE` column | absent (1054) | present | present | present |
| index `VISIBLE|INVISIBLE` attr | ERROR 1064 | works (`/*!80000 INVISIBLE */`) | works | works |
| expression key parts | ERROR 1064 | works (≥8.0.13) | works | works; unsafe-function members reject 8200 (whitelist nuance is expression semantics — unaudited) |
| `… DESC` member | accepted, stored ASC (silent drop; `COLLATION NULL`, SHOW CREATE drops DESC) | stored DESC (`COLLATION='D'`) | stored DESC | accepted, stored ASC (silent drop; `COLLATION='A'`, SHOW CREATE drops DESC) |
| prefix length `col(n)` | works | works (`SUB_PART=n`) | works | works |
| `FULLTEXT` | works | works | works | ERROR 8200 (vendor boundary) |
| `SPATIAL` (standalone create_index only — GEOMETRY columns and ALTER ADD SPATIAL do not parse) | works | works (`INDEX_TYPE='SPATIAL'`) | works | ERROR 8200 (vendor boundary) |
| multi-valued `CAST(x AS … ARRAY)` | ERROR 1064 | works (≥8.0.17) | works | accepted; cataloged as a functional index member (multi-valued query semantics unaudited) |

Version minima (MySQL dialect): visibility 8.0.0, expression keys 8.0.13,
multi-valued 8.0.17. TiDB is gated on the validated 8.5 series: in-range
supports visibility/expression/multi-valued, any other observed TiDB
version is `target_version_out_of_validated_range`. `descending` is not
min-version gated on TiDB — the anchor proves silent drop as a product
property, so the issue is unconditional there; on MySQL it is version
gated (<8.0 silent drop, ≥8.0 honored).

## 5. Rules

- `ddl.alter.alter_index.exists.require` (default blocker): the named
  index must exist on a vouched index set. `primary` resolves to the
  vouched primary key on MySQL dialect only (MySQL `ALTER INDEX PRIMARY`
  reports 3522 because the PK exists but cannot change visibility; TiDB
  reports 1176 because PRIMARY is not an index — a missing-index blocker
  is the honest TiDB answer). Missing → blocker `exists=false`;
  `IndexesUnknown`/unknown table → `target_table.indexes`(+`existence`)
  gap; never a finding under an unvouched set.
- `ddl.index.invisible.forbid` (default warning): fires whenever a
  declared or transitioned index resolves to `Visibility=="invisible"` —
  create_table index members (including a declared PRIMARY KEY),
  standalone `CREATE INDEX`, `ADD INDEX|KEY|UNIQUE|PRIMARY KEY`, and
  `alter_index` targets. Never fires on `visible`, `""`, or on
  provider-loaded invisible members that the statement does not declare.
- `ddl.index.advanced.compatibility.require` (default blocker; params
  `required` + `features` list): consumes `AssessAdvancedIndexes`; emits
  issue findings and version gaps for the enabled features only.
  `primary_index_visibility` rides the `visibility` feature and is
  emitted for MySQL `alter_index` targets and declared `PRIMARY KEY`
  invisibility only.

Reason codes added: `descending_ignored` (silent semantic drop, statement
succeeds — does not invalidate post-state), `primary_index_visibility`
(determined negative — invalidates). `version_unsupported` and
`primary_index_visibility` mark `InvalidTransition`.

## 6. Ordered-state semantics

`applyAlterIndexVisibility` (single-action path only; multi-action stays
the shared tombstone): unknown entry → present-incomplete; absent →
invalidate; unvouched index set → withhold; vouched missing → invalidate
(1176-class); MySQL `primary` target → invalidate (3522-class); found →
clone shape, set member `Visibility=<target>`,
`VisibilityExplicit=false`. `InvalidTransition` from the index assessment
forces invalidate on both `applyAlterIndexVisibility` and `applyAddIndex`
(a determined-negative statement never publishes a post-state).

## 7. Coverage vocabulary

- `alter_index` joins the audited action set for both dialects (TiDB
  executes the form natively).
- `<op>.index.option.visible` / `.invisible` aspects retire (modeled
  facts, not unmodeled options).
- `<op>.index.multi_valued` is emitted for `MultiValuedParts>0`
  (unaudited — multi-valued query semantics are not inspected).
- `expr`/`prefix`/`desc`/`predicate`/unmodeled-option aspects are
  unchanged; TiDB `fulltext`/`spatial` remain vendor boundaries; vector /
  columnar kinds are unchanged.

## 8. Golden path and denominator

`make ddl-golden TASK=T10 ARTIFACT_DIR=<dir>` runs `ddl_golden.py` against
`T10.json` — native DDL oracles per anchor, CLI offline cases, metadata
cases on all four anchors, error cases, frozen `required_case_ids` and
their SHA-256 in `scripts/t10_contract.py`, artifact validation through
`t10_artifact_failures`, and a `scripts/test_t10_contract.py` drift test
wired into `make ddl-golden-validator-test`.

## 9. Amendments

Amendments append below without rewriting history.

### 9.1 Amendment R1 — contract freeze (initial)

Initial freeze. Baseline behavior captured 2026-XX: `ALTER TABLE t ALTER
INDEX idx_c INVISIBLE` emitted `alter_table.alter_index` unaudited
evidence, `coverage=incomplete`, `verdict=review`, plus the
`ddl.table.exists.alter.require` existence gap. Provider classified
`INDEX_TYPE='SPATIAL'` members as secondary/unique.

### 9.2 Amendment R2 — member-level facts, implicit primary role, TiDB PRIMARY objects

Scope restoration round (issue #89's full responsibility, not visibility
alone). Corrections and additions; earlier frozen wording that R2
supersedes is noted inline.

**Model vocabulary added.** `spec.Index.KeyParts []IndexKeyPart` carries
per-member ordinal, kind (`column`/`expression`/`unmodeled`), bound
column, `PrefixLength *int` (explicit zero preserved), `Descending`,
structured `*ExpressionFacts` identity (references, functions, top form,
fingerprint, knownness), and `MultiValued`. `SetKeyParts` derives the
aggregate marks (`PrefixParts`/`DescParts`/`ExpressionCount`/
`MultiValuedParts`/`HasExpressionKeys`); provider and parser paths keep
them synchronized. `Index.PrimaryKeyType` models TiDB
`PRIMARY KEY … CLUSTERED|NONCLUSTERED`. `TableSnapshot` gains
`PrimaryKeyIsHandle` (TiDB PKIsHandle: a logical PK with no operable
PRIMARY index object) and `Partitioned *bool` (nil = unanswerable
catalog, never a guessed false).

**Correction — §4/§5 TiDB PRIMARY.** "TiDB reports 1176 because PRIMARY
is not an index" was an over-generalization pinned from a single-int
clustered fixture. Natively observed on TiDB v8.5.0: `PRIMARY KEY (a)
NONCLUSTERED` and a `VARCHAR` clustered PK *are* real index objects —
`ALTER INDEX PRIMARY VISIBLE` is a successful no-op, `INVISIBLE` returns
3522; only the single-integer clustered PK (`PKIsHandle`, detected via
`information_schema.TIDB_INDEXES.CLUSTERED` plus the integer column
shape, since STATISTICS synthesizes a PRIMARY row anyway) reports 1176
for either direction. `alterIndexTargetExists` now resolves `PRIMARY` by
index-object existence per dialect; an unvouched primary-key collection
surfaces `target_table.primary_key` as a gap instead of a finding.

**Correction — implicit primary role (fix A).** MySQL and TiDB both
promote the first declared qualifying `UNIQUE NOT NULL` index to the
implicit primary role and refuse its invisibility (3522 — natively
verified on 8.0/8.4/TiDB, including `CREATE UNIQUE INDEX … INVISIBLE`,
declared `UNIQUE KEY … INVISIBLE`, and `ALTER TABLE ADD UNIQUE`
variants). `ResolvePrimaryRole`/`ResolveDeclaredPrimaryRole` in
`internal/domain/spec/primary_key_role.go` is the shared determination
across alter_index, create_table, add_index, add_constraint, and
create_index. Candidate = unique, all members plain columns (prefix
members still qualify — natively pinned), expression members provably
disqualify, an unmodeled/unclassifiable member makes the decision
unprovable, and every bound column must be proven NOT NULL. Multiple
candidates resolve by declaration order only when the snapshot is
ordered (`IndexOrderCatalog` on derived create shapes); provider
snapshots sort by index name and cannot prove the pick → bounded gap
`target_table.indexes.ordinal`. Unvouched primary-key/index/member facts
degrade to `target_table.primary_key`, `target_table.indexes`, and
`target_table.indexes.key_parts` gaps — never `Indexes[0]` guesses.

**Member-legality features.** `AssessAdvancedIndexes` adds
`prefix_length`, `fulltext`, `spatial`, and `global` to the feature
vocabulary (non-version-gated — they never mint version gaps) with
natively-pinned reason codes: `prefix_required` (BLOB/TEXT member without
a length → 1170, all four anchors), `json_column_index` (direct JSON
member → 3152, prefix does not rescue), `fulltext_column_type`
(non-string family → 1283), `spatial_column_type` (non-geometry → 1687)
and `spatial_nullable` (nullable geometry → 1252, MySQL 5.7 InnoDB
included), `expression_column_missing` (unresolvable member reference →
1054), `global_not_unique`/`global_not_partitioned` (TiDB 8200 — GLOBAL
requires unique kind on a partitioned table; non-unique and
non-partitioned both refuse). Unknown column/type/partition facts emit
`target_table.columns`/`target_table.partitions` gaps rather than
guesses. All determined negatives mark `InvalidTransition` — the
publication gate tombstones instead of publishing a post-state the
product would reject. TiDB FULLTEXT/SPATIAL remain vendor boundaries
(8200); MySQL-side `primary_key_type` is a vendor boundary (TiDB-only
syntax).

**Denominator.** `required_case_ids` 81 → 123 (append-only; every
original id retained): per-anchor implicit-PK refuse/order-gap/create,
TiDB PRIMARY object split, prefix_required/prefix-ok, FULLTEXT-int,
SPATIAL-nullable/type/ok, JSON-direct, expression-missing,
multi-valued-supported, and TiDB GLOBAL gate cases. Frozen ID SHA-256
`5efb9502716c4b2bbf66d335674a3f820cd58d8c5312aab6a32292a3b003197d`.
