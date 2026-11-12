# T26-SRS-CONFLICT-IDENTITY-01 — MySQL identity / collation boundary (E-section)

## What is established from accepted evidence

| Fact | Source |
|---|---|
| Unique key `SRS_NAME` exists; insert-time 1062 `Duplicate entry '1-WGS 84'` | accepted archive `t26-srs-create-publication-phase0-r1` → `out-09-conflict-name.txt` |
| Composite unique key `ORGANIZATION_AND_ID` exists; 1062 `'1-EPSG-4326'` | same archive → `out-10-conflict-orgpair.txt` |
| Conflict errors are engine-integrity (unique-key) violations surfacing at insert AFTER the validation chain (1064>3709>3714>3708>3631>exists>3517>1062/3517 ordering per guard-oracle R3) | `t26-srs-create-guard-oracle-01` phase0 accepted docs |
| `information_schema.ST_SPATIAL_REFERENCE_SYSTEMS` exposes `SRS_ID`, `SRS_NAME`, `ORGANIZATION`, `ORGANIZATION_COORDSYS_ID` — the columns the provider queries (provider_server_srs.go:172,249,259) | source |

## `native oracle pending` — NOT established this round

The work order forbids new Docker/native probes this round; no accepted archive captured:

| Item | Status | Why it matters |
|---|---|---|
| `ST_SPATIAL_REFERENCE_SYSTEMS.SRS_NAME` declared type/collation | **native oracle pending** | whether `=` is `ci`/PAD-collated; if ci, authored-case-only differences would conflict natively — the param query matches by DB rule so direction stays consistent either way |
| `.ORGANIZATION` type/collation | **native oracle pending** | same reasoning |
| `.ORGANIZATION_COORDSYS_ID` type (unsigned int expected) | **native oracle pending** | provider binds int64 of decimal — overflow semantics assume u32 |
| 5.7 vs 8.x catalog shape (ST_SRS exists ≥8.0.3) | **native oracle pending** | provider's own version gate requires MySQL ≥8 (`mysqlSRSCatalogSupport`), so 5.7 lands `version_inapplicable` — consistent even without the column capture |
| Lower_bound/upper_bound column widths (srs_name varchar(80)? org varchar(256)?) | **native oracle pending** | overlong authored names under-match (safe direction) |
| Behavior of `srs_name` comparison under PAD SPACE | **native oracle pending** | expected PAD-collated; either way comparison runs DB-side |

## Known-safe by construction (no native capture needed)

- **Case/case-fold**: the product never folds NAME/ORG — the DB performs `=` under the column collation both for the insert (unique key) and for the query (predicate). Any collation the DB applies is applied identically to both sides; the authored spelling and the bound parameter are the same bytes.
- **Non-ASCII**: byte fidelity is what matters — the parameter carries the authored bytes verbatim and MySQL's own collation decides equality. No Go-side `lower()` is ever applied to these values.
- **Empty string**: `''` decoded → `TrimSpace(name)==""` → provider-side guard skips the read → premise owed. Native `NAME ''` behavior (validation refusal vs stored empty row) — the product never claims either.
- **NULL**: the grammar only admits `tokStr` — NULL cannot reach the qualifiers.

## Boundary statement

The dangerous divergence is NOT collation — it is **decode fidelity** (see trusted-identity-analysis.md): whether the bytes the product binds equal the bytes MySQL stores. Collation correctness is delegated to the DB engine by binding the authored bytes; decode correctness is the product's own obligation and is the only place a wrong identity can be minted.
