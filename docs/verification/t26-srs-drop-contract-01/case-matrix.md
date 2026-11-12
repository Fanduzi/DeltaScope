# T26-SRS-DROP-CONTRACT-01 — case matrix (R3)

Companion to `README.md`. Every row is a **contract expectation derived from baseline source** — nothing below was measured this round. Status per `expected`: **CURRENT** = baseline already produces it; **PROPOSED** = this contract selects it for the execution order.

**Observation layers** (one explicit combination per row): `parser/extraction` (claim + declaration only), `adjudication-unit` (spec-level arm evaluation — the only layer where no-producer inputs may appear), `enrich/apply` (bind + adjudication + own apply inside one enrich call), `public-projection` (per-statement serialized fields), `request/API` (full request incl. aggregate coverage/verdict), `CLI` (registered PLAN/UNEXECUTED only). `N/A` = the layer never runs — never a "zero output". Aggregate `coverage`/`verdict` are asserted **only** on `request/API` and `CLI` rows.

**Read-count seams** (per-statement unless per-request): `R` = `objectResolverFor` forwards (cache misses); `Qexist`/`Qdeps` = issued SRS catalog queries; `Qo` = `select version()` inside **the SRS loader only**; `L` = SERVER privilege-loader calls. Table/schema reads issued by non-SRS statements are reported in the row text, never folded into the SRS columns. Request-layer version discovery (`prepareMetadataAudit` / observed-banner read) is a separate surface — noted as `Vd` where relevant, never counted in `Qo`. An `(att)` suffix = attempted read that failed.

## Shared defaults (concrete)

- **SQL surface**: statements exactly as written per row; default schema `s` for unqualified table names.
- **Request**: dialect `mysql`; `--target-version 8.0.36` (`product=mysql`, validated series) unless the row overrides; `privilege_audit_mode` unset (empty — observation off); `--format json`.
- **Initial batch state**: `contaminated=false`, `srsExistenceStale={}`, `srsDepsStale=false`, objectCache empty, no failure records.
- **Rules**: `ddl.mysql.srs.drop.srid0` `{enabled:true, level:blocker}`, `ddl.mysql.srs.drop.dependency` `{enabled:true, level:blocker}`, `ddl.mysql.srs.drop.reserved` `{enabled:true, level:notice}`; no other rule names the family.
- **Provider wiring**: ObjectResolver-capable provider on a stub driver returning scripted responses in issue order. `ver("x")` = `select version()` inside the SRS loader answering banner `x`; `exist(row)` = one `st_spatial_reference_systems` row (`srs_id` matching, `srs_name='EPSG 4326'`); `exist(empty)` = zero rows; `deps([s.t.g])` = `st_geometry_columns` rows as **three-column `schema/table/column` results** — the matching `SRID` on `SRSDependentColumn` is a loader-constructed fact, not an extra SQL column; `deps([])` = zero rows (`Complete=false` always).
- **Reserved membership**: SRID 4326 ∈ `[0,32767]` — **every B/S2 row on 4326 co-fires exactly one `…reserved` notice**; S0/S1 suppress it. SRID 100001 is outside every reserved range (no advisory).
- **Owned shorthand**: `E`=`srs.existence`, `D`=`srs.column_dependencies`, `P`=`srs.drop.privilege`, `B`=`srs.batch_effects`. S2 owes `{D,P}` always, `E` prepended iff existence unproven.
- **Reads column order**: `R / Qexist / Qdeps / Qo / L`.
- **Serialization**: empty `findings`/`evidence_gaps`/`unsupported` = key-absent (`omitempty`); `index:0` omitted on `UnsupportedDetail`.

## E1 — present + positive dependency witness

| case | layer | SQL | version | provider → response | expected | reads R/Qe/Qd/Qo/L |
|---|---|---|---|---|---|---|
| E1.1 | request/API | `DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | `ver("8.0.36") exist(row) deps([s.t.g])` | **PROPOSED** `refused_dependency`; `$.statements[0].findings=[{rule_id:"ddl.mysql.srs.drop.dependency",level:"blocker",message:"spatial reference system 4326 is used by 1 visible geometry column(s)",metadata:{srid:4326,outcome:"refused_dependency",dependency_witness_count:1}}]`; `evidence_gaps` absent; `coverage.status:"complete"`; `$.verdict:"reject"`; `$.coverage.status:"complete"` | 1/1/1/1/0 |
| E1.2 | request/API | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326;` | pin 8.0.36 | same as E1.1 | same arm — `IF EXISTS` does not reach it; `normalized_sql:"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326"`; no `if_exists` key anywhere | 1/1/1/1/0 |

## E2 — present + empty-but-incomplete dependencies

| case | layer | SQL | version | provider | expected | reads |
|---|---|---|---|---|---|---|
| E2.1 | request/API | `DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | `ver("8.0.36") exist(row) deps([])` | **PROPOSED** `unproven` → `$.verdict:"review"`; exactly **1** finding `[{rule_id:"ddl.mysql.srs.drop.reserved",level:"notice",message:"spatial reference system 4326 is in a reserved SRID range",metadata:{srid:4326}}]` (4326 ∈ [0,32767]); gaps `[{reason_code:"unknown_srs_state",required_facts:[D,P]}]`; `coverage.status:"unverified"`; post → `srsExistenceStale[4326]` only | 1/1/1/1/0 |

## E3 — existence unknown / providerless / no read

| case | layer | SQL | version | provider | expected | reads |
|---|---|---|---|---|---|---|
| E3.1 | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | pin 8.0.36 | providerless (no DSN) | **PROPOSED** `unproven` → `$.verdict:"review"`; `findings` absent (100001 not reserved); gaps `[{required_facts:[E,D,P]}]` (`Read=false`, `Dependencies=nil`); `coverage.status:"unverified"` | 0/0/0/0/0 |
| E3.2 | request/API | `DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | `ver("8.0.36") exist(empty) deps([])` | `unproven`; 1 reserved notice; gaps `[E,D,P]` — zero-row ≠ absent, empty-incomplete ≠ no deps | 1/1/1/1/0 |
| E3.3 | enrich/apply | `DROP SPATIAL REFERENCE SYSTEM 100001;` | pin 8.0.36 | provider present without ObjectResolver | `unproven`; gaps `[E,D,P]`; `UnreadReason` empty (**CURRENT** shape); statement-level only — aggregate N/A | 0/0/0/0/0 |

## E4 — unknown existence + positive dependency

| case | layer | SQL | version | provider | expected | reads |
|---|---|---|---|---|---|---|
| E4.1 | request/API | `DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | `ver("8.0.36") exist(empty) deps([s.t.g])` | **PROPOSED** `refused_dependency` → `$.verdict:"reject"`; `evidence_gaps` absent (the witness discharges existence); `coverage.status:"complete"` | 1/1/1/1/0 |

## E5 — SRID 0 / numeric boundaries / unclaimed forms / reserved ranges

| case | layer | SQL | version | provider | expected | reads |
|---|---|---|---|---|---|---|
| E5.1 | request/API | `DROP SPATIAL REFERENCE SYSTEM 0;` | pin 8.0.36 | `ver("8.0.36") exist(row) deps([])` | **PROPOSED** `refused_srid0` → `$.verdict:"reject"`; `findings=[{rule_id:"ddl.mysql.srs.drop.srid0",level:"blocker",message:"dropping spatial reference system SRID 0 is not allowed",metadata:{srid:0,outcome:"refused_srid0"}}]`; `evidence_gaps` absent; `coverage.status:"complete"`; reserved advisory **suppressed** (0 ∈ [0,32767], refusal arm wins) | 1/1/1/1/0 |
| E5.2 | request/API | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 0;` | pin 8.0.36 | same as E5.1 | same refusal — `IF EXISTS` never suppresses SRID 0 | 1/1/1/1/0 |
| E5.3 | request/API | `DROP SPATIAL REFERENCE SYSTEM 4294967295;` | pin 8.0.36 | `ver("8.0.36") exist(empty) deps([])` | `unproven`; `findings` absent (4294967295 ∉ reserved); gaps `[E,D,P]`; `coverage.status:"unverified"`; `$.verdict:"review"` | 1/1/1/1/0 |
| E5.4 | enrich/apply | `DROP SPATIAL REFERENCE SYSTEM 32767;` | pin 8.0.36 | `ver("8.0.36") exist(empty) deps([])` | `unproven` + `findings=[{rule_id:"ddl.mysql.srs.drop.reserved",level:"notice",message:"spatial reference system 32767 is in a reserved SRID range",metadata:{srid:32767}}]`; gaps `[E,D,P]` | 1/1/1/1/0 |
| E5.5 | enrich/apply | `DROP SPATIAL REFERENCE SYSTEM 32768;` | pin 8.0.36 | same | `unproven`, `findings` absent — **no** advisory | 1/1/1/1/0 |
| E5.6a | enrich/apply | `DROP SPATIAL REFERENCE SYSTEM 60000000;` | pin 8.0.36 | same | `unproven` + reserved notice | 1/1/1/1/0 |
| E5.6b | enrich/apply | `DROP SPATIAL REFERENCE SYSTEM 59999999;` | pin 8.0.36 | same | `unproven`, no advisory | 1/1/1/1/0 |
| E5.6c | enrich/apply | `DROP SPATIAL REFERENCE SYSTEM 69999999;` | pin 8.0.36 | same | `unproven` + reserved notice | 1/1/1/1/0 |
| E5.6d | enrich/apply | `DROP SPATIAL REFERENCE SYSTEM 70000000;` | pin 8.0.36 | same | `unproven`, no advisory | 1/1/1/1/0 |
| E5.6e | enrich/apply | `DROP SPATIAL REFERENCE SYSTEM 2000000000;` | pin 8.0.36 | same | `unproven` + reserved notice | 1/1/1/1/0 |
| E5.6f | enrich/apply | `DROP SPATIAL REFERENCE SYSTEM 1999999999;` | pin 8.0.36 | same | `unproven`, no advisory | 1/1/1/1/0 |
| E5.6g | enrich/apply | `DROP SPATIAL REFERENCE SYSTEM 2147483647;` | pin 8.0.36 | same | `unproven` + reserved notice | 1/1/1/1/0 |
| E5.6h | enrich/apply | `DROP SPATIAL REFERENCE SYSTEM 2147483648;` | pin 8.0.36 | same | `unproven`, no advisory (fits uint32) | 1/1/1/1/0 |
| E5.7 | parser/extraction | `DROP SPATIAL REFERENCE SYSTEM -1;` | pin 8.0.36 | — | **CURRENT** unclaimed → base-grammar parse diagnostic; never a `SpatialReference` statement; all SRS seams N/A | N/A |
| E5.8 | parser/extraction | `DROP SPATIAL REFERENCE SYSTEM 4294967296;` | pin 8.0.36 | — | unclaimed → base-grammar diagnostic (uint32 overflow) | N/A |
| E5.9 | parser/extraction | `DROP SPATIAL REFERENCE SYSTEM 'x';` and `DROP SPATIAL REFERENCE SYSTEM foo;` | pin 8.0.36 | — | both unclaimed → base-grammar diagnostics | N/A |
| E5.10 | parser/extraction | `DROP SPATIAL REFERENCE SYSTEM 1 EXTRA;` and `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 1 X;` | pin 8.0.36 | — | both unclaimed (`len(rest)!=0`) | N/A |
| E5.11 | request/API | `DROP SPATIAL REFERENCE SYSTEM 2000000000;` | pin 8.0.36 | `ver("8.0.36") exist(row) deps([s.t.g])` | `refused_dependency` (reserved range but positive witness) — advisory **suppressed** on refusal arms; findings carry only the dependency blocker | 1/1/1/1/0 |

## E6 — same SRID ordering

| case | layer | SQL sequence | version | provider | expected | reads (per stmt) |
|---|---|---|---|---|---|---|
| E6.1 | enrich/apply | `DROP SPATIAL REFERENCE SYSTEM 4326; DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | `ver("8.0.36") exist(empty) deps([])` | stmt1 `unproven` gaps `[E,D,P]` + 1 reserved notice → mints `srsExistenceStale[4326]`; stmt2 arm `blocked` gaps `[B]` + 1 reserved notice — bind exits **before** the cache is consulted | stmt1 1/1/1/1/0; stmt2 0/0/0/0/0 |
| E6.2 | enrich/apply | `DROP SPATIAL REFERENCE SYSTEM 0; DROP SPATIAL REFERENCE SYSTEM 0;` | pin 8.0.36 | `ver("8.0.36") exist(row) deps([])` | stmt1 `refused_srid0` (reads still fire at its bind); stmt2 `refused_srid0` — refusal keeps facts consumable; stmt2 hits objectCache (R=0) and re-adjudicates identically | stmt1 1/1/1/1/0; stmt2 0/0/0/0/0 |
| E6.3 | enrich/apply | `DROP SPATIAL REFERENCE SYSTEM 4326; DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | `ver("8.0.36") exist(row) deps([s.t.g])` | stmt1 `refused_dependency`; stmt2 `refused_dependency` — cache-hit witness still consumable (refusal mints nothing) | stmt1 1/1/1/1/0; stmt2 0/0/0/0/0 |

## E7 — boundary SRS writes and different-SRID control

| case | layer | SQL sequence | version | provider | expected | reads |
|---|---|---|---|---|---|---|
| E7.1 | enrich/apply | `CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 4326 DEFINITION 'x'; DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | stmt1 bind `ver("8.0.36") exist(empty) deps([])` | stmt1 **CURRENT** boundary `{boundary:"implementation",object:"spatial_reference_system"}` — binds and reads, mints `srsExistenceStale[4326]`; stmt2 arm `blocked` gaps `[B]` + 1 reserved notice | stmt1 1/1/1/1/0; stmt2 0/0/0/0/0 |
| E7.2 | enrich/apply | `CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 4326 DEFINITION 'x'; DROP SPATIAL REFERENCE SYSTEM 100001;` | pin 8.0.36 | stmt1 `ver exist(empty) deps([])`; stmt2 `ver("8.0.36") exist(row) deps([s.t.g])` | stmt1 boundary → `srsExistenceStale[4326]` only; stmt2 SRID 100001 unaffected → `refused_dependency` (valid witness), `findings` absent except the blocker (100001 not reserved) | stmt1 1/1/1/1/0; stmt2 1/1/1/1/0 |

## E8 — table/schema writes → dependency invalidation

Shared setup: existence is `exist(row)` for SRID 4326 unless marked; table/schema statements' own reads are counted separately in row text, never inside the SRS columns.

| case | layer | SQL sequence | provider | expected | reads |
|---|---|---|---|---|---|
| E8.1 | enrich/apply | `DROP TABLE t; DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt2 `ver("8.0.36") exist(row) deps([s.t.g])` | `DROP TABLE` touching → `srsDepsStale`; stmt2's freshly-read positive witness is DB-pre-state (the simulated drop precedes it) → invalid → `unproven` gaps `[D,P]` + 1 reserved notice; existence stays consumable (`Exists=true`) | stmt2 1/1/1/1/0 |
| E8.2a | enrich/apply | `CREATE TABLE t (i INT); DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt2 `ver("8.0.36") exist(row) deps([s.u.g])` | `CREATE TABLE t` complete non-spatial column list → inert; the positive control sits on a **different existing** table `u` — witness valid → `refused_dependency`; `evidence_gaps` absent; `coverage.status:"complete"` | stmt2 1/1/1/1/0 |
| E8.2b | enrich/apply | `CREATE TABLE t (i INT); DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt2 `ver("8.0.36") exist(row) deps([])` | inert prefix + empty-incomplete deps → `unproven` gaps `[D,P]` + 1 reserved notice (unproven-empty, **not** stale) | stmt2 1/1/1/1/0 |
| E8.3 | enrich/apply | `CREATE TABLE t (g GEOMETRY SRID 4326); DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt2 `ver("8.0.36") exist(row) deps([s.t.g])` | spatial authored column → touching → `srsDepsStale` → `unproven` gaps `[D,P]` + 1 reserved notice | stmt2 1/1/1/1/0 |
| E8.4 | enrich/apply | `ALTER TABLE t COMPACT TIFLASH REPLICA; DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt2 `ver("8.0.36") exist(row) deps([s.t.g])` | stmt1 is a **statically provable vendor boundary under MySQL** (`coverage_boundary.go:189–194`) whose normalized model carries `Table=t` (`extractor.go:813–832`) → `TableTargets()≠∅` → touching → `unproven` gaps `[D,P]` + 1 reserved notice | stmt1 boundary (no SRS bind); stmt2 1/1/1/1/0 |
| E8.5 | enrich/apply ×2, same `batchState` | call1 `DROP TABLE t;` call2 `DROP SPATIAL REFERENCE SYSTEM 4326;` | call2 `ver("8.0.36") exist(row) deps([s.t.g])` | `srsDepsStale` persists across calls (flag lives on `batchState`, cache is per-call) → call2 `unproven` gaps `[D,P]` + 1 reserved notice | call2 1/1/1/1/0 |
| E8.6 | enrich/apply | `DROP SPATIAL REFERENCE SYSTEM 4326; DROP TABLE t; DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt1 `ver("8.0.36") exist(row) deps([s.t.g])`; stmt3 = cache hit | stmt1 `refused_dependency`; `DROP TABLE` → `srsDepsStale`; stmt3 cache hit returns the same witness — existence consumable, dependency stale → `unproven` gaps `[D,P]` + 1 reserved notice | stmt1 1/1/1/1/0; stmt3 0/0/0/0/0 |
| E8.7 | enrich/apply | `DROP SPATIAL REFERENCE SYSTEM 4326; ALTER TABLE t DROP COLUMN g; DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt1 `ver("8.0.36") exist(row) deps([s.t.g])`; stmt3 = cache hit | stmt1 `refused_dependency` on column `g`; `ALTER TABLE t DROP COLUMN g` (column action) → `srsDepsStale`; stmt3 cache hit — witness now names a column the batch dropped → invalid → `unproven` gaps `[D,P]` + 1 reserved notice | stmt1 1/1/1/1/0; stmt3 0/0/0/0/0 |
| E8.8 | enrich/apply | `ALTER TABLE t ENGINE=InnoDB; DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt2 `ver("8.0.36") exist(row) deps([s.t.g])` | complete action list = engine only → inert → witness valid → `refused_dependency` | stmt2 1/1/1/1/0 |
| E8.9 | enrich/apply | `DROP SCHEMA s2; DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt2 `ver("8.0.36") exist(row) deps([s.t.g])` | schema drop → touching → `unproven` gaps `[D,P]` + 1 reserved notice | stmt2 1/1/1/1/0 |
| E8.10 | enrich/apply | `CREATE SCHEMA s2; DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt2 `ver("8.0.36") exist(row) deps([s.t.g])` | `CREATE SCHEMA` provably empty → inert → `refused_dependency` | stmt2 1/1/1/1/0 |
| E8.11 | enrich/apply | `CREATE TABLE t2 LIKE t; DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt2 `ver("8.0.36") exist(row) deps([s.t.g])` | `LIKE`-source columns unvouched → completeness unproven → touching → `unproven` gaps `[D,P]` + 1 reserved notice | stmt2 1/1/1/1/0 |

## E9 — contamination and failure-record ordering

| case | layer | SQL sequence | provider | expected | reads |
|---|---|---|---|---|---|
| E9.1 | enrich/apply | `GARBAGE; DROP SPATIAL REFERENCE SYSTEM 4326;` | `ver("8.0.36") exist(row) deps([s.t.g])` scripted but never reached | failure record (line 1, col 1–7) precedes the statement → `contaminated` before stmt2 → arm `blocked` gaps `[B]` + 1 reserved notice; no resolver call | stmt2 0/0/0/0/0 |
| E9.2 | enrich/apply | `GARBAGE; DROP SPATIAL REFERENCE SYSTEM 4326;` (one line) | same | failure record authored at (line 1, col 1); SRS statement authored at (line 1, col 10) — `failureBefore(1,1,1,10)=true` → `blocked` per authored position | stmt2 0/0/0/0/0 |
| E9.3 | enrich/apply ×2, same `batchState` | call1 `DROP SPATIAL REFERENCE SYSTEM 0; GARBAGE;` — stmt1 authored (1,1)–(1,32), failure record authored (1,34); call2 `DROP SPATIAL REFERENCE SYSTEM 4326;` | call1 stmt1 `ver("8.0.36") exist(row) deps([])` | initial state clean; call1 stmt1 `refused_srid0` (reads fire; refusal mints no stale flag); trailing `GARBAGE` mints `contaminated` via tail sweep; call2 → `blocked` gaps `[B]` + 1 reserved notice — fresh per-call cache cannot launder `batchState` contamination | call1 stmt1 1/1/1/1/0; call2 0/0/0/0/0 |

## E10 — version/product inputs

| case | layer | input | expected | reads |
|---|---|---|---|---|
| E10.1 | request/API | dialect mysql, no pin, observed banner `"unknown"` (no leading `\d.\d.\d` — `observedVersionPrefix` fails, `tidbVersionMarker` absent) | `resolvedVersion=nil` → `gate:"version"` boundary — unknown ≠ feature-absent; provider reads still fire (unproven observed → fire): `$.unsupported` gains the frozen gate entry; `coverage.status:"incomplete"`; `$.verdict:"review"` | 1/1/1/1/0 |
| E10.2 | request/API | dialect mysql, no pin, observed banner `"8.0.36-log"`; SQL `DROP SPATIAL REFERENCE SYSTEM 4326;` | audited → `unproven` on `exist(empty) deps([])`; 1 reserved notice; gaps `[E,D,P]`; `coverage.status:"unverified"`; `$.verdict:"review"` (fixed facts, fixed arm) | 1/1/1/1/0 |
| E10.3 | request/API | dialect mysql, no pin, observed banner `"5.7.0-m14"` (prefix parses → proven 5.7.0) | `gate:"version"` boundary (proven ≠8.0.4+); provider: proven pre-8.0 → `version_inapplicable`, no catalog reads; `coverage.status:"incomplete"` | 1/0/0/1/0 |
| E10.3b | request/API | dialect mysql, no pin, observed banner `"8.0.3-log"` | `gate:"version"` boundary — **proven pre-feature**; reads still fire (observed 8.0.x → existence+deps per B2) | 1/1/1/1/0 |
| E10.3c | request/API | pin `8.0.3`; providerless | `gate:"version"` boundary (pin proves patch 3 < 4); `coverage.status:"incomplete"` | 0/0/0/0/0 |
| E10.3d | request/API | pin `8.0.4`; providerless | audited → `unproven` gaps `[E,D,P]` (floor is inclusive); `coverage.status:"unverified"` | 0/0/0/0/0 |
| E10.4 | request/API | pin `5.7.44`; providerless | `gate:"version"` boundary | 0/0/0/0/0 |
| E10.5 | parser/extraction + enrich | dialect `tidb`, no pin, observed banner `"8.0.11-TiDB-v8.5.0"` | **CURRENT** vendor boundary (`metadata:{boundary:"vendor"}`); `bindMySQLSRS` exits before resolver; `coverage.status:"incomplete"` | 0/0/0/0/0 |
| E10.6 | request/API | pin `8.0.x` | **CURRENT** `ErrInvalidTargetVersion` before enrichment — no audit result, no SRS reads | 0/0/0/0/0 |
| E10.7 | request/API | dialect `mysql` + observed TiDB banner `"8.0.11-TiDB-v8.5.0"` | **CURRENT** `ErrDialectProductMismatch` — typed error; SRS seams exactly zero; request-discovery reads (`Vd`) recorded separately at the connection surface | 0/0/0/0/0 (Vd N/A) |
| E10.8 | request/API | pin `8.0.36` + observed banner `"8.0.37-log"` | **CURRENT** `ErrTargetVersionMismatch` — typed error; SRS seams zero; `Vd` fired once at the connection surface | 0/0/0/0/0 (Vd=1) |

## E11 — error paths (provider-layer vs request-layer kept distinct)

| case | layer | fixture | expected | reads |
|---|---|---|---|---|
| E11.1 | enrich/apply | `DROP SPATIAL REFERENCE SYSTEM 4326;` resolver forward returns injected sentinel `srs-sentinel-7` | `metadataObjectReadError` propagates → enrich typed error terminating the statement's own bind (no adjudication, no apply, no gap substitute); `errors.Is` sees `srs-sentinel-7`; public text stays bounded (`metadata object read failed`) | 1(att)/0/0/0/0 |
| E11.2 | enrich/apply | `ver("8.0.36")` + existence query fails | wrapped error propagates (typed); deps never issued; same termination point | 1/1(att)/0/1/0 |
| E11.3 | enrich/apply | `ver("8.0.36") exist(row)` + deps query fails | wrapped error propagates | 1/1/1(att)/1/0 |
| E11.4 | enrich/apply | `select version()` inside the SRS loader fails | wrapped error propagates before catalog reads | 1/0/0/1(att)/0 |
| E11.5 | request/API | request-layer version-discovery read fails (connection surface, before enrichment) | typed error on the request path — never adjudicated, never a "feature absent" state; distinct from E11.4: the failing read is the discovery probe, not the SRS loader's own `select version()` | 0/0/0/0/0 (Vd=1 att) |

## E12 — redaction and cross-channel isolation

| case | layer | setup | expected |
|---|---|---|---|
| E12.1 | request/API | E1.1 fixture | `$.statements[0].raw_sql` and `$.statements[0].normalized_sql` both exactly `DROP SPATIAL REFERENCE SYSTEM 4326`; E1.2 fixture → `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326`; `if_exists` never appears as a metadata key |
| E12.2 | request/API | E1.1 shape with `deps([secret_schema.secret_tbl.secret_col])` | real `refused_dependency` fixture first (positive witness, arm confirmed), then **full serialized-result scan**: the entire emitted JSON contains none of `secret_schema`/`secret_tbl`/`secret_col`; the only dependency fact is `dependency_witness_count:1` inside the refusal finding's metadata |
| E12.3 | enrich/apply | non-empty state: table registry `{schema:"s",table:"t",columns:[{name:"g",type:"geometry"}]}` + SERVER channel entry `s1:OrderedPresent`; SRS statement `DROP SPATIAL REFERENCE SYSTEM 4326;` arm `unproven` | the SRS statement's **own apply** leaves table/schema/SERVER channels value-identical (fidelity-snapshot witness); its only writes are `srsExistenceStale[4326]` on `batchState` — flag writes are not channel mutations |
| E12.4 | enrich/apply | `DROP TABLE t; DROP SPATIAL REFERENCE SYSTEM 4326;` | observation point = the table statement's **own apply** (before the SRS statement runs): `srsDepsStale` set, `srsExistenceStale` untouched, SERVER channel untouched; the SRS S2's own apply later sets `srsExistenceStale[4326]` — the two writer classes stay disjoint at their own apply points |
| E12.5 | enrich/apply | `DROP SERVER s; DROP SPATIAL REFERENCE SYSTEM 4326;` under `privilege_audit_mode=observed_account_new_session`; healthy SERVER witness `CURRENT_USER()='u'@'%'` + `GRANT SUPER ON *.*`; SRS fixture `exist(row) deps([])` | `L=1` attributed to the SERVER statement (request-level loader, once); SRS lands `unproven` gaps `[D,P]` + 1 reserved notice — a SERVER privilege premise never substitutes for the unimplemented SRS one |

## Probe plan — `PLAN / UNEXECUTED` (registered, not run)

Fixed statement `DROP SPATIAL REFERENCE SYSTEM 100001;` (outside every reserved range → no advisory). Providerless (no connection flags). Privilege mode off (no `privilege_audit_mode`). Binary `bin/deltascope` per the execution-order build record. Every probe: stdout and stderr captured to separate files; `$?` captured **immediately** after the CLI command, before anything else runs.

Config file `policy-srs.yaml` (fixed text, identical for all three):

```yaml
dialect: mysql
rules:
  ddl.mysql.srs.drop.srid0:
    enabled: true
    level: blocker
  ddl.mysql.srs.drop.dependency:
    enabled: true
    level: blocker
  ddl.mysql.srs.drop.reserved:
    enabled: true
    level: notice
```

**P1 — pin `8.0.36` (audited `unproven` arm):**

```bash
bin/deltascope audit --config policy-srs.yaml --dialect mysql --format json --fail-on none --target-version 8.0.36 --sql "DROP SPATIAL REFERENCE SYSTEM 100001;" > /tmp/p1.stdout.json 2> /tmp/p1.stderr.txt; echo $? > /tmp/p1.rc
```

- `/tmp/p1.stdout.json` — JSON report: `$.statements` length 1; `$.statements[0].raw_sql` and `$.statements[0].normalized_sql` = `DROP SPATIAL REFERENCE SYSTEM 100001`; `findings` key **absent** (omitempty); `$.statements[0].evidence_gaps` = `[{rule_id:"ddl.mysql.srs.drop.dependency", reason_code:"unknown_srs_state", required_facts:["srs.existence","srs.column_dependencies","srs.drop.privilege"]}]` (exactly 1 entry, 3 ordered facts); `$.statements[0].coverage.status` = `"unverified"`; `$.unsupported` key absent; `$.verdict` = `"review"`; `$.coverage.status` = `"unverified"`.
- `/tmp/p1.stderr.txt` — empty.
- `/tmp/p1.rc` — `0`: `failOnTriggered` returns `false` under `--fail-on none` regardless of gaps (`cli/audit.go:586–600`), so `Execute` returns `exitOK=0`.

**P2 — no pin (version boundary):**

```bash
bin/deltascope audit --config policy-srs.yaml --dialect mysql --format json --fail-on none --sql "DROP SPATIAL REFERENCE SYSTEM 100001;" > /tmp/p2.stdout.json 2> /tmp/p2.stderr.txt; echo $? > /tmp/p2.rc
```

- `/tmp/p2.stdout.json` — JSON report: `$.unsupported` = one entry `{feature:"drop_spatial_reference_system", sql:"DROP SPATIAL REFERENCE SYSTEM 100001", reason:"parsed by the shared parser but not covered by audited semantics", metadata:{boundary:"unaudited", object:"spatial_reference_system", gate:"version"}}` (`index` omitted — it would be 0); `$.statements[0]` keeps `findings`/`evidence_gaps` keys absent; `$.statements[0].coverage.status` = `"incomplete"`; `$.verdict` = `"review"`; `$.coverage.status` = `"incomplete"`.
- `/tmp/p2.stderr.txt` — empty.
- `/tmp/p2.rc` — `0` (same `--fail-on none` derivation as P1).

**P3 — malformed pin `8.0.x` (typed input error):**

```bash
bin/deltascope audit --config policy-srs.yaml --dialect mysql --format json --fail-on none --target-version 8.0.x --sql "DROP SPATIAL REFERENCE SYSTEM 100001;" > /tmp/p3.stdout.json 2> /tmp/p3.stderr.txt; echo $? > /tmp/p3.rc
```

- `/tmp/p3.stdout.json` — empty (no audit JSON: `ValidateTargetVersion` fails in `RunE` preflight before any audit runs — `cli/audit.go:99–103`).
- `/tmp/p3.stderr.txt` — exactly `target_version must match [v]MAJOR.MINOR.PATCH: "8.0.x"` + newline (`ErrInvalidTargetVersion` wrapped `%w: %q` → `newUserError` → `fmt.Fprintln(stderr, err)`; `SilenceErrors`/`SilenceUsage` are `true` so cobra adds nothing — `cli/root.go:46–47`).
- `/tmp/p3.rc` — `2` (`RunE` sets `*exitCode = exitUser`, `exitUser=2` — `cli/cli.go:15`).

Expected fields are contract-derived and frozen from the real dispatch paths; probe outcomes are recorded verbatim in the execution round — any deviation is a defect, not an amendment.
