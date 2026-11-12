# T26-SRS-DROP-ORDERED-PUBLICATION-CONTRACT-01 — discriminant matrix (R4)

Every row is a **PLAN / UNEXECUTED** discriminant — nothing here is
implemented or measured this round. All numbers are expected values,
never executed readings.

**Layers** (explicit names; no letter codes):
- `internal` — `package audit` test seam: batchState + export-test hooks,
  custom resolvers, premise seeding. Resolved version is supplied
  directly (no discovery): `Vd = N/A` on every internal row.
- `realchain` — `package audit_test` real-provider seam
  (`mysqlmeta.NewProvider` over the recorded stub driver): the full
  bind→adjudicate→apply chain.
- `public` — `pkg/deltascope` API surface (`AuditSQL`/`Audit` + request
  discovery).
- `provider` — `internal/infrastructure/metadata/mysql` unit seam.

**Read vector** is always the fixed 4-tuple `(R, Qexist, Qdeps, Qo)`
per statement, where **R counts resolver dispatches actually forwarded
to the provider** — a cache return is a `cache-hit` event (R=0, all SQL
deltas 0), never a forwarded dispatch. `Qexist 0 (skipped)` marks the
ordered-hit skip — registered, never counted as executed.
`L` is a separate per-enrich column (privilege-observation loader,
attributed to the entry loader gate, not to a statement).
`Vd` is a separate per-request column (version discovery).
`SQL seams = N/A` when the wired resolver emits no SQL — never a
measured 0. `unobserved` marks a dimension the row does not measure,
with the reason stated. Totals are listed separately from deltas.

**Closed default fixture** (every row inherits; overrides listed per
row): dialect `mysql`; target-version pin `8.0.36`; discovery banner
`8.0.36-log`; provider Qo banner `8.0.36`; provider capabilities =
full ObjectResolver + `MySQLServerDropPrivilegeLoader` +
deps-capable SRS reader (`MySQLSRSDependencyReader`); account `app@%`;
grants row `GRANT SELECT, SUPER ON *.* TO `app`@`%``; privilege mode
`observed_account_new_session`; policy = the registered defaults:
`ddl.mysql.srs.drop.srid0` blocker, `.dependency` blocker, `.absent`
blocker, `.reserved` notice, `.privilege` notice, `.absent_noop`
notice (per `defaults_ddl.go`); initial state: `srsOrdered` empty,
`objectCache` empty, `srsExistenceStale` empty, `srsDepsStale` false,
`contaminated` false; **default query answers**: Qexist returns clean
zero observed rows (certified absence in-domain); Qdeps returns a
successful empty witness set on BOTH the full and the deps-only path;
iteration and checked close are healthy; named deps-answer fixture
`s.t.g` (a `s.t` geometry column declared SRID 100001) is available
only as a deps answer — rows that need an actual `DROP/ALTER TABLE s.t`
additionally cite the accepted table fixture + `LoadTableSnapshot`
binding; internal-layer resolvedVersion supplied directly = `8.0.36`.

## OP-1 — producer arms

| id | SQL | layer | overrides | arms (producer→consumer) | post-state asserts | reads Δ (R,Qexist,Qdeps,Qo) / L / Vd |
|---|---|---|---|---|---|---|
| OP-1a | `DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | none | stmt1 A-plain → produces entry `{100001,absent,arm=absent_refused,ownPrivilege=consumable,barrier:false}` | entry present after own-apply; no stale mint; no contamination; `ownPrivilege` records the ACTUAL consumable premise — a refusal arm carries it, never consumes it (premise-not-consumed is a statement/adjudication-level assertion, not an entry field) | Δ(1,1,1,1); L=1; Vd=1 |
| OP-1b | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;` | realchain | none | stmt1 A-IE✓ → produces `{arm=absent_noop,P=consumable}` | entry present; marker on stmt1 finding | Δ(1,1,1,1); L=1; Vd=1 |
| OP-1c | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;` | realchain | grants=`GRANT SELECT ON *.* TO 'app'@'%'` | stmt1 A-IE? → produces `{arm=absent_conditional,P=unproven}` | entry present; stmt1 owes `[P]`; absence fact unconditional | Δ(1,1,1,1); L=1; Vd=1 |
| OP-1d | `DROP SPATIAL REFERENCE SYSTEM 0;` | realchain | none | stmt1 S0 → no produce | no entry; one blocker; reads DID issue (adjudication follows bind) | Δ(1,1,1,1); L=1; Vd=1 |
| OP-1e | `DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | deps answer `[s.t.g@100001]` | stmt1 S1 → no produce | no entry despite certified absence (positive-witness arm never publishes) | Δ(1,1,1,1); L=1; Vd=1 |
| OP-1f | `DROP TABLE s.t; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | accepted table fixture `s.t` bound via `LoadTableSnapshot` | stmt1 spatialTouching → `srsDepsStale`; stmt2 A-plain → produces | tail is A-plain, NOT B — `srsDepsStale` demotes witness consumption only; existence entry unaffected; corrected vs R1 | stmt1 SRS-seam Δ(0,0,0,0) (table seams measured separately — unobserved here, table binding is not an SRS-vector dimension); stmt2 Δ(1,1,1,1); batch SRS-vector total (1,1,1,1); L=1; Vd=1 |
| OP-1g | `DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | Qo banner `unparseable` | stmt1 S2 → no produce | no entry — absence never certified; `srsExistenceStale{100001}` minted (existing rule); reads still issued (accepted direction: unparseable Qo does not zero queries) | Δ(1,1,1,1); L=1; Vd=1 |
| OP-1h | `CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 100001 DEFINITION 'x'; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | none | stmt1 implementation boundary → mints `srsExistenceStale{100001}`; stmt2 B | no entry — B publishes nothing; stmt2 owes `[B]` | stmt1 Δ(1,1,1,1) (boundary binds via full path — ordered gate excludes it); stmt2 Δ(0,0,0,0); batch total (1,1,1,1); L=1 attributed to the enrich-entry loader; Vd=1 |
| OP-1i | `DROP SPATIAL REFERENCE SYSTEM 100001;` | internal | custom resolver returns SRS state `{SRID:100002, Read:true, Exists:false, Absence:true, Dependencies:nil}` for the authored 100001; grants=`GRANT SELECT ON *.* TO 'app'@'%'` (negative — the enrich-entry loader produces the unproven premise; no seed conflicts) | stmt1 S2 → no produce | neither 100001 nor 100002 gets an entry — foreign-SRID absence never promoted | SQL seams N/A (custom resolver emits no SQL); R=1 dispatch; L=1; Vd=N/A |
| OP-1j | `DROP SPATIAL REFERENCE SYSTEM 100001;` | internal | custom resolver returns `{SRID:100001, Read:true, Exists:true, Absence:true, Dependencies:nil}`; grants=`GRANT SELECT ON *.* TO 'app'@'%'` (premise unproven via loader) | stmt1 S2 → no produce | no entry — `ValidAbsence` requires `!Exists`; contradictory fields never promoted | SQL seams N/A; R=1; L=1; Vd=N/A |

## OP-2 — hit & lifecycle

| id | SQL | layer | overrides | arms | post-state asserts | reads Δ / L / Vd |
|---|---|---|---|---|---|---|
| OP-2a | `DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | none | stmt1 A-plain→produce; stmt2 ordered-hit A-plain | stmt2 consumed the entry: `OrderedAbsent` bound; identical blocker shape | stmt1 Δ(1,1,1,1); stmt2 Δ(1,0(skipped),1,1); batch total R=2,Qexist=1,Qdeps=2,Qo=2; L=1; Vd=1 |
| OP-2b | `DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100002;` | realchain | zero-row for both | two A-plain→produce | entries `{100001,100002}` independent; stmt2 misses 100001's entry → full own read | both Δ(1,1,1,1); total (2,2,2,2); L=1; Vd=1 |
| OP-2c | `DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | none | both A-plain | distinguishes ordered-hit vs objectCache-hit explicitly: under PA-6a a second-statement cache return is R0/`cache-hit`/all-0; under this contract stmt2 is R1+Qexist skipped+Qdeps1+Qo1 — vectors differ and are asserted separately | same as OP-2a |
| OP-2d | `DROP SPATIAL REFERENCE SYSTEM 100001;` — two enrich passes over one SHARED batchState; provider answers mutated between passes: call1 full answer = clean-zero absence; call2 would-be full answer = `present {100001}` row; deps-only answer each call = empty | internal | `batchState` reused; `objectCache` rebuilt per pass; `SeedResolvedVersion` supplies 8.0.36; grants=`GRANT SELECT ON *.* TO 'app'@'%'` (unproven premise both passes, produced by the loader) | pass1 A-plain→produce; pass2 ordered-hit A-plain | **intentional-extension registration**: entry survives the per-enrich objectCache rebuild; pass2's ordered hit does NOT re-read the changed presence answer — it binds `OrderedAbsent` + fresh empty deps → A-plain; divergence from PA-6d baseline registered as the model decision | pass1 Δ(1,1,1,1); pass2 Δ(1,0(skipped),1,1); combined total (2,1,2,2); L=1 per enrich; Vd=N/A per pass |
| OP-2e | `DROP SPATIAL REFERENCE SYSTEM 100001;` — same text, `newBatchState` between the two enrich passes; inherits OP-2d's full fixture verbatim — same mutated provider answers, same negative grant, both premises unproven; the ONLY difference is `newBatchState` at pass2 | internal | fresh batchState pass2 | pass1 A-plain; pass2 full read → present witness → **S2** | no inheritance: pass2 issues the real full lookup, observes the present row, adjudicates S2 (existence owed discharged differently — present exists); the comparable pair with OP-2d proves the ordered channel, not caching, made the difference | both Δ(1,1,1,1); combined (2,2,2,2); L=1 each enrich; Vd=N/A |
| OP-2f | `DROP SPATIAL REFERENCE SYSTEM 100001; DROP TABLE s.t; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | accepted table fixture `s.t` bound via `LoadTableSnapshot`; deps answer stays empty | stmt1 A-plain→produce; stmt3 ordered-hit A-plain | `srsDepsStale` minted at stmt2 does NOT invalidate the existence entry; stmt3 still consumes it (deps witness empty + stale — both directions reach A) | stmt1 Δ(1,1,1,1); stmt2 Δ(0,0,0,0) SRS-seam (table seams unobserved — not an SRS vector dimension); stmt3 Δ(1,0(skipped),1,1); total R2,Qexist1,Qdeps2,Qo2; L=1; Vd=1 |
| OP-2g | `DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;` — same enrich, active entry, NO capability, warm cache | internal | provider satisfies full `ObjectResolver` + privilege loader but NOT `MySQLSRSDependencyReader` → `srsDepsReader=nil` (any wrapper must not accidentally expose the optional interface); grants=`GRANT SELECT ON *.* TO 'app'@'%'` (premise unproven, loader-produced); call1 full answer = clean-zero absence + empty deps | stmt1 full lookup → A-plain → mints the only entry; stmt2 eligible DROP + active entry but no capability → ordered entry NOT consumed; full-key cache-hit serves the bind → A-plain | stmt2 `OrderedAbsent=false`, bound Read/Absence come from the cached full answer; exactly one entry, first-producer provenance unchanged; no new stale, no barrier flip | stmt1 Δ(1,1,1,1); stmt2 Δ(0,0,0,0) + cache-hit=1; total (1,1,1,1); L=1; Vd=N/A (internal) |
| OP-2h | `DROP SPATIAL REFERENCE SYSTEM 100001;` per enrich call — shared batchState, active entry, NO capability, cold cache | internal | same capability/negative-grant/version fixture as OP-2g; same batchState reused across two enriches; objectCache rebuilt per enrich; call1 full answer = clean-zero absence + empty deps; call2 full answer = `present {100001}` + empty deps | call1 A-plain → publishes entry; call2: active entry present but no capability → REAL full lookup, `OrderedAbsent=false`, reads present → premise unproven → **S2** | call2 owed facts exactly `[srs.column_dependencies, srs.drop.privilege]`; call2 own-apply mints `srsExistenceStale{100001}` + flips the same entry's barrier false→true; first-producer provenance kept, no new absence source | each call Δ(1,1,1,1); combined (2,2,2,2); L=1 per enrich; Vd=N/A (internal) |

## OP-3 — own privilege premise & source independence

| id | SQL | layer | overrides | arms | post-state asserts | reads Δ / L / Vd |
|---|---|---|---|---|---|---|
| OP-3a | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001; DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;` | realchain | none | stmt1 A-IE✓→produce; stmt2 ordered-hit A-IE✓ | BOTH findings carry their own `conditional_audit` marker — per-statement premise consumption, attributed independently; the entry carries no inheritable marker | stmt1 Δ(1,1,1,1); stmt2 Δ(1,0(skipped),1,1); total (2,1,2,2); L=1; Vd=1 |
| OP-3b | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001; REVOKE SELECT ON *.* FROM 'app'@'%'; DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;` | realchain | none | stmt1 A-IE✓→produce; stmt2 REVOKE unsupported + scope terminator; stmt3 ordered-hit A-IE? | stmt3 premise re-derived `suppressed` → `[P]` gap; stmt1's finding unchanged; entry unaffected by the premise terminator | stmt1 Δ(1,1,1,1); stmt2 Δ(0,0,0,0); stmt3 Δ(1,0(skipped),1,1); total (2,1,2,2); L=1 (once per enrich, attributed to the enrich-entry observation — not inside stmt1's apply); Vd=1 |
| OP-3c | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001; REVOKE SELECT ON *.* FROM 'app'@'%'; DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;` | realchain | grants=`GRANT SELECT ON *.* TO 'app'@'%'` | stmt1 A-IE?; stmt3 ordered-hit A-IE? | both owe `[P]`; premise never consumable; no retro-modification | same vectors as OP-3b |
| OP-3d | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | grants=`GRANT SELECT ON *.* TO 'app'@'%'` | stmt1 A-IE?→produce; stmt2 ordered-hit A-plain | stmt2 is a plain drop on a hit → absent_refused; premise never consulted on A-plain | stmt1 Δ(1,1,1,1); stmt2 Δ(1,0(skipped),1,1); total (2,1,2,2); L=1; Vd=1 |

## OP-4 — conflicts, reads, error policy

| id | SQL | layer | overrides | arms | post-state asserts | reads Δ / L / Vd |
|---|---|---|---|---|---|---|
| OP-4a | `DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | deps answers mutate per call: stmt1 `[]`, stmt2 `[]`, stmt3 `[s.t.g@100001]` | stmt1 A-plain→produce; stmt2 ordered-hit A-plain; stmt3 ordered-hit → **S1** | THE per-hit-freshness discriminant: stmt3's deps-only read returns the new positive witness — S1 outranks the ordered absence; NO spatialTouching exists, `srsDepsStale` stays false, witness is consumable; entry survives | stmt1 Δ(1,1,1,1); stmt2 Δ(1,0(skipped),1,1); stmt3 Δ(1,0(skipped),1,1); total (3,1,3,3); L=1; Vd=1 |
| OP-4b | `DROP SPATIAL REFERENCE SYSTEM 100001; DROP TABLE s.t; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | accepted table fixture `s.t` bound via `LoadTableSnapshot`; deps answer at stmt3 = `[s.t.g@100001]` (positive but stale-demoted) | stmt1 A-plain→produce; stmt2 mints `srsDepsStale`; stmt3 ordered-hit A-plain | positive witness EXISTS at stmt3 but `srsDepsStale` demotes it → A arm stands via the ordered entry — distinguishes "witness demoted by staleness" from "no witness" | stmt1 Δ(1,1,1,1); stmt2 Δ(0,0,0,0) SRS-seam; stmt3 Δ(1,0(skipped),1,1); total (2,1,2,2); L=1; Vd=1 |
| OP-4c | `DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | deps query error injected at stmt2's deps-only call | stmt1 A-plain→produce; stmt2 = read error | error propagates; stmt2 never adjudicated/apply-minted; stmt1's bound snapshot and the `{100001}` entry unchanged | stmt1 Δ(1,1,1,1); stmt2 Δ(1,0(skipped),1,1) — Qo+deps attempted before the error; planned batch total (2,1,2,2); L=1; Vd=1 |
| OP-4d | `DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;` (bound to OP-2a's batch and internal hook seam) | internal | stub counters | n/a | the skipped existence query is recorded as `skipped-by-ordered`, never as `executed`/`rows=0`; distinct from cache-hit (R0 + all-0) | stmt2 Δ(1,0(skipped),1,1); L=1 |

## OP-5 — invalidation

| id | SQL | layer | overrides | arms | post-state asserts | reads Δ / L / Vd |
|---|---|---|---|---|---|---|
| OP-5a | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001; CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 100001 DEFINITION 'x'; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | none | stmt1 A-IE✓→produce; stmt2 implementation boundary — NOT ordered-eligible (`Unsupported!=nil`), takes the existing full path: `srsExistenceStale` for 100001 is not yet minted at ITS bind → resolver → objectCache holds stmt1's full-lookup answer → `cache-hit`; boundary apply mints `srsExistenceStale{100001}` + entry `barrier=true`; stmt3 blocked-B | entry kept (`barrier=true`); stmt3 owes `[B]`; no A arm | stmt1 Δ(1,1,1,1); stmt2 Δ(0,0,0,0) + `cache-hit=1` (R0 — the full key was memoized by stmt1); stmt3 Δ(0,0,0,0); total R1,Qexist1,Qdeps1,Qo1 + 1 cache-hit; L=1 attributed to the entry loader; Vd=1 |
| OP-5b | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001; DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100002; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | zero rows both SRIDs | all A arms | 100001's entry untouched by 100002's publication; stmt3 ordered-hit A-plain | stmt1/2 Δ(1,1,1,1) each; stmt3 Δ(1,0(skipped),1,1); total (3,2,3,3); L=1; Vd=1 |
| OP-5c | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001; GARBAGE@#$; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | middle statement is a real parse failure (batch record, not an SRS boundary) | stmt1 A-IE✓→produce; failure sets `contaminated`; stmt3 blocked-B | whole-batch contamination shadows the channel: stmt3 exits at step 2 before the ordered lookup — entry kept, never consumed while contaminated | stmt1 Δ(1,1,1,1); stmt3 Δ(0,0,0,0); total (1,1,1,1); L=1; Vd=1 |
| OP-5d | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001; DROP TABLE s.t; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | accepted table fixture `s.t` bound via `LoadTableSnapshot`; deps answer empty | stmt1 A-IE✓→produce; stmt3 ordered-hit **A-plain** (corrected vs R1 — tail is plain DROP) | `srsDepsStale` does not invalidate the existence entry; stmt3 consumes it as absent_refused | stmt1 Δ(1,1,1,1); stmt2 Δ(0,0,0,0); stmt3 Δ(1,0(skipped),1,1); total (2,1,2,2); L=1; Vd=1 |
| OP-5e | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001; ALTER TABLE s.t ENGINE=InnoDB; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | accepted table fixture `s.t` bound via `LoadTableSnapshot`; inert ALTER (engine-only, complete action list) | stmt1 A-IE✓→produce; stmt3 ordered-hit A-plain | inert op mints neither flag — clean consumption | as OP-5d |
| OP-5f | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001; REVOKE SELECT ON *.* FROM 'app'@'%'; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | none | stmt1 A-IE✓→produce; stmt3 ordered-hit A-plain | REVOKE is premise-level only — never invalidates the fact-level entry; stmt3 re-derives own premise (suppressed) but plain drops never consult it | as OP-3b vectors |

## OP-6 — admittance & target boundary

| id | SQL | layer | overrides | arms | post-state asserts | reads Δ / L / Vd |
|---|---|---|---|---|---|---|
| OP-6a | `DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | none | A-plain→produce | baseline in-domain publish path | Δ(1,1,1,1); L=1; Vd=1 |
| OP-6b | `DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | below-floor fixture: NO pin; observed/resolved `8.0.3`; Qo `8.0.3` | version boundary → no produce | boundary row: no entry; `srsExistenceStale{100001}` minted at own-apply (accepted authored-stale rule); boundary binds via full path (ordered gate excludes it); reads issue | Δ(1,1,1,1); L=0 (no audited drop → loader gate not entered); Vd=1 |
| OP-6c | `DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | dialect=`tidb`; NO pin; recognized TiDB banner `8.0.11-TiDB-v8.5.0` (product/dialect=tidb, parsed version 8.5.0) via real parse path | vendor boundary → no produce | no entry; no reads; boundary own-apply mints `srsExistenceStale{100001}` (accepted); public/realchain request layer and an internal companion (resolved identity supplied directly, Vd=N/A) registered separately | Δ(0,0,0,0); L=0; Vd=1 (real discovery) / N/A (internal companion) |
| OP-6d | `DROP SPATIAL REFERENCE SYSTEM 100001;` | public | providerless; target-version pin `8.0.36` | S2 → no produce | providerless never supplies certified absence; public projection unchanged | provider seams N/A — no provider attached (not measured-0); L=0; Vd=N/A (pin supplies the version) |
| OP-6e | `DROP SPATIAL REFERENCE SYSTEM 100001;` | public | malformed `target-version` pin (`8.0.x.x`) | typed preflight exit | request-level `ErrInvalidTargetVersion` (asserted via `errors.Is`); public projection N/A; no entry, no arm | installed counters show zero attempts — seams reported as 0-attempts-with-counters vs N/A projection: stated separately; L=0; Vd=0 — discovery never reached |

## OP-7 — public projection

| id | SQL | layer | overrides | arms | post-state asserts | reads Δ / L / Vd |
|---|---|---|---|---|---|---|
| OP-7a | `DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;` | public | none | stmt1 A-plain; stmt2 ordered-hit A-plain | each statement carries exactly one `ddl.mysql.srs.drop.absent` blocker — compared on fixed rule/level/message/metadata; positional index/location asserted separately (omitempty preserved); **aggregate coverage=complete, verdict=reject** (declared whole-request row) | stmt1 Δ(1,1,1,1); stmt2 Δ(1,0(skipped),1,1); total (2,1,2,2); L=1; Vd=1 |
| OP-7b | OP-3b's three-statement batch | public | none | A-IE✓ + REVOKE-unsupported + A-IE? | stmt1: noop finding + `conditional_audit` marker; stmt3: `[P]` gap only; batch returns `ErrUnsupportedStatement`; per-statement asserts only (no aggregate row) | as OP-3b |
| OP-7c | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326; DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326;` | public | reserved SRID 4326 | stmt1 A-IE✓; stmt2 ordered-hit A-IE✓ | each statement: absent_noop notice finding + marker + reserved advisory `{srid}` — the reserved advisory co-fires here because the arm is not a refusal | as OP-3a |
| OP-7c′ | `DROP SPATIAL REFERENCE SYSTEM 4326; DROP SPATIAL REFERENCE SYSTEM 4326;` | public | reserved SRID 4326 | stmt1 A-plain; stmt2 ordered-hit A-plain | each statement: exactly the absent blocker — A-plain is a refusal arm and never carries the reserved advisory | as OP-7a |
| OP-7d | `DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;` | public (+internal companion) | explicit policy `rules: {ddl.mysql.srs.drop.absent: {enabled: false}}` | both A-plain internally | **public asserts**: per statement complete finding/gap inventory — exactly zero findings, zero gaps (the absent rule is disabled; no reserved SRID in play); per-statement coverage complete; verdict `pass` (no enabled blocker or warning fires). **internal companion asserts**: exactly one entry for the SRID exists after both applies, first-producer provenance unchanged (stmt2 consume/keep, never re-mint; policy never gates publication) | reads unchanged |
| OP-7e | `DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;` | public (+internal companion) | explicit policy `rules: {ddl.mysql.srs.drop.absent: {enabled: true, level: warning}}` | both A-plain | **public asserts**: each statement carries the absent finding at warning + zero gaps; per-statement coverage complete; verdict `review`. **internal companion asserts**: exactly one entry for the SRID exists after both applies, first-producer provenance unchanged (stmt2 consume/keep, never re-mint) | reads unchanged |

## OP-8 — snapshots & isolation (two frozen observation classes)

| id | SQL | layer | overrides | asserts | reads Δ / L / Vd |
|---|---|---|---|---|---|
| OP-8a | OP-3b's three-statement batch (`DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001; REVOKE SELECT ON *.* FROM 'app'@'%'; DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;`) | internal export-hook seam (bound/channel deep-value snapshots + per-hook deltas); public layer owns only the discovery + final projection | none | **(1) bound-state invariance**: stmt1's `SpatialReference`+`SRSDrop` metadata deep-copied post-bind, post-own-apply, post-REVOKE-apply, post-stmt3/final — all four identical (publication must not mutate the producer's own bound state). **(2) channel transition**: post-bind `srsOrdered` has no 100001 entry; post-own-apply exactly `{absent,arm=absent_noop,ownPrivilege=consumable,barrier:false}`; post-REVOKE and post-stmt3 the SAME earliest provenance (re-publication keeps the first producer). nil-vs-non-nil-empty distinguished per slice; bidirectional mutation isolation both directions; publication itself is NOT judged a snapshot mutation | stmt vectors as OP-3b; hook-measured deltas per statement; L=1 attributed to the enrich-entry loader; Vd=N/A internal / 1 at the public companion |
| OP-8b | OP-5a's three-statement batch | internal export-hook seam | none | entry-only compare: before boundary `{absent,barrier=false}`; after boundary `{absent,barrier=true}` — barrier flip is the ONLY entry change; the whole-channel compare additionally registers the boundary-minted `srsExistenceStale{100001}` key and unchanged `srsDepsStale`/`contaminated` | as OP-5a; hook-measured per statement |
| OP-8c | `DROP SPATIAL REFERENCE SYSTEM 100001;` — one producer statement only | internal export-hook seam | pre-state: NO 100001 entry; unrelated entry `100002` preset with full provenance value; non-empty controls — SERVER `s1` bound entry, `s.t`/`s` schema table fixture, `srsExistenceStale`/`srsDepsStale`/`contaminated` at baseline values | the single own-apply window mints exactly `{100001,absent,arm=absent_refused,ownPrivilege=consumable,barrier:false}` (premise-not-consumed asserted at statement/adjudication level, not an entry field) and nothing else: `serverEntries`, table entries, both stale channels, `contaminated`, and the preset `100002` entry compared verbatim before/after | Δ(1,1,1,1); L=1; Vd=N/A |
