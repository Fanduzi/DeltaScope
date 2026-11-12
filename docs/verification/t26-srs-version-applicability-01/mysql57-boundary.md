# MySQL 5.7 boundary — SRS family

## Catalog reality

`information_schema.ST_SPATIAL_REFERENCE_SYSTEMS` does not exist on MySQL 5.7. The spatial surface on 5.7 is `ST_GEOMETRY_COLUMNS`-shaped column metadata only (SRID per column, no named SRS objects, no `CREATE/DROP SPATIAL REFERENCE SYSTEM` — the syntax itself is an 8.0 addition).

## Product behavior (source review)

A proven 5.7.x banner resolves `resolvedVersion` with `Product=mysql, Major=5, Minor=7, ValidatedRange=true`:

- **Application gate**: `MySQLSRSDropAuditedDomain` requires `Major==8 && Minor==0 && Patch>=4` → false → every SRS statement gets `Unsupported{reason:"unaudited", gate:"version"}`. No adjudication, no findings, no apply mints.
- **Provider gate**: `mysqlSRSCatalogSupport` returns `supported=false, proven=true` (Major<8) → `loadMySQLSRSObject` returns `MetadataUnreadVersionInapplicable` and issues **zero** catalog queries (:121-131).
- **Privilege premise**: `MySQLServerDropPrivilegeDomain` requires 8.0.x → SRS privilege premises stay `Unproven` — moot, since adjudication never runs.

## Consistency verdict

5.7 is the one cell where both gates agree: statement is boundary **and** no reads fire. Proven-too-old maps to a dedicated unread reason (`version_inapplicable`), not to generic unknown — satisfying the reviewer's rule that proven-inapplicable must not collapse into unknown.

## Residual gap

The public surface shows the statement-level unaudited marker (`gate:"version"`); there is no named `srs.version_applicability` premise gap distinguishing "proven 5.7 — feature absent" from "version unproven" at the finding level. Both render as unaudited boundary. Contract clause V5/V11 proposes the explicit mapping.
