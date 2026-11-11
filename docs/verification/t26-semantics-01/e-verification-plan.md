# T26-SEMANTICS-01 §E — minimal verification plan (R2 revision)

**Status: PLAN — unexecuted.** No case below has run; expected outputs
are design assertions for the implementation work order, not results.

## E.0 Observation layers (R2-D.1)

Every expectation names the seam that can actually observe it — no
single harness claims all layers:

| layer | observable | existing seam |
|---|---|---|
| enrich / application | resolver calls + qualifiers, bound `Metadata.Server`, ordered entries/barriers/writes | `enrichT26` + recording `t26Provider` stub (`batch_state_t26_test.go`) — records `ResolveObject` **requests**, NOT SQL |
| rule / AuditSQL | notice / reject / gap ids / final coverage | rule registry tests or real `AuditSQL` over a stub provider |
| provider + injected driver | whether catalog SQL was actually issued | the accepted provider tests (`provider_t26_test.go` — fake-driver fragment matching, already covering `identity_unproven` → zero queries) — referenced, not rewritten |

The `t26Provider` stub has no real-SQL visibility — "zero catalog
queries" is observed at the provider layer, "resolver invoked once"
at the enrich layer. They are separate assertions, not one mock.

## E.1 Cases

`[cur]` = reachable under the current fact set; `[dep]` = premise-
dependent (needs the privilege supply, D.0). Post-state expectations
follow R2-A: a non-refusal drop under unproven privilege leaves an
unproven barrier, never absent.

| # | case | expected (layer-tagged) |
|---|---|---|
| E1 | `DROP SERVER \`s1\``, provider present | [cur] rule: notice + gap `server.drop_privilege`, coverage unverified; enrich: `Metadata.Server` proven-present; ordered: `servers[s1]` = unproven-barrier (NOT absent) |
| E2 | same, provider zero rows | [cur] rule: notice + gaps `server.existence`,`server.drop_privilege`, unverified; enrich: `Read=true,Exists=false` still unknown; ordered: barrier |
| E3 | `DROP SERVER 's1'` (literal) | [cur] parser: `unaudited` boundary kept; enrich: one `ResolveObject` request without `name_form` → `identity_unproven` bound; provider layer: zero SQL queries (accepted contract); ordered: `serverChannelTainted` |
| E4 | `DROP SERVER IF EXISTS \`s1\``, present | [cur] same as E1 + `IfExists` declared |
| E5 | `DROP SERVER IF EXISTS \`s1\``, unproven | [cur] gaps `server.existence`+`server.drop_privilege`, unverified; ordered: unproven-barrier — publish condition for `absent`: privilege proven AND (present→delete, absent→no-op convergent arms); unmet ⇒ barrier, never absent |
| E6 | `DROP SERVER \`s1\`; DROP SERVER \`s1\``, first present | [cur] stmt1: barrier on `s1`; stmt2: entry unproven → gap `server.batch_effects`, unverified; catalog `present` NOT re-promoted; [dep] when privilege proven: stmt1 absent → stmt2 reject `[p] drop.absent` |
| E7 | `DROP SERVER \`s1\`; DROP SERVER 's1'` | [cur] stmt2 literal → boundary + `serverChannelTainted`; stmt1 barrier unaffected |
| E8 | provider error | [cur] audit error; `errors.Is` reaches cause; bounded public text; no cached absence |
| E9 | `DROP SERVER \` s1\`` (verbatim ` s1`) | [cur] enrich: resolver request verbatim ` s1`; objectCache key ` s1`; ordered key canonical ` s1` (interior space preserved) |
| E10 | `CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', USER 'u')` | [cur] boundary kept; ordered: write recorded in `serverWrites` even with no prior entry — a later `DROP SERVER \`s\`` gets `server.batch_effects` barrier rather than the catalog's stale answer (R2-B.5 first-write record) |
| E11 | `DROP SERVER \`s1\`` TiDB | [cur] vendor boundary; zero resolver requests |
| E12 | `DROP SERVER \`s1\`; CREATE TABLE t(...)` | [cur] tracked model untouched; table path unaffected |
| E13 | `DROP SERVER \` s1\`; DROP SERVER \`s1\`` | [cur] enrich: two distinct resolver requests (verbatim names differ); ordered keys differ |
| E14 | `ALTER SERVER s OPTIONS(USER 'u')` | [cur] boundary kept; write recorded; provably-same entry tainted if present |
| E15 | `DROP SERVER \`s1\`; DROP SERVER \`S1\``, first present | [cur] `S1`→canonical `s1`, provably-same → hits stmt1's barrier → `server.batch_effects` gap; [dep] proven absent → reject `[p] drop.absent`; alias never restores stale presence |
| E16 | `DROP SERVER \`s1\`; CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h'); DROP SERVER \`s1\`` | [cur] stmt2 boundary + provably-same taint → stmt3 `server.batch_effects` gap — neither old barrier nor catalog present promoted |
| E17 | `DROP SERVER \`s1\`; DROP SERVER IF EXISTS \`s1\`` | [cur] both rows see stmt1's barrier → stmt2 `server.batch_effects` gap (barrier ≠ ordered-absent — no no-op arm); [dep] proven absent + IF EXISTS → guard no-op + `server.drop_privilege` gap (D7) |
| E18 | `DROP SERVER \`s1\`; DROP SERVER IF EXISTS \`S1\`` | [cur] alias canonical hit on barrier → `server.batch_effects`; [dep] → D7 no-op arm, never reject |
| E19 | `DROP SERVER \`s1\`; DROP SERVER 'lit'; DROP SERVER \`s1\`` | [cur] stmt2 literal → `serverChannelTainted`; stmt3 fails the order's step 2 BEFORE touching `s1`'s entry → gap — matches corrected taint priority (channel taint outranks any entry) |
| E20 | `DROP SERVER \`s1\`; CREATE SERVER u ...; DROP SERVER \`s1\`` | [cur] `u` provably-distinct → `s1` entry survives → stmt3 sees stmt1's barrier (not catalog `present`); [dep] ordered-absent → reject |
| E21 | reverse-alias shapes: (a) `DROP SERVER \`sé1\`; DROP SERVER \`se1\`` — non-ASCII write then ASCII lookup; (b) `DROP SERVER \`se1\`; CREATE SERVER sé1 ...; DROP SERVER \`se1\`` — ASCII entry then unproven-relation write | [cur] (a) `R(sé1,se1)` unproven → stmt2 `server.batch_effects` (lookup side); (b) stmt2 write taints `se1` entry → stmt3 barrier (write side — symmetric check applies to existing entries too, R2-B.3) |
| E22 | `DROP SERVER \`s1\`; DROP SERVER \`s1\`` where stmt1 refused `[dep]` | [dep] proven-absent consumed → stmt2 reject; [cur] unreachable — no absent publishes |

Package-level expectations: focused `go test -count=1` on
`internal/application/audit`, `internal/domain/rule/ddl`, and
`internal/infrastructure/parser/tidb`; provider-layer zero-query
assertions reuse `provider_t26_test.go` seams; `gofmt`/`go vet` clean.
