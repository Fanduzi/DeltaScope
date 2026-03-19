# Domain Report Module

Audit result aggregation, summary counts, and verdict calculation.

## Files

| File | Responsibility |
|------|---------------|
| result.go | Defines result types and verdict aggregation |
| result_test.go | Verifies verdict and summary behavior |

## Exports

- `Verdict`
- `StatementResult`
- `Summary`
- `Result`
- `Aggregate()`

## Dependencies
- Upstream: application audit orchestration
- Downstream: `internal/domain/rule`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
