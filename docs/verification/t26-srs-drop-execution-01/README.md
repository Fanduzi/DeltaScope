# T26-SRS-DROP-EXECUTION-01 — execution record

Contract: `T26-SRS-DROP-CONTRACT-01` (accepted at `d68f4ab7` plus the R10
evidence supplement `parser-type-subtree.txt`). This directory records the
*execution* of that contract — the contract document and its PLAN/UNEXECUTED
records are unchanged and remain the frozen expectation source.

## Identity

| Round | Candidate | Parent | Status |
|---|---|---|---|
| R1 | `3c74445cea952d657e32a31ffb5c20057abbf511` | `d68f4ab7` | submitted; R1 ruling CHANGES REQUIRED |
| R2 | `35d41f5b52248364a7339794f58c80a0625e0277` | `3c74445c` | submitted, pending ruling |

Branch `milestone/mysql-tidb-ddl-completion`. No push, merge, tag, release,
or issue close.

## Scope

Four adjudication arms (`blocked`, `refused_srid0`,
`refused_dependency`, `unproven`), the two invalidation channels
(`srsExistenceStale` per-SRID, `srsDepsStale` batch-wide), three rules
(`ddl.mysql.srs.drop.srid0` / `.dependency` / `.reserved`), the D3
spatial-touching classifier, and the enrich-time version gate. Deferred:
proven-absent, ordered publication, SRS privilege witnesses, SRS CREATE
auditing, ALTER INSTANCE, T27.

## Row coverage

| Row | Witness (layer) |
|---|---|
| E1/E4/E5.11 refused_dependency | `TestT26SRSR1RefusedDependency`, `ZeroRowExistencePositiveDeps`, `TestT26SRSR2ReservedSuppressedInRefusal` |
| E1.2/E5.2 IF EXISTS | `IfExistsSameArm` (S1), `TestT26SRSR2IfExistsSRID0` (S0) |
| E2.* arm/owed projections | `UnprovenPresentEmptyDeps`, `AdjudicationUnit` (spec-level cells) |
| E3.1–E3.3 provider shapes | `UnprovenProviderless`, `UnprovenNoResolver`, `TestT26SRSR2RequestZeroRow` (request layer) |
| E5 reserved cells | `ReservedEndpoints` (helper table), `TestT26SRSR2ReservedStatementCells` (statement level, all three blocks + ceiling) |
| E5.7–E5.10 unclaimed forms | `UnclaimedForms` (parser diagnostics) |
| E6.1–E6.3 repeat SRID | `S2MintsExistenceStale`, `S0RepeatReads`, `S1RepeatRefuses` |
| E7.1/E7.2 boundary lifecycle | `BoundaryLifecycleMintsStale`, `BoundaryDifferentSRID`, `TestT26SRSR2BoundaryDifferentSRIDRefuses` |
| E8.1–E8.11 spatial touching | `DropTableTouchesDeps`, `InertCreateKeepsWitness`, `TestT26SRSR2InertCreateEmptyIncomplete`, `CompactTouchesWithoutContamination`, `DepsStaleCrossCall`, `RefusedThenTouching`, `AlterDropColumnTouches`, `InertAlterKeepsWitness`, `SchemaOps`, `CreateLikeTouches`, `D3ClassifierEdges`, `TestT26SRSR2D3Discriminating` |
| E8.3/E8.3b | `TestT26SRSR2GeometryTypeRefusal` — real GEOMETRY Type-position parse failures (vendored grammar has no `GEOMETRY` token and no identifier fallback), not seeded contamination |
| E9.1–E9.3 ordering | `ParseFailureBlocks`, `TestT26SRSR2AuthoredCoordinates` (failure (1,1) / DROP (1,10)), `TwoCallContamination` |
| B priority/sources | `BlockedOutranksRefusals`, `BlockedSourceDistinction`, `BlockedPreservesState` |
| E10 version gate | `VersionGate` (domain cells + providerless pins) + `TestT26SRSR2TiDBVendorBoundary`, `MalformedPin`, `DiscoveryMismatch`, `ProductMismatch`, `FloorFullProjection` |
| E11 error paths | `ErrorPaths`, `DiscoveryError`, `TestT26SRSR2ErrorSeamAccounting`, `TestT26SRSR2DiscoverySentinel` |
| E12 public/fidelity | `PublicShape`, `TestT26SRSR2SerializationScan` (secret identifiers scanned over serialized result), `TestT26SRSR2UnprovenOwnApplySnapshot`, `TestT26SRSR2TouchingApplyThenSRSMint`, `SiblingFamilyNoCrosstalk`, `TestT26SRSR2ServerPrivilegeIsolation` |

Read-shape columns follow the contract order R / Qexist / Qdeps / Qo / L
(+Vd for request discovery). Model-layer cells whose inputs no legal SQL
produces are labeled model-unit, not parser-integration witnesses.

## Probes (R2 binary, `-buildvcs=false`, byte-identical rebuild)

Recipe correction per the R2 order: the policy keeps only the three
rule entries (`enabled:true` + level), no top-level `dialect` key — the
policy decoder rejects that key — and `--dialect mysql` passes the
dialect explicitly. Providerless, `--format json --fail-on none`, single
`DROP SPATIAL REFERENCE SYSTEM 100001;`.

| Probe | Expected | Actual |
|---|---|---|
| P1 (`--target-version 8.0.36`) | rc 0; review; statement+aggregate `unverified`; findings absent; one `unknown_srs_state` gap, facts `[existence, column_dependencies, drop.privilege]` | exact match — rc 0, all fields verbatim |
| P2 (no pin) | rc 1; review; `incomplete`; findings/gaps absent; frozen version-gate unsupported entry + diagnostic; `--fail-on none` does not override the Unsupported-first exit | exact match — rc 1, `{boundary:"unaudited",gate:"version",object:"spatial_reference_system"}` + `unsupported_statement` diagnostic |
| P3 (`--target-version 8.0.x`) | rc 2; stdout 0 B; stderr `target_version must match [v]MAJOR.MINOR.PATCH: "8.0.x"` | exact match — rc 2, stderr verbatim |

Serialization detail: `$.statements[0].index` serializes `0` (the field
has no `omitempty`) — present and correct; the contract's omission note
governs `$.unsupported[0].index`, which stays absent at 0.

## Evidence boundaries and errata

- The vendored TiDB parser rejects `GEOMETRY` at the column-type
  position — the normalized-geometry-CREATE model input was retired in
  the contract phase; E8.3/3b now run the authored SQL end to end.
- `metadataObjectReadError` bounds every catalog-read failure to the
  public text `metadata object read failed` while `errors.Is` still
  reaches the sentinel.
- R1 archive errata (originals preserved, not rewritten): the receipt
  said "23 test functions / 36 PASS assertions" — the delivered source
  carries 36 top-level `TestT26SRSR1*` functions and PASS/FAIL counts
  are provable only by real logs; `srs-r1-tests.txt` was a count summary,
  not a verbose log; `pkg-tests.txt` was a six-package cached run, not
  the formal seven-package verbose gate; gate/build identity strings
  were reported, not captured.
- R2 gates: seven-package `-count=1 -v` (7858 PASS / 0 FAIL across the
  package set), full suite `go test -count=1 ./...` rc 0, vet clean,
  gofmt clean on cumulative changed files, `git diff --check` clean,
  `go build -buildvcs=false` with a byte-identical independent rebuild.
