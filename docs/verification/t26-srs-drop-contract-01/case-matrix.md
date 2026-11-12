# T26-SRS-DROP-CONTRACT-01 — case matrix (R10)

Companion to `README.md`. Every row is a **contract expectation derived from baseline source** — nothing below was measured this round. Status per `expected`: **CURRENT** = baseline already produces it; **PROPOSED** = this contract selects it for the execution order.

**Observation layers** (one explicit combination per row): `parser/extraction` (claim + declaration only), `adjudication-unit` (spec-level arm evaluation — the only layer where no-producer inputs may appear), `enrich/apply` (bind + adjudication + own apply inside one enrich call), `public-projection` (per-statement serialized fields), `request/API` (full request incl. aggregate coverage/verdict), `CLI` (registered PLAN/UNEXECUTED only). `N/A` = the observation is **not applicable or not observed** — never a "zero output"; "not executed" is claimed only where a stated path proves the read/call never issued (typed exit, gated discriminator). An executed-but-unobserved prefix or sibling read is marked `N/A` in the observed column — never claimed as zero and never described as unexecuted. Aggregate `coverage`/`verdict` are asserted **only** on `request/API` and `CLI` rows. Rows labeled `enrich/apply+pp` observe the enrich/apply seam **plus** the per-statement public-projection fields of the **SRS statement only** — sibling table/SERVER/failure statements' serialized output is `N/A` unless the row names it.

**Read-count seams** (per-statement unless per-request): `R` = `objectResolverFor` forwards (cache misses); `Qexist`/`Qdeps` = issued SRS catalog queries; `Qo` = `select version()` inside **the SRS loader only**; `L` = SERVER privilege-loader calls; `Vd` = request-layer version discovery — **`LoadInstanceFacts` inside `AuditSQL` (`metadata.go:141`) or `prepareMetadataAudit`→`auditmeta.Prepare` on the CLI side (`audit_metadata.go:48`), firing only when a provider/connection exists**: `N/A` providerless, `1` once per request when present, `1(att)` when the read fails. Table/schema reads issued by non-SRS statements are reported in the row text, never folded into the SRS columns and never written as zero while unobserved. An `(att)` suffix = attempted read that failed.

## Shared defaults (concrete)

- **SQL surface**: statements exactly as written per row; default schema `s` for unqualified table names.
- **Request**: dialect `mysql`; `--target-version 8.0.36` (`product=mysql`, validated series) unless the row overrides; `privilege_audit_mode` unset (empty — observation off); `--format json`.
- **Initial batch state**: `contaminated=false`, `srsExistenceStale={}`, `srsDepsStale=false`, objectCache empty, no failure records.
- **Group-8 named fixtures** (R5 — shared, referenced per row):
  - `tbl-t`: existing tracked `s.t` with column `g` of normalized type `geometry` — the target of every DROP/ALTER/COMPACT prefix and the dependency witness `s.t.g`.
  - `tbl-u`: existing tracked `s.u` with column `g` (geometry) — E8.2a's positive-control witness `s.u.g`.
  - `t-new`/`s2-new`: provably absent destinations for CREATE prefixes (never inherit an existing registry entry).
  - `like-src`: existing tracked `s.t` used as `CREATE TABLE t_new LIKE t`'s source.
  - every *normalized* prefix statement's own apply **completes without contaminating** (`contaminated` stays `false` — proven for COMPACT via `boundTargets`→`invalidateDropIdentities`, `batch_state.go:489–497`; drops/alters are ordinary audited transitions on a bound target). A **parse failure record** is not a normalized statement and contaminates through a different seam (`failureBefore` → `metadata.go:397–401`) — rows that use one (E8.3, E9.x) say so explicitly.
  - prefix statements' own metadata reads are **counted separately in row text** (`R/Qe/Qd/Qo` columns are SRS-statement-only); where a row does not observe them it says `N/A` — never zero.
- **Rules**: `ddl.mysql.srs.drop.srid0` `{enabled:true, level:blocker}`, `ddl.mysql.srs.drop.dependency` `{enabled:true, level:blocker}`, `ddl.mysql.srs.drop.reserved` `{enabled:true, level:notice}`; no other rule names the family.
- **Provider wiring**: ObjectResolver-capable provider on a stub driver returning scripted responses in issue order. `ver("x")` = `select version()` inside the SRS loader answering banner `x`; `exist(row)` = one `st_spatial_reference_systems` row (`srs_id` matching, `srs_name='EPSG 4326'`); `exist(empty)` = zero rows; `deps([s.t.g])` = `st_geometry_columns` rows as **three-column `schema/table/column` results** — the matching `SRID` on `SRSDependentColumn` is a loader-constructed fact, not an extra SQL column; `deps([])` = zero rows (`Complete=false` always).
- **Banner sources (R4 fix)**: an `observed` banner is the request-layer `LoadInstanceFacts`/discovery fact (`Vd`, once per request); `ver("x")` is the SRS **loader's own** `select version()` (`Qo`, once per objectResolver call). Both are scripted independently — a row may pin `8.0.36` while the loader still receives `ver("8.0.36")`.
- **Reserved membership**: SRID 4326 ∈ `[0,32767]` — **every B/S2 row on 4326 co-fires exactly one `…reserved` notice**; S0/S1 suppress it. SRID 100001 is outside every reserved range (no advisory).
- **Owned shorthand**: `E`=`srs.existence`, `D`=`srs.column_dependencies`, `P`=`srs.drop.privilege`, `B`=`srs.batch_effects`. S2 owes `{D,P}` always, `E` prepended iff existence unproven.
- **Reads column order**: `R / Qexist / Qdeps / Qo / L`.
- **Serialization**: empty `findings`/`evidence_gaps`/`unsupported` = key-absent (`omitempty`); `index:0` omitted on `UnsupportedDetail`; `$.unsupported` never feeds `Aggregate`'s local verdict — but `AuditSQL` upgrades `pass→review` whenever `len(result.Unsupported)>0` (`service.go:242–247`), so a boundary-only request returns `review`.
- **Named version fixture `healthy-observed`**: `LoadInstanceFacts` returns `spec.InstanceFacts` whose version banner parses to `spec.ParseObservedVersion("8.0.36-log")` → `VersionIdentity{Product:"mysql", Version:"8.0.36", Source:"observed"}` — the shared observed fact for every provider-backed request/API row (Vd=1); rows may override it explicitly.

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
| E3.3 | enrich/apply+pp | `DROP SPATIAL REFERENCE SYSTEM 100001;` | pin 8.0.36 | provider present without ObjectResolver | `unproven`; gaps `[E,D,P]`; `UnreadReason` empty (**CURRENT** shape); statement-level only — aggregate N/A | 0/0/0/0/0 |

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
| E5.4 | enrich/apply+pp | `DROP SPATIAL REFERENCE SYSTEM 32767;` | pin 8.0.36 | `ver("8.0.36") exist(empty) deps([])` | `unproven` + `findings=[{rule_id:"ddl.mysql.srs.drop.reserved",level:"notice",message:"spatial reference system 32767 is in a reserved SRID range",metadata:{srid:32767}}]`; gaps `[E,D,P]` | 1/1/1/1/0 |
| E5.5 | enrich/apply+pp | `DROP SPATIAL REFERENCE SYSTEM 32768;` | pin 8.0.36 | same | `unproven`, `findings` absent — **no** advisory | 1/1/1/1/0 |
| E5.6a | enrich/apply+pp | `DROP SPATIAL REFERENCE SYSTEM 60000000;` | pin 8.0.36 | same | `unproven` + reserved notice | 1/1/1/1/0 |
| E5.6b | enrich/apply+pp | `DROP SPATIAL REFERENCE SYSTEM 59999999;` | pin 8.0.36 | same | `unproven`, no advisory | 1/1/1/1/0 |
| E5.6c | enrich/apply+pp | `DROP SPATIAL REFERENCE SYSTEM 69999999;` | pin 8.0.36 | same | `unproven` + reserved notice | 1/1/1/1/0 |
| E5.6d | enrich/apply+pp | `DROP SPATIAL REFERENCE SYSTEM 70000000;` | pin 8.0.36 | same | `unproven`, no advisory | 1/1/1/1/0 |
| E5.6e | enrich/apply+pp | `DROP SPATIAL REFERENCE SYSTEM 2000000000;` | pin 8.0.36 | same | `unproven` + reserved notice | 1/1/1/1/0 |
| E5.6f | enrich/apply+pp | `DROP SPATIAL REFERENCE SYSTEM 1999999999;` | pin 8.0.36 | same | `unproven`, no advisory | 1/1/1/1/0 |
| E5.6g | enrich/apply+pp | `DROP SPATIAL REFERENCE SYSTEM 2147483647;` | pin 8.0.36 | same | `unproven` + reserved notice | 1/1/1/1/0 |
| E5.6h | enrich/apply+pp | `DROP SPATIAL REFERENCE SYSTEM 2147483648;` | pin 8.0.36 | same | `unproven`, no advisory (fits uint32) | 1/1/1/1/0 |
| E5.7 | parser/extraction | `DROP SPATIAL REFERENCE SYSTEM -1;` | pin 8.0.36 | — | **CURRENT** unclaimed → base-grammar parse diagnostic; never a `SpatialReference` statement; all SRS seams N/A | N/A |
| E5.8 | parser/extraction | `DROP SPATIAL REFERENCE SYSTEM 4294967296;` | pin 8.0.36 | — | unclaimed → base-grammar diagnostic (uint32 overflow) | N/A |
| E5.9 | parser/extraction | `DROP SPATIAL REFERENCE SYSTEM 'x';` and `DROP SPATIAL REFERENCE SYSTEM foo;` | pin 8.0.36 | — | both unclaimed → base-grammar diagnostics | N/A |
| E5.10 | parser/extraction | `DROP SPATIAL REFERENCE SYSTEM 1 EXTRA;` and `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 1 X;` | pin 8.0.36 | — | both unclaimed (`len(rest)!=0`) | N/A |
| E5.11 | request/API | `DROP SPATIAL REFERENCE SYSTEM 2000000000;` | pin 8.0.36 | `ver("8.0.36") exist(row) deps([s.t.g])` | `refused_dependency` (reserved range but positive witness) — advisory **suppressed** on refusal arms; findings carry only the dependency blocker | 1/1/1/1/0 |

## E6 — same SRID ordering

| case | layer | SQL sequence | version | provider | expected | reads (per stmt) |
|---|---|---|---|---|---|---|
| E6.1 | enrich/apply+pp | `DROP SPATIAL REFERENCE SYSTEM 4326; DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | `ver("8.0.36") exist(empty) deps([])` | stmt1 `unproven` gaps `[E,D,P]` + 1 reserved notice → mints `srsExistenceStale[4326]`; stmt2 arm `blocked` gaps `[B]` + 1 reserved notice — bind exits **before** the cache is consulted | stmt1 1/1/1/1/0; stmt2 0/0/0/0/0 |
| E6.2 | enrich/apply+pp | `DROP SPATIAL REFERENCE SYSTEM 0; DROP SPATIAL REFERENCE SYSTEM 0;` | pin 8.0.36 | `ver("8.0.36") exist(row) deps([])` | stmt1 `refused_srid0` (reads still fire at its bind); stmt2 `refused_srid0` — refusal keeps facts consumable; stmt2 hits objectCache (R=0) and re-adjudicates identically | stmt1 1/1/1/1/0; stmt2 0/0/0/0/0 |
| E6.3 | enrich/apply+pp | `DROP SPATIAL REFERENCE SYSTEM 4326; DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | `ver("8.0.36") exist(row) deps([s.t.g])` | stmt1 `refused_dependency`; stmt2 `refused_dependency` — cache-hit witness still consumable (refusal mints nothing) | stmt1 1/1/1/1/0; stmt2 0/0/0/0/0 |

## E7 — boundary SRS writes and different-SRID control

| case | layer | SQL sequence | version | provider | expected | reads |
|---|---|---|---|---|---|---|
| E7.1 | enrich/apply+pp | `CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 4326 DEFINITION 'x'; DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | stmt1 bind `ver("8.0.36") exist(empty) deps([])` | stmt1 **CURRENT** boundary `{boundary:"implementation",object:"spatial_reference_system"}` — binds and reads, mints `srsExistenceStale[4326]`; stmt2 arm `blocked` gaps `[B]` + 1 reserved notice | stmt1 1/1/1/1/0; stmt2 0/0/0/0/0 |
| E7.2 | enrich/apply+pp | `CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 4326 DEFINITION 'x'; DROP SPATIAL REFERENCE SYSTEM 100001;` | pin 8.0.36 | stmt1 `ver exist(empty) deps([])`; stmt2 `ver("8.0.36") exist(row) deps([s.t.g])` | stmt1 boundary → `srsExistenceStale[4326]` only; stmt2 SRID 100001 unaffected → `refused_dependency` (valid witness), `findings` absent except the blocker (100001 not reserved) | stmt1 1/1/1/1/0; stmt2 1/1/1/1/0 |

## E8 — table/schema writes → dependency invalidation

Shared setup: existence is `exist(row)` for SRID 4326 unless marked. Prefix statements use the named fixtures from shared defaults (`tbl-t`, `tbl-u`, `t-new`, `s2-new`, `like-src`); each row declares its prefix's own reads explicitly — `N/A` means unobserved this slice, never zero.

| case | layer | SQL sequence | provider | expected | reads |
|---|---|---|---|---|---|
| E8.1 | enrich/apply+pp | `DROP TABLE t; DROP SPATIAL REFERENCE SYSTEM 4326;` | prefix on `tbl-t` (existing `s.t`), own reads `N/A`; stmt2 `ver("8.0.36") exist(row) deps([s.t.g])` | `DROP TABLE` touching → `srsDepsStale`; stmt2's freshly-read positive witness is DB-pre-state (the simulated drop precedes it) → invalid → `unproven` gaps `[D,P]` + 1 reserved notice; existence stays consumable (`Exists=true`) | stmt2 1/1/1/1/0 |
| E8.2a | enrich/apply+pp | `CREATE TABLE t_new (i INT); DROP SPATIAL REFERENCE SYSTEM 4326;` | prefix destination `t_new` (absent — fixture `t-new`), own reads `N/A`; stmt2 `ver("8.0.36") exist(row) deps([s.u.g])` | `CREATE TABLE t_new` complete non-spatial column list → inert; the positive control sits on the **existing** fixture `tbl-u` (`s.u.g`) — witness valid → `refused_dependency`; `evidence_gaps` absent; `coverage.status:"complete"` | stmt2 1/1/1/1/0 |
| E8.2b | enrich/apply+pp | `CREATE TABLE t_new (i INT); DROP SPATIAL REFERENCE SYSTEM 4326;` | prefix destination `t_new` (absent), own reads `N/A`; stmt2 `ver("8.0.36") exist(row) deps([])` | inert prefix + empty-incomplete deps → `unproven` gaps `[D,P]` + 1 reserved notice (unproven-empty, **not** stale) | stmt2 1/1/1/1/0 |
| E8.3 | enrich/apply+pp | `CREATE TABLE t_new (g GEOMETRY SRID 4326); DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt1 produces a **`parseFailure` record**, not a normalized statement — bound route (R8-C/R9-B): no recognizer claims `CREATE TABLE` (`parser.go:45–70`); `planAdvancedSupplements`→`scanColumnDef` recognizes only `default`/`visible`/`invisible` sites and `hasWork` needs defaults/visibility/setter/burstable members (`supplement.go:190–192, 568–615, 913–932`) → `hasWork=false`; the **first bound rejection point is `GEOMETRY` at the column `Type` position** — `Type:` offers only `NumericType`/`StringType`/`DateAndTimeType` with no identifier alternative (`parser.y:13663–13667, 3795–3801`); `srid` and `geometry` have zero grammar tokens → syntax error → `parseFailure` append + `continue` (`parse.go:57–75`) | stmt1 failure record authored (1,1) precedes stmt2 (1,~45) → `failureBefore` → `contaminated=true` → stmt2 **arm B** gaps `[B]` + 1 reserved notice; no resolver call; scripted `ver/exist/deps` never reached | stmt2 0/0/0/0/0 |
| E8.3b | enrich/apply+pp | `CREATE TABLE t_new (g GEOMETRY); DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt1 produces a **`parseFailure` record** — the R8 normalized-CREATE claim stays withdrawn (erratum E-13); the **proven real path** (R10-②/③): bare `GEOMETRY` is a bare `Identifier` (per-word zero-hit in all five grammar/lexer files) and the `Type:` subtree (`parser.y:13663–14216`) starts only on quoted keyword tokens — zero `identifier` in range — so `ColumnDef` rejects at the type position → `parseFailure` append + `continue` (`parse.go:57–75`); CREATE supplement provably no-op (`setter`/`burstable` unreachable in the create branch, `supplement.go:250–300`) | same failure-record → `failureBefore` → `contaminated` → stmt2 **arm B** gaps `[B]` + 1 reserved notice; no resolver call | stmt2 0/0/0/0/0 |
| E8.4 | enrich/apply+pp | `ALTER TABLE t COMPACT TIFLASH REPLICA; DROP SPATIAL REFERENCE SYSTEM 4326;` | prefix on `tbl-t`, own reads `N/A`; stmt2 `ver("8.0.36") exist(row) deps([s.t.g])` | stmt1 is a **statically provable vendor boundary under MySQL** (`coverage_boundary.go:189–194`) whose normalized model carries `Table=t` (`extractor.go:813–832`) → `TableTargets()≠∅` → own apply `boundTargets`→`invalidateDropIdentities` (`batch_state.go:489–497, 2934–2951`) never writes `contaminated` → touching → `unproven` gaps `[D,P]` + 1 reserved notice | stmt1 boundary (no SRS bind); stmt2 1/1/1/1/0 |
| E8.5 | enrich/apply+pp ×2, same `batchState` | call1 `DROP TABLE t;` call2 `DROP SPATIAL REFERENCE SYSTEM 4326;` | call1 prefix on `tbl-t`, own reads `N/A`; call2 `ver("8.0.36") exist(row) deps([s.t.g])` | `srsDepsStale` persists across calls (flag lives on `batchState`, cache is per-call) → call2 `unproven` gaps `[D,P]` + 1 reserved notice | call2 1/1/1/1/0 |
| E8.6 | enrich/apply+pp | `DROP SPATIAL REFERENCE SYSTEM 4326; DROP TABLE t; DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt1 `ver("8.0.36") exist(row) deps([s.t.g])`; stmt2 on `tbl-t`, own reads `N/A`; stmt3 = cache hit | stmt1 `refused_dependency`; `DROP TABLE` → `srsDepsStale`; stmt3 cache hit returns the same witness — existence consumable, dependency stale → `unproven` gaps `[D,P]` + 1 reserved notice | stmt1 1/1/1/1/0; stmt3 0/0/0/0/0 |
| E8.7 | enrich/apply+pp | `DROP SPATIAL REFERENCE SYSTEM 4326; ALTER TABLE t DROP COLUMN g; DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt1 `ver("8.0.36") exist(row) deps([s.t.g])`; stmt2 on `tbl-t` drops `g`, own reads `N/A`; stmt3 = cache hit | stmt1 `refused_dependency` on column `g`; `ALTER TABLE t DROP COLUMN g` (column action) → `srsDepsStale`; stmt3 cache hit — witness now names a column the batch dropped → invalid → `unproven` gaps `[D,P]` + 1 reserved notice | stmt1 1/1/1/1/0; stmt3 0/0/0/0/0 |
| E8.8 | enrich/apply+pp | `ALTER TABLE t ENGINE=InnoDB; DROP SPATIAL REFERENCE SYSTEM 4326;` | prefix on `tbl-t`, own reads `N/A`; stmt2 `ver("8.0.36") exist(row) deps([s.t.g])` | complete action list = single `table_option`/`engine` member → inert → witness valid → `refused_dependency` | stmt2 1/1/1/1/0 |
| E8.9 | enrich/apply+pp | `DROP SCHEMA s2; DROP SPATIAL REFERENCE SYSTEM 4326;` | prefix on existing `s2`, own reads `N/A`; stmt2 `ver("8.0.36") exist(row) deps([s.t.g])` | schema drop → touching → `unproven` gaps `[D,P]` + 1 reserved notice | stmt2 1/1/1/1/0 |
| E8.10 | enrich/apply+pp | `CREATE SCHEMA s2_new; DROP SPATIAL REFERENCE SYSTEM 4326;` | prefix destination `s2_new` (absent — fixture `s2-new`), own reads `N/A`; stmt2 `ver("8.0.36") exist(row) deps([s.t.g])` | `CREATE SCHEMA` provably empty → inert → `refused_dependency` | stmt2 1/1/1/1/0 |
| E8.11 | enrich/apply+pp | `CREATE TABLE t_new LIKE t; DROP SPATIAL REFERENCE SYSTEM 4326;` | prefix destination `t_new`, source `like-src` (`s.t`), own reads `N/A`; stmt2 `ver("8.0.36") exist(row) deps([s.t.g])` | `LIKE`-source columns unvouched → completeness unproven → touching → `unproven` gaps `[D,P]` + 1 reserved notice | stmt2 1/1/1/1/0 |

## E9 — contamination and failure-record ordering

| case | layer | SQL sequence | provider | expected | reads |
|---|---|---|---|---|---|
| E9.1 | enrich/apply+pp | `GARBAGE; DROP SPATIAL REFERENCE SYSTEM 4326;` | `ver("8.0.36") exist(row) deps([s.t.g])` scripted but never reached | failure record (line 1, col 1–7) precedes the statement → `contaminated` before stmt2 → arm `blocked` gaps `[B]` + 1 reserved notice; no resolver call | stmt2 0/0/0/0/0 |
| E9.2 | enrich/apply+pp | `GARBAGE; DROP SPATIAL REFERENCE SYSTEM 4326;` (one line) | same | failure record authored at (line 1, col 1); SRS statement authored at (line 1, col 10) — `failureBefore(1,1,1,10)=true` → `blocked` per authored position | stmt2 0/0/0/0/0 |
| E9.3 | enrich/apply+pp ×2, same `batchState` | call1 `DROP SPATIAL REFERENCE SYSTEM 0; GARBAGE;` — stmt1 authored (1,1)–(1,32), failure record authored (1,34); call2 `DROP SPATIAL REFERENCE SYSTEM 4326;` | call1 stmt1 `ver("8.0.36") exist(row) deps([])` | initial state clean; call1 stmt1 `refused_srid0` (reads fire; refusal mints no stale flag); trailing `GARBAGE` mints `contaminated` via tail sweep; call2 → `blocked` gaps `[B]` + 1 reserved notice — fresh per-call cache cannot launder `batchState` contamination | call1 stmt1 1/1/1/1/0; call2 0/0/0/0/0 |

## E10 — version/product inputs

**Group default SQL** (every row, no exceptions): `DROP SPATIAL REFERENCE SYSTEM 4326;` — boundary rows never reach adjudication, so the reserved advisory is unobservable there; audited rows take the documented advisory. **Version-fact discipline (R5)**: `observed` = the scripted `LoadInstanceFacts` request fact (`Vd=1` whenever a provider is present — the shared `healthy-observed` fixture unless a row names its own banner); `ver("x")` = the SRS loader's own `select version()` (`Qo`, pinned per row); a pin is the caller's `target_version`. The three never substitute for each other. Providerless rows have `Vd=N/A` — no discovery surface exists. Request/API rows never assert a process rc (`N/A`); only `CLI` rows do.

| case | layer | input | expected | reads |
|---|---|---|---|---|
| E10.1 | request/API | dialect mysql, no pin, provider present, observed `"unknown"` (no leading `\d.\d.\d` — `observedVersionPrefix` fails); loader `ver("8.0.36")`, `exist(empty)`, `deps([])` | `resolvedVersion=nil` → `gate:"version"` boundary — unknown ≠ feature-absent; the request-layer observed banner decides **qualification** (no proven 8.0.4+ → gate fires), while the SRS loader's own `ver("8.0.36")` decides its **catalog-read path** (proven 8.0+ → existence+deps reads issue): `$.unsupported` gains the frozen gate entry; `coverage.status:"incomplete"` both levels; `$.verdict:"review"` (AuditSQL `pass→review` upgrade on non-empty Unsupported) | 1/1/1/1/0 (Vd=1) |
| E10.2 | request/API | dialect mysql, no pin, provider present, observed `healthy-observed`; loader `ver("8.0.36")`, `exist(empty)`, `deps([])` | audited → `unproven`; 1 reserved notice; gaps `[E,D,P]`; `coverage.status:"unverified"`; `$.verdict:"review"` (fixed facts, fixed arm) | 1/1/1/1/0 (Vd=1) |
| E10.3 | request/API | dialect mysql, no pin, provider present, observed `"5.7.0-m14"` (prefix parses → proven 5.7.0); loader `ver("5.7.0-m14")` — the loader sees the same banner via its own `select version()` | `gate:"version"` boundary (proven ≠8.0.4+); provider: proven pre-8.0 → `version_inapplicable`, no catalog reads; `coverage.status:"incomplete"`; `$.verdict:"review"` | 1/0/0/1/0 (Vd=1) |
| E10.3b | request/API | dialect mysql, no pin, provider present, observed `"8.0.3-log"`; loader `ver("8.0.3-log")`, `exist(empty)`, `deps([])` | `gate:"version"` boundary — **proven pre-feature**: the request-layer observed `8.0.3-log` decides qualification (proven ≠8.0.4+ → gate); the loader's own `ver("8.0.3-log")` is 8.0.x → its catalog-read path still issues existence+deps; `coverage.status:"incomplete"`; `$.verdict:"review"` | 1/1/1/1/0 (Vd=1) |
| E10.3c | request/API | pin `8.0.3`; providerless (no loader calls) | `gate:"version"` boundary (pin proves patch 3 < 4); `coverage.status:"incomplete"`; `$.verdict:"review"` | 0/0/0/0/0 (Vd=N/A) |
| E10.3d | request/API | pin `8.0.4`; providerless | audited → `unproven` gaps `[E,D,P]` (floor is inclusive) + 1 reserved notice; `coverage.status:"unverified"`; `$.verdict:"review"` | 0/0/0/0/0 (Vd=N/A) |
| E10.4 | request/API | pin `5.7.44`; providerless | `gate:"version"` boundary; `coverage.status:"incomplete"`; `$.verdict:"review"` | 0/0/0/0/0 (Vd=N/A) |
| E10.5 | request/API | dialect `tidb`, no pin, provider present, observed `"8.0.11-TiDB-v8.5.0"` | **CURRENT** vendor boundary (`metadata:{boundary:"vendor"}`) — attribution split: parser/extraction produces the TiDB vendor boundary; `LoadInstanceFacts` supplies the observed banner whose product parses to `tidb` (request-layer product consistency, `resolvedVersion{product:"tidb"}`); `AuditSQL` yields the final aggregate `$.verdict:"review"`. `bindMySQLSRS` exits before resolver; serialized: `$.unsupported` vendor entry, `coverage.status:"incomplete"` | 0/0/0/0/0 (Vd=1) |
| E10.6 | request/API | pin `8.0.x`; providerless | **CURRENT** `ErrInvalidTargetVersion` before enrichment — no audit result, no SRS reads | 0/0/0/0/0 (Vd=N/A) |
| E10.7 | request/API | dialect `mysql`, no pin, provider present — real `LoadInstanceFacts` returns TiDB banner `"8.0.11-TiDB-v8.5.0"` | **CURRENT** `ErrDialectProductMismatch` — typed error raised inside `resolvedVersion` (`metadata.go:152–155`); SRS seams exactly zero (typed exit precedes them — explicit zero, not N/A); the discovery read itself succeeded | 0/0/0/0/0 (Vd=1) |
| E10.8 | request/API | pin `8.0.36`, provider present — `LoadInstanceFacts` returns `"8.0.37-log"` | **CURRENT** `ErrTargetVersionMismatch` — typed error at the same seam; SRS seams zero (typed exit precedes them) | 0/0/0/0/0 (Vd=1) |

## E11 — error paths (provider-layer vs request-layer kept distinct)

Shared fixture (all rows): SQL `DROP SPATIAL REFERENCE SYSTEM 4326;`, pin `8.0.36`, provider present, scripted responses up to the failing seam; initial state clean. A sentinel is a named injected `error` value. **E11.1–E11.4 only**: public error text is the bounded string `metadata object read failed` — sentinels never appear in it. **E11.5 is a different path**: the request-layer `fmt.Errorf("load instance facts: %w", err)` wrap carries the sentinel text verbatim (see row).

| case | layer | fixture (SQL + failing seam + sentinel) | expected | reads |
|---|---|---|---|---|
| E11.1 | enrich/apply | resolver forward returns sentinel `srs-sentinel-7` | `metadataObjectReadError` propagates → enrich typed error terminating the statement's **own bind** (no adjudication, no apply, no gap substitute); `errors.Is` sees `srs-sentinel-7`; public text `metadata object read failed` | 1(att)/0/0/0/0 |
| E11.2 | enrich/apply | `ver("8.0.36")` succeeds; existence query fails with sentinel `srs-exist-fail-2` | same wrapper shape propagates; deps read never issued; terminates the statement's own bind; `errors.Is` sees `srs-exist-fail-2`; public text `metadata object read failed` | 1/1(att)/0/1/0 |
| E11.3 | enrich/apply | `ver("8.0.36") exist(row)` succeed; deps query fails with sentinel `srs-deps-fail-3` | same wrapper shape; terminates the own bind; `errors.Is` sees `srs-deps-fail-3`; public text `metadata object read failed` | 1/1/1(att)/1/0 |
| E11.4 | enrich/apply | `select version()` inside the SRS loader fails with sentinel `srs-ver-fail-4` | propagates before any catalog read; same wrapper/termination; `errors.Is` sees `srs-ver-fail-4`; public text `metadata object read failed` | 1/0/0/1(att)/0 |
| E11.5 | request/API | `LoadInstanceFacts` (`metadata.go:141`) returns sentinel `srs-discover-fail-5` | request/API error = `fmt.Errorf("load instance facts: %w", err)` at `metadata.go:143` — a plain `%w` wrap, so the sentinel text **does** appear in the message and `errors.Is` sees `srs-discover-fail-5`; `AuditSQL` returns a **zero-value `report.Result{}` plus the error** at `service.go:182–184` — no assertions are made on auditable-result projections (`coverage`/`verdict`/statement fields are `N/A`, not zero); terminates before any enrichment-loop iteration (SRS seams exactly 0 — explicit zero, not `N/A`); the path is request-layer discovery, not the `metadataObjectReadError` object-read wrapper and not the loader's `select version()` (E11.4) | 0/0/0/0/0 (Vd=1 att) |

## E12 — redaction and cross-channel isolation

| case | layer | setup | expected |
|---|---|---|---|
| E12.1 | request/API | E1.1 fixture | `$.statements[0].raw_sql` and `$.statements[0].normalized_sql` both exactly `DROP SPATIAL REFERENCE SYSTEM 4326`; E1.2 fixture → `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326`; `if_exists` never appears as a metadata key |
| E12.2 | request/API | E1.1 shape with `deps([secret_schema.secret_tbl.secret_col])` | real `refused_dependency` fixture first (positive witness, arm confirmed), then **full serialized-result scan**: the entire emitted JSON contains none of `secret_schema`/`secret_tbl`/`secret_col`; the only dependency fact is `dependency_witness_count:1` inside the refusal finding's metadata |
| E12.3 | enrich/apply | non-empty state: table registry `{schema:"s",table:"t",columns:[{name:"g",type:"geometry"}]}` + SERVER channel entry `s1:OrderedPresent`; SRS statement `DROP SPATIAL REFERENCE SYSTEM 4326;` with SRS facts `ver("8.0.36") exist(row) deps([])` → arm `unproven` (S2 proven by facts, not assumed) | the SRS statement's **own apply** leaves table/schema/SERVER channels value-identical (fidelity-snapshot witness); its only writes are `srsExistenceStale[4326]` on `batchState` — flag writes are not channel mutations; serialized fields are `N/A` — internal assertions only |
| E12.4 | enrich/apply | `DROP TABLE t; DROP SPATIAL REFERENCE SYSTEM 4326;` — reuses the `tbl-t` fixture from E8.1 verbatim | observation point = the table statement's **own apply** (before the SRS statement runs): `srsDepsStale` set, `srsExistenceStale` untouched, SERVER channel untouched; the SRS S2's own apply later sets `srsExistenceStale[4326]` — the two writer classes stay disjoint at their own apply points; serialized fields `N/A` |
| E12.5 | enrich/apply+pp | `DROP SERVER s; DROP SPATIAL REFERENCE SYSTEM 4326;` under `privilege_audit_mode=observed_account_new_session`; **consumable SERVER witness**: privilege-loader-capable provider returning `CURRENT_USER()` account `'u'@'%'` and a complete account-bound GrantLines set containing `GRANT SUPER ON *.* TO 'u'@'%'`; **SERVER object resolver response** = healthy row for `s` (bound server identity present) so the SERVER premise is consumable (not merely scripted); SRS fixture `ver("8.0.36") exist(row) deps([])` | SERVER statement's own seams: its resolver/read calls are `N/A` columns here — `L=1` is attributed to it (request-level privilege loader, once); SRS statement `unproven` gaps `[D,P]` + 1 reserved notice (serialized `required_facts:["srs.column_dependencies","srs.drop.privilege"]` — existence proven by `exist(row)`, so `E` is **not** owed); SRS reads `1/1/1/1/0` — a SERVER privilege premise never substitutes for the unimplemented SRS one; SERVER statement's serialized output `N/A` |

## Probe plan — `PLAN / UNEXECUTED` (registered, not run)

Fixed statement `DROP SPATIAL REFERENCE SYSTEM 100001;` (outside every reserved range → no advisory). Providerless (no connection flags — `connection.Enabled()` is false so `prepareMetadataAudit` never runs and `Vd=N/A` for all three). Privilege mode off (no `privilege_audit_mode`). Binary `bin/deltascope` per the execution-order build record.

**Shell discipline (R4):** probes run under a plain interactive/`bash` shell **without `set -e`/errexit** so a non-zero exit cannot abort the script before the rc capture; stdout and stderr go to separate files; `echo $?` is captured **immediately** on the same logical line, before anything else runs.

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
- Envelope (R5-A): `$.version` = `{product:"mysql", version:"8.0.36", major:8, minor:0, patch:36, source:"target", validated_range:true}` (`resolvedVersionIdentity`, service.go:292–299); `$.fail_on_triggered` = `false`; `$.context` = `{mode:"offline", dialect:"mysql", dialect_source:"flag", note:"existence not checked (no database connection)", unproven:[offline list]}`; `$.rule_summary` present (object `loaded`/`applicable`/`skipped`); `$.diagnostics` key absent; `$.explanation`/`$.statements[0].explanation` absent (stripped by `resultWithoutAggregateExplanations`, audit.go:514–519).
- `/tmp/p1.stderr.txt` — empty.
- `/tmp/p1.rc` — `0`: an audited `unproven` arm produces **no** `$.unsupported` entries, so `exitCodeForResult` falls through its `len(result.Unsupported)>0` check to `failOnTriggered`, which returns `false` under `--fail-on none` (`cli/audit.go:586–618`) → `exitOK=0`.

**P2 — no pin (version boundary):**

```bash
bin/deltascope audit --config policy-srs.yaml --dialect mysql --format json --fail-on none --sql "DROP SPATIAL REFERENCE SYSTEM 100001;" > /tmp/p2.stdout.json 2> /tmp/p2.stderr.txt; echo $? > /tmp/p2.rc
```

- `/tmp/p2.stdout.json` — JSON report: `$.unsupported` = one entry `{feature:"drop_spatial_reference_system", sql:"DROP SPATIAL REFERENCE SYSTEM 100001", reason:"parsed by the shared parser but not covered by audited semantics", metadata:{boundary:"unaudited", object:"spatial_reference_system", gate:"version"}}` (`index` omitted — it would be 0); `$.statements[0]` keeps `findings`/`evidence_gaps` keys absent; `$.statements[0].coverage.status` = `"incomplete"`; `$.coverage.status` = `"incomplete"`; **`$.verdict` = `"review"`** (R5-A correction — three distinct observation points): `Aggregate` derives the **local** verdict from finding counts + `hasEvidenceGaps` only → `pass` (`result.go:161–190`), but `AuditSQL` then runs its post-step (`service.go:242–247`): `len(result.Unsupported)>0` → append one unsupported diagnostic **and** upgrade `pass→review`, returning `ErrUnsupportedStatement`; the CLI renderer passes that returned value through unchanged.
- Envelope (R5-A): `$.diagnostics` = one entry `{classification:"unsupported_statement", reason:"DeltaScope recognized this statement or feature but does not audit it yet", action_hint:"treat this statement as not covered by DeltaScope policy checks; review it manually or track support in a future DeltaScope release", audited:false, dialect:"mysql"}` (`newUnsupportedStatementDiagnostic`, diagnostics.go:82–91); `$.version` key absent (no pin, providerless → `resolvedVersionIdentity` nil); `$.fail_on_triggered` = `false`; `$.context` = offline context as P1; `$.rule_summary` present.
- `/tmp/p2.stderr.txt` — empty: `RunE` treats `ErrUnsupportedStatement` as renderable (`audit.go:155`, `176`) — the typed error is swallowed after `exitCodeForResult` assigns rc; it is never printed.
- `/tmp/p2.rc` — **`1`** (corrected from R3's assumed `0`): `exitCodeForResult` checks `len(result.Unsupported) > 0` **first** and returns `exitAudit` (=1) before `failOnTriggered` is ever consulted (`cli/audit.go:611–618`) — a non-empty `$.unsupported` outranks `--fail-on none`; `hasRenderableAuditResult` is true so the report is rendered before the exit assignment (`audit.go:153–179`).

**P3 — malformed pin `8.0.x` (typed input error):**

```bash
bin/deltascope audit --config policy-srs.yaml --dialect mysql --format json --fail-on none --target-version 8.0.x --sql "DROP SPATIAL REFERENCE SYSTEM 100001;" > /tmp/p3.stdout.json 2> /tmp/p3.stderr.txt; echo $? > /tmp/p3.rc
```

- `/tmp/p3.stdout.json` — empty (no audit JSON: `ValidateTargetVersion` fails in `RunE` preflight before any audit runs — `cli/audit.go:99–103`).
- `/tmp/p3.stderr.txt` — exactly `target_version must match [v]MAJOR.MINOR.PATCH: "8.0.x"` + newline (`ErrInvalidTargetVersion` wrapped `%w: %q` → `newUserError` → `fmt.Fprintln(stderr, err)`; `SilenceErrors`/`SilenceUsage` are `true` so cobra adds nothing — `cli/root.go:46–47`).
- `/tmp/p3.rc` — `2` (`RunE` sets `*exitCode = exitUser`, `exitUser=2` — `cli/cli.go:15`).

Expected fields are contract-derived and frozen from the real dispatch paths; probe outcomes are recorded verbatim in the execution round — any deviation is a defect, not an amendment.
