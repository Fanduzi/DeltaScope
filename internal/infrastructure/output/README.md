# Output Infrastructure Module

Rendering adapters for turning internal audit results into transport-friendly formats.

## Files

| File | Responsibility |
|------|---------------|
| markdown/README.md | Documents the Markdown renderer module |
| json/README.md | Documents the JSON renderer module |
| githubactions/README.md | Documents the GitHub Actions annotation renderer module |
| githubsummary/README.md | Documents the GitHub Actions job-summary renderer module |

## Exports

- Package boundary only; concrete renderers live in child modules

## Dependencies
- Upstream: `internal/interfaces/cli`, future HTTP API and MCP adapters
- Downstream: `internal/domain/report`, `internal/domain/rule`, `internal/domain/rule/catalog`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
