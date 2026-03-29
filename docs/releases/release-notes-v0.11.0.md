# DeltaScope v0.11.0 Release Notes

## Overview

DeltaScope `v0.11.0` delivers two major capabilities: a GitHub Actions Composite Action that brings SQL audit into CI/CD pipelines in minutes, and HTTP service hardening with structured logging, health/readiness endpoints, and graceful shutdown for production deployments.

## What's Changed

### GitHub Actions Composite Action

DeltaScope is now a first-class GitHub Actions citizen. Add SQL audit to any workflow with a single `uses:` reference:

```yaml
- uses: Fanduzi/DeltaScope@v0.11.0
  with:
    files: migrations/*.sql
    fail-on: blocker
    token: ${{ secrets.GITHUB_TOKEN }}
```

The action:
- Downloads the correct binary for the runner architecture (`linux/amd64` or `linux/arm64`) from GitHub Releases
- Expands the `files` glob and runs `deltascope audit` per file
- Sets `has-issues: true/false` and `result` (JSON array) as step outputs
- Posts an audit summary as a PR comment when `token` is provided
- Exits non-zero when findings reach the `fail-on` threshold

See `docs/examples/github-actions.yml` for a complete workflow and `docs/examples/gitlab-ci.yml` for a GitLab CI equivalent.

### HTTP Service: Structured JSON Request Logging

Every HTTP request now emits a structured JSON log line:

```json
{"timestamp":"2026-03-30T10:00:00Z","level":"info","msg":"http request","method":"POST","path":"/v1/audit","status":200,"duration_ms":12,"request_id":"a1b2c3d4"}
```

The `timestamp` reflects request arrival time, making latency measurements accurate.

### HTTP Service: Health and Readiness Endpoints

Two new endpoints support container orchestration:

| Endpoint | Purpose | Response |
|----------|---------|----------|
| `GET /healthz` | Liveness probe — is the process alive? | `{"status":"ok"}` |
| `GET /readyz` | Readiness probe — is the engine ready to serve? | `{"status":"ready"}` |

Both endpoints bypass API-key auth and rate limiting.

### HTTP Service: Graceful Shutdown

The server now handles `SIGTERM` and `SIGINT` cleanly:

- In-flight requests are drained before exit
- Shutdown timeout is 15 seconds (configurable with `--shutdown-timeout`)
- Compatible with Kubernetes `terminationGracePeriodSeconds`

## Install / Upgrade

**macOS (recommended):**

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

Or upgrade:

```bash
brew upgrade --cask deltascope
```

**Linux:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.11.0/install.sh | \
  DELTASCOPE_VERSION=v0.11.0 sh
```

**MCP launcher (no install required):**

```bash
npx -y @fanduzi/deltascope-mcp --version
```

## Compatibility

No breaking changes. All existing CLI flags, HTTP API contracts, MCP tools, and policy config files from v0.10.0 remain unchanged.
