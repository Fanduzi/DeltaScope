# T26-SRS-DROP-ORDERED-PUBLICATION-CONTRACT-01 — discriminant matrix (R1)

Every row is a **PLAN / UNEXECUTED** discriminant — nothing in this file
has been implemented or run. Read seam vocabulary: R = resolver call,
Qexist = SRS existence query, Qdeps = ST_GEOMETRY_COLUMNS query,
Qo = `select version()`; **R/Qexist/Qdeps/Qo counted per statement**,
**L counted per enrich** (one-time privilege observation), **Vd counted
per request** (discovery). Delta and total are registered separately —
never substituted. A custom resolver that issues no SQL marks the SQL
seams `N/A` — never a measured 0. `unobserved` marks a dimension the
planned assertion does not measure, with the reason stated.

Fixed fixture unless a row says otherwise: dialect `mysql`,
target-version pin `8.0.36` (in-domain), provider = real
`mysqlmeta.NewProvider` over the recorded stub driver,
`account="app@%"`, `grants=["GRANT SELECT, SUPER ON *.* TO `app`@`%`"]`,
privilege mode `observed-account-new-session`, zero-row SRS cell
(clean zero-observed-row existence answer under the in-domain Qo ⇒
supplier-certified absence), no deps rows, no parse failures.

## OP-1 — producer arms (which arms may publish an ordered absence entry)

| id | SQL | layer | fixture difference | producer arm | planned asserts | reads Δ/total |
|---|---|---|---|---|---|---|
| OP-1a | `DROP SPATIAL REFERENCE SYSTEM 100001;` | R | default | A-plain (absent_refused) | entry `{100001: absent, sourceArm=absent_refused}` after own-apply; no stale mint; no contamination | Δ1,1,1,1,0 / total 1,1,1,1,0 (per-enrich L=1 separately: LΔ=1, L total=1; Vd=1 request) |
| OP-1b | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;` | R | default (SUPER grant) | A-IE✓ (absent_noop) | entry `{100001: absent, sourceArm=absent_noop}`; own P recorded `consumable`; no stale mint | Δ1,1,1,1,0 / 1,1,1,1,0; L=1; Vd=1 |
| OP-1c | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;` | R | `grants=[SELECT only]` | A-IE? (absent_conditional) | entry `{100001: absent, sourceArm=absent_conditional}`; own P recorded `unproven` (owed `[P]`); absence fact unconditional | Δ1,1,1,1,0 / 1,1,1,1,0; L=1; Vd=1 |
| OP-1d | `DROP SPATIAL REFERENCE SYSTEM 0;` | R | SRID 0 | S0 | NO entry; no stale mint; S0 blocker | Δ0,0,0,0,0 / 0,0,0,0,0 — S0 exits before the read pass; L=1 (premise still loads at enrich scope); Vd=1 |
| OP-1e | `DROP SPATIAL REFERENCE SYSTEM 100001;` | R | `deps=[s.t.g@100001]` | S1 | NO absence entry (positive witness present — the read proved presence of dependents; publishing absence under a simultaneous positive witness is deferred: the witness may be *visible-but-incomplete*, so the contract publishes ONLY when the arm is an A arm — see OP-C ruling); asserted as NO-PRODUCE | Δ1,1,1,1,0 / 1,1,1,1,0; L=1; Vd=1 |
| OP-1f | `DROP TABLE s.t; DROP SPATIAL REFERENCE SYSTEM 100001;` | R | table fixture `s.t` exists w/ geometry col | B (batch-effects) | NO entry for 100001; `srsDepsStale` minted by stmt1; `srsExistenceStale` for 100001 — the DROP TABLE does NOT mint it (non-SRS path); blocked arm preserves entry-state (none existed) | stmt1 Δ=measured-table-read; stmt2 Δ0,0,0,0,0 / totals per pair; L=1; Vd=1 |
| OP-1g | `DROP SPATIAL REFERENCE SYSTEM 100001;` | R | `version="unparseable"` Qo | S2 | NO entry — absence never proven (clean zero-row but outside D-proof); S2 mints `srsExistenceStale[100001]` (existing product rule, unchanged) | Δ1,1,1,1,0 / 1,1,1,1,0; L=1; Vd=1 |
| OP-1h | `DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;` (preceded by `CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 100001 DEFINITION 'x';`) | R | boundary prefix | stmt1 = implementation boundary; stmt2 = B | NO entry; boundary mints `srsExistenceStale[100001]`; tail B owes `[B]` | prefix Δ1,1,1,1,0, tail Δ0,0,0,0,0 / 1,1,1,1,0; L=1 (fired during prefix enrichment — existing behavior); Vd=1 |
| OP-1i | synthetic: resolver returns `Absence=true` for SRID 100002 while authored SRID is 100001 | I | custom resolver | foreign-SRID input | NO entry for 100001 or 100002 — `ValidAbsence` binds absence to the authored SRID; foreign absence is never promoted; S2 arm | SQL seams N/A (custom resolver); L=1 |
| OP-1j | synthetic: `Absence=true` but `Exists=true` (contradictory) | I | custom resolver | invalid input | NO entry — `ValidAbsence` requires `!Exists`; contradictory field never promoted; S2 arm | SQL seams N/A; L=1 |

## OP-2 — hit & lifecycle (when a later statement consumes the entry)

| id | SQL | layer | fixture difference | arms | planned asserts | reads Δ/total |
|---|---|---|---|---|---|---|
| OP-2a | `DROP SRS 100001; DROP SRS 100001;` (two plain drops, same SRID) | R | default | stmt1 A-plain→produce; stmt2 ordered-hit A-plain | stmt2 arm=absent_refused produced from the ordered entry; ordered hit substitutes ONLY the existence answer — deps read STILL issued (OP-C ruling); identical public blocker shape | stmt1 Δ1,1,1,1,0; stmt2 Δ1,0,1,1,0 (R still issued for deps-only lookup; Qexist skipped; Qdeps + Qo still issued) / total 2,1,2,2,0; L=1; Vd=1 |
| OP-2b | `DROP SRS 100001; DROP SRS 100002;` (different SRID) | R | `srs` empty for both | two A-plain | entries `{100001,100002}` independent; stmt2 misses 100001's entry → full own read | Δ1,1,1,1,0 + Δ1,1,1,1,0 / 2,2,2,2,0; L=1; Vd=1 |
| OP-2c | same-enrich cache control: `DROP SRS 100001; DROP SRS 100001;` vs baseline PA-6a cache-hit shape | R | default | A-plain ×2 | distinguishes ordered-hit vs objectCache-hit: under PA-6a a cache hit yields Δ0; under this contract an ordered hit yields Qexist-skip but Qdeps+Qo still fire — the two mechanisms' vectors differ and are asserted separately | see OP-2a; contract registers the R/Qexist distinction explicitly |
| OP-2d | two enrich passes over one batchState; provider answer CHANGED between passes (pass 2: `srs={100001}` present row) | I | shared batchState, `objectCache` rebuilt per enrich | pass1 A-plain; pass2 consumed | **intentional-extension registration**: the ordered entry survives objectCache rebuild (it lives on batchState); pass 2 consumes the ordered absence — the fresh provider "present" answer is NOT re-read; this diverges from PA-6d baseline (re-read per enrich) and is registered as the model decision, not a fix | pass1 Δ1,1,1,1,0; pass2 Δ1,0,1,1,0 / totals separately; L per-enrich=1 each; Vd=1 (Vd is request-level — same request) |
| OP-2e | `newBatchState` between the two drops (independent enrich) | I | fresh state | pass1 A-plain; pass2 full read | new batchState inherits NOTHING — second enrich reads provider again | both Δ1,1,1,1,0; L=1 each; Vd per request |
| OP-2f | ordered-hit + a stale-minted SRID (`DROP TABLE s.t` between the two drops) | R | table fixture | stmt1 A-plain; stmt2 = the DROP TABLE mints `srsDepsStale`; the SRS DROP's ordered hit still stands (existence entry valid) BUT deps witness consumption is demoted | stmt2 = A-plain via ordered entry (existence); depsStale only demotes the witness at adjudication — never the certified absence | stmt2 Δ1,0,1,1,0 / totals; L=1; Vd=1 |

## OP-3 — own privilege premise & source independence

| id | SQL | layer | fixture difference | arms | planned asserts | reads Δ/total |
|---|---|---|---|---|---|---|
| OP-3a | `DROP SRS IF EXISTS 100001; DROP SRS IF EXISTS 100001;` | R | default | stmt1 A-IE✓→produce; stmt2 ordered-hit A-IE✓ | stmt2 re-adjudicates ITS OWN premise from the shared witness (still consumable, same scope) → absent_noop; marker on stmt2's finding only | stmt1 Δ1,1,1,1,0; stmt2 Δ1,0,1,1,0 / 2,1,2,2,0; L=1; Vd=1 |
| OP-3b | `DROP SRS IF EXISTS 100001; REVOKE SELECT ON *.* FROM 'app'@'%'; DROP SRS IF EXISTS 100001;` | R | real REVOKE terminator | stmt1 A-IE✓; stmt3 ordered-hit A-IE? | stmt3's premise re-derived as `suppressed` (scope ended after stmt1) → absent_conditional, `[P]` gap on stmt3; stmt1's published projection unchanged; the ordered entry itself unaffected (premise terminator never invalidates fact-level absence) | stmt1 Δ1,1,1,1,0; REVOKE Δ0; stmt3 Δ1,0,1,1,0 / 2,1,2,2,0; L=1 (one-time load inside stmt1's window); Vd=1 |
| OP-3c | same as 3b but grants=`[SELECT only]` | R | unproven premise both stmts | stmt1 A-IE?; stmt3 A-IE? | both `[P]`; no retro-modification | as 3b; L=1; Vd=1 |
| OP-3d | prefix publishes; successor is `DROP SRS 100001` plain under unproven grants | R | grants SELECT | stmt1 A-IE?; stmt2 ordered-hit A-plain | stmt2's plain DROP on ordered hit → absent_refused (premise never consulted on A-plain) | stmt1 Δ1,1,1,1,0; stmt2 Δ1,0,1,1,0 / 2,1,2,2,0; L=1; Vd=1 |

## OP-4 — conflicts, reads, and error policy

| id | SQL | layer | fixture difference | arms | planned asserts | reads Δ/total |
|---|---|---|---|---|---|---|
| OP-4a | `DROP SRS 100001; DROP TABLE s.t; DROP SRS 100001;` with `deps=[s.t.g@100001]` AFTER stmt1 published | R | deps row present | stmt1 A-plain; stmt3 S1 | ordered hit on existence STILL runs the deps read → the fresh positive witness refuses (S1 outranks ordered absence) — OP-C's core safety: a visible dependent appearing mid-batch blocks re-drop | stmt1 Δ1,1,1,1,0; stmt3 Δ1,0,1,1,0 / totals; L=1; Vd=1 |
| OP-4b | same batch, `deps` empty at stmt3 (witness read returns zero rows under depsStale) | R | depsStale minted by stmt2 | stmt1 A-plain; stmt3 A-plain | ordered absence consumed; depsStale demotes the (empty) witness → A arm stands | as 4a |
| OP-4c | provider `failOn={"deps": err}` at stmt3 | R | injected deps query error | stmt1 A-plain; stmt3 = read error propagates | the ordered hit does NOT swallow a still-issued read's error — `ResolveObject`/`deps` failure propagates; no synthetic empty deps | stmt3 Δ1,0,1,1,0 then error; totals partial — asserted, not inferred |
| OP-4d | ordered-hit path must not register the skipped Qexist as "executed, zero rows" | I | stub counters | n/a (mechanism row) | read ledger records Qexist skipped-by-ordered, not `executed=1,rows=0` — skipped reads are never laundered into successful reads | Δ1,0,1,1,0 with explicit `skipped` annotation |

## OP-5 — invalidation (what shadows a published entry)

| id | SQL | layer | fixture difference | arms | planned asserts | reads Δ/total |
|---|---|---|---|---|---|---|
| OP-5a | `DROP SRS IF EXISTS 100001; CREATE OR REPLACE SRS 100001 DEFINITION 'x'; DROP SRS 100001;` | R | boundary mid | stmt1 A-IE✓→produce; stmt2 boundary mints `srsExistenceStale{100001}` AND sets the entry's `barrier`; stmt3 blocked-B | stmt3 arm=blocked, `[B]` gap — the barrier + stale flag shadow the ordered absence; entry kept (never deleted) with `barrier=true`; NO A arm | stmt1 Δ1,1,1,1,0; stmt2 Δ1,1,1,1,0 (boundary reads its own pass); stmt3 Δ0 / totals; L=1; Vd=1 |
| OP-5b | `DROP SRS IF EXISTS 100001; DROP SRS IF EXISTS 100002; DROP SRS 100001;` (different-SRID control) | R | both SRIDs absent | all A arms | 100001's entry untouched by 100002's publication; stmt3 ordered-hit A-plain | stmt3 Δ1,0,1,1,0 / totals; L=1; Vd=1 |
| OP-5c | `DROP SRS IF EXISTS 100001; GARBAGE;` (real parse failure inside the batch) | R | parse failure mid | stmt1 produces; stmt2 boundary/contaminated | whole-batch `contaminated` ⇒ channel taint → the ordered channel is shadowed batch-wide; any later SRS statement on ANY SRID takes the blocked path, never the ordered hit | post-contamination Δ0 on every SRS bind; L=1; Vd=1 |
| OP-5d | `DROP SRS IF EXISTS 100001; DROP TABLE s.t; DROP SRS 100001;` (spatialTouching mid, deps empty) | R | table exists | stmt1 A-IE✓; stmt3 ordered-hit consumed with depsStale demotion | `srsDepsStale` does NOT invalidate the existence entry (deps channel ≠ existence channel) — stmt3 stays A-IE✓ | stmt3 Δ1,0,1,1,0 / totals |
| OP-5e | `DROP SRS IF EXISTS 100001; ALTER TABLE s.t ENGINE=InnoDB; DROP SRS 100001;` (inert ALTER control) | R | inert ALTER | all ordered hits | inert op mints neither stale flag — ordered hit consumed cleanly | stmt3 Δ1,0,1,1,0 / totals |
| OP-5f | `DROP SRS IF EXISTS 100001; REVOKE ...; DROP SRS 100001;` | R | REVOKE terminator | stmt1 A-IE✓; stmt3 A-plain | REVOKE does NOT invalidate the ordered absence (fact-level); stmt3's own premise re-derived → plain DROP is still absent_refused | as 3b |

## OP-6 — admittance & target boundary

| id | SQL | layer | fixture difference | arms | planned asserts | reads Δ/total |
|---|---|---|---|---|---|---|
| OP-6a | `DROP SRS 100001;` | R | default | A-plain→produce | in-domain publish path | Δ1,1,1,1,0 / 1,1,1,1,0; L=1; Vd=1 |
| OP-6b | `DROP SRS 100001;` | R | Qo `8.0.3` (proven, supported, outside D-proof) | version boundary | NO entry (not an A arm — boundary rows never publish); `srsExistenceStale` NOT minted by a pure version boundary? — registered: boundary own-apply mints the authored stale key under the SAME rule as PA-6e | Δ1,1,1,1,0 / totals; L=0; Vd=1 |
| OP-6c | `DROP SRS 100001;` | R | `dialect=tidb` | vendor boundary | NO entry; no reads | Δ0,0,0,0,0; L=0; Vd=0 |
| OP-6d | `DROP SRS 100001;` | X | providerless + `target-version 8.0.36` | S2 | NO entry — providerless never supplies certified absence; CLI/public projection unchanged | provider N/A — no provider attached (not measured-0); L=0; Vd=0 |

## OP-7 — public projection

| id | SQL | layer | fixture difference | arms | planned asserts | reads Δ/total |
|---|---|---|---|---|---|---|
| OP-7a | OP-2a's two-statement batch | X | default | A-plain ×2 (stmt2 via ordered hit) | stmt1 + stmt2 findings byte-identical in shape: exactly one `ddl.mysql.srs.drop.absent` blocker each, same message/metadata keys, `srid` typed uint32, marker key ABSENT (not null) on both | stmt vectors per OP-2a; aggregate assert declared whole-request: complete coverage? NO — each statement's coverage complete; verdict reject |
| OP-7b | OP-3b's REVOKE batch | X | default | A-IE✓ + (REVOKE unsupported) + A-IE? | stmt1 noop finding + marker; stmt3 `[P]` gap and nothing else; `ErrUnsupportedStatement` for the batch | per-statement assertions only; no aggregate claim |
| OP-7c | OP-2a batch with `4326` (reserved SRID) | X | reserved | A-plain ×2 + reserved advisory per statement | each statement: absent blocker + reserved advisory `{srid}`-only; marker absent on refusal arms | as 7a |
| OP-7d | OP-2a batch with policy disabling `ddl.mysql.srs.drop.absent` | X | policy | both still A-plain internally | silent statements; entries STILL published (policy never gates fact publication) | reads unaffected |
| OP-7e | OP-2a batch with `level: warning` | X | policy | A-plain ×2 | findings at warning; review verdict; publication unchanged | reads unaffected |

## OP-8 — snapshots & isolation

| id | SQL | layer | fixture difference | planned asserts | reads Δ/total |
|---|---|---|---|---|---|
| OP-8a | OP-3b's three-statement batch | I+R | default | stmt1's full state (SpatialReference + SRSDrop + entry view) deep-copied at four points: post-bind, post-own-apply, post-REVOKE-apply, post-stmt3/final — all identical; entry snapshot carries `absent/sourceArm/ownPrivilege/barrier` verbatim; nil vs non-nil empty distinguished on every slice member; bidirectional mutation isolation both directions | hook-measured per-statement Δ; L=1; Vd=1 |
| OP-8b | OP-5a's boundary batch | I+R | boundary mid | entry before boundary `{absent, barrier=false}`; after boundary `{absent, barrier=true}` — deep-value compare proves the barrier flip is the ONLY change; `srsExistenceStale{100001}` and `srsDepsStale` unchanged by publication | per-statement Δ measured at hooks |
| OP-8c | SERVER + table/schema state boundary | I+R | mixed batch | publication touches ONLY the SRS ordered channel: `serverEntries`, table entries, `srsExistenceStale`, `srsDepsStale`, `contaminated` all compared verbatim before/after | per-statement Δ |
