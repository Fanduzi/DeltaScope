# Rule Catalog Module

Explanation-oriented metadata for shipped DeltaScope rules.

## Files

| File | Responsibility |
|------|---------------|
| catalog.go | Builds stable catalog entries from shipped defaults and explanation templates for CLI discovery |
| catalog_test.go | Verifies catalog completeness, lookup stability, and metadata-aware flags |

## Exports

- `Entry`
- `All()`
- `Lookup(ruleID)`
- `Search(query)`

## Dependencies
- Upstream: `internal/interfaces/cli` and future documentation tooling
- Downstream: `internal/domain/policy`, `internal/domain/rule`

## Notes

- Entries are generated from shipped default-policy rule IDs and explanation templates, so catalog coverage stays aligned with the actually shipped rule surface.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
