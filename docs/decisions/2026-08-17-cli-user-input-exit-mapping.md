# Decision: Map Unknown Flags and Parser-Error SQL to CLI Exit 2

Date: 2026-08-17
Status: Accepted
Related milestone/version: issue #24
Related commits:
Related tests:
- `TestAuditUnknownFlagSqllExitsUser`
- `TestAuditCommandRejectsRemovedPasswordFlag`
- `TestAuditUnparseableSQLJSONExitsUser`
- `TestAuditUnparseableSQLMarkdownExitsUser`
- `TestAuditExistingUserErrorsStayExitUser`
- `TestQueryAccessAnalyzeRejectsRemovedPasswordFlag`
Related docs:
- `docs/reference/cli.md`
- `docs/reference/cli.zh-CN.md`

## Context

Published CLI docs say exit `2` is bad user input, including invalid flags and malformed
SQL. Unknown flags such as `--sqll` and the removed `--password` flag took the Cobra
error path and defaulted to exit `3`. Unparseable SQL returned the existing
`parser_error` diagnostic plus a sentinel that `mapAuditError` did not classify as user
input, so it also exited `3`.

The original issue asked for a new verdict such as `error` or `unsupported`. Existing
shipped verdicts are only `pass`, `review`, and `reject`. Reusing one of those for
"not audited" would lie. Adding a fourth value is a public enum change.

## Decision

Unknown flags and unparseable SQL exit `2` on the audit/config/rules CLI table.

Parser-error results keep the current `diagnostics[]` contract and the empty JSON
`verdict`. Consumers must use `diagnostics[].classification == parser_error`. No new
verdict value is introduced.

`query-access` keeps its own table: usage errors, including unknown flags, stay at
exit `3`. That command already documents `2` as indeterminate admission.

## Rationale

Exit `3` is the runtime/internal bucket. A typo flag or a file the parser cannot read
is user input, not an engine crash.

An empty verdict is the honest current shape: the statement was not audited. Filling
it with `pass`, `review`, or `reject` would change meaning. A new enum value is
deferred until a later issue owns that public contract.

## Public Contract

- `deltascope audit --sqll ...` and `deltascope audit --password ...` exit `2` and
  print the Cobra unknown-flag error.
- Unparseable SQL exits `2`, still emits `diagnostics[].classification == parser_error`,
  and does not leak raw parser `near` text or the SQL payload.
- JSON `verdict` may be `""`. This is not `pass`.
- Already-correct user errors (bad `--format`, unsupported `--dialect`, missing
  `--file`) stay at exit `2`.
- `query-access` unknown flags stay at exit `3`.
- Verdict enum remains `pass` / `review` / `reject` only.

## Deferred / Out Of Scope

- Adding `error` or `unsupported` as a verdict
- Inferring findings from unparsed SQL
- Changing `--fail-on` mapping for completed audits
- Empty `--sql` hang (issue #19)
- Metadata connection messages (issue #23)
- Changing HTTP, MCP, or SDK status mapping

## Verification Evidence

CLI `Execute` tests cover `--sqll`, removed `--password`, JSON and markdown
unparseable SQL, and the existing format/dialect/missing-file controls.
`TestQueryAccessAnalyzeRejectsRemovedPasswordFlag` keeps query-access usage at 3.

## Consequences

Future verdict-enum work needs its own decision record. Do not reuse `pass`,
`review`, or `reject` for parser-error results. Query-access exit `2` remains
indeterminate, not "bad flag".

## Links

- Issue: https://github.com/Fanduzi/DeltaScope/issues/24
- Tests: `internal/interfaces/cli/cli_user_input_exit_test.go`,
  `internal/interfaces/cli/cli_test.go`,
  `internal/interfaces/cli/query_access_test.go`
- Mapping: `internal/interfaces/cli/cli.go`, `internal/interfaces/cli/audit.go`
