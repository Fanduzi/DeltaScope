# T26-SRS-DROP-ORDERED-PUBLICATION-CONTRACT-01 — discriminant matrix (R2)

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
deps-capable SRS reader; account `app@%`; grants row
`GRANT SELECT, SUPER ON *.* TO `app`@`%``; privilege mode
`observed_account_new_session`; policy = defaults (all SRS drop rules
enabled at blocker level); initial state: `srsOrdered` empty,
`objectCache` empty, `srsExistenceStale` empty, `srsDepsStale` false,
`contaminated` false; named table fixture `s.t` with geometry column
`g` declared SRID 100001 (available to deps answers only);
internal-layer resolvedVersion supplied directly = `8.0.36`.

## OP-1 — producer arms

| id | SQL | layer | overrides | arms (producer→consumer) | post-state asserts | reads Δ (R,Qexist,Qdeps,Qo) / L / Vd |
|---|---|---|---|---|---|---|
| OP-1a | `DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | none | stmt1 A-plain → produces entry `{100001,absent,arm=absent_refused,P=unproven}` | entry present after own-apply; no stale mint; no contamination | Δ(1,1,1,1); L=1; Vd=1 |
| OP-1b | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;` | realchain | none | stmt1 A-IE✓ → produces `{arm=absent_noop,P=consumable}` | entry present; marker on stmt1 finding | Δ(1,1,1,1); L=1; Vd=1 |
| OP-1c | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;` | realchain | grants=`GRANT SELECT ON *.* TO 'app'@'%'` | stmt1 A-IE? → produces `{arm=absent_conditional,P=unproven}` | entry present; stmt1 owes `[P]`; absence fact unconditional | Δ(1,1,1,1); L=1; Vd=1 |
| OP-1d | `DROP SPATIAL REFERENCE SYSTEM 0;` | realchain | none | stmt1 S0 → no produce | no entry; one blocker; reads DID issue (adjudication follows bind) | Δ(1,1,1,1); L=1; Vd=1 |
| OP-1e | `DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | deps answer `[s.t.g@100001]` | stmt1 S1 → no produce | no entry despite certified absence (positive-witness arm never publishes) | Δ(1,1,1,1); L=1; Vd=1 |
| OP-1f | `DROP TABLE s.t; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | table fixture bound | stmt1 spatialTouching → `srsDepsStale`; stmt2 A-plain → produces | tail is A-plain, NOT B — `srsDepsStale` demotes witness consumption only; existence entry unaffected; corrected vs R1 | stmt1 SRS-seam Δ(0,0,0,0) (table seams measured separately — unobserved here, table binding is not an SRS-vector dimension); stmt2 Δ(1,1,1,1); L=1; Vd=1 |
| OP-1g | `DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | Qo banner `unparseable` | stmt1 S2 → no produce | no entry — absence never certified; `srsExistenceStale{100001}` minted (existing rule); reads still issued (accepted direction: unparseable Qo does not zero queries) | Δ(1,1,1,1); L=1; Vd=1 |
| OP-1h | `CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 100001 DEFINITION 'x'; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | none | stmt1 implementation boundary → mints `srsExistenceStale{100001}`; stmt2 B | no entry — B publishes nothing; stmt2 owes `[B]` | stmt1 Δ(1,1,1,1) (boundary binds via full path — ordered gate excludes it); stmt2 Δ(0,0,0,0); L=1 attributed to the entry loader; Vd=1 |
| OP-1i | synthetic: resolver returns `Absence=true` for SRID 100002 while authored SRID is 100001 | internal | custom resolver; SRS state `{SRID:100002,Read:true,Exists:false,Absence:true}`; premise P seeded unproven | stmt1 S2 → no produce | neither 100001 nor 100002 gets an entry — foreign-SRID absence never promoted | SQL seams N/A (custom resolver emits no SQL); R=1 dispatch; L=1 |
| OP-1j | synthetic: `Absence=true` AND `Exists=true` (contradictory) | internal | custom resolver; SRS state `{SRID:100001,Read:true,Exists:true,Absence:true,Dependencies:nil}`; P unproven | stmt1 S2 → no produce | no entry — `ValidAbsence` requires `!Exists`; contradictory fields never promoted | SQL seams N/A; R=1; L=1 |

## OP-2 — hit & lifecycle

| id | SQL | layer | overrides | arms | post-state asserts | reads Δ / L / Vd |
|---|---|---|---|---|---|---|
| OP-2a | `DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | none | stmt1 A-plain→produce; stmt2 ordered-hit A-plain | stmt2 consumed the entry: `OrderedAbsent` bound; identical blocker shape | stmt1 Δ(1,1,1,1); stmt2 Δ(1,0(skipped),1,1); batch total R=2,Qexist=1,Qdeps=2,Qo=2; L=1; Vd=1 |
| OP-2b | `DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100002;` | realchain | zero-row for both | two A-plain→produce | entries `{100001,100002}` independent; stmt2 misses 100001's entry → full own read | both Δ(1,1,1,1); total (2,2,2,2); L=1; Vd=1 |
| OP-2c | `DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | none | both A-plain | distinguishes ordered-hit vs objectCache-hit explicitly: under PA-6a a second-statement cache return is R0/`cache-hit`/all-0; under this contract stmt2 is R1+Qexist skipped+Qdeps1+Qo1 — vectors differ and are asserted separately | same as OP-2a |
| OP-2d | two enrich passes over one shared batchState; provider SRS answer changed between passes (pass 2 would answer present) | internal | `batchState` reused, `objectCache` rebuilt; `SeedResolvedVersion` supplies 8.0.36; P seeded unproven both passes | pass1 A-plain→produce; pass2 ordered-hit A-plain | **intentional-extension registration**: entry survives the per-enrich objectCache rebuild; pass2 does NOT re-read the changed presence answer — divergence from PA-6d baseline registered as the model decision | pass1 Δ(1,1,1,1); pass2 Δ(1,0(skipped),1,1); L=1 per enrich; Vd=N/A per pass (internal layer supplies resolved identity — no discovery runs) |
| OP-2e | same batch text, `newBatchState` between the two enrich passes | internal | fresh batchState pass2 | pass1 A-plain; pass2 full read A-plain | no inheritance — pass2 re-reads the provider fully | both Δ(1,1,1,1); L=1 each enrich; Vd=N/A |
| OP-2f | `DROP SPATIAL REFERENCE SYSTEM 100001; DROP TABLE s.t; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | table fixture bound; deps answer stays empty | stmt1 A-plain→produce; stmt3 ordered-hit A-plain | `srsDepsStale` minted at stmt2 does NOT invalidate the existence entry; stmt3 still consumes it (deps witness empty + stale — both directions reach A) | stmt1 Δ(1,1,1,1); stmt2 Δ(0,0,0,0) SRS-seam (table seams unobserved — not an SRS vector dimension); stmt3 Δ(1,0(skipped),1,1); total R2,Qexist1,Qdeps2,Qo2; L=1; Vd=1 |

## OP-3 — own privilege premise & source independence

| id | SQL | layer | overrides | arms | post-state asserts | reads Δ / L / Vd |
|---|---|---|---|---|---|---|
| OP-3a | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001; DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;` | realchain | none | stmt1 A-IE✓→produce; stmt2 ordered-hit A-IE✓ | BOTH findings carry their own `conditional_audit` marker — per-statement premise consumption, attributed independently; the entry carries no inheritable marker | stmt1 Δ(1,1,1,1); stmt2 Δ(1,0(skipped),1,1); total (2,1,2,2); L=1; Vd=1 |
| OP-3b | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001; REVOKE SELECT ON *.* FROM 'app'@'%'; DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;` | realchain | none | stmt1 A-IE✓→produce; stmt2 REVOKE unsupported + scope terminator; stmt3 ordered-hit A-IE? | stmt3 premise re-derived `suppressed` → `[P]` gap; stmt1's finding unchanged; entry unaffected by the premise terminator | stmt1 Δ(1,1,1,1); stmt2 Δ(0,0,0,0); stmt3 Δ(1,0(skipped),1,1); total (2,1,2,2); L=1 (once, inside stmt1's window — attributed to the entry loader); Vd=1 |
| OP-3c | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001; REVOKE SELECT ON *.* FROM 'app'@'%'; DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;` | realchain | grants=`GRANT SELECT ON *.* TO 'app'@'%'` | stmt1 A-IE?; stmt3 ordered-hit A-IE? | both owe `[P]`; premise never consumable; no retro-modification | same vectors as OP-3b |
| OP-3d | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | grants=`GRANT SELECT ON *.* TO 'app'@'%'` | stmt1 A-IE?→produce; stmt2 ordered-hit A-plain | stmt2 is a plain drop on a hit → absent_refused; premise never consulted on A-plain | stmt1 Δ(1,1,1,1); stmt2 Δ(1,0(skipped),1,1); total (2,1,2,2); L=1; Vd=1 |

## OP-4 — conflicts, reads, error policy

| id | SQL | layer | overrides | arms | post-state asserts | reads Δ / L / Vd |
|---|---|---|---|---|---|---|
| OP-4a | `DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | deps answers mutate per call: stmt1 `[]`, stmt2 `[]`, stmt3 `[s.t.g@100001]` | stmt1 A-plain→produce; stmt2 ordered-hit A-plain; stmt3 ordered-hit → **S1** | THE per-hit-freshness discriminant: stmt3's deps-only read returns the new positive witness — S1 outranks the ordered absence; NO spatialTouching exists, `srsDepsStale` stays false, witness is consumable; entry survives | stmt1 Δ(1,1,1,1); stmt2 Δ(1,0(skipped),1,1); stmt3 Δ(1,0(skipped),1,1); total (3,1,3,3); L=1; Vd=1 |
| OP-4b | `DROP SPATIAL REFERENCE SYSTEM 100001; DROP TABLE s.t; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | deps answer at stmt3 = `[s.t.g@100001]` (positive but stale-demoted) | stmt1 A-plain→produce; stmt2 mints `srsDepsStale`; stmt3 ordered-hit A-plain | positive witness EXISTS at stmt3 but `srsDepsStale` demotes it → A arm stands via the ordered entry — distinguishes "witness demoted by staleness" from "no witness" | stmt1 Δ(1,1,1,1); stmt2 Δ(0,0,0,0) SRS-seam; stmt3 Δ(1,0(skipped),1,1); total (2,1,2,2); L=1; Vd=1 |
| OP-4c | `DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | deps query error injected at stmt2's deps-only call | stmt1 A-plain→produce; stmt2 = read error | error propagates; stmt2 never adjudicated/apply-minted; stmt1's bound snapshot and the `{100001}` entry unchanged | stmt1 Δ(1,1,1,1); stmt2 Δ(1,0(skipped),1,1) then error — Qo+deps attempted; totals asserted per actual attempt; L=1; Vd=1 |
| OP-4d | mechanism row — stmt2's vector must read `Qexist 0 (skipped)` | internal | stub counters | n/a | the skipped existence query is recorded as `skipped-by-ordered`, never as `executed`/`rows=0`; distinct from cache-hit (R0 + all-0) | stmt2 Δ(1,0(skipped),1,1); L=1 |

## OP-5 — invalidation

| id | SQL | layer | overrides | arms | post-state asserts | reads Δ / L / Vd |
|---|---|---|---|---|---|---|
| OP-5a | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001; CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 100001 DEFINITION 'x'; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | none | stmt1 A-IE✓→produce; stmt2 implementation boundary — NOT ordered-eligible (`Unsupported!=nil`), takes the existing full path: `srsExistenceStale` for 100001 is not yet minted at ITS bind → resolver → objectCache holds stmt1's full-lookup answer → `cache-hit`; boundary apply mints `srsExistenceStale{100001}` + entry `barrier=true`; stmt3 blocked-B | entry kept (`barrier=true`); stmt3 owes `[B]`; no A arm | stmt1 Δ(1,1,1,1); stmt2 Δ(0,0,0,0) + `cache-hit=1` (R0 — the full key was memoized by stmt1); stmt3 Δ(0,0,0,0); total R1,Qexist1,Qdeps1,Qo1 + 1 cache-hit; L=1 attributed to the entry loader; Vd=1 |
| OP-5b | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001; DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100002; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | zero rows both SRIDs | all A arms | 100001's entry untouched by 100002's publication; stmt3 ordered-hit A-plain | stmt1/2 Δ(1,1,1,1) each; stmt3 Δ(1,0(skipped),1,1); total (3,2,3,3); L=1; Vd=1 |
| OP-5c | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001; GARBAGE@#$; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | middle statement is a real parse failure (batch record, not an SRS boundary) | stmt1 A-IE✓→produce; failure sets `contaminated`; stmt3 blocked-B | whole-batch contamination shadows the channel: stmt3 exits at step 2 before the ordered lookup — entry kept, never consumed while contaminated | stmt1 Δ(1,1,1,1); stmt3 Δ(0,0,0,0); total (1,1,1,1); L=1; Vd=1 |
| OP-5d | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001; DROP TABLE s.t; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | table fixture bound; deps answer empty | stmt1 A-IE✓→produce; stmt3 ordered-hit **A-plain** (corrected vs R1 — tail is plain DROP) | `srsDepsStale` does not invalidate the existence entry; stmt3 consumes it as absent_refused | stmt1 Δ(1,1,1,1); stmt2 Δ(0,0,0,0); stmt3 Δ(1,0(skipped),1,1); total (2,1,2,2); L=1; Vd=1 |
| OP-5e | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001; ALTER TABLE s.t ENGINE=InnoDB; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | inert ALTER (engine-only, complete action list) | stmt1 A-IE✓→produce; stmt3 ordered-hit A-plain | inert op mints neither flag — clean consumption | as OP-5d |
| OP-5f | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001; REVOKE SELECT ON *.* FROM 'app'@'%'; DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | none | stmt1 A-IE✓→produce; stmt3 ordered-hit A-plain | REVOKE is premise-level only — never invalidates the fact-level entry; stmt3 re-derives own premise (suppressed) but plain drops never consult it | as OP-3b vectors |

## OP-6 — admittance & target boundary

| id | SQL | layer | overrides | arms | post-state asserts | reads Δ / L / Vd |
|---|---|---|---|---|---|---|
| OP-6a | `DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | none | A-plain→produce | baseline in-domain publish path | Δ(1,1,1,1); L=1; Vd=1 |
| OP-6b | `DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | below-floor fixture: NO pin; observed/resolved `8.0.3`; Qo `8.0.3` | version boundary → no produce | boundary row: no entry; `srsExistenceStale{100001}` minted at own-apply (accepted authored-stale rule); boundary binds via full path (ordered gate excludes it); reads issue | Δ(1,1,1,1); L=0 (no audited drop → loader gate not entered); Vd=1 |
| OP-6c | `DROP SPATIAL REFERENCE SYSTEM 100001;` | realchain | dialect=`tidb`; NO pin; recognized TiDB banner/product via real parse path | vendor boundary → no produce | no entry; no reads; boundary own-apply mints `srsExistenceStale{100001}` (accepted) | Δ(0,0,0,0); L=0; Vd=1 at public/realchain (real discovery); N/A when the internal layer supplies the resolved identity directly |
| OP-6d | `DROP SPATIAL REFERENCE SYSTEM 100001;` | public | providerless; target-version pin `8.0.36` | S2 → no produce | providerless never supplies certified absence; public projection unchanged | provider seams N/A — no provider attached (not measured-0); L=0; Vd=N/A (pin supplies the version) |
| OP-6e | `DROP SPATIAL REFERENCE SYSTEM 100001;` | public | malformed `target-version` pin (`8.0.x.x`) | typed preflight exit | request-level error before any statement processing; public projection N/A; no entry, no arm | all seams N/A (preflight exits before any dispatch); L=0; Vd=0 — discovery never reached |

## OP-7 — public projection

| id | SQL | layer | overrides | arms | post-state asserts | reads Δ / L / Vd |
|---|---|---|---|---|---|---|
| OP-7a | `DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;` | public | none | stmt1 A-plain; stmt2 ordered-hit A-plain | each statement carries exactly one `ddl.mysql.srs.drop.absent` blocker — compared on fixed rule/level/message/metadata; positional index/location asserted separately (omitempty preserved); **aggregate coverage=complete, verdict=reject** (declared whole-request row) | stmt1 Δ(1,1,1,1); stmt2 Δ(1,0(skipped),1,1); total (2,1,2,2); L=1; Vd=1 |
| OP-7b | OP-3b's three-statement batch | public | none | A-IE✓ + REVOKE-unsupported + A-IE? | stmt1: noop finding + `conditional_audit` marker; stmt3: `[P]` gap only; batch returns `ErrUnsupportedStatement`; per-statement asserts only (no aggregate row) | as OP-3b |
| OP-7c | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326; DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326;` | public | reserved SRID 4326 | stmt1 A-IE✓; stmt2 ordered-hit A-IE✓ | each statement: absent_noop notice finding + marker + reserved advisory `{srid}` — the reserved advisory co-fires here because the arm is not a refusal | as OP-3a |
| OP-7c′ | `DROP SPATIAL REFERENCE SYSTEM 4326; DROP SPATIAL REFERENCE SYSTEM 4326;` | public | reserved SRID 4326 | stmt1 A-plain; stmt2 ordered-hit A-plain | each statement: exactly the absent blocker — A-plain is a refusal arm and never carries the reserved advisory | as OP-7a |
| OP-7d | OP-7a's batch | public | policy `rules: {ddl.mysql.srs.drop.absent: {enabled: false}}` | both A-plain internally | zero absent-rule findings per statement; statements silent on that rule; entries STILL published (policy never gates publication); per-statement coverage complete; verdict `pass` (no enabled blocker or warning fires — the suppressed arm is internal-only) | reads unchanged |
| OP-7e | OP-7a's batch | public | policy `rules: {ddl.mysql.srs.drop.absent: {level: warning}}` | both A-plain | each statement: absent finding at warning; verdict review; publication unchanged | reads unchanged |

## OP-8 — snapshots & isolation (two frozen observation classes)

| id | SQL | layer | overrides | asserts | reads Δ / L / Vd |
|---|---|---|---|---|---|
| OP-8a | OP-3b's three-statement batch | internal+realchain | none | **(1) bound-state invariance**: stmt1's `SpatialReference`+`SRSDrop` metadata deep-copied post-bind, post-own-apply, post-REVOKE-apply, post-stmt3/final — all four identical (publication must not mutate the producer's own bound state). **(2) channel transition**: post-bind `srsOrdered` has no 100001 entry; post-own-apply exactly `{absent,arm=absent_noop,P=consumable,barrier=false}`; post-REVOKE and post-stmt3 the SAME earliest provenance (re-publication keeps the first producer). nil-vs-non-nil-empty distinguished per slice; bidirectional mutation isolation both directions; publication itself is NOT judged a snapshot mutation | stmt vectors as OP-3b; hook-measured deltas per statement; L=1; Vd per layer (realchain=1, internal=N/A) |
| OP-8b | OP-5a's three-statement batch | internal+realchain | none | entry-only compare: before boundary `{absent,barrier=false}`; after boundary `{absent,barrier=true}` — barrier flip is the ONLY entry change; the whole-channel compare additionally registers the boundary-minted `srsExistenceStale{100001}` key and unchanged `srsDepsStale`/`contaminated` | as OP-5a; hook-measured per statement |
| OP-8c | OP-5b's three-SRID batch | internal+realchain | non-empty fixture: SERVER `s1` bound entry, `s.t`/`s` schema table fixture, entries on 100001+100002 | single SRS own-apply window compare: the mint of `{100001}` touches only `srsOrdered` — `serverEntries`, table entries, `srsExistenceStale`, `srsDepsStale`, `contaminated`, unrelated-SRID `100002` entry all compared verbatim before/after that one apply | as OP-5b |
