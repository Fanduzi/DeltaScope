# T26-SERVER-ALTER-EXECUTION-01 — Execution Evidence Index

Work order: `T26-SERVER-ALTER-EXECUTION-01 / R2` — R1 was ruled
CHANGES REQUIRED (test witnesses, assertion precision,
observation-layer mapping, and evidence reconciliation incomplete);
R2 supplies the remediation without a product redesign. R1's raw
evidence stays preserved; R1 historical statistics carry the errata
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
  unaudited with `gate:"version"` before the family privilege loader;
  the keep-boundary-bind order still issues its one catalog read.
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
| premise independence (AM-2.1–2.3) | `audit.TestT26AlterPrivilegeProjection`, `TestT26AlterAM23FrozenPublicProjection` (frozen per-ALTER projection: main unverified + fixed advisory + exact `{server.alter_privilege}` gap + unverified coverage + L=1/R=1/S=1) |
| A3/A4 publish + republish (AM-1.x, AM-2.4i/2.5i, AM-6.x) | `audit.TestT26AlterProvenRepublish`, `TestT26AlterConditionalAxesProjection`, `TestT26AlterInheritedRepublish`, `TestT26AlterZeroRowNoState`, `TestT26AlterSiblingUnchanged`; R2 adds `TestT26AlterAM24iAM25iSuppressedChains` (explicit own-premise `Suppressed`), `TestT26AlterAM12ModeOff`, `TestT26AlterAM14AM15ExactUnknown`, `TestT26AlterAM61OrderedConsume`, `TestT26AlterAM62ZeroPair`, `TestT26AlterAM64AM65NoShadow` |
| A1 uniform no-op (AM-7.x, AM-8.x) | `audit.TestT26AlterRelationBarrierNoOp`, `TestT26AlterChannelTaintBarrier` (both re-anchored to COMPLETE ordered-channel snapshots — entries, writes and taint — taken across the ALTER's own apply), `TestT26AlterRealContamination`; R2 adds `TestT26AlterAM81TwoCallContamination`, `TestT26AlterAM82LoaderBeforeFailure`, `TestT26AlterAM76OrderedPresentFinalConsumer`, `TestT26AlterAM73aDropOwnBarrier`, `TestT26AlterAM73bOwnReadExistence`, `TestT26AlterAM73ApplyOwnership`, `TestT26AlterAM71AM72CaseAlias`, `TestT26AlterAM74LiteralVsOverlong`, `TestT26AlterAM75LiteralClearsStalePresence` |
| lifecycle chains (AM-4.x, AM-5.x) | `audit.TestT26AlterAM41AbsentSurvivesRefusal`, `TestT26AlterAM42ModeOffBarrier`, `TestT26AlterAM43LiteralDropTaint`, `TestT26AlterAM51FullChain` |
| metadata lifecycle (AM-6.6, AM-10.2) | `audit.TestT26AlterAM66MetadataLifecycle`, `TestT26AlterAM102DropConsumesMetadataStable` — three-timepoint independent deep copies of `Metadata.Server` + `Metadata.ServerAlter` |
| channel isolation (AM-10.3/10.4) | `audit.TestT26AlterTableChannelIsolation` (deep-rendered complete-state snapshots with a pre-seeded populated entry), `TestT26AlterChannelTaintBarrier`, `TestT26AlterAM104LiteralModeOff` |
| public redaction + advisory (AM-10.1) | `audit.TestT26AlterPublicRedaction` (complete result serialized and scanned — RawSQL and NormalizedSQL both exactly `ALTER SERVER <redacted>`), rule `TestMySQLAlterServer*` (7 tests) |
| error boundaries (AM-9.4/9.5) | `deltascope.TestT26AlterAM94ResolverSentinel` (L=1/R=1/S=0, `errors.Is` preserved), `deltascope.TestT26AlterAM95LoaderSentinel` (L=1/R=0/S=0), `audit.TestT26AlterLoaderErrorBoundary` |
| CLI probes | three recorded probes below — in-domain pin, no pin, malformed pin |

## Verification summary (all commands logged; logs delivered as attachments)

- Six-package verbose gate (`spec`, `audit`, `rule/ddl`, `parser/tidb`,
  `policy`, `pkg/deltascope`): `go test -count=1 -v` — rc=0, 7,756
  `--- PASS` lines at all indent levels, 0 FAIL.
- Full repository suite (`go test -count=1 ./...`): rc=0, 35 `ok`
  package lines, 0 FAIL.
- `go vet` on the six packages: rc=0. `git diff --check`: rc=0.
  `gofmt -l` on every file in the diff: clean.
- Binary: built from this exact source state with
  `go build -buildvcs=false -o deltascope ./cmd/deltascope`, Go 1.26.1
  darwin/arm64, CGO_ENABLED=0; identity recorded in
  `build-record.txt` (delivered attachment).

## Three CLI probes (frozen expectations, contract AM-10 probe plan)

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
- Accepted CREATE R4 / DROP semantics frozen; the sibling slices'
  product, tests, and evidence are untouched except the shared
  `statementsNeedMySQLServerPrivilege` widened to the alter kind and
  shared README prose.
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
