# T26-SRS-VERSION-APPLICABILITY-01 — unified version gate for the SRS family

## Decision

One request-level version fact now controls BOTH SRS audit admission and SRS catalog-read admission (frozen V1). Previously the application gate (`MySQLSRSDropAuditedDomain`, registered MySQL 8.0.4+) and the provider gate (`mysqlSRSCatalogSupport`, proven MySQL Major>=8 with `!proven` still issuing reads) diverged: boundary statements were marked unaudited yet the provider still fired existence/conflict/dependency queries.

`bindMySQLSRS` now runs `spec.MySQLSRSVersionGateReason(resolvedVersion, observed)` before the resolver: outside the registered domain the bind returns the version reason as `UnreadReason` and **zero** SRS catalog reads fire — the provider's loader is never invoked, so its own version check becomes defense-in-depth reachable only inside the approved domain. The provider's public supply contract was NOT modified (the frozen B2 STOP trigger was not reached).

## Public contract

- `UnsupportedDetail.metadata` on the SRS version gate gains `version_reason` (one of `version_inapplicable` / `version_unproven` / `version_out_of_series` / `version_observation_error`) and `gap_fact` (`srs.version_applicability`, the named applicability premise, frozen V4/V11). `gate:"version"` is unchanged.
- `MetadataUnread*` gains three tokens: `version_unproven`, `version_out_of_series`, `version_observation_error` (frozen V2/V10).

## Non-goals / deferred

- MySQL 8.4 audited-domain widening — separate future work order.
- SRS dependency absent arm; adjudication/conflict-identity/provider-SQL/grammar changes; generic version framework; T25/SERVER/ALTER INSTANCE; T27.
- `srsDepsStale` semantics unchanged (D3-0 family exclusion stays closed).

## Own-apply

Unchanged fail-closed: boundary writes mint `srsExistenceStale` + ordered barrier flip; no positive mints off-domain (frozen V12).

## Verification

- `internal/application/audit/t26_srs_version_gate_test.go` — 9-cell version matrix + observation-error/mismatch/TiDB/static-preservation cells + six own-apply/batch cells.
- Prior boundary tests updated to the frozen contract (zero reads where they previously asserted the dual-track read).
- `go test ./...`, `go vet ./...`, gofmt clean.

Evidence: accepted Phase 0 archive `artifacts/t26-srs-version-applicability-phase0-r2.tar.gz` (SHA-256 b3410f2a…); contract docs under `docs/verification/t26-srs-version-applicability-01/`.

## ER1 correction (reviewer CHANGES REQUIRED, R2)

The caller `target_version` pin is a conflict check only and never
substitutes for a provider observation. `MySQLSRSDropAuditedDomain`
and `MySQLSRSVersionGateReason` now require
`Source == VersionSourceObserved`: any non-observed identity —
including a valid in-domain pin — classifies as `version_unproven`
with zero SRS catalog reads. The global `resolvedVersion` pin fallback
for other objects (T24-accepted) is unchanged; the scoping lives inside
the SRS gate. Tests now stamp `Source: observed` on literals modelling
proven banners, and the former providerless-pin S2 cells assert the
`version_unproven` boundary. ER2 (provider-internal O2 observation)
awaits a technical-proposal adjudication before any Provider contract
change.

## ER2 correction (reviewer-approved O-E/O-A design, R3)

One request must use one authoritative identity from version
observation to catalog query. The approved design binds O1 into every
SRS read and demotes the provider-internal observation (O2) to a
consistency witness:

- `spec.ObjectLookupRequest` gains the additive
  `RequestVersion *VersionIdentity` field — `bindMySQLSRS` stamps the
  request's `resolvedVersion` on every SRS object lookup (existence,
  NAME conflict, ORGANIZATION+ID conflict all ride one request), and
  `enrichOrderedStatements` passes the same `resolvedVersion` into
  `MySQLSRSDependencyReader.ResolveMySQLSRSDependencies(ctx, srid,
  version)` for ordered SRS DROP dependency reads. Non-SRS consumers
  see nil and keep prior behavior.
- `loadMySQLSRSObject` adjudicates O1 first: nil O1 →
  `version_unproven` with zero queries (not even the version probe);
  off-domain O1 → the exact gate reason with zero queries. Only an
  in-domain O1 reaches `mysqlSRSChannelVersionWitness`, which issues
  `select version()` and admits catalog reads solely when O2's parsed
  product+version equals O1's. `sql.ErrNoRows`, an unparseable
  banner, or a disagreement fails closed with
  `ErrSRSChannelVersionInconsistent`; a real query error propagates
  wrapped — an O2 observation error can never surface as an empty
  catalog answer. `Absence` is now supplied from O1
  (`MySQLSRSDropAuditedDomain(o1)`), never from O2.
- `ResolveMySQLSRSDependencies` applies the identical order: nil or
  off-domain version → nil fact and zero queries; in-domain → witness
  then dependency read.

### Public contract

- `ErrSRSChannelVersionInconsistent` — the bounded consistency error
  marking proxy/topology split or backend drift; exported for
  `errors.Is` assertions. It is never an adjudication input.
- `MySQLSRSDependencyReader` signature gains `version
  *spec.VersionIdentity` (the approved additive supply binding —
  limited to the SRS request supply contract, not a generic version
  framework).

### Verification

- Provider matrix `TestT26SRSR3ChannelWitnessMatrix`: O1=8.0.46 ×
  O2 {8.0.46 match; 8.4.10/5.7.44 mismatch; unparseable; empty;
  query-error} — witness fires once, zero catalog reads on failure,
  no existence/dependency fact minted.
- `TestT26SRSVersionGate` missing-O1 cell: zero queries of any kind;
  off-domain O1 cells (5.7.44, 8.0.3, 8.4.x) name the exact reason.
- Application cells: mid-request O2 drift (`versionScript`)
  propagates the bounded error while stmt1's ordered entry survives;
  own-apply stale/barrier cells unchanged.

## R4 correction (reviewer ER2: pinned channel connection)

`*sql.DB` is a connection pool. Under R3 the O2 witness ran on
`p.db.QueryRowContext` and the following catalog reads on
`p.db.QueryContext`, so a proxy or multi-backend topology could land
the witness on one backend and the reads on another — the consistency
check would be vacuous. R4 pins each read unit to one `*sql.Conn`
acquired via `p.db.Conn(ctx)`.

- `loadMySQLSRSObject` keeps the O1 gate ahead of `p.db.Conn`: a nil
  or off-domain O1 still closes with zero queries and never acquires a
  connection at all. An in-domain O1 pins one conn, then the O2
  witness, existence, qualified conflict, and dependency reads all run
  on it (`loadMySQLSRSObjectOnConn`).
- `ResolveMySQLSRSDependencies` pins one conn per call for the witness
  plus dependency read — no generic shared channel, each read unit
  keeps its own backend proof.
- Both paths release the conn on success and on failure; a release
  error on a clean read propagates wrapped
  (`release mysql srs channel connection`).
- The `srsChannelQuerier` interface (`QueryRowContext` +
  `QueryContext`) is the minimal query surface so helpers keep taking
  the pinned conn, and the stub driver can observe per-connection
  identity.

### Verification (R4)

- `provider_t26_r4_pinned_test.go`: a connection-counting stub driver
  that hands out a distinct connection ID per `*sql.Conn` and can
  answer `select version()` differently per backend proves — match
  shares one conn ID across witness + catalog reads; mismatch and
  unparseable O2 fail closed after exactly one witness query with zero
  catalog reads; missing/off-domain O1 acquires zero connections;
  `ResolveMySQLSRSDependencies` shares its conn between witness and
  dependency read; both units release on success and on failure; and
  the topology-split cell (banner differs per backend connection)
  shows the witness only vouches for its own pinned backend.
