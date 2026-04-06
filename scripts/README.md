# Scripts Module

Operational scripts for local DeltaScope workflows.

## Files

| File | Responsibility |
|------|---------------|
| verify_pg_manylinux_baseline.sh | Builds `deltascope-pg` in a manylinux2014 container and fails if the Linux glibc baseline exceeds the approved threshold |
| package_pg_cli_release.sh | Packages the verified manylinux `deltascope-pg` binary into the only approved public PG v1 release archive and emits a checksum sidecar |
| test_cli_metadata_e2e.sh | Starts Docker fixtures, seeds TiDB, runs metadata-aware CLI e2e flows, and provides JSON assertion helpers |
| test_mcp_metadata_e2e.sh | Starts Docker MySQL/TiDB fixtures and runs the tagged MCP metadata-aware e2e smoke tests for direct and connection_ref paths |
| test_http_metadata_e2e.sh | Starts Docker MySQL/TiDB fixtures and runs the tagged HTTP metadata-aware e2e smoke tests against the live JSON API |

## Exports

- `verify_pg_manylinux_baseline.sh`
- `package_pg_cli_release.sh`
- `make package-pg-cli-release VERSION=<tag-or-version>`
- `test_cli_metadata_e2e.sh [mysql|tidb|all]`
- `test_mcp_metadata_e2e.sh [mysql|tidb|all]`
- `test_http_metadata_e2e.sh [mysql|tidb|all]`
- `make smoke-pg-cli-manylinux-baseline`
- `make test-e2e-cli`
- `make test-e2e-cli-mysql`
- `make test-e2e-cli-tidb`
- `make test-e2e-mcp-mysql`
- `make test-e2e-mcp-tidb`
- `make test-e2e-http-mysql`
- `make test-e2e-http-tidb`

## Dependencies
- Upstream: local developers, `Makefile`, and release-verification workflows
- Downstream: Docker Engine, Docker Compose, Python 3, Go toolchain, `docker/cli-e2e-compose.yaml`, and `quay.io/pypa/manylinux2014_x86_64`

## Notes

- The CLI e2e script builds `./cmd/deltascope` once per run, while the MCP and HTTP e2e scripts run tagged Go tests against the real server entrypoints.
- The Docker-backed suites are intentionally separate from `go test ./...`.
- The manylinux baseline verifier is Phase 7 Slice 3's reusable gate for the only public PG v1 artifact, `deltascope-pg`.
- The PG release packager is Phase 7 Slice 4's only public PostgreSQL publish helper; it packages `deltascope-pg` and does not emit `deltascope-server-pg` or `deltascope-mcp-pg` archives.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
