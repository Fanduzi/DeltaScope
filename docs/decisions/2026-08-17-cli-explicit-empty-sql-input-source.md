# Decision: Treat Explicit Empty `--sql` as Provided Input

Date: 2026-08-17 (updated 2026-09-04)
Status: Accepted
Related milestone/version: issue #19; follow-up issues #32, #68
Related commits:
Related tests:
- `TestAuditCommandRejectsExplicitEmptySQLWithoutReadingStdin`
- `TestExplicitEmptySQLErrorsNameTheirCommands`
- `TestAuditCommandRejectsEmptyFileInput`
- `TestResolveAuditSQLRejectsConflictingOrEmptyInput`
- `TestAuditCommandSupportsStdinInput`
- `TestQueryAccessAnalyzeRejectsExplicitEmptySQLWithoutReadingStdin`
- `TestQueryAccessAnalyzeEmptyStdin`
- `TestQueryAccessAnalyzeMissingFileErrorIsBounded`
- `TestQueryAccessAnalyzeSupportsStdinAndFileInput`
- `TestAuditSQLToolReturnsStructuredErrorForEmptySQL`
Related docs:
- `docs/reference/cli.md`
- `docs/reference/cli.zh-CN.md`
- `docs/reference/query-access-analysis.md`
- `docs/recipe/audit-sql-offline.md`
- `docs/recipe/audit-sql-offline.zh-CN.md`

## Context

`deltascope audit --sql ""` and `--sql "   "` treated the flag as omitted because
`resolveAuditSQL` only inspected `strings.TrimSpace(inlineSQL)`. The command then
called `io.ReadAll(stdin)`. If stdin stayed open, the process hung with no output.

Issue #32 found the same provided-vs-omitted fallthrough in
`query-access analyze`, whose documented usage exit class is 3.

The same empty value with stdin closed already failed with `SQL input must not be
empty` and exit 2. MCP `audit_sql sql=""` already fail-closes with `bad_request`.

## Decision

If the `--sql` flag is present (`Flags().Changed("sql")`), that value is the SQL
input. Empty or whitespace-only `--sql` fails immediately without reading stdin.

Issue #68 keeps the exit classes split and names each command in the error text:
`audit: SQL input must not be empty` with exit 2, and
`query-access: SQL input must not be empty` with exit 3. The two stderr texts
must not be identical. A missing or unreadable Query Access `--file` returns the
bounded message `cannot read SQL file` with exit 3 and no OS error or filesystem
path.

`--file` empty content and omitted-flag stdin audits keep their existing behavior.
MCP empty-SQL handling is unchanged.

## Rationale

Agents and CI snippets copy `--sql "..."` and sometimes pass an empty string. That
must be a usage error, not a blocked process. Flag presence is the correct
"provided vs omitted" signal; trimming the value cannot distinguish `--sql ""`
from a missing flag.

## Public Contract

- Explicit `--sql`, including `""` and whitespace-only text, is the audit SQL source.
- Empty explicit `--sql` on `audit` exits 2 with `audit: SQL input must not be empty`
  and does not read stdin.
- `echo 'delete from users' | deltascope audit` is unchanged.
- Empty `--file` content still exits 2 with `audit: SQL input must not be empty`.
- MCP empty SQL remains `isError` / `code=bad_request` / `audit SQL must not be empty`.
- Query Access with neither `--sql` nor `--file` reads stdin; empty stdin exits 3
  with `query-access: SQL input must not be empty`.
- Empty explicit `--sql` on `query-access analyze` exits 3 with
  `query-access: SQL input must not be empty` and does not read stdin.
- The two empty-SQL stderr texts are not identical; each names its command.
- Query Access valid inline, file, and stdin inputs retain their result and
  admission exit codes.

## Deferred / Out Of Scope

- Changing the existing TTY stdin hint
- Metadata connection errors (#23)
- Unknown-flag / parser-error exit codes (#24)
- Successful stdin or `--file` audits

## Verification Evidence

CLI `Execute` tests pass a stdin reader that fails if read, proving `--sql ""` and
`--sql "   "` do not fall through. Existing stdin and empty-file tests remain.
Query Access `Execute` tests prove the same behavior with a non-EOF stdin reader,
preserve empty-stdin/file/stdin behavior, and assert missing-file path redaction.
The two empty-`--sql` stderr texts are asserted distinct and command-named, with
exit 2 versus 3 unchanged. MCP `TestAuditSQLToolReturnsStructuredErrorForEmptySQL`
remains the empty-SQL fail-closed check.

## Consequences

Future CLI input sources must distinguish flag presence from empty values. Do not
treat `TrimSpace(flag) == ""` as "flag omitted" when a missing flag should fall
through to another source.

## Links

- Issue: https://github.com/Fanduzi/DeltaScope/issues/19
- Follow-up issues: https://github.com/Fanduzi/DeltaScope/issues/32,
  https://github.com/Fanduzi/DeltaScope/issues/68
- Tests: `internal/interfaces/cli/cli_test.go`, `internal/interfaces/cli/query_access_test.go`, `internal/interfaces/mcp/server_test.go`
- Docs: `docs/reference/cli.md`, `docs/reference/cli.zh-CN.md`, `docs/reference/query-access-analysis.md`
- Related: `docs/decisions/2026-09-04-named-public-signals.md`
