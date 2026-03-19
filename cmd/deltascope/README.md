# deltascope Command Module

Process entrypoint for the `deltascope` CLI.

## Files

| File | Responsibility |
|------|---------------|
| main.go | Starts the CLI adapter package |

## Exports

- No exported Go API; this directory builds the executable entrypoint.

## Dependencies
- Upstream: shell users, future package build tooling
- Downstream: `internal/interfaces/cli`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
