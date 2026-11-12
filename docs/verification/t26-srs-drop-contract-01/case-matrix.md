# T26-SRS-DROP-CONTRACT-01 — case matrix (R2)

Companion to `README.md`. Every row is a **contract expectation derived from baseline source** — nothing below was measured this round. Status legend per row's `expected`: **CURRENT** = baseline already produces it; **PROPOSED** = this contract selects it for the execution order; **UNREACHABLE/DEFERRED** noted where they apply.

Read-count seams (all per-statement unless marked per-request): `R` = `objectResolverFor` forwards (cache misses); `Qexist`/`Qdeps` = issued catalog queries; `Qo` = `select version()` issued **inside the SRS loader only** — request-level version discovery (banner/`select version()` at connect time) is a different layer and is reported separately where a row depends on it; `L` = SERVER privilege-loader calls (always 0 for pure-SRS fixtures). `N/A` = the layer never runs (parser-only rows: no bind, no provider). `0` = the layer ran and issued nothing. An `(att)` suffix marks an attempted read that failed.

## Shared defaults (concrete — no placeholders)

- **Dialect/format**: `mysql`, `--format json`.
- **Rules config** (fixed for every enrich/request row): `ddl.mysql.srs.drop.srid0` blocker, `ddl.mysql.srs.drop.dependency` blocker, `ddl.mysql.srs.drop.reserved` notice; no other rule names the SRS family. Privilege mode: **empty** (observation off — no `privilege_audit_mode` key).
- **Version**: `--target-version 8.0.36` (pin, `product=mysql`, source=target) unless the row overrides; observed banner fixtures say so explicitly.
- **Provider**: ObjectResolver-capable provider on a stub driver returning, in issue order, the row's scripted responses. Fixture vocabulary: `ver("8.0.36")` = `select version()` banner answering `8.0.36-log`; `exist(row)` = one `st_spatial_reference_systems` row (`srs_id` matching, `srs_name='EPSG 4326'`); `exist(empty)` = zero rows; `deps([t.g])` = one `st_geometry_columns` row (`TABLE_SCHEMA='s'`, `TABLE_NAME='t'`, `COLUMN_NAME='g'`, `SRS_ID` matching); `deps([])` = zero rows (`Complete=false` always).
- **Batch**: single enrich call unless `×2` marked; `batchState` carries over only where stated.
- **Owned shorthand**: `E`=`srs.existence`, `D`=`srs.column_dependencies`, `P`=`srs.drop.privilege`, `B`=`srs.batch_effects`.
- **Public SQL** is always `DROP SPATIAL REFERENCE SYSTEM [IF EXISTS] <decimal srid>`; asserted verbatim in E12 only.
- **Reads column order**: `R / Qexist / Qdeps / Qo / L`.

## E1 — present + positive dependency witness

| case | layer | SQL | version | provider → response | expected | reads R/Qe/Qd/Qo/L |
|---|---|---|---|---|---|---|
| E1.1 | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | `ver("8.0.36") exist(row) deps([t.g])` | **PROPOSED** `refused_dependency` → verdict reject; `findings=[{rule_id:"ddl.mysql.srs.drop.dependency", level:"blocker", metadata:{srid:4326, outcome:"refused_dependency", dependency_witness_count:1}}]`; `evidence_gaps=[]`; `coverage.status=complete` | 1/1/1/1/0 |
| E1.2 | enrich+request | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326;` | pin 8.0.36 | same as E1.1 | same arm — `IF EXISTS` does not reach it; `normalized_sql` shows the `IF EXISTS` skeleton; no `if_exists` key anywhere | 1/1/1/1/0 |

## E2 — present + empty-but-incomplete dependencies

| case | layer | SQL | version | provider | expected | reads |
|---|---|---|---|---|---|---|
| E2.1 | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | `ver("8.0.36") exist(row) deps([])` | **PROPOSED** `unproven` → review; findings `[]`; gaps `[D,P]`; coverage `unverified`; post → `srsExistenceStale[4326]` only | 1/1/1/1/0 |

## E3 — existence unknown / providerless / no read

| case | layer | SQL | version | provider | expected | reads |
|---|---|---|---|---|---|---|
| E3.1 | request/API | `DROP SPATIAL REFERENCE SYSTEM 100001;` | pin 8.0.36 | providerless (no DSN) | **PROPOSED** `unproven` → review; gaps `[E,D,P]` (`Read=false`, `Dependencies=nil`); `unverified` | 0/0/0/0/0 |
| E3.2 | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | `ver("8.0.36") exist(empty) deps([])` | `unproven`; gaps `[E,D,P]` — zero-row ≠ absent, empty-incomplete ≠ no deps | 1/1/1/1/0 |
| E3.3 | unit (enrich) | `DROP SPATIAL REFERENCE SYSTEM 100001;` | pin 8.0.36 | provider present without ObjectResolver | `unproven`; gaps `[E,D,P]`; `UnreadReason` empty (**CURRENT** shape) | 0/0/0/0/0 |

## E4 — unknown existence + positive dependency

| case | layer | SQL | version | provider | expected | reads |
|---|---|---|---|---|---|---|
| E4.1 | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | `ver("8.0.36") exist(empty) deps([t.g])` | **PROPOSED** `refused_dependency` → reject; gaps `[]` (the witness discharges existence); `complete` | 1/1/1/1/0 |

## E5 — SRID 0 / numeric boundaries / unclaimed forms / reserved ranges

| case | layer | SQL | version | provider | expected | reads |
|---|---|---|---|---|---|---|
| E5.1 | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 0;` | pin 8.0.36 | `ver("8.0.36") exist(row) deps([])` | **PROPOSED** `refused_srid0` → reject; `findings=[{rule_id:"ddl.mysql.srs.drop.srid0", level:"blocker", metadata:{srid:0, outcome:"refused_srid0"}}]`; `evidence_gaps=[]`; `coverage.status=complete`; reserved advisory suppressed (0 ∈ [0,32767] but refusal arm wins) | 1/1/1/1/0 |
| E5.2 | enrich+request | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 0;` | pin 8.0.36 | same as E5.1 | same refusal — `IF EXISTS` never suppresses SRID 0 | 1/1/1/1/0 |
| E5.3 | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 4294967295;` | pin 8.0.36 | `ver("8.0.36") exist(empty) deps([])` | `unproven`, gaps `[E,D,P]`; **no** advisory (4294967295 ∉ reserved) | 1/1/1/1/0 |
| E5.4 | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 32767;` | pin 8.0.36 | `ver("8.0.36") exist(empty) deps([])` | `unproven` + `findings=[{rule_id:"ddl.mysql.srs.drop.reserved", level:"notice", metadata:{srid:32767}}]`; gaps `[E,D,P]` | 1/1/1/1/0 |
| E5.5 | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 32768;` | pin 8.0.36 | same | `unproven`, **no** advisory | 1/1/1/1/0 |
| E5.6a | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 60000000;` | pin 8.0.36 | same | `unproven` + reserved advisory | 1/1/1/1/0 |
| E5.6b | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 59999999;` | pin 8.0.36 | same | `unproven`, **no** advisory | 1/1/1/1/0 |
| E5.6c | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 69999999;` | pin 8.0.36 | same | `unproven` + reserved advisory | 1/1/1/1/0 |
| E5.6d | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 70000000;` | pin 8.0.36 | same | `unproven`, **no** advisory | 1/1/1/1/0 |
| E5.6e | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 2000000000;` | pin 8.0.36 | same | `unproven` + reserved advisory | 1/1/1/1/0 |
| E5.6f | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 1999999999;` | pin 8.0.36 | same | `unproven`, **no** advisory | 1/1/1/1/0 |
| E5.6g | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 2147483647;` | pin 8.0.36 | same | `unproven` + reserved advisory | 1/1/1/1/0 |
| E5.6h | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 2147483648;` | pin 8.0.36 | same | `unproven`, **no** advisory (parses: fits uint32) | 1/1/1/1/0 |
| E5.7 | parser | `DROP SPATIAL REFERENCE SYSTEM -1;` | pin 8.0.36 | — | **CURRENT** unclaimed → base-grammar parse diagnostic; never a `SpatialReference` statement | N/A |
| E5.8 | parser | `DROP SPATIAL REFERENCE SYSTEM 4294967296;` | pin 8.0.36 | — | unclaimed → base-grammar diagnostic (uint32 overflow) | N/A |
| E5.9 | parser | `DROP SPATIAL REFERENCE SYSTEM 'x';` and `DROP SPATIAL REFERENCE SYSTEM foo;` | pin 8.0.36 | — | both unclaimed → base-grammar diagnostics | N/A |
| E5.10 | parser | `DROP SPATIAL REFERENCE SYSTEM 1 EXTRA;` and `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 1 X;` | pin 8.0.36 | — | both unclaimed (`len(rest)!=0`) | N/A |
| E5.11 | enrich+request | `DROP SPATIAL REFERENCE SYSTEM 2000000000;` | pin 8.0.36 | `ver("8.0.36") exist(row) deps([t.g])` | `refused_dependency` (reserved range but positive witness) — advisory **suppressed** on refusal arms; findings carry only the dependency blocker | 1/1/1/1/0 |

## E6 — same SRID ordering

| case | layer | SQL sequence | version | provider | expected | reads (per stmt) |
|---|---|---|---|---|---|---|
| E6.1 | enrich | `DROP SPATIAL REFERENCE SYSTEM 4326; DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | `ver("8.0.36") exist(empty) deps([])` | stmt1 `unproven` gaps `[E,D,P]` → mints `srsExistenceStale[4326]`; stmt2 arm `blocked` gaps `[B]` — bind exits **before** the cache is consulted (existence-stale suppresses read and memoized answer) | stmt1 1/1/1/1/0; stmt2 0/0/0/0/0 |
| E6.2 | enrich | `DROP SPATIAL REFERENCE SYSTEM 0; DROP SPATIAL REFERENCE SYSTEM 0;` | pin 8.0.36 | `ver("8.0.36") exist(row) deps([])` | stmt1 `refused_srid0` (reads still fire at its bind); stmt2 `refused_srid0` — refusal keeps facts consumable; stmt2 hits the objectCache (R=0) and re-adjudicates identically | stmt1 1/1/1/1/0; stmt2 0/0/0/0/0 |
| E6.3 | enrich | `DROP SPATIAL REFERENCE SYSTEM 4326; DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | `ver("8.0.36") exist(row) deps([t.g])` | stmt1 `refused_dependency`; stmt2 `refused_dependency` — cache-hit witness still consumable (refusal mints nothing) | stmt1 1/1/1/1/0; stmt2 0/0/0/0/0 |

## E7 — boundary SRS writes and different-SRID control

| case | layer | SQL sequence | version | provider | expected | reads |
|---|---|---|---|---|---|---|
| E7.1 | enrich | `CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 4326 DEFINITION 'x'; DROP SPATIAL REFERENCE SYSTEM 4326;` | pin 8.0.36 | `ver("8.0.36") exist(empty) deps([])` for stmt1's bind | stmt1 **CURRENT** boundary `object=spatial_reference_system` — binds and reads (boundary does not suppress reads), mints `srsExistenceStale[4326]`; stmt2 arm `blocked` gaps `[B]` | stmt1 1/1/1/1/0; stmt2 0/0/0/0/0 |
| E7.2 | enrich | `CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 4326 DEFINITION 'x'; DROP SPATIAL REFERENCE SYSTEM 100001;` | pin 8.0.36 | stmt1 `ver exist(empty) deps([])`; stmt2 `ver exist(row) deps([t.g])` | stmt1 boundary → `srsExistenceStale[4326]` only; stmt2 SRID 100001 unaffected → `refused_dependency` (valid witness) — numeric identity, no folding | stmt1 1/1/1/1/0; stmt2 1/1/1/1/0 |

## E8 — table/schema writes → dependency invalidation

Shared setup: existence is always `exist(row)` for SRID 4326; deps fixtures vary per row. `srsDepsStale` set by a preceding `spatialTouching` write demotes the dependency fact at adjudication — the read itself still fires (facts demote at consumption, bind does not pre-filter dependency facts).

| case | layer | SQL sequence | provider | expected | reads |
|---|---|---|---|---|---|
| E8.1 | enrich | `DROP TABLE t; DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt2 `ver exist(row) deps([t.g])` | stmt2 `unproven` gaps `[D,P]` — `DROP TABLE` sets `srsDepsStale`; the freshly-read positive witness is **DB-pre-state** (the batch's simulated drop precedes it) → invalid → demoted to owed; existence stays consumable (`Exists=true`) | stmt2 1/1/1/1/0 |
| E8.2a | enrich | `CREATE TABLE t (i INT); DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt2 `ver exist(row) deps([t.g])` | `CREATE TABLE` with a complete non-spatial column list is **inert** → witness valid → `refused_dependency`; gaps `[]`; `complete` | stmt2 1/1/1/1/0 |
| E8.2b | enrich | `CREATE TABLE t (i INT); DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt2 `ver exist(row) deps([])` | inert prefix + empty-incomplete deps → `unproven` gaps `[D,P]` (unproven-empty, **not** stale) | stmt2 1/1/1/1/0 |
| E8.3 | enrich | `CREATE TABLE t (g GEOMETRY SRID 4326); DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt2 `ver exist(row) deps([t.g])` | spatial-family authored column → touching → `depsStale` → `unproven` gaps `[D,P]` — the witness cannot prove the post-create state | stmt2 1/1/1/1/0 |
| E8.4 | enrich | `CREATE TRIGGER trg AFTER INSERT ON t FOR EACH ROW SET @x = 1; DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt2 `ver exist(row) deps([t.g])` | unmodeled boundary statement naming a table — effect unmodeled → touching → `depsStale` → `unproven` gaps `[D,P]` (boundary classification per the coverage catch-all seam; touching does not depend on `TableTargets`) | stmt1 boundary (no SRS bind); stmt2 1/1/1/1/0 |
| E8.5 | enrich ×2 on one `batchState` | call1 `DROP TABLE t;` call2 `DROP SPATIAL REFERENCE SYSTEM 4326;` | call2 `ver exist(row) deps([t.g])` | `srsDepsStale` persists across calls (objectCache is per-call; the flag lives on `batchState`) → call2 `unproven` `[D,P]` — a fresh re-query cannot fix a simulated write | call2 1/1/1/1/0 |
| E8.6 | enrich | `DROP SPATIAL REFERENCE SYSTEM 4326; DROP TABLE t; DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt1 `ver exist(row) deps([t.g])`; stmt3 = cache hit | stmt1 `refused_dependency`; `DROP TABLE t` sets `srsDepsStale`; stmt3: cache hit returns the same witness — existence consumable, dependency stale → `unproven` gaps `[D,P]` (**distinguishing witness**: stale deps never erase valid existence and never become absence) | stmt1 1/1/1/1/0; stmt3 0/0/0/0/0 |
| E8.7 | enrich | `ALTER TABLE t DROP COLUMN p; DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt2 `ver exist(row) deps([t.g])` | column action → touching → `unproven` `[D,P]` (the required `DROP COLUMN` control) | stmt2 1/1/1/1/0 |
| E8.8 | enrich | `ALTER TABLE t ENGINE=InnoDB; DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt2 `ver exist(row) deps([t.g])` | complete action list with only non-column actions → inert → witness valid → `refused_dependency` | stmt2 1/1/1/1/0 |
| E8.9 | enrich | `DROP SCHEMA s; DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt2 `ver exist(row) deps([t.g])` | schema-scoped drop → touching → `unproven` `[D,P]` | stmt2 1/1/1/1/0 |
| E8.10 | enrich | `CREATE SCHEMA s2; DROP SPATIAL REFERENCE SYSTEM 4326;` | stmt2 `ver exist(row) deps([t.g])` | `CREATE SCHEMA` provably empty → inert → `refused_dependency` | stmt2 1/1/1/1/0 |

## E9 — contamination and failure-record ordering

| case | layer | SQL sequence | expected | reads |
|---|---|---|---|---|
| E9.1 | enrich | `GARBAGE; DROP SPATIAL REFERENCE SYSTEM 4326;` | failure record precedes the statement → contaminated before stmt2 → arm `blocked` gaps `[B]`; no resolver call | stmt2 0/0/0/0/0 |
| E9.2 | enrich | `GARBAGE; DROP SPATIAL REFERENCE SYSTEM 4326;` (single line) | failure record authored at (line 1, col 1); SRS statement authored at (line 1, col 10) — `failureBefore(1,1,1,10)=true` → `blocked` per **authored position**, not list order | stmt2 0/0/0/0/0 |
| E9.3 | enrich ×2 | call1 `DROP SPATIAL REFERENCE SYSTEM 0; GARBAGE;` call2 `DROP SPATIAL REFERENCE SYSTEM 4326;` | call1 stmt1 `refused_srid0` (a real adjudicated refusal before the failure); trailing `GARBAGE` mints `contaminated` (tail sweep); call2 → `blocked` gaps `[B]` — a fresh per-call cache cannot launder batchState contamination | call1 stmt1 1/1/1/1/0; call2 0/0/0/0/0 |

## E10 — version/product inputs

| case | layer | input | expected | reads |
|---|---|---|---|---|
| E10.1 | enrich+request | observed banner `5.7.0-m14` (unparseable series for the model); no pin | `resolvedVersion=nil` → `SRSKindDrop` unaudited `gate:"version"` boundary — unknown ≠ feature-absent; reads still issued (unproven observed → fire) | 1/1/1/1/0 |
| E10.2 | enrich+request | observed banner `8.0.36-log`; no pin | audited → arm per facts (`unproven` on `exist(empty) deps([])`) | 1/1/1/1/0 |
| E10.3 | enrich+request | observed banner `5.7.44-log`; no pin | `SRSKindDrop` unaudited `gate:"version"` (proven ≠8.0); provider: proven pre-8.0 → `version_inapplicable`, no catalog reads | 1/0/0/1/0 |
| E10.3b | enrich+request | observed banner `8.0.3-log`; no pin | `SRSKindDrop` unaudited `gate:"version"` — **proven pre-feature** (8.0.0–8.0.3 predate the 8.0.4 introduction); reads still fire (observed 8.0.x → existence+deps per B2) | 1/1/1/1/0 |
| E10.3c | request | pin `8.0.3`; providerless | `SRSKindDrop` unaudited `gate:"version"` (pin proves patch 3 < 4) | 0/0/0/0/0 |
| E10.3d | request | pin `8.0.4`; providerless | audited → `unproven` `[E,D,P]` (patch 4 is the floor, inclusive) | 0/0/0/0/0 |
| E10.4 | request | pin `5.7.44`; providerless | unaudited `gate:"version"` | 0/0/0/0/0 |
| E10.5 | parser/enrich | dialect `tidb`, observed banner `8.0.11-TiDB-v8.5.0` | **CURRENT** vendor boundary; `bindMySQLSRS` exits before resolver | 0/0/0/0/0 |
| E10.6 | request | pin `8.0.x` | **CURRENT** `ErrInvalidTargetVersion` — typed error before enrichment; no audit result, no SRS reads | 0/0/0/0/0 |
| E10.7 | request | dialect `mysql` + observed TiDB banner `8.0.11-TiDB-v8.5.0` | **CURRENT** `ErrDialectProductMismatch` — typed error | — |
| E10.8 | request | pin `8.0.36` + observed banner `8.0.37-log` | **CURRENT** `ErrTargetVersionMismatch` — typed error | — |

## E11 — error paths (provider-layer vs request-layer kept distinct)

| case | layer | provider response | expected | reads |
|---|---|---|---|---|
| E11.1 | enrich | resolver forward returns injected sentinel `srs-sentinel-7` | `metadataObjectReadError` propagates → enrich typed error; `errors.Is` sees the sentinel; bounded public text; no adjudication, no gap substitute | 1(att)/0/0/0/0 |
| E11.2 | enrich | `ver("8.0.36")` + existence query fails | wrapped error propagates (typed); deps never issued | 1/1(att)/0/1/0 |
| E11.3 | enrich | `ver("8.0.36") exist(row)` + deps query fails | wrapped error propagates | 1/1/1(att)/1/0 |
| E11.4 | enrich | `select version()` inside the SRS loader fails | wrapped error propagates before catalog reads | 1/0/0/1(att)/0 |
| E11.5 | request | request-level version discovery (observed banner read) fails | typed error on the request path — never adjudicated, never a "feature absent" state (distinguished from E11.4: the failure surface is before enrichment) | 0/0/0/0/0 |

## E12 — redaction and cross-channel isolation

| case | layer | setup | expected |
|---|---|---|---|
| E12.1 | request | E1.1 fixture | serialized `statements[0].raw_sql` and `statements[0].normalized_sql` both exactly `DROP SPATIAL REFERENCE SYSTEM 4326`; E1.2 fixture → `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326` — `if_exists` never appears as a metadata key |
| E12.2 | request | `deps([secret_schema.secret_tbl.secret_col])` fixture | serialized report contains none of `secret_schema`/`secret_tbl`/`secret_col`; the only dependency fact is `dependency_witness_count:1` inside the refusal finding's metadata |
| E12.3 | enrich | populated table registry + SERVER channel entry present; then the SRS statement's own apply | SRS own apply leaves table/schema/SERVER channels value-identical (fidelity-snapshot witness); the only writes this slice are the PROPOSED stale flags |
| E12.4 | enrich | `DROP TABLE t; DROP SPATIAL REFERENCE SYSTEM 4326;` | table apply sets only `srsDepsStale`; `srsExistenceStale` untouched; SERVER channel untouched — disjoint writer classes hold |
| E12.5 | enrich | `DROP SERVER s; DROP SPATIAL REFERENCE SYSTEM 4326;` under `privilege_audit_mode=observed_account_new_session` | `L=1` attributed to the SERVER statement (request-level loader, once); the SRS statement still owes `P` — a SERVER privilege premise never substitutes for the unimplemented SRS one |

## Probe plan — `PLAN / UNEXECUTED` (registered, not run)

Fixed statement for all three: `DROP SPATIAL REFERENCE SYSTEM 100001;` — 100001 is outside every reserved range, so no advisory fires. Binary `bin/deltascope` (path per the execution-order build record). Common flags: `--dialect mysql --format json --fail-on none`. Providerless throughout (no `--host`/DSN). Privilege mode: off (no `privilege_audit_mode` key — default empty). Streams separated: stdout captures the JSON report; stderr captures diagnostics/errors; rc captured immediately after each run.

Config file `policy-srs.yaml` (fixed text, all three probes):

```yaml
dialect: mysql
rules:
  ddl.mysql.srs.drop.srid0:
    level: blocker
  ddl.mysql.srs.drop.dependency:
    level: blocker
  ddl.mysql.srs.drop.reserved:
    level: notice
```

| probe | command | expected stdout | expected stderr | expected rc |
|---|---|---|---|---|
| P1 (pin 8.0.36) | `bin/deltascope audit --config policy-srs.yaml --dialect mysql --format json --fail-on none --target-version 8.0.36 --sql "DROP SPATIAL REFERENCE SYSTEM 100001;"` | JSON report: `statements` length 1; `statements[0].raw_sql` and `statements[0].normalized_sql` = `DROP SPATIAL REFERENCE SYSTEM 100001`; `statements[0].findings` = `[]`; `statements[0].evidence_gaps` = `[{rule_id:"ddl.mysql.srs.drop.dependency", reason_code:"unknown_srs_state", required_facts:["srs.existence","srs.column_dependencies","srs.drop.privilege"]}]` (exactly 1 entry, 3 ordered facts); `statements[0].coverage.status` = `unverified`; `result.unsupported` = `[]`; `result.verdict` = `review` | empty | `0` (fail-on none; gaps never reach the threshold) |
| P2 (no pin) | `bin/deltascope audit --config policy-srs.yaml --dialect mysql --format json --fail-on none --sql "DROP SPATIAL REFERENCE SYSTEM 100001;"` | JSON report: `result.unsupported` contains one entry for statement index 0 with `feature` = the SRS statement feature, `reason` = the bounded unaudited reason, `metadata.object` = `spatial_reference_system`, `metadata.gate` = `version`; `statements[0].findings` = `[]`; `statements[0].evidence_gaps` = `[]`; `statements[0].coverage.status` per existing boundary mechanics (`unverified`); `result.verdict` = `review` | empty | `0` |
| P3 (malformed pin) | `bin/deltascope audit --config policy-srs.yaml --dialect mysql --format json --fail-on none --target-version 8.0.x --sql "DROP SPATIAL REFERENCE SYSTEM 100001;"` | no audit JSON emitted (typed error precedes the audit) | bounded user-error text for `ErrInvalidTargetVersion` (the CLI's `newUserError` surface — malformed `--target-version`) | `2` (user error per the documented `0/1/2/3` table) |

Expected fields are contract-derived and frozen; probe outcomes are recorded verbatim in the execution round — any deviation from this table is a defect, not an amendment. Pin⇒rc conclusions from SERVER probes are not carried over.
