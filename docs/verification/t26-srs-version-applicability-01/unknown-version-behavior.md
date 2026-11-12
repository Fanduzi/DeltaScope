# Unknown-version behavior — SRS family

"Unknown" here means `resolvedVersion==nil` (no provider / unparseable banner / banner absent) **or** a non-nil identity with `ValidatedRange=false` (e.g. a syntactically valid 9.x, or TiDB-banner parse edge). Both fail `MySQLSRSDropAuditedDomain` identically — the gate reads presence/absence and validity, never a default.

## Statement-level outcome

Every SRS statement → `Unsupported{reason:"unaudited", object:"spatial_reference_system", gate:"version"}` (`metadata.go:293-305`). No adjudication, so no exists_noop, no outcome_unproven, no refusal findings — the guard-correction arms are unreachable, never regressed.

## Read behavior under unknown version — the load-bearing finding

`mysqlSRSCatalogSupport` returns `supported=false, proven=false` for an unproven banner, and `loadMySQLSRSObject` deliberately **does not skip** reads in that branch (:123-130 comment: an unproven version never launders a failed query into "not loaded"). So under unknown version the provider still issues:

- existence query (`ST_SPATIAL_REFERENCE_SYSTEMS` by SRID),
- NAME conflict query when the decode-complete qualifier is present,
- ORGANIZATION conflict query on the composite qualifier,
- dependency query (`ST_GEOMETRY_COLUMNS.SRS_ID`).

All query/scan/iteration/close errors propagate — unknown version can therefore turn an audit into a hard provider error (e.g. actually-5.7 server lacking the catalog), not a bounded gap.

## Own-apply / batch effects

- Boundary write still mints `srsExistenceStale[SRID]` (`batch_state.go:535`) — fail-closed, correct.
- No present mint, no ordered entry, no publication: adjudication is unreachable.
- Later same-SRID statements read `batch_effects` unread — honest degradation, never silent staleness.

## What unknown version does NOT do

- Does not upgrade ineligible NAME/ORG spellings into query eligibility (decode-complete flags are parse-time, version-blind).
- Does not mint `srsExistencePresent`.
- Does not produce a named `srs.version_applicability` gap — today it surfaces only as the generic unaudited marker (contract V2/V11 addresses the vocabulary).

## TiDB / other vendors

`bindMySQLSRS` early-returns for non-MySQL dialect — no reads at all; vendor boundary is a dialect fact, not a version fact. A TiDB banner observed via O1/O2 yields `Product=tidb` → fails the MySQL gates cleanly rather than inheriting them.
