# Scripts Module

Operational scripts for local DeltaScope workflows.

## Files

| File | Responsibility |
|------|---------------|
| verify_pg_manylinux_baseline.sh | Builds the converged Linux PG-capable binaries in a manylinux2014 container and fails if the Linux glibc baseline exceeds the approved threshold |
| verify_homebrew_cask.sh | Verifies rendered Homebrew cask files against the release version, darwin archive URLs, sha256 values, and binary stanza |
| verify_release_archive.sh | Verifies packaged release archives by checking checksums, required files, binary version output, PostgreSQL CLI smoke, and optional Linux glibc baseline |
| verify_release_version_surfaces.sh | Checks that source version constants, package docs, npm launcher and MCP Registry versions/names, README install pins, release notes, release index links, and landing current-version surfaces all match VERSION |
| verify_release_dialect_hygiene.sh | Runs release-blocking MySQL, TiDB, and PostgreSQL default-policy dialect smoke checks against a built or extracted deltascope binary |
| verify_gitlab_codequality_output.sh | Validates `--format gitlab-codequality` JSON output contract against a built CLI binary (inline SQL path fallback, file path propagation, required fields, severity values, fingerprint format) |
| verify_source_location_fidelity.sh | Validates source location fidelity across GitHub Actions, SARIF, GitLab Code Quality, and TiDB SARIF outputs (statement-start line numbers, artifact/file paths, no empty path fallbacks) |
| verify_release_workflow_hygiene.sh | Validates release workflow Homebrew verification avoids noisy tolerated cleanup (`|| true`), uppercase tap tokens, and requires conditional probes and lowercase tap names — prevents successful workflows from carrying spurious `unavailable` error annotations. Also invokes the structural Homebrew trust contract checker, the release workflow provenance DAG checker, and the recovery workflow provenance contract checker |
| verify_release_workflow_hygiene.py | Structural checker for Homebrew trust workflow contract: verifies both release.yml and release-recover.yml contain the exact trust command before install in verify-homebrew-cask-install job |
| test_verify_release_workflow_hygiene.py | Unit tests for the Homebrew trust workflow contract checker |
| verify_release_consistency.py | Validates release semantic consistency: versioned release facts (including v0.510.4); release sequence and every non-archive landing card with a `release-card` class token's paired version, i18n keys, and EN/ZH release-note hrefs; residual census arithmetic; SQL corpus metrics; the current supported rule-and-dialect fixture coverage label; PG ALTER TABLE rule count; required rule IDs across EN/ZH surfaces; no-overclaim wording; and no-leak wording |
| test_verify_release_consistency.py | Unit tests for the release consistency checker, including landing-card field pairing and the current SQL corpus metric label |
| test_pr_workflow_contract.py | Regression test that the pull-request / `main` workflow keeps unguarded `go test ./...`, `make pg-unit-test-gates`, and SQL corpus gates, rejects step/job `if:` guards, and does not fold Docker metadata e2e into those unit jobs |
| verify_docs_examples.py | Static, release-oriented drift check for current public docs/examples: catches stale DeltaScope commands, frozen CLI `"loaded": 371` examples that treated Loaded as Catalog size, incomplete audit output-format inventories, each PostgreSQL metadata-aware audit command without `--database` or `--schema`, GitHub Actions/GitLab CI workflow-shape drift, release-version pins (including the `action.yml` `uses: Fanduzi/deltascope@vX.Y.Z` example pin; never a floating `@v0`), CLI audit metadata/connection flag inventory (including `--metadata-connect-timeout` and PostgreSQL `--database`), SDK `Result` shape tokens (`Unsupported`, `Diagnostics`, `ErrUnsupportedStatement`), MCP source-build version drift (rejects a literal `defaults to vX.Y.Z` in favor of `pkg/deltascope.DefaultVersion`), and unpinned active MCP launcher surfaces (requires `npx -y --prefer-online @fanduzi/deltascope-mcp@latest` in the curated root README, MCP recipe, npm package README, landing, and `.mcp.json` surfaces). GitHub/GitLab checks are YAML-structure-aware via stdlib only (no PyYAML): they require `permissions.contents: read` in a real permissions block and `artifacts.reports.codequality` in a real artifacts block, not merely in prose/comments, and every finding carries a 1-based line number anchored to the relevant section. Does not execute docs snippets or call external services |
| test_verify_docs_examples.py | Unit tests for the docs/examples drift checker |
| test_cli_metadata_e2e.sh | Starts Docker fixtures, seeds TiDB, runs metadata-aware CLI e2e flows including MySQL/TiDB database/schema aliases, MODIFY nullability transitions, and DML target-table existence cases, and provides JSON assertion helpers |
| test_mcp_metadata_e2e.sh | Starts Docker MySQL/TiDB fixtures and runs the tagged MCP metadata-aware e2e smoke tests for direct and connection_ref paths |
| test_http_metadata_e2e.sh | Starts Docker MySQL/TiDB fixtures and runs the tagged HTTP metadata-aware e2e smoke tests against the live JSON API |
| test_cli_metadata_e2e_postgresql.sh | Builds the PG-capable CLI, starts PostgreSQL fixtures, and runs metadata-aware PostgreSQL CLI end-to-end coverage with explicit database/schema selection. Includes the ALTER INDEX ... RENAME contract: rule `ddl.pg.alter_index.rename.notice` with process exit 0 |
| test_http_metadata_e2e_postgresql.sh | Starts PostgreSQL fixtures and runs tagged HTTP metadata-aware PostgreSQL end-to-end tests against the live JSON API |
| test_mcp_metadata_e2e_postgresql.sh | Starts PostgreSQL fixtures and runs tagged MCP metadata-aware PostgreSQL end-to-end tests |
| test_cli_tls_e2e.sh | Starts Docker TLS fixtures with dynamic ports, builds CLI, runs 12 CLI TLS E2E cases (MySQL 8.4 + PostgreSQL 17 x audit + query-access x trusted/untrusted/hostname-mismatch); the trusted MySQL Query Access case also proves an unqualified relation binds through `--schema`. Fail-closed cleanup: force-removes and re-verifies absence of containers/networks/volumes/workspace; success path fails on residuals; original nonzero exit preserved. Runs in PR/push CI via `.github/workflows/cli-tls-e2e.yml` and in release gate via `make release-test-gates` |
| test_cli_tls_e2e_regression.sh | Verifies CLI TLS fixture lifecycle: tolerates externally pre-occupied legacy ports ([external], owner untouched), occupies free legacy ports with harness-owned Python holders ([owned]), re-asserts full legacy-port coverage before each run, kills only owned holders and requires only owned ports released on every exit path while requiring pre-existing ports to remain listening during cleanup, and asserts no residual Docker resources or workspace files after normal and intentional-failure runs, plus Docker availability policy |
| release_from_candidate.sh | Local release orchestrator. Accepts VERSION and --dry-run. Mutating sequence: preflight → release gates → pretag-candidate-gate → annotated tag → posttag-candidate-gate → push main → push tag. Stops on any failure; no automatic deletion, retry, or force push. Dry-run is read-only. |
| test_release_from_candidate.sh | Temporary-repository tests for the orchestrator: valid dry-run path, missing RC, candidate drift, dirty tree, local/remote tag collision, dry-run no-tag/no-push evidence |
| test_install.sh | Hermetic curl/wget contract tests for API discovery, latest-release redirect fallback, bounded failure hints, pinned installer bypass, and unprefixed semver tag prefixing |
| test_verify_release_workflow_provenance.py | Release workflow provenance contract checker: parses needs DAG, verifies provenance job exists with read-only permissions, fetches origin/main, runs posttag-candidate-gate with RELEASE_MAIN_REF, all mutation jobs are transitively downstream, and publish-mcp-launcher-package depends on the four platform-build jobs rather than Homebrew publish or install verification |
| test_verify_release_workflow_provenance_negative.py | Negative tests for the provenance contract checker: missing provenance job, missing dependency, write permissions, missing fetch, missing RELEASE_MAIN_REF, independent publisher bypass, npm waiting on Homebrew verification, npm missing a required platform-build need, plus a positive control |
| verify_release_recover_workflow_provenance.py | Recovery workflow provenance contract checker for release-recover.yml: fail-closed `refs/heads/main` dispatch-ref guard as the first preflight step (with `exit 1` inside the mismatch branch), read-only preflight permissions with no publisher secrets, full-history input-tag checkout, origin/main fetch, same-step RELEASE_MAIN_REF post-tag candidate gate before any external release-state work, tag_target_sha resolved from the input tag's peeled commit and exported after the gate, publisher jobs pinned to exactly the verified SHA, all mutation jobs transitively downstream of preflight, no historical-tag bypass literals, and no inline workflow-input interpolation in run scripts |
| test_verify_release_recover_workflow_provenance.py | Adversarial tests for the recovery provenance checker: 30 static tampering cases (guard removal/weakening/reordering/inversion/dead-code exit, checkout drift, env/run split, permissions widening, DAG bypasses, historical-tag allowlist, wrong SHA-resolve target, inline input interpolation) plus a positive control and behavior tests executing the real guard step under branch/tag dispatch refs and malformed version input |
| test_release_recovery_contract.sh | Hermetic recovery contract test in temporary git repos: future-valid RC chain admits and reaches the publisher stub; v0.240.0 (no provenance) and v0.460.0 (broken chain) fail closed before publisher work; lightweight and off-main tags fail closed; hygiene wiring proven load-bearing via a mutating test against a contract-violating workflow fixture. No network, GitHub, or npm access |

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
- `make test-e2e-cli-tls`
- `make test-e2e-http-tls`
- `make test-e2e-cli-tls-regression`
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
- `bash scripts/release_from_candidate.sh VERSION [--dry-run]`
- `make release-from-candidate VERSION=vX.Y.Z`
- `make release-from-candidate-dry-run VERSION=vX.Y.Z`
- `make release-provenance-contract-test`
- `make release-provenance-negative-test`
- `make release-recovery-contract-test`
- `make release-recovery-provenance-negative-test`
- `make pretag-candidate-test`
- `make posttag-candidate-test`
- `bash scripts/test_install.sh`

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
- The release orchestrator (`release_from_candidate.sh`) is the documented local release path. It runs preflight checks, release gates, the pre-tag candidate gate, creates an annotated tag, runs the post-tag gate, then pushes main and the tag separately. Dry-run executes only read-only preflight and pre-tag verification.
- The post-tag verifier (`verify_release_tag_candidate.sh`) accepts an explicit trusted main ref via `RELEASE_MAIN_REF` env var (defaults to `main` for local use). In CI, pass `refs/remotes/origin/main` after fetching origin; fail closed if the ref cannot resolve.
- The provenance contract checker (`test_verify_release_workflow_provenance.py`) parses the release workflow needs DAG and verifies every mutation job (GoReleaser, GitHub Release, npm publish, Homebrew cask push) is transitively downstream of the provenance job. It also requires `publish-mcp-launcher-package` to exist, to depend directly on the four platform-build jobs, and not to be transitively downstream of Homebrew publish or install verification.
- The recovery provenance checker (`verify_release_recover_workflow_provenance.py`) applies the same fail-closed discipline to `release-recover.yml`: recovery may only publish after the input tag proves a valid release-candidate chain against fetched `origin/main`, and publishers may only check out the verified peeled tag target SHA. It runs inside `verify_release_workflow_hygiene.sh` and `make release-recovery-contract-test`. Historical tags without candidate provenance (v0.240.0, v0.460.0) fail admission by design; there is no allowlist.
- `verify_docs_examples.py` is a static, release-oriented checker: it scans curated public docs/examples for known drift patterns (stale commands, missing audit output formats, PostgreSQL metadata-aware audit commands without `--database` or `--schema`, GitHub Actions/GitLab CI workflow shape, version pins including the `action.yml` `uses: Fanduzi/deltascope@vX.Y.Z` example pin, CLI audit metadata flag inventory including PostgreSQL `--database`, SDK `Result` shape tokens, MCP source-build version literal, and the active canonical MCP launcher spec) and never executes Markdown/YAML snippets or contacts external services. The GitHub/GitLab shape checks use a small stdlib-only YAML structural extractor (no PyYAML dependency) so that `permissions.contents: read` and `artifacts.reports.codequality` are verified in their real block position rather than as loose substrings; findings always carry a 1-based line number anchored to the nearest relevant section (file-level findings use line 1, never 0). The CLI flag, per-command PostgreSQL metadata example, SDK field/sentinel, MCP version, action.yml uses pin, and active launcher checks are static guards read directly from their target files so the generic stale-command/severity scans keep their narrower scope. The launcher guard requires `npx -y --prefer-online @fanduzi/deltascope-mcp@latest` on the curated current root README, MCP recipe, npm package README, landing surfaces, and structured `.mcp.json` args, while historical decisions/releases/changelog remain outside the guard. The action.yml pin must be a published stable `vX.Y.Z` tag (not a floating `@v0`) and must match `VERSION` when that env is set. It runs via `make docs-example-gates VERSION=vX.Y.Z`, is wired into `release-surface-gates`, and is intentionally not part of `make test`. Canonical valid fixtures live under `scripts/testdata/docs_examples/` and are exercised by `scripts/test_verify_docs_examples.py`.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
