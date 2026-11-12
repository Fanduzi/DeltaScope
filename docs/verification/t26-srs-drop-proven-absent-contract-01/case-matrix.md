# T26-SRS-DROP-PROVEN-ABSENT-CONTRACT-01 — case matrix (PLAN / UNEXECUTED)

All rows are candidate contract rows, not executed tests. Layers:
**API** = request-level `AuditSQL` with stub-driver provider;
**int** = batch/enrich unit slice; **CLI** = providerless binary probe.
Default SRIDs: `100001`/`100002` non-reserved, `4326` reserved, `0`
refusal. Mode `observed_account_new_session` where P matters.
Read vector: R resolver forwards, Qexist/Qdeps catalog queries,
Qo catalog-support `select version()`, L privilege loader calls,
Vd version-discovery calls. `—` = not applicable; `N/A` = not observed.

## PA-1 — zero-row without/with completeness proof

| row | layer | input | supply | arm | owed | finding | own-apply | vector |
|---|---|---|---|---|---|---|---|---|
| PA-1a | API | `DROP SPATIAL REFERENCE SYSTEM 100001;` providerless | none | S2 | `[E,D,P]` | notice only if P… none; gap `unknown_srs_state` | mint `{100001}` | R=0,Qexist=0,Qdeps=0,Qo=0,L=0,Vd=N/A |
| PA-1b | API | same, provider, proven 8.0.36, zero rows | complete | A-plain | `[]` | `absent` blocker `refused_absent` | none | R=1,Qexist=1,Qdeps=1,Qo=1,L=mode?1:0,Vd≥1 |
| PA-1c | int | same but version `!proven`, zero rows | out-of-subdomain | S2 | `[E,D,P]` | gap | mint `{100001}` | reads still issue per provider contract |

PA-1a vs PA-1b is the unknown↔absent discriminant: identical SQL and
identical row count differ only by the supply predicate (A4).

## PA-2 — plain / IF EXISTS × premise

| row | layer | input | P | arm | owed | projection | own-apply |
|---|---|---|---|---|---|---|---|
| PA-2a | API | `DROP … 100001` absent | unproven | A-plain | `[]` | `absent` blocker | none |
| PA-2b | API | `DROP … IF EXISTS 100001` absent | consumable | A-IE✓ | `[]` | `absent_noop` notice `{srid,outcome,conditional_audit}` | none |
| PA-2c | API | `DROP … IF EXISTS 100001` absent | unproven | A-IE? | `[P]` | gap `unproven_privilege` | none |
| PA-2d | API | `DROP … IF EXISTS 100001` absent | suppressed (REVOKE earlier) | A-IE? | `[P]` | gap `unproven_privilege` | none |

## PA-3 — existing S2 control rows (unchanged)

| row | layer | input | arm | owed |
|---|---|---|---|---|
| PA-3a | API | proven present, P consumable | S2 | `[D]` + privilege notice |
| PA-3b | API | unknown existence, P unavailable | S2 | `[E,D,P]` |

## PA-4 — conflicts (B3)

| row | input state | arm | note |
|---|---|---|---|
| PA-4a | absence + positive deps witness (unit-layer construct only) | A-plain | documented contradiction; complete proof outranks witness; refused_absent matches server ordering |
| PA-4b | absence + srid 0 | S0 | unreachable in practice (SRID 0 is a permanent DD row) — listed not hidden |
| PA-4c | absence + pre-existing `srsExistenceStale[srid]` | B | stale blocks before read; absence never observed |
| PA-4d | absence + `srsDepsStale` | A-plain/A-IE* | depsStale demotes witnesses only, never the existence fact |
| PA-4e | third-party resolver sets `Exists=false,Absence=false` | S2 | zero-value keeps unproven — old callers unaffected |

## PA-5 — stale channels & own-apply

| row | pre | statement | post `srsExistenceStale` | post `srsDepsStale` |
|---|---|---|---|---|
| PA-5a | `{}`/false | A-plain refused | `{}` | false |
| PA-5b | `{}`/false | A-IE✓ noop | `{}` | false |
| PA-5c | `{}`/false | A-IE? conditional | `{}` | false |
| PA-5d | `{100001}`/false | DROP 100001 | `{100001}` (B arm, unread) | false |
| PA-5e | `{}`/false | S2 drop 100001 | `{100001}` | false (control) |

## PA-6 — ordering

| row | sequence | expected |
|---|---|---|
| PA-6a | `DROP 100001`(absent) then `DROP 100001` | both A arms; second read sees same proven absence (cache serves first snapshot) |
| PA-6b | `DROP 100001`(absent) then `DROP 100002` | independent; second own read |
| PA-6c | TiDB boundary SRS write then MySQL DROP same SRID | B — boundary minted stale; no dialect mixing |
| PA-6d | `DROP TABLE s.t` (touching) then absent-SRID DROP | depsStale=true; A arm unaffected (deps not consulted) |

## PA-7 — REVOKE scoping

| row | sequence | expected |
|---|---|---|
| PA-7a | grant witness; `REVOKE SELECT ON *.*`; `DROP … IF EXISTS`(absent) | scopeEnded without consuming → A-IE? owes `[P]`; snapshot before REVOKE unchanged |

## PA-8 — version subdomain & typed exits

| row | layer | condition | expected |
|---|---|---|---|
| PA-8a | API | resolved 8.0.3 (proven, below floor) | version boundary `unaudited`; reads still issued; no `Absence` consumer (no adjudication attaches) |
| PA-8b | API | TiDB real parse of DROP SRS | vendor boundary; no SRS reads; own-apply mints `{srid}` (accepted behavior) |
| PA-8c | API | banner malformed → `!proven` | read issues, clean zero-row ⇒ `Absence=false` ⇒ S2 `[E,D,P]` |
| PA-8d | API | `ErrTargetVersionMismatch` (pin 8.0.37 vs observed 8.0.36) | typed exit; Result preserved; zero provider reads |

## PA-9 — supply failures

| row | failure | result |
|---|---|---|
| PA-9a | existence query error | propagates `metadataObjectReadError`-wrapped; never absence |
| PA-9b | scan/iteration error on deps | propagates; `Absence` untouched |
| PA-9c | ctx cancellation mid-read | propagates; no state written |
| PA-9d | provider unsupported / absent | `Status=Unavailable` snapshot; S2 unknown |
| PA-9e | `sql.ErrNoRows` under `!proven` version | `Read=true,Exists=false,Absence=false` → S2 |

## PA-10 — public shape & hygiene

| row | check |
|---|---|
| PA-10a | reserved `4326` + absent → `absent` blocker + reserved advisory co-fires (non-refusal? → co-fires on A-IE* only; refused_absent gets no advisory) |
| PA-10b | `absent` blocker metadata exactly `{srid:uint32, outcome:"refused_absent"}`; `absent_noop` exactly `{srid,outcome,conditional_audit}` |
| PA-10c | rule disabled → no finding; premise/gap unaffected |
| PA-10d | no catalog payload, SRS name, account, or grant text in any finding/gap |
| PA-10e | aggregate: refused_absent→reject; absent_noop→pass; absent_conditional→review (request projection rows only) |
| PA-10f | CLI providerless rc0/1/2 probes unchanged (reference, no rerun) |

## Verification plan (future execution work order)

1. Extend spec unit table for `AdjudicateSRSDrop` arm matrix incl.
   IfExists split and contradiction row.
2. API rows with stub-driver zero-row answers under proven 8.0.x.
3. Internal rows for `Absence` zero-value safety (PA-4e) and own-apply
   tables (PA-5/PA-6).
4. New `unproven_privilege` gap token assertion; serialized shape
   checks for both new findings.
5. Re-run the frozen providerless CLI probes unchanged.
