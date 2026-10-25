# Decision: Advanced index facts and visibility governance

Date: 2026-10-24
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T10 under #89; extends `2026-10-22-ddl-index-governance-unification.md`)
Related commits: pending on milestone/mysql-tidb-ddl-completion (contract + implementation + frozen oracle + evidence in this task set)
Related tests: `internal/infrastructure/parser/tidb/extractor_t10_test.go`, `internal/application/audit/batch_state_t10_test.go`, `scripts/test_t10_contract.py`, `make ddl-golden TASK=T10 ARTIFACT_DIR=...`
Related docs: `docs/work-plans/t10-advanced-indexes/`, `testdata/ddl-golden/T10.json`

## Context

Before T10, every `ALTER TABLE ... ALTER INDEX name VISIBLE|INVISIBLE`
parsed but produced only a statement-level unsupported entry
(`option.invisible`/`option.visible` aspect): no rule saw the action, no
post-state transition existed, and the provider never read index
visibility. Declared `VISIBLE`/`INVISIBLE` index options sat in
`Index.UnmodeledOptions` as unextracted evidence, and provider-loaded
indexes carried key-part knownness (`PrefixParts`/`DescParts`/expression
marks) but no visibility and no `SPATIAL` kind.

Four-anchor native probing pinned the product boundaries before the
contract froze:

- `ALTER INDEX ... INVISIBLE|VISIBLE` is supported on MySQL 8.0/8.4 and
  TiDB 8.5; MySQL 5.7 rejects it (ERROR 1064 grammar). The
  `INFORMATION_SCHEMA.STATISTICS.IS_VISIBLE` column exists on MySQL 8.x
  and TiDB, absent on 5.7.
- `ALTER INDEX PRIMARY ...` fails on MySQL with ERROR 3522 (primary key
  cannot be invisible) and on TiDB with ERROR 1176 (`PRIMARY` is not an
  index there — TiDB normalizes the identifier to lowercase `primary`).
- Descending key parts: MySQL 8.x stores `COLLATION='D'`; MySQL 5.7
  silently stores ascending; TiDB accepts the syntax and also stores
  ascending — a product boundary, not a parse failure.
- TiDB expression indexes are gated by the `allow-expression-index`
  config and a function allowlist (LOWER accepted, CONCAT → ERROR 8200);
  FULLTEXT/SPATIAL are ERROR 8200 on TiDB. MySQL multi-valued
  `CAST(... AS ... ARRAY)` indexes require 8.0.17+.

## Decision

1. **`spec.Index` gains an authored/effective visibility pair and a
   multi-valued mark.** `Visibility` is the effective fact (`"visible"`,
   `"invisible"`, or `""` when unestablished); `VisibilityExplicit`
   records that a VISIBLE|INVISIBLE attribute was authored. Provider and
   post-state members claim the effective fact only — observed state is
   never laundered into a declaration. `MultiValuedParts` counts
   `CAST(… AS … ARRAY)` members (they remain expression parts too), and
   `UnmodeledOptions` no longer lists `visibility`.
2. **The extractor emits the `alter_index` action** with the target name
   on `OldName` and the normalized visibility on the action index, fills
   `Visibility`/`VisibilityExplicit` across primary/table/standalone/
   ALTER-declared index sites, and counts multi-valued members in the
   shared part-kind counter.
3. **`AssessAdvancedIndexes` in `internal/domain/spec` is the one shared
   assessment** consumed by the rules, the ordered-state publisher, and
   the coverage model: it derives the declared feature set (visibility,
   expression_keys, descending_keys, multi_valued) and evaluates it
   against the frozen per-product/version responsibility table,
   emitting determined findings for proven incompatibilities and bounded
   version gaps (`missing_target_version` /
   `target_version_out_of_validated_range`) when the fact is absent or
   outside the validated series. A syntactically valid out-of-series
   version is a bounded gap, never a CLI input error and never a
   fabricated compatibility finding.
4. **The provider probes `STATISTICS.IS_VISIBLE` capability against the
   connected catalog** (a backticked `information_schema`.`columns`
   existence check). Capable catalogs (MySQL 8.x, TiDB) add the column
   to the index select and land effective visibility per named index;
   a zero-count probe (MySQL 5.7) leaves member visibility empty rather
   than guessing `"visible"`; a real probe failure propagates as a load
   error. `INDEX_TYPE` now also classifies `SPATIAL`/`FULLTEXT` into
   `Index.Kind`.
5. **`batch_state_index.go` publishes the `alter_index` transition**: a
   provable named member flips its `Visibility` in place on a vouched
   index set; `PRIMARY` invalidates unconditionally on every dialect
   (the transition is impossible, even against an unknown table);
   a missing index tombstones; an unvouched index set withholds. All
   other member facts carry verbatim.
6. **Three new default-enabled rules** close the governance
   hole: `ddl.alter.alter_index.exists.require` (shared alter-object
   existence frame — unvouched sets emit the member-collection gap,
   never fabricated absence), `ddl.index.invisible.forbid` (binds every
   authored invisibility event — `alter_index` transitions and
   `INVISIBLE` attributes on all four index-declaring sites — with
   `{table,index,action,visibility}` metadata; authored `VISIBLE` stays
   silent), and `ddl.index.advanced.compatibility.require` (per-feature
   findings with `{feature,reason_code,product,target_version,
   minimum_version}` metadata, feature-scoped gaps on missing version
   evidence).
7. **Coverage treats `alter_index` as audited** and adds the bounded
   `multi_valued` aspect; expression/prefix/descending key parts keep
   their existing bounded aspects (version applicability is audited,
   member/query semantics are not). The T03 corpus expectation that
   recorded `alter_index` and `create-index-invisible` as unsupported
   upgrades to audited — a documented expected upgrade, not a
   denominator change.

## Rationale

- Visibility is an observable catalog fact with real operational
  consequences (invisible indexes are ignored by the optimizer), so
  leaving it inside `UnmodeledOptions` made a governed attribute look
  like unmodeled evidence. Splitting authored vs effective mirrors the
  T07 column-visibility contract and keeps provider answers from
  masquerading as intent.
- `ALTER INDEX PRIMARY` is impossible on every supported dialect —
  publishing it as present-incomplete would fabricate a transition the
  server always rejects, so it invalidates unconditionally regardless of
  table knownness.
- The capability probe (not a version guess) decides the IS_VISIBLE
  column because the fact source is the connected catalog itself; on 5.7
  an empty member-visibility fact lets the compatibility rule emit its
  own bounded gap instead of the provider pre-answering `"visible"`.
- Feature-scoped findings/gaps follow the T07 precedent: a multi-valued
  member is still an expression part, so the version gate must name the
  precise feature (`multi_valued` ≥ 8.0.17, `expression_keys` ≥ 8.0.13)
  instead of collapsing both into one compatibility claim.

## Public Contract

- Three new rule IDs ship enabled with frozen IDs, params, and catalog
  entries — `ddl.alter.alter_index.exists.require` and
  `ddl.index.advanced.compatibility.require` default `blocker`,
  `ddl.index.invisible.forbid` defaults `warning` (invisibility is a
  legitimate staged-migration step; teams harden it per policy).
  `configs/deltascope.example.yaml` documents them.
- Findings carry stable metadata: `{table,index,action,visibility}` for
  invisibility/existence, `{feature,reason_code,product,target_version,
  minimum_version}` for compatibility. Evidence gaps keep the bounded
  `required_facts` vocabulary.
- `spec.Index` serializes `multi_valued_parts`, `visibility`, and
  `visibility_explicit` alongside the existing part-kind counts — an
  additive-only public shape change.
- Unsupported constructs still surface bounded entries: TiDB
  FULLTEXT/SPATIAL declarations and MySQL-grammar rejections ride the
  existing `create_index`/parser channels with exact feature IDs, never
  silently clean.
- `rule_summary.applicable` counts the deduplicated union of applied
  rule IDs across statements; `loaded` omits the summary entirely when
  the profile enables zero rules (no fabricated zero).
- Deferred: optimizer-effect claims (invisible-index usage, descending
  scan benefit) and TiDB expression-function allowlist modeling — the
  audit governs declaration and version applicability, not planner
  behavior; TiDB's silent descending→ascending storage is pinned as an
  `ignored` outcome rather than a rejection.

## Verification

- Golden path: `make ddl-golden TASK=T10 ARTIFACT_DIR=<dir>` — 78 cases
  (24 CLI + 46 metadata + 8 error-role CLI), all assertions green on the
  four locked anchors (MySQL 5.7.44/8.0.46/8.4.10, TiDB v8.5.0).
- Independent oracle: `scripts/t10_contract.py` freezes the manifest
  expectations; `scripts/test_t10_contract.py` reports 33/33
  constructed-control checks including direction flips, version-gap
  laundering, observed-as-declared, and skipped lifecycle legs.
- Unit/TDD: extractor and audit tests were red before implementation
  (missing `alter_index`, missing visibility facts, no transition, no
  rules) and green after; `make test` is all-pass.

## Amendment R2 (2026-10-25) — scope restoration

The R1 delivery under-counted issue #89: it covered visibility plus the
version gates but kept member-level facts and the member-legality table
unmodeled, and it pinned two primary-key identity over-generalizations.
R2 restores the full responsibility:

- **Member-level key parts.** `spec.Index.KeyParts` replaces
  count-only marks: ordinal, member kind (column/expression/unmodeled),
  bound column, explicit `PrefixLength *int` (zero preserved),
  `Descending`, structured `ExpressionFacts` identity (references,
  functions, top form, fingerprint, knownness), and `MultiValued`. Parser
  and provider both populate it; `SetKeyParts` derives the aggregate
  counters so the two never drift.
- **Implicit primary-key role.** Both products promote the first
  declared qualifying `UNIQUE NOT NULL` index to an implicit primary
  role and refuse its invisibility (natively pinned 3522 on MySQL
  8.0/8.4 and TiDB 8.5 across ALTER/CREATE/ADD entries). The shared
  `ResolvePrimaryRole`/`ResolveDeclaredPrimaryRole` answers holder or a
  bounded gap — prefix members still qualify, expression members
  disqualify on TiDB but stay unprovable on MySQL (the hidden
  generated column can qualify; its nullability and the mixed
  functional/plain selection order are facts we cannot prove),
  unmodelable member facts withhold the answer, and
  declaration order is only provable on ordered
  (derived create-table) snapshots: provider catalogs are name-sorted,
  so multi-candidate sets emit `target_table.indexes.ordinal` rather
  than guessing `Indexes[0]`.
- **TiDB PRIMARY is an index object, sometimes.** The R1 contract's
  "PRIMARY is not an index on TiDB" held only for the single-integer
  clustered `PKIsHandle` shape. `NONCLUSTERED` and non-integer clustered
  primary keys are real objects: `VISIBLE` is a no-op success and
  `INVISIBLE` refuses 3522 (natively pinned). Provider detects
  PKIsHandle via `TIDB_INDEXES.CLUSTERED` + the integer member shape
  (STATISTICS synthesizes a PRIMARY row regardless, so presence cannot
  prove the object). Unvouched PK identity emits a
  `target_table.primary_key` gap, not a finding.
- **Member legality, not just presence.** New non-version-gated
  features `prefix_length`/`fulltext`/`spatial`/`global` drive
  natively-pinned checks: `prefix_required` 1170, `json_column_index`
  3152, `fulltext_column_type` 1283, `spatial_column_type` 1687,
  `spatial_nullable` 1252, `expression_column_missing` 1054, and TiDB
  `global_not_unique`/`global_not_partitioned` 8200. Unknown column,
  type, or partition facts degrade to bounded gaps. Determined negatives
  mark `InvalidTransition` and tombstone rather than publishing a
  rejected post-state.
- **Provider additions.** `STATISTICS.EXPRESSION` capability probe,
  `PARTITIONS`-backed `Partitioned *bool` (nil = unanswerable), and the
  TIDB_INDEXES handle detection above.
- Denominator 81→123 frozen cases (append-only) under amendment R2 of
  `CONTRACT.md`; frozen ID SHA-256
  `5efb9502716c4b2bbf66d335674a3f820cd58d8c5312aab6a32292a3b003197d`.

Deferred unchanged: expression equivalence/evaluation, per-function
TiDB expression legality, multi-valued query semantics, optimizer-effect
claims.

## Amendment R3 (2026-10-24) — role knownness, legality matrices, coverage graduation

The R1-R2 review confirmed four residual gaps against #89; R3 closes
them and graduates the member forms it can now audit.

- **Four-state primary role.** `ResolvePrimaryRole`/
  `ResolveDeclaredPrimaryRole` collapse nothing: explicit primary,
  proven implicit holder (a member-identity pointer, so two unnamed
  `UNIQUE` members stay distinct), proven no-candidate, or bounded
  unresolved. The R1 `Resolved`/`HasHolder` pair misfiled "explicit
  primary exists" as "no holder" (a later UNIQUE INVISIBLE was wrongly
  refused) and name-string identity conflated unnamed members (the
  second unnamed UNIQUE was wrongly refused).
- **Unresolved transitions withhold publication.** The ordered-state
  gate tombstones on `UnresolvedTransition` with features exactly like
  `InvalidTransition` — an unprovable role/member answer or a missing
  version on a gated form no longer publishes a success post-state.
- **TiDB handle is a tri-state.** `PrimaryKeyIsHandle` +
  `PrimaryKeyHandleUnknown`: provider no-row/NULL CLUSTERED evidence and
  unmatchable member shapes stay unknown; declared shapes resolve
  explicit `CLUSTERED|NONCLUSTERED` first, then the session-observed
  `tidb_enable_clustered_index` instance fact (new provider probe,
  TiDB-only); unprobed mode stays unknown. Anchored on
  pingcap/tidb v8.5.0 `pkg/ddl/create_table.go`.
- **Literal-`NULL` members classify three ways.** TiDB catalog rows
  report expression members as the literal string `'NULL'` in
  `COLUMN_NAME`; MySQL permits a backticked column named `NULL`.
  SQL-NULL or literal-with-expression-text → expression; literal
  matching a loaded column → plain column; neither → unmodeled.
- **Kind-aware member legality.** Index kind routes before prefix logic:
  FULLTEXT text members bind without `prefix_required` (non-string →
  1283 `fulltext_column_type`), SPATIAL requires geometry + NOT NULL
  (1687/1252), the prefix matrix is zero(1391)/non-string(1089)/
  over-length(1089)/required(1170) with nonzero FULLTEXT prefixes
  ignored natively, and multi-valued casts split by dialect — MySQL
  {signed,unsigned,char(n),binary(n),date,time,datetime,decimal} vs TiDB
  allowing `double` where MySQL allows `decimal`; {json,year,float} and
  length-less char/binary refuse on both, unlisted families stay gaps.
  Unknown member facts attribute gaps to the feature that needed them.
- **Coverage graduation.** `expr`/`prefix`/`desc`/`multi_valued` member
  forms and TiDB `GLOBAL` no longer mint unsupported aspects — legal
  positives audit complete and publish member facts (ordinals, prefix
  lengths, directions, expression dependencies) into the next
  statement's pre-state. The remaining boundary is honest:
  `unmodeled_parts`, predicates, MySQL-side `GLOBAL`/`primary_key_type`,
  unmodeled options, TiDB fulltext/spatial, vector/columnar. Graduated
  forms still do not answer plain-key pair comparisons — a vouched
  pairing with a non-plain member keeps the
  `target_table.indexes.key_parts` gap (T09 boundary preserved).
- **Oracle upgrade, not shrink.** 30 of the 126 frozen expects moved off
  the incorrect R1 unsupported exit to their natively-verified outcomes
  (supported-version positives pass; old-version refusals stay
  determined with complete coverage; TiDB/MySQL<8 descending emit
  `descending_ignored`; TiDB GLOBAL splits pass/refuse on the
  unique+partitioned gate). Case IDs are append-only and the frozen
  identity SHA-256 is unchanged
  (`3d526e8ef8199b18fbbf39fa1af024192948fee3a0ebefeea6d086c330329311` —
  126 cases; expect bodies sit outside the identity hash).

Deferred still unchanged: expression equivalence/evaluation,
per-function TiDB expression legality, multi-valued query semantics,
optimizer-effect claims, and every boundary that stays bounded above.

## T10-R3 — consumer interfaces, member fidelity, evidence gaps

Independent review of R2 confirmed three consumer-side defect classes;
R3 corrects them without expanding the audited surface.

- **Create-table comparisons are member-bound.** A pair comparison
  between two different declarations inside one `CREATE TABLE` now
  applies the same evidence discipline as `create_index`: when the
  column shape pairs under the rule's existing pairing predicate but a
  member is non-plain, the enabled rule emits
  `unknown_table_state` + `target_table.indexes.key_parts` instead of a
  silent complete/pass. Plain-comparable pairs, non-pairing members,
  lone non-plain indexes, and disabled rules keep exact outcomes.
- **Primary-key identity is a three-way fact everywhere.**
  `PrimaryKeyHandleUnknown` joins `PrimaryKeyUnknown`/
  `ConstraintsUnknown` in the exists rule's knowledge premise — an
  unvouched TiDB handle answer yields the `target_table.primary_key`
  gap under exists-only, compat-only, or both. Derived create-table
  snapshots compute stored names via `spec.StoredIndexNames` (first
  column member's name, `_N` ordered collision suffix — MySQL 8.0.46
  `invisible_indexes.result` verified); unprovable name spaces mark
  `IndexNamesUnknown` so name-keyed consumers gap instead of reading
  an unmatched name as confirmed absence. TiDB `int_only` additionally
  defers to the observed `alter-primary-key` config fact
  (`InstanceFacts.TiDBAlterPrimaryKey{,Known}`, loaded via the all-rows
  `SHOW CONFIG` pattern); unobserved keeps the handle unknown.
- **Provider expression members keep direction.**
  `classifyExpressionMember` reads `STATISTICS.COLLATION` like the
  column branch: `D`→`Descending`, `A`→ascending, NULL/unrecognized→
  unmodeled member with expression identity preserved. No expression
  evaluation introduced.
- **Evidence archive corrected.** The R2 archive's consolidated pin
  log is byte-identical to the R1 file and lacks the FT(5)/FT(0), MV
  DECIMAL/DOUBLE, clustered default-mode, and explicit-PK-then-UNIQUE
  probes it was described as containing; original scripts were not
  preserved. The R2 closeout is annotated in place and the missing
  matrices receive bounded re-verification under the T10-R3 archive
  with real execution metadata.

## T10-R4 — fact propagation and post-state truthfulness

Independent acceptance of R3 confirmed four remaining defect classes;
R4 corrects them without expanding the audited surface.

- **Name-keyed mutations stay truthful under `IndexNamesUnknown`.**
  DROP/RENAME/visibility transitions now answer per target: a matched
  authored name updates the member set while an unmatched target
  unvouches the collection (`IndexesUnknown`) rather than publishing a
  stale member set that can seed fabricated duplicate findings. A
  names-only unknown without mutation keeps provable findings; the
  exists rule's member gap is per-target so all-PRIMARY lists resolve
  from primary-key facts without an unrelated
  `target_table.indexes` gap.
- **Stored names are dialect-bound.** `spec.StoredIndexNames(dialect,
  …)` anchors on MySQL 8.0.46 `make_unique_key_name` and TiDB v8.5.0
  `setEmptyConstraintName`: MySQL claims sequentially with `primary`
  reserved and `functional_index` for any expression member; TiDB
  pre-occupies authored names (unnamed collisions suffix, never fail)
  with `expression_index`. `primary` reserves on both anchors
  (`primary_2` for unnamed members; authored `primary` is natively
  refused 1280). Unprovable shapes keep the whole name space
  unvouched — no partially-guessed names.
- **Clustered storage and PRIMARY addressability are separate facts.**
  `PrimaryKeyClustered`/`PrimaryKeyClusteredUnknown` answer the TiDB
  DROP PRIMARY KEY refusal class for every member shape (8200 covers
  PKIsHandle and common-handle primaries alike — natively verified on
  VARCHAR and composite clustered primaries, which still resolve
  `ALTER INDEX \`PRIMARY\`` as addressable objects), while
  `PrimaryKeyIsHandle`/`PrimaryKeyHandleUnknown` keep answering the
  single-integer row-id promotion. CREATE derivation and the provider's
  `TIDB_INDEXES.CLUSTERED` read feed both facts; a proven clustered
  target keeps the PK (refused drop), a provably nonclustered one
  publishes, an unproven answer invalidates.
- **Direction-withheld members keep expression identity.** A NULL or
  unrecognized `COLLATION` keeps the member `IndexKeyPartExpression`
  with `DirectionUnknown` and its expression/multi-valued facts rather
  than demoting to unmodeled; `DirectionUnknownParts` joins the
  non-plain exclusions in `indexPlainComparable`, and a `KeyParts`
  count beyond the `Columns` projection marks the member partially
  modeled so left-prefix comparisons owe the
  `target_table.indexes.key_parts` gap.
- **R3 evidence labels corrected.** The R3 archive now distinguishes
  the work-CLI SHA-256 from the golden-runner binary identity, states
  the focused-test log as 300 PASS records (not 600 checks), records
  that no historical red logs were preserved, and labels the M7
  clustered-DROP probes as a separately-executed append rather than
  part of `r3-bounded-probes.sh`.

### R4 review pass — member-first `primary` binding

The independent R4 review confirmed a second defect class around the
reserved `primary` name; natively anchored (M10 matrix):

- `primary` resolves **member-first, then the PK identity**. TiDB v8.5.0
  lets a member literally own the name (`RENAME INDEX s TO \`primary\``
  is legal on PK-less and PKIsHandle tables; DROP/ALTER/RENAME then hit
  the member), while a real PRIMARY index object occupies the name
  exclusively — a member `primary` can never coexist with it (1061).
  MySQL 8.0.46 reserves `primary` outright: `DROP INDEX \`PRIMARY\`` is
  DROP PRIMARY KEY, `RENAME` touching `primary` refuses 1280, an
  authored `primary` member name refuses 1280, and `ALTER INDEX
  \`PRIMARY\`` refuses 3522 — refused transitions keep their shape or
  tombstone through the invalid-transition gate.
- Post-state mutation paths mirror the binding order: DROP routes
  `primary` to the drop-primary-key outcome once a member `primary` is
  ruled out (TiDB clustered 8200 keeps; nonclustered drops), RENAME on
  a `primary` old name updates the index-object name while the
  constraint identity stays PRIMARY, a `primary` new name publishes
  only while no real object occupies it (unproven occupancy unvouches
  the name space), and duplicated stored names unvouch the member set
  rather than mutating every equal-fold match.
- `ddl.alter.{alter,drop,rename}_index.exists.require` now share one
  `indexExistenceRule` shell: all-`primary` targets on an unprovable
  table owe `target_table.primary_key` in place of the member fact
  (TiDB keeps the member fact alongside — a member `primary` may own
  the name), and `assessIndexFact` resolves `primary` member-first so
  a member visibility flip can never be misread as the 3522
  primary-object refusal.

### R4 acceptance corrections — per-member name provability

The frozen-diff Standards+Spec review found the collection-level
`IndexNamesUnknown` flag too coarse in both directions: it let stale
projected names bind members (fabricated post-state) and froze provable
authored-name targets. The corrections:

- `Index.NameUnknown` marks name provability per member — set on every
  member when `StoredIndexNames` cannot resolve the collection
  (cross-member suffix effects make honesty all-or-nothing there) and
  cleared by construction for provider catalog names and authored
  ADD/CREATE merges. Name-keyed transitions publish only on a proven
  match; a target no proven name can bind unvouches at the mutation's
  own granularity (`IndexesUnknown` where membership or member
  attributes could have changed, `IndexNamesUnknown` for rename, which
  can never change membership).
- One `spec.ResolvePrimaryNameBinding` tri-state (member / PK /
  ambiguous) backs every `primary` consumer in all three layers —
  ordered-state publishers, the `*TargetExists` predicates, and
  `assessIndexFact` — so member-first precedence cannot drift.
- Ambiguous `primary` bindings unvouch what they cannot prove: drop
  additionally unvouches the PK facts when a droppable real object
  cannot be ruled out (`markPrimaryUnknown`), rename unvouches the
  stored-name space including `PrimaryKey.Name` (which
  `primaryKeyConstraintName` now withholds under `IndexNamesUnknown`),
  and an unprovable destination freedom unvouches the matched member's
  name instead of publishing a maybe-refused rename. A proven name
  collision keeps the shape (1061 refusal), and `DROP INDEX \`primary\`
  IF EXISTS` keeps a provably absent PK without consulting the
  constraint-collection flag.

### R4 spec-review residual — hidden collisions and provider binding

A second frozen-diff pass pinned three more corrections:

- Member-adding transitions under an unproven member name space can
  collide on a *hidden* stored name — the add either landed or refused
  on the 1061 — so `applyAddIndex`/`applyCreateIndex` unvouch the member
  set rather than appending a member the statement may never have
  created. The merge still publishes when every member name is
  individually proven (no hidden name can hold the authored name).
- TiDB `ADD PRIMARY KEY` is shape-dependent under a `primary` name
  space a member provably or possibly owns (handle PK coexists, real
  PRIMARY object collides) — only a proven `PrimaryNameBindsPK` binding
  publishes; anything else invalidates.
- A `primary`-fold TiDB STATISTICS row is not self-evidently the
  constraint object, so `resolveTiDBPrimaryName` reads
  `information_schema.table_constraints` for a `PRIMARY KEY` row — the
  initial `tables.tidb_pk_type` probe conflates PK-less tables with
  real nonclustered primaries (both NONCLUSTERED, verified on the TiDB
  v8.5.0 anchor). Zero `primary`-fold rows prove no PK object (a real
  PK always emits one — handle tables included), a proven constraint
  plus exactly one fold promotes it, a proven-absent constraint keeps
  folds secondary, and multi-fold or unproven-plus-candidate states
  withhold all three primary facts. Rule-side name-keyed predicates
  bind only proven member names (`FindProvenIndex`), so an unproven
  projected name can never assert existence.

### R4 spec-review residual round 2 — proven-name fidelity

A second spec-axis pass on the corrected diff pinned three more
fidelity defects, all closed in place:

- TiDB `RENAME INDEX s TO \`primary\`` with unresolved PRIMARY
  occupancy only set the collection flag, leaving the matched member's
  old name falsely proven — `markIndexMemberNameUnknown` now unvouches
  that member's stored name so it can never re-bind as `s`.
- A proven 1061 collision in `applyAddIndex` / standalone
  `applyCreateIndex` (authored name already held by a proven member)
  invalidated/tombstoned the table instead of preserving the unchanged
  shape — the add provably refused before touching the table, or
  `IF NOT EXISTS` no-ops, so the entry keeps every fact.
- The member-existence gap projection stayed collection-granular while
  Evaluate went per-target — `indexMemberExistenceGaps` now owes
  `target_table.indexes` only for targets with no proven match under an
  unproven sibling name (or an unvouched member set), and a `primary`
  target on the resolving variant owes it exactly when the binding is
  ambiguous.
- A third spec-axis pass closed the residual edges: standalone
  `applyCreateIndex` now checks the authored-`primary` 1280 refusal
  before the unknown-entry publish (an unknown table stays unknown —
  the statement provably never touched it, mirroring `applyAddIndex`);
  the `table_constraints`/`tidb_indexes` probes withhold their fact
  set on any non-context error (`count(*)` can never be empty, so an
  unreachable `ErrNoRows` arm was honest but dead); the implicit-
  primary visibility assessment binds the target member by
  `FindProvenIndex` and the holder by `HolderIndex` identity instead of
  name-fold; and `ResolvePrimaryNameBinding` ignores the bare
  collection flag once every member name is individually proven.

### R5 amendment — actual stored names, error contract, suffix boundary

Independent review of the R4 candidate pinned three remaining fact
boundaries; all close in place without re-expanding T10:

- **Actual PK-object name continuity (R5-A).** `RENAME INDEX \`PRIMARY\`
  TO pk2` leaves the TiDB object stored as `pk2` (M10 log). All
  name-keyed consumers — ordered-state successors, provider reload,
  `snapshotHasIndex`, the `primary`-target existence/visibility paths —
  now resolve through `spec.ResolveIndexNameBinding`: the PK object
  claims only its actual stored name, a proven member name binds
  member-first (a member may legally own `primary` beside a handle PK),
  a handle PK claims `primary` as a logical identity with no operable
  object, and an unvouched name space stays ambiguous rather than
  guessing. `DROP PRIMARY KEY` is member-first per the native
  `dropIndex(…, "PRIMARY")` route; an attribution-withheld reload keeps
  a name-unknown PK placeholder with unique candidates demoted to
  `IndexKindUnknown`, and a drop on such a candidate unvouches PK
  presence. Constraint presence is proven by `table_constraints` while
  object attribution disambiguates unique folds through
  `TIDB_INDEXES.CLUSTERED` then the `KEY_COLUMN_USAGE` column list.
- **Provider error contract restored (R5-B).** The R4 note above
  recorded probe failures as withholding; that misstated issue #79's
  contract. Actual query/scan/connection failures propagate as load
  errors — even with all rules disabled — while clean absence keeps the
  unknown-fact posture; `count(*)`=0 is an answer, not a failure.
- **MySQL 61-byte suffix boundary (R5-C).** `make_unique_key_name`
  truncates a colliding base to 61 bytes before `_n` suffixing, so
  `StoredIndexNames` derives `base[:61]+"_2"` on MySQL; TiDB does not
  truncate — a 62-byte base suffixes to a 64-byte stored name while
  63/64-byte bases cross the limit natively (1059) and stay unprovable.

## References

- Issue: Fanduzi/DeltaScope#79 (milestone), #89 (T10 scope)
- Contract: `docs/work-plans/t10-advanced-indexes/CONTRACT.md`,
  `roles.json`, `EXPECTED-UPGRADES.md`
- Oracle/manifest: `scripts/t10_contract.py`, `testdata/ddl-golden/T10.json`
- Code: `internal/domain/spec/advanced_indexes.go`,
  `internal/domain/rule/ddl/index_visibility_rules.go`,
  `internal/application/audit/batch_state_index.go`,
  `internal/infrastructure/metadata/mysql/provider.go`,
  `internal/infrastructure/parser/tidb/extractor.go`
