# SARIF Output Module

Renders audit results as SARIF 2.1.0 JSON.

## Files

| File | Responsibility |
|------|----------------|
| `render.go` | Converts findings and located parser-error diagnostics into SARIF rules, results, and physical locations |
| `render_test.go` | Verifies SARIF schema, finding levels, diagnostic locations, rule metadata, and no-leak behavior |

## Exports

- `Render(report.Result, Options) ([]byte, error)`
- `Options` — optional source artifact path

## Dependencies

- Upstream: CLI audit format dispatch
- Downstream: `internal/domain/report`, `internal/domain/rule`, `internal/domain/spec`

## Update Rule

- If members/interfaces/dependencies change, update this file in the same change.
