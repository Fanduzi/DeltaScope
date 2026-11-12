# Source facts — T26-SRS-VERSION-APPLICABILITY-01 Phase 0

Baseline: `41ea7895a1df62df924fe4f3576bbf54f72c5576` (tree `b14f910ea6c1f71073e9c1d871d37afd772aecce`, parent `9ac5330c10e4d81fa7ff3eb1de285907963fa278`), branch `milestone/mysql-tidb-ddl-completion`, tracked tree clean. All findings are source review only — no product change, no new native probes.

## A. Two independent version observation points

1. **Request level** (`internal/application/audit/metadata.go:161-176`): `spec.ParseObservedVersion(instanceFactVersion(instanceFacts))` → `resolvedVersion`. The observed `select version()` banner is the only authoritative online fact; product is derived from the banner, never the requested dialect. `ErrDialectProductMismatch` and `ErrTargetVersionMismatch` are typed input errors. Unparseable banner → `resolvedVersion=nil`.
2. **Provider internal** (`internal/infrastructure/metadata/mysql/provider_server_srs.go:106-118`): `mysqlSRSCatalogSupport` runs its own `select version()` → `spec.ParseObservedVersion`. Request-supplied/target versions never substitute (comment at :100-105).

## B. Version identity contract (`internal/domain/spec/version.go`)

- `VersionIdentity{Product,Version,Major,Minor,Patch,Source,ValidatedRange}`; raw banner never carried into public projections (:12-17).
- Validated series = MySQL 5.7.x / 8.0.x / 8.4.x and TiDB 8.5.x (`VersionInValidatedSeries` :65-74).
- `ParseTargetVersion` requires strict `[v]M.m.p`; out-of-series but well-formed → `ValidatedRange=false`, not an error (:82-106, :108-121).
- `ParseObservedVersion`: `-TiDB-v` marker → product=tidb real version; otherwise leading M.m.p → product=mysql (:132-162).

## C. Two gates, two different domains

| Gate | Location | Domain | Off-domain result |
|---|---|---|---|
| Application audited-slice | `metadata.go:293-305` via `spec.MySQLSRSDropAuditedDomain` (`srs_drop.go:135-140`) | MySQL + ValidatedRange + **8.0.x && patch>=4** | statement.Unsupported{reason:unaudited, gate:"version"} |
| Provider catalog read | `mysqlSRSCatalogSupport` (`provider_server_srs.go:106-118`) | proven MySQL + ValidatedRange + **Major>=8** | proven&&!supported → `version_inapplicable`, zero reads; **!proven → reads still issue** |

- SRS CREATE family inherits the drop's audited domain verbatim (frozen F-2 comment, `metadata.go:288-295`).
- `bindMySQLSRS` (`batch_state_server_srs.go:134-219`) is invoked for **every** MySQL SRS declaration including version-gated boundary ones (`metadata.go:761`), and does not consult version except inside `orderedSRSDropEligible` (:231-237). The resolver call at :212 therefore fires for boundary statements.
- Adjudications run only when `Unsupported==nil` (`metadata.go:778` SRSDrop, :794 SRSCreate) — a boundary statement never gets an arm, so `applySRSStatement`/`applySRSCreateStatement` mint nothing from it (`batch_state_srs.go:86-90,152-154`).

## D. What fires under version unknown / out-of-domain

- Unparseable or unvalidated banner → `resolvedVersion` nil or non-validated → **every** SRS statement takes the unaudited `gate:"version"` boundary, yet `bindMySQLSRS` still calls the resolver; provider `!proven` → existence + qualified conflict + dependency queries all still run. The bound `SpatialReferenceSystemState` attaches to `Metadata.SpatialReference` (:770) but no adjudication consumes it.
- MySQL 8.4.x (proven, validated, Major>=8): application gate → boundary (requires Minor==0); provider → reads run. Divergence.
- MySQL 8.0.0–8.0.3 (proven): application → boundary (patch<4); provider → supported → queries fire against a catalog surface the audited domain declined to certify.
- MySQL 5.7.x (proven): application → boundary; provider → `version_inapplicable`, no reads. Consistent.
- TiDB: `bindMySQLSRS` early-return at dialect check (:137-139) → no reads; vendor boundary.
- Offline (no provider): `resolver==nil` → bound empty state (:185-188); statement still boundary (nil resolvedVersion).

## E. Own-apply and batch effects under boundary

- Boundary SRS write mints `srsExistenceStale[SRID]` unconditionally (`batch_state.go:520-536`) — fail-closed: the write may have landed, so later same-SRID reads get `batch_effects`. No present mint, no ordered entry, no barrier delete.
- `mintSRSExistenceStale` (`batch_state_srs.go:54-60`) clears present mint and flips ordered barrier.
- Ordered consumption additionally requires the audited domain (`orderedSRSDropEligible` :236).
- `spatialTouching` D3-0 exits on the whole SRS family — version-gated SRS writes never mint `srsDepsStale` (:179-183).

## F. Privilege premises share the SERVER observation, with their own version domain

- `MySQLServerDropPrivilegeDomain` = MySQL && 8.0.x (no ValidatedRange check) (`server_drop_premise.go:218-222`).
- `srsDropPrivilegePremise`/`srsCreatePrivilegePremise` project that witness; the adjudications only run for audited statements anyway, so a privilege premise is never computed for a boundary statement.

## G. Version-marked vocabulary that exists today

- Statement boundary: `UnsupportedDetail{reason:"unaudited", metadata:{boundary:"unaudited",object:"spatial_reference_system",gate:"version"}}`.
- Provider unread reason: `MetadataUnreadVersionInapplicable = "version_inapplicable"` (metadata.go:738-741).
- **No `srs.version_applicability` premise fact exists** — the gap is a statement-level marker, not a named premise; the reviewer's C3 requirement for an explicit evidence gap is a contract item (see proposed-contract V4/V11).
