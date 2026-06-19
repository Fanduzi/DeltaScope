# Scripts Module

Operational scripts for local DeltaScope workflows.

## Files

| File | Responsibility |
|------|---------------|
| verify_pg_manylinux_baseline.sh | Builds the converged Linux PG-capable binaries in a manylinux2014 container and fails if the Linux glibc baseline exceeds the approved threshold |
| verify_homebrew_cask.sh | Verifies rendered Homebrew cask files against the release version, darwin archive URLs, sha256 values, and binary stanza |
| verify_release_archive.sh | Verifies packaged release archives by checking checksums, required files, binary version output, PostgreSQL CLI smoke, and optional Linux glibc baseline |
| verify_release_version_surfaces.sh | Checks that source version constants, package docs, npm launcher package version, README install pins, release notes, release index links, and landing current-version surfaces all match VERSION |
| verify_release_dialect_hygiene.sh | Runs release-blocking MySQL, TiDB, and PostgreSQL default-policy dialect smoke checks against a built or extracted deltascope binary |
| verify_gitlab_codequality_output.sh | Validates `--format gitlab-codequality` JSON output contract against a built CLI binary (inline SQL path fallback, file path propagation, required fields, severity values, fingerprint format) |
| verify_source_location_fidelity.sh | Validates source location fidelity across GitHub Actions, SARIF, GitLab Code Quality, and TiDB SARIF outputs (statement-start line numbers, artifact/file paths, no empty path fallbacks) |
| verify_release_workflow_hygiene.sh | Validates release workflow Homebrew verification avoids noisy tolerated cleanup (`|| true`), uppercase tap tokens, and requires conditional probes and lowercase tap names — prevents successful workflows from carrying spurious `unavailable` error annotations |
| verify_release_consistency.py | Validates release semantic consistency: release sequence, residual census arithmetic, SQL corpus metrics, PG ALTER TABLE rule count, required rule IDs across EN/ZH surfaces, no-overclaim wording, and no-leak wording |
| test_verify_release_consistency.py | Unit tests for the release consistency checker |
| verify_docs_examples.py | Static, release-oriented drift check for current public docs/examples: catches stale DeltaScope commands, incomplete audit output-format inventories, GitHub Actions/GitLab CI workflow-shape drift, and release-version pins; does not execute docs snippets or call external services |
| test_verify_docs_examples.py | Unit tests for the docs/examples drift checker |
| test_cli_metadata_e2e.sh | Starts Docker fixtures, seeds TiDB, runs metadata-aware CLI e2e flows, and provides JSON assertion helpers |
| test_mcp_metadata_e2e.sh | Starts Docker MySQL/TiDB fixtures and runs the tagged MCP metadata-aware e2e smoke tests for direct and connection_ref paths |
| test_http_metadata_e2e.sh | Starts Docker MySQL/TiDB fixtures and runs the tagged HTTP metadata-aware e2e smoke tests against the live JSON API |
| test_cli_metadata_e2e_postgresql.sh | Builds the PG-capable CLI, starts PostgreSQL fixtures, and runs metadata-aware PostgreSQL CLI end-to-end coverage |
| test_http_metadata_e2e_postgresql.sh | Starts PostgreSQL fixtures and runs tagged HTTP metadata-aware PostgreSQL end-to-end tests against the live JSON API |
| test_mcp_metadata_e2e_postgresql.sh | Starts PostgreSQL fixtures and runs tagged MCP metadata-aware PostgreSQL end-to-end tests |

## Exports

- `verify_pg_manylinux_baseline.sh`
- `verify_homebrew_cask.sh`
- `verify_release_archive.sh`
- `test_cli_metadata_e2e.sh [mysql|tidb|all]`
- `test_mcp_metadata_e2e.sh [mysql|tidb|all]`
- `test_http_metadata_e2e.sh [mysql|tidb|all]`
- `test_cli_metadata_e2e_postgresql.sh`
- `test_http_metadata_e2e_postgresql.sh`
- `test_mcp_metadata_e2e_postgresql.sh`
- `make smoke-pg-cli-manylinux-baseline`
- `make verify-pg-linux-release-archive-cn VERSION=<tag-or-version>`
- `make test-e2e-cli`
- `make test-e2e-cli-mysql`
- `make test-e2e-cli-tidb`
- `make test-e2e-mcp-mysql`
- `make test-e2e-mcp-tidb`
- `make test-e2e-http-mysql`
- `make test-e2e-http-tidb`
- `make test-e2e-cli-postgresql`
- `make test-e2e-http-postgresql`
- `make test-e2e-mcp-postgresql`
- `make pg-unit-test-gates`
- `make pg-e2e-gates`
- `make pg-confidence-gates`
- `make release-surface-gates VERSION=<tag-or-version>`
- `make release-version-surface-gates VERSION=<tag-or-version>`
- `make release-version-contract-gates VERSION=<tag-or-version>`
- `make release-dialect-hygiene-gates`
- `make release-gitlab-codequality-smoke`
- `make release-source-location-smoke`
- `make release-workflow-hygiene-gates`
- `make release-consistency-test`
- `VERSION=vX.Y.Z python3 scripts/verify_release_consistency.py`
- `VERSION=vX.Y.Z python3 scripts/verify_docs_examples.py`
- `make docs-example-gates VERSION=vX.Y.Z`

## Dependencies
- Upstream: local developers, `Makefile`, and release-verification workflows
- Downstream: Docker Engine, Docker Compose, Python 3, Go toolchain, `docker/cli-e2e-compose.yaml`, and `quay.io/pypa/manylinux2014_x86_64`

## Notes

- The CLI e2e script builds `./cmd/deltascope` once per run, while the MCP and HTTP e2e scripts run tagged Go tests against the real server entrypoints.
- The Docker-backed suites are intentionally separate from `go test ./...`.
- The release archive verifier is the final package-level contract gate: cask/install-facing archives must contain version-matched PG-capable binaries before upload.
- Linux main-archive verification is executed inside the matching manylinux container so the verifier can execute the packaged Linux binaries instead of relying on the host OS.
- The Homebrew cask verifier ensures the tap update still points at the exact darwin release assets and checksums produced by the release jobs.
- The manylinux baseline verifier is the reusable gate for the converged Linux PG-capable binaries and enforces the approved glibc baseline before release packaging.
- The manylinux verifier and manylinux release packagers inherit host `HTTP_PROXY` / `HTTPS_PROXY` / `ALL_PROXY` plus Go module env like `GOPROXY` and `GOSUMDB`, so constrained networks can use local proxies or domestic mirrors without patching scripts.
- `make verify-pg-linux-release-archive-cn` is a local-only convenience wrapper that defaults to `GOPROXY=https://goproxy.cn,direct` and `GOSUMDB=off` before delegating to the normal Linux archive verifier.
- `verify_docs_examples.py` is a static, release-oriented checker: it scans curated public docs/examples for known drift patterns (stale commands, missing audit output formats, GitHub Actions/GitLab CI workflow shape, version pins) and never executes Markdown/YAML snippets or contacts external services. It runs via `make docs-example-gates VERSION=vX.Y.Z`, is wired into `release-surface-gates`, and is intentionally not part of `make test`.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
