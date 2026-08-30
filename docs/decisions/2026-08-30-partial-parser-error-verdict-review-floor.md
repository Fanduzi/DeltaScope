# Decision: Floor Otherwise-Passing Partial Parser Results to Review

Date: 2026-08-30
Status: Accepted
Related issue: GitHub issue #56
Related commits: [`ad777bf2ec9667e91259fb0b5713f43196b35363`](https://github.com/Fanduzi/DeltaScope/commit/ad777bf2ec9667e91259fb0b5713f43196b35363) (`fix(audit): floor partial parser pass verdicts to review (#56)`)
Related decisions:
- `2026-08-17-cli-user-input-exit-mapping.md` (issue #24)
- `2026-08-30-partial-parser-error-recovery.md` (issue #43)

## Context

Issue #43 made mixed migrations useful by retaining statements that parse while returning a safe `audited=false` `parser_error` diagnostic and a non-nil error for each failed slice. Its verdict still summarized only audited findings. When every retained finding was notice-level, that computation produced `pass` even though the CLI exited 2 and HTTP/MCP signaled an error.

Issue #24 intentionally left wholly unparseable verdict behavior unchanged rather than adding a fourth verdict value. The existing `pass`, `review`, and `reject` vocabulary remains sufficient for the narrower mixed-input completeness signal.

## Decision

At the shared application result seam, a partial parse with any parser-error failure applies a `review` floor only when normal finding aggregation computed `pass`. Existing `review` and `reject` verdicts remain unchanged.

The floor applies after valid siblings complete extraction, metadata enrichment, impact estimation, rule evaluation, and aggregation. It does not run on the existing wholly unparseable return paths.

## Rationale

- `pass` must not imply a complete audit when a bounded statement has `classification=parser_error` and `audited=false`.
- Reusing `review` as a floor avoids a new public enum and preserves the #24 verdict vocabulary.
- Applying the floor once in the application service keeps SDK, CLI, HTTP, and MCP consistent without renderer-specific policy.
- Conditional promotion preserves stronger verdicts and all finding-derived counts.

## Public Contract

- A mixed/partial result containing an `audited=false` `parser_error` diagnostic cannot have verdict `pass`; it has at least `review`.
- Existing `review` and `reject` verdicts never downgrade.
- Audited sibling statements, findings, impact, source locations, summary counts, diagnostics, and unsupported details are preserved.
- The application and SDK still return a non-nil parser error, the CLI still exits 2, HTTP still returns its parser-error status/envelope, and MCP still marks the tool result as an error.
- Wholly unparseable inputs retain their existing #24/#43 behavior.

## Deferred / Out Of Scope

- No new verdict enum value.
- No parser grammar, fallback parsing, semantic guessing, or diagnostic-shape changes.
- No change to parser-error exit/status/error codes or renderer-specific verdict logic.

## Verification Evidence

- Application tests cover the `pass` to `review` floor and unchanged `review`/`reject` verdicts.
- SDK, CLI JSON/Markdown, HTTP, and MCP tests exercise the same mixed migration through their public seams while retaining siblings, findings, locations, and error signaling.
- Existing wholly unparseable and parser-diagnostic suites retain the #24/#43 compatibility behavior.

## Consequences

Consumers may treat `pass` as requiring every bounded statement to have reached rule evaluation. Consumers must still honor non-nil errors and transport error signaling because `review` does not make a partial audit complete.

Future changes to this completeness floor or the verdict enum require a new decision record that links this revision.

## Links

- Implementation commit: `fix(audit): floor partial parser pass verdicts to review (#56)` (co-committed with this record)
- Issue #56: https://github.com/Fanduzi/DeltaScope/issues/56
- Issue #24: https://github.com/Fanduzi/DeltaScope/issues/24
- Issue #43: https://github.com/Fanduzi/DeltaScope/issues/43
- Application tests: `internal/application/audit/service_test.go`
- SDK tests: `pkg/deltascope/audit_unsupported_diagnostics_evidence_test.go`
- CLI tests: `internal/interfaces/cli/cli_unsupported_diagnostics_evidence_test.go`
- HTTP tests: `internal/interfaces/http/handler_unsupported_diagnostics_evidence_test.go`
- MCP tests: `internal/interfaces/mcp/server_unsupported_diagnostics_evidence_test.go`
