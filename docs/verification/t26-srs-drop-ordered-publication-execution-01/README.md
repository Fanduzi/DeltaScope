# T26-SRS-DROP-ORDERED-PUBLICATION-EXECUTION-01 — receipt

Implementation + verification round over the accepted contract
`docs/verification/t26-srs-drop-ordered-publication-contract-01/`
(R4, closed at `1346f230`). Expected vectors come from the frozen
case-matrix; every measured number below was captured from raw logs
under this candidate — expected and measured are never conflated.

## Scope

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

## Reviewer-level R4 errata registered

1. Cumulative contract diffstat attribution: `+565/-0` (corrected
   from an earlier `+572/-0` attribution).
2. The interface signature names the complete `ctx context.Context`
   type — this implementation uses the full spelling; the abbreviated
   form in the contract text is a record-level erratum only.

## OP mapping — expected vs measured

All rows EXECUTED. Vectors are `(R, Qexist, Qdeps, Qo)` measured at
the r3 seam counters; L is the privilege-loader count; Vd is N/A at
the internal enrich layer.

| OP row | witness (test → subcase) | layer | measured vector |
|---|---|---|---|
| OP-1 producers/non-producers, foreign SRID | `TestT26SRSOrderedNonProducersRealChain`; `TestT26SRSOrderedApplyTableInt` (mint/keep/consume/contradictory/non-producer arms) | realchain + internal | minted entries asserted via `SRSOrderedEntry` view |
| OP-2a real ordered hit | `TestT26SRSOrderedHitPlainRealChain` | realchain (mysqlmeta) | cumulative (1,1,2,2), L=1; hit's own delta (0,0,1,1) |
| OP-2c ordered hit IF EXISTS | `TestT26SRSOrderedHitIfExistsRealChain` | realchain | (1,1,2,2), L=1 |
| OP-2d shared batchState ordered hit | `TestT26SRSOrderedSharedVsNewStateRealChain` | realchain | call1 (1,1,1,1); call2 delta (0,0,1,1) |
| OP-2e new batchState full path | same test, second half | realchain | (1,1,1,1), S2 + stale mint |
| OP-2g no-capability warm cache | `TestT26SRSOrderedNoCapabilityWarmCache` | realchain, `opNoCapProvider` | stmt1 (1,1,1,1); stmt2 (0,0,0,0) cache-hit; total (1,1,1,1), L=1, Vd=N/A |
| OP-2h no-capability cold cache | `TestT26SRSOrderedNoCapabilityColdCache` | realchain | per-enrich (1,1,1,1) ×2; barrier false→true; owed exactly `[srs.column_dependencies, srs.drop.privilege]` |
| OP-4a fresh deps per hit | `TestT26SRSOrderedFreshDepsRealChain` | realchain | deps attempts=4 (empty→empty→positive→S1), Qo=4, exist=1 |
| OP-4b stale demotion | `TestT26SRSOrderedDepsStaleDemotesRealChain` | realchain | ordered A holds under positive-but-stale witness |
| OP-4c reader error | `TestT26SRSOrderedDepsErrorRealChain` | realchain | `errors.Is` sentinel; deps attempt counted; prior snapshot + entry unchanged |
| OP-4 direct-reader variants | `TestT26SRSDepsReader{HealthyEmpty,PositiveWitness,ProvenUnsupported,UnparseableQoContinues,ErrorsPropagate}` | provider (stub driver) | exist attempts=0 on every row |
| OP-5 invalidation | `TestT26SRSOrderedInvalidationRealChain` /5a..5e | realchain | boundary barrier+stale; inert no-op; deps-stale demote-only; REVOKE scope ends premise not entry; contamination blocks consumption |
| OP-6 admission | `TestT26SRSOrderedAdmissionRealChain` + `TestT26SRSOrderedEligibilityInt` | realchain + internal | version boundary → no marker/entry; providerless → S2 then B; eligibility conjuncts unit-tested |
| OP-7 public projection | `TestT26SRSOrderedPublicProjection` | realchain → evaluate | identical absent_noop findings, zero gaps, no provenance keys in serialized output |
| OP-8 snapshots | `TestT26SRSOrderedSnapshotInvariance` (+ `paSnapEqual` field extension) | realchain | producer bound state invariant across post-apply/final; barrier flip and sibling isolation asserted via `SRSOrderedEntry`/`SRSOrderedSRIDs` |
| Regression twins | `TestT26SRSAbsentSameSRIDCacheHit` (ordered-hit vector (1,1,2,2,1,1)); `TestT26SRSAbsentRecachesPerEnrichRealChain` (non-minting call1 preserves per-enrich cache witness) | existing PA rows updated to ordered semantics | see tests |

## Gates (bound to C `5f9e8439`, tree `094ac461`, parent `1346f230`)

- `go test -count=1 -v` over the seven packages: rc=0; 8414 `--- PASS`
  / 1 `--- SKIP` / 0 `--- FAIL` (all indentation levels); 7 package
  `ok` lines, 0 `[no test files]`.
- `go test -count=1 ./...`: rc=0; 39 package `ok`, 0 FAIL.
- `go vet` over the same seven packages: rc=0, empty output.
- `gofmt -l` over the 18 baseline→C changed Go files (expanded argv in
  `gates/04-gofmt.cmd`): rc=0, zero-byte output. The unrelated
  baseline `mysql_t26r5_test.go` formatting issue is out of scope and
  untouched.
- `git diff --check 1346f230..5f9e8439`: rc=0, empty output.

## Build + probes

- `CGO_ENABLED=1 go build -tags postgresql -buildvcs=false
  -o <path> ./cmd/deltascope` run twice, identical cwd/toolchain/env:
  both builds 52,778,946 B, SHA-256
  `b1d441d98e5c9a0c25ca7f1c0c6cfab92f27a8ecf8c867b949e124e4011a446b`,
  `cmp` rc=0. Toolchain go1.26.1 darwin/arm64; go.mod SHA
  `80dd9105…`, go.sum SHA `cc5fa030…` (full values in
  `build/toolchain.txt`).
- Three providerless mode-off probes against the new binary
  (`probes/run-probes.sh`, six-rule policy, immediate `$?` capture):
  - CLI-1 (`--target-version 8.0.36`): rc=0, stdout 896 B — verdict
    review / coverage unverified / exactly one
    `ddl.mysql.srs.drop.dependency` `unknown_srs_state` gap with
    required facts `[srs.existence, srs.column_dependencies,
    srs.drop.privilege]`, no findings, `index=0` present, no ordered
    marker.
  - CLI-2 (no version): rc=1, stdout 1152 B — existing version-gate
    unsupported projection, findings 0 / gaps 0.
  - CLI-3 (`8.0.x`): rc=2, stdout 0 B, stderr exactly
    `target_version must match [v]MAJOR.MINOR.PATCH: "8.0.x"\n`
    (56 B).

## Deferred / not in this round

Successful-deletion-driven absence publication, ordered-present,
dependency completeness, SRS CREATE, ALTER INSTANCE, T27 — all
explicitly out of scope.
