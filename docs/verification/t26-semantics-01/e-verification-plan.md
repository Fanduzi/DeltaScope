# T26-SEMANTICS-01 §E — minimal verification plan

**Status: PLAN — unexecuted.** No case below has run; expected outputs
are design assertions for the implementation work order, not results.

Enrich-level harness: existing `enrichT26` stub pattern
(`internal/application/audit/batch_state_t26_test.go`) — recording
`t26Provider` answers `mysql_server` existence by verbatim name.

| # | case (input → stub) | expected |
|---|---|---|
| E1 | `DROP SERVER \`s1\``, provider answers present | statement audited (not `Unsupported`); notice `[p] ddl.mysql.server.drop.notice`; coverage complete; post `servers[s1]=absent` |
| E2 | same, provider answers zero rows | notice + gap `server.existence`; coverage unverified; post unchanged; `servers[s1]` stays unknown |
| E3 | `DROP SERVER 's1'` (literal) | `unaudited` boundary kept; zero resolver calls; post unchanged |
| E4 | `DROP SERVER IF EXISTS \`s1\``, present | notice; complete; post absent |
| E5 | `DROP SERVER IF EXISTS \`s1\``, zero rows | notice + gap; unverified; post unchanged (never written absent) |
| E6 | `DROP SERVER \`s1\`; DROP SERVER \`s1\``, first present | stmt1 notice+complete+absent; stmt2 pre-state proven-absent via ordered channel → reject `[p] drop.absent`; second resolver call NOT issued (batch-proven wins) |
| E7 | `DROP SERVER \`s1\`; DROP SERVER 's1'`, first present | stmt2 literal → boundary kept; stmt1 unaffected |
| E8 | `DROP SERVER \`s1\``, provider error | audit returns error; `errors.Is` reaches cause; public text fixed; no cached absence |
| E9 | `DROP SERVER \` s1\`` (verbatim ` s1`), provider keyed on ` s1` | resolver receives verbatim ` s1`; notice path; post key ` s1` verbatim |
| E10 | `CREATE SERVER s FDW mysql OPTIONS(HOST 'h')`, provider present | `unaudited` boundary unchanged (create not in slice) — proves slice does not leak to siblings |
| E11 | `DROP SERVER \`s1\`` under TiDB dialect | vendor boundary; zero resolver calls |
| E12 | mixed `DROP SERVER \`s1\`; CREATE TABLE t(...)` | table path unaffected; tracked schema/table model untouched by server channel (line ~410 contract preserved for tracked facts) |
| E13 | resolver issued once per distinct verbatim name across batch (` s1` vs `s1` distinct) | two calls, independent states — cache identity regression guard |
| E14 | `ALTER SERVER s OPTIONS(USER 'u')` | `unaudited` boundary kept |

Package-level expectations: focused `go test -count=1` on
`internal/application/audit`, `internal/domain/rule/ddl`, and
`internal/infrastructure/parser/tidb`; affected-package run covering the
same set used in the provider round; `gofmt`/`go vet` clean. No
full-matrix rerun unless the implementation work order requires it.
