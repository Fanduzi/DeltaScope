# T26-SRS-DROP-CONTRACT-01 — case matrix (R1)

Companion to `README.md`. Every row is a **contract expectation derived from baseline source** — nothing below was measured this round. Read counts are per the SRS seams: `R` = `objectResolverFor` forward (cache miss); `Qexist`/`Qdeps` = issued catalog queries; `Qother` = `select version()` inside the SRS loader; `L` = SERVER privilege-loader calls (always 0 for pure-SRS fixtures; mixed rows attribute it to the SERVER statement). `N/A` = the layer is not observed or not applicable — never "zero output".

## Shared defaults (expansion rules — each row below still expands uniquely)

- dialect `mysql`; provider = ObjectResolver-capable seam with a stub driver unless a row says providerless; batch = single enrich call unless sequenced.
- resolved version: pin → `TargetVersion{product:mysql, source:target}` providerless, or observed banner → `resolvedVersion=observed` with provider; pin≠observed → typed error row (E10).
- audited precondition: `MySQLSRSDropAuditedDomain(resolvedVersion)` = product mysql, 8.0.x.
- provider fixture vocabulary: `exist(row)` / `exist(empty)` = existence read outcome; `deps([k])` = `Dependencies{Columns:k}` (`Complete=false` always); `ver(p)` = `select version()` banner parse result.
- rule config: three proposed IDs at defaults — `ddl.mysql.srs.drop.srid0` blocker, `ddl.mysql.srs.drop.dependency` blocker, `ddl.mysql.srs.drop.reserved` notice; no other rule names the family.
- public SQL is always `DROP SPATIAL REFERENCE SYSTEM [IF EXISTS] <srid>`; never asserted again per row.
- owed shorthand: `E`=`srs.existence`, `D`=`srs.column_dependencies`, `P`=`srs.drop.privilege`, `B`=`srs.batch_effects`.

## E1 — present + positive dependency witness (families 1)

| case | layer | SQL | version / pin | provider → response | expected | reads R/Qe/Qd/Qo/L |
|---|---|---|---|---|---|---|
| E1.1 plain | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | `ver(8.0.36) exist(row) deps([s.t.g])` | arm `refused_dependency` → reject; 1 blocker finding; owed ∅; coverage complete; post provably-unchanged | 1/1/1/1/0 |
| E1.2 if-exists | enrich+request | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326;` | pin 8.0.36 | same | same arm — `IF EXISTS` does not reach it; finding metadata `if_exists:true` | 1/1/1/1/0 |

## E2 — present + empty-but-incomplete dependencies (family 2)

| case | layer | SQL | version / pin | provider → response | expected | reads |
|---|---|---|---|---|---|---|
| E2.1 | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | `ver(8.0.36) exist(row) deps([])` | arm `unproven` → review; owed `{D,P}`; 0 blocker; coverage unverified; post → `existenceStale[4326]`, `depsStale` | 1/1/1/1/0 |

## E3 — existence unknown / providerless / no read (family 3)

| case | layer | SQL | version / pin | provider → response | expected | reads |
|---|---|---|---|---|---|---|
| E3.1 providerless | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | pin 8.0.36 | — | arm `unproven`; owed `{E,D,P}` (`Read=false`, `Dependencies=nil`); unverified | 0/0/0/0/0 |
| E3.2 zero-row + empty deps | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | `ver(8.0.36) exist(empty) deps([])` | arm `unproven`; owed `{E,D,P}` — zero-row ≠ absent, empty-incomplete ≠ no deps | 1/1/1/1/0 |
| E3.3 resolver nil | unit (enrich) | `DROP SPATIAL REFERENCE SYSTEM 100001;` | pin 8.0.36 | provider without ObjectResolver | arm `unproven`; owed `{E,D,P}`; `UnreadReason` empty (CURRENT) | 0/0/0/0/0 |

## E4 — unknown existence + positive dependency (family 4)

| case | layer | SQL | version / pin | provider → response | expected | reads |
|---|---|---|---|---|---|---|
| E4.1 | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | `ver(8.0.36) exist(empty) deps([a.b.c])` | arm `refused_dependency` → reject; owed ∅ (witness discharges existence); complete | 1/1/1/1/0 |

## E5 — SRID 0 / numeric boundaries / unclaimed forms / reserved ranges (family 5)

| case | layer | SQL | version / pin | provider | expected | reads |
|---|---|---|---|---|---|---|
| E5.1 srid0 | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 0;` | pin 8.0.36 | `ver(8.0.36) exist(row) deps([])` | arm `refused_srid0` → reject; 1 blocker; owed ∅; complete | 1/1/1/1/0 |
| E5.2 srid0+ie | enrich+request | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 0;` | pin 8.0.36 | same | same refusal — `IF EXISTS` never suppresses SRID 0 | 1/1/1/1/0 |
| E5.3 u32 max | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 4294967295;` | pin 8.0.36 | `ver(8.0.36) exist(empty) deps([])` | `unproven`, owed `{E,D,P}`; no reserved advisory (4294967295 ∉ reserved) | 1/1/1/1/0 |
| E5.4 reserved edge | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 32767;` | pin 8.0.36 | `exist(empty) deps([])` | `unproven` + notice `…reserved`; owed `{E,D,P}` | 1/1/1/1/0 |
| E5.5 just-above | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 32768;` | pin 8.0.36 | same | `unproven`, **no** advisory | 1/1/1/1/0 |
| E5.6 reserved mid | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 60000000;` + `… 2147483647;` (two rows) | pin 8.0.36 | `exist(empty) deps([])` per SRID | both `unproven` + advisory; 59999999/70000000/2000000000−1/2147483648 controls → no advisory | 1/1/1/1/0 each |
| E5.7 negative | parser | `DROP SPATIAL REFERENCE SYSTEM -1;` | pin 8.0.36 | — | unclaimed → base-grammar diagnostic; never a `SpatialReference` statement | 0/0/0/0/0 |
| E5.8 overflow | parser | `DROP SPATIAL REFERENCE SYSTEM 4294967296;` | pin 8.0.36 | — | unclaimed → base-grammar diagnostic | 0/0/0/0/0 |
| E5.9 non-numeric | parser | `DROP SPATIAL REFERENCE SYSTEM 'x';` / `… foo;` | pin 8.0.36 | — | unclaimed → base-grammar diagnostic | 0/0/0/0/0 |
| E5.10 trailing | parser | `DROP SPATIAL REFERENCE SYSTEM 1 EXTRA;` / `… IF EXISTS 1 X;` | pin 8.0.36 | — | unclaimed (`len(rest)!=0`) | 0/0/0/0/0 |
| E5.11 refused+reserved suppressed | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 4326;` where fixture `deps([s.t.g])` and SRID reserved variant `DROP … 60000000` with `deps([s.t.g])` | pin 8.0.36 | `exist(row) deps([…])` | `refused_dependency`; advisory **suppressed** on refusal arms | 1/1/1/1/0 |

## E6 — same SRID ordering (family 6)

| case | layer | SQL sequence | version / pin | provider | expected | reads (per stmt) |
|---|---|---|---|---|---|---|
| E6.1 uncertain→drop | enrich | `DROP SPATIAL REFERENCE SYSTEM 4326; DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | `exist(empty) deps([])` | stmt1 `unproven` `{E,D,P}` → marks stale; stmt2 arm `blocked` owed `{B}` (cache: would-hit → pre-check exits first) | stmt1 1/1/1/1/0; stmt2 0/0/0/0/0 |
| E6.2 refusal→drop | enrich | `DROP SPATIAL REFERENCE SYSTEM 0; DROP SPATIAL REFERENCE SYSTEM 0;` | pin 8.0.36 | any | stmt1 `refused_srid0`; stmt2 `refused_srid0` again — refusal keeps facts consumable | stmt2 cache-hit 0/0/0/0/0 |
| E6.3 dep-refusal→drop | enrich | `DROP … 4326; DROP … 4326;` | pin 8.0.36 | `exist(row) deps([x])` | stmt1 `refused_dependency`; stmt2 `refused_dependency` (cache hit, witness still consumable) | 1/1/1/1/0; 0/0/0/0/0 |

## E7 — boundary SRS writes and different-SRID control (families 7)

| case | layer | SQL sequence | version / pin | provider | expected | reads |
|---|---|---|---|---|---|---|
| E7.1 boundary create→drop | enrich | `CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 4326 NAME 'a' DEFINITION 'x'; DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | `exist(empty) deps([])` if reached | stmt1 boundary `unaudited implementation` (unmodeled write → stale both facts); stmt2 arm `blocked` `{B}` | stmt1 boundary still binds: 1/1/1/1/0; stmt2 0/0/0/0/0 |
| E7.2 different SRID | enrich | `CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 4326 NAME 'a' DEFINITION 'x'; DROP SPATIAL REFERENCE SYSTEM 100001;` | pin 8.0.36 | per-SRID fixtures | stmt1 boundary (stale only for 4326); stmt2 `unproven` `{E,D,P}` — no cross-SRID folding | 1/1/1/1/0; 1/1/1/1/0 |

## E8 — table/schema change prefix → deps invalidation (family 8)

Shared setup: stmt2 is always `DROP SPATIAL REFERENCE SYSTEM 4326`; the deps witness was bound at stmt1's own bind where listed. `exist(row)` fixed.

| case | layer | SQL sequence | expected | reads |
|---|---|---|---|---|
| E8.1 drop-table prefix | enrich | `DROP TABLE t; DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt2 arm `unproven` owed `{D,P}` — `depsStale` set by the table drop (existence still consumable → `Exists=true`) | stmt2 1/1/1/1/0 (exist read issued at resolve; deps flagged stale post-bind) |
| E8.2 inert create exempt | enrich | `CREATE TABLE t (i INT); DROP SPATIAL REFERENCE SYSTEM 4326;` | provably spatial-inert (complete authored column list, none spatial) → `depsStale` not set → owed `{D,P}` only if deps `deps([])`-incomplete… — fixture `deps([c])` variant → `refused_dependency` survives | 1/1/1/1/0 |
| E8.3 spatial-column create | enrich | `CREATE TABLE g (p POINT SRID 4326); DROP SPATIAL REFERENCE SYSTEM 4326;` | `spatialTouching` → `depsStale` → `unproven` `{D,P}` (positive witness consumed pre-write is stale) | 1/1/1/1/0 |
| E8.4 boundary table stmt | enrich | `<any unsupported stmt carrying table targets>; DROP SPATIAL REFERENCE SYSTEM 4326;` | unmodeled effect → `depsStale` → `{D,P}` | 1/1/1/1/0 |
| E8.5 cross-call staleness | enrich ×2 on one `batchState` | call1 `DROP TABLE t;` call2 `DROP SPATIAL REFERENCE SYSTEM 4326;` | `depsStale` persists across calls (objectCache is per-call; the flag is on state) → `{D,P}` | call2 1/1/1/1/0 — re-query cannot fix a simulated write |

## E9 — contamination and failure-record ordering (family 9)

| case | layer | SQL sequence | expected | reads |
|---|---|---|---|---|
| E9.1 failure-before | enrich | `GARBAGE; DROP SPATIAL REFERENCE SYSTEM 4326;` | contaminated before stmt2 → arm `blocked` `{B}`; no resolver call | stmt2 0/0/0/0/0 |
| E9.2 authored ordering | enrich | same; assert failure record (Line,Col) precedes statement (Line,Col) | `blocked` per authored ordering, not list order | — |
| E9.3 trailing failure cross-call | enrich ×2 | call1 `DROP SPATIAL REFERENCE SYSTEM 4326; GARBAGE` call2 `DROP SPATIAL REFERENCE SYSTEM 4326;` | call2 `blocked` `{B}` — contaminated persists on `batchState`; fresh per-call cache cannot launder it | call1 1/1/1/1/0; call2 0/0/0/0/0 |

## E10 — version/product inputs (family 10)

| case | layer | input | expected | reads |
|---|---|---|---|---|
| E10.1 version unknown, observed unparseable | enrich+request | provider banner `n/a`; no pin | `resolvedVersion=nil` → unaudited `gate:"version"`; reads still issued (unproven observed → fire) | 1/1/1/1/0 |
| E10.2 no pin, observed in-domain | enrich+request | banner `8.0.36` | audited → arm per facts | 1/1/1/1/0 |
| E10.3 observed out-domain | enrich+request | banner `5.7.44` | unaudited `gate:"version"`; provider: proven pre-8.0 → `version_inapplicable`, no catalog reads | 1/0/0/1/0 |
| E10.4 providerless out-domain pin | request | pin `5.7.44` | unaudited `gate:"version"` | 0/0/0/0/0 |
| E10.5 TiDB | parser/enrich | dialect tiddb, banner `8.0.11-TiDB-v8.5.0` | vendor boundary; bind skipped | 0/0/0/0/0 |
| E10.6 malformed pin | request | pin `8.0.x` | `ErrInvalidTargetVersion` — typed error, no enrichment | 0/0/0/0/0 |
| E10.7 product mismatch | request | dialect mysql + TiDB banner | `ErrDialectProductMismatch` | — |
| E10.8 pin≠observed | request | pin `8.0.36` + banner `8.0.37` | `ErrTargetVersionMismatch` | — |

## E11 — error paths (family 11)

| case | layer | provider response | expected | reads |
|---|---|---|---|---|
| E11.1 resolver forward error | enrich | resolver returns injected sentinel | `metadataObjectReadError` propagates → enrich typed error; `errors.Is` sees sentinel; no adjudication, no gap | 1(att)/0/0/0/0 |
| E11.2 existence SQL error | enrich | `ver(8.0.36)` + existence query fails | wrapped error propagates (typed); deps never issued | 1/1(att)/0/1/0 |
| E11.3 deps SQL error | enrich | `ver(8.0.36) exist(row)` + deps query fails | wrapped error propagates | 1/1/1(att)/1/0 |
| E11.4 version() error | enrich | `select version()` fails | wrapped error propagates before catalog reads | 1/0/0/1(att)/0 |

## E12 — redaction and cross-channel isolation (family 12)

| case | layer | setup | expected |
|---|---|---|---|
| E12.1 skeleton | request | E1.1 fixture | serialized `sql` exactly `DROP SPATIAL REFERENCE SYSTEM 4326`; no authored names anywhere (only SRID digits) |
| E12.2 no dep-name leak | request | `deps([s.t.g])` fixture with names `secret_schema`,`secret_tbl`,`secret_col` | serialized report contains none of the three strings; finding metadata may carry `dependency_witness_count:1` only |
| E12.3 SRS apply channel isolation | enrich | populated table registry + SERVER channel present | SRS own apply leaves table/schema/SERVER state byte-identical (fidelity snapshot witnesses, reused); stale flags are the only writes |
| E12.4 table apply SRS-isolation | enrich | `DROP TABLE t; DROP SPATIAL REFERENCE SYSTEM 4326;` | table apply sets only `depsStale`; `srsExistenceStale` untouched; SERVER channel untouched |
| E12.5 mixed fixture L | enrich | `DROP SERVER s; DROP SPATIAL REFERENCE SYSTEM 4326;` under `observed_account_new_session` | `L=1` attributed to the SERVER statement; SRS row keeps `P` owed (SERVER premise never substitutes) |

## Probe plan — `PLAN / UNEXECUTED` (registered, not run)

All three: single statement `DROP SPATIAL REFERENCE SYSTEM 100001;`, `deltascope audit --dialect mysql --config <cfg> --input <file>` (exact binary path per execution-order build record), explicit policy block enabling the three proposed rule IDs, providerless (no metadata DSN), default privilege-observation (off). Config text (fixed):

```yaml
dialect: mysql
target_version: <per row>
rules:
  ddl.mysql.srs.drop.srid0:      { level: blocker }
  ddl.mysql.srs.drop.dependency: { level: blocker }
  ddl.mysql.srs.drop.reserved:   { level: notice }
```

| probe | pin | expected per this contract |
|---|---|---|
| P1 | `8.0.36` | audited `unproven` arm: verdict review; one row, coverage unverified; 3 evidence gaps `{srs.existence, srs.column_dependencies, srs.drop.privilege}`; 0 blocker findings; `unsupported:[]` empty for the statement; **rc 0** expected if review-level results exit 0 — record actual; R/Q all 0 (providerless) |
| P2 | none | unaudited boundary `gate:"version"` + `object:"spatial_reference_system"`; coverage incomplete; 0 findings; rc per boundary convention — record actual |
| P3 | `8.0.x` | `ErrInvalidTargetVersion` typed error before audit; non-zero rc per CLI error convention — record actual |

Expected fields are contract-derived; probe outcomes are recorded, never asserted here. Pin⇒rc conclusions from SERVER probes are not carried over.
