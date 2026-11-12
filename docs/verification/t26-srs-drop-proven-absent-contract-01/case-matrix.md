# T26-SRS-DROP-PROVEN-ABSENT-CONTRACT-01 — R4 case matrix (PLAN / UNEXECUTED)

All rows are candidate contract rows, not executed tests. Every row
inherits the shared default fixture and lists only explicit deltas.
Layers: `unit` = spec/adjudication slice (constructed state only —
public finding, real bind and own-apply are never asserted at this
layer); `provider` = provider read layer; `enrich-apply` = internal
batch/enrich slice; `api` = request-level `AuditSQL`; `public` =
serialised result projection; `cli` = providerless binary.

Counting convention: R/Qexist/Qdeps/Qo are per-statement deltas; L is a
per-enrich-entry count; Vd is a per-request-discovery count (internal
rows: `Vd=N/A — no discovery channel`). Multi-statement rows give
explicit totals and split prefix vs tail increments.

Canonical finding/gap literals (RuleID/level/message/metadata) are
frozen ONLY in README §3; rows below reference the arm names.

## Shared default fixture `PA-DEFAULT`

| field | frozen value | layers inheriting |
| --- | --- | --- |
| dialect | `mysql` | all |
| sql | `DROP SPATIAL REFERENCE SYSTEM 100001;` | all (rows may override with full text) |
| srids | `100001`,`100002` non-reserved; `4326` reserved; `0` refusal | all |
| request pin | target version `8.0.36` (equals observed → no mismatch) | api/cli |
| request observed banner | `8.0.36` MySQL ValidatedRange | api |
| internal enrich input | directly-provided `resolvedVersion` = in-domain `8.0.36` MySQL identity (the api request-observed value is NOT an implicit enrich parameter) | enrich-apply/unit |
| provider capabilities | `LoadInstanceFacts` + `ObjectResolver` + `LoadMySQLServerDropPrivilege` | api/enrich-apply/provider |
| LoadInstanceFacts answer | version banner `8.0.36` (+ benign instance fields) | api |
| privilege loader fixture | `CURRENT_USER()` cell `` `app`@`%` ``; `SHOW GRANTS` row `GRANT SELECT, SUPER ON *.* TO `app`@`%`` (full legal line, account binding consistent — accepted r3PrivilegeFixture shape) | api/enrich-apply |
| negative grant override | `GRANT SELECT ON *.* TO `app`@`%`` (full legal line — used by PA-5c) | enrich-apply |
| SRS Qo banner | provider `select version()` → `8.0.36` → proven && supported && in D-proof | provider/enrich-apply/api |
| existence answer | zero observed rows; `Next()` never true, `Err()=nil`, `Close()=nil` | provider/enrich-apply/api |
| dependency answer | zero rows, no error | provider/enrich-apply/api |
| mode | `observed_account_new_session` | all |
| policy | all SRS rules default enabled | api/public |
| initial channels | `srsExistenceStale={}`, `srsDepsStale=false`, `serverPrivilegeScopeEnded=false`, empty objectCache, no contamination | enrich-apply/api |
| read vector | R,Qexist,Qdeps,Qo per-statement; L per-enrich; Vd per-request | all |

## PA-1 — absence supply discriminant

| row | layer | deltas | arm | owed | reads |
| --- | --- | --- | --- | --- | --- |
| PA-1a | api | providerless request, pin `8.0.36` | S2 | `[E,D,P]` | R=0,Qexist=0,Qdeps=0,Qo=0, L=0, Vd=N/A (no discovery channel exists without a provider); pin admits D-qualify but cannot supply absence — no existence read ever runs |
| PA-1b | api | — | A-plain | `[]` | R=1,Qexist=1,Qdeps=1,Qo=1, L=1, Vd=1 |
| PA-1c | api | SRS Qo banner = literal `unparseable` (frozen — the ONLY changed input; `ParseObservedVersion` prefix regexp `^(\d+)\.(\d+)\.(\d+)` fails → nil → `mysqlSRSCatalogSupport` returns `(false,false,nil)` at provider_server_srs.go:103-106) | S2 | `[E,D]` + S2 privilege notice (P consumable) | R=1,Qexist=1,Qdeps=1,Qo=1, L=1, Vd=1; clean zero observed rows outside D-proof → `Absence` unset |

Completeness single-variable control is **PA-1b vs PA-1c**: identical
request pin, request observed, account witness and E/D answers — only
the Qo banner changes. PA-1a is an independent providerless control.

## PA-2 — plain / IF EXISTS × premise

| row | layer | deltas | arm | owed | projection |
| --- | --- | --- | --- | --- | --- |
| PA-2a | api | — | A-plain | `[]` | README §3 A-plain literal; complete; reject |
| PA-2b | api | sql `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;` | A-IE✓ | `[]` | README §3 A-IE✓ literal; complete; pass |
| PA-2c | api | PA-2b sql; grant `GRANT SELECT ON *.* TO `app`@`%`` (no SUPER) | A-IE? | `[P]` | README §3 A-IE? gap literal; unverified; review |
| PA-2d | api | PA-2b sql; prefix stmt `REVOKE SELECT ON *.* FROM 'app'@'%';` | A-IE? | `[P]` | scope ended → suppressed; same gap |

## PA-3 — S2 controls

| row | layer | deltas | arm | owed |
| --- | --- | --- | --- | --- |
| PA-3a | api | existence answer = row `{srs_id:100001,srs_name:'ZZTEST'}` | S2 | `[D]` + privilege notice |
| PA-3b | api | providerless (PA-1a conditions) | S2 | `[E,D,P]` |

## PA-4 — conflicts and invalid-absence discriminants

| row | layer | deltas | arm | note |
| --- | --- | --- | --- | --- |
| PA-4a | api | deps answer = `{table_schema:'s',table_name:'t',column_name:'g'}` bound to query-param SRID 100001; existence zero observed rows | S1 | refused_dependency — real two-read provider sequence |
| PA-4b | api | sql `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;` + same deps answer | S1 | refused_dependency |
| PA-4c | api | prefix stmt `DROP TABLE s.t` (accepted `tblTFixture`; real prefix-minted `srsDepsStale=true`, not seeded) + same deps answer + existence zero observed rows | A-plain | deps demoted → loses S1 eligibility; ValidAbsence governs |
| PA-4d | api | sql `DROP SPATIAL REFERENCE SYSTEM 0;` | S0 | refused_srid0 |
| PA-4e | unit | constructed state `{SRID:100002,Read:true,Exists:false,Absence:true}` for authored `100001`; sql `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;`; P=consumable; no valid deps; clean | S2 | unit-layer fields only: ValidAbsence fails on SRID → not A; owed `[E,D]`; downstream bind/public/own-apply N/A at this layer — see PA-4e2 |
| PA-4e2 | enrich-apply + public | custom `ObjectResolver` at the bind seam deliberately returns `{SRID:100002,Read:true,Exists:false,Absence:true}` for authored `100001`; loader fixture per PA-DEFAULT; direct resolvedVersion `8.0.36`; clean deps via resolver; real bind/adjudication/apply | S2 | projection: owed `[E,D]` + S2 privilege notice; own-apply mints authored `{100001}` only — never `100002`. Counts: resolver attempt R=1; SQL driver seams Qexist/Qdeps/Qo = N/A (custom resolver issues no catalog SQL — N/A, not a measured zero); L=1 (entry gate); Vd=N/A |
| PA-4f | unit | `{SRID:100001,Read:false,Exists:false,Absence:true}`; sql `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;`; P=consumable; no valid deps; clean | S2 | `Absence` ignored — existence unknown → owed `[E,D]`; downstream N/A at unit layer |
| PA-4f2 | unit | `{SRID:100001,Read:true,Exists:true,Absence:true}` (restored contradictory pair); sql `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;`; P=consumable; no valid deps; clean | S2 | `!Exists` fails → `Exists=true` witness governs → owed `[D]`; `Absence` ignored; downstream N/A at unit layer |
| PA-4g | provider | `QueryContext` error value = `sql.ErrNoRows` (1 attempt) | propagate | no snapshot → bind/adjudication/apply unreachable |
| PA-4h | provider | `rows.Err()==sql.ErrNoRows` after `Next()=false` with zero observed rows | propagate | never Absence |
| PA-4i | provider | positive row then `Next()=false`,`Err()=nil` (EOF after positive) | present | `Exists=true`, `Absence=false` — EOF-after-positive is not a zero-row answer |

## PA-5 — stale channels & own-apply (every row declares its arm)

| row | layer | arm | initial | post `srsExistenceStale` | post `srsDepsStale` |
| --- | --- | --- | --- | --- | --- |
| PA-5a | enrich-apply | A-plain | `{77777}`, `depsStale=true`, premise consumable | `{77777}` | `true` |
| PA-5b | enrich-apply | A-IE✓ (sql `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;`, premise consumable) | `{77777}`, `depsStale=true` | `{77777}` | `true` |
| PA-5c | enrich-apply | A-IE? — PA-5b sql + explicit NEGATIVE privilege witness override: loader fixture grant `GRANT SELECT ON *.* TO `app`@`%``, account `` `app`@`%``, mode/provider per PA-DEFAULT → premise unproven | `{77777}`, `depsStale=true` | `{77777}` | `true`; L=1 (entry gate from the explicit mode/provider, not from the arm) |
| PA-5d | enrich-apply | B (initial `{100001}`) | `{100001}` | `{100001}`; bind short-circuits → ΔR=0,ΔQexist=0,ΔQdeps=0,ΔQo=0 | unchanged |
| PA-5e | enrich-apply | S2 (existence answer = present row; initials `{77777}`, `depsStale=true` explicit) | `{77777}` | `{77777,100001}` | `true` |
| PA-5f | enrich-apply | B — whole-batch contamination. Prefix SQL `GARBAGE` (accepted P5a parse-failure fixture, t26_srs_priv_exec_int_test.go:222), then `DROP SPATIAL REFERENCE SYSTEM 100001;`; authored order = sequence order; initial stale empty | contamination | no mint; unread reason `batch_effects` | unchanged; L=1 (entry gate — mode on per PA-DEFAULT, derived from the gate not the B arm) |

## PA-6 — ordering & cache

| row | layer | sequence | expected |
| --- | --- | --- | --- |
| PA-6a | api | `DROP SPATIAL REFERENCE SYSTEM 100001;` `DROP SPATIAL REFERENCE SYSTEM 100001;` | stmt1 A-plain R=1,Qexist=1,Qdeps=1,Qo=1; stmt2 same-enrich cache hit (key `(schema,mysql_spatial_reference_system,100001,∅)`) → ΔR=0,ΔQexist=0,ΔQdeps=0,ΔQo=0, also A-plain. L=1, Vd=1 |
| PA-6b | api | `DROP SPATIAL REFERENCE SYSTEM 100001;` `DROP SPATIAL REFERENCE SYSTEM 100002;` | stmt2 ΔR=1,ΔQexist=1,ΔQdeps=1,ΔQo=1 — independent key re-read; L=1, Vd=1 |
| PA-6c | api | stmt1 `CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 100001 DEFINITION 'x';` — accepted E7.1 fixture family (t26_srs_drop_r1_test.go:789, `srsSQLFixture`/`enrichSRS`): typed SRS declaration → `Unsupported{implementation boundary}` → boundary own-apply mints authored `{100001}`; stmt2 `DROP SPATIAL REFERENCE SYSTEM 100001;` same dialect, same resolvedVersion | stmt1: its own resolve reads R=1,Qexist=1,Qdeps=1,Qo=1 (the boundary snapshot feeds the mint; public scope = boundary-marked statement output); stmt2 = **B** with `SRSDrop` adjudication attached → bind short-circuit ΔR=0,ΔQexist=0,ΔQdeps=0,ΔQo=0; blocked public output. L=1 (entry gate — loader evaluates the qualified DROP before arm selection), Vd=1. No SRS CREATE auditing added |
| PA-6d | enrich-apply | two `enrichOrderedStatements` calls sharing `batchState`, each sql `DROP SPATIAL REFERENCE SYSTEM 100001;`, direct resolvedVersion `8.0.36`, mode on; call1 existence zero observed rows (clean), call2 existence row `{100001}` present | call1: R=1,Qexist=1,Qdeps=1,Qo=1 → A-plain, no mint, L counted at that enrich entry; call2: NEW objectCache → R=1,Qexist=1,Qdeps=1,Qo=1 → S2 `[D]`, own-apply `{100001}`; loader re-evaluates at each enrich entry — L=2 total; Vd=N/A both calls |
| PA-6e | api | no-pin fixture (pin absent; request observed `8.0.3`; Qo `8.0.3`; capabilities/mode inherited): `DROP SPATIAL REFERENCE SYSTEM 100001;` `DROP SPATIAL REFERENCE SYSTEM 100001;` | both statements version-boundary (`Unsupported{boundary:unaudited,gate:version}`) — the second is NOT an SRSDrop arm B. stmt1: boundary resolve reads R=1,Qexist=1,Qdeps=1,Qo=1 and boundary own-apply mints `{100001}`; stmt2 Δ=0 — attributed to the same-SRID stale minted by stmt1's boundary apply (bind short-circuit), NOT to "boundary reads nothing". L=0 — attributed to the qualification gate (no audited-DROP statement qualifies below floor), NOT to the arm; Vd=1 |

## PA-7 — REVOKE scoping (two observation layers)

| row | layer | sequence | expected |
| --- | --- | --- | --- |
| PA-7a | enrich-apply | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;` `REVOKE SELECT ON *.* FROM 'app'@'%';` `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100002;` | internal companion: independent-value snapshots of `SpatialReference`/`SRSDrop`/premise at post-stmt1-own-apply, post-REVOKE, post-stmt3/final — stmt1 adjudication `{arm:A-IE✓, premise:consumable}` identical at all three points; `serverPrivilegeScopeEnded` false→true→true; `srsExistenceStale={}`, `srsDepsStale=false` throughout. Reads: stmt1 R=1,Qexist=1,Qdeps=1,Qo=1; REVOKE Δ=0; stmt3 (different SRID, no stale) R=1,Qexist=1,Qdeps=1,Qo=1; L=1 (one enrich entry); Vd=N/A |
| PA-7b | public | same sequence | post-projection: stmt1 `absent_noop` finding + `conditional_audit` marker (produced by the rule at projection — not an apply-time field); stmt3 1 gap `unproven_privilege`/`[P]`; the projection adds no catalog reads. REVOKE public output + aggregate contribution: **N/A — unobserved, scope limited to the two SRS statements** |

## PA-8 — version seams

| row | layer | deltas | expected |
| --- | --- | --- | --- |
| PA-8a | api | no-pin fixture (pin absent; request observed `8.0.3`; Qo `8.0.3`; capabilities/mode inherited) | D-qualify fails on observed `8.0.3` → `Unsupported{boundary:unaudited,gate:version}`; provider reads still issue (R=1,Qexist=1,Qdeps=1,Qo=1); boundary own-apply mints `{100001}`; L=0 (qualification gate); Vd=1 |
| PA-8b | api | PA-1c (Qo banner `unparseable` is the ONLY change; request observed stays `8.0.36` → admitted) | S2 `[E,D]` + privilege notice; same per-statement reads as PA-1b |
| PA-8c | api | pin `8.0.37` vs request observed `8.0.36` | typed exit `ErrTargetVersionMismatch`; `report.Result{}` zero-value — public fields N/A (nothing projected); L=0, SRS seams R=Qexist=Qdeps=Qo=0; Vd=1 (the discovery call ran; counted separately from SRS seams) |
| PA-8d | enrich-apply | real TiDB path — accepted R3 P9c input reused verbatim: `parseSQL("DROP SPATIAL REFERENCE SYSTEM 100001;", DialectTiDB)` → `Extract` → internal enrich/apply; provider fixture banner `8.0.11-TiDB-v8.5.0`; internal `resolvedVersion` param = banner-parsed tidb `8.5.0`; priv observation SUPER | vendor boundary (`Unsupported{boundary:vendor}` at coverage_boundary.go:91-96); SRS seams R=Qexist=Qdeps=Qo=0; boundary own-apply mints `{100001}`; request pin/discovery = N/A (internal row — not executed); L derived from the TiDB gate (SRS privilege loader requires MySQL-dialect qualification → L=0) |

## PA-9 — supply failures

| row | layer | injection | result |
| --- | --- | --- | --- |
| PA-9a | provider | `QueryContext` generic error, 1 attempt | propagates wrapped; bind/adjudication/apply unreachable |
| PA-9b | provider | iteration error during `Next()` | `rows.Err()` propagates; no snapshot delivered |
| PA-9c | provider | ctx cancellation | `Err()`=ctx error → propagate; no snapshot |
| PA-9d | api | capability set minus `ObjectResolver` (LoadInstanceFacts + privilege loader retained) | `Status=Unavailable` snapshot → S2 owed `[E,D]`; premise stays consumable + privilege notice |
| PA-9e | provider | clean zero observed rows under Qo banner `unparseable` | provider-layer asserts only the returned state `{Read:true,Exists:false,Absence:false}`; S2 outcome asserted at the PA-1c/PA-8b enrich-API twin |
| PA-9f | provider | existence clean zero observed rows then dependency read errors | error propagates; snapshot never delivered — a helper's local field is not a published fact |

## PA-10 — public shape & policy

| row | layer | deltas | expected |
| --- | --- | --- | --- |
| PA-10a | public | srid `4326` (reserved), sql `DROP SPATIAL REFERENCE SYSTEM 4326;`, absent | `absent` blocker only — NO reserved advisory |
| PA-10b | public | sql `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326;`, absent, consumable | `absent_noop` notice + exactly 1 membership-only `reserved` notice (`{srid:4326}` only); no extra marker/gap |
| PA-10c | public | PA-10b sql, unproven premise (`GRANT SELECT ON *.* TO `app`@`%``) | `unproven_privilege` gap + exactly 1 `reserved` notice; no marker |
| PA-10d | public | policy: only `ddl.mysql.srs.drop.absent` disabled; single non-reserved plain absent stmt | findings=0, gaps=0, coverage=complete, verdict=pass; internal adjudication/own-apply unchanged |
| PA-10e | public | sql `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;`, absent, consumable; policy: `absent_noop` level→warning | warning → verdict review; `conditional_audit` marker preserved |
| PA-10f | public | arrangement = registered PA-3a-variant + PA-4a-variant, each scanned separately. PA-3a-variant: existence row `{srs_id:100001,srs_name:'SRS_ZZTEST'}`, deps empty → S2 (premise consumable → privilege notice); sentinel `SRS_ZZTEST` must not appear in the full serialised JSON. PA-4a-variant: same existence-zero + deps `{zz_schema,zz_table,zz_col}` → S1 refused_dependency; sentinels `zz_schema`/`zz_table`/`zz_col` absent from the full serialised JSON. Loader sentinels `` `zz_acct`@`%` `` + grant text `GRANT SUPER ON *.* TO `zz_acct`@`%`` on a separate PA-2b-variant scan | each variant's arm, premise, finding/gap scope declared here BEFORE scanning the complete serialised result JSON post-projection; no sentinel ever enters RawSQL — the "absent from result" claim stays meaningful; no single-S1 scan claimed to cover other projections |
| PA-10g | cli | reference only: accepted providerless probes at `56bcf05f` (rc 0/1/2) | unchanged shapes; NOT positive evidence for proven absence |

## Verification plan (future execution work order)

1. Spec unit table for `AdjudicateSRSDrop`: IfExists split, S1/A
   ordering, ValidAbsence field conjunction, invalid-absence rows
   (foreign SRID, Read=false, Exists=true+Absence).
2. API rows vs stub-driver provider: zero-observed-row answers,
   two-read conflict sequence, boundary-prefix B, double-enrich
   changed answer, foreign-SRID bind companion (PA-4e2).
3. Internal rows: per-statement/per-call deltas, non-empty stale
   controls, REVOKE three-point snapshots, contamination B control.
4. Serialised-shape assertions for README §3 literals on real JSON
   paths; sentinel scans.
5. Frozen providerless CLI probes remain reference-only.
