# GitLab Code Quality Output Module

Renders audit results as GitLab Code Quality JSON.

## Files

| File | Responsibility |
|------|----------------|
| `render.go` | Converts findings and located parser-error diagnostics into deterministic GitLab Code Quality issues |
| `render_test.go` | Verifies finding mapping, fingerprints, locations, no-leak behavior, and parser-error issues |

## Exports

- `Render(report.Result, Options) ([]byte, error)`
- `Options` — optional source path; defaults to `deltascope.sql`

## Dependencies

- Upstream: CLI audit format dispatch
- Downstream: `internal/domain/report`, `internal/domain/rule`, `internal/domain/spec`

## Update Rule

- If members/interfaces/dependencies change, update this file in the same change.
