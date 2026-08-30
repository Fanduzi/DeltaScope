# Decision: Compact CLI JSON Skipped-Rule Output

- Date: 2026-08-30
- Status: Accepted
- Issue: [#55](https://github.com/Fanduzi/DeltaScope/issues/55)
- Related decision: [Aggregate Markdown Rule Skip Reasons](2026-08-17-markdown-rule-summary-aggregation.md)
- Related commit subject: `fix(cli): compact audit JSON skipped rules (#55)` (co-committed with this record)
- Related tests: `internal/interfaces/cli/cli_test.go`
- Related docs: `docs/reference/cli.md`, `docs/reference/cli.zh-CN.md`

## Context

The CLI JSON audit result inherited the complete per-rule `rule_summary.skipped`
list from the domain result. A small MySQL audit can therefore emit hundreds of
PostgreSQL rule IDs even when the useful verdict and findings are tiny. Issue
[#17](https://github.com/Fanduzi/DeltaScope/issues/17) bounded Markdown but
intentionally left this machine-output problem unresolved.

## Decision

Change only the CLI audit JSON presentation. `rule_summary.skipped` is now a
deterministic reason-sorted array of `{reason, count}` objects whose counts
cover every skipped rule, including an empty array when none are skipped. The
audit-only `--include-skipped-rules` flag adds the stable complete per-rule list
in a separate optional `rule_summary.skipped_rules` field. The two fields
remain distinct and are never polymorphic.

The transformation is implemented by a CLI-specific output view. Application
and domain result types, SDK/HTTP/MCP output, Markdown, quiet Markdown, GitHub
Actions, and SARIF are unchanged. JSON with `--quiet` is byte-for-byte equal to
ordinary JSON for the same audit flags.

## Rationale

Reason aggregation makes the default machine output bounded while preserving
the operational signal of why rules were skipped. An explicit opt-in retains
the existing per-rule evidence for consumers that need rule IDs. Keeping this
at the CLI presentation boundary avoids changing shared semantics or adding a
serializer abstraction or dependency.

## Public Contract

- Default CLI JSON emits `rule_summary.skipped` as sorted `{reason, count}` objects, or `[]` when no rule was skipped, and omits `rule_summary.skipped_rules`.
- `--include-skipped-rules` adds `rule_summary.skipped_rules`, with objects shaped `{rule_id, reason}`, while retaining the aggregate `skipped` field.
- Aggregate counts are exact and include unknown future reason codes; ordering is deterministic by reason code.
- The full opt-in list preserves its existing deterministic rule-ID order.
- `--quiet --format json` has the same bytes and exit behavior as `--format json` with the same other flags.
- Verdict, findings, context, exit codes, all other CLI formats, and non-CLI surfaces remain unchanged.

## Deferred / Out Of Scope

- No change to `report.Result`, `report.RuleSummary`, `rule.SkippedRule`, or any application/domain public struct.
- No change to SDK, HTTP, MCP, Markdown, quiet Markdown, GitHub Actions, SARIF, or GitLab Code Quality output.
- No generic serializer, output version field, or new skip-reason inference.
- No release/version behavior change beyond documenting this CLI JSON contract.

## Verification Evidence

- Focused CLI contract tests: 17 passed, covering default aggregation and deterministic ordering, the zero-skip `[]` shape, the opt-in stable list, quiet JSON byte stability with and without the flag, unchanged non-JSON formats, and help advertising.
- `go test ./... -count=1`: 4,570 passed across 40 packages.
- `go vet ./...`, `make build`, `make docs-example-gates`, `make sql-corpus-gates`, and `make pg-unit-test-gates`: passed.
- Staged three-level documentation, decision-record, and `git diff --check` gates: passed; root README architecture was unchanged because no module boundary changed.
- Fixed-point review against `bc9193c86b08a076ca0282471d7991b561b1a458`: Standards had one judgment-only smell, resolved with typed `rule.SkipReason`; Spec had one P2 for the zero-skip array, resolved; no unresolved P1/P2 findings.

## Consequences

Consumers that read skipped rule IDs from default CLI JSON must add
`--include-skipped-rules` and read `rule_summary.skipped_rules`. Consumers that
only need skip counts can use the smaller default aggregate without catalog
specific knowledge.

## Links

- Issue: [#55](https://github.com/Fanduzi/DeltaScope/issues/55)
- Preceding decision: [2026-08-17-markdown-rule-summary-aggregation.md](2026-08-17-markdown-rule-summary-aggregation.md)
- Commit subject: `fix(cli): compact audit JSON skipped rules (#55)` (co-committed with this record)
- Tests: `internal/interfaces/cli/cli_test.go`
- CLI reference: `docs/reference/cli.md`, `docs/reference/cli.zh-CN.md`
