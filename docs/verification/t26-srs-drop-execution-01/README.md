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
| R3 product | `4a1a99fd6e01d5a14aba1622e16bb121ebd74e02` | `f3a70987` | submitted; R3 ruling CHANGES REQUIRED |
| R3 test fixup | `432af22761339a78c58adb1e4e5d8e3084509a76` | `4a1a99fd` | frozen E1.1/E5.1 finding-text assertions |
| R3 final | `cce8c05b9a304886b6ba3045fca101f8b41abf57` | `432af227` | submitted; R3 ruling CHANGES REQUIRED |
| R4 product | `b90bf8eb023838364b08483eb351399ecfd26381` | `cce8c05b` | submitted; R4 ruling CHANGES REQUIRED |
| R4 final | `e0f2a3d15f6a5e97acf3bac6a47fd9714406bda5` | `b90bf8eb` | submitted; R4 ruling CHANGES REQUIRED |
| R5 product | `ec913df429f90ca008162e85420df25e09c67ebe` | `e0f2a3d1` | submitted; R5 ruling **ACCEPTED** |
| R5 final | `128d459520c17660efce37e867fa7b779a9e55e3` (tree `ebe2cd8c`) | `ec913df4` | **ACCEPTED — execution order closed** |

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
| E7.1 | enrich + public eval | `CREATE OR REPLACE SRS 4326 DEFINITION 'x'; DROP SRS 4326` — same SRID | boundary write mints `srsExistenceStale[4326]`; stmt2 `blocked`, owed `[B]` | stmt1 `UnsupportedUnauditedReason`; findings exactly 1 = reserved co-fire (`assertReservedAdvisoryText`); stale mint observed behaviorally via stmt2 blocked (bind exits pre-cache) — flag map not separately read in this test | 1/1/1/1/0 | `TestT26SRSR1BoundaryLifecycleMintsStale` |
| E7.2 | enrich + public eval | same boundary write on 4326, then `DROP SRS 100001` | per-SRID scope: 100001 unaffected | `TestT26SRSR1BoundaryDifferentSRID`: stmt2 `unproven`; `TestT26SRSR2BoundaryDifferentSRIDRefuses` (srs{100001}+deps[s.t.g@100001]): stmt2 `refused_dependency` + dependency blocker | 2/2/2/2/0 each | `TestT26SRSR1BoundaryDifferentSRID`, `TestT26SRSR2BoundaryDifferentSRIDRefuses` |
| E8.1 | enrich/apply+pp | `DROP TABLE s.t` BEFORE `DROP SRS 4326`; stmt2 `exist(row) deps([s.t.g])` | table own apply mints `srsDepsStale` + writes its legitimate table change | stmt2's positive witness is DB-pre-state → demoted → `unproven` `[D,P]` + 1 reserved; existence stays consumable | stmt2 1/1/1/1/0 | `TestT26SRSR1DropTableTouchesDeps` |
| E8.2a | enrich/apply + public eval | pre-existing `s.u` carries `g geometry` (tbl fixture) + deps witness `[s.u.g@4326]`; SQL `CREATE TABLE s.t_new (i INT); DROP SRS 4326` — inert int CREATE | no `srsDepsStale` mint; witness stays consumable | stmt2 arm `refused_dependency`, owed empty, `state.srsDepsStale==false`, exactly 1 finding dependency blocker, 0 gaps, coverage complete | 1/1/1/1/0 | `TestT26SRSR2InertCreatePositiveControl`; arm-level `TestT26SRSR1InertCreateKeepsWitness` (deps `[s.t.g]` → refused, reads 1/1/1/1/0) |
| E8.2b | enrich/apply + public eval | same inert CREATE, `srs{4326}` empty evidence (no deps) | `srsDepsStale=false` AT SRS bind (observed) | `afterBindObserve(1)` hook captures flag false; stmt2 `unproven`, owed `[D,P]`; public: 1 gap `[D,P]` owner `ddl.mysql.srs.drop.dependency`, 1 reserved notice | 1/1/1/1/0 | `TestT26SRSR2InertCreateEmptyIncomplete` |
| E8.3 | parse→enrich + public eval | authored `CREATE TABLE t_new (g GEOMETRY SRID 4326); DROP SRS 4326` | real Type-position parse failure → `blocked` | failure exactly (1,1); 1 failure + 1 extracted (DROP only); `state.contaminated`; arm blocked, owed `[B]`; findings 1 = blocked-arm reserved co-fire | 0/0/0/0/0 | `TestT26SRSR2GeometryTypeRefusal` (subcase E8.3) |
| E8.3b | parse→enrich + public eval | authored `CREATE TABLE t_new (g GEOMETRY); DROP SRS 4326` | same real failure path (no SRID clause) | identical blocked shape as E8.3 | 0/0/0/0/0 | `TestT26SRSR2GeometryTypeRefusal` (subcase E8.3b) |
| E8.4 | apply | compact multi-op batch | touching op mints; siblings clean | no contamination | — | `TestT26SRSR1CompactTouchesWithoutContamination` |
| E8.5 | enrich×2 + public eval | call1 `DROP TABLE s.t`; counters reset; call2 `DROP SRS 4326` | `srsDepsStale` persists across enrich calls on the same batchState | call1 delta `[0,0,0,0,0]` (table write binds no SRS); call2 `unproven`, owed `[D,P]`, delta `[1,1,1,1,0]` (per-call object cache re-forwards); public: 1 gap `[D,P]`, 1 reserved, unverified | call1 `[0,0,0,0,0]`, call2 `[1,1,1,1,0]` | `TestT26SRSR1DepsStaleCrossCall` |
| E8.6 | apply | refusal then touching write | refusal arm unchanged | no extra mint | — | `TestT26SRSR1RefusedThenTouching` |
| E8.7 | apply | `ALTER TABLE … DROP COLUMN g` | touches deps | D3 ALTER member path | — | `TestT26SRSR1AlterDropColumnTouches` |
| E8.8 | apply | inert ALTER | keeps witness | no mint | — | `TestT26SRSR1InertAlterKeepsWitness` |
| E8.9/E8.10 | apply | schema ops | per D3 table | schema-channel behavior | — | `TestT26SRSR1SchemaOps` |
| E8.11 | enrich/apply+pp (LIKE) / model-unit (CTAS) | `CREATE TABLE t_new LIKE s.t` then SRS | LIKE-source columns unvouched → completeness unproven → touching → `unproven [D,P]` + 1 reserved | CTAS covered only as classifier model input | stmt2 1/1/1/1/0 | `TestT26SRSR1CreateLikeTouches`, `TestT26SRSR2D3Discriminating` |
| D3 | classifier | HasSelect, multi-member ALTER, nil/empty source/member, spatial cols, unextracted col/member/DDL opts, LIKE/CTAS, unknown ops | table-driven expected touching | each input discriminates | — | `TestT26SRSR1D3ClassifierEdges`, `TestT26SRSR2D3Discriminating` |
| E9.1/E9.2 | parse→enrich | authored batch `bad; DROP SRS` | failure blocks; authored coords | failure at (1,1), DROP at (1,10) | — | `TestT26SRSR1ParseFailureBlocks`, `TestT26SRSR2AuthoredCoordinates` |
| E9.3 | enrich×2 + public eval | call1 `DROP SRS 0; GARBAGE;`; call2 `DROP SRS 4326` (same batchState) | contamination persists across calls; per-call object cache cannot launder it | call1: `refused_srid0`, delta `[1,1,1,1,0]` (S0 reads preserved), tail-sweep `contaminated`, zero stale mints, 1 srid0 finding, complete; call2: `blocked`, owed `[B]`, cumulative `[1,1,1,1,0]` (delta zero), public gap `[B]`, 1 reserved, unverified | call1 `[1,1,1,1,0]`, call2 delta `[0,0,0,0,0]` | `TestT26SRSR1TwoCallContamination` |
| B sources | enrich | distinct blocked sources | B outranks refusals; preserves state | source distinction | — | `TestT26SRSR1BlockedOutranksRefusals`, `BlockedSourceDistinction`, `BlockedPreservesState`, `TestT26SRSR2BlockedOwnApplyNoop` |
| E10.1 | request/API | MySQL no pin; observed `unknown`, loader `8.0.36` | version boundary; unsupported entry; no findings | incomplete stmt+aggregate; review | 1/1/1/1/0, Vd=1 | `TestT26SRSR3E10ObservedUnknown`, `TestT26SRSR1VersionGate` |
| E10.2 | request/API | no pin; observed `8.0.36-log`, loader `8.0.36` | S2; owed `[E,D,P]` | one reserved; unverified; review | 1/1/1/1/0, Vd=1 | `TestT26SRSR3E10HealthyObserved` |
| E10.3 | request/API | observed `5.7.0-m14` | version boundary, floor below 8.0.4 → loader declines | unsupported entry; SRS seams: existence/deps suppressed | 1/0/0/1/0, Vd=1 | `TestT26SRSR3E10Proven57` |
| E10.3b | request/API | observed `8.0.3-log` | boundary; catalog queries still issue | unsupported entry | 1/1/1/1/0, Vd=1 | `TestT26SRSR3E10Proven803` |
| E10.3c | request/API | providerless pin `8.0.3` | boundary | boundary projection OBSERVED via `r3Boundary`: `ErrUnsupportedStatement`, exactly 1 statement row, zero findings/gaps, statement+aggregate incomplete, unsupported entry `{boundary:"unaudited",gate:"version",object:"spatial_reference_system"}`, review | 0/0/0/0/0, Vd=N/A | `TestT26SRSR3E10ProviderlessBoundary` (8.0.3 subcase) |
| E10.3d | request/API | providerless pin `8.0.4` | S2; owed `[E,D,P]` | complete public projection (verdict/coverage/findings/gaps/unsupported/version) | 0/0/0/0/0, Vd=N/A | `TestT26SRSR3E10FloorProviderless`, `TestT26SRSR2FloorFullProjection` |
| E10.4 | request/API | providerless pin `5.7.44` | boundary | same observed `r3Boundary` projection as E10.3c | 0/0/0/0/0, Vd=N/A | `TestT26SRSR3E10ProviderlessBoundary` (5.7.44 subcase) |
| E10.5 | request/API | TiDB observed `8.0.11-TiDB-v8.5.0` | vendor boundary; resolved product TiDB | `len(Statements)==1` checked before fields; zero findings/gaps, incomplete; `VendorBoundaryReason`+`boundary:"vendor"`; product TiDB; no SRS reads | 0/0/0/0/0, Vd=1 | `TestT26SRSR3E10TiDBVendorBoundary`, `TestT26SRSR2TiDBVendorBoundary` |
| E10.6 | request/API | providerless pin `8.0.x` | typed exit `ErrInvalidTargetVersion` | error; zero seams; N/A projections | 0/0/0/0/0, Vd=N/A | `TestT26SRSR3E10MalformedPin`, `TestT26SRSR2MalformedPin` |
| E10.7 | request/API | MySQL dialect + TiDB banner | `ErrDialectProductMismatch` | typed error; SRS zeros | 0/0/0/0/0, Vd=1 | `TestT26SRSR3E10ProductMismatch`, `TestT26SRSR2ProductMismatch` |
| E10.8 | request/API | pin `8.0.36`, observed `8.0.37-log` (R3) / `5.7.44` (R2) | `ErrTargetVersionMismatch` | typed error; SRS zeros | 0/0/0/0/0, Vd=1 | `TestT26SRSR3E10VersionMismatch`, `TestT26SRSR2DiscoveryMismatch` |
| E11.1 | request/API | resolver forward fails | sentinel via `metadata object read failed`; no adjudication/apply | `errors.Is` preserved; downstream seams suppressed | 1/0/0/0/0, Vd=1 | `TestT26SRSR3E11ResolverFailure` |
| E11.2 | request/API + internal | existence read fails | same wrapper; terminate before adjudicate/apply | public text exact; bind/apply observe hooks never fire; deps never issued | 1/1/0/1/0 | `TestT26SRSR3E11ExistenceFailure`, `TestT26SRSR2ErrorSeamAccounting` (E11.2 subtest) |
| E11.3 | request/API + internal | dependency read fails | same wrapper | exact text; sentinel; sibling isolation witness: `TestT26SRSR2ErrorSiblingIsolation` fires `afterBindObserve`/`afterApplyObserve` observers AND a post-stmt1 `srsChannels` deep snapshot — post-error state value-identical to post-stmt1 baseline; indices ≥1 never reach bind/apply; `srsExistenceStale` stays empty | 1/1/1/1/0 | `TestT26SRSR3E11DepsFailure`, `TestT26SRSR2ErrorSeamAccounting` (E11.3 subtest), `TestT26SRSR2ErrorSiblingIsolation` |
| E11.4 | request/API + internal | loader `select version()` fails | same wrapper | no adjudication (bind/apply observe hooks never fire) | 1/0/0/1/0 | `TestT26SRSR3E11LoaderVersionFailure`, `TestT26SRSR2ErrorSeamAccounting` (E11.4 subtest) |
| E11.5 | request/API | `LoadInstanceFacts` sentinel | `fmt.Errorf("load instance facts: %w")`; `report.Result{}` zero | `errors.Is` + exact text; all SRS seams zero; no enrich entry | 0/0/0/0/0, Vd=1(attempt) | `TestT26SRSR3E11DiscoverySentinel`, `TestT26SRSR2DiscoverySentinel` |
| E12.1 | request/API | raw vs normalized SQL | exact raw/normalized projection | both forms serialized | — | `TestT26SRSR3SerializedSQLForms`, `TestT26SRSR1PublicShape` |
| E12.2 | request/API | full serialized result | no catalog identities (schema/table/column names) leak | full-blob scan | — | `TestT26SRSR3SerializationScanRequest`, `TestT26SRSR2SerializationScan` |
| E12.3 | apply + internal observers | unproven S2 own apply; pre-seeded unrelated `srsExistenceStale[100001]`; frozen initial snapshot `s.t(g geometry)` + SERVER `s1` OrderedPresent | SRS own apply mints `srsExistenceStale[4326]` ONLY, union-preserved | `afterBindObserve`/`afterApplyObserve` deep snapshots: bind read-only; post-apply stale set exactly `{100001,4326}`; `srsDepsStale` false; table/schema + SERVER channels unchanged | — | `TestT26SRSR2UnprovenOwnApplySnapshot` |
| E12.4 | apply + internal observers | pre-seeded `srsExistenceStale[100001]` + deps witness `[s.t.g@4326]`; stmt1 `DROP TABLE s.t` then `DROP SRS 4326` | table apply mints `srsDepsStale` (witness demotes SRS to unproven `[D,P]`); SRS apply unions own key | stmt1: `srsDepsStale` true, existence-stale + SERVER unchanged, legit table write; stmt2 SRS apply: table/SERVER/deps channels preserved, stale set exactly `{100001,4326}` | — | `TestT26SRSR2TouchingApplyThenSRSMint` |
| E12.5 | request/API + internal | `PrivilegeAuditModeObservedAccountNewSession`; catalog `s1` + observed account `app@%` + direct-global SUPER grant via product `mysqlmeta` loader (CURRENT_USER + SHOW GRANTS scripted) | positive SERVER consumability + SRS isolation | SERVER: ONE `ddl.mysql.server.drop.notice` finding carries `outcome=conditional_drop` AND `conditional_audit="observed_account_new_session"` (string type-asserted — missing/nil/non-string/empty/wrong-token all fail); no `server.drop_privilege` owed. SRS statement: `r3S2Statement` — 1 gap owner `ddl.mysql.srs.drop.dependency`, ordered `[D,P]`, findings exactly 1 = reserved notice; marker scope: `conditional_audit` is a legitimate projection on the SERVER finding, forbidden everywhere on the SRS statement (full-blob scan); whole-result scan forbids raw account/grant text | R/Qexist/Qdeps/Qo/L/Vd = 1/1/1/1/1/1 | `TestT26SRSR3ServerPrivilegeIsolationProduct` (request layer), `TestT26SRSR2ServerPrivilegeIsolation` (internal layer) |

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

## Gates (R5 — tests/docs-only revision)

Run after the R5 test changes; raw logs + immediate rc files archived.

- `go test -count=1 -v ./internal/application/audit` — rc 0; 4529 PASS / 0 FAIL / 0 SKIP counted across all indentation (raw `audit-verbose.log`).
- `go vet ./internal/application/audit` — rc 0.
- `gofmt -l` over cumulative `d68f4ab7..HEAD` changed `.go` files — rc 0; file list preserved.
- `git diff --check d68f4ab7..HEAD` — rc 0.
- REUSED (unchanged compilation inputs — R5 moved `*_test.go` + docs only): R3 seven-package/full-suite/vet/build/rebuild bound to `cce8c05b`; registered binary SHA `8cf4821c3914744b3678be7140abbab1b5f44c88d03ba5ed130ad4992e5f6162` (37,179,042 B); R4 P1/P2/P3 raw streams/policy/SQL/invocation record. Not re-run this round per the R5 order.

## Gates (R4 — tests/docs-only revision)

- `go test -count=1 -v ./internal/application/audit` — rc 0; 4529 PASS / 0 FAIL (audit package, all indentation). Raw log in the R4 archive.
- `go vet ./internal/application/audit` — rc 0, 0-byte output.
- `gofmt -l` over the 17 cumulative `d68f4ab7..HEAD` changed `.go` files — rc 0, 0-byte.
- `git diff --check d68f4ab7..HEAD` — rc 0, 0-byte.
- R3 seven-package + full-suite logs are REUSED evidence bound to `cce8c05b` — R4 moved only `*_test.go` and docs, so no new seven-package/full-suite/build run was performed or claimed. The registered binary (SHA `8cf4821c…f6162`) remains valid for the R4 source state.

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
- R4 errata (originals preserved, not rewritten):
  - the R4 receipt described the E12.5 SRS debt as `[E,D,P]` — the actual
    asserted debt is `[D,P]`: the fixture carries a positive existence
    row so `srs.existence` is discharged and only
    `[srs.column_dependencies, srs.drop.privilege]` stays owed. Literal
    documentation erratum; the test fixture was correct and unchanged
    (R5-A.4 instruction).
  - the R4 receipt's E12.5 wording implied the `nil != ""` check was a
    valid positive witness — it was not (a missing key passes). Fixed in
    R5-A by asserting the full three-field conditional projection on the
    same `ddl.mysql.server.drop.notice` finding.
  - E10.3c's row previously claimed "N/A projections" — the boundary
    statement row IS observed (`r3Boundary` asserts row count, zero
    findings/gaps, incomplete coverage, frozen unsupported entry,
    verdict); corrected in the table.
  - E8.2b previously mapped `TestT26SRSR1InertCreateKeepsWitness` — that
    test uses a positive deps witness (refused arm) and is an arm-level
    E8.2a-shape witness; remapped. E8.2b's sole witness is
    `TestT26SRSR2InertCreateEmptyIncomplete`.
- R5 changed test files plus this README — no product `.go`, dependency,
  policy, registry, corpus/golden, or frozen contract semantics moved.
