# Domain Report Module

Audit result aggregation, summary counts, and verdict calculation.

## Files

| File | Responsibility |
|------|---------------|
| result.go | Defines result types and verdict aggregation |
| result_test.go | Verifies verdict and summary behavior |

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

## Notes

- `StatementResult` and `Result` now expose an optional `Explanation` field for additive, shared result context without changing verdict calculation.
- `StatementResult` also exposes an optional `Impact` field for additive statement-level DML impact estimates without changing verdict aggregation semantics.
- `Result` now also exposes an `Unsupported` array for structured partial-support outcomes, allowing supported statements to audit while recognized-but-unsupported statements are still returned to callers.
- The additive `Impact` payload carries `estimated_rows`, `estimated_ratio`, `risk_level`, `confidence`, `source`, `reason_codes`, and optional `notes` for conservative `UPDATE` / `DELETE` estimation.

## Dependencies
- Upstream: application audit orchestration
- Downstream: `internal/domain/rule`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
