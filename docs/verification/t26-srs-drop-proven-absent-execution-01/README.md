# T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01 — R2 test + evidence hardening

Round: **R2 — TEST + EVIDENCE-CHAIN REVISION** against the accepted
contract at `0fb1b7ff` + evidence supplement `f206060d`
(`docs/verification/t26-srs-drop-proven-absent-contract-01`, R4 ACCEPTED;
issue #105, milestone T26/#105). R1 (`9fb5d3dc` impl + `90fc080f` docs)
returned CHANGES REQUIRED on test depth and evidence-chain form only —
the reviewer found no product-path deviation requiring a contract
reversal. No push, merge, tag, release, or issue closure is claimed or
authorized by this receipt.

Attribution discipline (R2-F): **implementation and test work happened
in C2** (`1291b934`); the C2 commit/tree identity, gates, build, and
probe captures were **recorded in F2** (this commit's archive); this
README was **written in F2**. R1-captured logs are historical — nothing
was re-run to rewrite them.

## Identity

- Implementation candidate (C2): `1291b934166220552d2db60e2b29d4d77fc6414c`
- C2 tree: `d50155cd2ad14bfc93d8f94b843e2c3e51db02c4`
- C2 parent: `90fc080f` (R1 docs — sequential commits per R2 ruling)
- Branch: `milestone/mysql-tidb-ddl-completion`
- cwd `/Users/fan/GolangProjects/DeltaScope`; toolchain `go1.26.1
  darwin/arm64`; `CGO_ENABLED=1`, `-tags postgresql`,
  `-buildvcs=false` (expanded `make build-cli` recipe, `VERSION` unset
  → no ldflags)

## R2 scope executed (C2, `.go`-only)

- `internal/infrastructure/metadata/mysql/provider_t26_srs_absence_test.go`:
  - `TestT26SRSAbsenceSuppliedInDomain804` — Qo `8.0.4` (the domain
    lower bound) + clean zero-observed-row existence answer ⇒
    `{Read, !Exists, Absence}`; Qo/Qexist/Qdeps attempts all = 1.
  - `TestT26SRSAbsenceNotSuppliedSupportedNon80` — Qo `8.4.3` parses as
    MySQL major 8 (D-support holds → reads still issue once each) while
    `MySQLSRSDropAuditedDomain` (D-proof, 8.0.4+) fails ⇒ clean zero-row
    stays `{Read, !Exists, !Absence}` — support vs completeness domain
    separated on the same Qo read.
  - `t26AssertSeams` added to every failure row: query error `(1,1,0)`,
    iteration error `(1,1,0)`, `ErrNoRows`-valued query `(1,1,0)`,
    `ErrNoRows`-valued iteration `(1,1,0)`, checked-close `(1,1,0)`,
    dependency read `(1,1,1)` — attempted/never-attempted is measured,
    not inferred. Request-discovery and privilege-loader dimensions are
    N/A at the provider seam (no such calls exist there).
  - `TestT26SRSAbsenceMidIterationCancelPropagates` — the REAL PA-9c:
    Qo succeeds (attempt=1), the existence query is attempted, the
    context is cancelled while the existence rowset is mid-iteration
    (`testQueryResult.blockOnCtx`), `errors.Is(err, context.Canceled)`
    holds, nil snapshot, dependency read never attempted (=0). The
    former pre-cancelled row is kept, renamed
    `TestT26SRSAbsencePreCancelledRequestPropagates`, and labelled
    request-control — it is NOT the mid-read witness.
- `internal/infrastructure/metadata/mysql/provider_test.go`: stub gains
  `closeErr` + `blockOnCtx` (iteration blocks on `ctx.Done()`); every
  query — failed or clean — was already attempt-recorded per DSN.
- `internal/application/audit/export_test.go` (new): test-only exports —
  `TestBatchState`, `NewTestBatchState`, `EnrichSRSPrivForTest`,
  `BatchStateStaleSet`, `SetAfterApplyObserve` — so `package audit_test`
  can reach batch-state hooks while delegating to the real provider.
- `internal/application/audit/t26_srs_absent_realchain_test.go` (new,
  `package audit_test`): `paRealState` thin adapter —
  `ResolveObject` delegates to `mysqlmeta.NewProvider(db)` over the
  recorded stub driver (same seam counters as `r3ProductProvider`).
  - `TestT26SRSAbsentStaleChannelsRealChain` — PA-5a/5b/5c real-chain
    plus the same-SRID B, real `GARBAGE` B, and S2-union controls; each
    A arm asserts BOTH stale channels (`srsExistenceStale` exact set
    equality — no additions/removals — and `srsDepsStale` persistent).
  - `TestT26SRSAbsentRecachesPerEnrichRealChain` — PA-6d per-enrich
    recaching over the real provider path.
  - `TestT26SRSAbsentRevokeScopingRealChain` — PA-7: the SAME stmt1 is
    deep-copied at three hook points (① after stmt1 own-apply,
    ② after the REVOKE apply, ③ after stmt3 apply) plus post-batch;
    `paSnapStmt`/`paSnapEqual` compare complete `SpatialReference` +
    `SRSDrop` + premise with explicit nil-vs-empty distinction and no
    shared mutable descendants.
- `internal/application/audit/t26_srs_absent_int_test.go`:
  `srsForeignAbsentProvider` counts privilege loads (`privCalls`);
  PA-4e2 now asserts `premise=consumable`, measured `L=1`, owed exactly
  `[srs.existence, srs.column_dependencies]`, authored SRID 100001
  minted (foreign 100002 never), resolver called exactly once, and the
  real rule projection — exactly one `ddl.mysql.srs.drop.privilege`
  notice + the dependency-owned `[E,D]` gap. The custom resolver is an
  intentional abnormal-input seam; no real-mysqlmeta mismatch claim.
- `internal/application/audit/t26_srs_absent_test.go`:
  - Reserved/policy/REVOKE public rows are strict — exact finding count,
    exact RuleID, exact level, exact message, exact metadata key set,
    `srid` typed `uint32`, exact marker token, exact gap count/owner/
    reason/ordered facts, statement+aggregate coverage, verdict. The
    reserved advisory is membership-only and `{srid}`-only.
    `paAbsentNoopFinding` checks one finding's full frozen shape.
    PA-7b asserts stmt3 carries the conditional gap and NOTHING else.
  - `TestT26SRSAbsentCanonicalJSON` — real-serializer skeleton asserts
    for A-plain / A-IE✓ / A-IE?: `statements[0].index` key present at
    numeric 0, empty `findings`/`evidence_gaps` keys omitted,
    `statement_index=0` omitted via `omitempty`, `metadata.srid` a JSON
    number, exact metadata key sets, absent marker vs `marker:null`
    distinguished, the `[P]` gap's owner/reason/ordered facts exact.
  - `TestT26SRSAbsentSentinelScan` — each variant asserts its arm's
    public projection FIRST (S2 `[D]`+notice / S1 refused-dependency /
    A-IE✓ noop+marker+zero-gaps), THEN scans the serialized result;
    sentinels stay out of `raw_sql`.
  - PA-6c asserts the first statement's typed CREATE OR REPLACE
    implementation boundary (`boundary=implementation`,
    `object=spatial_reference_system`, unaudited reason) and the tail
    DROP's audited-B shape with the frozen read vector.
  - PA-6e asserts both statements are version boundaries — exact
    `{unaudited, gate:version, object:spatial_reference_system}` per
    `UnsupportedDetail`, second statement never an SRSDrop-B, zero
    findings/gaps, incomplete coverage, and the measured single-pass
    read vector (stmt2's zero increment is measured, not inferred).
- `internal/domain/spec/srs_drop.go`: comment-only — the S2 bullet and
  `SRSDropPrivilegeConsumable` now distinguish S2 privilege consumption
  (S2-specific notice) from A-IE✓ consumption (absent_noop marker); the
  privilege notice remains S2-specific.

## Gates (all at C2 `1291b934`; invocation + immediate rc captured)

- `go test -count=1 -v ./internal/domain/spec ./internal/application/audit
  ./internal/domain/rule/ddl ./internal/infrastructure/parser/tidb
  ./internal/domain/policy ./pkg/deltascope
  ./internal/infrastructure/metadata/mysql` — rc 0; **8,378 PASS /
  1 SKIP / 0 FAIL** counted across all indentation; 7 `ok` packages.
  Raw `gates/gate-verbose.*`.
- `go test -count=1 ./...` — rc 0; 39 `ok` packages, 5
  `[no test files]`, 0 package failures. `gates/full-suite.*`.
- `go vet` same seven packages — rc 0, 0-byte stdout/stderr. `gates/vet.*`.
- `gofmt -l` over the 21 cumulative changed `.go` files
  (`gates/gofmt-filelist.txt`) — rc 0, 0-byte output. The pre-existing
  unformatted `internal/infrastructure/parser/tidb/mysql_t26r5_test.go`
  is NOT touched by this chain (tracked, unchanged at baseline;
  registered erratum, out of scope).
- `git diff --check 0fb1b7ff..HEAD` — rc 0, 0-byte. `gates/diff-check.*`.

Every gate file is a `*.cmd.txt` + `*.stdout.log` + `*.stderr.log` +
`*.rc.txt` quartet; each rc was captured by `rc=$?` on the line
immediately after its own command — no pipeline masking.

## Build + rebuild (from C2; explicit `-buildvcs=false`)

Expanded recipe (Makefile `build-cli`, `VERSION` unset):
`CGO_ENABLED=1 go build -tags postgresql -buildvcs=false -o <out> ./cmd/deltascope`.

- env: `go1.26.1 darwin/arm64`, `GOOS=darwin GOARCH=arm64`,
  `CGO_ENABLED=1`, build tag `postgresql`, working tree clean for
  tracked paths at C2 (`build/env.txt`); `go.mod` SHA-256
  `80dd91053db571bf7e14af57d806e254726afa3a16bbd0998486c4356b628c98`;
  `go.sum` SHA-256
  `cc5fa030a1c92992d2003ff2fd64efa1a247cd5f3679e5e3debae21c55000696`.
- build 1: rc 0, 52,761,890 B, SHA-256
  `39aaaefb680597b4f39121da2d6ab0d2812d9a98f2a26e9a3332678a69af7728`
  — `build/deltascope-b1` + `build1.*`.
- build 2: rc 0, identical size and SHA-256 — `build/deltascope-b2` +
  `build2.*`. `cmp` rc 0.
- R1 historical report used the same flags minus `-buildvcs=false` and
  produced SHA `df4db54a…` at the same byte size — historical record
  preserved; no cross-candidate hash-equality claim is made (source
  changed).

## Providerless CLI probes (C2 binary b1; contract PA-10g reference)

Policy `probes/probe-policy.yaml` (SHA-256 `327a3e53…`, unchanged since
R1) enables all six SRS drop rules; SQL `DROP SPATIAL REFERENCE SYSTEM
100001;`; `--dialect mysql --format json --fail-on none`; providerless;
privilege mode off. `probes/run-probes.sh` is the capture script —
each invocation's `$?` is saved on the next line before any other
command (`probes/transcript.txt`).

| probe | tail | rc | stdout | stderr | expected | actual |
|---|---|---|---|---|---|---|
| CLI-1 | `--target-version 8.0.36` | 0 | 896 B | 0 B | review; unverified; 0 findings; 1 `unknown_srs_state` gap `[srs.existence,srs.column_dependencies,srs.drop.privilege]` | exact match |
| CLI-2 | (no pin) | 1 | 1152 B | 0 B | boundary `{unaudited,gate:"version",object:"spatial_reference_system"}` + `unsupported_statement` diagnostic | exact match |
| CLI-3 | `--target-version 8.0.x` | 2 | 0 B | 56 B | stderr exactly `target_version must match [v]MAJOR.MINOR.PATCH: "8.0.x"`\n | exact match |

Providerless probes are regression-only evidence — they cannot supply
certified absence; proven-absence supply is proven by the provider PA
rows and the online/API matrix.

## Case-matrix mapping (PA rows → tests; one row each)

Layer: **P** = provider (`package mysqlmeta`), **I** = internal
(`package audit`), **R** = real-chain (`package audit_test` over
`mysqlmeta.NewProvider`), **X** = external API (`AuditSQL`).

| PA | function / subcase | layer | fixture difference | asserted fields | measured vector |
|---|---|---|---|---|---|
| 1a | `TestT26SRSAbsentProviderlessStaysUnproven` | X | no MetadataProvider | S2, `[E,D,P]` gap, notice | Vd/R/Qexist/Qdeps/Qo/L all 0 |
| 1b/2a | `TestT26SRSAbsentPlainRefused` | X | zero-row cell, Qo 8.0.36 | absent_refused blocker, exact msg/meta, no gap, complete/pass | 1,1,1,1,0,1 |
| 1c/8b | `TestT26SRSAbsentUnprovenQoStaysUnproven` | X | Qo `unparseable` | S2 `[E,D]` + notice | 1,1,1,1,0,1 |
| 2b | `TestT26SRSAbsentIfExistsNoop` | X | grants SUPER | absent_noop+marker, no gap | 1,1,1,1,1,1 |
| 2c | `TestT26SRSAbsentIfExistsConditional` | X | grants SELECT only | `[P]` `unproven_privilege`, 0 findings | 1,1,1,1,1,1 |
| 2d | `TestT26SRSAbsentIfExistsSuppressed` | X | REVOKE between | suppressed ⇒ same `[P]` gap | 2,2,2,2,1,1 |
| 3a | `TestT26SRSAbsentExistsStaysUnproven` | X | positive row | S2 `[D]` + notice | 1,1,1,1,1,1 |
| 4a | `TestT26SRSR3ZeroRowPositiveDepsRefuses` (reused) | X | zero-row + deps row | S1 refused_dependency | R3-frozen |
| 4b | `TestT26SRSAbsentIfExistsDependencyRefused` | X | deps row + IF EXISTS | S1 refused | 1,1,1,1,0,1 |
| 4c | `TestT26SRSAbsentDepsStaleDemotesWitness` | X | prefix-minted stale | depsStale demotes → A-plain | 1,1,1,1,0,1 |
| 4d | `TestT26SRSR3SRID0Request` (reused) | X | SRID 0 | S0 blocker | R3-frozen |
| 4e/4f/4f2 | `TestT26SRSAbsentAdjSupplyDiscriminants`, `TestT26SRSAbsentAdjIfExistsSplit` | I | state-literal table | arm + owed per discriminant; S1-outranks-absence | reads N/A (no provider) |
| 4e2 | `TestT26SRSAbsentEnrichForeignSRID` | I | custom resolver: authored 100001, returned state SRID 100002 | S2; premise=consumable; owed `[E,D]`; `L=1`; authored minted, foreign never; resolver ×1; strict notice + `[E,D]` gap | SQL seams N/A (custom resolver issues no catalog SQL) |
| 4g | `TestT26SRSAbsenceQueryErrNoRowsPropagates` | P | `queryErr=sql.ErrNoRows` | `errors.Is`, nil snapshot | (1,1,0) |
| 4h | `TestT26SRSAbsenceIterationErrNoRowsPropagates` | P | `rowErrAt:0,rowErr=ErrNoRows` | `errors.Is`, nil snapshot | (1,1,0) |
| 4i | `TestT26SRSAbsenceEOFFterPositiveRow` | P | positive row then EOF | present; Absence=false | implicit (success row) |
| 5a–f | `TestT26SRSAbsentStaleChannels` (I, 6 subtests) + `TestT26SRSAbsentStaleChannelsRealChain` (R) | I+R | pre-minted foreign stale + depsStale | per-A-arm both stale channels; same-SRID B, `GARBAGE` B, S2-union controls | per-arm reads measured at entry |
| 6a/6b | `TestT26SRSAbsentSameSRIDCacheHit`, `TestT26SRSAbsentDifferentSRIDRereads` | X | 2-statement same/diff SRID | cache hit vs re-read | Δ counted per seam |
| 6c | `TestT26SRSAbsentBoundaryPrefixBlocks` | X | CREATE OR REPLACE prefix | stmt1 implementation boundary (boundary/object/reason); tail audited B; Δreads=0 | 1,1,1,1,1,1 total |
| 6d | `TestT26SRSAbsentRecachesPerEnrich` + `…RealChain` | I+R | two enrich passes | per-enrich re-read | per-enrich counts |
| 6e | `TestT26SRSAbsentBoundaryVersionPair` | X | no pin, Qo 8.0.3 | both statements version boundaries (exact metadata), stmt2 never SRSDrop-B, measured single pass | 1,1,1,1,0,1 + forwards=1 |
| 7a | `TestT26SRSAbsentRevokeScoping` (I) + `TestT26SRSAbsentRevokeScopingRealChain` (R) | I+R | consumable→REVOKE→suppressed | SAME stmt1 snapshotted at 3 hook points; complete SRS+adj+premise; nil/empty distinct; no shared descendants | 2,2,2,2,1,* |
| 7b | `TestT26SRSAbsentRevokeScopingPublic` | X | same batch | stmt1 noop; stmt3 `[P]` gap and NOTHING else | 2,2,2,2,1,1 |
| 8a | `TestT26SRSR3E10Proven803` (reused) | X | Qo 8.0.3 no pin | single-statement version boundary | R3-frozen |
| 8c | `TestT26SRSR2DiscoveryMismatch` (reused) | X | pin/observed mismatch | `ErrTargetVersionMismatch` | R2-frozen |
| 8d | `TestT26SRSPrivP9cInternalVendorBoundaryMint` (reused) | I | real TiDB parse | vendor boundary mints `{100001}` | R3-frozen |
| 9a | `TestT26SRSAbsenceQueryErrorPropagates` | P | generic `queryErr` | `errors.Is`, nil snapshot | (1,1,0) |
| 9b | `TestT26SRSAbsenceIterationErrorPropagates` | P | `rowErrAt:0` | `errors.Is`, nil snapshot | (1,1,0) |
| 9c | `TestT26SRSAbsenceMidIterationCancelPropagates` | P | `blockOnCtx` rowset | cancel DURING iteration; `errors.Is(context.Canceled)`; nil | Qo=1,Qexist=1,Qdeps=0 |
| 9c-ctl | `TestT26SRSAbsencePreCancelledRequestPropagates` | P | pre-cancelled ctx | request-control only — NOT the 9c witness | n/a (renamed row) |
| 9c-close | `TestT26SRSAbsenceCloseErrorPropagates` | P | `closeErr` sentinel | error reaches caller via `rows.Err()`/EOF path | (1,1,0) |
| 9d | `TestT26SRSAbsentNoResolverStaysUnproven` | X | capability minus ObjectResolver | S2 `[E,D]` + notice | loader N/A |
| 9e | `TestT26SRSAbsenceNotSuppliedUnprovenVersion` | P | Qo `unparseable` | state literal `{Read,!Exists,!Absence}` | (1,1,1) |
| 9f | `TestT26SRSAbsenceDepsErrorPropagates` | P | deps `queryErr` | no partial snapshot | (1,1,1) |
| 10a | `TestT26SRSAbsentRefusedNoReservedAdvisory` | X | reserved SRID plain | no advisory on refusal | — |
| 10b | `TestT26SRSAbsentNoopReservedAdvisory` | X | reserved 4326 IE✓ | exactly 2 findings; strict noop + strict `{srid}`-only reserved; no leak; complete/pass | — |
| 10c | `TestT26SRSAbsentConditionalReservedAdvisory` | X | reserved 4326 IE? | exactly 1 finding = strict reserved; `[P]` gap; unverified | — |
| 10d | `TestT26SRSAbsentDisabledRuleSilent` | X | policy disables absent | silent, coverage complete | — |
| 10e | `TestT26SRSAbsentNoopWarningLevel` | X | policy level=warning | full frozen shape at warning; review verdict | — |
| 10f | `TestT26SRSAbsentSentinelScan` | X | sentinel cells | arm proven FIRST per variant (S2 `[D]`+notice / S1 refused / A-IE✓ noop), then full-blob scan | — |
| 10g | three providerless probes | CLI | frozen SQL/policy | rc 0/1/2 + byte-exact streams | transcript-bound |
| 10j | `TestT26SRSAbsentCanonicalJSON` | X | all three A arms | real serializer skeleton: index key at 0, omitempty behavior, typed metadata, marker absent-vs-null, `[P]` gap exact | — |
| in-domain supply | `TestT26SRSAbsenceSuppliedInDomain` + `…InDomain804` (P) + `…PlainRefused` (X) | P+X | Qo 8.0.36 / 8.0.4 | `Absence=true` supply + consumed | Qo/Qexist/Qdeps =1 |
| D-vs-D-proof | `TestT26SRSAbsenceNotSuppliedSupportedNon80` | P | Qo 8.4.3 | supported yet not proven → zero-row stays unknown | (1,1,1) |

Test-function accounting (R2-F): **44 PA-dedicated test functions** —
6 internal (`package audit`), 3 real-chain (`package audit_test`), 22
external API, 13 provider — plus reused R2/R3 functions listed above.
This counts functions, not `--- PASS` lines (subtests expand them) and
not package counts. The R1 receipt's "31" line was corrected to its
then-actual 37 by the reviewer; the current set is 44.

Reused-helper binding (R2-D2): `r3ProductProvider`/`r3Harness`/
`r3Fixture` — `t26_srs_drop_r3_test.go`; `r3PrivilegeFixture`/
`r3PrivilegeRequest`/`r3AssertReads`/`r3OnlyFinding`/`r3Aggregate` —
`t26_srs_priv_exec_test.go`; `r3S2Consumed`/`r3RefusedDependency`/
`r3PrivilegeNotice`/`r3Boundary` — `t26_srs_drop_r3_test.go`;
`assertSRSPrivilegeNotice`/`intExactGap`/`evaluateEnriched` —
`t26_srs_drop_r3_test.go` + `t26_srs_absent_int_test.go`; reused test
bodies PA-4a/4d/8a/8c/8d — see table rows above.

## Documentation

- Module READMEs unchanged this round (no new exports; test-only
  `export_test.go` is package-internal and not an export surface).
- Decision record `docs/decisions/2026-11-12-mysql-drop-srs-proven-absence.md`
  keeps the refusal explanation grounded in the accepted audit-model
  basis — it is not re-presented as native-execution proof.
- Observation layer noted for C2: in-memory `report.Result` vs the
  external JSON are pinned separately — `TestT26SRSAbsentCanonicalJSON`
  binds the serializer itself (`omitempty` at index 0, typed metadata,
  marker absent-vs-null).

## Errata / notes

- `internal/infrastructure/parser/tidb/mysql_t26r5_test.go` fails
  `gofmt -l` repo-wide at baseline `0fb1b7ff` (tracked, untouched) —
  registered observation only.
- REVOKE in an api-layer batch surfaces `ErrUnsupportedStatement` while
  every audited statement still projects (existing contract — PA-2d/7b
  assert it via `errors.Is`).
- R1 historical gaps (preserved, not rewritten): R1 gate/probe captures
  recorded outputs and rcs; the R1 MANIFEST self-listed and omitted
  `RECEIPT.md` — both are registered as historical errata, corrected in
  R2's manifest.
- The close-error provider row documents actual `database/sql`
  propagation: the stub's `Close` sentinel surfaces through the
  iteration/`rows.Err()` path (EOF auto-close) — it is NOT claimed as
  proof that defer-only readers necessarily lose errors.
- `sql.ErrNoRows` wording: on the SRS existence-read path an
  ErrNoRows-valued error (query or iteration) is always an error —
  never a clean zero-row answer; the Qo `select version()` read keeps
  its existing `ErrNoRows` → unknown-support behavior.
