# deltascope Command Module

Process entrypoint for the `deltascope` CLI.

## Files

| File | Responsibility |
|------|---------------|
| main.go | Starts the Cobra-based CLI adapter package |

## Exports

- No exported Go API; this directory builds the executable entrypoint.

## Notes

- `deltascope --version` prints only the semantic version for scripts.
- `deltascope version` prints the ASCII logo plus the semantic version for humans.
- The command surface currently includes `audit`, `config init`, and `version`.

## Dependencies
- Upstream: shell users, future package build tooling
- Downstream: `internal/interfaces/cli`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
