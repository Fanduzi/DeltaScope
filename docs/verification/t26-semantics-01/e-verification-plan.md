# T26-SEMANTICS-01 §E — minimal verification plan (R1 revision)

**Status: PLAN — unexecuted.** No case below has run; expected outputs
are design assertions for the implementation work order, not results.

Enrich-level harness: existing `enrichT26` stub pattern
(`internal/application/audit/batch_state_t26_test.go`) — recording
`t26Provider` answers `mysql_server` existence by verbatim name and
counts `ResolveObject` calls separately from issued SQL queries. Per
the accepted contract, "resolver called" and "catalog query issued"
are different observations: a literal name reaches the resolver
without `name_form=identifier` and binds `identity_unproven` with zero
queries.

| # | case (input → stub) | expected |
|---|---|---|
| E1 | `DROP SERVER \`s1\``, provider answers present | audited (not `Unsupported`); notice `[p] drop.notice`; coverage complete (conditional on registered exclusions — see C-table); post `servers[s1]=absent` |
| E2 | same, provider answers zero rows | notice + gap `server.existence`; unverified; post withheld; `Read=true` does NOT turn into existence-known |
| E3 | `DROP SERVER 's1'` (literal) | `unaudited` boundary kept; **resolver invoked once** (no `name_form` qualifier) → bound `identity_unproven`; **zero catalog queries issued**; channel tainted (D5) |
| E4 | `DROP SERVER IF EXISTS \`s1\``, present | notice; complete-conditional; post absent |
| E5 | `DROP SERVER IF EXISTS \`s1\``, unproven existence | notice + gap `server.existence`; unverified; post = conditional `absent` (convergent arms, C.4) — NOT unchanged and NOT written on the affected key unless the exclusion disposition withholds it |
| E6 | `DROP SERVER \`s1\`; DROP SERVER \`s1\``, first present | stmt1 notice + post absent; stmt2 ordered-proven-absent → reject `[p] drop.absent`; second catalog read NOT issued for the overlay decision (batch-proven entry wins) |
| E7 | `DROP SERVER \`s1\`; DROP SERVER 's1'`, first present | stmt2 literal → boundary kept + channel taint; stmt1 unaffected; any later ident-name drop → `server.batch_effects` gap |
| E8 | `DROP SERVER \`s1\``, provider error | audit returns error; `errors.Is` reaches cause; public text fixed; no cached absence |
| E9 | `DROP SERVER \` s1\`` (verbatim ` s1`), provider keyed on ` s1` | resolver receives verbatim ` s1`; notice path; objectCache key ` s1` verbatim; canonical ordered key ` s1` (fold preserves interior space) |
| E10 | `CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', USER 'u')`, provider present | `unaudited` boundary unchanged — sibling boundary does not leak; **also taints nothing provably-same unless the name collides** (see E15/E16) |
| E11 | `DROP SERVER \`s1\`` under TiDB dialect | vendor boundary; zero resolver calls AND zero catalog queries |
| E12 | `DROP SERVER \`s1\`; CREATE TABLE t(...)` | table path unaffected; tracked schema/table model untouched by the server channel (batch_state.go ~410 contract preserved for tracked facts) |
| E13 | `DROP SERVER \` s1\`; DROP SERVER \`s1\``, provider keyed verbatim | two resolver calls (distinct verbatim names), independent object states; ordered channel keys differ (` s1` vs `s1`) |
| E14 | `ALTER SERVER s OPTIONS(USER 'u')` | `unaudited` boundary kept; entry/taints: provably-same-key entries tainted (C20) |
| E15 | `DROP SERVER \`s1\`; DROP SERVER \`S1\``, first present | canonical `S1`→`s1` hits ordered-absent → stmt2 reject `[p] drop.absent`; catalog not re-consulted for the overlay — case alias does NOT restore stale presence (R1-B.2) |
| E16 | `DROP SERVER \`s1\`; CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h'); DROP SERVER \`s1\`` | stmt1 post absent; stmt2 boundary kept + taints `s1` entry; stmt3 → `server.batch_effects` gap + unverified — neither the old absent nor the catalog's stale present is promoted (R1-B.3–4) |
| E17 | `DROP SERVER \`s1\`; DROP SERVER IF EXISTS \`s1\``, first present | stmt2 ordered-absent + guard → no-op success arm (D7): review-level notice, no reject; post stays absent |
| E18 | `DROP SERVER \`s1\`; DROP SERVER IF EXISTS \`S1\``, first present | alias + guard: canonical hit → no-op arm, not reject |
| E19 | `DROP SERVER \`s1\`; DROP SERVER 'lit'; DROP SERVER \`s1\`` | stmt2 literal-name write → `serverChannelTainted`; stmt3 → `server.batch_effects` gap even though the catalog could read `s1` — taint shadows catalog (C20/C22) |
| E20 | `DROP SERVER \`s1\`; CREATE SERVER u ...; DROP SERVER \`s1\`` | provably-distinct write (`u` ≠ `s1`): s1's ordered entry survives → stmt3 reject on ordered-absent — no over-nuking |
| E21 | `DROP SERVER \`sé1\`; DROP SERVER \`SÉ1\`` (non-ASCII), first present | non-ASCII name → fold-equivalence unproven → stmt2 `server.batch_effects` gap (C18c) rather than silent overlay miss or silent catalog promote |

Package-level expectations: focused `go test -count=1` on
`internal/application/audit`, `internal/domain/rule/ddl`, and
`internal/infrastructure/parser/tidb`; affected-package run covering the
same set used in the provider round; `gofmt`/`go vet` clean. No
full-matrix rerun unless the implementation work order requires it.
