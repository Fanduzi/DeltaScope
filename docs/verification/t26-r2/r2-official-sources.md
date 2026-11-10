# T26-R2 official source table (pinned references)

Supersedes the R1 source section for citation completeness. R1 content is
preserved unmodified under `docs/verification/t26-r1/`; this document
records the fixed-URL citations, version columns, and evidence-type
split the reviewer required.

Legend for evidence type: **[DOC]** official manual text ·
**[WL]** worklog spec · **[RN]** release notes · **[SRC]** fixed-version
source (read-only) · **[PEND]** not yet established, needs native/source
forensics — never mixed into a [DOC]/[SRC] claim.

## 1. SERVER family

| fact | MySQL 5.7 | MySQL 8.0 | MySQL 8.4 | TiDB 8.5 | evidence |
|---|---|---|---|---|---|
| `CREATE SERVER` exists | yes | yes | yes | no statement | [DOC] refman `create-server.html` (8.0/8.4 URLs below; 5.7 edition exists at `/5.7/en/create-server.html` — **5.7 page not fetched this round**, see PEND-1) · TiDB: inventory `vendor_not_supported` + no TiDB doc entry **[PEND]** native confirmation |
| syntax `FOREIGN DATA WRAPPER w OPTIONS (...)` | same grammar family | yes | yes | — | [DOC] 8.0 syntax block |
| option domain: HOST/DATABASE/USER/PASSWORD/SOCKET/OWNER char-literal ≤64 chars silently truncated; PORT numeric 0–9999; OWNER unapplied | same option set per 5.7 manual family | yes | yes | — | [DOC] 8.0 options section |
| `server_name` global, **case-insensitive**, ≤64 chars silently truncated, may be quoted string | same per 5.7 family | yes | yes | — | [DOC] 8.0 "server_name" paragraph |
| stored as `mysql.servers` row; SUPER required; implicit commit; not binlogged | same | yes | yes | — | [DOC] |
| `ALTER SERVER s OPTIONS (...)` | yes | yes | yes | no | [DOC] `alter-server.html` |
| `DROP SERVER [IF EXISTS]` | yes | yes | yes | no | [DOC] `drop-server.html`; drop does not affect previously-created FEDERATED tables |
| wrapper value domain (`mysql` documented; FEDERATED linkage only if engine installed) | same | yes | yes | — | [DOC]; installing FEDERATED = out of scope |

URLs (8.0 edition; 8.4 mirrors under `/8.4/en/`):
- `https://dev.mysql.com/doc/refman/8.0/en/create-server.html`
- `https://dev.mysql.com/doc/refman/8.0/en/alter-server.html`
- `https://dev.mysql.com/doc/refman/8.0/en/drop-server.html`
- 5.7 editions: `https://dev.mysql.com/doc/refman/5.7/en/create-server.html`
  (and `alter-server`/`drop-server` siblings) — **[PEND-1]** redirected
  when fetched in R1; content not directly obtained. 5.7 applicability
  is asserted from the 5.7 statement family + 8.0 doc's own 5.7 notes;
  exact 5.7 page text remains unverified — flagged, not blended into a
  [DOC] claim.

## 2. SRS family

| fact | MySQL 5.7 | MySQL 8.0 | MySQL 8.4 | TiDB 8.5 | evidence |
|---|---|---|---|---|---|
| `CREATE SPATIAL REFERENCE SYSTEM` exists | **no — version inapplicable (vendor boundary)** | yes — **introduced in 8.0 line** (WL#9059 is an 8.0 worklog; SRS catalogs ship from **8.0.x**, commonly cited 8.0.4-era) — exact GA point **[PEND]** per-series check | yes | no statement | [WL] WL#9059 `dev.mysql.com/worklog/task/?id=9059` · [DOC] refman `create-spatial-reference-system.html` · **5.7 inapplicability** = feature absent from 5.7 statement list (family-level, [DOC] 5.7 refman statement index) — recorded as *version boundary*, NOT the same basis as the TiDB row |
| `CREATE OR REPLACE SPATIAL REFERENCE SYSTEM` | no | yes | yes | no | [DOC] |
| `DROP SPATIAL REFERENCE SYSTEM [IF EXISTS]` | no | yes | yes | no | [DOC] `drop-spatial-reference-system.html` |
| `ALTER SPATIAL REFERENCE SYSTEM` | no | **no — removed from WL#9059 scope** | no | no | [WL] explicit scope note |
| mandatory attrs NAME + DEFINITION; optional ORGANIZATION…IDENTIFIED BY / DESCRIPTION; duplicates forbidden | — | yes | yes | — | [DOC] syntax + attribute rules |
| length caps NAME≤80, DEFINITION≤4096, ORGANIZATION≤256, DESCRIPTION≤2048 | — | yes | yes | — | [DOC] |
| SRID u32; SRID 0 cannot be created/modified/dropped; reserved ranges warn ([0,32767] ∪ [60000000,69999999] ∪ [2000000000,2147483647]) | — | yes | yes | — | [DOC] + [WL] `ER_CANT_MODIFY_SRID_0` (SR000), reserved-range messages |
| column-dependency refusal on replace/drop: `ERROR 3716 (SR005)`, catalog `ST_GEOMETRY_COLUMNS` | — | yes | yes | — | [DOC] + [WL] `ER_CANT_MODIFY_SRID_USED_BY_COLUMN` |
| DEFINITION well-formedness tightened **8.0.13** (GEOGCS must carry two AXIS clauses: one NORTH/SOUTH + one EAST/WEST); newline inside string literals rejected | — | 8.0.13+ stricter | yes | — | [RN] MySQL 8.0.13 release notes "Spatial Reference Systems" section |
| `IF NOT EXISTS` short-circuit ordering vs attribute validation | — | **PENDING ORACLE** — docs say "ignored and a warning occurs"; whether validation precedes existence check is unstated | same | — | [DOC] ambiguous; **[PEND]** requires fixed-version source or native confirmation (plan only) |
| `CREATE OR REPLACE` on absent SRID | — | **PENDING ORACLE** — docs describe replacing "any existing SRS"; absent-case unspecified | same | — | [DOC] ambiguous; **[PEND]** |
| inspection catalog `ST_SPATIAL_REFERENCE_SYSTEMS` (INFORMATION_SCHEMA) | — | yes | yes | — | [DOC] |

URLs:
- `https://dev.mysql.com/doc/refman/8.0/en/create-spatial-reference-system.html`
- `https://dev.mysql.com/doc/refman/8.0/en/drop-spatial-reference-system.html`
- `https://dev.mysql.com/worklog/task/?id=9059`
- `https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-13.html` (SRS parser tightening)

## 3. ALTER INSTANCE (inventory-owned two subactions stay in T26)

| subaction | 5.7 | 8.0 | 8.4 | evidence |
|---|---|---|---|---|
| `ROTATE INNODB MASTER KEY` | 5.7.11+ | yes | yes | [DOC] `alter-instance.html` + 5.7 family docs |
| `RELOAD TLS [FOR CHANNEL {mysql_main\|mysql_admin}] [NO ROLLBACK ON ERROR]` | no | yes (FOR CHANNEL 8.0.21+) | yes | [DOC] |

URL: `https://dev.mysql.com/doc/refman/8.0/en/alter-instance.html`

Surplus subactions are **out of T26 scope** — tracked locally as
`DDL-ALTER-INSTANCE-EXT` (see `r2-alter-instance-ext.md`): REDO_LOG
(8.0.21+), ROTATE BINLOG MASTER KEY (8.0), RELOAD KEYRING (8.0.24+),
plus RELOAD TLS modifiers FOR CHANNEL / NO ROLLBACK as extension
surface.

## 4. TiDB boundary — separate basis from 5.7

| entry | basis | evidence |
|---|---|---|
| `tidb.create-server` | dialect/vendor boundary: TiDB SQL reference has no SERVER statements; the shared TiDB parser rejects them | inventory `vendor_not_supported` · [DOC] TiDB v8.5 SQL reference absence · **[PEND]** native refusal text |
| `tidb.spatial-reference-system` | dialect/vendor boundary: same as above | same |
| `tidb.logfile-group` | vendor boundary (NDB-only feature line) | T25 ADR |
| TiDB `ALTER INSTANCE` (RELOAD TLS etc.) | **TiDB's own statement, not MySQL semantics** — the shared parser parses it; product treats as named unaudited boundary (`alter_instance` feature) | [SRC] `coverage_boundary.go` + probe outputs |

MySQL 5.7 SRS inapplicability is a **version boundary** (feature line
absent); TiDB boundary is a **dialect boundary** (vendor grammar has no
such statement / has own statement). Different adjudication bases — not
collapsed into one enum row.

## 5. Evidence-type separation maintained

- [DOC]/[RN]/[WL]/[SRC] claims cite the fixed URL or file above.
- Every **[PEND]** item lists the only admissible resolution paths
  (fixed-version source read, or a future authorized native case) — no
  PEND item is resolved by inference.
- Nothing above asserts DeltaScope capability; this table is
  applicability, not coverage.
