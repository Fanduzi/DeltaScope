# JSON Output Module

Machine-oriented JSON renderer for internal audit results.

## Files

| File | Responsibility |
|------|---------------|
| render.go | Marshals audit results into stable indented JSON |
| render_test.go | Verifies JSON field stability and round-trip decoding |

## Exports

- `Render(result report.Result) ([]byte, error)`

## Dependencies
- Upstream: CLI and future API adapters that need machine-readable output
- Downstream: `internal/domain/report`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
