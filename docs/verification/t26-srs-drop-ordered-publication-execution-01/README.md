# T26-SRS-DROP-ORDERED-PUBLICATION-EXECUTION-01 — receipt (R2)

Implementation + verification round over the accepted contract
`docs/verification/t26-srs-drop-ordered-publication-contract-01/`
(R4, closed at `1346f230`). Status: **submitted / pending reviewer
ruling** — the R1 delivery was returned CHANGES REQUIRED; this
receipt is the R2 resubmission bound to candidate `fc872115`
(tree `dacd7e7b`, parent `99454da2`). Expected vectors come from the
frozen case-matrix; every measured number below was captured from raw
logs under this candidate — expected and measured are never conflated.

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
| OP-1i/1j apply table | `TestT26SRSOrderedApplyTableInt` | internal | mint/keep/consume/contradictory/non-producer/barrier⇒stale rows |
| OP-2a plain ordered hit | `TestT26SRSOrderedHitPlainRealChain` | realchain | cumulative (2,1,2,2), L=1; hit delta (1,0,1,1) |
| OP-2b foreign SRID | `TestT26SRSOrderedNonProducersRealChain` (foreign section) | realchain | two entries, publishedIdx 0/1, sibling untouched |
| OP-2c IE ordered hit | `TestT26SRSOrderedHitIfExistsRealChain` | realchain | (2,1,2,2), L=1 |
| OP-2d shared batchState | `TestT26SRSOrderedSharedVsNewStateRealChain` first half | realchain | call1 (1,1,1,1); call2 delta (1,0,1,1) ordered hit |
| OP-2e new batchState | same test, second half | realchain | call2 (1,1,1,1), S2 + stale mint |
| OP-2f depsStale mid-batch | `TestT26SRSOrderedInvalidationRealChain/OP-5d` | realchain | (2,1,2,2); depsStale demotes only |
| OP-2g no-capability warm | `TestT26SRSOrderedNoCapabilityWarmCache` | realchain | stmt2 (0,0,0,0) cache-hit; total (1,1,1,1) |
| OP-2h no-capability cold | `TestT26SRSOrderedNoCapabilityColdCache` | realchain | per-enrich (1,1,1,1) ×2; barrier flips; owed [deps, privilege] |
| OP-3a IE hit consumable | `TestT26SRSOrderedHitIfExistsRealChain` | realchain | absent_noop ordered-served |
| OP-3b IE→REVOKE→IE | `TestT26SRSOrderedRevokeBetweenIEHits` | realchain | stmt3 absent_conditional + exactly [P]; earliest (consumable) provenance kept; (2,1,2,2), L=1 |
| OP-3c negative grant both | `TestT26SRSOrderedNegativeGrantBothSides` | realchain | both absent_conditional + [P]; (2,1,2,2), L=1 |
| OP-3d negative IE + plain tail | `TestT26SRSOrderedNegativeGrantPlainTail` | realchain | ordered-served A-plain, no owed; (2,1,2,2), L=1 |
| OP-4a fresh deps per hit | `TestT26SRSOrderedFreshDepsRealChain` | realchain | deps attempts=3 (empty→empty→positive); third S1 outranks; (3,1,3,3) |
| OP-4b stale demotion | `TestT26SRSOrderedDepsStaleDemotesRealChain` | realchain | ordered A holds under positive-but-stale witness |
| OP-4c same-enrich reader error | `TestT26SRSOrderedDepsErrorSameEnrich` + `DepsErrorRealChain` (two-enrich retained) | realchain | bounded text; `errors.Is` sentinel; no stmt2 adjudication/apply; stmt1 unchanged; failed forward counted; Qexist=0 |
| OP-4d skip label | `TestT26SRSOrderedConsumedPredicate` + realchain per-stmt deltas | internal + realchain | hit ΔQexist=0 asserted per statement |
| OP-4 provider variants | `TestT26SRSDepsReader{HealthyEmpty,PositiveWitness,ProvenUnsupported,UnparseableQoContinues,ErrorsPropagate}` | provider | exist attempts=0 on every row |
| OP-5a boundary barrier | `TestT26SRSOrderedInvalidationRealChain/OP-5a` | realchain | per-stmt Δ(1,1,1,1)/(0,0,0,0)/(0,0,0,0); barrier+stale; provenance kept |
| OP-5b siblings | `…/OP-5b` | realchain | (3,2,3,3); sibling provenance independent |
| OP-5c contamination | `…/OP-5c` | realchain | real `GARBAGE;` parse-failure record; tail zero reads; entry kept |
| OP-5d deps-stale only | `…/OP-5d` | realchain | (2,1,2,2); ordered A consumes; no barrier |
| OP-5e inert ALTER | `…/OP-5e` | realchain | no flag mints; (2,1,2,2) |
| OP-5f REVOKE scope | `…/OP-5f` | realchain | scope ends, entry survives; plain tail A-plain |
| OP-6a in-domain baseline | `TestT26SRSOrderedBaselineProduceRealChain` | realchain | (1,1,1,1), L=1 |
| OP-6b below floor | `TestT26SRSOrderedBelowFloorRealChain` | realchain | version boundary; stale{100001}; (1,1,1,1), L=0 |
| OP-6c TiDB vendor boundary | `TestT26SRSOrderedTiDBVendorBoundaryPublic` + `TestT26SRSOrderedEligibilityInt` companion | public + internal | Vd=1; zero SRS/privilege reads; one vendor-boundary item |
| OP-6d providerless | `TestT26SRSOrderedProviderlessPublic` + `ProviderlessInternal` twin | public + internal | exact [E,D,P] gap; unverified coverage; no marker/entry |
| OP-6e malformed pin | `TestT26SRSOrderedMalformedPinPublic` | public | `errors.Is(ErrInvalidTargetVersion)`; all counters 0; Vd=0 |
| OP-7a two plain drops | `TestT26SRSOrderedPublicAbsentPlain` | public | identical non-positional findings; own index asserted; complete+reject; leak scan |
| OP-7b OP-3b batch public | `TestT26SRSOrderedPublicRevokeLifecycle` | public | REVOKE unsupported item; stmt3 unproven_privilege gap [P]; leak scan |
| OP-7c reserved co-fire | `TestT26SRSOrderedPublicReservedCoFire` + `PublicReservedPlain` control | public | absent_noop + reserved advisory (exact shapes); plain control has no advisory |
| OP-7d disabled | `TestT26SRSOrderedPolicyDisabled` | public + internal | zero findings/gaps; complete; pass; companion shows entry minted |
| OP-7e warning level | `TestT26SRSOrderedPolicyWarning` | public + internal | warning-level findings; complete; review; publication unchanged |
| OP-7 supplementary | `TestT26SRSOrderedPublicProjection` | realchain → evaluate | absent_noop shape; zero gaps; leak scan |
| OP-8a four-point snapshots | `TestT26SRSOrderedSnapshotInvariance` | realchain | post-bind / post-own-apply / post-REVOKE-apply / final identical; channel transitions asserted separately |
| OP-8b barrier-only diff | `TestT26SRSOrderedBoundaryFlipIsolation` | realchain | only `barrier` + authored stale key differ; controls equal |
| OP-8c sibling isolation | `TestT26SRSOrderedSiblingIsolation` | realchain | only 100001 added; SERVER/table/schema/100002 controls verbatim |
| snapshot fidelity | `paSnapEqual`/`paDepsSame` discrimination controls | internal helpers | nil-vs-empty, element change, Complete-only mismatch, bidirectional isolation |
| regression twins | `TestT26SRSAbsentSameSRIDCacheHit` (2,1,2,2); `TestT26SRSAbsentRecachesPerEnrichRealChain` | existing PA rows | ordered-hit vector under corrected R |

## Gates (bound to C2 `fc872115`, tree `dacd7e7b`, parent `99454da2`)

- `go test -count=1 -v` over the seven packages
  (`internal/application/audit`, `internal/domain/spec`,
  `internal/domain/rule`, `internal/domain/report`,
  `internal/infrastructure/metadata/mysql`,
  `internal/infrastructure/parser/tidb`, `pkg/deltascope`): rc=0;
  6781 `--- PASS` / 1 `--- SKIP` / 0 `--- FAIL`; 7 package `ok`
  lines.
- `go test -count=1 ./...`: rc=0; 39 package `ok`, 0 FAIL, 5
  `[no test files]`.
- `go vet` over the same seven packages: rc=0, empty output.
- `gofmt -l` over the 15 baseline→C2 changed Go files (expanded argv
  in `gates/04-gofmt.command.txt`): rc=0, zero-byte output.
- `git diff --check 1346f230..fc872115`: rc=0, empty output.

Raw command/stdout/stderr/rc files per gate live under `gates/` in the
archive.

## Build + probes (bound to C2 `fc872115`)

- `CGO_ENABLED=1 go build -tags postgresql -buildvcs=false
  -o <path> ./cmd/deltascope` run twice, identical cwd/toolchain/env:
  both builds 52,779,058 B, SHA-256
  `fa1a9a6a7e8107cd004f592ef9ee3aa8bb708411fde7d8f0a5b98798425d02a2`,
  `cmp` rc=0. Toolchain go1.26.1 darwin/arm64; go.mod SHA
  `80dd9105…b628c98`, go.sum SHA `cc5fa030…000696`. Full transcript
  in `build/build-transcript.txt`.
- Three providerless mode-off probes against the b1 binary
  (`probes/run-probes.sh`, six-rule policy, expanded invocations,
  immediate `$?` capture — full transcript in
  `probes/probe-transcript.txt`):
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
- R1 probe/build results (CLI rc 0/1/2, bytes 896/1152/56; two
  identical 52,778,946 B builds) are retained under
  `historical-r1/` marked **reported / uncaptured** — their content
  was validated but they were never bound to a recorded binary
  invocation; the R2 records above are the bound evidence.

## Deferred / not in this round

Successful-deletion-driven absence publication, ordered-present,
dependency completeness, SRS CREATE, ALTER INSTANCE, T27 — all
explicitly out of scope.
