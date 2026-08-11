# deltascope Command Module

Process entrypoint for the `deltascope` CLI.

## Files

| File | Responsibility |
|------|---------------|
| main.go | Starts the Cobra-based CLI adapter package |
| main_e2e_postgresql_query_access_test.go | Verifies Docker-backed PG17 COUNT(1) query-access behavior through the CLI surface, including foreign-table negative coverage that must remain fail-closed |

## Exports

- No exported Go API; this directory builds the executable entrypoint.

## Notes

- `deltascope --version` prints only the semantic version for scripts.
- `deltascope version` prints the ASCII logo plus the semantic version for humans.
- The command surface now includes `audit`, `rules list/show/search`, `config init/lint/show-default`, `capabilities`, and `version`.
- `deltascope audit` supports both offline and metadata-aware runs with MySQL-style connection flags.

## Dependencies
- Upstream: shell users, future package build tooling
- Downstream: `internal/interfaces/cli`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
