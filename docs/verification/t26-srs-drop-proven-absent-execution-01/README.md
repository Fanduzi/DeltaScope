# T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01 — R4 assertion + record hardening

Round: **R4 — LIMITED ASSERTION + RECORD REVISION** against the accepted
contract at `0fb1b7ff` + evidence supplement `f206060d`
(`docs/verification/t26-srs-drop-proven-absent-contract-01`, R4 ACCEPTED;
issue #105, milestone T26/#105). R3 (`0354540b` impl + `8edf579d` docs)
returned CHANGES REQUIRED on assertion completeness, conversion-chain
binding, and record accuracy only — the reviewer confirmed the R3-A
handshake, R3-B clone semantics, R3-C per-statement measurements, and
both PA-6 internal companions stand. This round changed tests, comments,
and docs; **no executable product behavior was modified**. No push,
merge, tag, release, or issue closure is claimed or authorized by this
receipt.

Attribution discipline: **implementation and test work happened in C4**
(`4db2c1e4`, sequential on F3); the C4 commit/tree identity, its
two-package gates, the diffs, and this README were **recorded in F4**
(this commit's archive). R1/R2/R3-captured logs are historical —
nothing was re-run to rewrite them; R3-record gaps are registered as
facts, not reconstructed. C2 evidence (seven-package + full-suite
gates, builds, probes) is **reused per the member-level ledger** —
the reuse ledger references the delivered R2 archive members by path,
byte size, and full SHA-256 under their C2 source-commit labels; dates
and sources are not re-labelled.

## Identity

- Implementation candidate (C4): `4db2c1e409bb814e7e7ab87c0137f88264218491`
- C4 tree: `e8228d74cab1428724f7553134f6715c1e15d62e`
- C4 parent: `8edf579d` (F3 docs — sequential commits per R4 ruling)
- Branch: `milestone/mysql-tidb-ddl-completion`
- cwd `/Users/fan/GolangProjects/DeltaScope`; toolchain `go1.26.1
  darwin/arm64`

## R4 scope executed (C4; `.go` = tests + comments)

- **R4-A** — `TestT26SRSAbsenceMidIterationCancelPropagates` now asserts
  the FINAL read vector after `ResolveObject` returns:
  `t26AssertSeams(t, 1, 1, 0)` post-return rules out any Qo or
  existence attempt issued after the cancel. The pre-cancel handshake
  counts stay as the landing-point proof only — the reported
  `(1,1,0)` no longer mixes two observation times.
- **R4-B** — `TestT26SRSAbsentSnapshotFidelity` now asserts exact
  captured values (`MatchedSRIDs=[1]`, the full dependency-column
  tuple `{s,t,g,9}`, `Owed=[a]`); after mutating live the snapshot
  still equals the ORIGINAL values; after mutating the snapshot live
  still equals the WRITTEN values; `paSRSConflictSame` gains a
  positive control (identical non-empty → true) and a negative
  control (same Complete/nilness/length, differing element → false)
  — a shape-preserving clone that zeroed values can no longer pass.
- **R4-C** — `TestT26SRSAbsentBoundaryPrefixBlocks` now asserts
  `len(result.Unsupported) == 1`, binds that single implementation
  boundary to `Index == 0` (the prefix) and
  `Feature == "create_spatial_reference_system"` (the literal the
  prefix carries — `DDLOperationCreateSpatialReferenceSystem`), and
  asserts `result.Statements[0].Coverage.Status == CoverageIncomplete`;
  the frozen reason/boundary/object metadata, prefix zero
  findings/gaps, tail `[B]` gap, and total-read asserts are kept.
- **R4-D1** — `bindings/r4-bindings.txt` carries the full bodies of
  `fromDomainFindings`, `fromDomainEvidenceGaps`, and the decisive
  `cloneMetadataMap`/`cloneMetadataValue` helpers at C4 — the
  conversion preserves metadata values verbatim (no key renames,
  `srid` stays `uint32` → JSON number) and gap owner/reason/facts
  pass through verbatim. The public `Finding`/`EvidenceGap` JSON tags
  are reused per member identity from the accepted contract's
  `baseline-source-additions.txt` (pkg/deltascope/audit.go unchanged
  in this chain) — not re-copied.
- **R4-D2** — same file: `r3PrivilegeNotice`'s full body (the helper
  `r3S2Consumed` calls at `t26_srs_drop_r3_test.go:419`).
- **R4-E1** — PA-6e timing wording corrected: L=0 is an AFTER-APPLY
  measured value at each hook; the reason the loader never fired is
  the enrich-entry qualification gate plus the fixture — the
  after-apply counts no longer masquerade as a pre-mint observation.
  Comments corrected in both the internal companion and this README.
- **R4-E2** — PA-6e public comment corrected: the short-circuit cause
  is stmt1's COMPLETED boundary own-apply (it minted the authored
  stale key), not stmt2's own apply; the public row measures the pair
  TOTAL while per-statement increments belong to the internal
  companion — the two layers' attributions are registered separately.
- **R4-E3** — the reuse section below now describes the actual
  mechanism (`reuse-ledger.txt` pointing at delivered R2-archive
  members); the R3 README's `historical-c2/*` member paths did not
  exist in the R3 archive, and the ledger's member count is the
  actual 69 rows (the 69th is `REUSED probes/probe-policy.yaml`
  carried over from R1 classification).
- **R4-E4** — the R3 `gates/gofmt.cmd.txt` recorded a placeholder
  description (`<21-file filelist>`), not the executed argv. The R3
  original is kept as-is in the R3 archive; the fact is registered
  here. R4 gates record the literal expanded argv (see below).
- **R4-E5** — `identity-f4.txt` carries the command, the REAL
  `git status --porcelain` output, and its rc (clean → empty output);
  the R3 archive's bare `0` is registered as insufficient, not
  reconstructed.

## Gates at the immutable C4 `4db2c1e4` (literal argv + immediate rc)

- `go test -count=1 -v ./internal/application/audit
  ./internal/infrastructure/metadata/mysql` — rc 0; **5,007 PASS /
  0 FAIL / 0 SKIP** counted across all indentation; 2 `ok` packages.
  Raw: `gates/gate-verbose.*`.
- `go vet` same two packages — rc 0, 0-byte stdout/stderr.
  `gates/vet.*`.
- `gofmt -l` over the 21 cumulative changed `.go` files — rc 0, 0-byte
  output; `gates/gofmt.cmd.txt` records the LITERAL expanded argv
  (every file path on the command line) and
  `gates/gofmt-filelist.txt` the same list one-per-line. The
  pre-existing unformatted
  `internal/infrastructure/parser/tidb/mysql_t26r5_test.go` is NOT
  touched by this chain (tracked, unchanged at baseline; registered
  erratum, out of scope).
- `git diff --check 0fb1b7ff..4db2c1e4` — rc 0, 0-byte.
  `gates/diff-check.*`.
- `git diff --check 4db2c1e4..<F4>` (C4→F4 docs segment) — rc 0.
  `gates/diff-check-f4.*`.

Every gate file is `*.cmd.txt` + `*.stdout.log` + `*.stderr.log` +
`*.rc.txt`; each rc was captured by `rc=$?` on the line immediately
after its own command — no pipeline masking.

## Reused C2 evidence (member-ledger reuse; no re-run, no re-label)

This round changed no product `.go` file; product bytes and all build
inputs (`go.mod`/`go.sum`/toolchain) are unchanged between C2 `1291b934`
and C4 `4db2c1e4` (C2→C4 diffs are tests/test-exports/comments/docs
only). The mechanism is a **member-level reuse ledger**: this archive's
`reuse-ledger.txt` lists the **69 member rows** of the delivered R2
archive (`/tmp/t26-srs-pa-r2-archive.tar.gz`, SHA-256
`90432f8f51d6df06db394d3208095fbcc97f6667e0160bbb2e70ce54bbf8102a`,
21,738,209 B) — each row carries the R2-archive member path, byte size,
full SHA-256, and its R2 class (NEW / REUSED / HISTORICAL), under the
C2 `1291b934` source-commit label:

- `gates/*` — C2's seven-package verbose gate (8,378 PASS/
  1 SKIP/0 FAIL over 7 `ok` packages), `go test -count=1 ./...`
  full-suite (39 `ok`, 5 no-test-files, 0 failures), vet, gofmt,
  diff-check.
- `build/*` — C2's deterministic build pair: SHA-256
  `39aaaefb680597b4f39121da2d6ab0d2812d9a98f2a26e9a3332678a69af7728`,
  52,761,890 B, `CGO_ENABLED=1 -tags postgresql -buildvcs=false`,
  `go.mod` `80dd9105…`, `go.sum` `cc5fa030…`.
- `probes/*` — C2 binary b1's three providerless CLI probes: rc 0/1/2,
  stdout 896/1152/0 B, stderr 0/0/56 B, byte-exact frozen shapes.
- `historical-r1/*` — R1 captures preserved verbatim (class
  HISTORICAL in the R2 archive), never re-run.

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
| 6c | `TestT26SRSAbsentBoundaryPrefixBlocks` (X) + `…PrefixInternal` (R) | X+R | CREATE OR REPLACE prefix | X: exactly ONE Unsupported entry bound to index 0 + `create_spatial_reference_system` feature + implementation boundary + stmt1 CoverageIncomplete; R: no adjudication; tail audited B; stale `{100001}` no contamination | X total 1,1,1,1,1,1; R increments: prefix `[1,1,1,1,1]`, tail `[0,0,0,0,0]` (Vd N/A at R) |
| 6d | `TestT26SRSAbsentRecachesPerEnrich` + `…RealChain` | I+R | two enrich passes | per-enrich re-read | per-enrich counts measured |
| 6e | `TestT26SRSAbsentBoundaryVersionPair` (X) + `…VersionPairInternal` (R) | X+R | Qo 8.0.3, no pin | both version boundaries exact `{unaudited,gate:version,object:spatial_reference_system}`; neither adjudicates; minted `{100001}` | X total 1,1,1,1,0,1 + forwards=1; R increments measured after-apply: stmt1 `[1,1,1,1,0]`, stmt2 `[0,0,0,0,0]`; L=0 registered as an after-apply measured value — the loader-not-fired reason is the enrich-entry qualification gate + fixture, not a pre-mint observation; Vd N/A at R |
| 7a | `TestT26SRSAbsentRevokeScoping` (I) + `…RealChain` (R) | I+R | consumable→REVOKE→suppressed | SAME stmt1 at 3 hook points; complete SRS+adj+premise; nil/empty distinct; bidirectional isolation | R: total 2,2,2,2,1 + per-statement Δ `[1,1,1,1,1]`/`[0,0,0,0,0]`/`[1,1,1,1,0]`; **Vd=N/A — no discovery runs inside enrich**; L=1 enrich-level |
| 7b | `TestT26SRSAbsentRevokeScopingPublic` | X | same batch | stmt1 noop+marker; stmt3 `[P]` gap and nothing else | 2,2,2,2,1,1 measured |
| 8a | `TestT26SRSR3E10Proven803` (reused) | X | Qo 8.0.3 no pin | single-statement version boundary | R3-frozen body (`bindings/`) |
| 8c | `TestT26SRSR2DiscoveryMismatch` (reused) | X | pin/observed mismatch | `ErrTargetVersionMismatch` | R2-frozen body (`bindings/`) |
| 8d | `TestT26SRSPrivP9cInternalVendorBoundaryMint` (reused) | I | real TiDB parse | vendor boundary mints `{100001}` | R3-frozen body (`bindings/`) |
| 9a | `TestT26SRSAbsenceQueryErrorPropagates` | P | generic `queryErr` | `errors.Is`, nil snapshot | (1,1,0) measured |
| 9b | `TestT26SRSAbsenceIterationErrorPropagates` | P | `rowErrAt:0` | `errors.Is`, nil snapshot | (1,1,0) measured |
| 9c | `TestT26SRSAbsenceMidIterationCancelPropagates` | P | `blockOnCtx` rowset + `enteredNext` handshake | cancel DURING real `Next()` iteration; `errors.Is(context.Canceled)`; nil snapshot; **post-return final vector** excludes any post-cancel attempt | (1,1,0) measured twice — pre-cancel landing + post-return final |
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

Test-function accounting (R3-E5, unchanged at C4): **47 PA-dedicated test functions** —
6 internal (`package audit`), **6 real-chain** (`package audit_test`),
22 external API, 13 provider — plus reused R2/R3 functions named above.
Functions ≠ `--- PASS` lines (subtests expand) ≠ package counts.

Reused-helper binding (R3-D1 + R4-D): this archive carries two binding
files — `bindings/r3-helper-and-json-bindings.txt` (C3 `0354540b`-bound
verbatim excerpts of every helper/member the mapping relies on; the
product files it excerpts are unchanged at C4, verified byte-identical
across C3→C4 where applicable) and `bindings/r4-bindings.txt` (C4
`4db2c1e4`-bound full bodies of `fromDomainFindings`/
`fromDomainEvidenceGaps`/`cloneMetadataMap`/`cloneMetadataValue`/
`r3PrivilegeNotice`).

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
| C3 `0354540b` → F3 `8edf579d` | 1 doc | +230/−227 |
| F3 `8edf579d` → C4 `4db2c1e4` | 3 | +75/−37 |
| baseline `0fb1b7ff` → C4 `4db2c1e4` | 28 | +3360/−128 |
| C3 `0354540b` → C4 `4db2c1e4` | 4 | +305/−264 |

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

- R4-E1 wording correction: the R3 internal companion's comment
  claimed L=0 was observed "before any mint" — the hook is
  after-apply and `staleAt[0] == {100001}` was already asserted at
  that point; L=0 is now registered as an after-apply measured value
  and the loader-not-fired reason is attributed to the enrich-entry
  qualification gate + fixture, not to a timing the counts don't
  observe. No new timing hook was added.
- R4-E2 wording correction: the public PA-6e comment attributed the
  short-circuit to stmt2's own apply; the cause is stmt1's COMPLETED
  boundary own-apply (which minted the authored stale key). The
  public row asserts the pair total; per-statement increments are
  the internal companion's.
- R4-E3 record correction: the R3 README described reuse members as
  `historical-c2/*` — those paths never existed in the R3 archive.
  The actual mechanism is `reuse-ledger.txt` referencing delivered
  R2-archive members; the ledger's true member-row count is **69**
  (the R3 receipt's "68" undercounted `REUSED
  probes/probe-policy.yaml`, which keeps its R1-copied class).
- R4-E4 record correction: the R3 `gates/gofmt.cmd.txt` held a
  placeholder description rather than the executed argv; the R3
  original is kept in its archive and the fact is registered here.
  R4 gate captures record the literal expanded argv.
- R4-E5 record correction: the R3 `identity-f3.txt` carried a bare
  `0` for the status check; F4's identity file carries the command,
  real `git status` output, and rc (empty output stays empty). R3's
  transcript is not reconstructed.
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
  reported-not-captured history, NOT re-derived here.
- The close-error provider row documents actual `database/sql`
  propagation: the stub's `Close` sentinel surfaces through the
  iteration/`rows.Err()` path (EOF auto-close) — it is NOT claimed as
  proof that defer-only readers necessarily lose errors.
