# MySQL 8.4 applicability — SRS family

## Registered domain says: boundary

`MySQLSRSDropAuditedDomain` requires `Minor==0` → proven 8.4.x takes `gate:"version"` unaudited boundary, even though MySQL 8.4 supports SRS DDL natively and was native-probed (8.4.10) in the guard-oracle Phase 0 rounds.

## Provider gate says: supported

`mysqlSRSCatalogSupport` uses `Major>=8` → proven 8.4.x issues existence, qualified conflict, and dependency reads and returns real catalog facts. The bound state attaches to `Metadata.SpatialReference` but no adjudication consumes it.

## Divergence (the central 8.4 finding)

Statement = unaudited boundary; reads still fire and land in metadata. Consequences:

- The audit reports "outside audited domain" while having silently queried the catalog — cost and semantics disagree.
- The boundary write still mints `srsExistenceStale` — later same-SRID statements degrade honestly. Fail-closed, no false facts.
- Conflict-identity qualifiers still ship (bind is version-blind); the reads just never adjudicate.

## Evidence base

8.4.10 native probes exist for guard semantics (guard-oracle Phase 0 accepted) but the audited slice was never registered for 8.4 — a deliberate scoping decision, not an omission discovered here. The SRS feature's first-version claim (8.0) does not extend: the product's audited domain, not the server's capability, defines the boundary.

## Contract relevance

V7: 8.4 stays boundary under the current registered domain; widening to include 8.4 is a separate future work order, and this Phase 0 neither proposes nor blocks it — it records that read issuance and audit admission disagree on this surface today.
