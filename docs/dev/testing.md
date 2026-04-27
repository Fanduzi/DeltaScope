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

## Notes

- `go test ./...` is the default fast verification path.
- `make sql-corpus-gates` enforces the SQL corpus contract: every currently supported `rule_id × dialect` surface must have at least one corpus case.
- `make sql-corpus-report` prints the current supported-rule inventory: rule count, supported `rule_id × dialect` target count, covered count, corpus fixture counts by dialect, and deferred surfaces.
- That contract is intentionally narrower than “every policy key on every dialect”. The coverage gate tracks the current stable extractor/rule support surface, not theoretical future support.
- CLI metadata e2e targets require Docker, Go, and Python 3.
- MCP metadata e2e targets require Docker and Go.
- HTTP metadata e2e targets require Docker and Go.
- Release readiness should verify both the normal test path and the artifact/build path.

## Release Contract Gates

Run `make release-contract-gates VERSION=vX.Y.Z` before tagging a release. This target verifies source version constants, package docs, npm launcher package version, README install pins, release notes, release index links, landing current-version surfaces, local binary version output, npm launcher tests, GoReleaser configuration, default-policy dialect hygiene smoke, and GitLab Code Quality output contract smoke.

Use this alongside `make release-test-gates` when preparing a release. The GitHub release workflow also runs the contract gate for the tag version before publishing release assets.

### GitLab Code Quality Smoke

`make release-gitlab-codequality-smoke` validates the `--format gitlab-codequality` output against the GitLab Code Quality JSON contract. It requires no Docker and no GitLab API connection — it runs the built CLI binary locally and checks:

- Output is a valid JSON array with at least one issue
- Each issue has required fields (`description`, `check_name`, `fingerprint`, `severity`, `location.path`, `location.lines.begin`)
- `fingerprint` is a 64-character hex string
- `severity` is one of `info`, `minor`, `major`, `critical`, `blocker`
- Inline SQL (`--sql`) produces `location.path` = `deltascope.sql`
- File path (`--file`) propagates the user-supplied path into `location.path`

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
