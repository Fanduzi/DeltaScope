# Domain Report Module

Audit result aggregation, summary counts, and verdict calculation.

## Files

| File | Responsibility |
|------|---------------|
| result.go | Defines result types, per-scope `Coverage`/`CoverageStatus` completeness records, per-statement `EvidenceGaps`, and verdict aggregation |
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
- `Coverage`
- `CoverageStatus`
  Completeness states `CoverageComplete`, `CoverageUnverified`, and `CoverageIncomplete`
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
- `StatementResult` and `Result` carry a `Coverage` record. `Aggregate()` rolls statement coverage up to result coverage (`incomplete` dominates `unverified`, which dominates `complete`). Coverage is a capability fact: recognized-but-unaudited statements stay represented as statement results with `coverage.status=incomplete`, and callers floor `pass` to `review` on incomplete coverage without ever downgrading `reject`.
- `StatementResult.EvidenceGaps` (issue #83 T04-A) carries per-statement `rule.EvidenceGap` records: enabled, applicable metadata-required rules that could not obtain declared source-column facts emit `missing_source_column`/`incomplete_source_column` with a bounded sorted `required_facts` list. Gaps are not findings — they never raise severity counters — but they lower an otherwise-complete statement's coverage to `unverified` and floor the verdict to `review` without ever overriding `reject`.
- `Result.Version` (issue #83 T04-B) carries the resolved `spec.VersionIdentity` when a version fact exists — canonical product/version/components plus `source` (`target` for offline caller input, `observed` for provider-reported identity) and `validated_range`. It is nil when no version fact was established and never carries the raw provider banner.
- The additive `Impact` payload carries `estimated_rows`, `estimated_ratio`, `risk_level`, `confidence`, `source`, `reason_codes`, and optional `notes` for conservative `UPDATE` / `DELETE` estimation.

## Dependencies
- Upstream: application audit orchestration
- Downstream: `internal/domain/rule`, `internal/domain/rule/catalog` (catalog read by `BuildActionSummary`)

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
