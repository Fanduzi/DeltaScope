# T26-SRS-DROP-PROVEN-ABSENT-CONTRACT-01 — R2 case matrix (PLAN / UNEXECUTED)

All rows are candidate contract rows, not executed tests. Every row
inherits the shared default fixture below and lists only its explicit
deltas. Layers: `unit` = adjudication/spec slice; `provider` = provider
read layer; `enrich-apply` = internal batch/enrich slice; `api` =
request-level `AuditSQL`; `public` = serialised result projection;
`cli` = providerless binary.

## Shared default fixture `PA-DEFAULT`

| field | frozen value |
| --- | --- |
| dialect | `mysql` |
| sql | `DROP SPATIAL REFERENCE SYSTEM 100001;` (full parseable text; srid 100001 non-reserved, 100002 non-reserved, 4326 reserved, 0 refusal) |
| pin | `--target-version 8.0.36` equivalent request field |
| request observed banner | `8.0.36` (Valida­tedRange, matches pin) |
| catalog-support banner | provider `select version()` → `8.0.36` → proven && supported && in D-proof |
| mode | `observed_account_new_session` |
| grant witness | observed account `r1`@`%`, `SUPER ON *.*` (positive) |
| provider | ObjectResolver-capable; answers below |
| existence answer | zero rows, `Next()=false`, `Err()=nil`, `Close()=nil` |
| dependency answer | zero rows, no error |
| policy | all SRS rules default enabled |
| initial channels | `srsExistenceStale={}`, `srsDepsStale=false`, `serverPrivilegeScopeEnded=false`, empty objectCache, no contamination |
| read vector | R resolver forwards; Qexist/Qdeps catalog; Qo `select version()`; L privilege loader; Vd request version discovery (internal rows: Vd=N/A) |

Counts are per-statement deltas unless a row says per-call.
`N/A` entries carry a reason; nothing unobserved is written as zero.

## PA-1 — absence supply discriminant

| row | layer | deltas from PA-DEFAULT | arm | owed | reads Δ |
| --- | --- | --- | --- | --- | --- |
| PA-1a | api | providerless request (no provider); pin supplies version admission | S2 | `[E,D,P]` | R=0, Qexist=0, Qdeps=0, Qo=0, L=1 (observation loaded, premise unproven), Vd=1 |
| PA-1b | api | (default) | A-plain | `[]` | R=1, Qexist=1, Qdeps=1, Qo=1, L=1, Vd=1 |
| PA-1c | api | catalog-support banner `!proven` (malformed) while request pin supplies admission | S2 | `[E,D,P]` | R=1, Qexist=1, Qdeps=1, Qo=1, L=1, Vd=1; clean zero-row but outside D-proof → `Absence` never set |

PA-1a vs PA-1c is the discriminant: providerless never issues the
zero-row query (unavailable snapshot), while `!proven` issues it and
still cannot supply absence — the changed completeness condition is
explicitly the provider banner leaving D-proof, not row count.

## PA-2 — plain / IF EXISTS × premise

| row | layer | deltas | arm | owed | projection |
| --- | --- | --- | --- | --- | --- |
| PA-2a | api | — | A-plain | `[]` | 1 `absent` blocker; complete; reject |
| PA-2b | api | sql `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;` | A-IE✓ | `[]` | 1 `absent_noop` notice `{srid,outcome,conditional_audit}`; complete; pass |
| PA-2c | api | PA-2b sql; grant witness `SELECT` only (no SUPER) | A-IE? | `[P]` | 1 gap `unproven_privilege`/`[P]`; unverified; review |
| PA-2d | api | PA-2b sql; REVOKE prefix stmt `REVOKE SELECT ON *.* FROM 'r1'@'%';` | A-IE? | `[P]` | suppressed; same gap |

## PA-3 — S2 control rows (unchanged semantics)

| row | layer | deltas | arm | owed |
| --- | --- | --- | --- | --- |
| PA-3a | api | existence answer = row `{srs_id:100001,srs_name:'ZZTEST'}` | S2 | `[D]` + privilege notice |
| PA-3b | api | providerless | S2 | `[E,D,P]` |

## PA-4 — conflicts (B3): valid witness beats absence

| row | layer | deltas | arm | note |
| --- | --- | --- | --- | --- |
| PA-4a | api | dependency answer = `{srs_id:100001,f_table:'t',f_col:'g'}` (valid, non-stale) | S1 | refused_dependency; absence never consumed — real two-read provider sequence (existence zero + positive deps) |
| PA-4b | api | sql `… IF EXISTS 100001`; same positive deps answer | S1 | refused_dependency — R1 wrongly gave absent_noop; witness outranks absence |
| PA-4c | api | `srsDepsStale=true` (prior touching stmt `DROP TABLE s.t`) + same deps answer + absence | A-plain | demoted witness is not valid positive → S1 unreachable; absence governs |
| PA-4d | api | srid `0` (existence read would find row) | S0 | refused_srid0; listed for completeness |
| PA-4e | unit | `Absence=true, Read=false` | S2 | invalid absence ignored → unknown |
| PA-4f | unit | `Absence=true, Exists=true` | S2 | positive fact wins; bit ignored |
| PA-4g | provider | `QueryContext` returns `sql.ErrNoRows` as its error | propagate | no snapshot → no bind → statement sees resolver error path, never A |
| PA-4h | provider | `rows.Err()==sql.ErrNoRows` after `Next()=false` | propagate | never Absence |

## PA-5 — stale channels & own-apply (non-empty controls)

| row | layer | deltas | post `srsExistenceStale` | post `srsDepsStale` |
| --- | --- | --- | --- | --- |
| PA-5a | enrich-apply | initial `{77777}`, `depsStale=true` | `{77777}` | `true` |
| PA-5b | enrich-apply | PA-2b sql, same non-empty initials | `{77777}` | `true` |
| PA-5c | enrich-apply | PA-2c sql, same initials | `{77777}` | `true` |
| PA-5d | enrich-apply | initial `{100001}` | `{100001}` (B arm — bind short-circuits, zero catalog reads) | unchanged |
| PA-5e | enrich-apply | existence row present (S2 control) | `{77777,100001}` | `true` |

## PA-6 — ordering & cache (per-statement and per-call deltas)

| row | layer | sequence | expected |
| --- | --- | --- | --- |
| PA-6a | api | `DROP … 100001; DROP … 100001;` (two stmts, one enrich) | stmt1 A-plain: R=1,Qexist=1,Qdeps=1,Qo=1; stmt2 B-arm? No — stmt1 mints nothing → stmt2 binds again, **cache hit**: ΔR=0,ΔQexist=0,ΔQdeps=0,ΔQo=0 (key `(schema,type,100001,∅)`); stmt2 also A-plain |
| PA-6b | api | `DROP … 100001; DROP … 100002;` | independent keys → stmt2 ΔR=1,ΔQexist=1,ΔQdeps=1,ΔQo=1 |
| PA-6c | api | MySQL-only batch: stmt1 `DROP SPATIAL REFERENCE SYSTEM 100001` bound under a below-floor observed banner `8.0.3` (version-gate Unsupported → boundary own-apply mints `{100001}`); stmt2 same SQL under same request | stmt2 sees `srsExistenceStale[100001]` → **B**, zero catalog reads on stmt2; both statements are real same-dialect path — bound to the accepted R3 boundary-mint witness (TiDB real-parse boundary stays its own separate row in the accepted privilege evidence) |
| PA-6d | enrich-apply | two `enrichOrderedStatements` calls sharing `batchState`, call1 absence, call2 existence answer CHANGED to present row | call1 A-plain; call2 builds a NEW objectCache → fresh reads → S2 `[D]`; proves new cache actually re-reads |

## PA-7 — REVOKE scoping (full sequence)

| row | layer | sequence | expected |
| --- | --- | --- | --- |
| PA-7a | api | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;` `REVOKE SELECT ON *.* FROM 'r1'@'%';` `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100002;` | stmt1 A-IE✓ consumes P (marker); REVOKE sets scopeEnded; stmt3 A-IE? suppressed → owes `[P]`; stale channels stay `{}/false`; stmt1's metadata (premise/arm/marker) identical at post-own-apply, post-REVOKE, and final snapshots; REVOKE's own public output and aggregate contribution declared observed separately (request-layer row asserts them; internal row marks them N/A — out of scope) |

## PA-8 — version seams (request vs provider separated)

| row | layer | deltas | expected |
| --- | --- | --- | --- |
| PA-8a | api | request banner `8.0.3`, provider banner `8.0.3` | version gate (D-qualify) → `Unsupported{boundary:unaudited,gate:version}` before bind; provider reads still issue per existing contract; own-apply mints `{100001}` (accepted boundary semantics) |
| PA-8b | api | provider banner `!proven` while request pin admission ok | PA-1c equivalent: reads issue, zero-row clean, `Absence` unset → S2 |
| PA-8c | api | `ErrTargetVersionMismatch` (pin 8.0.37 vs observed 8.0.36) | typed request-layer exit: `ErrTargetVersionMismatch`; Result carries audited statements with public fields N/A; **Vd=1, SRS seams R=Qexist=Qdeps=Qo=0** — Vd and SRS reads counted separately, never "zero provider reads" |
| PA-8d | unit | TiDB real parse of the DROP SQL | vendor boundary statement; no SRS reads; own-apply mints `{100001}` — kept as its own boundary witness, never mixed with the MySQL path |

## PA-9 — supply failures (injection seam, attempts, unreachable points)

| row | layer | injection | result |
| --- | --- | --- | --- |
| PA-9a | provider | `QueryContext` returns generic error (1 attempt) | propagates wrapped; bind/adjudication/apply for that statement unreachable |
| PA-9b | provider | iteration error mid-`Next()` | `rows.Err()` propagates; `Absence` untouched; no snapshot delivered |
| PA-9c | provider | ctx cancellation | `Err()`=ctx error → propagate; no snapshot |
| PA-9d | api | provider lacks `ObjectResolver` | `Status=Unavailable` snapshot; S2 `[E,D,P]` |
| PA-9e | provider | clean zero rows under `!proven` banner | `Read=true,Exists=false,Absence=false` → S2 |
| PA-9f | provider | existence clean zero → dependency read errors | dependency error propagates; snapshot never delivered — the helper's local `Absence` bit is NOT evidence of publication |

## PA-10 — public shape & policy

| row | layer | deltas | expected |
| --- | --- | --- | --- |
| PA-10a | public | srid `4326`, absent, plain | `absent` blocker only — NO reserved advisory |
| PA-10b | public | `DROP … IF EXISTS 4326`, absent, consumable | `absent_noop` notice + exactly 1 membership-only `reserved` notice |
| PA-10c | public | `DROP … IF EXISTS 4326`, absent, unproven | gap `[P]` + exactly 1 `reserved` notice |
| PA-10d | public | policy: `absent` disabled | no finding; bound state, premise, own-apply unchanged; verdict recomputed |
| PA-10e | public | policy: `absent_noop` level→warning | finding becomes warning → review; `conditional_audit` still present |
| PA-10f | public | sentinel scan: `SRS_ZZTEST`,`zz_acct`,`zz_schema`,`zz_table`,`zz_col` must not appear anywhere in the serialised result | bounded metadata only |
| PA-10g | cli | reference only: accepted rc0/1/2 providerless probes (`56bcf05f` evidence) | unchanged shapes; not proven-absence evidence |

## Verification plan (future execution work order)

1. Spec unit table for `AdjudicateSRSDrop` incl. IfExists split, S1/A
   ordering and invalid-absence rows.
2. API rows against the stub-driver provider: zero-row answers, the
   two-read conflict sequence, double-enrich with changed second
   answer.
3. Internal rows for ValidAbsence fields, per-statement/per-call read
   deltas, non-empty stale controls, REVOKE sequence.
4. Serialized-shape assertions for both new findings + the
   `unproven_privilege` gap on real JSON paths.
5. Frozen providerless CLI probes remain reference-only.
