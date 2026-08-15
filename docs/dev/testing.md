# Testing

Local verification is intentionally split between fast default checks and slower Docker-backed metadata e2e coverage.

## Core Targets

```bash
make test
make sql-corpus-gates
make sql-corpus-report
make build
make build-cli
make build-server
make build-mcp
```

`make build` produces `bin/deltascope`, `bin/deltascope-server`, and `bin/deltascope-mcp`.
`make build-linux` produces `bin/deltascope-linux-amd64`, `bin/deltascope-server-linux-amd64`, and `bin/deltascope-mcp-linux-amd64`.
Local `make build` now produces PostgreSQL-capable `deltascope`, `deltascope-server`, and `deltascope-mcp` binaries by building with `CGO_ENABLED=1` and `-tags postgresql`.
`make build-linux` remains on the portable `CGO_ENABLED=0` path until the public release matrix converges on unified PostgreSQL-capable artifacts.

## Query Access Test Ownership

The unified SDK session is the intended exhaustive semantic owner for MySQL 5.7/8.0/8.4,
TiDB 8.5, and PostgreSQL 17. Until the milestone ledger identifies complete
unified replacement evidence, existing semantic rows remain retained. Add a new
SQL shape, capability profile, exact admission/result, detailed identity/catalog
probe, or user-SQL no-execution case at the unified SDK seam. The Query Access
corpus remains the offline semantic owner.

| Evidence | Owner | Do not replace with |
|---|---|---|
| Product/profile/SQL-shape semantics and detailed recording probes | Unified SDK | CLI or HTTP copies |
| Deprecated API notices, stubs, exact errors, validation priority, caller ownership, and per-target equivalence | Deprecated SDK compatibility tests | Unified-only assertions |
| Flags, TLS/session lifecycle, exit code, stdout/stderr, bounded CLI errors, and CLI no-leak | CLI | SDK result assertions |
| Request/status/body, registry, `connection_id`, authorization-before-dial, access logs, lifecycle, and HTTP no-leak | HTTP | SDK result assertions |
| One real route smoke per MySQL 8.4, TiDB 8.5, and PostgreSQL 17 family on each transport | CLI and HTTP real-binary/adapter tests | Recording tests alone |
| PostgreSQL syntax-envelope, foreign-table, and default/offline failures | SDK, CLI, and HTTP at their distinct boundaries | Another PostgreSQL negative |
| Absence of a Query Access tool | MCP surface contract | Any SDK or transport test |

Future changes add semantic breadth at the unified SDK seam. Do not delete an
existing semantic row until the ledger identifies its complete unified owner.
Add a transport case only when it observes a transport-owned sink, lifecycle,
configuration, authorization, or routing boundary. A new server version in an
existing transport configuration family needs SDK coverage, not copied transport
matrices. A new family or configuration needs one smoke per supported transport.

Before deleting a test or table row, update the milestone ledger in
`docs/plans/2026-08-15-query-access-test-ownership-consolidation-implementation.md`,
run its listed owner gates, and obtain read-only review. Similar SQL is never
sufficient: foreign-table, offline/default, per-sink no-leak,
authorization-before-dial, and per-target deprecated-API evidence are
non-substitutable.

## Metadata E2E

```bash
make test-e2e-cli
make test-e2e-cli-mysql
make test-e2e-cli-tidb
make test-e2e-mcp-mysql
make test-e2e-mcp-tidb
make test-e2e-http-mysql
make test-e2e-http-tidb
```

## TLS E2E

```bash
make test-e2e-cli-tls
make test-e2e-http-tls
make test-e2e-cli-tls-regression
```

`make test-e2e-cli-tls` runs the 12-case CLI TLS E2E suite covering MySQL 8.4 and PostgreSQL 17 with trusted CA, untrusted CA, and hostname mismatch scenarios for both `audit` and `query-access analyze`. This target runs in PR/push CI (`.github/workflows/cli-tls-e2e.yml` on `pull_request` and `push` to `main`) and is part of `make release-test-gates`. It fails closed when Docker is unavailable — Docker unavailability, test skips, or `--docker-optional` all fail the CI job.

`make test-e2e-http-tls` runs the HTTP TLS E2E suite independently.

`make test-e2e-cli-tls-regression` verifies fixture lifecycle: dynamic port allocation, cleanup after passing and failed runs, and Docker availability policy. The regression harness tracks all port-holder PIDs, verifies ports are released after cleanup, parses the machine-readable `CLI_TLS_E2E_PORTS` line from each run, and uses `lsof` to confirm no listener remains on any dynamic port after both normal and intentional-failure runs. It also asserts no residual Docker containers/networks/volumes or workspace files remain.

Cleanup in `test_cli_tls_e2e.sh` is fail-closed: leftover Docker resources are force-removed and re-verified absent; if residuals persist, the success path fails. The original nonzero test exit code is preserved (cleanup never masks a test failure as success).

### Prerequisites

- Docker Engine with Compose v2
- Go toolchain (for building the CLI)
- OpenSSL (for certificate generation)
- Python 3 (for regression harness port holders)

### Developer-Only Optional Mode

For local development when Docker is not available:

```bash
./scripts/test_cli_tls_e2e.sh --docker-optional
```

This skips the suite only when Docker is unavailable and `CI` is not set and `DELTASCOPE_CLI_TLS_E2E_REQUIRED` is not `1`. The optional mode is rejected in CI or when the required-mode marker is set.

## Notes

- `go test ./...` is the default fast verification path.
- `make sql-corpus-gates` enforces the SQL corpus contract: every currently supported `rule_id × dialect` surface must have at least one corpus case.
- `make sql-corpus-report` prints the current supported-rule inventory: rule count, supported `rule_id × dialect` target count, covered count, corpus fixture counts by dialect, and deferred surfaces.
- That contract is intentionally narrower than “every policy key on every dialect”. The coverage gate tracks the current stable extractor/rule support surface, not theoretical future support.
- `make ddl-census-report` prints the tracked DDL coverage census for MySQL, TiDB, and PostgreSQL. This is an inventory/reporting gate — it shows how many tracked DDL forms are finding-covered, silently normalized, explicitly unsupported, or parser-blocked for each dialect. It is not a full SQL grammar coverage claim. The current census informs future MySQL/TiDB DDL rule prioritization.
- `make ddl-parser-error-feasibility-report` prints the parser-error feasibility classification for all 29 tracked DDL parser-error cases across MySQL (15), TiDB (9), and PostgreSQL (5). Each case is classified into one bucket (`parser_upgrade_candidate`, `bounded_fallback_candidate`, `product_unsupported_or_inapplicable`, `unsafe_fallback_defer`, `needs_research`). This is a classification/report gate — it does not add parser support, rules, or fallback extraction.
- `make parser-error-unsupported-contract-test` runs the parser-error unsupported contract tests across application, SDK, CLI, HTTP, and MCP surfaces. This gate verifies that when the selected dialect parser cannot parse a tracked DDL statement, all public surfaces return a clear diagnostic stating that no audit was performed and no findings were inferred from unparsed SQL. This does not add parser support, fallback parsing, or new SQL audit rules. Parser-error counts are not reduced.
- `make unsupported-diagnostics-evidence-test` runs the unsupported diagnostics evidence contract tests across application, SDK, CLI, HTTP, and MCP surfaces. This gate verifies that parser-error and unsupported statement outcomes expose structured diagnostic evidence (`classification`, `reason`, `action_hint`, `audited`, `dialect`) without leaking raw SQL or parser internals. This does not add parser support, fallback parsing, or new SQL audit rules. Parser-error counts are not reduced.
- `make parser-upgrade-candidate-evidence-report` prints the parser-upgrade candidate evidence report. It delegates directly to `ddl-parser-error-feasibility-report` and shows all 29 tracked `parser_error` feasibility buckets across MySQL, TiDB, and PostgreSQL. This is a classification/report gate only — it does not add parser support, SQL audit rules, or fallback extraction, and it does not reduce `parser_error` counts. Expected bucket facts:
  - `parser_upgrade_candidate`: MySQL 5, TiDB 0, PostgreSQL 5, total 10
  - `bounded_fallback_candidate`: MySQL 1, TiDB 3, PostgreSQL 0, total 4
  - `product_unsupported_or_inapplicable`: MySQL 0, TiDB 6, PostgreSQL 0, total 6
  - `unsafe_fallback_defer`: MySQL 9, TiDB 0, PostgreSQL 0, total 9
  - `needs_research`: MySQL 0, TiDB 0, PostgreSQL 0, total 0
- CLI metadata e2e targets require Docker, Go, and Python 3.
- MCP metadata e2e targets require Docker and Go.
- HTTP metadata e2e targets require Docker and Go.
- Release readiness should verify both the normal test path and the artifact/build path.

## Release Contract Gates

Run `make release-contract-gates VERSION=vX.Y.Z` before tagging a release. This target verifies source version constants, package docs, npm launcher package version, README install pins, release notes, release index links, landing current-version surfaces, local binary version output, npm launcher tests, GoReleaser configuration, default-policy dialect hygiene smoke, and GitLab Code Quality output contract smoke.

`release-version-surface-gates` also runs the release semantic consistency checker, which validates: landing current/recent release sequence, residual census arithmetic, SQL corpus metrics, PostgreSQL ALTER TABLE rule count consistency across surfaces, required rule IDs across EN/ZH release notes/rules/matrix, no-overclaim wording, and no-leak wording. Use `make release-consistency-test` to run the checker unit tests independently.

Use this alongside `make release-test-gates` when preparing a release. The GitHub release workflow also runs the contract gate for the tag version before publishing release assets.

### Release Tag Annotation Guard

`make release-tag-annotation-test` runs offline unit tests for the tag annotation verifier (no tag required, safe to run pre-release).

`VERSION=vX.Y.Z make release-tag-annotation-gate` checks that an existing local tag is annotated (`git cat-file -t` returns `tag`), not lightweight. Run this after tagging, before or after pushing. The GitHub release workflow also runs this check as an early guard step — lightweight tags fail the build before any artifacts are published.

If the gate detects a lightweight tag, do not move or delete the published tag. Instead, decide whether a new patch release with a correctly annotated tag is warranted.

### GitLab Code Quality Smoke

`make release-gitlab-codequality-smoke` validates the `--format gitlab-codequality` output against the GitLab Code Quality JSON contract. It requires no Docker and no GitLab API connection — it runs the built CLI binary locally and checks:

- Output is a valid JSON array with at least one issue
- Each issue has required fields (`description`, `check_name`, `fingerprint`, `severity`, `location.path`, `location.lines.begin`)
- `fingerprint` is a 64-character hex string
- `severity` is one of `info`, `minor`, `major`, `critical`, `blocker`
- Inline SQL (`--sql`) produces `location.path` = `deltascope.sql`
- File path (`--file`) propagates the user-supplied path into `location.path`

This gate is included in `make release-contract-gates`.

### Source Location Fidelity Smoke

`make release-source-location-smoke` validates that the built CLI correctly propagates source locations (file path, statement-start line, column) through all CI renderer outputs. It requires no Docker — it runs the built CLI binary locally and checks:

- GitHub Actions: `file=<path>`, `line=9`, `col=1` for `dml.where.require`; no empty `file=,`
- SARIF: `artifactLocation.uri=<path>`, `startLine=9`, `startColumn=1` for `dml.where.require`
- GitLab Code Quality: `location.path=<path>`, `location.lines.begin=9` for `dml.where.require`
- TiDB SARIF: same assertions as MySQL SARIF with explicit `--dialect tidb`

The SQL fixture places `delete from users;` on line 9 inside a multi-statement migration file. If the progressive source mapper regresses to statement-index fallback, the line number would be 2 instead of 9 and the gate would fail.

This gate is included in `make release-contract-gates`.

### Homebrew Verification Hygiene

The release workflow runs a `verify-homebrew-cask-install` job on macOS that performs a real Homebrew install from the published tap. It verifies:

- The cask can be installed from `fanduzi/deltascope`
- `deltascope --version` contains the release tag
- The binary includes PostgreSQL support
- A PostgreSQL audit smoke passes

A successful run must not produce Homebrew `unavailable` error annotations. The cleanup logic uses conditional execution instead of `|| true`:

```bash
# CORRECT: conditional cleanup
if brew list --cask deltascope >/dev/null 2>&1; then
  brew uninstall --cask deltascope
fi

if brew tap | grep -Fxq "fanduzi/deltascope"; then
  brew untap fanduzi/deltascope
fi
```

`|| true` must not appear on cleanup commands. It swallows the exit code but not stderr; GitHub Actions still promotes stderr to error annotations on successful runs.

The tap/install/version-check/audit commands must also not use `|| true` — real failures must block the release.

`make release-workflow-hygiene-gates` (included in `release-contract-gates`) statically checks the release workflow for these violations.

### Homebrew Trust Workflow Contract

The `verify-homebrew-cask-install` job must contain the exact Homebrew cask trust command before the install command. This contract protects the trust sequence from silent removal during workflow edits.

**Required sequence in `verify-homebrew-cask-install` job:**
```bash
brew trust --cask fanduzi/deltascope/deltascope
brew install --cask deltascope
```

**Contract enforcement:**
- Both `release.yml` and `release-recover.yml` are checked
- Trust command must appear before install command in the same job
- Commands in other jobs, comments, or prose do not satisfy the contract
- The checker uses structural YAML parsing, not whole-file substring search

**What this gate does NOT do:**
- Execute workflows, Homebrew, or any external commands
- Access secrets, network, Docker, or npm
- Validate workflow syntax beyond the trust/install sequence
- Replace runtime Homebrew install verification

Run `python3 scripts/test_verify_release_workflow_hygiene.py` to verify the contract checker behavior.

## Release Recovery

See [release-recovery.md](release-recovery.md) for the failure matrix and recovery procedures when a release partially fails.

### Release Recovery Contract Gate

`make release-recovery-contract-test` is hermetic and static — it needs no GitHub Release, npm registry, network access, or historical repo tags. It runs:

- `scripts/test_release_recovery_contract.sh`: simulated recovery admission in temporary git repos. A future-valid `.release-candidate` chain passes the post-tag candidate gate and reaches the publisher stub; `v0.240.0` (no candidate provenance) and `v0.460.0` (broken candidate chain) fail closed before any publisher work; lightweight and off-main tags fail closed. The hygiene-script wiring is proven load-bearing with a mutating test: a contract-violating workflow fixture fails under the original hygiene script and passes under a copy with the checker invocation removed.
- `scripts/verify_release_recover_workflow_provenance.py`: static provenance contract checks on `.github/workflows/release-recover.yml` (fail-closed `refs/heads/main` dispatch guard as first step with `exit 1` inside the mismatch branch, read-only preflight permissions, full-history tag checkout, `origin/main` fetch, same-step `RELEASE_MAIN_REF` post-tag gate before external release-state work, `tag_target_sha` resolved from the input tag's peeled commit and exported after the gate, publishers pinned to the verified SHA, all mutation jobs transitively downstream of preflight, no historical-tag bypass, no inline `${{ inputs.* }}` interpolation in run scripts).
- `scripts/test_verify_release_recover_workflow_provenance.py`: the adversarial checker suite (guard tampering including inverted and dead-code guards, publisher checkout drift, permissions widening, DAG bypasses, historical-tag exceptions, wrong SHA-resolve target, inline input interpolation) plus behavior tests that execute the real guard step under branch/tag dispatch refs and malformed version input.
- Static grep checks for the dry-run contract markers (`dry_run` input, Homebrew/npm dry-run markers, `!inputs.dry_run` guards, `GH_TOKEN` preflight wiring).

This target does not dispatch any workflow and does not contact GitHub or npm. The read-only operator diagnostic `make release-recovery-preflight VERSION=vX.Y.Z` still exists for inspecting real release asset state, but it is not part of the contract gate.
