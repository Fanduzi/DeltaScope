# Decision: Recover Partial Audits at Statement Boundaries

Date: 2026-08-30
Status: Accepted
Related milestone/version: GitHub issue #43
Related commits: `2e4361490f345001bcf05939cc2cab471aa9f1f6` (statement recovery), `38878afa4e42dfb568b91055d25fad10235278f8` (public surfaces), `7d62fae02f0782a9424fc656c7548502965328aa` (location and boundary review fixes), `c6034181b4f3ad7b03764bd7204f9608d6f48cca` (Unicode dollar-quote boundary fix)
Related tests: `internal/application/audit/service_test.go`, `internal/application/audit/ddl_parser_error_unsupported_contract_postgresql_tag_test.go`, `internal/application/audit/corpus_test.go`, `internal/application/audit/corpus_postgresql_tag_test.go`, public surface diagnostic tests
Related docs: `internal/application/audit/README.md`, `internal/domain/spec/README.md`, `testdata/sql-corpus/README.md`

## Context

DeltaScope previously sent an entire migration string to one dialect parser call. A syntax error in any statement caused that call to discard every parser result, so valid statements before and after the error disappeared from the report. The process failed closed with exit 2, but users received zero audited statements and could lose findings, impact estimates, and source locations from valid SQL.

The existing parser-error contract remains correct for an individual failed statement: DeltaScope must not infer findings or unsupported details from text the dialect parser did not understand. This decision refines that contract for multi-statement input without adding grammar support.

## Decision

DeltaScope performs a bounded lexical scan for top-level semicolon statement boundaries before invoking the existing dialect adapter. The scanner recognizes the quote and comment forms needed to avoid false boundaries, including PostgreSQL dollar-quoted and escape-string bodies. Each resulting slice is still parsed independently by the selected dialect parser.

Successful slices continue through the normal extraction, metadata, impact, rule-evaluation, aggregation, and location paths in source order. Each failed slice produces exactly one safe `audited=false` `parser_error` diagnostic with its 1-based start line and column. If any slice fails, the audit call still returns a non-nil parser error and process surfaces still exit 2.

SDK, CLI, HTTP, and MCP error paths retain the same partial result. JSON and CI renderers surface the bounded diagnostic or unaudited count without embedding the failed SQL. Metadata target inference may use successfully parsed siblings but fails when none were parsed.

## Rationale

- The PingCAP adapter returns no partial AST when its parser reports an error, so recovering after a whole-input call cannot restore valid siblings.
- Calling the installed dialect parser once per lexically bounded statement reuses the authoritative grammar and extraction paths instead of creating a fallback grammar.
- A small scanner is required because splitting on every semicolon would corrupt supported strings, comments, identifiers, and PostgreSQL routine bodies.
- Keeping the existing non-nil error preserves fail-closed automation behavior while the partial report restores useful verified work.

## Public Contract

- Valid statements around a parser failure appear in original source order with their normal findings, impact estimates, and source locations.
- The report verdict and summary describe only statements that reached normal rule evaluation.
- Every parser-failed statement contributes one `parser_error` diagnostic with `audited=false` and optional 1-based `line` / `column` fields.
- Any parser-failed statement keeps the audit error non-nil and the CLI process exit at 2, including when audited siblings would otherwise pass or reject.
- A wholly unparsable single statement remains fail closed with no audited statement result and one bounded diagnostic.
- Diagnostics contain no raw SQL, parser `near ...` fragments, inferred object names, routine bodies, or other parser internals.
- SDK, CLI, HTTP, and MCP serialize the same retained result fields. HTTP adds its normal error object; MCP marks the tool result as an error and adds its bounded code/message.

## Deferred / Out Of Scope

- No fallback grammar, token-to-AST recovery, or semantic guessing for failed statements.
- No parser dependency upgrade or claim of broader MySQL, TiDB, or PostgreSQL grammar support.
- No conversion of failed SQL into findings or structured unsupported details.
- No downgrade of parser failures to exit 0 or 1.
- No support for client-side delimiter directives or procedural dialect forms not already accepted by the selected parser.

## Verification Evidence

- RED reproduction: the issue migration returned exit 2 with `summary.statements=0`; the focused assertion required two retained statements and failed.
- GREEN application tests cover valid-invalid-valid order, findings, impact/location preservation, failures at beginning/middle/end, multiple dialects, wholly invalid input, and semicolons in supported strings/comments.
- PostgreSQL-tagged tests cover dollar-quoted bodies, `E'...'` escape strings, mixed unsupported/parser-error diagnostics, and trailing findings.
- Corpus fixtures cover a beginning failure for MySQL, middle failure for TiDB, and end failure for PostgreSQL.
- SDK, CLI, HTTP, and MCP tests assert the same partial-result and no-leak contract.
- GitHub Actions, GitHub Summary, SARIF, and GitLab Code Quality tests assert visible bounded CI diagnostics.
- Independent Spec review exposed a failed-chunk source-text collision; `7d62fae02f0782a9424fc656c7548502965328aa` confines successful location matching to the original chunk and adds the regression.
- Final Spec review exposed PostgreSQL Unicode dollar-quote tags; `c6034181b4f3ad7b03764bd7204f9608d6f48cca` recognizes the documented Unicode identifier-letter boundary without broadening parser semantics.

## Consequences

- New supported quote/comment forms must be reflected in the boundary scanner regression coverage before they can safely contain semicolons.
- Parser adapters remain the sole authority for statement semantics; the scanner must stay lexical.
- Consumers must treat a non-nil error or `audited=false` diagnostic as an incomplete audit even when the result also contains valid statements and findings.
- Future changes to diagnostic location fields or cross-surface partial-result shape require an update to this decision.

## Links

- Issue: https://github.com/Fanduzi/DeltaScope/issues/43
- Base: `9d8251e89b23c687a2102e343e9b446565c8ad47`
- Commits: `2e4361490f345001bcf05939cc2cab471aa9f1f6`, `38878afa4e42dfb568b91055d25fad10235278f8`, `7d62fae02f0782a9424fc656c7548502965328aa`, `c6034181b4f3ad7b03764bd7204f9608d6f48cca`
- Tests: `internal/application/audit/service_test.go`, `internal/application/audit/ddl_parser_error_unsupported_contract_postgresql_tag_test.go`, `pkg/deltascope/audit_unsupported_diagnostics_evidence_test.go`
- Corpus: `testdata/sql-corpus/mysql/ddl/boundary/`, `testdata/sql-corpus/tidb/ddl/boundary/`, `testdata/sql-corpus/postgresql/ddl/boundary/`
