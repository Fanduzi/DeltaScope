# T26-SRS-DROP-EXECUTION-01 — execution record

Contract: `T26-SRS-DROP-CONTRACT-01` (accepted at `d68f4ab7` plus the R10
evidence supplement `parser-type-subtree.txt`). This directory records the
*execution* of that contract — the contract document and its PLAN/UNEXECUTED
records are unchanged and remain the frozen expectation source.

## Identity

| Round | Candidate | Parent | Status |
|---|---|---|---|
| R1 | `3c74445cea952d657e32a31ffb5c20057abbf511` | `d68f4ab7` | submitted; R1 ruling CHANGES REQUIRED |
| R2 product | `35d41f5b52248364a7339794f58c80a0625e0277` | `3c74445c` | submitted; R2 ruling CHANGES REQUIRED |
| R2 final | `f3a70987a178a62137448dd1e3d5c1684f597a99` | `35d41f5b` | submitted; R2 ruling CHANGES REQUIRED |
| R3 product | `4a1a99fd6e01d5a14aba1622e16bb121ebd74e02` | `f3a70987` | submitted, pending ruling |
| R3 test fixup | `432af22761339a78c58adb1e4e5d8e3084509a76` | `4a1a99fd` | frozen E1.1/E5.1 finding-text assertions |

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

## R3 evidence structure

`package audit_test` in `t26_srs_drop_r3_test.go` holds the product-loader
witnesses: it injects a scripted `database/sql` stub driver into the real
`internal/infrastructure/metadata/mysql` provider (`mysqlmeta.NewProvider(db)`)
and wraps the provider in a read-counter that records, per seam,
`R` (SRS resolver forward), `Qexist`, `Qdeps`, `Qo` (loader `select version()`),
`L` (SERVER privilege loader) and `Vd` (request `LoadInstanceFacts`). Failed
calls count as attempts; `Vd` counts method invocations, not DB queries. No
copied loader logic is presented as product-loader evidence — the `R`-seam
delegates to `mysqlmeta.Provider.ResolveObject`.

## Row coverage (per-E-ID map)

Reads columns are R / Qexist / Qdeps / Qo / L (+Vd where request discovery runs).

| E-ID | Layer | SQL / fixture | Expected (arm, owed, state) | Public assertions | Reads | Witness |
|---|---|---|---|---|---|---|
| E1.1 | request/API | `DROP SRS 4326`, pin `8.0.36`; product loader ver 8.0.36 + exist row + deps `[s.t.g]` | S1 refused_dependency; findings covered; no mint | exact finding shape (`ddl.mysql.srs.drop.dependency`, blocker, dependency_witness_count); coverage complete; verdict reject | 1/1/1/1/0, Vd=1 | `TestT26SRSR3RefusedDependencyRequest` |
| E1.2 | request/API | `DROP SRS IF EXISTS 4326`, same fixture | S1 same arm; `if_exists` not a metadata key | normalized_sql keeps authored IF EXISTS; same refusal | 1/1/1/1/0, Vd=1 | `TestT26SRSR3IfExistsRequest`, `TestT26SRSR1IfExistsSameArm` |
| E2.1 | request/API | pin `8.0.36`; exist row + deps `[]` | S2 unproven; owed `[D,P]`; own apply mints `srsExistenceStale[4326]` | exactly one reserved notice; unverified; review | 1/1/1/1/0, Vd=1 | `TestT26SRSR3UnprovenReservedRequest`, `TestT26SRSR1UnprovenPresentEmptyDeps` |
| E3.1 | request/API | pin `8.0.36`, SRID `100001`; providerless | S2 unproven; owed `[E,D,P]` | zero findings (100001 ∉ reserved blocks); unverified; review | 0/0/0/0/0, Vd=N/A | `TestT26SRSR3UnprovenProviderless`, `TestT26SRSR1UnprovenProviderless` |
| E3.2 | request/API | pin `8.0.36`; exist empty + deps `[]` | S2 unproven; owed `[E,D,P]` | gap length=1, order `[E,D,P]`; exactly one reserved notice; aggregate unverified | 1/1/1/1/0, Vd=1 | `TestT26SRSR3ZeroRowProductLoader`, `TestT26SRSR2RequestZeroRow` |
| E3.3 | enrich | provider without ObjectResolver | S2 unproven; owed `[E,D,P]` | zero findings (100001 ∉ reserved); unverified | R counts only SRS forwards | `TestT26SRSR1UnprovenNoResolver` |
| E4.1 | request/API | pin `8.0.36`; exist empty + deps `[s.t.g]` | S1 refused_dependency (witness discharges existence) | dependency blocker; complete; reject | 1/1/1/1/0, Vd=1 | `TestT26SRSR3ZeroRowPositiveDepsRefuses` |
| E5.1 | request/API | `DROP SRS 0`; exist row + deps `[]` | S0 refused_srid0 | frozen finding text; complete; reject; reserved advisory suppressed | 1/1/1/1/0, Vd=1 | `TestT26SRSR3SRID0Request`, `TestT26SRSR1RefusedSRID0` |
| E5.2 | request/API | `DROP SRS IF EXISTS 0`; same fixture | S0 same arm | srid0 blocker; IF EXISTS never suppresses | 1/1/1/1/0, Vd=1 | `TestT26SRSR3SRID0Request`, `TestT26SRSR2IfExistsSRID0` |
| E5.3 | request/API | `DROP SRS 4294967295`; exist empty + deps `[]` | S2 unproven; owed `[E,D,P]` | no findings; no reserved (endpoint excluded); unverified; review | 1/1/1/1/0, Vd=1 | `TestT26SRSR3MaxSRIDRequest` |
| E5.4–E5.6h | request/API + cells | `32767/32768`, `59999999/60000000/69999999/70000000`, `1999999999/2000000000/2147483647/2147483648` | S2; owed `[E,D,P]`; reserved notice exactly inside the three blocks, absent at endpoints/corridors | exact advisory presence/absence per cell | request rows 1/1/1/1/0 | `TestT26SRSR3ReservedCellsEmptyEvidence`, `TestT26SRSR2ReservedStatementCells`, `TestT26SRSR1ReservedEndpoints` |
| E5.7–E5.10 | parser | unclaimed spellings | no SRS statement forms | parser diagnostics only | N/A | `TestT26SRSR1UnclaimedForms` |
| E5.11 | request/API | `DROP SRS 2000000000`; exist row + deps `[s.t.g]` | S1 refused_dependency (reserved range but positive witness) | advisory suppressed on refusal arms; only the dependency blocker | 1/1/1/1/0, Vd=1 | `TestT26SRSR3ReservedRefusalRequest`, `TestT26SRSR2ReservedSuppressedInRefusal` |
| E6.1–E6.3 | enrich/apply+pp | two same-SRID statements in ONE batch | stmt2 re-adjudicates per frozen rule | E6.1: stmt2 blocked [B] (bind exits pre-cache); E6.2: refusal cache-hit re-adjudicates identical; E6.3: refused twice | stmt2 0/0/0/0/0 each | `TestT26SRSR1S2MintsExistenceStale`, `S0RepeatReads`, `S1RepeatRefuses` |
| E7.1/E7.2 | enrich | batch lifecycle / different SRID | stale flags per scope | same-SRID skip, different-SRID fresh read | per-call R | `TestT26SRSR1BoundaryLifecycleMintsStale`, `TestT26SRSR2BoundaryDifferentSRIDRefuses` |
| E8.1 | enrich/apply+pp | `DROP TABLE s.t` BEFORE `DROP SRS 4326`; stmt2 `exist(row) deps([s.t.g])` | table own apply mints `srsDepsStale` + writes its legitimate table change | stmt2's positive witness is DB-pre-state → demoted → `unproven` `[D,P]` + 1 reserved; existence stays consumable | stmt2 1/1/1/1/0 | `TestT26SRSR1DropTableTouchesDeps` |
| E8.2a | apply | inert `CREATE TABLE` (no spatial col) with positive witness | no deps stale; witness retained | positive control | — | `TestT26SRSR2InertCreatePositiveControl` |
| E8.2b | apply | inert CREATE, empty evidence | deps stale absent; owed `[D,P]` kept | reserved co-fire; public gap intact | — | `TestT26SRSR2InertCreateEmptyIncomplete`, `TestT26SRSR1InertCreateKeepsWitness` |
| E8.3/E8.3b | parse→enrich | authored GEOMETRY CREATEs | real Type-position parse failures → B | blocked source = parse | — | `TestT26SRSR2GeometryTypeRefusal` |
| E8.4 | apply | compact multi-op batch | touching op mints; siblings clean | no contamination | — | `TestT26SRSR1CompactTouchesWithoutContamination` |
| E8.5 | enrich×2 | deps stale across calls | second call re-reads | per-call read deltas | R/Qexist/Qdeps deltas | `TestT26SRSR1DepsStaleCrossCall` |
| E8.6 | apply | refusal then touching write | refusal arm unchanged | no extra mint | — | `TestT26SRSR1RefusedThenTouching` |
| E8.7 | apply | `ALTER TABLE … DROP COLUMN g` | touches deps | D3 ALTER member path | — | `TestT26SRSR1AlterDropColumnTouches` |
| E8.8 | apply | inert ALTER | keeps witness | no mint | — | `TestT26SRSR1InertAlterKeepsWitness` |
| E8.9/E8.10 | apply | schema ops | per D3 table | schema-channel behavior | — | `TestT26SRSR1SchemaOps` |
| E8.11 | enrich/apply+pp (LIKE) / model-unit (CTAS) | `CREATE TABLE t_new LIKE s.t` then SRS | LIKE-source columns unvouched → completeness unproven → touching → `unproven [D,P]` + 1 reserved | CTAS covered only as classifier model input | stmt2 1/1/1/1/0 | `TestT26SRSR1CreateLikeTouches`, `TestT26SRSR2D3Discriminating` |
| D3 | classifier | HasSelect, multi-member ALTER, nil/empty source/member, spatial cols, unextracted col/member/DDL opts, LIKE/CTAS, unknown ops | table-driven expected touching | each input discriminates | — | `TestT26SRSR1D3ClassifierEdges`, `TestT26SRSR2D3Discriminating` |
| E9.1/E9.2 | parse→enrich | authored batch `bad; DROP SRS` | failure blocks; authored coords | failure at (1,1), DROP at (1,10) | — | `TestT26SRSR1ParseFailureBlocks`, `TestT26SRSR2AuthoredCoordinates` |
| E9.3 | enrich×2 | contaminated batch across calls | per-call isolation | public projection + deltas | per-call | `TestT26SRSR1TwoCallContamination` |
| B sources | enrich | distinct blocked sources | B outranks refusals; preserves state | source distinction | — | `TestT26SRSR1BlockedOutranksRefusals`, `BlockedSourceDistinction`, `BlockedPreservesState`, `TestT26SRSR2BlockedOwnApplyNoop` |
| E10.1 | request/API | MySQL no pin; observed `unknown`, loader `8.0.36` | version boundary; unsupported entry; no findings | incomplete stmt+aggregate; review | 1/1/1/1/0, Vd=1 | `TestT26SRSR3E10ObservedUnknown`, `TestT26SRSR1VersionGate` |
| E10.2 | request/API | no pin; observed `8.0.36-log`, loader `8.0.36` | S2; owed `[E,D,P]` | one reserved; unverified; review | 1/1/1/1/0, Vd=1 | `TestT26SRSR3E10HealthyObserved` |
| E10.3 | request/API | observed `5.7.0-m14` | version boundary, floor below 8.0.4 → loader declines | unsupported entry; SRS seams: existence/deps suppressed | 1/0/0/1/0, Vd=1 | `TestT26SRSR3E10Proven57` |
| E10.3b | request/API | observed `8.0.3-log` | boundary; catalog queries still issue | unsupported entry | 1/1/1/1/0, Vd=1 | `TestT26SRSR3E10Proven803` |
| E10.3c | request/API | providerless pin `8.0.3` | boundary | unsupported; review; N/A projections | 0/0/0/0/0, Vd=N/A | `TestT26SRSR3E10ProviderlessBoundary` |
| E10.3d | request/API | providerless pin `8.0.4` | S2; owed `[E,D,P]` | complete public projection (verdict/coverage/findings/gaps/unsupported/version) | 0/0/0/0/0, Vd=N/A | `TestT26SRSR3E10FloorProviderless`, `TestT26SRSR2FloorFullProjection` |
| E10.4 | request/API | providerless pin `5.7.44` | boundary | unsupported entry | 0/0/0/0/0, Vd=N/A | `TestT26SRSR3E10ProviderlessBoundary` |
| E10.5 | request/API | TiDB observed `8.0.11-TiDB-v8.5.0` | vendor boundary; resolved product TiDB | no SRS reads | 0/0/0/0/0, Vd=1 | `TestT26SRSR3E10TiDBVendorBoundary`, `TestT26SRSR2TiDBVendorBoundary` |
| E10.6 | request/API | providerless pin `8.0.x` | typed exit `ErrInvalidTargetVersion` | error; zero seams; N/A projections | 0/0/0/0/0, Vd=N/A | `TestT26SRSR3E10MalformedPin`, `TestT26SRSR2MalformedPin` |
| E10.7 | request/API | MySQL dialect + TiDB banner | `ErrDialectProductMismatch` | typed error; SRS zeros | 0/0/0/0/0, Vd=1 | `TestT26SRSR3E10ProductMismatch`, `TestT26SRSR2ProductMismatch` |
| E10.8 | request/API | pin `8.0.36`, observed `8.0.37-log` (R3) / `5.7.44` (R2) | `ErrTargetVersionMismatch` | typed error; SRS zeros | 0/0/0/0/0, Vd=1 | `TestT26SRSR3E10VersionMismatch`, `TestT26SRSR2DiscoveryMismatch` |
| E11.1 | request/API | resolver forward fails | sentinel via `metadata object read failed`; no adjudication/apply | `errors.Is` preserved; downstream seams suppressed | 1/0/0/0/0, Vd=1 | `TestT26SRSR3E11ResolverFailure` |
| E11.2 | request/API + internal | existence read fails | same wrapper; terminate before adjudicate/apply | public text exact; bind/apply observe hooks never fire; deps never issued | 1/1/0/1/0 | `TestT26SRSR3E11ExistenceFailure`, `TestT26SRSR2ErrorSeamAccounting` (E11.2 subtest) |
| E11.3 | request/API + internal | dependency read fails | same wrapper | exact text; sentinel; sibling isolation witness (post-stmt1 baseline, later indices never reach bind/apply) | 1/1/1/1/0 | `TestT26SRSR3E11DepsFailure`, `TestT26SRSR2ErrorSeamAccounting` (E11.3 subtest), `TestT26SRSR2ErrorSiblingIsolation` |
| E11.4 | request/API + internal | loader `select version()` fails | same wrapper | no adjudication (bind/apply observe hooks never fire) | 1/0/0/1/0 | `TestT26SRSR3E11LoaderVersionFailure`, `TestT26SRSR2ErrorSeamAccounting` (E11.4 subtest) |
| E11.5 | request/API | `LoadInstanceFacts` sentinel | `fmt.Errorf("load instance facts: %w")`; `report.Result{}` zero | `errors.Is` + exact text; all SRS seams zero; no enrich entry | 0/0/0/0/0, Vd=1(attempt) | `TestT26SRSR3E11DiscoverySentinel`, `TestT26SRSR2DiscoverySentinel` |
| E12.1 | request/API | raw vs normalized SQL | exact raw/normalized projection | both forms serialized | — | `TestT26SRSR3SerializedSQLForms`, `TestT26SRSR1PublicShape` |
| E12.2 | request/API | full serialized result | no catalog identities (schema/table/column names) leak | full-blob scan | — | `TestT26SRSR3SerializationScanRequest`, `TestT26SRSR2SerializationScan` |
| E12.3 | apply | S2 own apply | mints `srsExistenceStale[SRID]` only | deep snapshots at bind/apply observation points; table/schema/SERVER untouched | — | `TestT26SRSR2UnprovenOwnApplySnapshot` |
| E12.4 | apply | touching op then SRS | deps stale consumed; SRS mints own flag | complete channel snapshots; disjoint mints | — | `TestT26SRSR2TouchingApplyThenSRSMint` |
| E12.5 | request/API | `PrivilegeAuditMode` + product SERVER loader | SRS debt `[D,P]` retained; SERVER witness consumed; no raw account/grant in serialization | forbidden strings scanned | L=1 | `TestT26SRSR3ServerPrivilegeIsolationProduct`, `TestT26SRSR2ServerPrivilegeIsolation` |

All rows in scope for the R1–R3 execution work orders. Model-layer cells
whose inputs no legal SQL produces stay labeled model-unit witnesses.

## Probes (R3 binary, `-buildvcs=false`, byte-identical rebuild)

Recipe (frozen): providerless, privilege mode off, rules-only policy with
three entries `enabled: true` + original levels, no top-level `dialect`
key; `--dialect mysql --config policy-srs.yaml --format json --fail-on none`;
SQL exactly `DROP SPATIAL REFERENCE SYSTEM 100001;`.

| Probe | Expected | Actual |
|---|---|---|
| P1 (`--target-version 8.0.36`) | rc 0; review; statement+aggregate `unverified`; findings absent; one `unknown_srs_state` gap, facts `[existence, column_dependencies, drop.privilege]` | exact match — rc 0, all fields verbatim (stdout 896 B, stderr 0 B) |
| P2 (no pin) | rc 1; review; `incomplete`; findings/gaps absent; frozen version-gate unsupported entry + diagnostic; `--fail-on none` does not override the Unsupported-first exit | exact match — rc 1, `{boundary:"unaudited",gate:"version",object:"spatial_reference_system"}` + `unsupported_statement` diagnostic (stdout 1152 B, stderr 0 B) |
| P3 (`--target-version 8.0.x`) | rc 2; stdout 0 B; stderr `target_version must match [v]MAJOR.MINOR.PATCH: "8.0.x"` | exact match — rc 2, stderr verbatim 56 B, stdout 0 B |

Serialization detail: `$.statements[0].index` serializes `0` (the field
has no `omitempty`) — present and correct; the contract's omission note
governs `$.unsupported[0].index`, which stays absent at 0.

Probe environment: binary `/tmp/r3-evidence/deltascope`
SHA256 `8cf4821c3914744b3678be7140abbab1b5f44c88d03ba5ed130ad4992e5f6162`
(37,179,042 B), policy `/tmp/policy-srs.yaml`
SHA256 `ed43bc6a4b3c983d8e1378572bf7e805a15d54510dce1ad9fbe6df32da75ace2`,
probe SQL `dd9bb7a7b4bd5bb3e5a11444709d2caa85cb62ba57d21e6decb174e32601b6c3`.

## Gates (R3)

- Seven-package `go test -count=1 -v ./internal/domain/spec ./internal/application/audit ./internal/domain/rule/ddl ./internal/infrastructure/parser/tidb ./internal/domain/policy ./pkg/deltascope ./internal/infrastructure/metadata/mysql` — rc 0, 8267 PASS / 1 SKIP / 0 FAIL counted across all indentation, 7 `ok` package lines. Raw log `gate-verbose.log`.
- `go test -count=1 ./...` — rc 0. Raw log `full-suite.log`.
- `go vet` same seven packages — rc 0, 0-byte output preserved.
- `gofmt -l` over all 17 cumulative `d68f4ab7..HEAD` changed `.go` files — rc 0, 0-byte output preserved; file list recorded.
- `git diff --check d68f4ab7..HEAD` — rc 0, 0-byte output preserved.
- `CGO_ENABLED=0 go build -buildvcs=false -o /tmp/r3-evidence/deltascope ./cmd/deltascope` — rc 0; independent rebuild byte-identical (`cmp` rc 0).

## Evidence boundaries and errata

- The vendored TiDB parser rejects `GEOMETRY` at the column-type
  position — the normalized-geometry-CREATE model input was retired in
  the contract phase; E8.3/3b now run the authored SQL end to end.
- `metadataObjectReadError` bounds every catalog-read failure to the
  public text `metadata object read failed` while `errors.Is` still
  reaches the sentinel.
- `conditional_audit` is an intentional public projection key for
  SERVER conditional findings, not a leak; E12.5 forbids only raw
  account/grant strings and internal field names.
- R1 archive errata (originals preserved, not rewritten): the receipt
  said "23 test functions / 36 PASS assertions" — the delivered source
  carries 36 top-level `TestT26SRSR1*` functions and PASS/FAIL counts
  are provable only by real logs; `srs-r1-tests.txt` was a count summary,
  not a verbose log; `pkg-tests.txt` was a six-package cached run, not
  the formal seven-package verbose gate; gate/build identity strings
  were reported, not captured.
- R2 erratum: the R2 gate log's true count is 8236 PASS / 1 SKIP /
  0 FAIL across all indentation (378 deep-indented subtests were
  previously omitted from the 7858 figure); the raw log is unchanged
  and the recount is recorded as a counting correction only.
- R2 gates: seven-package `-count=1 -v` rc 0, full suite rc 0, vet clean,
  gofmt clean, `git diff --check` clean, `go build -buildvcs=false` with
  a byte-identical independent rebuild.
- R3 changed three test files plus this README and the ADR — no product `.go` source
  moved between R2 and R3, so the R3 build inputs (non-test packages)
  are identical to R2's; the build was still re-run and re-hashed on the
  R3 source state rather than inferred.
