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
  `multi_valued`, plus the R2 member-legality features `prefix_length`,
  `fulltext`, `spatial`, `global`; issues with bounded `reason_code` and
  optional `minimum_version`; gaps for version facts; `VersionGated` marks
  the version-consulting subset; `InvalidTransition` marks
  determined-negative issues that must never publish post-state, and
  `UnresolvedTransition` marks feature-bearing answers that could not be
  proven either way (unprovable role/member facts, missing version on a
  version-gated form) — the ordered-state gate withholds publication for
  both.

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
  **Superseded by 9.3:** retired under R2-final — the cast-type legality
  matrix is member-audited; only multi-valued *query* semantics stay out
  of scope and need no coverage mark.
- `expr`/`prefix`/`desc`/`predicate`/unmodeled-option aspects are
  unchanged; TiDB `fulltext`/`spatial` remain vendor boundaries; vector /
  columnar kinds are unchanged.
- **Superseded by 9.3:** under R2-final, `expr`/`prefix`/`desc`/
  `multi_valued` member forms and TiDB `GLOBAL` graduated out of the
  aspect set — they are audited by the shared member-legality matrix
  instead of marking unsupported coverage. What remains bounded:
  `unmodeled_parts` (provider member rows that could not classify),
  index `WHERE` predicates, `primary_key_type` on MySQL (TiDB-only
  syntax), other unmodeled `IndexOption` members, TiDB
  `fulltext`/`spatial` vendor boundaries, and vector/columnar kinds.

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
members still qualify — natively pinned); expression members disqualify
on TiDB (hidden member columns never carry the role) while on MySQL a
functional unique can qualify through its hidden generated column —
whose nullability and the mixed functional/plain selection order we
cannot prove, so the index stays unprovable — and an
unmodeled/unclassifiable member also makes the decision unprovable.
Every bound column must be proven NOT NULL. Multiple
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

**Denominator.** `required_case_ids` 81 → 126 (append-only; every
original id retained): per-anchor implicit-PK refuse/order-gap/create,
TiDB PRIMARY object split, prefix_required/prefix-ok, FULLTEXT-int,
SPATIAL-nullable/type/ok, JSON-direct, expression-missing,
multi-valued-supported, and TiDB GLOBAL gate cases. Frozen ID SHA-256
`3d526e8ef8199b18fbbf39fa1af024192948fee3a0ebefeea6d086c330329311`
(123-case intermediate freeze was `5efb9502…`).

### 9.3 Amendment R2-final — four-state role, member-legality matrices, coverage graduation

Close-out of the four confirmed R1 review gaps. Corrections and additions;
earlier frozen wording that R2-final replaces is annotated in place.

**Four-state primary-role answer.** `ResolvePrimaryRole`/
`ResolveDeclaredPrimaryRole` now return `PrimaryRoleResolution` with four
distinct outcomes instead of a `Resolved`/`HasHolder` pair that collapsed
"explicit primary present" into "no holder" and compared holders by name:

- `ExplicitPrimary` — a real PRIMARY KEY exists; a later UNIQUE can never
  hold the implicit role, so `UNIQUE … INVISIBLE` is legal on every entry
  (natively rc:0 on MySQL 8.0/8.4 and TiDB 8.5; previously refused as a
  spurious first-candidate).
- `HasHolder` + `HolderIndex` — a member *identity* pointer into the
  index collection, not a name string. Two unnamed `UNIQUE` members no
  longer collapse: the first qualifying member holds the role and the
  second may be invisible (named and unnamed variants pinned).
- proven no-candidate — resolved, no qualifier.
- unresolved — the primary/member facts needed for the answer are
  unvouched (`PrimaryKeyUnknown`, unproven member sets, unprovable
  declaration order on name-sorted provider snapshots); the answer emits
  bounded gaps (`target_table.primary_key`, `target_table.indexes`,
  `target_table.indexes.ordinal`, `target_table.indexes.key_parts`) and
  marks `UnresolvedTransition`.

**Unresolved-transition publication gate.** The shared ordered-state gate
now withholds publication on `UnresolvedTransition` exactly like
`InvalidTransition` whenever the transition carries features — an
`ALTER TABLE t ADD UNIQUE KEY u_a(a) INVISIBLE` whose role answer is
unprovable tombstones `t`, so a follow-up `ALTER INDEX u_a` sees the
existence gap instead of an unproven post-state. Missing version on a
version-gated form follows the same precedent the column assessment
already set.

**TiDB handle tri-state.** `PrimaryKeyIsHandle` gains
`PrimaryKeyHandleUnknown`: provider `TIDB_INDEXES` no-row/NULL or an
unmatchable member shape no longer defaults the PRIMARY object into
existence. Declared create-table shapes resolve `PKIsHandle` through
explicit `CLUSTERED|NONCLUSTERED` first, then the session-observed
`tidb_enable_clustered_index` instance fact (new provider probe;
on/int_only → handle, off → real object); unprobed or ambiguous mode
leaves the answer unknown. Anchored on pingcap/tidb v8.5.0
`pkg/ddl/create_table.go` handle selection.

**Literal-`NULL` member classification.** TiDB `STATISTICS.COLUMN_NAME`
returns the literal string `'NULL'` for expression members; MySQL allows
a backticked column named `NULL`. The provider now classifies three ways:
SQL NULL or literal-with-expression-text → expression member; a literal
matching a loaded real column name → plain column member; neither
provable → `UnmodeledKeyParts`. A reserved-word column no longer
disappears from `Columns`/comparison.

**Kind-aware member legality.** `assessIndexMembers` routes by index kind
before ordinary prefix logic, so FULLTEXT text members no longer mint
`prefix_required`, and unknown member types attribute the gap to the
feature that needed the fact (fulltext) instead of a filtered prefix
gap. Natively pinned matrix:

- prefix: `prefix_required` 1170 (BLOB/TEXT no-length), `prefix_zero`
  1391 on *every* kind including FULLTEXT, `prefix_not_string` 1089
  (enum/date/int pinned non-capable; unlisted families stay gaps),
  `prefix_over_length` 1089; nonzero FULLTEXT prefixes are ignored by
  MySQL (rc:0) and SPATIAL prefixes refuse as non-string.
- FULLTEXT members: string/text families (1283 on others).
- SPATIAL members: geometry family + NOT NULL (1687/1252).
- multi-valued casts: MySQL allows
  {signed, unsigned, char(n), binary(n), date, time, datetime, decimal}
  and TiDB allows `double` where MySQL allows `decimal` — refused on
  both: {json, year, float} plus char/binary without a length (1235);
  unlisted families and missing cast facts emit the
  `multi_valued_cast_type` bounded gap.
- expression members keep structured facts and dependencies; an
  unresolvable bound reference refuses `expression_column_missing`
  (1054), unproven identity emits the feature gap.

**Coverage graduation.** `expr`/`prefix`/`desc`/`multi_valued` member
forms and TiDB `GLOBAL` left the unsupported aspect set — legal positives
on enabled checks complete their audit and publish member facts into the
next statement's pre-state (ordinals, prefix lengths, directions,
expression dependencies verified through `Metadata.TargetTable`), while
the graduated-member boundary keeps `unmodeled_parts`, predicates,
MySQL-side `GLOBAL`/`primary_key_type`, unmodeled options, TiDB
fulltext/spatial, and vector/columnar. Graduated does not mean
comparable: the T09 pair-comparison boundary is preserved — a vouched
pairing where either side is non-plain still emits
`target_table.indexes.key_parts` rather than answering under plain-key
semantics.

**Oracle upgrade.** 30 frozen expects inside the same 126 `required_case_ids`
moved off the R1-frozen unsupported exit where the audited semantics
differ (natively verified per anchor): expression/multi-valued/prefix
positives on supported versions pass with complete coverage; MySQL 5.7
expression/multi-valued stay determined version refusals *with complete
coverage*; MySQL <8 and TiDB descending emit `descending_ignored`
compat findings (TiDB unconditional, MySQL version-gated); missing
version on a gated form emits the version gap; TiDB GLOBAL partitioned-
unique passes while non-partitioned/non-unique refuse
`global_not_partitioned`/`global_not_unique`; expression-missing refuses
`expression_column_missing`. Unchanged: TiDB FULLTEXT/SPATIAL vendor
boundaries, MySQL-side GLOBAL/`primary_key_type` vendor boundaries,
unprovable-fact cases. The frozen ID SHA-256 is unchanged —
`3d526e8ef8199b18fbbf39fa1af024192948fee3a0ebefeea6d086c330329311`
(126 append-only IDs; expect bodies are not part of the identity hash).

### 9.4 Amendment R3 — consumer-interface, member-fidelity, and evidence-gap corrections

Independent review confirmed three classes of consumer-side defects;
R3 corrects them without expanding the audited surface.

**Create-table member-comparison gap.** A pair comparison between two
different declarations inside one `CREATE TABLE` is member-bound exactly
like the declared-vs-existing `create_index` comparison: when the column
shape pairs under duplicate/left-prefix/unique-overlap semantics but
either member is non-plain, the plain-key verdict is unverifiable and the
enabled rule emits `unknown_table_state` + `target_table.indexes.key_parts`
instead of a silent pass (`declaredComparisonGap`, never self-paired).
Non-pairing members, lone non-plain indexes, provable plain pairs, and
disabled rules keep their exact outcomes; the `create_index` /
`ALTER ADD INDEX` member controls are unchanged.

**Primary-key identity through all consumers.** `PrimaryKeyHandleUnknown`
now participates in the exists rule's knowledge premise exactly as
`PrimaryKeyUnknown`/`ConstraintsUnknown` do — an unvouched TiDB handle
answer suppresses the absence finding *and* reports the
`target_table.primary_key` identity gap under `exists` alone, `compat`
alone, or both. Derived create-table snapshots assign product stored
names via `spec.StoredIndexNames` (native anchor: MySQL 8.0.46
invisible_indexes.result L274-285 — an unnamed member stores under its
first column member's name with `_N` suffixing): a provable name space
lets successors resolve `ALTER INDEX`/`DROP INDEX` members, while an
unprovable name (expression first member, empty member list,
authored-name collision) marks `IndexNamesUnknown` — the member set
stays known but name-keyed consumers report the collection gap rather
than reading an unmatched name as confirmed absence. TiDB `int_only`
clustered mode additionally defers to the observed `alter-primary-key`
config fact (upstream: `!AlterPrimaryKey && isSingleIntPK`), carried as
`InstanceFacts.TiDBAlterPrimaryKey{,Known}` and loaded through the same
all-rows `SHOW CONFIG` verification as `max-index-length`; an unobserved
fact keeps the handle answer unknown. `on`/`off`/explicit
`CLUSTERED`/`NONCLUSTERED` semantics are unchanged.

**Provider expression direction.** `classifyExpressionMember` reads the
same `STATISTICS.COLLATION` fact the column branch does (native anchor:
MySQL 8.0.46 functional_index.result L186-193 — descending functional
indexes are real objects): `D` lands `Descending` on the expression key
part, `A` stays ascending, and a NULL/unrecognized collation leaves the
member unmodeled with its expression identity still attached — never a
guessed ascending. No expression evaluation or optimizer analysis is
introduced.

**Evidence-archive correction.** `docs/verification/t10/d4d6d5a8…` shipped
`native-probe/r2-consolidated-pins.log` byte-identical to the R1 file
(blob `c83362698c9b91664f5f7e6cd2a90890e38f4f72`): it contains the
implicit-PK promotion matrix, TiDB PRIMARY-object truth table, and the
member-legality error table, but *not* the FT(5)/FT(0), multi-valued
DECIMAL/DOUBLE dialect, clustered default-mode, or explicit-PK-then-UNIQUE
probes the R2 closeout attributed to it, and no probe scripts were
preserved. The R2 closeout is corrected in place; the missing matrices
receive bounded re-verification recorded with real execution time and
code identity in the T10-R3 archive. The merged validator suite total is
938 checks — `test_ddl_golden.py` itself contributes 758 contract cases.
