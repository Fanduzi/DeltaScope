# Scripts Module

Operational scripts for local DeltaScope workflows.

## Files

| File | Responsibility |
|------|---------------|
| test_cli_metadata_e2e.sh | Starts Docker fixtures, seeds TiDB, runs metadata-aware CLI e2e flows, and provides JSON assertion helpers |
| test_mcp_metadata_e2e.sh | Starts Docker MySQL/TiDB fixtures and runs the tagged MCP metadata-aware e2e smoke tests for direct and connection_ref paths |
| test_http_metadata_e2e.sh | Starts Docker MySQL/TiDB fixtures and runs the tagged HTTP metadata-aware e2e smoke tests against the live JSON API |

## Exports

- `test_cli_metadata_e2e.sh [mysql|tidb|all]`
- `test_mcp_metadata_e2e.sh [mysql|tidb|all]`
- `test_http_metadata_e2e.sh [mysql|tidb|all]`
- `make test-e2e-cli`
- `make test-e2e-cli-mysql`
- `make test-e2e-cli-tidb`
- `make test-e2e-mcp-mysql`
- `make test-e2e-mcp-tidb`
- `make test-e2e-http-mysql`
- `make test-e2e-http-tidb`

## Dependencies
- Upstream: local developers, `Makefile`, and release-verification workflows
- Downstream: Docker Engine, Docker Compose, Python 3, Go toolchain, and `docker/cli-e2e-compose.yaml`

## Notes

- The CLI e2e script builds `./cmd/deltascope` once per run, while the MCP and HTTP e2e scripts run tagged Go tests against the real server entrypoints.
- The Docker-backed suites are intentionally separate from `go test ./...`.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
