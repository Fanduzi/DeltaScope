# T26-SEMANTICS-01 §D — recommended first implementation slice (R3 revision)

**Slice: `DROP SERVER` audited adjudication on MySQL** (decision-table
units C3–C7 + ordered-channel contract C17–C22). Per R3-B.3 this slice
adopts the **smaller, currently-reachable** scope: boundary narrowing,
existence findings, bounded gaps, barriers, write records,
invalidation, and identity-relation protection. `OrderedAbsent` —
its representation, the successful publish path, and every arm that
consumes it — stays a **future premise-supply-stage design contract**,
not code required by this slice.

## D.0 What is reachable now vs premise-dependent

Reachable under the current fact set (`decl.*` + `server.existence` +
the resolver seam) — **this slice implements exactly these**:

- audited (non-boundary) adjudication of `DROP SERVER` on MySQL
  `NameIdent` forms — the boundary narrows;
- existence findings (`present` provable, others gap-carrying);
- the bounded `server.drop_privilege` gap on every non-terminal-refusal
  drop arm → `coverage=unverified`;
- the ordered layer's full protective semantics: unknown barriers,
  channel taint, write records, relation-checked invalidation — all
  protecting *every* subsequent statement without needing absent
  publishing.

Future premise-supply stage (registered design contract only — D.4;
**not implemented in this slice**):

- `OrderedAbsent` representation, the successful `absent` publish
  path, and every arm consuming ordered-absent (C7 arms, D6/D7 rows
  below marked `[dep]`) — gated on a wired privilege fact for
  `DROP SERVER` (e.g., a grants read on the T24 premise-bundle
  pattern) or a separately-approved exclusion ruling. No privilege
  collection is implemented this round; the publish path is not
  activated by assumed wording or an unwired boolean.

## D.1 Included rows (aligned columns)

| row | statement | decl facts | provider/ordered facts | privilege | outcome | coverage | modeled post-state |
|---|---|---|---|---|---|---|---|
| D1 | `DROP SERVER \`n\``, present | `drop`,`NameIdent` | proven-present | unproven → gap | notice + gap `server.drop_privilege` | review / unverified | unproven-barrier (transition table, C.1b) |
| D2 | `DROP SERVER \`n\``, unproven | same | bound-unvouched / not-read / read-unknown / unbound | unproven → gap | notice + gaps `server.existence`,`server.drop_privilege` | review / unverified | unproven-barrier |
| D3 | `DROP SERVER IF EXISTS \`n\``, present | `IfExists`+ | proven-present | unproven → gap | notice + gap | review / unverified | unproven-barrier |
| D4 | `DROP SERVER IF EXISTS \`n\``, unproven | `IfExists`+ | unproven | unproven → gap | notice + gaps | review / unverified | unproven-barrier — C.4 arm-2 keeps present possible |
| D5 | `DROP SERVER 'n'` (literal) | `NameIdent=false` | resolver invoked → bound not-read `identity_unproven`, zero catalog queries | n/a (boundary) | `unaudited` boundary | incomplete | unchanged + `serverChannelTainted` (C20) |
| D6 `[dep]` | `DROP SERVER \`n\`` on ordered-absent | `drop`,`NameIdent` | `OrderedAbsent` | n/a — refusal proven by absent alone | reject `[p] drop.absent` | reject / complete | absent stands |
| D7 `[dep]` | `DROP SERVER IF EXISTS \`n\`` on ordered-absent | `IfExists`+ | `OrderedAbsent` | unproven → gap | guard arm + gap `server.drop_privilege` — every arm (guard no-op, privilege refusal) preserves absent; the no-op is NOT a proven runtime success | review / unverified | absent stands |
| D8 | drop consumed on barrier / tainted channel | — | step-4 barrier or step-2 taint | unproven → gap | notice + gaps `server.batch_effects`,`server.drop_privilege` — batch_effects replaces the duplicate existence reason per the T04 dedup contract; the independent privilege gap is NOT waived (R3-B.1) | review / unverified | barrier stands |
| D9 | resolver failure | — | error propagates | — | audit error — never a gap, never cached absence | n/a | n/a |

`server.connection_semantics` is ruled inapplicable to DROP (R2-A.1) —
it appears nowhere above. `server.execution_premises` as an umbrella
exclusion is rejected (R2-A.2) — the concrete gap
`server.drop_privilege` replaces it and is registered on every
non-terminal-refusal drop arm, including barrier-consuming ones
(R3-B.1: a barrier only says the pre-state is unproven; it does not
prove this statement refused, so R4 point 6 cannot waive the premise).

## D.2 Ordered identity — single relation table (R3-A)

Relation `R(A,B)` uses the single mutually-exclusive table in §C —
identical in C, D, and E, no per-document envelope:

| relation | sufficient condition | effect |
|---|---|---|
| provably-same | byte-identical; OR ASCII-envelope + ASCII-fold-equal | same object — consume entry / same identity |
| provably-distinct | ASCII-envelope; fold-different; difference not space-only (non-space char differs at same length, or longer tail contains non-space) | different objects — unaffected |
| unproven | everything else (non-ASCII, or space-only deltas) | unproven barrier |

- `canonical(name)` = ASCII fold is a **bucket key**, not an identity
  verdict — different fold keys never prove different objects.
- `DROP s1; DROP S1;` — provably-same → stmt2 hits stmt1's step-4
  barrier (or ordered-absent once the premise lands).
- `DROP \` s1\`; DROP \`s1\`` — leading-space delta → `R` unproven →
  stmt2 gets the step-4 barrier **before** any resolver call — the
  verbatim-objectCache regression keeps its own seam and identity;
  it does not pre-decide this ordered-channel outcome.
- Reverse-alias shapes share one algorithm: non-ASCII write then ASCII
  lookup, or ASCII entry then non-ASCII write — `R` unproven on the
  write side marks the entry a barrier; on the lookup side it returns
  the barrier at step 4.

## D.3 Unified resolve order + transition rules (R3-A.2/A.3, R3-B.2)

Six-step `resolveServer` (identical to C.1b):

1. whole-batch `s.contaminated` → bound-unvouched;
2. `serverChannelTainted` → bound-unvouched — the single taint stop;
3. relation scan over **every** related entry AND recorded write —
   a provably-same hit does not stop the scan;
4. any applicable unknown effect (provably-same recorded write —
   not a consumable fact; unproven relation; existing barrier) →
   return **unproven barrier** — no resolver call on this path;
5. else a provably-same entry holding a **valid** ordered proof →
   consume it ("entry exists" ≠ "entry holds a valid proof");
6. only then provider/cache read (verbatim key) → bind answer.

Transition rule (same table as C.1b): barrier publication follows the
adjudication row's possible effects — present/unknown pre-state +
unproven drop → barrier; **proven absent stays absent** under every
arm (refusal / guard / privilege failure — none create); unaudited
writes invalidate by relation (same → barrier, unproven → barrier,
distinct → untouched, unprovable identity → channel taint). Entries
are channel-owned views; the provider snapshot and prior statements'
bound metadata are never mutated.

## D.4 Future-stage contract: `OrderedAbsent` (registered, not in slice)

`spec.ServerState` gains `OrderedAbsent bool` (recommended over
`Status=not_found` — that vocabulary stays catalog-only). Contract:

- `OrderedAbsent=true` ⇒ `Exists=false`, `Status` ≠ `not_found`,
  `Read` never set by the ordered channel, mutually exclusive with
  `Exists=true`; marks only a valid, proven, uninvalidated ordered
  derivation;
- set only by a same-adjudication *proven* drop — requires the
  privilege-premise supply registered in D.0; **not implemented or
  activated in this slice**;
- cleared into the unproven barrier by write relation or channel
  taint (transition table);
- consumers read ONE adjudicated existence: `OrderedAbsent` → absent;
  else `Read && Exists` → present; else unknown — including
  `Read=true` zero-rows.

## D.5 Seams and expected files (this slice only)

1. `internal/infrastructure/parser/tidb/coverage_boundary.go` —
   narrow the `DDL.Server` MySQL branch: `Operation==drop_server &&
   decl.NameIdent` skips the unaudited mark; everything else keeps
   today's boundary; TiDB arm untouched.
2. `internal/domain/rule/ddl/mysql_server_rules.go` — **new**:
   `AppliesTo`: `Kind==DDL && Dialect==mysql && DDL.Server!=nil &&
   Operation==drop_server && Unsupported==nil`; `Evaluate` reads the
   adjudicated existence → notice / notice+gaps / reject;
   `EvidenceGaps`: `server.existence`, `server.drop_privilege`,
   `server.batch_effects` as applicable.
3. `internal/domain/rule/ddl/register.go` — one config-gated entry.
4. `internal/application/audit/batch_state_server.go` — **new**:
   `serverEntries map[string]*batchServerEntry` (fold-key buckets) +
   `serverWrites []serverWrite` + `serverChannelTainted bool` +
   `resolveServer` (six-step order, barriers before provider) +
   `applyServerStatement` (barrier/transition/write records/taint) +
   `projectServerEntry` (confirmed / unavailable / barrier — never
   `not_found`, never mutates the cached snapshot). The
   `OrderedAbsent` publish branch is left as the documented future
   stage — the entry type carries the shape but no statement can set
   it under this slice.
5. `internal/application/audit/batch_state_server_srs.go` —
   `bindMySQLServer` merges the ordered overlay over the provider
   answer; literal-name path keeps its unissued-read contract.
6. `internal/application/audit/batch_state.go` — replace the SERVER
   early-return (~line 410): SERVER statements route to
   `applyServerStatement`; tracked schema/table model untouched; SRS
   early-return unchanged.
7. Module READMEs; tests in `batch_state_t26_test.go` + rule tests +
   boundary-narrowing parser cases.

The `spec.ServerState.OrderedAbsent` field and publish arm move to the
premise-supply stage's file list — D.5 contains no publish-path code.

## D.6 Order of work

1. entry/barrier/write-list model + `resolveServer` six-step order →
2. `applyServerStatement` transitions + write records + taint →
3. bind merge → 4. boundary narrowing → 5. rule + registration →
6. verification cases (§E, `[cur]` items only as this round's gate;
   `[dep]` expectations are future-stage assertions, not executable
   gates here).

## D.7 Boundaries that stay

`create_server`, `alter_server`, all TiDB SERVER forms, SRS family,
ALTER INSTANCE features, string-literal SERVER names,
`server.connection_semantics` (CREATE/ALTER-side proposal). Option
facts stay inert. No privilege/grant collection is added this round —
`server.drop_privilege` is a registered bounded gap, not a new read.
