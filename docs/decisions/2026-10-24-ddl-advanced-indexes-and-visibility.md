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
  provably disqualify, unmodelable member facts withhold the answer, and
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
