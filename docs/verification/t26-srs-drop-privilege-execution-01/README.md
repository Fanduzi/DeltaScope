# T26-SRS-DROP-PRIVILEGE-EXECUTION-01 — execution receipt (R1)

Status: **awaiting R1 ruling**. Contract
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
| receipt | (this file's commit — chain-final docs closeout) | — | `8738402f` | this receipt; its commit/tree hash cannot self-embed and is registered in the delivery manifest and message |

Final candidate HEAD: the receipt commit itself (docs-only child of
`8738402f`) — its full commit/tree/parent triple is registered verbatim
in the archive manifest and the delivery message, since a file cannot
carry its own commit's hash. Branch:
`milestone/mysql-tidb-ddl-completion`. Cumulative diff under review:
`2f7231b59744d6bbafb7150ffe32891002a2f6ab..FINAL_HEAD`.

Cumulative diffstat: 19 files, +2034/−63 (incl. this receipt).
Categories: product
(`srs_drop.go`, `srs_drop_premise.go`, `metadata.go`,
`mysql_srs_rules.go`, `common.go`, `register.go`, `defaults_ddl.go`),
config (`deltascope.example.yaml`), tests (r1/r2/r3 updates + two new
matrix files + corpus deferral), docs (three module READMEs, decision
record). `.go` files changed: 12.

## Gate/build/probe source identity

Every gate below ran at product/test source commit `8738402f` on a
clean working tree (`git status --porcelain` non-untracked = 0). Build,
rebuild, and the three CLI probes used the binary built from
`8738402f`. Between that gate source commit and final HEAD
the only delta is this receipt itself — a docs-only file under
`docs/verification/` that touches no product, test, build, or config
input, so the gate results bind unchanged.

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
| P2 | request/API | default fixture | S2 consumable `[D]`; 1 notice/1 gap; unverified/review; own apply mints | exact match — serialized statement + seam counts preserved (`p2-public-result.json`, `p2-seam-counts.json`) | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP2Consumable` |
| P3 | request/API | `srs=nil` (zero rows) | S2 consumable `[E,D]`; 1 notice/1 gap | exact match | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP3ConsumableZeroRowExistence` |
| P4a | request/API | `GRANT SELECT ON *.*` (complete, parsed) | unproven `[D,P]` | exact match — privilege queries 1/1 issued | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP4aGlobalSelectNoSuper` |
| P4b | request/API | `current_user()` zero rows (account `""`), grants still return | unproven `[D,P]` | exact match — unbound account blocks the witness | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP4bUnboundAccount` |
| P4c | request/API | `GRANT \`r1\`@\`%\` TO \`app\`@\`%\`` (role-assignment shape, no ON) | unproven `[D,P]` | exact match | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP4cRoleAssignmentLine` |
| P4d | request/API | `GRANT ALL PRIVILEGES ON s.*` (legal non-global) | unproven `[D,P]` | exact match | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP4dNonGlobalAll` |
| P4e | request/API | `GRANT ALL PRIVILEGES ON *.*` | consumable `[D]`; 1 notice | exact match — registered static-set implication | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP4eGlobalAllImpliesSuper` |
| P4f | request/API | `current_user` QueryContext sentinel | bounded error; user/grants attempts 1/0; no bind/adjudicate/apply; no normal P gap | exact match — `err.Error()` exactly `server drop privilege observation failed`, `errors.Is` reaches sentinel | 0/0/0/0/1, Vd=1 | `TestT26SRSPrivP4fCurrentUserFailure` |
| P4g | request/API | `SHOW GRANTS` QueryContext sentinel | same; attempts 1/1 | exact match | 0/0/0/0/1, Vd=1 | `TestT26SRSPrivP4gGrantsFailure` |
| P4h | request/API | `GRANT SUPERUSER ON *.* TO app@%` — complete, parsed, exact-token negative | unproven `[D,P]`; not a malformed line | exact match — the request/API-layer substring-matching discriminator (registered erratum honoured) | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP4hSuperuserExactToken` |
| P5a | request/API | `GARBAGE;` prefix → real parse failure | B `[B]`; 0 findings/1 gap; premise consumable unconsumed | exact match — request returns `errParserUnsupported` + the audited DROP's B projection + parser diagnostic | 0/0/0/0/1, Vd=1 | `TestT26SRSPrivP5aParseFailureBlocked` |
| P5b | request/API + internal | `DROP SRS 100001` twice, same batch | stmt1 S2 consumable `[D]` + notice; stmt2 B `[B]`, premise consumable unconsumed; scope open; stmt1 snapshot stable | exact match | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP5bSameSRIDTwice`, `TestT26SRSPrivP5bInternalSnapshotStable` |
| P5c/P5d | request/API | SRID 0 plain + IF EXISTS | S0; premise consumable unconsumed; 0 gaps; srid0 blocker; complete/reject; no mint | exact match | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP5cdSRID0` |
| P5e | request/API | deps `[s.t.g@100001]` | S1; consumable unconsumed; dependency blocker `dependency_witness_count=1`; complete/reject; no mint | exact match | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP5eDependencyRefusal` |
| P6a | internal + request shapes | `DROP TABLE s.t; DROP SRS 100001` (tblTFixture: `s.t` with `g:geometry`) | stmt1 apply mints `srsDepsStale`, contamination false; positive deps demoted → S2 consumable `[D]`; 1 notice/1 gap; stmt2 mints existence stale | exact match — `afterApplyObserve` captures depsStale=true at stmt1's own apply; the demotion lands S2, never S1 | 1/1/1/1/1, Vd=1 (internal layer has no discovery — Vd observed at the request-layer twin rows) | `TestT26SRSPrivP6aInternalStaleDemotion` |
| P6a-bis | request/API | `CREATE TABLE s.x (g GEOMETRY); DROP SRS 100001` — real type-position parse rejection | B `[B]`; 0 findings/1 gap; consumable unconsumed; no mint | exact match | 0/0/0/0/1, Vd=1 | `TestT26SRSPrivP6aBisGeometryParseBlocked` |
| P6b | internal | two `enrichStatements` calls on one `batchState`, `DROP SRS 100001` each | call1 S2 `[D]` mints existence stale; call2 B `[B]`; delta 0/0/0/0/1 — loader reruns, SRS reads do not; call1 snapshot untouched; scope open | exact match — `readsVector` delta `[0,0,0,0,1]`; separately named `evaluateEnriched` per call | call1 1/1/1/1/1; call2 delta 0/0/0/0/1; Vd=N/A (internal layer runs no discovery) | `TestT26SRSPrivP6bInternalDoubleEnrich` |
| P7 | request/API + internal | `DROP SRS 100001; REVOKE SELECT ON *.* FROM 'r1'@'%'; DROP SRS 100002` | stmt1 S2 consumable `[D]` + notice; stmt3 suppressed `[D,P]`, 0 findings; REVOKE ends scope, mints no SRS stale; each DROP mints its SRID; stmt1 snapshot stable; minimal unit `witness=false+ended=true` → unproven | exact match — request returns `ErrUnsupportedStatement` (REVOKE is unsupported) carrying all three projections; `serverPrivilegeScopeEnded` true after stmt2 | 2/2/2/2/1, Vd=1 | `TestT26SRSPrivP7ScopeTerminator`, `TestT26SRSPrivP7InternalSuppression`, `TestT26SRSPrivPremiseTruthTable` |
| P8a/P8b | request/API + internal | `DROP SERVER s1; DROP SRS 100001` and reversed | one shared observation; SERVER finding carries `outcome=conditional_drop` + `conditional_audit=observed_account_new_session` on the SAME finding; SRS independently consumable S2 `[D]`; marker never leaks across | exact match — SERVER seam 1/1/0 (every `version()` belongs to the SRS catalog) | SRS 1/1/1/1; SERVER 1/1/0; L=1; Vd=1 | `TestT26SRSPrivP8aServerThenSRS`, `TestT26SRSPrivP8bSRSThenServer`, `TestT26SRSPrivP8InternalSiblingIndependence` |
| P9a | request/API | pin/observed/loader `8.0.4` (floor) | S2 consumable `[D]`; 1 notice | exact match | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP9aFloor` |
| P9b | request/API + internal | mysql, no pin, observed/loader `8.0.3-log` | version boundary; 0 findings/0 gaps; incomplete/review; boundary apply mints `srsExistenceStale[100001]` only | exact match — `r3Boundary` projection + internal mint check | 1/1/1/1/0, Vd=1 | `TestT26SRSPrivP9bBelowFloorBoundary`, `TestT26SRSPrivP9bInternalBoundaryMint` |
| P9c | request/API | tidb, no pin, observed `8.0.11-TiDB-v8.5.0` | vendor boundary; resolved product TiDB; 0 findings/0 gaps; incomplete/review; all SRS reads zero; loader-internal version N/A | exact match — `ErrUnsupportedStatement`, `boundary=vendor_boundary` | 0/0/0/0/0, Vd=1 | `TestT26SRSPrivP9cTiDBVendorBoundary` |
| P9d | request/API | `PrivilegeAuditMode="bogus_mode"` | `ErrInvalidPrivilegeMode`; all zero; Vd=0 | exact match | 0/0/0/0/0, Vd=0 | `TestT26SRSPrivP9dInvalidMode` |
| P9e | request/API | `--target-version`-equivalent pin `8.0.x` | `ErrInvalidTargetVersion`; all zero; Vd=0 | exact match | 0/0/0/0/0, Vd=0 | `TestT26SRSPrivP9eMalformedPin` |
| P9f | request/API | mysql dialect + TiDB banner discovery | `ErrDialectProductMismatch`; all zero; Vd=1 | exact match | 0/0/0/0/0, Vd=1 | `TestT26SRSPrivP9fProductMismatch` |
| P9g | request/API | pin `8.0.36`, parsed observed `8.0.37-log` | `ErrTargetVersionMismatch`; all zero; Vd=1 | exact match — mismatch returns before `resolvedVersion` binds | 0/0/0/0/0, Vd=1 | `TestT26SRSPrivP9gVersionMismatch` |
| P10a | request/API | SRID `4326` consumable | exactly 2 findings (privilege + reserved), 1 gap `[D]` | exact match | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP10aReservedConsumed` |
| P10b | request/API | SRID `4326` + global SELECT-only grant | exactly 1 reserved finding, 1 gap `[D,P]`; no privilege marker anywhere | exact match — serialized statement scanned for `conditional_audit` | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP10bReservedUnproven` |
| P10c | request/API | `ddl.mysql.srs.drop.privilege` disabled via `--config`-equivalent policy | 0 findings; premise still consumable; gap `[D]`; loader/apply unchanged | exact match — the flag suppresses only the finding | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP10cRuleDisabled` |
| P10d | request/API | level `warning` rewrite | same text/metadata; review | exact match — JSON marker re-verified | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP10dWarningRewrite` |
| P10e | request/API | level `blocker` rewrite | same premise/Owed; reject; statement coverage stays unverified | exact match | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP10eBlockerRewrite` |
| P10f | request/API | mode via legacy `Request.Metadata.PrivilegeAuditMode` only | same as P2 | exact match | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP10fLegacyMetadataMode` |
| P10g | request/API | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001` | same premise/arm/Owed/notice as P2; authored form preserved | exact match — `normalized_sql` keeps `IF EXISTS` | 1/1/1/1/1, Vd=1 | `TestT26SRSPrivP10gIfExists` |

Companion/minimal units beyond the matrix:
`TestT26SRSPrivPremiseTruthTable` (all four premise cells),
`TestT26SRSPrivP2InternalOwnApplyStale` (consuming arm's own-apply
lifecycle). Every unavailable/boundary/error row asserts its own N/A
scope — N/A never means zero output: arm/premise/result-projection N/A
on typed exits; reads/aggregate observation N/A only where the layer
cannot produce them (P1a Vd, P6b Vd).

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

## Gates (product/test inputs bound to `8738402f`; HEAD adds only this receipt)

- Seven-package verbose:
  `go test -count=1 -v ./internal/domain/spec ./internal/application/audit ./internal/domain/rule/ddl ./internal/infrastructure/parser/tidb ./internal/domain/policy ./internal/infrastructure/metadata/mysql ./internal/interfaces/cli`
  — rc 0; 16,181 test lines counted; 0 FAIL; 7 `ok` package lines.
  Raw log `gate-7pkg.log`.
- Full suite `go test -count=1 ./...` — rc 0, 0 FAIL lines.
  Raw log `gate-full.log`.
- `go vet ./...` — rc 0. `gate-vet.log`.
- `gofmt -l` over the 12 changed `.go` files — clean. Pre-existing
  unformatted `internal/infrastructure/parser/tidb/mysql_t26r5_test.go`
  is NOT touched by this chain (tracked, unchanged; recorded as a
  repo-wide observation only).
- `git diff --check 2f7231b5..8738402f` — rc 0. `gate-diffcheck.log`.
- Build: `go build -o deltascope ./cmd/deltascope` → 37,180,178 B,
  SHA-256 `53735b9a7fbb856c830f6d2034e75ba50147e9b9fdd8418ae32be3f64df8970f`.
  Independent rebuild byte-identical (`cmp` clean).

## CLI probes (binary `53735b9a`, providerless, mode-off)

cwd `/Users/fan/GolangProjects/DeltaScope`; evidence dir
`artifacts/t26-srs-priv-exec-r1/`; policy `policy-srs-privilege.yaml`
enables all four SRS rules explicitly; no top-level dialect key; no
privilege CLI flags exist or were assumed.

| probe | command tail | rc | stdout | stderr | expected | actual |
|---|---|---|---|---|---|---|
| CLI-1 | `--target-version 8.0.36` | 0 | 896 B `59f784e0` | 0 B | review; unverified; 0 findings; 1 `unknown_srs_state` gap facts `[E,D,P]`; no privilege marker | exact match |
| CLI-2 | (no pin) | 1 | 1152 B `4b2e45f5` | 0 B | Unsupported-first; `{boundary:"unaudited",gate:"version",object:"spatial_reference_system"}` + `unsupported_statement` diagnostic; `--fail-on none` does not override | exact match |
| CLI-3 | `--target-version 8.0.x` | 2 | 0 B | 56 B `38fc9412` | stderr exactly `target_version must match [v]MAJOR.MINOR.PATCH: "8.0.x"`\n | exact match |

True zero-byte files (`cli1.stderr`, `cli2.stderr`, `cli3.stdout`) are
preserved as real originals — SHA-256 `e3b0c442…` (the empty hash).

## Executed / not executed

- Executed: every P1–P10 row at its contract layer; all seven-package,
  full-suite, vet, gofmt, diff-check gates; build + byte-identical
  rebuild; the three CLI probes; P2 serialized public capture.
- Not executed: Docker/native metadata e2e (not part of this work
  order); push/merge/tag/release/issue close (not authorised).

## Errata and attribution carried forward (review-level records from the
accepted contract ruling — no repository revision required)

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
