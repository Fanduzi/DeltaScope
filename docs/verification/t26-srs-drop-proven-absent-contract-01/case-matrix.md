# T26-SRS-DROP-PROVEN-ABSENT-CONTRACT-01 — R3 case matrix (PLAN / UNEXECUTED)

All rows are candidate contract rows, not executed tests. Every row
inherits the shared default fixture and lists only explicit deltas.
Layers: `unit` = spec/adjudication slice (constructed state, never
substitutes integration); `provider` = provider read layer; `enrich-
apply` = internal batch/enrich slice; `api` = request-level `AuditSQL`;
`public` = serialised result projection; `cli` = providerless binary.

Counting convention: R/Qexist/Qdeps/Qo are per-statement deltas; L is a
per-enrich-entry count; Vd is a per-request-discovery count (internal
rows: `Vd=N/A — no discovery channel`). Multi-statement/call rows give
explicit totals.

## Shared default fixture `PA-DEFAULT`

| field | frozen value | layers inheriting |
| --- | --- | --- |
| dialect | `mysql` | all |
| sql | `DROP SPATIAL REFERENCE SYSTEM 100001;` | all (rows may override with full text) |
| srids | `100001`,`100002` non-reserved; `4326` reserved; `0` refusal | all |
| request pin | target version `8.0.36` (equals observed → no mismatch) | api/cli |
| request observed banner | `8.0.36` MySQL ValidatedRange | api |
| provider capabilities | `LoadInstanceFacts` + `ObjectResolver` + `LoadMySQLServerDropPrivilege` | api/enrich-apply/provider |
| LoadInstanceFacts answer | version banner `8.0.36` (+ benign instance fields) | api |
| privilege loader fixture | `CURRENT_USER()` → `'r1'@'%'`; `SHOW GRANTS` → `SUPER ON *.*` positive witness | api/enrich-apply |
| SRS Qo banner | provider `select version()` → `8.0.36` → proven && supported && in D-proof | provider/enrich-apply/api |
| existence answer | zero rows; `Next()=false`, `Err()=nil`, `Close()=nil` | provider/enrich-apply/api |
| dependency answer | zero rows, no error | provider/enrich-apply/api |
| mode | `observed_account_new_session` | all |
| policy | all SRS rules default enabled | api/public |
| initial channels | `srsExistenceStale={}`, `srsDepsStale=false`, `serverPrivilegeScopeEnded=false`, empty objectCache, no contamination | enrich-apply/api |
| read vector | R,Qexist,Qdeps,Qo per-statement; L per-enrich; Vd per-request | all |

## PA-1 — absence supply discriminant

| row | layer | deltas | arm | owed | reads |
| --- | --- | --- | --- | --- | --- |
| PA-1a | api | providerless request (no provider at all) | S2 | `[E,D,P]` | R=0,Qexist=0,Qdeps=0,Qo=0, L=0, Vd=N/A (no discovery channel exists without a provider) |
| PA-1b | api | — | A-plain | `[]` | R=1,Qexist=1,Qdeps=1,Qo=1, L=1, Vd=1 |
| PA-1c | api | SRS Qo banner `!proven` (e.g. `8.0.36-zzgarbage`) — the ONLY changed input | S2 | `[E,D]` (P consumable) + privilege notice | R=1,Qexist=1,Qdeps=1,Qo=1, L=1, Vd=1; clean zero-row outside D-proof → `Absence` unset |

Completeness single-variable control is **PA-1b vs PA-1c** (same
witnesses, same zero rows; only D-proof membership changes). PA-1a is
an independent providerless control — it never issues the zero-row
query, so it is not a same-row-count comparison.

## PA-2 — plain / IF EXISTS × premise

| row | layer | deltas | arm | owed | projection |
| --- | --- | --- | --- | --- | --- |
| PA-2a | api | — | A-plain | `[]` | 1 `absent` blocker; complete; reject |
| PA-2b | api | sql `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;` | A-IE✓ | `[]` | 1 `absent_noop` notice `{srid:100001,outcome:"absent_noop",conditional_audit:"observed_account_new_session"}`; complete; pass |
| PA-2c | api | PA-2b sql; grant witness `SELECT ON *.*` (no SUPER) | A-IE? | `[P]` | 1 gap `unproven_privilege`/`[srs.drop.privilege]`; unverified; review |
| PA-2d | api | PA-2b sql; prefix stmt `REVOKE SELECT ON *.* FROM 'r1'@'%';` | A-IE? | `[P]` | scope ended → suppressed; same gap |

## PA-3 — S2 controls

| row | layer | deltas | arm | owed |
| --- | --- | --- | --- | --- |
| PA-3a | api | existence answer = row `{srs_id:100001,srs_name:'ZZTEST'}` | S2 | `[D]` + privilege notice |
| PA-3b | api | providerless (PA-1a conditions) | S2 | `[E,D,P]` |

## PA-4 — conflicts: valid witness beats absence; invalid absence rejected

| row | layer | deltas | arm | note |
| --- | --- | --- | --- | --- |
| PA-4a | api | dependency answer = `{table_schema:'s',table_name:'t',column_name:'g'}` matching query-param SRID 100001; existence zero | S1 | refused_dependency — real two-read provider sequence |
| PA-4b | api | sql `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;` + same deps answer | S1 | refused_dependency |
| PA-4c | api | prefix stmt `DROP TABLE s.t` (bound to the accepted `tblTFixture`; real prefix-minted `srsDepsStale=true`, not seeded) + same deps answer + existence zero | A-plain | deps demoted → loses S1 eligibility; absence governs |
| PA-4d | api | sql `DROP SPATIAL REFERENCE SYSTEM 0;` | S0 | refused_srid0 (completeness listing) |
| PA-4e | unit | constructed state `{SRID:100002,Read:true,Exists:false,Absence:true}` for authored `100001`, sql `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;`, P consumable, no valid deps | S2 | ValidAbsence fails on SRID → owed `[E,D]` + privilege notice; own-apply mints authored `{100001}` only. Layer declared: unit; a real mysqlmeta provider is not claimed to produce the mismatch |
| PA-4f | unit | `{SRID:100001,Read:false,Absence:true}` | S2 | `Absence` ignored → unknown |
| PA-4g | provider | `QueryContext` error value = `sql.ErrNoRows` (1 attempt) | propagate | no snapshot → bind/adjudication/apply unreachable for the statement |
| PA-4h | provider | `rows.Err()==sql.ErrNoRows` after `Next()=false` | propagate | never Absence |

## PA-5 — stale channels & own-apply (every row declares its arm)

| row | layer | arm | initial | post `srsExistenceStale` | post `srsDepsStale` |
| --- | --- | --- | --- | --- | --- |
| PA-5a | enrich-apply | A-plain | `{77777}`, `depsStale=true`, premise consumable | `{77777}` | `true` |
| PA-5b | enrich-apply | A-IE✓ (sql `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;`, premise consumable) | `{77777}`, `depsStale=true` | `{77777}` | `true` |
| PA-5c | enrich-apply | A-IE? (PA-5b sql + premise explicitly `unproven`) | `{77777}`, `depsStale=true` | `{77777}` | `true` |
| PA-5d | enrich-apply | B (initial `{100001}`) | `{100001}` | `{100001}`; bind short-circuits → ΔR=0,ΔQexist=0,ΔQdeps=0,ΔQo=0 | unchanged |
| PA-5e | enrich-apply | S2 (existence answer = present row; initials `{77777}`, `depsStale=true` explicit) | `{77777}` | `{77777,100001}` | `true` |
| PA-5f | enrich-apply | B — whole-batch contamination via the accepted real parse-failure fixture (prefix unparseable statement) | contamination | no mint; unread reason `batch_effects` | unchanged |

## PA-6 — ordering & cache

| row | layer | sequence | expected |
| --- | --- | --- | --- |
| PA-6a | api | `DROP SPATIAL REFERENCE SYSTEM 100001;` ×2, one request | stmt1 A-plain: R=1,Qexist=1,Qdeps=1,Qo=1; stmt2 binds again — A mints nothing, same-enrich **cache hit** key `(schema,mysql_spatial_reference_system,100001,∅)` → stmt2 ΔR=0,ΔQexist=0,ΔQdeps=0,ΔQo=0; stmt2 also A-plain. L=1 total (enrich-entry), Vd=1 total |
| PA-6b | api | `DROP SPATIAL REFERENCE SYSTEM 100001;` `DROP SPATIAL REFERENCE SYSTEM 100002;` | stmt2 ΔR=1,ΔQexist=1,ΔQdeps=1,ΔQo=1 — independent key re-reads; L=1 total, Vd=1 total |
| PA-6c | api | stmt1 `CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 100001 SRID 100001 DEFINITION '…' DESCRIPTION 'zz'` (accepted MySQL 8.0.36 implementation-boundary fixture → `Unsupported` → boundary own-apply mints `{100001}`); stmt2 `DROP SPATIAL REFERENCE SYSTEM 100001;` same dialect, same resolvedVersion | stmt2 = **B** with `SRSDrop` adjudication attached (this is a real audited-DROP B, not boundary): ΔR=0,ΔQexist=0,ΔQdeps=0,ΔQo=0 (bind short-circuit); post-stmt1 `{100001}` minted by the boundary branch; public scope: stmt1 incomplete-boundary output, stmt2 blocked. No SRS CREATE auditing added |
| PA-6d | enrich-apply | two `enrichOrderedStatements` calls sharing `batchState`, each SQL `DROP SPATIAL REFERENCE SYSTEM 100001;`, directly-provided resolvedVersion `8.0.36`, mode on; provider answers: call1 existence-zero (clean), call2 existence row `{100001}` present | call1: R=1,Qexist=1,Qdeps=1,Qo=1 → A-plain, no mint, L evaluated at that enrich entry; call2: NEW objectCache → R=1,Qexist=1,Qdeps=1,Qo=1 again → S2, owed `[D]`, post-own-apply `{100001}`; loader re-evaluates at each enrich entry (L=2 total across calls) |

The below-floor double-DROP variant is retained only as a
double-boundary row (both statements boundary, second is never called
a B adjudication): PA-6e | api | request banner `8.0.3` observed for
both statements | stmt1 boundary-mint `{100001}`; stmt2 blocked-by-B?
NO — boundary statements carry no `SRSDrop` adjudication; observed
result = two boundary-marked statements, own-apply `{100001}`.

## PA-7 — REVOKE scoping (two observation layers)

| row | layer | sequence | expected |
| --- | --- | --- | --- |
| PA-7a | enrich-apply | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;` `REVOKE SELECT ON *.* FROM 'r1'@'%';` `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100002;` | internal companion: independent-value snapshots of `Metadata.SpatialReference`/`Metadata.SRSDrop`/premise at post-stmt1-own-apply, post-REVOKE, post-stmt3/final — stmt1 adjudication `{arm:A-IE✓, premise:consumable}` identical at all three points; `serverPrivilegeScopeEnded` false→true→true; `srsExistenceStale={}` `srsDepsStale=false` throughout |
| PA-7b | public | same sequence | post-projection: stmt1 `absent_noop` finding + `conditional_audit` marker (produced by the rule at projection time — not an apply-time field); stmt3 1 gap `unproven_privilege`/`[P]`; stmt2 REVOKE public output + aggregate contribution: **N/A — unobserved, scope limited to the two SRS statements** |

## PA-8 — version seams

| row | layer | deltas | expected |
| --- | --- | --- | --- |
| PA-8a | api | request NO pin (or pin `8.0.3` matching observed `8.0.3` — explicit, no accidental mismatch); provider banner `8.0.3` | D-qualify fails → `Unsupported{boundary:unaudited,gate:version}`; provider reads still issue (R=1,Qexist=1,Qdeps=1,Qo=1); boundary own-apply mints `{100001}` |
| PA-8b | api | PA-1c (banner `!proven` only change) | S2 `[E,D]` + privilege notice |
| PA-8c | api | pin `8.0.37` vs observed `8.0.36` | typed exit `ErrTargetVersionMismatch`; `report.Result{}` zero-value — public fields N/A (nothing projected); L=0, SRS seams R=Qexist=Qdeps=Qo=0; Vd=1 counted separately |
| PA-8d | enrich-apply | dialect `tidb`, real TiDB parse of `DROP SPATIAL REFERENCE SYSTEM 100001;`, pin/banner/mode frozen as PA-DEFAULT | vendor boundary statement (`Unsupported{boundary:vendor}`); SRS seams 0; boundary own-apply mints `{100001}` — real parse→enrich/apply layer, bound to the accepted R3 P9c witness |

## PA-9 — supply failures

| row | layer | injection | result |
| --- | --- | --- | --- |
| PA-9a | provider | `QueryContext` generic error, 1 attempt | propagates wrapped; bind/adjudication/apply unreachable |
| PA-9b | provider | iteration error during `Next()` | `rows.Err()` propagates; no snapshot delivered |
| PA-9c | provider | ctx cancellation | `Err()`=ctx error → propagate; no snapshot |
| PA-9d | api | capability set minus `ObjectResolver` (LoadInstanceFacts + privilege loader retained) | `Status=Unavailable` snapshot → S2 owed `[E,D]`; premise stays consumable (loader unaffected) + privilege notice |
| PA-9e | provider | clean zero rows under `!proven` Qo banner | provider-layer asserts only the returned state `{Read:true,Exists:false,Absence:false}`; the S2 outcome is asserted at the PA-1c/PA-8b enrich-API twin |
| PA-9f | provider | existence clean zero then dependency read errors | error propagates; snapshot never delivered — a helper's local field is not a published fact |

## PA-10 — public shape & policy

| row | layer | deltas | expected |
| --- | --- | --- | --- |
| PA-10a | public | srid `4326` (reserved), plain, absent | `absent` blocker only — NO reserved advisory |
| PA-10b | public | sql `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326;`, absent, consumable | `absent_noop` notice + exactly 1 membership-only `reserved` notice; no extra marker/gap |
| PA-10c | public | PA-10b sql, unproven premise | `unproven_privilege` gap + exactly 1 `reserved` notice; no marker |
| PA-10d | public | policy: only `ddl.mysql.srs.drop.absent` disabled; single non-reserved plain absent stmt | findings=0, gaps=0, coverage=complete, verdict=pass; internal adjudication/own-apply unchanged |
| PA-10e | public | sql `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;`, absent, consumable; policy: `absent_noop` level→warning | warning → verdict review; `conditional_audit` marker preserved |
| PA-10f | public | sentinels injected at declared positions: `srs_name='SRS_ZZTEST'` in the existence-row answer (PA-3a variant); account `'zz_acct'@'%'` and grant text `GRANT SUPER ON *.* TO 'zz_acct'` in the loader fixture; schema/table/column `zz_schema`/`zz_table`/`zz_col` in the PA-4a deps answer | scan = the complete serialised result JSON at post-projection; none of the sentinel strings may appear; RawSQL is a declared observed field — sentinels are never placed inside RawSQL so the "absent from result" claim stays meaningful |
| PA-10g | cli | reference only: accepted providerless probes at `56bcf05f` (rc 0/1/2) | unchanged shapes; NOT positive evidence for proven absence |

## Verification plan (future execution work order)

1. Spec unit table for `AdjudicateSRSDrop`: IfExists split, S1/A
   ordering, ValidAbsence field conjunction, invalid-absence rows.
2. API rows vs stub-driver provider: zero-row answers, two-read
   conflict sequence, boundary-prefix B, double-enrich changed answer.
3. Internal rows: per-statement/per-call deltas, non-empty stale
   controls, REVOKE three-point snapshots, contamination B control.
4. Serialised-shape assertions for both findings + the
   `unproven_privilege` gap on real JSON paths; sentinel scan.
5. Frozen providerless CLI probes remain reference-only.
