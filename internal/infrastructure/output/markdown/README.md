# Markdown Output Module

Default human-readable renderer for internal audit results.

## Files

| File | Responsibility |
|------|---------------|
| render.go | Formats audit results into deterministic Markdown |
| render_test.go | Verifies summary, statement findings, and global finding rendering |

## Exports

- `Render(result report.Result) ([]byte, error)`

## Dependencies
- Upstream: CLI and future API adapters that need human-readable output
- Downstream: `internal/domain/report`, `internal/domain/rule`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
