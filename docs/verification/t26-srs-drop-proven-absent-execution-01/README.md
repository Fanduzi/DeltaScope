# T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01 — R3 test + evidence hardening

Round: **R3 — TEST/DOCS/EVIDENCE-CHAIN REVISION** against the accepted
contract at `0fb1b7ff` + evidence supplement `f206060d`
(`docs/verification/t26-srs-drop-proven-absent-contract-01`, R4 ACCEPTED;
issue #105, milestone T26/#105). R2 (`1291b934` impl + `f3d09134` docs)
returned CHANGES REQUIRED on test depth, source-binding, mapping-table
accuracy, and ledger form only — no product-path deviation was found.
This round changed tests, test-only exports, comments, and docs; **no
executable product behavior was modified**. No push, merge, tag,
release, or issue closure is claimed or authorized by this receipt.

Attribution discipline: **implementation and test work happened in C3**
(`0354540b`, sequential on F2); the C3 commit/tree identity, its
two-package gates, the diffs, and this README were **recorded in F3**
(this commit's archive). R1/R2-captured logs are historical — nothing
was re-run to rewrite them. C2 evidence (seven-package + full-suite
gates, builds, probes) is **reused per R3-F4** because this round's diff
is test/doc-only against product code — the reuse ledger names the
source commit and member identities; dates and sources are not
re-labelled.

## Identity

- Implementation candidate (C3): `0354540be4eb4cb2c9b3551e5933cfc11e45c33d`
- C3 tree: `f1f93d89fedb9cc7bdf50a60286c24409a554f89`
- C3 parent: `f3d09134` (F2 docs — sequential commits per R3 ruling)
- Branch: `milestone/mysql-tidb-ddl-completion`
- cwd `/Users/fan/GolangProjects/DeltaScope`; toolchain `go1.26.1
  darwin/arm64`

## R3 scope executed (C3; `.go` = tests + test-only export + comments)

- **R3-A** — `TestT26SRSAbsenceMidIterationCancelPropagates` now syncs
  the cancel to the rowset's REAL `Next()` entry: `testRows.enteredNext`
  is closed on the first `Next()` call, the cancel lands *inside*
  iteration, and the test asserts `(Qo,Qexist,Qdeps)=(1,1,0)`,
  `errors.Is(err, context.Canceled)`, and a nil snapshot — within a
  bounded wait, not a sleep.
- **R3-B** — `paSnapStmt` preserves nil-vs-non-nil-empty on every slice
  member (`paCloneSRIDs`/`paCloneDepCols`/`paCloneStrings` return a nil
  result for a nil input); `paSRSConflictSame` is the pure comparator
  (`t.Fatalf` runs `runtime.Goexit` — it cannot be probed through
  `recover`), wrapped by `paSRSConflictEqual` for diagnostics;
  `TestT26SRSAbsentSnapshotFidelity` proves nil-vs-empty distinction,
  element-change detection, and bidirectional mutation isolation on
  `MatchedSRIDs`/`Columns`/`Owed`.
- **R3-C1** — `TestT26SRSAbsentStaleChannelsRealChain` PA-5b/PA-5e rows
  assert their own read vectors (single full pass + L=1 on the first
  enrichment of each enrich window).
- **R3-C2** — `TestT26SRSAbsentRevokeScopingRealChain` captures
  per-statement read increments at the apply hook: stmt1
  `[1,1,1,1,1]` (its own pass + the ONE-TIME premise load inside its
  enrichment window), REVOKE `[0,0,0,0,0]`, stmt3 `[1,1,1,1,0]`; L=1
  is measured across the whole enrich and attributed to the
  enrich-level observation — NOT re-described as an independent
  per-statement catalog read.
- **R3-C3** — internal companions for PA-6c/PA-6e in
  `t26_srs_absent_realchain_test.go`:
  - `TestT26SRSAbsentBoundaryPrefixInternal` — the CREATE OR REPLACE
    prefix carries the implementation boundary and NO SRSDrop
    adjudication; its own-apply mints authored stale `{100001}` with
    no contamination; the tail DROP is audited B short-circuited by
    the same-SRID stale flag; prefix increment measured at its hook
    `[1,1,1,1,1]`, tail increment `[0,0,0,0,0]` measured — never
    inferred from totals.
  - `TestT26SRSAbsentBoundaryVersionPairInternal` — both statements
    are version boundaries (`unaudited`/`gate=version`/
    `object=spatial_reference_system`), NEITHER carries an SRSDrop
    adjudication; stmt1's boundary apply mints `{100001}`; stmt1's
    increment `[1,1,1,1,0]` and stmt2's `[0,0,0,0,0]` are measured at
    their own hooks; **L=0 is attributed to the admittance gate** —
    L was already 0 at stmt1's hook, before any mint existed — not to
    the later stale short-circuit. `SeedResolvedVersion` (new test
    export) pins the resolved out-of-domain identity.
- **R3-D1/D2** — the archive's
  `bindings/r3-helper-and-json-bindings.txt` carries candidate-bound
  (commit `0354540b` + real line numbers) verbatim excerpts for the
  full minimum set: `r3Fixture`/`r3Classify`/`r3Conn.QueryContext`/
  `r3ProductProvider`/`r3Harness`/`r3AssertReads`, `r3PrivilegeFixture`/
  `r3PrivilegeRequest`/`r3Aggregate`/`r3OnlyFinding`, `r3S2Consumed`/
  `r3RefusedDependency`, `evaluateEnriched`, `assertSRSPrivilegeNotice`,
  `intExactGap`, `newSRSState`, `enrichSRSPriv`, `paClone*`/
  `paSnapStmt`/`paSRSConflictSame`/`paSnapEqual`, `paRealState`/
  `paReadsVector`, the stub `closeErr`/`blockOnCtx`/`enteredNext`
  members + `Next`/`Close`/`recordTestQueryCall`, the named reused
  PA-4a/4d/8a/8c/8d bodies, and the pre-existing scan/dependency
  iteration-error bodies; plus the report→public canonical binding
  (`report.StatementResult`/`report.Result`/`rule.Finding`/
  `rule.EvidenceGap`/`deltascope.StatementResult`/`deltascope.Result`
  JSON tags + `fromDomainResult`'s field-by-field copy — the frozen
  keys survive conversion; `TestT26SRSAbsentCanonicalJSON` runs at
  the internal request layer, which this binding now labels).
- **R3-D3** — the case-matrix mapping below is corrected row-by-row
  (see errata for each fixed contradiction).
- **R3-E1** — `internal/infrastructure/metadata/mysql/README.md`
  narrows `sql.ErrNoRows` to the existence-read path; the Qo
  `select version()` read keeps its existing ErrNoRows →
  unknown-support behavior.
- **R3-E2** — `provider_test.go`'s `closeErr` comment now records the
  actual propagation (`database/sql` surfaces it through `Rows.Err()`
  / the EOF auto-close path), matching the corrected explanation.
- **R3-E3** — PA-4e2's premise message corrected: S2 `owed=[E,D]` —
  P IS consumed there; the text now says the observation projects
  onto S2 without blocking the arm.
- **R3-E4** — all diffstats below are real intervals recomputed from
  `git diff --shortstat`; no C2 number is re-used as an F3 cumulative.
- **R3-E5** — test-function accounting re-registered at C3: **47
  PA-dedicated test functions** — 6 internal (`package audit`), **6
  real-chain** (`package audit_test`; +`SnapshotFidelity` +2 boundary
  companions this round), 22 external API, 13 provider — plus reused
  R2/R3 functions named in the mapping. Functions ≠ `--- PASS` lines
  ≠ packages.

## Gates at the immutable C3 `0354540b` (invocation + immediate rc)

- `go test -count=1 -v ./internal/application/audit
  ./internal/infrastructure/metadata/mysql` — rc 0; **5,007 PASS /
  0 FAIL / 0 SKIP** counted across all indentation; 2 `ok` packages.
  Raw: `gates/gate-verbose.*`.
- `go vet` same two packages — rc 0, 0-byte stdout/stderr.
  `gates/vet.*`.
- `gofmt -l` over the 21 cumulative changed `.go` files
  (`gates/gofmt-filelist.txt`) — rc 0, 0-byte output. The pre-existing
  unformatted `internal/infrastructure/parser/tidb/mysql_t26r5_test.go`
  is NOT touched by this chain (tracked, unchanged at baseline;
  registered erratum, out of scope).
- `git diff --check 0fb1b7ff..HEAD` — rc 0, 0-byte.
  `gates/diff-check.*`.

Every gate file is `*.cmd.txt` + `*.stdout.log` + `*.stderr.log` +
`*.rc.txt`; each rc was captured by `rc=$?` on the line immediately
after its own command — no pipeline masking.

## Reused C2 evidence (R3-F4 — source commit + member identity; no re-run)

This round changed no product `.go` file; product bytes and all build
inputs (`go.mod`/`go.sum`/toolchain) are unchanged between C2 `1291b934`
and C3 `0354540b` (C2→C3 = 8 files, all tests/test-exports/comments/docs:
`+630/−211`). The following C2 members are therefore reused as evidence
of the frozen behavior — each is delivered in this archive with its
source-commit label intact:

- `historical-c2/gates/*` — C2's seven-package verbose gate (8,378 PASS/
  1 SKIP/0 FAIL over 7 `ok` packages) and `go test -count=1 ./...`
  full-suite (39 `ok`, 5 no-test-files, 0 failures), vet, gofmt,
  diff-check — bound to C2, not re-labelled.
- `historical-c2/build/deltascope-b1` (+ `build2` cmp rc 0) — C2's
  deterministic build: SHA-256
  `39aaaefb680597b4f39121da2d6ab0d2812d9a98f2a26e9a3332678a69af7728`,
  52,761,890 B, `CGO_ENABLED=1 -tags postgresql -buildvcs=false`,
  `go.mod` `80dd9105…`, `go.sum` `cc5fa030…`.
- `historical-c2/probes/*` — C2 binary b1's three providerless CLI
  probes: rc 0/1/2, stdout 896/1152/0 B, stderr 0/0/56 B, byte-exact
  frozen shapes (transcript + policy `327a3e53…`).

## Providerless CLI probes (reused C2 capture; contract PA-10g reference)

| probe | tail | rc | stdout | stderr | expected | actual |
|---|---|---|---|---|---|---|
| CLI-1 | `--target-version 8.0.36` | 0 | 896 B | 0 B | review; unverified; 0 findings; 1 `unknown_srs_state` gap `[srs.existence,srs.column_dependencies,srs.drop.privilege]` | exact match (C2 capture) |
| CLI-2 | (no pin) | 1 | 1152 B | 0 B | boundary `{unaudited,gate:"version",object:"spatial_reference_system"}` + `unsupported_statement` diagnostic | exact match (C2 capture) |
| CLI-3 | `--target-version 8.0.x` | 2 | 0 B | 56 B | stderr exactly `target_version must match [v]MAJOR.MINOR.PATCH: "8.0.x"`\n | exact match (C2 capture) |

## Case-matrix mapping (PA rows → tests; corrected per R3-D3)

Layer: **P** = provider (`package mysqlmeta`), **I** = internal
(`package audit`), **R** = real-chain (`package audit_test` over
`mysqlmeta.NewProvider`), **X** = external API (`AuditSQL`).
"measured" = an explicit assert call in the body; "unobserved" = the
body asserts no read vector (honest label — not a claim the channel
doesn't exist). Vector order: R, Qexist, Qdeps, Qo, L, Vd.

| PA | function / subcase | layer | fixture difference | asserted fields | measured vector |
|---|---|---|---|---|---|
| 1a | `TestT26SRSAbsentProviderlessStaysUnproven` | X | no MetadataProvider | S2, `[E,D,P]` gap, **0 findings** (no notice — nothing discharges an unproven premise); unverified | all-0 by construction — no provider attached; not a counter measurement |
| 1b/2a | `TestT26SRSAbsentPlainRefused` | X | zero-row cell, Qo 8.0.36 | absent blocker, exact msg/meta, 0 gaps, complete, **reject** | 1,1,1,1,**1**,1 measured |
| 1c/8b | `TestT26SRSAbsentUnprovenQoStaysUnproven` | X | Qo `unparseable` | S2 `[E,D]` + notice | 1,1,1,1,**1**,1 measured |
| 2b | `TestT26SRSAbsentIfExistsNoop` | X | grants SUPER | absent_noop+marker, 0 gaps, complete/pass | 1,1,1,1,1,1 measured |
| 2c | `TestT26SRSAbsentIfExistsConditional` | X | grants SELECT only | `[P]` `unproven_privilege` gap, 0 findings, unverified | **unobserved** (no read assert in body) |
| 2d | `TestT26SRSAbsentIfExistsSuppressed` | X | **REVOKE + ONE DROP** (2-statement batch) | `ErrUnsupportedStatement` + stmt2 `[P]` gap, 0 findings | **unobserved** (no read assert in body) |
| 3a | `TestT26SRSAbsentExistsStaysUnproven` | X | positive row | S2 `[D]` + notice | 1,1,1,1,1,1 measured |
| 4a | `TestT26SRSR3ZeroRowPositiveDepsRefuses` (reused) | X | zero-row + deps row | S1 refused_dependency | R3-frozen body (binding excerpt `bindings/`) |
| 4b | `TestT26SRSAbsentIfExistsDependencyRefused` | X | deps row + IF EXISTS | S1 refused | 1,1,1,1,**1**,1 measured |
| 4c | `TestT26SRSAbsentDepsStaleDemotesWitness` | X | prefix-minted depsStale | demotion → absent blocker, 0 gaps | **unobserved** (no read assert in body) |
| 4d | `TestT26SRSR3SRID0Request` (reused) | X | SRID 0 | S0 blocker | R3-frozen body (`bindings/`) |
| 4e/4f/4f2 | `TestT26SRSAbsentAdjSupplyDiscriminants`, `TestT26SRSAbsentAdjIfExistsSplit` | I | state-literal table | arm + owed per discriminant; S1-outranks-absence | reads N/A — adjudication literals, no provider attached |
| 4e2 | `TestT26SRSAbsentEnrichForeignSRID` | I | custom resolver: authored 100001, state SRID 100002 | S2; premise=consumable; owed `[E,D]`; `L=1`; authored minted, foreign never; resolver ×1; strict notice + `[E,D]` gap | SQL seams N/A — the custom resolver issues no catalog SQL; L counted on the provider stub |
| 4g | `TestT26SRSAbsenceQueryErrNoRowsPropagates` | P | `queryErr=sql.ErrNoRows` | `errors.Is`, nil snapshot | (1,1,0) measured via `t26AssertSeams` |
| 4h | `TestT26SRSAbsenceIterationErrNoRowsPropagates` | P | `rowErrAt:0,rowErr=ErrNoRows` | `errors.Is`, nil snapshot | (1,1,0) measured |
| 4i | `TestT26SRSAbsenceEOFFterPositiveRow` | P | positive row then EOF | present; Absence=false | success row — vector not asserted |
| 5a–f | `TestT26SRSAbsentStaleChannels` (I) + `TestT26SRSAbsentStaleChannelsRealChain` (R) | I+R | pre-minted stale + depsStale | both stale channels per A arm; same-SRID B, `GARBAGE` B, S2-union controls | I: no counters exist; R: PA-5b/5e per-row vectors measured at entry |
| 6a/6b | `TestT26SRSAbsentSameSRIDCacheHit`, `TestT26SRSAbsentDifferentSRIDRereads` | X | 2-statement same/diff SRID | cache hit 1,1,1,1,1,1 / re-read 2,2,2,2,1,1 | measured per seam |
| 6c | `TestT26SRSAbsentBoundaryPrefixBlocks` (X) + `…PrefixInternal` (R) | X+R | CREATE OR REPLACE prefix | stmt1 implementation boundary + no adjudication; tail audited B; stale `{100001}` no contamination | X total 1,1,1,1,1,1; R increments: prefix `[1,1,1,1,1]`, tail `[0,0,0,0,0]` (Vd N/A at R) |
| 6d | `TestT26SRSAbsentRecachesPerEnrich` + `…RealChain` | I+R | two enrich passes | per-enrich re-read | per-enrich counts measured |
| 6e | `TestT26SRSAbsentBoundaryVersionPair` (X) + `…VersionPairInternal` (R) | X+R | Qo 8.0.3, no pin | both version boundaries exact `{unaudited,gate:version,object:spatial_reference_system}`; neither adjudicates; minted `{100001}` | X total 1,1,1,1,0,1 + forwards=1; R increments: stmt1 `[1,1,1,1,0]`, stmt2 `[0,0,0,0,0]`; L=0 attributed to admittance (already 0 before the mint); Vd N/A at R |
| 7a | `TestT26SRSAbsentRevokeScoping` (I) + `…RealChain` (R) | I+R | consumable→REVOKE→suppressed | SAME stmt1 at 3 hook points; complete SRS+adj+premise; nil/empty distinct; bidirectional isolation | R: total 2,2,2,2,1 + per-statement Δ `[1,1,1,1,1]`/`[0,0,0,0,0]`/`[1,1,1,1,0]`; **Vd=N/A — no discovery runs inside enrich**; L=1 enrich-level |
| 7b | `TestT26SRSAbsentRevokeScopingPublic` | X | same batch | stmt1 noop+marker; stmt3 `[P]` gap and nothing else | 2,2,2,2,1,1 measured |
| 8a | `TestT26SRSR3E10Proven803` (reused) | X | Qo 8.0.3 no pin | single-statement version boundary | R3-frozen body (`bindings/`) |
| 8c | `TestT26SRSR2DiscoveryMismatch` (reused) | X | pin/observed mismatch | `ErrTargetVersionMismatch` | R2-frozen body (`bindings/`) |
| 8d | `TestT26SRSPrivP9cInternalVendorBoundaryMint` (reused) | I | real TiDB parse | vendor boundary mints `{100001}` | R3-frozen body (`bindings/`) |
| 9a | `TestT26SRSAbsenceQueryErrorPropagates` | P | generic `queryErr` | `errors.Is`, nil snapshot | (1,1,0) measured |
| 9b | `TestT26SRSAbsenceIterationErrorPropagates` | P | `rowErrAt:0` | `errors.Is`, nil snapshot | (1,1,0) measured |
| 9c | `TestT26SRSAbsenceMidIterationCancelPropagates` | P | `blockOnCtx` rowset + `enteredNext` handshake | cancel DURING real `Next()` iteration; `errors.Is(context.Canceled)`; nil snapshot | Qo=1,Qexist=1,Qdeps=0 measured |
| 9c-ctl | `TestT26SRSAbsencePreCancelledRequestPropagates` | P | pre-cancelled ctx | request-control only — NOT the 9c witness | renamed row; vector not asserted |
| 9c-close | `TestT26SRSAbsenceCloseErrorPropagates` | P | `closeErr` sentinel | error reaches caller via `Rows.Err()`/EOF path | (1,1,0) measured |
| 9d | `TestT26SRSAbsentNoResolverStaysUnproven` | X | capability minus ObjectResolver | S2 `[E,D]` + notice | R/Qexist/Qdeps/Qo not counted (no seam counters on the custom provider); **L channel EXISTS — `paNoResolverProvider` implements the loader — but is not counted** |
| 9e | `TestT26SRSAbsenceNotSuppliedUnprovenVersion` | P | Qo `unparseable` | state literal `{Read,!Exists,!Absence}` | (1,1,1) measured |
| 9f | `TestT26SRSAbsenceDepsErrorPropagates` | P | deps `queryErr` | no partial snapshot | (1,1,1) measured |
| 10a | `TestT26SRSAbsentRefusedNoReservedAdvisory` | X | reserved SRID plain | no advisory on refusal | vector not asserted |
| 10b | `TestT26SRSAbsentNoopReservedAdvisory` | X | reserved 4326 IE✓ | exactly 2 findings; strict noop + strict `{srid}`-only reserved; no leak; complete/pass | vector not asserted |
| 10c | `TestT26SRSAbsentConditionalReservedAdvisory` | X | reserved 4326 IE? | exactly 1 finding = strict reserved; `[P]` gap; unverified | vector not asserted |
| 10d | `TestT26SRSAbsentDisabledRuleSilent` | X | policy disables absent | silent, complete coverage | vector not asserted |
| 10e | `TestT26SRSAbsentNoopWarningLevel` | X | policy level=warning | full frozen shape at warning; review verdict | vector not asserted |
| 10f | `TestT26SRSAbsentSentinelScan` | X | sentinel cells | arm proven FIRST per variant (S2 `[D]`+notice / S1 refused / A-IE✓ noop), then full-blob scan | vector not asserted |
| 10g | three providerless probes | CLI | frozen SQL/policy | rc 0/1/2 + byte-exact streams | C2 transcript-bound |
| 10j | `TestT26SRSAbsentCanonicalJSON` | X | all three A arms | internal-layer serializer skeleton: index key at 0, omitempty, typed metadata, marker absent-vs-null, `[P]` gap exact (report→public equivalence bound in `bindings/`) | vector not asserted |
| in-domain supply | `TestT26SRSAbsenceSuppliedInDomain` + `…InDomain804` (P) + `…PlainRefused` (X) | P+X | Qo 8.0.36 / 8.0.4 | `Absence=true` supply + consumed | Qo/Qexist/Qdeps=1 measured |
| D-vs-D-proof | `TestT26SRSAbsenceNotSuppliedSupportedNon80` | P | Qo 8.4.3 | supported yet not proven → zero-row stays unknown | (1,1,1) measured |

Test-function accounting (R3-E5): **47 PA-dedicated test functions** —
6 internal (`package audit`), **6 real-chain** (`package audit_test`),
22 external API, 13 provider — plus reused R2/R3 functions named above.
Functions ≠ `--- PASS` lines (subtests expand) ≠ package counts.

Reused-helper binding (R3-D1): `bindings/r3-helper-and-json-bindings.txt`
in this archive carries candidate-bound (commit `0354540b` + real line
numbers) verbatim excerpts of every helper/member the mapping relies
on — see the R3-D1 bullet above for the member list.

## Real-interval diffstats (R3-E4 — recomputed; not carried forward)

| interval | files | +/− |
|---|---|---|
| baseline `0fb1b7ff` → C2 `1291b934` | 28 | +2900/−128 |
| R1 final `90fc080f` → C2 `1291b934` | 7 | +1048/−50 |
| C2 `1291b934` → F2 `f3d09134` | 2 docs | +254/−161 |
| baseline `0fb1b7ff` → F2 `f3d09134` | 28 | +2993/−128 |
| R1 final `90fc080f` → F2 `f3d09134` | 9 | +1302/−211 |
| F2 `f3d09134` → C3 `0354540b` | 6 | +376/−50 |
| C2 `1291b934` → C3 `0354540b` | 8 | +630/−211 |
| baseline `0fb1b7ff` → C3 `0354540b` | 28 | +3319/−128 |

## Documentation

- Module READMEs: `internal/infrastructure/metadata/mysql/README.md`
  narrowed its ErrNoRows sentence (R3-E1); no other module README
  changed (test-only `export_test.go` remains package-internal).
- Decision record `docs/decisions/2026-11-12-mysql-drop-srs-proven-absence.md`
  keeps the refusal explanation grounded in the accepted audit-model
  basis — it is not re-presented as native-execution proof.
- Observation layer: `TestT26SRSAbsentCanonicalJSON` runs at the
  internal request layer (`report.Result`); the report→public
  conversion binding in `bindings/` proves the frozen keys survive
  `fromDomainResult` (field-by-field copy, identical JSON tags).

## Errata / notes

- R2 mapping-table contradictions corrected per R3-D3: PA-1a had
  "notice" (actual: 0 findings); PA-1b/2a had "pass"+`L=0` (actual:
  reject + `L=1`); PA-1c/8b and PA-4b had `L=0` (actual: `L=1`);
  PA-2d was described as a three-statement fixture with a measured
  vector (actual: REVOKE+one DROP, no read assert); PA-2c/PA-4c had
  measured vectors the bodies never assert (now "unobserved"); PA-7's
  `*` is now `Vd=N/A` with reason; PA-9d's "loader N/A" is corrected
  — the channel exists, only the counting was absent; coverage/verdict
  claims are now stated per row at their asserted scope only.
- `internal/infrastructure/parser/tidb/mysql_t26r5_test.go` fails
  `gofmt -l` repo-wide at baseline `0fb1b7ff` (tracked, untouched) —
  registered observation only.
- REVOKE in an api-layer batch surfaces `ErrUnsupportedStatement`
  while every audited statement still projects (existing contract —
  PA-2d/7b assert it via `errors.Is`).
- R1/R2 historical gaps (preserved, not rewritten): R1 gate/probe
  captures recorded outputs and rcs; the R1 MANIFEST self-listed and
  omitted `RECEIPT.md`; the R1 build capture did not record the full
  expanded build command or the go.mod/go.sum hashes — those remain
  reported-not-captured history, NOT re-derived here (R3-E6).
- The close-error provider row documents actual `database/sql`
  propagation: the stub's `Close` sentinel surfaces through the
  iteration/`rows.Err()` path (EOF auto-close) — it is NOT claimed as
  proof that defer-only readers necessarily lose errors.
