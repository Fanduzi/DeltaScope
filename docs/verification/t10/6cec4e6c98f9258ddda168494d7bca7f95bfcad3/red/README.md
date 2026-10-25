# T10-R6 red evidence — worktree replay

Method: `git worktree add /tmp/r6-red-wt 2f37fbd4…` (the R5 product
commit), copy the three R6 test files in unchanged
(`batch_state_t10r6_test.go`, `index_exists_t10r6_test.go`,
`provider_t10r6_test.go`), then `go test -count=1` per package. This is
a replay of the final test files against the pre-R6 product — real
behavioral assertion failures, not compile blocks and not the original
capture session (the first-session red transcripts were captured under
`/tmp/r6-red/` and are superseded by these logs, which match the shipped
test files exactly).

| Log | Package | Top-level behavioral FAILs |
|-----|---------|----------------------------|
| application-audit.log | internal/application/audit | 9 |
| metadata-mysql.log | internal/infrastructure/metadata/mysql | 5 |
| rule-ddl.log | internal/domain/rule/ddl | 2 |
| domain-spec.log | internal/domain/spec | 0 — the spec-layer fix (handle-uncertain PK-object name binding) is pinned by `TestT10R6HandleUnknownPKObjectNameStillBinds` in the audit package; no spec test file exists in R6 |

Every FAIL is an assertion about a real semantic gap the R5 product
carried: unordered/first-hit attribution promotion, unrecognized
clustered values read as NO, withheld-kind consumers silently passing,
`DROP PRIMARY KEY` answering off stale presence, and member-side gates
swallowing a proven PK-object name.
