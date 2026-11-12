# T26-SRS-DROP-ORDERED-PUBLICATION-EXECUTION-01 — receipt (R5, CLOSED)

Implementation + verification round over the accepted contract
`docs/verification/t26-srs-drop-ordered-publication-contract-01/`
(R4, closed at `1346f230`). Status: **R5 ACCEPTED / CLOSED** — the
R1–R4 deliveries were each returned CHANGES REQUIRED; the R4 ruling
accepted the product behavior and all test-layer semantics
(R4-A…D CLOSED); the R5 ruling closed evidence provenance.
Final identities: product/test candidate C4
`cfa4295712291979db14a618456106ad4b79b6b4` (tree
`2d722436286c5f77e7a774a8f4c1a5a8b15c57af`); evidence closeout F5
`ff6069a51a6694efa509eef2773a2f686fa82cc2` (tree
`84be9cb7a4d7a8b154448dce5b4c2387935bfc4b`). This acceptance is NOT
a push/merge/tag/release/issue-close authorization; SRS CREATE /
ALTER INSTANCE / T27 remain separate follow-up work orders.
Expected vectors come from the frozen case-matrix; every measured
number below was captured from raw logs — expected and measured are
never conflated.

## Scope (unchanged product surface)

- `SpatialReferenceSystemState.OrderedAbsent` — internal `json:"-"`
  marker; zero value preserves behavior, never serialized.
- `batchState.srsOrdered map[uint32]*srsOrderedEntry` — the ordered
  provenance channel (`srid`/`absent`/`sourceArm`/`ownPrivilege`/
  `publishedIdx`/`barrier`), plus `applyStmtIndex` for producer
  provenance.
- `spec.OrderedAbsentConsumed` — the unique predicate
  `state != nil && state.OrderedAbsent && state.SRID == authoredSRID`;
  the A slot reads `ValidAbsence(...) || OrderedAbsentConsumed(...)`,
  first-match order B → S0 → S1 → A → S2 unchanged.
- `bindMySQLSRS` ordered lookup after dialect/contamination/stale
  checks: `orderedSRSDropEligible` + active entry + `srsDepsReader`
  capability commit the ordered branch (marker bound, Qo+Qdeps fresh,
  Qexist skipped, objectCache bypassed); otherwise the full path runs
  unchanged and the entry stays unconsumed.
- `MySQLSRSDependencyReader` optional interface
  (`ResolveMySQLSRSDependencies(ctx context.Context, srid uint32)`,
  re-judged on `state.provider` each enrich call — `context.Context`
  is the full type, per erratum 2) implemented by `mysqlmeta.Provider`.
- Mint/keep/consume/barrier per the contract: catalog-certified A arms
  mint (earliest wins), ordered-served A consumes without re-minting,
  same-SRID S2/boundary writes flip `barrier` while minting
  `srsExistenceStale` (`barrier ⇒ stale` invariant holds by
  construction), provenance is never deleted.

## R2 repairs over R1

- **R2-A**: the wiring closure now wraps every direct-reader error in
  `metadataObjectReadError{cause}` — public text is exactly
  `metadata object read failed`, `errors.Is` reaches the injected
  sentinel, the cause is never flattened away. The frozen same-enrich
  two-statement witness (`TestT26SRSOrderedDepsErrorSameEnrich`)
  produces catalog A on stmt1, fails the stmt2 ordered direct read,
  and asserts the bounded error, the unchanged stmt1 bound state and
  entry, the failed-forward count, and `Qexist=0` for stmt2.
- **R2-B**: `R` is now measured as full `ResolveObject` forwards plus
  direct dependency-reader forwarding attempts (failures included) —
  `r3ProductProvider.depsForward` counts every call before
  delegation. Ordered-hit cumulative vectors are `(2,1,2,2)`; the
  three-hit freshness row is `(3,1,3,3)`.
- **R2-C**: OP-5 is restored to the six frozen rows /5a../5f verbatim;
  OP-6 adds the real request/API rows (below-floor enrich, real TiDB
  `AuditSQL` vendor boundary, providerless public row, malformed-pin
  typed exit) with internal companions kept separate.
- **R2-D**: OP-7 asserts exact finding/rule/level/message/metadata
  shapes, positional `StatementIndex` separately (omitempty
  preserved), zero-gap / complete-coverage / reject-verdict rows, the
  disabled- and warning-level policy companions (entry still minted,
  earliest provenance unchanged), and a serialized-JSON provenance
  scan.
- **R2-E**: OP-8a snapshots stmt1's bound projection at four points
  (post-bind, post-own-apply, post-REVOKE-apply, final);
  `paSnapEqual` now compares `Dependencies.Complete` via `paDepsSame`
  with dedicated discrimination controls (nil-vs-empty, element
  change, Complete-only mismatch, bidirectional isolation). OP-8b
  permits only `barrier` + the authored stale key to differ; OP-8c
  starts with no 100001 entry and non-empty SERVER/table/schema
  controls.

## R3 repairs over R2 (test/evidence layer only — product unchanged)

The R3 ruling confirmed the product implementation needs no further
change and ordered a limited test/evidence revision. The entire delta
is `*_test.go` plus this receipt: the R3 product binary is
byte-identical to the R2 binary.

- **R3-A (ordered consumers / provider errors)**: the same-enrich
  error witness now uses independent per-statement snapshots and a
  hook-counted bind/apply split; every provider error row asserts a
  nil fact plus the full attempted vector. Two real ordered-consumer
  witnesses added (`TestT26SRSOrderedConsumerReaderVariants`):
  proven-unsupported Qo `(nil, nil)` consumes the ordered absence
  with dependencies nil and no dependency query after the support
  gate; unparseable Qo continues to the dependency read and binds a
  real empty fact. Qo responses are scripted per attempt
  (`versionScript`, mirroring `depsScript`).
- **R3-B (resolver seam / capability / lifecycle)**:
  `TestT26SRSOrderedForeignBoundNoPublication` and
  `TestT26SRSOrderedContradictoryBoundNoPublication` inject a custom
  scripted resolver at the real `mysqlSRSResolver` seam — foreign
  SRID `100002`-bound-while-authored-`100001` and contradictory
  same-SRID `Exists && Absence` states — both rejected by
  `ValidAbsence`, producing S2/unproven with authored-only stale mint
  and no ordered publication. `opNoCapProvider` (a provider adapter
  that deliberately does not implement `MySQLSRSDependencyReader`)
  proves no-capability warm-cache (stmt2 zero reads, no marker) and
  cold-cache (each enrich full read; changed catalog answer → S2;
  existing entry barrier flips while earliest provenance holds)
  behaviour with per-statement snapshots (`r3SnapReads`/`r3AssertDelta`).
- **R3-C (real table fixture / REVOKE / parse witness / real
  requests)**: `opTableProvider` is a table-only adapter over the real
  product provider scripting an existing `s.t` with geometry column
  `g`; table seam calls are counted separately and never inflate the
  SRS vector. `DROP TABLE` mints `srsDepsStale`; positive and empty
  dependency witnesses under staleness are exercised; an inert
  `ALTER TABLE … ENGINE=InnoDB` mints no dependency staleness.
  `TestT26SRSOrderedGarbageParseWitness` freezes the real parser
  behaviour for the exact fixture `GARBAGE@#$;` (swallows the tail
  into one failure record at line 1 column 48) and companion
  `GARBAGE;` (recovers; tail survives as a real statement).
  `TestT26SRSOrderedRevokeBetweenIEHits` /
  `NegativeGrantBothSides` freeze the REVOKE lifecycle rows
  (suppressed premise only after a positive consumable witness;
  negative grant stays unproven). `TestT26SRSOrderedBelowFloorRealChain`
  is now a real providerless request row with
  `TestT26SRSOrderedBelowFloorInternalCompanion` kept separate;
  `TestT26SRSOrderedTiDBParseCompanion` freezes TiDB parse shape.
- **R3-D (public layer)**: new `pkg/deltascope` file
  `audit_t26_srs_ordered_test.go` drives the public API for the
  plain absent pair, REVOKE lifecycle, reserved SRID with and
  without `IF EXISTS`, policy disabled, policy warning,
  providerless mode and malformed target version — asserting exact
  findings/levels/messages/metadata/coverage/verdicts,
  `errors.Is` against `ErrUnsupportedStatement` and
  `ErrInvalidTargetVersion`, and a recursive JSON leak scan over the
  serialised result (no `OrderedAbsent`/`source_arm`/`published_idx`/
  `own_privilege`/`barrier` tokens, with missing-vs-null
  discrimination). The public SDK adapter does not expose
  `ResolveMySQLSRSDependencies`, so public behaviour is
  mechanism-independent.
- **R3-E (snapshot fidelity / isolation)**: `ChannelControlsCapture`
  is now a true deep-value snapshot — servers, persistent tables,
  temporary tables, schemas, invalidated schemas, stale SRS set,
  dependency-stale flag, contamination flag, privilege scope-ended
  flag and ordered entries — cloning nested table snapshots via
  `cloneTableSnapshot` and preserving nil vs non-nil-empty
  distinctions. `TestT26SRSChannelControlsFidelity` adds the
  discriminating controls (nil-vs-empty, nested-element and map
  mutation, bidirectional isolation, invalidated-schema capture).
  OP-8a attributes L=1 to the whole pre-bind→post-bind enrich-entry
  window (the hook cannot isolate loader work more precisely).
  OP-8b uses the exact frozen three-statement shape
  (IE producer → `CREATE OR REPLACE` boundary → plain tail blocked
  with exactly `[B]`). OP-8c presets non-empty controls (SERVER
  `s1`, real `s.t` table entry, schema `s`, complete-provenance
  `100002` entry, no pre-entry for `100001`), captures post-bind and
  post-own-apply state, and proves exactly one new `100001` entry
  with every other channel verbatim.
- **R3-F (gates / build / identity / manifest)**: the verbose gate
  now covers the changed packages plus `internal/domain/rule/ddl`
  and `internal/domain/policy`; the same-condition rebuild is
  recorded against the b1 invocation; identity is bound in
  `identity-record.txt`; the manifest classifies every member as NEW
  or REUSED and registers the historical corrections below.

## R4 repairs over R3 (test/evidence layer only — product unchanged)

The R4 ruling again confirmed zero product change and ordered a
limited test/evidence revision. The entire delta is `*_test.go`
plus this receipt: the R4 product binary is byte-identical to the
R3/R2 binary (hash comparison below).

- **R4-A (capability forwarding / conversion layer / exact public
  shapes)**: `TestT26SRSOrderedPublicCapableInput` installs
  `opCapablePublicProvider` — a fixture that genuinely implements
  `ResolveMySQLSRSDependencies` — through the public SDK and proves
  the adapter never forwards it (`depsCalls == 0`,
  `resolverCalls == 1` for the pair, two absent blockers, complete
  coverage, reject verdict). `TestT26SRSOrderedConversionLayer`
  invokes `appaudit.AuditSQL` directly with a genuinely capable
  provider (measured resolver/deps vector `1/1` — a real ordered
  hit), converts through `fromDomainResult`, serializes, and asserts
  the same two-blocker/reject projection with a clean leak scan:
  the two layers' mechanisms and counts are asserted separately,
  never conflated. OP-7a/b/c/c′/d/e and OP-6d/e each carry exact
  per-statement assertions — canonical metadata key/value sets
  (`srid` as `uint32`, `outcome` string, `conditional_audit`
  presence/absence), each finding's own `StatementIndex`, exact
  cardinality, coverage status, verdict, and omission-vs-null shape
  (`evidence_gaps`/`findings` keys absent, never `null`).
- **R4-A3 (scan controls)**: `opPublicScan` is a real recursive walk
  over the serialized JSON document hunting internal provenance
  tokens; `TestT26SRSOrderedPublicScanControls` is the dedicated
  positive/negative control proving the walk detects an injected
  `barrier` key and accepts a clean document.
- **R4-B (capture fidelity)**: `ChannelControlsCapture` no longer
  delegates to the product `cloneTableSnapshot`. The new test-only
  `opDeepClone` (a mechanical `reflect` walk re-owning pointers,
  slices, maps and interface members) preserves observed value AND
  nilness verbatim — nil maps stay nil, non-nil-empty stays
  non-nil-empty — and `TestT26SRSChannelControlsFidelity` now
  discriminates with real different inputs: bare-state nil channel
  maps vs non-nil-empty maps, member-container nilness inside
  `TableSnapshot`, non-empty nested values (`Cardinality *int64`,
  `IdentityOptions map[string]any`, `KeyParts`, `View.Updatable`,
  `TTL`, `PartitionPlacements`), and bidirectional isolation across
  all those member kinds.
- **R4-C (two-timepoint / baseline asserts)**: OP-2g now captures
  the complete `SRSOrderedEntryView` at each post-apply and
  compares the full struct across both timepoints (stmt1's mint
  verbatim preserved by stmt2's no-capability fallback). OP-8b
  keeps the expected-from-before stale-set construction plus the
  stripped-channel verbatim compare; OP-8c keeps the internal-seam
  preset (`100002` complete provenance, SERVER `s1`, real `s.t`,
  schema `s`) with the post-bind `atBind` baseline proving no entry
  exists before own-apply.
- **R4-D (TiDB companion)**: `TestT26SRSOrderedTiDBParseCompanion`
  now walks the REAL `Extract` → `enrichStatements` → own-apply
  path on a TiDB-dialect `batchState` with a nil provider (any
  attempted catalog read would fail loudly). Both extracted rows
  carry the extractor's own `Unsupported{boundary:vendor}` mark;
  neither is adjudicated, neither commits the marker, the ordered
  channel stays empty across the batch, and the boundary own-apply
  still mints exactly `srsExistenceStale{100001}` — TiDB can never
  publish or consume an ordered entry even though
  `orderedStateDialect(tidb)` is true.
- **R4-E (evidence)**: `identity-record.txt` records the real
  commands run this round verbatim with immediate rc captures; a
  single bound build at C4 (product surface unchanged) produced the
  byte-identical binary, so the three probes are classified REUSED
  from R3 AND re-run against the R4 binary with all six streams
  byte-compared (`cmp` identical, rc 0/1/2); the manifest
  classifies every archive member NEW or REUSED individually.
  ⚠ R5 closeout correction (erratum 9): parts of this R4 evidence
  paragraph overclaimed — the manifest was an aggregate wildcard
  ledger, the "fresh rerun" transcripts carry no literal invocation
  binding, and the pre-format gofmt capture was cited as clean.
  The R5 section below supersedes the claim level, not the files.

## R5 evidence closeout (docs/evidence-only — zero Go changes)

The R4 ruling (CHANGES REQUIRED — evidence-only closeout) accepted
R4-A…D and the product semantics; this section answers the R5 §9
checklist item-by-item. R1/R2/R3/R4 records below are retained
verbatim — corrections are marked, never rewritten.

1. **C4/F4 current recheck** (`identity-recheck-r5.txt`, literal
   argv + immediate rc, captured now — labeled current recheck,
   never re-dated as R4 capture):
   - C4 `cfa4295712291979db14a618456106ad4b79b6b4`, tree
     `2d722436286c5f77e7a774a8f4c1a5a8b15c57af`, parent
     `d97907fb245c7b94ba3939009fd68407b2a95e33`.
   - F4 `19fd6de6df56ef6fd77d5dab5a631db42cdf617f`, tree
     `eb0a7f1270dbf4c2573f2207d632121736e31c5e`, parent
     `cfa4295712291979db14a618456106ad4b79b6b4`.
   - Branch `milestone/mysql-tidb-ddl-completion`;
     `git status --porcelain=v1 --untracked-files=no` empty.
   - F5 (this receipt + archive corrections) is appended
     sequentially — no amend/rebase.
2. **C4→F4 interval identity**: `git diff --name-status
   cfa42957..19fd6de6` = exactly `M
   docs/verification/t26-srs-drop-ordered-publication-execution-01/README.md`
   — the interval is docs-only by literal capture;
   `git diff --check` rc=0.
3. **Manifest**: `manifest-sha256.txt` now carries one row per
   actual member in the format `CLASS | bytes | SHA-256 | path |
   source-commit | source-archive | source-member`. Member count
   reconciled: **67 regular files = 66 manifest rows + 1
   self-excluded outer manifest**; nested historical manifests are
   listed as members (the prior `! -name` filter silently dropped
   them — registered, corrected).
4. **REUSED members**: every REUSED row names the source archive
   (`t26-srs-ordered-exec-r3.tar.gz`, 21,376,958 B, SHA-256
   `2c327422dcec85e4fac81bd16fd2e2b53ded97a4f70f79b015ea17e55831a41f`,
   bound to R3 candidate `de879003`), the source member path, and
   the member bytes/SHA; all 40 REUSED members were re-hashed
   against the extracted R3 archive — 40/40 match.
5. **gofmt evidence split**: `gates/static-checks-r4-preformat.txt`
   is relabeled the R4 **pre-format** capture (2 files listed — not
   a clean capture); `gates/gofmt-clean-r5.txt` is the R5 current
   clean capture (`gofmt -l` over the four literal file paths,
   zero-byte stdout, rc=0).
6. **Build**: bound by `build/build-verify-r5.txt` — literal cwd,
   literal build argv, build rc=0, literal `shasum`/`wc` argv, and
   a real `cmp deltascope-r5 b1` rc=0 against the archive `b1`
   (52,779,058 B / `fa1a9a6a…d02a2`). The R4 summary transcript is
   retained verbatim as `build/transcript.txt`.
7. **Probes**: `probes/cli1/2/3.{rc,stdout,stderr}` +
   `policy-srs-ordered.yaml` + `run-probes.sh` stay REUSED from the
   R3 archive (per-member source SHA verified). The R4 re-run
   outputs are re-homed under `probes/r4-unbound/` and classified
   **UNBOUND comparison artifacts — invocation binding not
   retained**; the "fresh rerun is bound evidence" claim is
   **withdrawn** (the transcripts corroborate only; the bound probe
   evidence is the REUSED R3 originals plus the shared byte-
   identical binary).
8. **Member count**: re-registered above — 67 files, 66 listed
   members, self-exclusion + nested-manifest handling explicit.
9. **History**: R1/R2/R3/R4 receipts, captures and claims are
   retained verbatim; every R5 correction is a relabel or an
   explicit withdrawal, never an overwrite.

## Reviewer-level errata registered

1. OP-R4-ER1 — cumulative contract diffstat `+565/-0` (the R4 close
   measure). This is a separate figure from the earlier `+531/-0`
   R4-commit measure and from R3's `+572/-0`; the three numbers are
   not interchangeable.
2. The interface signature names the complete `ctx context.Context`
   type — this implementation uses the full spelling; the abbreviated
   form in the contract text is a record-level erratum only.
3. R1 record-level erratum: the R1 receipt claimed "All rows
   EXECUTED" — withdrawn. Several rows were partial (OP-5 variants
   were not the frozen rows; OP-6 lacked real request/API witnesses;
   OP-7/OP-8 assertions were thinner than the contract). The R2
   mapping below is the corrected, per-row account.
4. R1 record-level erratum: "18 changed Go files" — corrected. The
   baseline→C2 diff is 20 files: **15 Go + 3 module READMEs + 2
   docs** (decision record + this receipt).
5. ADR date note: `docs/decisions/2026-11-18-…` follows the
   repository's sequential decision-record naming convention, not the
   execution timestamp; the filename is retained for sequence
   consistency.
6. R3 row-mapping errata over the R2 mapping: OP-7 rows now name the
   `pkg/deltascope` public-layer tests
   (`TestT26SRSOrderedPublic{PlainAbsentPair,RevokeLifecycle,
   ReservedIfExists,PolicyDisabled,PolicyWarning,Providerless,
   MalformedPin}`) — the R2-era internal `Public*` tests remain as
   realchain/internal companions, so both names appear per row.
   OP-1i/1j now name the real custom-resolver rows
   (`ForeignBoundNoPublication`/`ContradictoryBoundNoPublication`)
   alongside the retained apply-table test. OP-6b names the real
   request row plus its internal companion; OP-6c gains
   `TiDBParseCompanion`; OP-5c gains the internal exact-input witness
   `GarbageParseWitness`; the snapshot-fidelity row gains
   `TestT26SRSChannelControlsFidelity`; a new `R3-A3` row maps
   `TestT26SRSOrderedConsumerReaderVariants`.
7. R2 gate-package record-level erratum: the R2 seven-package gate
   list (`internal/domain/rule`, `internal/domain/report`,
   `internal/infrastructure/parser/tidb`) did not cover
   `internal/domain/rule/ddl` or `internal/domain/policy`. The R3
   gate replaces it with the changed packages plus `rule/ddl` and
   `policy`; the full `./...` gate is unchanged and covers all
   packages either way.
8. R4 row-mapping errata over the R3 mapping: the public-layer rows
   gain `TestT26SRSOrderedPublicCapableInput` (capable-input
   non-forwarding witness), `TestT26SRSOrderedConversionLayer`
   (application→public conversion companion), and
   `TestT26SRSOrderedPublicScanControls` (dedicated scan
   discrimination). `TestT26SRSOrderedTiDBParseCompanion` is now an
   Extract+enrich+own-apply row, no longer parse-only.
   `TestT26SRSChannelControlsFidelity` is restructured into four
   subtests (`channel-map-nilness`, `member-container-nilness`,
   `nested-values-and-isolation`, `required-channels-populated`) and
   `ChannelControlsCapture` now clones via the test-only
   `opDeepClone`, never the product `cloneTableSnapshot`. OP-2g's
   entry assertion is a two-timepoint complete-struct compare. The
   R4 gate is the two-package `-v` minimum over the changed
   packages per the R4 ruling; the R3 seven-package and full
   `./...` gates remain the standing round record.
9. R4 evidence-claim errata (per the R5 ruling, section seven):
   (a) "manifest classifies every archive member" was overclaimed —
   the R4 ledger used aggregate wildcard rows and its member count
   silently skipped nested manifests; the R5 7-column manifest is
   the corrective. (b) "fresh rerun is bound evidence" is
   **withdrawn** — the r4 probe transcripts kept no literal
   invocation binding and are reclassified UNBOUND comparison
   artifacts. (c) `gates/static-checks-r4-preformat.txt` is the
   PRE-FORMAT capture (2 files listed), never the clean capture —
   `gofmt-clean-r5.txt` is the clean one. (d) the R4 build summary
   transcript carried no real `cmp` argv — `build-verify-r5.txt`
   supplies the literal binding. No file content was rewritten;
   labels only.
10. R5-ER1 (reviewer-level attribution note, record-only):
    `identity-recheck-r5.txt` captured the current Git identity
    while F4 was HEAD — it does not contain F5's own
    `git rev-parse` output. Correct attribution: C4/F4 identity is
    proven by that original capture; F5
    `ff6069a5` / tree `84be9cb7` / parent `19fd6de6` is the R5
    delivery's own closeout identity recorded here. No amend was
    required — the ruling explicitly avoids a self-reference loop
    for the final evidence commit.

## OP mapping — expected vs measured

Vectors are `(R, Qexist, Qdeps, Qo)` measured at the r3 seam
counters; `R` includes direct deps-reader forwards per R2-B. L is the
privilege-loader count; Vd is N/A at the internal enrich layer.

| OP row | witness (test → subcase) | layer | measured |
|---|---|---|---|
| OP-1a baseline producer | `TestT26SRSOrderedBaselineProduceRealChain` | realchain | (1,1,1,1), L=1; entry {absent, absent_refused, consumable, idx 0} |
| OP-1b IE producer | `TestT26SRSOrderedInvalidationRealChain/OP-5a` stmt1 | realchain | absent_noop mint, marker on hit |
| OP-1c IE negative grant | `TestT26SRSOrderedProducerNegativeGrant` | realchain | absent_conditional + exactly [P]; entry ownPrivilege=unproven; (1,1,1,1), L=1 |
| OP-1d authored SRID 0 | `TestT26SRSOrderedProducerSRID0` | realchain | refused_srid0; no entry; (1,1,1,1) |
| OP-1e positive deps → S1 | `TestT26SRSOrderedNonProducersRealChain` (S1 section) | realchain | refused_dependency; no entry |
| OP-1f depsStale before first SRS | `TestT26SRSOrderedDepsStaleBeforeFirstSRS` | realchain | depsStale minted; tail producer A-plain |
| OP-1g unparseable Qo | `TestT26SRSOrderedUnparseableQo` | realchain | S2 + stale{100001}; no entry; (1,1,1,1) |
| OP-1h boundary before first drop | `TestT26SRSOrderedBoundaryBeforeFirstDrop` | realchain | tail blocked-B owed exactly [B]; stale{100001}; no entry |
| OP-1i/1j apply table | `TestT26SRSOrderedApplyTableInt` + `TestT26SRSOrderedForeignBoundNoPublication` + `TestT26SRSOrderedContradictoryBoundNoPublication` | internal | mint/keep/consume/contradictory/non-producer/barrier⇒stale rows; real scripted-resolver seam: foreign-SRID (100002 bound vs 100001 authored) and contradictory Exists&&Absence states both rejected by ValidAbsence → S2/unproven, authored-only stale, no publication, L counted |
| OP-2a plain ordered hit | `TestT26SRSOrderedHitPlainRealChain` | realchain | cumulative (2,1,2,2), L=1; hit delta (1,0,1,1) |
| OP-2b foreign SRID | `TestT26SRSOrderedNonProducersRealChain` (foreign section) | realchain | two entries, publishedIdx 0/1, sibling untouched |
| OP-2c IE ordered hit | `TestT26SRSOrderedHitIfExistsRealChain` | realchain | (2,1,2,2), L=1 |
| OP-2d shared batchState | `TestT26SRSOrderedSharedVsNewStateRealChain` first half | realchain | call1 (1,1,1,1); call2 delta (1,0,1,1) ordered hit |
| OP-2e new batchState | same test, second half | realchain | call2 (1,1,1,1), S2 + stale mint |
| OP-2f depsStale mid-batch | `TestT26SRSOrderedInvalidationRealChain/OP-5d` | realchain | (2,1,2,2); depsStale demotes only |
| OP-2g no-capability warm | `TestT26SRSOrderedNoCapabilityWarmCache` | realchain | capability absence proven by negative `MySQLSRSDependencyReader` assertion on `opNoCapProvider`; per-statement deltas via `r3SnapReads`: stmt1 (1,1,1,1), stmt2 Δ(0,0,0,0) cache-hit, no `OrderedAbsent` marker on stmt2; two-timepoint complete `SRSOrderedEntryView` compare (post-apply ×2 + final identical) |
| OP-2h no-capability cold | `TestT26SRSOrderedNoCapabilityColdCache` | realchain | per-enrich full read (1,1,1,1) ×2; changed catalog answer → S2; existing entry Barrier flips false→true while every other field stays verbatim (complete `SRSOrderedEntryView` compare); owed [deps, privilege] |
| OP-3a IE hit consumable | `TestT26SRSOrderedHitIfExistsRealChain` | realchain | absent_noop ordered-served |
| OP-3b IE→REVOKE→IE | `TestT26SRSOrderedRevokeBetweenIEHits` | realchain | stmt3 absent_conditional + exactly [P]; earliest (consumable) provenance kept; (2,1,2,2), L=1 |
| OP-3c negative grant both | `TestT26SRSOrderedNegativeGrantBothSides` | realchain | both absent_conditional + [P]; (2,1,2,2), L=1 |
| OP-3d negative IE + plain tail | `TestT26SRSOrderedNegativeGrantPlainTail` | realchain | ordered-served A-plain, no owed; (2,1,2,2), L=1 |
| OP-4a fresh deps per hit | `TestT26SRSOrderedFreshDepsRealChain` | realchain | deps attempts=3 (empty→empty→positive); third S1 outranks; (3,1,3,3) |
| OP-4b stale demotion | `TestT26SRSOrderedDepsStaleDemotesRealChain` + `TestT26SRSOrderedDepsStaleEmptyWitnessRealChain` | realchain | real `s.t` fixture (`opTableProvider`, geometry `g`): `DROP TABLE s.t` mints depsStale, real `LoadTableSnapshot` read counted separately and never inflates the SRS vector; stmt3 binds a positive deps fact `s.t.g@100001` while staleness demotes its use — ordered A holds; empty-witness twin proves stale deps never invalidate a proven absence |
| OP-4c same-enrich reader error | `TestT26SRSOrderedDepsErrorSameEnrich` + `DepsErrorRealChain` (two-enrich retained) | realchain | bounded text; `errors.Is` sentinel; no stmt2 adjudication/apply; stmt1 unchanged; failed forward counted; Qexist=0 |
| OP-4d skip label | `TestT26SRSOrderedConsumedPredicate` + realchain per-stmt deltas | internal + realchain | hit ΔQexist=0 asserted per statement |
| OP-4 provider variants | `TestT26SRSDepsReader{HealthyEmpty,PositiveWitness,ProvenUnsupported,UnparseableQoContinues,ErrorsPropagate}` | provider | exist attempts=0 on every row |
| OP-5a boundary barrier | `TestT26SRSOrderedInvalidationRealChain/OP-5a` | realchain | per-stmt Δ(1,1,1,1)/(0,0,0,0)/(0,0,0,0); barrier+stale; provenance kept |
| OP-5b siblings | `…/OP-5b` | realchain | (3,2,3,3); sibling provenance independent |
| OP-5c contamination | `…/OP-5c` + `TestT26SRSOrderedGarbageParseWitness` (internal, exact input) | realchain + internal | real `GARBAGE;` parse-failure record; tail zero reads; entry kept. Internal witness freezes the real parser for both fixtures: `GARBAGE@#$;` swallows the tail into one failure record (RawSQL contains `GARBAGE@#$;DROP SPATIAL REFERENCE SYSTEM 100001;`, line 1 column 48); `GARBAGE;` recovers — two statements, one failure `RawSQL == "GARBAGE;"` |
| OP-5d deps-stale only | `…/OP-5d` | realchain | (2,1,2,2); ordered A consumes; no barrier |
| OP-5e inert ALTER | `…/OP-5e` | realchain | no flag mints; (2,1,2,2) |
| OP-5f REVOKE scope | `…/OP-5f` | realchain | scope ends, entry survives; plain tail A-plain |
| OP-6a in-domain baseline | `TestT26SRSOrderedBaselineProduceRealChain` | realchain | (1,1,1,1), L=1 |
| OP-6b below floor | `TestT26SRSOrderedBelowFloorRealChain` (real providerless request, no SeedResolvedVersion) + `TestT26SRSOrderedBelowFloorInternalCompanion` | public-shape + internal | real request unsupported item `drop_spatial_reference_system` / `mysql` / object `spatial_reference_system`; zero findings/gaps; companion pins stale{100001} + (1,1,1,1), L=0 |
| OP-6c TiDB vendor boundary | `TestT26SRSOrderedTiDBVendorBoundaryPublic` + `TestT26SRSOrderedEligibilityInt` + `TestT26SRSOrderedTiDBParseCompanion` | public + internal | Vd=1; zero SRS/privilege reads; one vendor-boundary item; companion now walks real Extract→enrich→own-apply on a TiDB batchState (nil provider): extractor marks both rows `Unsupported{boundary:vendor}`, no adjudication/marker/entry ever, boundary own-apply mints exactly stale{100001} |
| OP-6d providerless | `TestT26SRSOrderedProviderlessPublic` + `ProviderlessInternal` twin | public + internal | exact [E,D,P] gap; unverified coverage; no marker/entry |
| OP-6e malformed pin | `TestT26SRSOrderedMalformedPinPublic` | public | `errors.Is(ErrInvalidTargetVersion)`; all counters 0; Vd=0 |
| OP-7a two plain drops | `TestT26SRSOrderedPublicPlainAbsentPair` (pkg/deltascope) + `TestT26SRSOrderedPublicAbsentPlain` (internal companion) | public | identical non-positional `ddl.mysql.srs.drop.absent` findings ×2; own index asserted; complete+reject; recursive JSON leak scan |
| OP-7b OP-3b batch public | `TestT26SRSOrderedPublicRevokeLifecycle` (pkg/deltascope + internal twin) | public | REVOKE unsupported item; stmt3 `unproven_privilege` gap [P]; `conditional_audit: observed_account_new_session` metadata on the absent_noop finding; leak scan |
| OP-7c reserved co-fire | `TestT26SRSOrderedPublicReservedIfExists` (pkg/deltascope) + `TestT26SRSOrderedPublicReservedCoFire` + `PublicReservedPlain` controls | public | absent_noop + reserved advisory (exact shapes; `srid` asserted as uint32); plain control has no advisory |
| OP-7d disabled | `TestT26SRSOrderedPublicPolicyDisabled` (pkg/deltascope) + `TestT26SRSOrderedPolicyDisabled` companion | public + internal | zero findings/gaps; complete; pass; companion shows entry minted |
| OP-7e warning level | `TestT26SRSOrderedPublicPolicyWarning` (pkg/deltascope) + `TestT26SRSOrderedPolicyWarning` companion | public + internal | warning-level findings; complete; review; publication unchanged |
| OP-6d/6e public layer | `TestT26SRSOrderedPublicProviderless` + `TestT26SRSOrderedPublicMalformedPin` (pkg/deltascope) | public | providerless: exact [E,D,P] gap + unverified coverage; malformed pin: `errors.Is(ErrInvalidTargetVersion)`, no result |
| OP-7 supplementary | `TestT26SRSOrderedPublicProjection` | realchain → evaluate | absent_noop shape; zero gaps; leak scan |
| OP-8a four-point snapshots | `TestT26SRSOrderedSnapshotInvariance` | realchain | post-bind / post-own-apply / post-REVOKE-apply / final identical; channel transitions asserted separately; L=1 attributed to the whole pre-bind→post-bind enrich-entry window |
| OP-8b barrier-only diff | `TestT26SRSOrderedBoundaryFlipIsolation` | realchain | exact frozen three-statement shape (IE producer → CREATE OR REPLACE boundary → plain tail); only `barrier` + authored stale key differ; tail blocked with exactly `[B]`; `srsDepsStale`/`contaminated` unchanged |
| OP-8c sibling isolation | `TestT26SRSOrderedSiblingIsolation` | realchain | presets SERVER `s1` + real `s.t` entry + schema `s` + complete-provenance `100002`; post-bind no `100001`; post-own-apply adds exactly `100001`; other channels verbatim; Δ(1,1,1,1), L=1 |
| snapshot fidelity | `TestT26SRSChannelControlsFidelity` (4 subtests) + `paSnapEqual`/`paDepsSame` discrimination controls | internal helpers | capture built on test-only `opDeepClone` (no product helper): nil channel maps stay nil, non-nil-empty stays non-nil-empty, member-container nilness preserved, non-empty nested values (`Cardinality *int64`, `IdentityOptions`, `KeyParts`, `View.Updatable`, `TTL`, `PartitionPlacements`) verbatim, bidirectional isolation across all member kinds; paDepsSame nil-vs-empty/element/Complete-only controls |
| R4-A1 capable input | `TestT26SRSOrderedPublicCapableInput` | public SDK | provider genuinely implements `ResolveMySQLSRSDependencies`; through the SDK adapter depsCalls=0, resolverCalls=1, two absent blockers, complete coverage, reject verdict; leak scan clean |
| R4-A1 conversion | `TestT26SRSOrderedConversionLayer` | application→public | `appaudit.AuditSQL` direct with capable provider: measured resolver/deps 1/1 (real ordered hit); `fromDomainResult` + serialize: two absent blockers, exact message, reject verdict, no provenance leak |
| R4-A3 scan control | `TestT26SRSOrderedPublicScanControls` | public | recursive walk detects injected `barrier` key (negative control) and accepts clean document (positive control) |
| R3-A3 ordered consumers | `TestT26SRSOrderedConsumerReaderVariants` | realchain | proven-unsupported Qo `(nil,nil)`: ordered absence consumed, deps nil, no deps query after support gate; unparseable Qo: continues to deps read, binds real empty fact; per-attempt scripted Qo; exact vectors + arm/provenance asserted |
| regression twins | `TestT26SRSAbsentSameSRIDCacheHit` (2,1,2,2); `TestT26SRSAbsentRecachesPerEnrichRealChain` | existing PA rows | ordered-hit vector under corrected R |

## Gates (bound to C4 `cfa42957`, tree `2d722436`, parent `d97907fb`)

The R4 ruling scoped the gate to the two changed packages; the R3
seven-package and full `./...` gates remain the standing record for
the unchanged product surface.

- `go test -v ./internal/application/audit/ ./pkg/deltascope/`:
  rc=0; 7049 assertion lines (`=== RUN`/`--- PASS`), 0 `--- FAIL`;
  both package `ok` lines.
- `go vet ./internal/application/audit/ ./pkg/deltascope/`: rc=0,
  empty output.
- `gofmt -l` over the 4 R4 changed Go files: the PRE-FORMAT
  capture listed 2 files (`static-checks-r4-preformat.txt`); the
  R5 current clean capture `gofmt-clean-r5.txt` is zero-byte,
  rc=0 — the two captures are kept separate per erratum 9(c).
- `git diff --check d97907fb..cfa42957`: rc=0, empty output.

Raw command/stdout/stderr/rc files per gate live under `gates/` in the
archive.

## Build + probes (R5 evidence model)

- **Build**: bound by `build/build-verify-r5.txt` — a literal
  current verification: `pwd`, literal
  `CGO_ENABLED=1 go build -tags postgresql -buildvcs=false
  -o /tmp/t26-ordered-exec-r4/build/deltascope-r5 ./cmd/deltascope`
  rc=0, literal `shasum -a 256` (fa1a9a6a…d02a2) and `wc -c`
  (52,779,058 B), then literal `cmp deltascope-r5 b1` rc=0 — the
  current build is byte-identical to the archive `b1`, which is
  byte-identical to the R3/R2 product binary. The R4 summary
  transcript (`build/transcript.txt`) is retained verbatim but no
  longer cited as the binding. Toolchain go1.26.1 darwin/arm64;
  go.mod SHA `80dd9105…b628c98`, go.sum SHA `cc5fa030…000696`.
- **Probes**: the bound probe evidence is the REUSED R3 originals
  (`probes/cli1/2/3.{rc,stdout,stderr}`, `policy-srs-ordered.yaml`,
  `run-probes.sh` — per-member SHA verified against the R3 archive)
  exercised by a byte-identical binary:
  - CLI-1 (`--target-version 8.0.36`): rc=0, stdout 896 B — verdict
    review / coverage unverified / exactly one
    `ddl.mysql.srs.drop.dependency` `unknown_srs_state` gap with
    required facts `[srs.existence, srs.column_dependencies,
    srs.drop.privilege]`, no findings, no ordered provenance keys.
  - CLI-2 (no version): rc=1, stdout 1152 B — existing version-gate
    unsupported projection, findings 0 / gaps 0.
  - CLI-3 (`8.0.x`): rc=2, stdout 0 B, stderr exactly
    `target_version must match [v]MAJOR.MINOR.PATCH: "8.0.x"\n`
    (56 B).
- `probes/r4-unbound/` holds the R4 ad-hoc re-run outputs as
  **UNBOUND comparison artifacts** (invocation binding not
  retained); they corroborate byte-equality and are not claimed as
  bound evidence.
- R1/R2/R3 records are retained under `historical-r3/` marked
  **reported / uncaptured** where their raw invocation binding was
  not recorded.

## Deferred / not in this round

Successful-deletion-driven absence publication, ordered-present,
dependency completeness, SRS CREATE, ALTER INSTANCE, T27 — all
explicitly out of scope.
