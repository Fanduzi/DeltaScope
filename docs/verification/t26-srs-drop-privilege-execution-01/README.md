# T26-SRS-DROP-PRIVILEGE-EXECUTION-01 — execution receipt (R2)

Status: **awaiting R2 ruling**. R1 ruling was CHANGES REQUIRED —
evidence/assertion/provenance deficiencies only; the accepted contract
and product semantics were left unchanged. R2 closes the ordered
deficiencies: a real P6a request/API twin, corrected P9f/P9b fixtures,
internal own-apply and stale-set witnesses, strict public-output
assertions, real counter serialization, the prescribed
`CGO_ENABLED=0 -buildvcs=false` build recipe, fresh probe captures, and
receipt/provenance corrections. Contract
`docs/verification/t26-srs-drop-privilege-contract-01` was ACCEPTED at
`2f7231b59744d6bbafb7150ffe32891002a2f6ab` (R3); this directory is the
implementation's execution evidence — a separate phase that begins from
that accepted contract commit. Issue #105 / milestone T26 stays open; no
push/merge/tag/release/issue close is implied or authorised.

## Identity chain

| link | commit | tree | parent | scope |
|---|---|---|---|---|
| contract (accepted) | `2f7231b59744d6bbafb7150ffe32891002a2f6ab` | `104f7bde7f4a8433ee55c3deceecb1d9fe4b97f0` | `1c48d4ec9795ae5c3b2fcfa616055700cf19b7e9` | docs-only contract R3 |
| impl | `e0f36f62591da60b5cb3883b5dad57f1ee80e944` | `a7f06ef0ebc57a4473a451e0254d074567d8c6d5` | `2f7231b5` | spec premise tri-state + `AdjudicateSRSDrop` premise arg; `srs_drop_premise.go`; `metadata.go` loader gate + attach; `ddl.mysql.srs.drop.privilege` rule + registration + default policy + example YAML; corpus deferral; regression assertion updates only where the approved increment changes them |
| impl | `924e3ac873f0051df839d901412d45fb2ddcf130` | `b73fee47d8c5417050ee97c017c8f85e76026052` | `e0f36f62` | P1–P10 matrix: `t26_srs_priv_exec_test.go` (external, real provider chain) + `t26_srs_priv_exec_int_test.go` (internal batchState companions) |
| impl docs | `8738402f6a30f6df69ac373d48db6bdf74c850d7` | `f3166487cf1497c67f328919f17e15e4a27a1701` | `924e3ac8` | module README sync (spec/rule/audit) + decision record `2026-11-12-mysql-drop-srs-conditional-privilege.md` |
| receipt R1 | `2d4ee54c8f545d33fa0fd3e134c4cdb31e209d27` | `c671a5fd217e32d88a597b64cc02419e31b33346` | `8738402f` | R1 receipt (superseded wording preserved in the r1-to-r2 diff) |
| receipt R1 | `14795fb46effb04b17d0d30a8cafa2f5061ec79b` | `62c874b5c644b37e4d629bac8299b7012b64e2fd` | `2d4ee54c` | R1 identity-chain closeout |
| receipt R1 | `5c09d04f44f20418f1134a9b460aedbab19312fa` | `181447ed7e9ebb819abead81eee5afd5ed08c7ee` | `14795fb4` | R1 final HEAD — the R1 candidate, unchanged by R2 |
| R2 tests | `0183db7e6abbb7e83eb8f9011b52383d7eebb78e` | `a49328e36a4b40cab837d5ef4fc5bbc251733a0d` | `5c09d04f` | R2-A..C evidence closure: P6a request/API twin + `r3TableFixtureProvider` seam; P9f explicit no-pin; P9c public boundary/product asserts; P9b aligned `8.0.3-log` fixture; P5 non-consuming-arm internal table (arm/premise/Owed + exact stale key sets); P6b call-2 `[B]` Owed + stale-set equality + named evaluations; P7 in-position REVOKE observation + exact per-SRID final set; P8b reversed-order companion; P9c counterexample row; strict `r3ExactGap`/`r3OnlyFinding`/`r3Aggregate` public assertions; P2 real-counter serialization |
| receipt R2 | (this file's commit — chain-final docs closeout) | — | `0183db7e` | this receipt; its commit/tree hash cannot self-embed and is registered in the delivery manifest and message |

The full six-commit R1 chain is now registered verbatim:
`e0f36f62 → 924e3ac8 → 8738402f → 2d4ee54c → 14795fb4 → 5c09d04f` —
the R1 receipt's "direct child of 8738402f" phrasing collapsed the two
docs commits and is corrected here.

Final candidate HEAD: the R2 receipt commit itself (docs-only child of
`0183db7e`) — its full commit/tree/parent triple is registered verbatim
in the archive manifest and the delivery message, since a file cannot
carry its own commit's hash. Branch:
`milestone/mysql-tidb-ddl-completion`. Cumulative diff under review:
`2f7231b59744d6bbafb7150ffe32891002a2f6ab..FINAL_HEAD`.

Cumulative diffstat (corrected): **19 files, +2536/−63, 13 `.go`
files** — the R1 receipt's `+2034/−63, 12 Go` was a transcription error;
the true R1 cumulative was `+2039/−63, 13 Go`, and R2 adds `+559/−62`
(net +497) in the two execution test files only. Categories: product
(`srs_drop.go`, `srs_drop_premise.go`, `metadata.go`,
`mysql_srs_rules.go`, `common.go`, `register.go`, `defaults_ddl.go`),
config (`deltascope.example.yaml`), tests (r1/r2/r3 updates + two
matrix files + corpus deferral), docs (three module READMEs, decision
record, this receipt).

## Gate/build/probe source identity

Every gate below ran at test source commit `0183db7e` on a clean
tracked tree. Build, rebuild, and the three CLI probes used the binary
built from `0183db7e` via the prescribed recipe. Between that gate
source commit and final HEAD the only delta is this receipt itself — a
docs-only file under `docs/verification/` that touches no product,
test, build, or config input, so the gate results bind unchanged. The
R1 seven-package and full-suite logs were bound to `8738402f`; they are
preserved as R1 originals and superseded by the R2 captures below — R2
test-source changes make the audit package's R1 log non-authoritative
for the added tests.

## P1–P10 row map

Layers: `request/API` = real `AuditSQL` through `mysqlmeta.NewProvider`
over the scripted `database/sql` driver (file
`t26_srs_priv_exec_test.go`); `internal` = real `batchState` through
`enrichStatements`/`evaluateEnriched` with `afterApplyObserve` snapshots
(`t26_srs_priv_exec_int_test.go`). Counts follow the frozen order
R/Qexist/Qdeps/Qo/L with Vd separate; SERVER counts its own
forward/mysql.servers/internal-version seam.

| row | layer | input (fixture deltas from default) | frozen expected | actual | reads (SRS / SERVER / L / Vd) | test |
|---|---|---|---|---|---|---|
| P1a | request/API | providerless, mode on | S2 unproven `[E,D,P]`; 0 findings/1 gap | exact match | 0/0/0/0/0, Vd=N/A | `TestT26SRSPrivP1aProviderless` |
| P1b | request/API | mode off (no `PrivilegeAuditMode`) | S2 `[D,P]` (E read → discharged); 0 findings | exact match | 1/1/1/1/0, Vd=1 | `TestT26SRSPrivP1bModeOff` |
| P1c | request/API | provider lacks loader capability (`r3NoPrivilegeLoader`) | S2 `[D,P]`; no loader error | exact match — request returned normally | 1/1/1/1/0, Vd=1 | `TestT26SRSPrivP1cMissingLoaderCapability` |
| P2 | request/API | default fixture | S2 consumable `[D]`; 1 notice/1 gap; unverified/review; own apply mints | exact match — serialized statement + REAL measured seam counters preserved (`p2-public-result.json`, `p2-seam-counts.json`; the R1 counts file was the expected vector — corrected to live counters in R2-C6) | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP2Consumable` |
| P3 | request/API | `srs=nil` (zero rows) | S2 consumable `[E,D]`; 1 notice/1 gap | exact match | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP3ConsumableZeroRowExistence` |
| P4a | request/API | `GRANT SELECT ON *.*` (complete, parsed) | unproven `[D,P]` | exact match — privilege queries 1/1 issued | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP4aGlobalSelectNoSuper` |
| P4b | request/API | `current_user()` zero rows (account `""`), grants still return | unproven `[D,P]` | exact match — unbound account blocks the witness | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP4bUnboundAccount` |
| P4c | request/API | `GRANT \`r1\`@\`%\` TO \`app\`@\`%\`` (role-assignment shape, no ON) | unproven `[D,P]` | exact match | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP4cRoleAssignmentLine` |
| P4d | request/API | `GRANT ALL PRIVILEGES ON s.*` (legal non-global) | unproven `[D,P]` | exact match | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP4dNonGlobalAll` |
| P4e | request/API | `GRANT ALL PRIVILEGES ON *.*` | consumable `[D]`; 1 notice | exact match — registered static-set implication | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP4eGlobalAllImpliesSuper` |
| P4f | request/API | `current_user` QueryContext sentinel | bounded error; user/grants attempts 1/0; no bind/adjudicate/apply; no normal P gap | exact match — `err.Error()` exactly `server drop privilege observation failed`, `errors.Is` reaches sentinel | 0/0/0/0/1, Vd=1 | `TestT26SRSPrivP4fCurrentUserFailure` |
| P4g | request/API | `SHOW GRANTS` QueryContext sentinel | same; attempts 1/1 | exact match | 0/0/0/0/1, Vd=1 | `TestT26SRSPrivP4gGrantsFailure` |
| P4h | request/API | `GRANT SUPERUSER ON *.* TO app@%` — complete, parsed, exact-token negative | unproven `[D,P]`; not a malformed line | exact match — the request/API-layer substring-matching discriminator (registered erratum honoured) | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP4hSuperuserExactToken` |
| P5a | request/API + internal | `GARBAGE;` prefix → real parse failure | B `[B]`; 0 findings/1 dependency-owned gap; premise consumable unconsumed; unverified coverage | exact match — request returns `errParserUnsupported` + the audited DROP's B projection + parser diagnostic; internal arm table pins arm/premise/Owed and proves own-apply mints no stale of either kind (prefix contamination is never attributed to the DROP) | 0/0/0/0/1, Vd=1 | `TestT26SRSPrivP5aParseFailureBlocked`, `TestT26SRSPrivP5ArmsInternalNonConsumption` |
| P5b | request/API + internal | `DROP SRS 100001` twice, same batch | stmt1 S2 consumable `[D]` + notice; stmt2 B `[B]` Owed=`[srs.batch_effects]`, premise consumable unconsumed; exact stale set `{100001}` across stmt2; scope open; stmt1 snapshot stable; whole-request review | exact match | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP5bSameSRIDTwice`, `TestT26SRSPrivP5bInternalSnapshotStable` |
| P5c/P5d | request/API | SRID 0 plain + real `IF EXISTS` | S0; premise consumable unconsumed; 0 gaps; exactly one srid0 blocker — fixed message + metadata exactly `{srid:0,outcome:"refused_srid0"}` (int srid), no `conditional_audit`, no reserved advisory; complete/reject; no mint | exact match | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP5cdSRID0` |
| P5e | request/API | deps `[s.t.g@100001]` | S1; consumable unconsumed; exactly one dependency blocker — fixed message + metadata exactly `{srid:100001,outcome:"refused_dependency",dependency_witness_count:1}`, no `conditional_audit`; complete/reject; no mint | exact match | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP5eDependencyRefusal` |
| P6a | request/API + internal | `DROP TABLE s.t; DROP SRS 100001` (accepted table fixture via test-only `LoadTableSnapshot` seam — `s.t` Exists=true, `g:geometry`; real discovery/privilege/SRS chain) | stmt1 apply mints `srsDepsStale`, contamination false; positive deps demoted → S2 consumable `[D]`; 1 notice/1 dependency-owned gap; stmt2 mints existence stale; coverage unverified | exact match — `r3TableFixtureProvider` delegates every other method to the real `mysqlmeta.NewProvider` chain; `afterApplyObserve` captures depsStale=true at stmt1's own apply; the demotion lands S2, never S1 | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP6aRequestStaleDemotion` (request/API twin — R2-A1), `TestT26SRSPrivP6aInternalStaleDemotion` |
| P6a-bis | request/API | `CREATE TABLE s.x (g GEOMETRY); DROP SRS 100001` — real type-position parse rejection | B `[B]`; 0 findings/1 gap; consumable unconsumed; no mint | exact match | 0/0/0/0/1, Vd=1 | `TestT26SRSPrivP6aBisGeometryParseBlocked` |
| P6b | internal | two `enrichStatements` calls on one `batchState`, `DROP SRS 100001` each | call1 S2 consumable `[D]` mints existence stale `{100001}`; call2 B `[B]` Owed=`[srs.batch_effects]`; delta 0/0/0/0/1 — loader reruns, SRS reads do not; post-call2 stale set equals call1 post-state exactly, depsStale unchanged; call1 snapshot untouched; scope open; named evals: call1 1 notice + 1 `[D]` gap, call2 0 findings + 1 `[B]` gap | exact match — `readsVector` delta `[0,0,0,0,1]`; exact stale key sets compared, separately named `evaluateEnriched` per call | call1 1/1/1/1/1; call2 delta 0/0/0/0/1; Vd=N/A (internal layer runs no discovery) | `TestT26SRSPrivP6bInternalDoubleEnrich` |
| P7 | request/API + internal | `DROP SRS 100001; REVOKE SELECT ON *.* FROM 'r1'@'%'; DROP SRS 100002` | stmt1 S2 consumable `[D]` + notice; stmt3 suppressed `[D,P]`, 0 findings; REVOKE ends scope, mints no SRS stale; each DROP mints its SRID; stmt1 snapshot stable; minimal unit `witness=false+ended=true` → unproven | exact match — request returns `ErrUnsupportedStatement` (REVOKE is unsupported) carrying all three projections; in-position observation after stmt2: `serverPrivilegeScopeEnded=true`, stale set exactly stmt1's post-state `{100001}`, depsStale unchanged by REVOKE; final set exactly `{100001,100002}` | 2/2/2/2/1, Vd=1 | `TestT26SRSPrivP7ScopeTerminator`, `TestT26SRSPrivP7InternalSuppression`, `TestT26SRSPrivPremiseTruthTable` |
| P8a/P8b | request/API + internal | `DROP SERVER s1; DROP SRS 100001` and reversed | one shared observation; SERVER finding carries `outcome=conditional_drop` + `conditional_audit=observed_account_new_session` on the SAME finding; SRS independently consumable S2 `[D]`; marker never leaks across | exact match — SERVER seam 1/1/0 (every `version()` belongs to the SRS catalog) | SRS 1/1/1/1; SERVER 1/1/0; L=1; Vd=1 | `TestT26SRSPrivP8aServerThenSRS`, `TestT26SRSPrivP8bSRSThenServer`, `TestT26SRSPrivP8InternalSiblingIndependence`, `TestT26SRSPrivP8bInternalSiblingReversed` — each family publishes its own post-state (SERVER OrderedAbsent; SRS S2 `[D]` + its own stale) in BOTH orders |
| P9a | request/API | pin/observed/loader `8.0.4` (floor) | S2 consumable `[D]`; 1 notice | exact match | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP9aFloor` |
| P9b | request/API + internal | mysql, no pin, observed+loader banner `8.0.3-log`, resolved `8.0.3` | version boundary; 0 findings/0 gaps; incomplete/review; boundary apply mints `srsExistenceStale[100001]` only | exact match — `r3Boundary` projection + internal mint check; R2-A4 aligned the internal fixture banner with the resolved version (the 8.0.36-loader/8.0.3-resolved pairing is a historical variant only) | 1/1/1/1/0, Vd=1 | `TestT26SRSPrivP9bBelowFloorBoundary`, `TestT26SRSPrivP9bInternalBoundaryMint` |
| P9c | request/API + internal | tidb, no pin, observed `8.0.11-TiDB-v8.5.0` | vendor boundary; resolved product TiDB; `boundary="vendor"` metadata + `UnsupportedVendorBoundaryReason` asserted separately; 0 findings/0 gaps; incomplete/review; all SRS reads zero; loader-internal version N/A | exact match — `ErrUnsupportedStatement`; **internal companion registers a bounded counterexample (see errata)**: the real TiDB vendor-boundary statement carries `Unsupported=nil` (the boundary is aspect-level), so own-apply mints NOTHING — no `srsExistenceStale`, no `srsDepsStale` | 0/0/0/0/0, Vd=1 | `TestT26SRSPrivP9cTiDBVendorBoundary`, `TestT26SRSPrivP9cInternalVendorBoundaryMint` |
| P9d | request/API | `PrivilegeAuditMode="bogus_mode"` | `ErrInvalidPrivilegeMode`; all zero; Vd=0 | exact match | 0/0/0/0/0, Vd=0 | `TestT26SRSPrivP9dInvalidMode` |
| P9e | request/API | `--target-version`-equivalent pin `8.0.x` | `ErrInvalidTargetVersion`; all zero; Vd=0 | exact match | 0/0/0/0/0, Vd=0 | `TestT26SRSPrivP9eMalformedPin` |
| P9f | request/API | mysql dialect + TiDB banner `8.0.11-TiDB-v8.5.0` + explicit no-pin `Request` (R2-A2 — the shared helper's 8.0.36 pin would falsify the frozen input) | `ErrDialectProductMismatch`; all zero; Vd=1 | exact match | 0/0/0/0/0, Vd=1 | `TestT26SRSPrivP9fProductMismatch` |
| P9g | request/API | pin `8.0.36`, parsed observed `8.0.37-log` | `ErrTargetVersionMismatch`; all zero; Vd=1 | exact match — mismatch returns before `resolvedVersion` binds | 0/0/0/0/0, Vd=1 | `TestT26SRSPrivP9gVersionMismatch` |
| P10a | request/API | SRID `4326` consumable | exactly 2 findings (privilege + reserved), 1 gap `[D]`; reserved advisory carries exactly `{srid}` — no `conditional_audit`/`outcome` on its carrier | exact match | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP10aReservedConsumed` |
| P10b | request/API | SRID `4326` + global SELECT-only grant | exactly 1 reserved finding, 1 gap `[D,P]`; reserved metadata exactly `{srid}`; no privilege marker anywhere | exact match — serialized statement scanned for `conditional_audit` | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP10bReservedUnproven` |
| P10c | request/API | `ddl.mysql.srs.drop.privilege` disabled via `--config`-equivalent policy | 0 findings; premise still consumable; exact `[D]` gap; statement+aggregate unverified; review; loader/apply unchanged | exact match — the flag suppresses only the finding, never upgrades coverage | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP10cRuleDisabled` |
| P10d | request/API | level `warning` rewrite | same text/metadata; exact `[D]` gap; statement+aggregate unverified; review — the level rewrite never re-owes P | exact match — JSON marker re-verified | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP10dWarningRewrite` |
| P10e | request/API | level `blocker` rewrite | same premise/Owed; reject; exact `[D]` gap; statement+aggregate coverage unverified — a blocker verdict never implies verified coverage | exact match | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP10eBlockerRewrite` |
| P10f | request/API | mode via legacy `Request.Metadata.PrivilegeAuditMode` only | same as P2 | exact match | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP10fLegacyMetadataMode` |
| P10g | request/API | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001` | same premise/arm/Owed/notice as P2; authored form preserved | exact match — `normalized_sql` keeps `IF EXISTS` | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP10gIfExists` |

Companion/minimal units beyond the matrix:
`TestT26SRSPrivPremiseTruthTable` (all four premise cells),
`TestT26SRSPrivP2InternalOwnApplyStale` (consuming arm's own-apply
lifecycle),
`TestT26SRSPrivP5ArmsInternalNonConsumption` (R2-B1 — the five
non-consuming arms' attached arm/premise/Owed tuples plus exact stale
key sets and depsStale before/after their own apply; prefix
contamination is never attributed to the DROP's apply),
`TestT26SRSPrivP8bInternalSiblingReversed` (P8 in the opposite
order), `TestT26SRSPrivP9cInternalVendorBoundaryMint` (the R2-B5
counterexample row). Every unavailable/boundary/error row asserts its
own N/A scope — N/A never means zero output: arm/premise/result-
projection N/A on typed exits; reads/aggregate observation N/A only
where the layer cannot produce them (P1a Vd, P6b Vd).

## Regression updates made under this increment (all registered)

- `TestT26SRSR2ServerPrivilegeIsolation` +
  `TestT26SRSR3ServerPrivilegeIsolationProduct`: SRS projection
  `[D,P]` → `[D]` with the family's own `ddl.mysql.srs.drop.privilege`
  notice — the approved behaviour change (shared positive witness now
  feeds the independent SRS premise).
- `corpus_coverage_test.go`: `ddl.mysql.srs.drop.privilege` added to the
  deferred list — the offline corpus cannot supply a provider-backed
  privilege observation; the request/API matrix is the honest carrier.
- `t26_srs_drop_r1_test.go` spec-cell callers: `AdjudicateSRSDrop` gained
  the premise argument; cells pin `SRSDropPrivilegeUnproven` to keep
  their frozen expectations.
- r3 driver `user` seam: `account==""` now returns a zero-row answer —
  the P4b unbound-account input (previously unreachable).
- No frozen row was deleted; no fixture was substituted to make a row
  pass.

## Gates (test inputs bound to `0183db7e`; HEAD adds only this receipt)

- `go test -count=1 -v ./internal/application/audit` — rc 0;
  4,581 PASS, 0 FAIL, 0 SKIP; 1 `ok` package line.
  Raw log `gate-audit-verbose.log`.
- `go test -count=1 -v ./pkg/deltascope` — rc 0;
  580 PASS, 0 FAIL, 0 SKIP; 1 `ok` package line.
  Raw log `gate-deltascope-verbose.log`.
- `go vet ./internal/application/audit ./pkg/deltascope` — rc 0.
  Raw log `gate-vet-gofmt-diffcheck.log`.
- `gofmt -l` over the 13 cumulative changed `.go` files (real file list
  `gate-gofmt-filelist.txt`) — clean. Pre-existing unformatted
  `internal/infrastructure/parser/tidb/mysql_t26r5_test.go` is NOT
  touched by this chain (tracked, unchanged; repo-wide observation
  only). Raw log `gate-gofmt.log`.
- `git diff --check 2f7231b5..0183db7e` — rc 0.
  `gate-vet-gofmt-diffcheck.log`.
- R1 originals preserved as historical evidence: seven-package verbose
  (8,279 PASS / 1 SKIP / 0 FAIL / 7 `ok` at `8738402f`), full suite
  (39 `ok` / 5 no-test) — classified REUSED/report originals; any R1
  command whose immediate rc was not captured stays labelled
  `reported/uncaptured`.

## Build + rebuild (R2-D recipe)

Source: commit `0183db7e6abbb7e83eb8f9011b52383d7eebb78e`, tree
`a49328e36a4b40cab837d5ef4fc5bbc251733a0d`, cwd
`/Users/fan/GolangProjects/DeltaScope`, tracked tree clean.
Toolchain `go1.26.1 darwin/arm64`; `GOOS=darwin GOARCH=arm64
CGO_ENABLED=1 GOFLAGS=` (build command overrides CGO to 0); tags none;
ldflags none. `go.mod` sha256
`80dd91053db571bf7e14af57d806e254726afa3a16bbd0998486c4356b628c98`;
`go.sum` sha256
`cc5fa030a1c92992d2003ff2fd64efa1a247cd5f3679e5e3debae21c55000696`.

- Build: `CGO_ENABLED=0 go build -buildvcs=false -o
  /tmp/t26-priv-r2/build/deltascope-r2 ./cmd/deltascope` — rc 0;
  37,180,178 B, SHA-256
  `87cf741a1174259a367a3819256e0365117719ff10ee09776dfba544a0732f4a`.
- Rebuild (independent output path, same cwd/toolchain/flags):
  `CGO_ENABLED=0 go build -buildvcs=false -o
  /tmp/t26-priv-r2/build/deltascope-r2-rebuild ./cmd/deltascope` —
  rc 0; identical size and hash.
- `cmp deltascope-r2 deltascope-r2-rebuild` — no output, rc 0.
- No cross-directory stability is claimed — same cwd, same source.
  The R1 binary (`53735b9a`) carried `CGO_ENABLED=1` and embedded VCS
  data with `vcs.modified=true`; the R2 recipe replaces it.

## CLI probes (binary `87cf741a`, providerless, mode-off)

cwd `/tmp/t26-priv-r2`; evidence captured to `cliN.stdout` /
`cliN.stderr` / `cliN.rc` (six output streams + three rc files);
policy `probe-policy.yaml` enables all four SRS rules at their
required levels; no top-level dialect key; providerless (no connection
flags); privilege mode off (no privilege flags); `--dialect mysql`,
`--format json`, `--fail-on none`; SQL
`DROP SPATIAL REFERENCE SYSTEM 100001;`. Full invocation record
`probe-commands.txt` (real commands — the R1 file's unexpanded
variables are preserved as a HISTORICAL artifact; a real R1
invocation transcript was never captured).

| probe | command tail | rc | stdout | stderr | expected | actual |
|---|---|---|---|---|---|---|
| CLI-1 | `--target-version 8.0.36` | 0 | 896 B | 0 B | review; unverified; 0 findings; 1 `unknown_srs_state` gap ordered facts `[srs.existence,srs.column_dependencies,srs.drop.privilege]` | exact match |
| CLI-2 | (no pin) | 1 | 1152 B | 0 B | review; incomplete; 0 findings/0 gaps; `{boundary:"unaudited",gate:"version",object:"spatial_reference_system"}` + `unsupported_statement` diagnostic; `--fail-on none` does not override | exact match |
| CLI-3 | `--target-version 8.0.x` | 2 | 0 B | 56 B | stderr exactly `target_version must match [v]MAJOR.MINOR.PATCH: "8.0.x"`\n | exact match |

## Executed / not executed

- Executed: every P1–P10 row at its contract layer incl. the new
  P6a request/API twin; the R2 verbose gates for both changed-test
  package and `pkg/deltascope`; vet, gofmt (real file list),
  diff-check; the prescribed build + byte-identical rebuild; the three
  CLI probes on the new binary; P2 real-counter + serialized public
  capture.
- Not executed: Docker/native metadata e2e (not part of this work
  order); push/merge/tag/release/issue close (not authorised);
  the R1 seven-package/full-suite gates were NOT re-run (only tests
  changed — R1 logs preserved as source-bound originals).

## Loader qualification (verbatim)

`statementsNeedMySQLServerPrivilege(statements) ||
statementsNeedMySQLSRSPrivilege(statements)` — the R1 receipt's
wording shortened this condition; the OR of both family predicates is
the actual loader gate (`metadata.go`).

## Errata and attribution

### R2-B5 counterexample (raised for ruling — no product change made)

The R2 work order asked to prove the real TiDB vendor-boundary
statement's boundary own-apply mints `srsExistenceStale[100001]`. The
real product chain contradicts the expectation:

- Under TiDB the vendor boundary never sets `statement.Unsupported` —
  the boundary is a computed coverage aspect (`coverage_boundary.go`
  emits `boundary:"vendor"` details to the report's unsupported list);
  `statement.Unsupported` is assigned only for version-gated unaudited
  rows (`metadata.go:274-287`).
- The accepted contract's mint claim is conditional: `batch_state.go`'s
  boundary branch requires `Unsupported != nil`. With the marker absent
  under TiDB, dispatch reaches `applySRSStatement`, which mints only on
  an attached S2 adjudication — and no adjudication attaches under TiDB
  (the bind block is `dialect == spec.DialectMySQL`-gated,
  `metadata.go:702`).
- `TestT26SRSPrivP9cInternalVendorBoundaryMint` pins the actual
  behaviour: `srsExistenceStale` empty, `srsDepsStale` false, no
  contamination, `Unsupported=nil`, no adjudication — a boundary row,
  not an audited SRS row.

Whether the expectation should be amended (aspect-level boundary → no
mint is coherent: no same-batch MySQL-dialect read can follow under
TiDB) or the product should gain a TiDB-boundary mint is a ruling
question — R2 made no semantic change.

### R1 receipt corrections registered

- Cumulative diffstat was `19 files, +2039/−63, 13 .go` — the R1
  receipt's `+2034/−63, 12 Go` was wrong; R2's cumulative is
  `+2536/−63` in the same 19 files.
- The R1 candidate is a six-commit chain ending at `5c09d04f`; it was
  described as a direct child of `8738402f`.
- The R1 binary used `go build` without `-buildvcs=false` and with
  ambient `CGO_ENABLED=1`, embedding `vcs.modified=true` provenance —
  replaced by the R2 recipe binary `87cf741a`.
- R1's `probe-commands.txt` contained unexpanded variables — preserved
  unchanged as HISTORICAL; a real R1 invocation transcript does not
  exist.
- R1's seven-package gate substituted `internal/interfaces/cli` for the
  required `pkg/deltascope` — the R2 gate runs the correct packages.
- P2's `p2-seam-counts.json` serialized the expected vector; R2-C6
  serializes the live measured counters.

### Carried forward (accepted contract ruling — no repository revision required)

- The P4h request/API exact-token row is the real SUPER-prefix
  discriminator; earlier set/scope/negative tests alone are not.
- README §6.1-style "exit before loop" language applies only to typed
  validation/discovery errors; boundary statements still flow through
  enrichment/apply and produce a public boundary report.
- P9g's `8.0.37` is a *parsed observed* version — the mismatch returns
  before `resolvedVersion` assignment.
- Historical contract R1 `rc=1` stays `reported/uncaptured`; no cause is
  reconstructed from current state.

## Distinction reminder

The contract documents (`t26-srs-drop-privilege-contract-01`) are
ACCEPTED artefacts — their matrix rows stay PLAN/UNEXECUTED by design.
This directory is the separate implementation phase now awaiting its own
ruling.
