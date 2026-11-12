# T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01 — R1 implementation + verification

Round: **R1 — IMPLEMENTATION + VERIFICATION** against the accepted
contract at `0fb1b7ff` + evidence supplement `f206060d`
(`docs/verification/t26-srs-drop-proven-absent-contract-01`, R4 ACCEPTED;
issue #105, milestone T26/#105). No push, merge, tag, release, or issue
closure is claimed or authorized by this receipt.

## Identity

- Candidate commit: `9fb5d3dc0b1f8194eadf9b5dd5939b1301766f19`
- Candidate tree: `a0b4d160c5009962b5a0b280e114f3ecaa7731bd`
- Parent: `0fb1b7ff` (contract R4 closeout — `.go` scope untouched there)
- Branch: `milestone/mysql-tidb-ddl-completion`
- cwd `/Users/fan/GolangProjects/DeltaScope`; toolchain `go1.26.1
  darwin/arm64`; `CGO_ENABLED=1`, `-tags postgresql` (Makefile recipe)

## Scope executed

- `internal/domain/spec/srs.go`: `SpatialReferenceSystemState.Absence` —
  the supplier-certified absence bit; `Read && !Exists` alone never
  upgrades.
- `internal/domain/spec/srs_drop.go`: `ValidAbsence` bound to the
  authored SRID; `AdjudicateSRSDrop` gains `ifExists`; the frozen order
  B → S0 → S1 → A → S2 adds `absent_refused` / `absent_noop` /
  `absent_conditional` (owes exactly `srs.drop.privilege`).
- `internal/infrastructure/metadata/mysql/provider_server_srs.go`:
  `mysqlSRSCatalogSupport` returns the parsed Qo identity; the existence
  read switches to explicit `*sql.Rows` — `Absence=true` only on a clean
  zero-observed-row answer under the same Qo observation's
  `MySQLSRSDropAuditedDomain` proof; `sql.ErrNoRows` anywhere is an
  error; conflict + dependency reads gain checked `rows.Close()`.
- `internal/application/audit/metadata.go`: `decl.IfExists` passed as the
  fifth argument.
- `internal/application/audit/batch_state_srs.go`: doc only — all A arms
  mint/clear nothing; only `SRSDropArmUnproven` mints.
- Rules: `ddl.mysql.srs.drop.absent` (blocker) +
  `ddl.mysql.srs.drop.absent_noop` (notice) registered
  (`common.go`/`register.go`/`mysql_srs_rules.go`), defaults in
  `internal/domain/policy/defaults_ddl.go`, example config; dependency
  gap `unproven_privilege` on the conditional arm; reserved advisory
  widened to `absent_noop`/`absent_conditional`.
- `internal/application/audit/corpus_coverage_test.go`: both new rules
  defer corpus coverage — a providerless corpus cannot supply
  certified absence (same mechanism as `srs.drop.privilege`).
- Stub/instrumentation: `testQueryResult.closeErr`/`testRows` and the
  mirror `srsSQLRows.closeErr`/`closeOn` channel; mirror `loadSRS`
  resynced to the real supply predicate.
- Test fixtures: legacy zero-row cells pinned to Qo `8.0.3` /
  `8.0.3-log` (supported, outside the proven-absence domain) so the
  unknown-branch contract stays under test.

## Gates (all at `9fb5d3dc`)

- Seven-package `go test -count=1 -v ./internal/domain/spec
  ./internal/application/audit ./internal/domain/rule/ddl
  ./internal/infrastructure/parser/tidb ./internal/domain/policy
  ./pkg/deltascope ./internal/infrastructure/metadata/mysql` — rc 0;
  **8,362 PASS / 1 SKIP / 0 FAIL** counted across all indentation; 7 `ok`
  package lines. Raw `gates/gate-verbose.log`.
- `go test -count=1 ./...` — rc 0; 39 `ok` packages. Raw
  `gates/full-suite.log`.
- `go vet` same seven packages — rc 0, 0-byte output. `gates/vet.log`.
- `gofmt -l` over the 19 cumulative changed `.go` files
  (`gates/gofmt-filelist.txt`) — clean, 0-byte output. The pre-existing
  unformatted `internal/infrastructure/parser/tidb/mysql_t26r5_test.go`
  is NOT touched by this chain (tracked, unchanged; repo-wide
  observation only — registered erratum, out of scope).
- `git diff --check 0fb1b7ff..HEAD` — rc 0. `gates/diff-check.log`.

## Build + rebuild

- `make build-cli` twice → identical bytes:
  SHA-256 `df4db54a61f6bfd39256be0c437ef6cba0106e54610a5a405b14e0b7e677021b`,
  52,761,890 B. `probes/build1.sha`, `probes/build2.sha`, cmp identical.

## Providerless CLI probes (new binary; contract PA-10g reference shape)

Policy `probes/probe-policy.yaml` enables all six SRS drop rules at
their required levels; SQL `DROP SPATIAL REFERENCE SYSTEM 100001;`,
`--dialect mysql --format json --fail-on none`, providerless,
privilege mode off. Streams `probes/cliN.stdout|.stderr|.rc`.

| probe | tail | rc | stdout | stderr | expected | actual |
|---|---|---|---|---|---|---|
| CLI-1 | `--target-version 8.0.36` | 0 | 896 B | 0 B | review; unverified; 0 findings; 1 `unknown_srs_state` gap `[srs.existence,srs.column_dependencies,srs.drop.privilege]` | exact match |
| CLI-2 | (no pin) | 1 | 1152 B | 0 B | boundary `{unaudited,gate:"version",object:"spatial_reference_system"}` + `unsupported_statement` diagnostic | exact match |
| CLI-3 | `--target-version 8.0.x` | 2 | 0 B | 56 B | stderr exactly `target_version must match [v]MAJOR.MINOR.PATCH: "8.0.x"`\n | exact match |

Providerless probes cannot exercise supplier-certified absence — that is
by design (reference only, per contract PA-10g); the proven-absence
public shape is proven by the online/API PA matrix.

## Case-matrix mapping (PA rows → tests)

- PA-1a providerless `[E,D,P]`:
  `TestT26SRSAbsentProviderlessStaysUnproven`
- PA-1b/PA-2a plain absent → `absent_refused` + counts:
  `TestT26SRSAbsentPlainRefused`
- PA-1c/PA-8b Qo `unparseable` → S2 `[E,D]` + notice:
  `TestT26SRSAbsentUnprovenQoStaysUnproven`
- PA-2b IF EXISTS consumable → `absent_noop` + marker:
  `TestT26SRSAbsentIfExistsNoop`
- PA-2c unproven → `[P]` under `unproven_privilege`:
  `TestT26SRSAbsentIfExistsConditional`
- PA-2d REVOKE suppressed → same gap:
  `TestT26SRSAbsentIfExistsSuppressed`
- PA-3a present → S2 `[D]` + notice: `TestT26SRSAbsentExistsStaysUnproven`
- PA-4a S1 precedence (zero-row exist + positive deps): reused —
  `TestT26SRSR3ZeroRowPositiveDepsRefuses`
- PA-4b IF EXISTS variant: `TestT26SRSAbsentIfExistsDependencyRefused`
- PA-4c prefix-minted `srsDepsStale` demotes witness → A-plain:
  `TestT26SRSAbsentDepsStaleDemotesWitness`
- PA-4d S0: reused — `TestT26SRSR3SRID0Request`
- PA-4e/4f/4f2 + valid/IF-EXISTS split + S1-outranks-absence unit rows:
  `TestT26SRSAbsentAdjSupplyDiscriminants`,
  `TestT26SRSAbsentAdjIfExistsSplit`
- PA-4e2 foreign-SRID custom resolver → S2 `[E,D]`, mint authored only:
  `TestT26SRSAbsentEnrichForeignSRID`
- PA-4g/4h `sql.ErrNoRows` at query/iteration → propagate:
  `TestT26SRSAbsenceQueryErrNoRowsPropagates`,
  `TestT26SRSAbsenceIterationErrNoRowsPropagates`
- PA-4i EOF after positive row → present:
  `TestT26SRSAbsenceEOFFterPositiveRow`
- PA-5a–PA-5f stale channels / contamination / mint behavior:
  `TestT26SRSAbsentStaleChannels` (six subtests)
- PA-6a/6b same/different SRID cache counts:
  `TestT26SRSAbsentSameSRIDCacheHit`,
  `TestT26SRSAbsentDifferentSRIDRereads`
- PA-6c boundary CREATE prefix → B, Δreads=0:
  `TestT26SRSAbsentBoundaryPrefixBlocks`
- PA-6d per-enrich cache recount: `TestT26SRSAbsentRecachesPerEnrich`
- PA-6e no-pin observed 8.0.3 boundary pair:
  `TestT26SRSAbsentBoundaryVersionPair`
- PA-7a REVOKE scoping three-point snapshots:
  `TestT26SRSAbsentRevokeScoping`
- PA-7b public REVOKE twin: `TestT26SRSAbsentRevokeScopingPublic`
- PA-8a no-pin observed 8.0.3 boundary: reused —
  `TestT26SRSR3E10Proven803` (single-statement twin of PA-6e)
- PA-8c pin/observed mismatch → `ErrTargetVersionMismatch`: reused —
  `TestT26SRSR2DiscoveryMismatch` (R2 E10.7/E10.8)
- PA-8d real TiDB parse → vendor boundary mint `{100001}`: reused —
  `TestT26SRSPrivP9cInternalVendorBoundaryMint`
- PA-9a/9b/9c query/iteration/cancel propagation:
  `TestT26SRSAbsenceQueryErrorPropagates`,
  `TestT26SRSAbsenceIterationErrorPropagates`,
  `TestT26SRSAbsenceCancellationPropagates` (+ checked-close twin
  `TestT26SRSAbsenceCloseErrorPropagates`)
- PA-9d capability minus `ObjectResolver` → S2 `[E,D]` + notice:
  `TestT26SRSAbsentNoResolverStaysUnproven`
- PA-9e unproven-Qo state literal `{Read:true,Exists:false,Absence:false}`:
  `TestT26SRSAbsenceNotSuppliedUnprovenVersion`
- PA-9f deps-read failure never publishes a partial snapshot:
  `TestT26SRSAbsenceDepsErrorPropagates`
- PA-10a no reserved advisory on `absent_refused`:
  `TestT26SRSAbsentRefusedNoReservedAdvisory`
- PA-10b/10c advisory co-fires on the other A arms:
  `TestT26SRSAbsentNoopReservedAdvisory`,
  `TestT26SRSAbsentConditionalReservedAdvisory`
- PA-10d/10e policy disable + level rewrite:
  `TestT26SRSAbsentDisabledRuleSilent`,
  `TestT26SRSAbsentNoopWarningLevel`
- PA-10f serialized-JSON sentinel scans:
  `TestT26SRSAbsentSentinelScan`
- PA-10g providerless CLI reference: the three probes above.
- In-domain supply: `TestT26SRSAbsenceSuppliedInDomain` (provider) +
  `TestT26SRSAbsentPlainRefused` (api).

## Documentation

- Module READMEs synced in-candidate: `internal/domain/spec/README.md`,
  `internal/domain/rule/ddl/README.md`,
  `internal/infrastructure/metadata/mysql/README.md`,
  `internal/application/audit/README.md`.
- Decision record: `docs/decisions/2026-11-12-mysql-drop-srs-proven-absence.md`.

## Errata / notes

- `internal/infrastructure/parser/tidb/mysql_t26r5_test.go` fails
  `gofmt -l` repo-wide at baseline `0fb1b7ff` (tracked, untouched by this
  chain) — registered observation only, unchanged per scope.
- REVOKE in an api-layer batch surfaces `ErrUnsupportedStatement` while
  every audited statement still projects (existing contract — PA-2d/7b
  assert it via `errors.Is`).
- `internal/application/audit/t26_srs_drop_r3_test.go` legacy zero-row
  cells moved their Qo banner to `8.0.3-log` (observed/discover banners
  unchanged) — the expected S2 arms are preserved, now for the right
  reason (outside the proven-absence domain).
