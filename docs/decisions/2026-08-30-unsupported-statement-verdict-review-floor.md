# Decision: Floor Otherwise-Passing Unsupported Statement Results to Review

Date: 2026-08-30
Status: Accepted
Related milestone/version:
Related issue: GitHub issue #60
Related commits:
Related tests:
- `internal/application/audit/service_unsupported_verdict_floor_postgresql_tag_test.go`
- `pkg/deltascope/audit_unsupported_verdict_floor_postgresql_tag_test.go`
- `internal/interfaces/cli/cli_unsupported_verdict_floor_postgresql_tag_test.go`
- `internal/interfaces/http/handler_unsupported_verdict_floor_postgresql_tag_test.go`
- `internal/interfaces/mcp/server_unsupported_verdict_floor_postgresql_tag_test.go`
Related docs:
- `internal/application/audit/README.md`
- `pkg/deltascope/README.md`
- `internal/interfaces/cli/README.md`
- `internal/interfaces/http/README.md`
- `internal/interfaces/mcp/README.md`
Related decisions:
- `2026-08-30-partial-parser-error-verdict-review-floor.md` (issue #56)
- `2026-08-17-cli-user-input-exit-mapping.md` (issue #24)

## Context

Issue #56 floored otherwise-passing mixed parser-error results to `review` at the shared application seam. Structured unsupported statements had the same completeness gap: PostgreSQL `SELECT 1` produced `unsupported_statement`, `audited=false`, `ErrUnsupportedStatement`, and CLI exit 1 while aggregation still reported `verdict=pass`.

The original issue proposed counting unaudited statements in `Statements`. That would mix incompleteness into the audited-statement contract. `Unsupported` and `Diagnostics` already represent those slices.

## Decision

At the shared application result seam, a result containing one or more structured unsupported statements applies a `review` floor only when normal finding aggregation computed `pass`. Existing `review` and `reject` verdicts remain unchanged.

Unsupported statements stay in `Unsupported` and `Diagnostics`. They are not fabricated as audited `StatementResult` entries. `Statements` and `Summary.Statements` continue to count only statements that reached normal rule evaluation.

The floor applies in `Service.Audit` before returning the existing wrapped `ErrUnsupportedStatement`. SDK, CLI, HTTP, and MCP inherit the floored verdict without renderer-specific policy.

## Rationale

- `pass` must not imply a complete audit when a bounded statement has `classification=unsupported_statement` and `audited=false`.
- Reusing `review` as a floor extends the #56 completeness rationale without a fourth verdict value.
- Applying the floor once in the application service keeps SDK, CLI, HTTP, and MCP consistent.
- Leaving unaudited slices in `Unsupported`/`Diagnostics` preserves the audited-statement summary contract.
- Conditional promotion preserves stronger verdicts, findings, impact, source order, and audited counts.

## Public Contract

- A result containing structured unsupported details cannot have verdict `pass`; it has at least `review`.
- Existing `review` and `reject` verdicts never downgrade.
- Audited sibling statements, findings, impact, source locations, and summary counts are preserved.
- Unsupported details and `unsupported_statement` / `audited=false` diagnostics remain the representation of unaudited recognized statements.
- The application and SDK still return a non-nil `ErrUnsupportedStatement` (wrapped), the CLI still exits 1, HTTP still returns a non-success diagnostic envelope, and MCP still marks the tool result as an error.
- Parser-error behavior is unchanged: mixed parser failures keep the #56 review floor and exit/status; wholly unparseable input keeps the empty-verdict contract.

## Deferred / Out Of Scope

- No PostgreSQL `SELECT` audit rules or parser support.
- No new verdict enum value.
- No change that counts unaudited statements as audited `StatementResult` entries.
- No change to `ErrUnsupportedStatement`, CLI exit 1, diagnostic shape, or no-leak guarantees.
- No renderer-specific verdict policy.

## Verification Evidence

- Application tests cover PostgreSQL `SELECT 1` pass-to-review, mixed notice-only siblings, unchanged `review`/`reject`, and metadata-aware inheritance of the same floor.
- SDK, CLI JSON/Markdown plus exit 1, HTTP, and MCP tests exercise the same `SELECT 1` contract through public seams.
- Existing parser-error review-floor tests remain green.

## Consequences

Consumers may treat `pass` as requiring every bounded statement to have reached rule evaluation. Consumers must still honor non-nil unsupported errors and transport error signaling because `review` does not make an unsupported statement audited.

Future changes to this completeness floor or the verdict enum require a new decision record that links this revision.

## Links

- Issue #60: https://github.com/Fanduzi/DeltaScope/issues/60
- Issue #56: https://github.com/Fanduzi/DeltaScope/issues/56
- Related decision: `docs/decisions/2026-08-30-partial-parser-error-verdict-review-floor.md`
- Application tests: `internal/application/audit/service_unsupported_verdict_floor_postgresql_tag_test.go`
- SDK tests: `pkg/deltascope/audit_unsupported_verdict_floor_postgresql_tag_test.go`
- CLI tests: `internal/interfaces/cli/cli_unsupported_verdict_floor_postgresql_tag_test.go`
- HTTP tests: `internal/interfaces/http/handler_unsupported_verdict_floor_postgresql_tag_test.go`
- MCP tests: `internal/interfaces/mcp/server_unsupported_verdict_floor_postgresql_tag_test.go`
