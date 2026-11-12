# T26-SRS-DROP-ORDERED-PUBLICATION-CONTRACT-01 — R1 contract candidate

Docs-only contract round for **ordered publication of already-proven SRS
absence** — saving a supplier-certified absence fact to ordered batch
state and letting later statements of the same audited batch consume it
under a closed invalidation model.

This slice does **not** cover "a present object successfully dropped,
then published as absent". Successful-deletion-driven ordered absence,
ordered present, dependency completeness, SRS CREATE, ALTER INSTANCE,
and T27 stay deferred. **Nothing here is implemented** — every matrix
row and witness in this round is `PLAN` / `UNEXECUTED`, and a separate
execution work order is required after acceptance.

- Baseline: `27ab21f0ea44cbadc3bc7613aed5f5b582a604b6`
  (tree `2ffde084ed1dc6a408c33203a3c9c826580c8948`)
- Scope: 2 new docs files only; `.go changed = 0`

## 1. Terms

- **catalog-certified absence** — `Read && !Exists && Absence` on a bound
  `SpatialReferenceSystemState`, produced only by the supplier's clean
  zero-observed-row existence read under the registered completeness
  proof (in-domain MySQL 8.0.4+), bound to the statement's authored
  SRID (`ValidAbsence`).
- **ordered absence entry** — a new `batchState`-owned record, one per
  authored SRID, minted by an A arm's own-apply. It is *provenance*, not
  a second catalog: it records that a certified absence was established
  by a specific statement at a specific position.
- **ordered hit** — a later bind consuming the entry as its existence
  answer. The entry substitutes **only** the existence answer; it never
  synthesizes `Read`/`Absence`, never fabricates a dependency witness,
  and never suppresses a still-owed read.
- **unknown / unread / stale / blocked** — unchanged vocabulary:
  `Read=false` (+`UnreadReason`) is an unissued read;
  `MetadataUnreadBatchEffects` marks a blocked bind; the two stale
  channels (`srsExistenceStale` per SRID, `srsDepsStale` batch-wide)
  keep their accepted semantics.

## 2. OP-A — production eligibility (which arms publish)

Publication happens at own-apply, driven by the shared adjudication —
the single-truth rule is preserved.

| arm | publishes? | keeps existing entry? | own-P recorded | stale channels |
|---|---|---|---|---|
| B (blocked) | no | yes, untouched | n/a (never reached premise) | entry/state unchanged |
| S0 | no | yes | projection carried, unused | unchanged |
| S1 (positive dep witness) | no | yes | carried | unchanged |
| A-plain (absent_refused) | **yes** | entry minted/kept | `unproven`-equivalent provenance, never consumed | unchanged |
| A-IE✓ (absent_noop) | **yes** | minted/kept | `consumable`, consumed | unchanged |
| A-IE? (absent_conditional) | **yes** | minted/kept | `unproven`/`suppressed`, owed `[P]` | unchanged |
| S2 (unproven) | no | yes | as projected | mints `srsExistenceStale[srid]` (existing rule — unchanged); the stale flag then shadows the SRID's entry for every later statement |
| boundary (any non-audited SRS write) | no | sets `barrier` on the SRID's entry | n/a | mints `srsExistenceStale[srid]` (existing) |

Rulings:

1. All three A arms publish — the catalog-certified absence fact is
   real regardless of which A arm classified the drop. A-plain being a
   blocker does not disqualify the fact; A-IE?'s owed premise does not
   condition the fact (OP-B separates fact conditioning from own-P).
2. S1 does **not** publish. A simultaneous positive dependency witness
   is not a de-proof of the absence fact, but it is a statement-level
   witness that a dependent is *visible*; publishing absence under S1
   could mask that witness for later statements. OP-1e asserts
   no-produce.
3. B/S0/boundary publish nothing — they never consumed a certified
   absence (B's read never issued).
4. Statement outcome never substitutes for post-state proof: an entry
   exists only because an A arm carried a certified fact, never because
   a projection "looked like" a drop.
5. Re-publication is idempotent: a second A arm on the same SRID keeps
   the first entry (earliest producer wins — provenance records the
   first certified statement; a later hit already consumed it).

## 3. OP-B — carrier, identity, conditioning

**Entry** (internal, `batchState`-owned, keyed by authored `uint32` SRID):

```
srsOrderedEntry {
    srid          uint32            // authored identity — binding key
    absent        bool              // the certified fact (only true value carried)
    sourceArm     SRSDropArm        // which A arm published (provenance)
    ownPrivilege  SRSDropPrivilege  // publishing statement's own premise state (provenance)
    publishedIdx  int               // statement index of the producer
    barrier       bool              // shadowed by a same-SRID boundary/write
}
```

**Bound-state marker**: `SpatialReferenceSystemState.OrderedAbsent bool`
— set on a hit, parallel to `ServerState.OrderedAbsent`. It is **not**
`Read`/`Absence`: catalog fields stay zero on an ordered-served bind,
and every consumer must read `OrderedAbsent` explicitly. `ValidAbsence`
is unchanged — it is the *catalog* predicate. The A-slot predicate in
the arm table becomes `ValidAbsence(state, srid) || OrderedAbsentConsumed(state, srid)`
where `OrderedAbsentConsumed` = `OrderedAbsent && !barrier && same authored SRID`
(the SRID equality is structural — entries are keyed by it).

**Identity**: keyed on authored `uint32` SRID only. A resolver state
whose `Absence` is for a foreign SRID, a contradictory field set
(`Exists && Absence`), or an unread/missing source is never promoted
(OP-1i/1j).

**Conditioning separation** — three independent facts:

1. the absence fact itself — unconditional for all three A arms;
2. the publishing statement's own premise consumption — recorded as
   `ownPrivilege` provenance, never inherited by successors (each later
   statement re-derives its own premise from the shared witness +
   scope position);
3. the public `conditional_audit` marker — statement-level only, never
   attached to the entry.

No conditionalized/revision state is pre-built for the deferred
"successful drop then publish absence" scope.

## 4. OP-C — bind order, S1 conflict, read & error policy

Frozen first-match arm order stands: **B → S0 → S1 → A → S2**.

Ordered bind path (inside `bindMySQLSRS`, before the resolver seam —
mirroring the accepted SERVER six-step position):

1. non-MySQL dialect → unvouched `bound` (unchanged);
2. `s.contaminated` or `srsExistenceStale[srid]` → `MetadataUnreadBatchEffects` (unchanged);
3. **ordered lookup**: `srsOrdered[srid]` exists, `absent && !barrier`
   → bind `SpatialReferenceSystemState{SRID, OrderedAbsent: true}` and
   continue at step 4 with the existence answer consumed; `barrier`
   entries fall through to step 4 as if no entry existed — the stale
   flag already handles the block (defense-in-depth, never a
   resurrecting path);
4. resolver seam issues **deps-only** lookup (`ObjectLookupRequest`
   gains a `fields` qualifier — implementation touchpoint) → the
   dependency witness is still read fresh and errors still propagate;
5. no ordered hit → full resolver call, unchanged.

**Critical ruling — deps are never skipped on a hit.** `ResolveObject`
today answers existence + dependencies in one call. An ordered hit that
skipped the whole call would (a) hide a positive dependency witness
that appeared mid-batch and (b) swallow a still-reachable provider
error. So the contract registers one implementation touchpoint: the
lookup gains a deps-only form (`fields=dependencies` qualifier on
`ObjectLookupRequest`, honored by the MySQL-SRS provider — Qdeps + Qo
still fire, Qexist skipped). If that seam is not available at
implementation time, the ordered entry degrades to provenance-only and
the hit path issues the full lookup (read savings lost, safety kept) —
the contract does **not** authorize laundering a skipped read into a
"executed, zero rows" record (OP-4d).

Read policy on a hit:

- R: still issued (deps-only lookup) — Δ1
- Qexist: skipped-by-ordered — Δ0, registered as `skipped`, not `0`
- Qdeps: issued fresh — Δ1
- Qo: issued with the deps call — Δ1
- vector on a hit row: `Δ = R1, Qexist 0(skip), Qdeps 1, Qo 1` — never
  rewritten as a successful existence read
- errors on still-issued reads propagate unchanged (OP-4c)

## 5. OP-D — lifecycle, invalidation, cache separation

- **Owner/lifetime**: `batchState`, request-scoped, same batch, same
  target identity + version admittance. New `batchState` inherits
  nothing (OP-2e). Not a cross-request cache; no concurrency claims.
- **vs objectCache**: `objectCache` is rebuilt per enrich; the ordered
  channel lives on `batchState` and survives (OP-2d registers this as
  the intentional extension over PA-6d's re-read-per-enrich baseline —
  registered as a model decision, not a baseline defect). Two distinct
  mechanisms with distinct vectors: cache-hit = Δ0 on all reads;
  ordered-hit = Qexist skipped, deps+Qo still fire.
- **Closed invalidation table**:

| event | effect on the SRID's entry |
|---|---|
| same-SRID SRS boundary/write or S2 stale mint | `barrier=true` + existing `srsExistenceStale[srid]` blocks independently |
| different SRID anything | none |
| whole-batch contamination | channel shadowed — binds exit at step 2 before the ordered lookup; entries kept, never consumed while contaminated |
| `srsDepsStale` (spatialTouching) | none — deps staleness demotes witness consumption at adjudication only |
| inert ops (incl. engine-only ALTER) | none |
| REVOKE / privilege scope terminator | none — premise-level event; successors re-derive own P; entry is fact-level |
| new batchState / new request | entries don't exist (no inheritance) |

  Publication never clears stale flags, never clears `contaminated`,
  never resurrects a barrier'd entry. Invalidation model decided
  uniquely: **keep the entry + flip `barrier`** (SERVER precedent) —
  never delete; a deleted entry would let the fall-through path re-read
  the catalog, while a barrier'd entry + stale flag guarantees the
  blocked arm and preserves provenance.

## 6. OP-E — public projection & policy

- All RuleIDs, outcomes, messages, levels, gap owners/reasons,
  `required_facts` order, reserved advisory, and `conditional_audit`
  marker rules stay frozen. A hit row is publicly indistinguishable
  from a catalog-served A arm (OP-7a asserts byte-shape equality).
- Ordered provenance stays internal. No new public field, rule, or
  marker is proposed — none is needed; if a later reviewer judges the
  provenance worth surfacing, it is a separate contract decision.
- Marker semantics: `conditional_audit` present on A-IE✓ findings only;
  absent ≠ null/empty-string semantics preserved.
- Policy disabled/level-rewrite touches projections only — publication
  and premise truth unaffected (OP-7d/7e).
- Providerless CLI three-probe stays regression-only — it can never
  produce ordered publication evidence and no new probe is planned.
- Coverage: per-statement coverage complete on hit rows; aggregate
  coverage/verdict asserted only on declared whole-request rows
  (OP-7a).

## 7. Source facts vs model deductions vs assumptions

**Source facts (baseline `27ab21f0`, excerpted in the evidence pack)**:
`SpatialReferenceSystemState` fields + `ValidAbsence`; arm table +
`AdjudicateSRSDrop`; `applySRSStatement` (Unproven-only stale mint);
`bindMySQLSRS` body + stale ordering; `objectCache` construction +
per-request wiring in `metadata.go`; privilege loader gate + REVOKE
scope terminator; SRS boundary stale-mint at `batch_state.go:493`;
`spatialTouching`/`srsDepsStale`; SERVER ordered-channel precedent
(`batchServerEntry` fields, six-step order, barrier semantics); rule
projection seams in `mysql_srs_rules.go`.

**Model deductions** (contract-level, derivable from the above):
A arms are the only producers; deps read must survive a hit to keep
S1-outranks-A and error propagation; `barrier`+stale-flag dual shadow
is the minimal safe invalidation; fact/premise/marker are separable
because the code already carries them in three different slots.

**Unverified assumptions** (must be re-checked at implementation):
the `fields=dependencies` qualifier is the minimal seam change (the
degraded full-lookup fallback is registered); `OrderedAbsent` naming
mirrors `ServerState` conventions; per-hit Qo is required by the
provider's support gate rather than cacheable (kept issued — safe
side).

## 8. Implementation touchpoints (registered, not authorized)

1. `batchState` + `srsOrdered map[uint32]*srsOrderedEntry`; own-apply
   mint on A arms; barrier flip on same-SRID boundary/write.
2. `SpatialReferenceSystemState.OrderedAbsent` marker field.
3. `bindMySQLSRS` ordered lookup step 3 + deps-only fall-through.
4. `ObjectLookupRequest.fields` qualifier + provider deps-only path
   (or full-lookup fallback).
5. A-slot predicate `ValidAbsence || OrderedAbsentConsumed`.
6. Read-ledger annotation for `skipped-by-ordered` (Qexist).
7. OP-1..OP-8 witness tests across internal/real-chain/public/provider
   layers (all `PLAN` here).

## 9. Status

- **CONTRACT READY: YES** — one recommended contract per OP-A..E;
  every design decision uniquely resolved (barrier-not-delete;
  deps-never-skipped with a registered seam touchpoint + safe fallback;
  three A arms all publish; fact/premise/marker separation closed).
- **IMPLEMENTED/VERIFIED: NO** — `.go changed = 0`; every row is PLAN.
- **EXECUTION AUTHORIZED: NO** — separate work order required.
- T26/#105 stays OPEN; no push/merge/tag/release.
