# T26-SERVER-ALTER-EXECUTION-01 — Execution Evidence Index

Work order: `T26-SERVER-ALTER-EXECUTION-01 / R5` — single-item
closeout: the table/schema snapshot's fidelity clone. R4 closed
R4-A/B/C/E and the ParseObservedVersion/AM-9.3d parts of R4-D; the
only open item was the snapshot helper's losslessness
(cloneTableSnapshot normalizes post-state). R5 replaces it with the
test-only `fidelityCloneValue` reflection walk plus three
distinguishing checks. Prior work order text — R3's snapshot,
timepoint, flag-true, frozen-row, loader-error, and six-package log
statistics were verified kept; the ruling closed most R3 items and
required: serialized-scan rows pinned to their real arms (the A2 row
had landed A1), frozen mode-off L/R/S fixtures, a REAL two-call
SQL-seam measurement for AM-8.1, authored record-numbering and
resolver-termination witnesses, static source supplements for
`cloneTableSnapshot`/`ParseObservedVersion`, gate-package restoration
(`internal/domain/policy`), and evidence reconciliation. R1–R3 raw
evidence stays preserved; historical statistics carry the errata
recorded below.

Baseline: `93945c014411ad0ef46f14b4075136965c37fb85`
(tree `8df836398a918399e0b38ebc8862b81609c2a017`, branch
`milestone/mysql-tidb-ddl-completion`).

Authoritative contract: `docs/verification/t26-server-alter-contract-01/`
(R5 ACCEPTED).

## Contents

| file | role |
|---|---|
| `README.md` | this index — scope, identities, verification summary |

## Implemented path (contract §D)

`parser facts → final qualification → request privilege observation →
per-statement bind/adjudication → findings/coverage → ordered apply`

- Qualification: `mysqlServerAuditedForm` admits identifier-form names
  within the 64-rune bound for `ServerKindAlter`; ALTER has no
  `IF EXISTS`/`IF NOT EXISTS`, no `FOREIGN DATA WRAPPER`; literal
  names, overlong forms, and TiDB keep their boundaries.
- Gate: the G2 version gate extends to audited identifier-form ALTER —
  unknown or out-of-domain resolved versions mark the statement
  unaudited with `gate:"version"` before the family privilege loader.
  The keep-boundary-bind path still issues its one catalog read only
  under the full premises: clean channel (no taint/contamination), no
  ordered answer on the identity, resolver cache miss, identifier-form
  name, and a provider that actually executes — never generalized to
  every gated/boundary form (R3-F.3).
- Premise: `serverAlterPrivilegePremise()` — independent projection of
  the shared request witness (unproven / consumable /
  suppressed-by-scope); never borrowed from DROP/CREATE.
- Adjudication: `spec.AdjudicateServerAlter` on `Metadata.ServerAlter`
  — barrier / terminal refusal on OrderedAbsent / proven alter /
  unproven; owed facts `server.existence`, `server.batch_effects`,
  `server.alter_privilege`, `server.alter_options`; `PublishPresent`
  republishes proven presence; `ConditionalSource` equals the published
  `OrderedPresentConditional`, kept separate from `ConditionalAudit`.
- Ordered apply: presence-preserving — A1 uniform no-op, A2 leaves the
  consumed absent standing, A3/A4 republish where applicable; no
  audited `serverWrite`, no minted barrier on unknown existence.
- Rule: `ddl.mysql.server.alter.notice` (default notice) projecting
  outcome `conditional_alter` / `refused_absent` (blocker) /
  `blocked_batch_effects` / `unverified`, plus the A3/A4-only
  `notice: connection_members` advisory with the fixed contract text.

## AM → test mapping

| AM row group | tests |
|---|---|
| G1 adjudication-unit (AM-1.3, AM-2.4, AM-2.5) | `spec.TestAdjudicateServerAlterTruthTable` (R2 adds the two exact seeded rows: conditional OrderedAbsent + own Unproven + clean → A2, conditional OrderedPresent + own Unproven + clean → A4), `spec.TestAdjudicateServerAlterConditionalAxes` |
| G2 qualification + version gate (AM-9.x) | `audit.TestT26AlterVersionGate`, `TestT26AlterVersionGateStillBinds`, `TestT26AlterResolvedVersionFixture`, `TestT26AlterLoaderErrorBoundary`, `deltascope.TestT26AlterPrivilegeHookPublicSurface`; R2 adds the SDK-seam rows `TestT26AlterAM91bOutOfDomain`, `TestT26AlterAM93aPinnedOutOfDomain`, `TestT26AlterAM93bTiDBVendorBoundary`, `TestT26AlterAM93dProductMismatch`, `TestT26AlterAM93eVersionMismatch` with independent L/R/S counts |
| G3 parser/extraction boundary (AM-3.x) | `tidb.TestT26AlterServerIdentLeavesBoundary` (R2 adds PORT 9999/10000 rows), `tidb.TestT26AlterServerNonAuditedStaysUnaudited`, `tidb.TestT26R4ServerOptionsStrict`, `audit.TestT26AlterAM3OptionsChain`, corpus `mysql_server_alter_rules` |
| premise independence (AM-2.1–2.3) | `audit.TestT26AlterPrivilegeProjection`, `TestT26AlterAM23FrozenPublicProjection` (frozen per-ALTER projection: main unverified + fixed advisory + exact `{server.alter_privilege}` gap + unverified coverage + L=1/R=1/S=1); AM-2.5i stored `flag=true` post-apply asserted inside `TestT26AlterAM24iAM25iSuppressedChains` (R3-B.4) |
| A3/A4 publish + republish (AM-1.x, AM-2.4i/2.5i, AM-6.x) | `audit.TestT26AlterProvenRepublish`, `TestT26AlterConditionalAxesProjection`, `TestT26AlterInheritedRepublish`, `TestT26AlterZeroRowNoState`, `TestT26AlterSiblingUnchanged`; R2 adds `TestT26AlterAM24iAM25iSuppressedChains` (explicit own-premise `Suppressed`), `TestT26AlterAM12ModeOff`, `TestT26AlterAM14AM15ExactUnknown`, `TestT26AlterAM61OrderedConsume`, `TestT26AlterAM62ZeroPair`, `TestT26AlterAM64AM65NoShadow` |
| A1 uniform no-op (AM-7.x, AM-8.x) | `audit.TestT26AlterRelationBarrierNoOp`, `TestT26AlterChannelTaintBarrier` — R3: compared on COMPLETE `serverChannelSnapshot` deep-value copies (entry fields via `serverEntrySnapshot`, writes, taint, contamination — never `%#v` pointer renders). The contamination source's COMPLETE-snapshot proof is `TestT26AlterAM81TwoCallContamination` (R4 attribution: `TestT26AlterRealContamination` retains partial field checks only) — R3: + call1 metadata stability across call2 + public projections `conditional_alter`/`blocked_batch_effects`; `TestT26AlterAM82LoaderBeforeFailure` (R4: record#1/stmt#2 authored-position ordering asserted at the parse layer — failure line 1 col 1 precedes ALTER line 1 col 11), `TestT26AlterAM76OrderedPresentFinalConsumer`, `TestT26AlterAM73aDropOwnBarrier`, `TestT26AlterAM73bOwnReadExistence`, `TestT26AlterAM73ApplyOwnership`, `TestT26AlterAM71AM72CaseAlias` (R3: frozen ALTER→DROP alias chain, present consumes ordered answer R=1, zero/off read s9+S9 verbatim R=2), `TestT26AlterAM74LiteralVsOverlong`, `TestT26AlterAM75LiteralClearsStalePresence` (R3: direct own-apply OrderedPresent clearing, DROP barrier as corroboration only; R4 attribution: this is the segmented-auxiliary L=2 witness — AM-10.3 registers L=1), `TestT26AlterAM81TwoCallLayerCounts` (R4: REAL two-call shared-batchState fixture on a genuine database/sql→driver seam — call1 1/1/1, call2 0/0/0, totals 1/1/1; ΔL=0←mode off, ΔR=0←contamination, ΔS=0←driver boundary) |
| lifecycle chains (AM-4.x, AM-5.x, AM-6.3) | `audit.TestT26AlterAM41AbsentSurvivesRefusal`, `TestT26AlterAM42ModeOffBarrier`, `TestT26AlterAM43LiteralDropTaint`, `TestT26AlterAM51FullChain` (R3: + stored `presentConditional=true` at the post-apply seam + final DROP `ConditionalSource=true`), `TestT26AlterAM63CleanAlterThenCreateRefused` (R3: clean ALTER→CREATE refused_present, no inherited marker) |
| metadata lifecycle (AM-6.6, AM-10.2) | `audit.TestT26AlterAM66MetadataLifecycle`, `TestT26AlterAM102DropConsumesMetadataStable` — R3: independent copies at the THREE REAL internal timepoints via the nil-defaulted `afterBindObserve`/`afterApplyObserve` seams (post-bind/pre-apply, post-own-apply, post-later-apply) on one enrich call, single-request L=1 |
| channel isolation (AM-10.3/10.4) | `audit.TestT26AlterTableChannelIsolation` — bidirectional complete snapshots (`tableSchemaSnapshot`; `serverChannelSnapshot` with the accepted `serverEntrySnapshot` field copy), ALTER-apply leaves table/schema value-identical AND CREATE-apply leaves the server channel value-identical; `TestT26AlterChannelTaintBarrier`, `TestT26AlterAM104LiteralModeOff` (frozen s9-zero mode-off fixture). R5: the table-shape member is cloned by the TEST-ONLY `fidelityCloneValue` walk — the product `cloneTableSnapshot` normalizes post-state (VisibilityExplicit, nil-vs-empty) and cannot serve as a lossless witness; distinguishing coverage in `TestT26AlterSnapshotFidelity` |
| public redaction + advisory (AM-10.1) | `audit.TestT26AlterPublicRedaction` (complete A3 result serialized and scanned), `TestT26AlterAM101ArmSerializedScans` (R3: complete serialized scans on A1/A2/A4/boundary fixtures with distinguishable injected sentinels — name/options/password/host/account/grant — exact RawSQL/NormalizedSQL skeletons), rule `TestMySQLAlterServer*` (7 tests) |
| error boundaries (AM-9.4/9.5) | `deltascope.TestT26AlterAM94ResolverSentinel` (R3: injected `pw-sentinel-455` payload IS the checked text — bounded `metadata object read failed`, `errors.Is` preserved, L=1/R=1/S=0), `audit.TestT26AlterAM94InternalResolverTermination` (R4: resolver sentinel terminates the statement's own bind inside enrich — nil enriched, no adjudication/apply, no gap substitute, L=1/R=1), `deltascope.TestT26AlterAM95LoaderSentinel` (L=1/R=0/S=0), `audit.TestT26AlterAM95InternalLoaderBoundary` (R3: internal `serverPrivilegeReadError` class via `errors.As`, bounded message, enrich-entry termination, L=1/R=0), `audit.TestT26AlterLoaderErrorBoundary` |
| layered L/R/S on same fixtures (AM-7.2–8.1, AM-10.4) | `deltascope.TestT26AlterR3DMeasuredLayerCounts` — real provider+stub driver: AM-7.2 frozen mode-off row L/R/S=0/2/2, AM-7.3a/b S=1/2, AM-7.4 frozen 65-rune mode-off row 0/1/1, AM-7.5 1/2/1, AM-7.6 1/1/1, AM-8.1-single-request 1/1/1 (supplementary — the real two-call measured fixture is `audit.TestT26AlterAM81TwoCallLayerCounts`), AM-10.4 0/1/0; grants SQL counted separately, never mixed into server-channel S |
| CLI probes | three recorded probes below (contract Group 9 probe rows) — in-domain pin, no pin, malformed pin |

## Verification summary (all commands logged; logs delivered as attachments)

- Six-package verbose gate — the established six are
  `internal/domain/spec`, `internal/application/audit`,
  `internal/domain/rule/ddl`, `internal/infrastructure/parser/tidb`,
  `internal/domain/policy`, `pkg/deltascope`.
  **R3 log identification (R4 erratum):** the R3 `six-pkg.log` was run
  against a different set — `internal/application/policy` substituted
  for `internal/domain/policy`. Its 7,808/7,814/1/0/6 statistics are
  correct FOR THAT SET; `internal/domain/policy` was still covered by
  the R3 full-suite run, so nothing was untested — the label was wrong,
  not the coverage. R4 gates restore the established list.
  `go test -count=1 -v` — R1: 7,772
  `--- PASS` lines at all indent levels (7,778 lines containing
  PASS), 1 SKIP, 0 FAIL, 6 `ok`. R2: 7,806 / 7,812 / 1 / 0 / 6.
  R3 (application/policy-substituted set, see note): rc=0, 7,808
  `--- PASS` lines at all indent levels, 7,814 lines containing PASS,
  1 SKIP, 0 FAIL, 6 `ok`. R4 (established set): numbers recorded in
  this round's commands.txt/build-record.txt and log statistics.
- Full repository suite (`go test -count=1 ./...`): R1/R2/R3: rc=0,
  39 `ok` package lines + 5 `[no test files]` packages, 0 FAIL.
- `go vet` on the six packages: rc=0. `git diff --check`: rc=0.
  `gofmt -l` on every file in the diff: clean.
- Binary: built from this exact source state with
  `go build -buildvcs=false -o /tmp/t26alter-r3-evidence/deltascope
  ./cmd/deltascope` (the recorded output path — shown shortened as
  `-o deltascope` in earlier rounds; same flags), Go 1.26.1
  darwin/arm64, CGO_ENABLED=0; identity recorded in
  `build-record.txt` (delivered attachment).

## Three CLI probes (frozen expectations, contract Group 9 probe rows)

Providerless, privilege mode off, explicit `policy-alter.yaml`; stdout
and stderr captured separately, rc captured immediately.

| probe | expected | actual |
|---|---|---|
| pin `8.0.36` | A4 owed `{server.existence, server.alter_privilege}`; 2 notice findings; main `unverified`, no marker; coverage `unverified`; verdict `review`; rc 0 | rc=0; `required_facts` exactly `{server.existence, server.alter_privilege}`; 2 notice findings; `outcome=unverified`, no `conditional_audit`; statement+aggregate coverage `unverified`; verdict `review` |
| no pin | `unaudited` + `gate:"version"`; findings=0, gaps=0; coverage `incomplete`; verdict `review`; rc 1 | rc=1; `unsupported[0].metadata.gate="version"`; 0 findings/gaps; coverage `incomplete`; verdict `review` |
| malformed pin `8.0.x` | typed `ErrInvalidTargetVersion` preflight error; no audit result; rc 2 | rc=2; stderr `target_version must match [v]MAJOR.MINOR.PATCH: "8.0.x"`; empty stdout |

All three match the frozen expectations exactly.

## Boundaries kept (unchanged by this slice)

- SRS, `ALTER INSTANCE`, T27, TiDB audited ALTER, identity-relation
  widening, stored-options reads, remote connectivity, FEDERATED
  viability, new public API/CLI flags, opaque revision markers, native
  error classes, duplicate/range oracles — all deferred per contract.
- Accepted CREATE identity: product commit
  `2afe8ecbb2c2df22015e8a17b7afafb320973826` with its docs-only child
  `28152dfd254f7786b77e7a7e656058c58ae9849d` — `08266403` survives
  only as pre-amend history. DROP semantics frozen. The sibling
  slices' product is untouched; their tests/evidence gained only the
  shared `statementsNeedMySQLServerPrivilege` widen to the alter kind,
  the nil-defaulted observe hooks (below), and shared README prose.
- P5/P6 native evidence remains optional follow-up — not run here.

## R2 product-logic delta (the only non-test change)

`metadata.go` gains a trailing-failure tail sweep at the end of
`batchState.enrichStatements`: a parse failure record positioned after
the last statement is still a real batch failure and now marks
`state.contaminated` at loop end. R1 only marked contamination when a
failure preceded a still-unprocessed statement, so `ALTER; GARBAGE(`
never tripped the flag — the AM-8.1 two-call contract requires the
call-1 record to contaminate call 2. Semantically any failure anywhere
makes the batch's whole-effect unknowable, so the sweep is the honest
completion of the existing per-statement check; mid-batch failures
were already consumed inside the loop and are unchanged.

## R3 product-logic delta (test-observation seam only)

`batchState` gains two nil-defaulted hooks — `afterBindObserve` and
`afterApplyObserve` — fired per statement inside
`enrichStatements` between the completed bind/adjudication attach and
the statement's apply, and immediately after that apply. They are nil
in production and change no behavior; they exist because AM-6.6/10.2
require metadata snapshots at the REAL post-bind/pre-apply and
post-own-apply timepoints, which no external seam can observe
(`enrichSlice` returns post-apply only). The R2 trailing-failure
contamination sweep stays — it has contract basis (AM-8.1) and R3 did
not reopen it.

## R3 errata / honest gaps

- Contract AM-9.3d's literal `observed product postgres` is
  unexpressible: `spec.ParseObservedVersion` derives product from the
  banner itself — its only constructions are `VersionProductTiDB`
  (TiDB marker) and `VersionProductMySQL` (numeric prefix); no input
  yields product `postgres`, so `ErrDialectProductMismatch` is
  reachable only via a recognized cross-product banner. **R4 ruling:
  erratum authorized.** The TiDB-observed banner carries the typed
  `ErrDialectProductMismatch` witness; the concrete
  `"PostgreSQL 16.1"` banner outcome is registered separately
  (`TestT26AlterAM93dPostgresBannerBoundary`: unestablished version
  fact → `gate:"version"` boundary, L=0). Contract case-matrix carries
  the in-place erratum record; the source excerpt proving the
  two-product vocabulary ships with R4.
- Segmented-fixture loader counts are registered separately from the
  frozen single-request L=1 wherever slicing doubles the calls
  (AM-7.5 internal L=2 segmented — the auxiliary witness; whole-batch
  fixtures and AM-10.3 carry L=1).

## R1 archive errata (historical evidence preserved, statistics corrected)

- Spec truth table: 26 rows, not 25 (R2 adds two seeded rows → 28).
- R1 six-package log: `--- PASS` at all indent levels = 7,772; lines
  containing PASS = 7,778; `--- SKIP` = 1; FAIL = 0; six packages `ok`.
- R1 full-suite log: 39 `ok` packages plus 5 `[no test files]`
  packages, FAIL = 0.
- The three zero-byte CLI streams (`alter-indomain.err`,
  `alter-nopin.err`, `alter-badpin.out`) exist as real 0-byte files;
  they are redelivered inside the R2 archive so their absence cannot
  be misread as verified.
- "Byte-identical snapshot" now applies only where complete
  independent snapshots are actually asserted (ordered channel,
  table/schema registries, metadata objects).
