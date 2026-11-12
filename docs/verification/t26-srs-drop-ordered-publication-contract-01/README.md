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

Unified publication/keep/invalidation table (R3-A2). `ownPrivilege`
records the producer's **actual** premise projection — never rewritten
as "unused/unconsumed" (R3-A1):

| situation | action on the SRID's entry | own-P recorded | stale channels |
|---|---|---|---|
| A arm + catalog `ValidAbsence` + no existing entry | **mint** `{absent,sourceArm,ownPrivilege,publishedIdx,barrier:false}` | actual premise | unchanged |
| A arm + catalog `ValidAbsence` + existing active entry | keep the earliest producer's full provenance (earliest wins) | entry unchanged | unchanged |
| A arm reached via `OrderedAbsentConsumed` (hit) | consume the existing entry — no re-mint, no fabricated catalog `Read`/`Absence`, no new source attribution | entry unchanged | unchanged |
| B / S0 / S1 | no entry created | actual projected premise (never consumed by these arms) | unchanged |
| S2 (unproven) | no absence published; an existing same-SRID entry gets `barrier=true` | as projected | mints `srsExistenceStale[srid]` (existing rule) + flips entry barrier |
| boundary (any non-audited SRS write) | no absence published; an existing same-SRID entry gets `barrier=true` | n/a | mints `srsExistenceStale[srid]` (existing) + flips entry barrier |

Premise projections per arm (exact):

- B/S0/S1 carry their actual projected premise; these arms never
  consume it.
- A-plain carries the actual premise (consumable, unproven, or
  suppressed) — it never consumes it and never gains a
  `conditional_audit` marker from a positive witness. Under the
  default SUPER fixture OP-1a therefore records
  `ownPrivilege=consumable, consumed=false`.
- A-IE✓: premise consumable → consumed; the marker belongs to that
  statement.
- A-IE?: premise unproven/suppressed → owed `[P]`.

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
5. Re-publication keeps the earliest producer: `publishedIdx` records
   the statement index **within the producing enrich call** — it is
   not a cross-enrich global sequence; a later enrich's index restarts
   and must not re-key provenance (R3-A5).

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
— the single definition of `OrderedAbsentConsumed` lives in §4.5.

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

### 4.1 Ordered-consumer eligibility (R2-A1)

`bindMySQLSRS` is invoked for every MySQL SRS declaration — audited
drops AND boundary writes — before the caller checks `Kind`/`
Unsupported` and attaches adjudication. Being inside `bindMySQLSRS`
does not mean serving audited drops only, so ordered consumption is
gated by an explicit, codable predicate evaluated inside the bind:

```
orderedSRSDropEligible(statement, s):
    s.dialect == spec.DialectMySQL
    && decl.Kind == spec.SRSKindDrop
    && statement.Unsupported == nil
    && MySQLSRSDropAuditedDomain(s.statementVersion())
```

A statement failing any conjunct never reaches the ordered lookup — it
takes the pre-existing resolver path unchanged (boundary binds and
their own-apply stale mint keep the accepted semantics). No new
SRID-0 or boundary pre-read short-circuit is introduced: SRID 0 still
exits at the S0 slot inside adjudication, and boundary statements
still bind through the full resolver path.

### 4.2 Ordered bind path (determinate)

1. non-MySQL dialect → unvouched `bound` (unchanged);
2. `s.contaminated` or `srsExistenceStale[srid]` →
   `MetadataUnreadBatchEffects` (unchanged);
3. `orderedSRSDropEligible` false → step 5 (unchanged full path);
4. **ordered lookup**: `srsOrdered[srid]` exists, `absent && !barrier`
   → certify the active entry, bind
   `SpatialReferenceSystemState{SRID: authored, OrderedAbsent: true}`,
   and issue the **deps-only** read (§4.3); a `barrier` entry is
   treated as no entry → step 5 (the stale flag already guarantees
   the blocked arm — defense in depth, never a resurrecting path);
5. no active entry → full resolver call, unchanged.

### 4.3 Deps-only read — bypasses objectCache (R2-A2, unique scheme)

`ResolveObject` today answers existence + dependencies in one call,
and `objectCache` memoizes per `(schema,type,name,qualifiers)`. The
contract's requirement is stronger: **every ordered-consuming
statement must see the dependency witness fresh**, because a positive
witness appearing mid-batch still outranks the ordered absence (OP-4a
discriminant: publish → ordered hit → *same SRID* ordered hit whose
deps answer turned positive or errored).

Unique scheme (no implementation-time fallback):

- The hit path dispatches a deps-only request **directly to the
  provider** — never through `cachedObjectResolver`. objectCache is
  not consulted and not written for deps-only requests.
- Request form: `ObjectLookupRequest{Type:"mysql_spatial_reference_system",
  Name: <authored SRID decimal>, Qualifiers: {srs_facts:"dependencies"}}`
  — new qualifier constant `ObjectLookupQualifierSRSFields` /
  value `dependencies` (implementation seam).
- **Capability contract (executable determination, R3-B1)**: the
  object checked is `state.provider` — the wired provider itself,
  not the request-level resolver adapter. The minimal contract is one
  optional interface:

  ```
  type MySQLSRSDependencyReader interface {
      // Runs the support gate (Qo) + the ST_GEOMETRY_COLUMNS witness
      // read for srid. Never issues the existence query. Returns the
      // fresh witness fact; read errors propagate.
      ResolveMySQLSRSDependencies(ctx, srid uint32) (*spec.SRSDependencyFact, error)
  }
  ```

  The enrich wiring hands the bind an uncached forwarder function
  alongside `mysqlSRSResolver`: `srsDepsReader` — set only when
  `state.provider` satisfies the interface. The bind checks
  `s.srsDepsReader != nil`: non-nil → ordered consumption available;
  nil → §4.4's no-capability row (entry not consumed, existing full
  path). A plain `ResolveObject` implementation does NOT imply the
  capability; the wrapper/adapter is never probed — only the provider.
- Qo is still issued inside the deps call (the provider's support
  gate is per dispatch); an unparseable Qo does not zero the deps
  query — the accepted full-path direction (reads still run, only
  `Absence` supply stops) is preserved.

### 4.4 Result/merge table (R2-A3 + R3-B)

The ordered branch is committed FIRST (capability + active entry both
checked), and only then is the marker bound — `OrderedAbsent=true`
is never written on a path that later falls back (R3-B3). Bound state
on a committed ordered hit is built ONLY from the entry + the
deps-only response:

| field | value on hit | source |
|---|---|---|
| SRID | authored SRID | declaration |
| OrderedAbsent | true | certified active entry |
| Dependencies | fresh fact or nil | deps-only response |
| Read / Exists / Absence / Status / UnreadReason / CatalogName / NameConflict / OrgConflict | all zero | never touched — the skipped existence read is not laundered into catalog fields |

Response-branch table (the determinate cases; attempts vector is
`(R,Qexist,Qdeps,Qo)`):

| deps-only outcome | merge result | arm consequence | attempts |
|---|---|---|---|
| healthy supported, empty deps fact | `Dependencies = fact (0 cols)` | A arm stands | (1,0,1,1) |
| healthy supported, positive valid witness + `!srsDepsStale` | `Dependencies = fact` | **S1** refused_dependency (witness unchanged — `srsDropDependencyWitness` still validates per-column SRID) | (1,0,1,1) |
| positive witness + `srsDepsStale` | `Dependencies = fact` | witness demoted → A arm stands (OP-4b control) | (1,0,1,1) |
| nil snapshot / nil SpatialReference | `Dependencies=nil` | no S1 witness available → A arm proceeds | (1,0,1,1) |
| outer `SpatialReference.SRID != authored` | deps fact foreign → discarded → `Dependencies=nil` | A arm proceeds; a foreign answer is never promoted. **Return-contract member (R3-B4)**: the outer-SRID admission check is new — the baseline helper validates per-column SRID only, so this filtering belongs to the deps-only reader's contract. A foreign outer state carrying authored-SRID columns is a custom/non-contract response — the contract registers it as discard-to-nil, never claiming real mysqlmeta produces it | (1,0,1,1) |
| proven unsupported (Qo parsed, major<8) | support gate short-circuits before the catalog query — deps fact unavailable → `Dependencies=nil`, `OrderedAbsentConsumed` still holds (the certified absence stands), arm proceeds A | (1,0,0,1) |
| unparseable Qo | accepted direction preserved — the deps read still issues, `Absence` supply is irrelevant on this path | (1,0,1,1) |
| Qo/query/read error | propagates through bind — no adjudication, no own-apply for that statement, no new publication; the prefix statement's bound snapshot and the existing entry are unchanged | partial, per actual attempts |
| `srsDepsReader == nil` (no capability) | entry **not consumed** — statement takes the existing full path: same-enrich same full key already cached → `cache-hit` (R=0, all SQL 0, bound state from the full-path answer); cache cold → real full lookup whose answer may change the arm; the entry's keep/barrier is decided by that statement's real adjudication+apply — "fresh" is never claimed | per real path |

- The skipped existence query is never recorded as an executed read;
  a `skipped-by-ordered` annotation marks it.
- A full-lookup return value never overwrites an ordered bound state
  — that rule applies only to the committed ordered branch (R3-B3).

### 4.5 OrderedAbsentConsumed (cross-layer contract)

`OrderedAbsentConsumed(state, authoredSRID)` =
`state != nil && state.OrderedAbsent && state.SRID == authoredSRID`.

The marker is set by the bind **only after** certifying
`entry.absent && !entry.barrier` on the same-SRID entry — barrier is
decided at bind, so adjudication consumes the marker verbatim with no
second barrier check. `state.SRID` is always assigned `decl.SRID` at
bind; the map key does not substitute for this field-level contract.
A-slot predicate: `ValidAbsence(state, srid) || OrderedAbsentConsumed(state, srid)`.

### 4.6 Read policy

- **R** = resolver dispatches actually forwarded to the provider
  (either path); a cache return registers a `cache-hit` event — R=0,
  all SQL deltas 0. Per-statement SQL attempts are counted on
  Qexist/Qdeps/Qo separately.
- On an ordered hit: `R Δ1` (deps-only forwarded),
  `Qexist 0 — skipped-by-ordered` (registered, not executed),
  `Qdeps Δ1`, `Qo Δ1`. Never rewritten as a successful existence read.
- Errors on still-issued reads propagate unchanged.

**Observable differences vs baseline (narrowed)**: (i) same-enrich
read counts change — a hit skips Qexist; (ii) S1 remains reachable on
every hit via the fresh deps read; (iii) error reachability on the
deps path is preserved; (iv) cross-enrich retention (OP-2d) is the
intentional extension. These four deltas ARE the contract's changes —
no other observable behavior is claimed changed.

## 5. OP-D — lifecycle, invalidation, cache separation

- **Owner/lifetime**: `batchState`, request-scoped, same batch, same
  target identity + version admittance. New `batchState` inherits
  nothing (OP-2e). Not a cross-request cache; no concurrency claims.
- **vs objectCache**: three distinct mechanisms, distinct vectors:
  (a) `objectCache` — rebuilt per enrich, full-lookup memoization,
  cache-hit = R0 + `cache-hit` event + all SQL deltas 0; (b) the
  ordered channel — `batchState`-owned, survives the per-enrich cache
  rebuild (OP-2d registers this intentional extension over PA-6d's
  re-read-per-enrich baseline — a model decision, not a baseline
  defect); (c) the deps-only hit path — bypasses objectCache, always
  forwards, fresh witness per consuming statement.
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
  never delete; the reason is provenance preservation, NOT reread
  avoidance — `srsExistenceStale` already blocks the bind before the
  ordered lookup independently (R3-A4). In every reachable state the
  invariant `entry.barrier ⇒ srsExistenceStale[entry.srid]` holds
  (the same boundary/write event mints both); a barrier-only entry
  without the stale flag is a defensive shape the contract never
  produces and is not part of any reachable fixture.

## 6. OP-E — public projection & policy

- All RuleIDs, outcomes, messages, levels, gap owners/reasons,
  `required_facts` order, reserved advisory, and `conditional_audit`
  marker rules stay frozen. A hit row projects the same rule/level/
  message/metadata shape as a catalog-served row of the same arm —
  positional fields (statement index, location) differ per statement
  and are asserted separately; omitempty semantics preserved.
- **Coverage is determined by the arm, never by the hit**: A-plain and
  A-IE✓ keep the accepted complete coverage; A-IE? still owes `[P]` →
  `coverage=unverified`; a statement whose read errored carries the
  error's layer and no completed arm/projection.
- `conditional_audit` marker: inside the A family it is present only
  on A-IE✓; the accepted S2 privilege-notice marker is unaffected —
  this round does not touch it. Successor statements never inherit a
  marker from the entry (each A-IE✓ carries its own premise
  consumption; OP-3a asserts per-statement attribution).
- Ordered provenance stays internal. No new public field, rule, or
  marker is proposed — none is needed; if a later reviewer judges the
  provenance worth surfacing, it is a separate contract decision.
- Policy disabled/level-rewrite touches projections only — publication
  and premise truth unaffected (OP-7d/7e specify concrete config and
  exact findings/gaps/coverage/verdict).
- Providerless CLI three-probe stays regression-only — it can never
  produce ordered publication evidence and no new probe is planned.
- Aggregate coverage/verdict asserted only on declared whole-request
  rows (OP-7a: two clean A-plain statements → aggregate complete,
  verdict reject).

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
`srs_facts=dependencies` is the minimal qualifier vocabulary;
`MySQLSRSDependencyReader` is the right optional-capability surface;
`OrderedAbsent` naming mirrors `ServerState` conventions; per-hit Qo
is required by the provider's support gate rather than cacheable
(kept issued — safe side).

## 8. Implementation touchpoints (registered, not authorized)

1. `batchState` + `srsOrdered map[uint32]*srsOrderedEntry`; own-apply
   mint on A arms; barrier flip on same-SRID boundary/write.
2. `SpatialReferenceSystemState.OrderedAbsent` marker field.
3. `bindMySQLSRS` `orderedSRSDropEligible` gate + ordered lookup step
   + deps-only fall-through.
4. `ObjectLookupQualifierSRSFields`/`srs_facts=dependencies` qualifier;
   `MySQLSRSDependencyReader` optional interface; provider deps-only
   branch (Qo + Qdeps, Qexist skipped); hit path bypasses
   `cachedObjectResolver` and dispatches to the provider directly.
5. A-slot predicate `ValidAbsence || OrderedAbsentConsumed`.
6. Read-ledger annotations: `skipped-by-ordered` (Qexist) and
   `cache-hit` events (zero-SQL returns, R=0).
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
