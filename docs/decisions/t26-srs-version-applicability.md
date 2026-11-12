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
