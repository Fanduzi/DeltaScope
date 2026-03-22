# Scripts Module

Operational scripts for local DeltaScope workflows.

## Files

| File | Responsibility |
|------|---------------|
| test_cli_metadata_e2e.sh | Starts Docker fixtures, seeds TiDB, runs metadata-aware CLI e2e flows, and provides JSON assertion helpers |

## Exports

- `test_cli_metadata_e2e.sh [mysql|tidb|all]`
- `make test-e2e-cli`
- `make test-e2e-cli-mysql`
- `make test-e2e-cli-tidb`

## Dependencies
- Upstream: local developers, `Makefile`, and release-verification workflows
- Downstream: Docker Engine, Docker Compose, Python 3, Go toolchain, and `docker/cli-e2e-compose.yaml`

## Notes

- The script builds `./cmd/deltascope` once per run, then reuses that binary across all e2e assertions.
- The Docker-backed suite is intentionally separate from `go test ./...`.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
