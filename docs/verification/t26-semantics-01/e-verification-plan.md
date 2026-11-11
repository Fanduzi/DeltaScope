# T26-SEMANTICS-01 §E — minimal verification plan (R3 revision)

**Status: PLAN — unexecuted.** No case below has run; expected outputs
are design assertions for the implementation work order, not results.

## E.0 Observation layers (R2-D.1, R3-D.2.5)

Every expectation names the seam that can actually observe it — no
single harness claims all layers:

| layer | observable | existing seam |
|---|---|---|
| enrich / application | resolver calls + qualifiers, bound `Metadata.Server` | `enrichT26` + recording `t26Provider` stub (`batch_state_t26_test.go`) — records `ResolveObject` **requests**, NOT SQL |
| ordered channel (application state) | entries / barriers / write records / taint | observed via the application batch-state seam (a test-side accessor on the batch state or the `batchState` fields the package's own tests already reach), OR observed indirectly through a successor statement's bound metadata (`server.batch_effects` on the next statement). `enrichT26` returns enriched statements only — its return value cannot by itself assert the internal post-state map |
| rule / AuditSQL | notice / reject / gap ids / final coverage | rule registry tests or real `AuditSQL` over a stub provider |
| provider + injected driver | whether catalog SQL was actually issued | the accepted provider tests (`provider_t26_test.go` — fake-driver fragment matching, already covering `identity_unproven` → zero queries) — referenced, not rewritten |

The `t26Provider` stub has no real-SQL visibility — "zero catalog
queries" is observed at the provider layer, "resolver invoked once"
at the enrich layer. They are separate assertions, not one mock.

## E.1 Cases

`[cur]` = reachable under the current fact set — this slice's gate.
`[dep]` = future premise-supply-stage assertion — registered, NOT an
executable gate this round. Where a `[dep]` case needs a proven
pre-state, the source is named explicitly. Post-state expectations
follow the C.1b transition table.

| # | case | expected (layer-tagged) |
|---|---|---|
| E1 | `DROP SERVER \`s1\``, provider present | [cur] rule: notice + gap `server.drop_privilege`, unverified; enrich: `Metadata.Server` proven-present; ordered (application-state seam or successor binding): `servers[s1]` = unproven-barrier — NOT absent |
| E2 | same, provider zero rows | [cur] rule: notice + gaps `server.existence`,`server.drop_privilege`, unverified; enrich: `Read=true,Exists=false` still unknown; ordered: barrier |
| E3 | `DROP SERVER 's1'` (literal) | [cur] parser: `unaudited` boundary; enrich: one `ResolveObject` request without `name_form` → bound not-read `identity_unproven`; provider layer: zero SQL queries; ordered: `serverChannelTainted` (exits at step 2 for all later statements) |
| E4 | `DROP SERVER IF EXISTS \`s1\``, present | [cur] same as E1 + `IfExists` declared |
| E5 | `DROP SERVER IF EXISTS \`s1\``, unproven | [cur] gaps `server.existence`+`server.drop_privilege`, unverified; ordered: unproven-barrier. Publish condition for `absent` is explicit: a proven privilege premise AND convergent arms (present→delete, absent→no-op); unmet ⇒ barrier, never absent |
| E6 | `DROP SERVER \`s1\`; DROP SERVER \`s1\``, first present | [cur] stmt1 publishes barrier on `s1`; stmt2's step-3 scan hits it → step-4 barrier → gaps `server.batch_effects`+`server.drop_privilege`, unverified; catalog `present` NOT re-promoted; resolver NOT called for stmt2. [dep] — requires the privilege-premise supply: stmt1 proven drop → ordered-absent; stmt2 → reject `[p] drop.absent` (refusal preserves absent); a third `DROP s1` would reject again — refusal arms never erode absent |
| E7 | `DROP SERVER \`s1\`; DROP SERVER 's1'` | [cur] stmt2 literal → boundary + `serverChannelTainted`; stmt1's barrier unaffected |
| E8 | provider error | [cur] audit error; `errors.Is` reaches cause; bounded public text; no cached absence |
| E9 | `DROP SERVER \` s1\`` (verbatim ` s1`) | [cur] enrich: resolver request verbatim ` s1`; objectCache key ` s1`; ordered bucket canonical ` s1` (interior space preserved) |
| E10 | `CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', USER 'u'); DROP SERVER \`s\`` | [cur] stmt1 boundary + write record (no prior entry needed, R3-A.3); stmt2 step-3 scan finds the provably-same recorded write → step-4 barrier → `server.batch_effects`+`server.drop_privilege` — the write record is an unknown effect, never a consumable fact, and never lets stmt2 fall back to the stale catalog answer |
| E11 | `DROP SERVER \`s1\`` TiDB | [cur] vendor boundary; zero resolver requests |
| E12 | `DROP SERVER \`s1\`; CREATE TABLE t(...)` | [cur] tracked model untouched; table path unaffected |
| E13 | `DROP SERVER \` s1\`; DROP SERVER \`s1\`` | [cur] `R(` s1`,`s1`)` — leading-space delta → **unproven** (PAD-dependent, C/D/E single table); stmt2 exits at step-4 barrier before any resolver call → exactly ONE resolver request total. The accepted verbatim-objectCache regression (distinct keys, distinct requests) keeps its own seam and identity — it records cache-key behavior before the ordered layer existed; it does not pre-decide this ordered-channel outcome (R3-A.1 E13 sync) |
| E14 | `ALTER SERVER s OPTIONS(USER 'u'); DROP SERVER \`s\`` | [cur] stmt1 boundary + provably-same write record → stmt2 step-4 barrier |
| E15 | `DROP SERVER \`s1\`; DROP SERVER \`S1\``, first present | [cur] `R(S1,s1)` provably-same (ASCII fold) → stmt2 hits stmt1's barrier at step 3–4 → `server.batch_effects`+`server.drop_privilege`. [dep] proven absent → reject `[p] drop.absent`; alias never restores stale presence |
| E16 | `DROP SERVER \`s1\`; CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h'); DROP SERVER \`s1\`` | [cur] verifies the write-record + relation path: stmt2's unaudited create records; stmt3 barrier. [dep] for valid-proof invalidation: stmt1 a proven drop → ordered-absent; stmt2 provably-same unaudited write → absent invalidated to barrier (transition table); stmt3 sees barrier |
| E17 | `DROP SERVER \`s1\`; DROP SERVER IF EXISTS \`s1\`` | [cur] stmt1 reads its own pre-state and publishes a barrier; stmt2 reads stmt1's post-state → barrier → `server.batch_effects`+`server.drop_privilege` (stmt1's bound metadata stays immutable). [dep] — explicit precondition: a privilege proof scoped to cover BOTH statements; then stmt1 publishes ordered-absent; stmt2 consumes absent → guard arm → absent stands; a shared proof does not emit a second privilege gap — if the future contract instead supplies per-statement proofs, stmt2's gap is expected and must say so |
| E18 | `DROP SERVER \`s1\`; DROP SERVER IF EXISTS \`S1\`` | [cur] alias provably-same → stmt2 barrier. [dep] same shared-proof precondition as E17 → stmt2 guard arm, absent stands, no repeat gap |
| E19 | `DROP SERVER \`s1\`; DROP SERVER 'lit'; DROP SERVER \`s1\`` | [cur] stmt2 literal → `serverChannelTainted`; stmt3 exits at step 2 — the taint stop precedes the entry scan, so `s1`'s surviving entry is never consulted (matches the single stop position). [dep] valid-proof-taint arm: stmt1 proven absent → taint still wins at step 2 |
| E20 | `DROP SERVER \`s1\`; CREATE SERVER u FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', USER 'u'); DROP SERVER \`s1\`` | [cur] `u` provably-distinct (non-space differing char) → `s1` entry untouched → stmt3 sees stmt1's barrier, not catalog `present`. [dep] ordered-absent survives → reject |
| E21 | reverse-alias: (a) `DROP SERVER \`sé1\`; DROP SERVER \`se1\``; (b) `DROP SERVER \`se1\`; CREATE SERVER sé1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', USER 'u'); DROP SERVER \`se1\`` | [cur] both directions land `unproven` (non-ASCII) → (a) lookup side: stmt2 step-4 barrier; (b) write side: stmt2's recorded write taints `se1`'s entry → stmt3 barrier. [dep] for valid-proof invalidation: stmt1 a proven drop first |

Package-level expectations: focused `go test -count=1` on
`internal/application/audit`, `internal/domain/rule/ddl`, and
`internal/infrastructure/parser/tidb`; provider-layer zero-query
assertions reuse `provider_t26_test.go` seams; `gofmt`/`go vet` clean.
