# Domain Report Module

Audit result aggregation, summary counts, and verdict calculation.

## Files

| File | Responsibility |
|------|---------------|
| result.go | Defines result types and verdict aggregation |
| result_test.go | Verifies verdict and summary behavior |
| action_summary.go | Derives a human-oriented action summary from findings and catalog entries |
| action_summary_test.go | Verifies action summary grouping, ordering, and fallbacks |

## Exports

- `Verdict`
- `Explanation`
- `ImpactSource`
- `ImpactRisk`
- `ImpactConfidence`
- `Impact`
- `StatementResult`
- `Summary`
- `Result`
- `Aggregate()`
- `ActionSummaryOptions`
- `ActionSummary`
- `ActionItem`
- `BuildActionSummary()`

## Action Summary

`BuildActionSummary` is a **derived human-report helper**. It groups statement and global findings by `rule_id` and orders them by remediation priority so a human reader can decide what to fix first.

- It is derived from `report.Result` and `internal/domain/rule/catalog` entries. It does **not** change `Result` JSON shape.
- It uses `rule.Level` (`blocker`, `warning`, `notice`). It does **not** introduce a `severity` field.
- It does **not** parse SQL, run the audit, evaluate rules, read raw SQL, or read metadata. It only reads existing findings and catalog metadata.
- Statement indexes are 1-based positions into `Result.Statements` and are deduplicated within a rule group; global findings set `HasGlobalFindings` and carry no statement index.
- Ordering is deterministic: level priority (`blocker`, `warning`, `notice`), then count descending, then `rule_id` ascending.
- `ActionSummaryOptions.Limit <= 0` means no truncation; a positive limit truncates `Items` but preserves `TotalItems`.
- An empty result returns a non-nil empty `Items` slice.
- It does not mutate `Result` or catalog entries.

## Notes

- `StatementResult` and `Result` now expose an optional `Explanation` field for additive, shared result context without changing verdict calculation.
- `StatementResult` also exposes an optional `Impact` field for additive statement-level DML impact estimates without changing verdict aggregation semantics.
- `Result` now also exposes an `Unsupported` array for structured partial-support outcomes, allowing supported statements to audit while recognized-but-unsupported statements are still returned to callers.
- The additive `Impact` payload carries `estimated_rows`, `estimated_ratio`, `risk_level`, `confidence`, `source`, `reason_codes`, and optional `notes` for conservative `UPDATE` / `DELETE` estimation.

## Dependencies
- Upstream: application audit orchestration
- Downstream: `internal/domain/rule`, `internal/domain/rule/catalog` (catalog read by `BuildActionSummary`)

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
