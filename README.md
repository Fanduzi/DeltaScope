<div align="center">

# DeltaScope

[![Release](https://img.shields.io/github/v/release/Fanduzi/DeltaScope?display_name=tag)](https://github.com/Fanduzi/DeltaScope/releases)
![Platform](https://img.shields.io/badge/platform-darwin%20%7C%20linux-blue)
![Go Version](https://img.shields.io/badge/go-1.26.1-00ADD8?logo=go)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

English | [中文](README_ZH.md) | [Changelog](CHANGELOG.md) | [Security](SECURITY.md) | [License](LICENSE) | [Release Notes](docs/releases/release-notes-v0.7.0.md)
</div>

DeltaScope is an offline-first SQL audit engine for MySQL and TiDB. It gives DBAs, application engineers, CI pipelines, and AI agents one consistent way to review DDL and DML before they reach a database.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/main/install.sh | sh
```

Pin a specific release:

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.7.0/install.sh | \
  DELTASCOPE_VERSION=v0.7.0 sh
```

## MCP Quick Start

Launcher requirements:

- Node.js 20 or newer
- supported native targets: `darwin` or `linux`, `amd64` or `arm64`

Recommended launcher:

```bash
claude mcp add --scope user deltascope -- npx -y @fanduzi/deltascope-mcp
codex mcp add deltascope -- npx -y @fanduzi/deltascope-mcp
```

For raw stdio TOML, native `deltascope-mcp`, direct connection, `connection_ref`, proxy setup, and common errors, see [Use DeltaScope MCP](docs/recipe/use-deltascope-mcp.md).

## More Docs

- [Recipes](docs/recipe/README.md)
- [Dev docs](docs/dev/README.md)
- [Reference docs](docs/reference/README.md)
- [Audit SQL with metadata](docs/recipe/audit-sql-with-metadata.md)
- [Review DDL before migration](docs/recipe/review-ddl-before-migration.md)
- [Guard DML in CI](docs/recipe/guard-dml-in-ci.md)
- [Use with AI agents](docs/recipe/use-with-ai-agents.md)
- [Inspect rules and config](docs/recipe/inspect-rules-and-config.md)
- [Troubleshoot metadata-aware audit](docs/recipe/troubleshoot-metadata-aware-audit.md)

## Documentation

- [Admin docs](docs/admin/README.md)
- [Concept docs](docs/concept/README.md)
- [Dev docs](docs/dev/README.md)
- [Reference docs](docs/reference/README.md)
- [Audit capability matrix](docs/reference/audit-capability-matrix.md)

## Developer Workflows

- `make test` runs `go test ./...`
- `make build` produces all local binaries under `bin/`
- `make build-linux` produces Linux amd64 binaries under `bin/`
- `make test-e2e-cli` runs the Docker-backed metadata CLI smoke suite
- [docs/dev/testing.md](docs/dev/testing.md) covers the full target set

## HTTP Service

Run the HTTP adapter over the same audit engine:

```bash
deltascope-server -listen 127.0.0.1:8083
```

Endpoints:

- `GET /healthz`
- `GET /version`
- `POST /v1/audit`

## Library Usage

```go
result, err := deltascope.Audit(ctx, deltascope.Request{
    SQL:     "delete from users",
    Dialect: deltascope.DialectMySQL,
})
```

The stable public API lives in [pkg/deltascope](pkg/deltascope/README.md).

## Architecture

DeltaScope keeps one audit path and exposes it through multiple entrypoints. Product-level and implementation-level diagrams live in [docs/concept/architecture.md](docs/concept/architecture.md) and [docs/dev/architecture.md](docs/dev/architecture.md).

### Modules

| Module | Description | Doc |
|--------|-------------|-----|
| `cmd/deltascope` | CLI process entrypoint | [README](cmd/deltascope/README.md) |
| `cmd/deltascope-server` | HTTP service entrypoint | [README](cmd/deltascope-server/README.md) |
| `cmd/deltascope-mcp` | MCP service entrypoint | [README](cmd/deltascope-mcp/README.md) |
| `internal/interfaces` | Transport adapter namespace | [README](internal/interfaces/README.md) |
| `internal/interfaces/cli` | CLI adapter layer | [README](internal/interfaces/cli/README.md) |
| `internal/interfaces/http` | HTTP adapter layer | [README](internal/interfaces/http/README.md) |
| `internal/interfaces/mcp` | MCP adapter layer | [README](internal/interfaces/mcp/README.md) |
| `internal/application` | Use-case orchestration layer | [README](internal/application/README.md) |
| `internal/application/audit` | Application parse/audit orchestration | [README](internal/application/audit/README.md) |
| `internal/application/auditmeta` | Shared metadata-aware audit preparation | [README](internal/application/auditmeta/README.md) |
| `internal/application/policy` | Application policy loader | [README](internal/application/policy/README.md) |
| `internal/domain` | Core domain types and rules | [README](internal/domain/README.md) |
| `internal/domain/spec` | Normalized statement specifications | [README](internal/domain/spec/README.md) |
| `internal/domain/rule` | Rule findings and severity model | [README](internal/domain/rule/README.md) |
| `internal/domain/rule/catalog` | Explanation-oriented shipped rule catalog | [README](internal/domain/rule/catalog/README.md) |
| `internal/domain/rule/ddl` | DDL rule catalog | [README](internal/domain/rule/ddl/README.md) |
| `internal/domain/rule/dml` | DML rule catalog | [README](internal/domain/rule/dml/README.md) |
| `internal/domain/policy` | Policy configuration model | [README](internal/domain/policy/README.md) |
| `internal/domain/report` | Audit result aggregation and verdicts | [README](internal/domain/report/README.md) |
| `internal/infrastructure` | Infrastructure adapter layer | [README](internal/infrastructure/README.md) |
| `internal/infrastructure/parser` | Parser adapter namespace | [README](internal/infrastructure/parser/README.md) |
| `internal/infrastructure/parser/tidb` | TiDB parser adapter | [README](internal/infrastructure/parser/tidb/README.md) |
| `internal/infrastructure/config/viper` | YAML config adapter | [README](internal/infrastructure/config/viper/README.md) |
| `internal/infrastructure/metadata/mysql` | Metadata provider for MySQL/TiDB-compatible engines | [README](internal/infrastructure/metadata/mysql/README.md) |
| `internal/infrastructure/output` | Output renderer namespace | [README](internal/infrastructure/output/README.md) |
| `internal/infrastructure/output/markdown` | Markdown renderer | [README](internal/infrastructure/output/markdown/README.md) |
| `internal/infrastructure/output/json` | JSON renderer | [README](internal/infrastructure/output/json/README.md) |
| `configs` | Example configuration files | [README](configs/README.md) |
| `pkg/deltascope` | Stable public package surface | [README](pkg/deltascope/README.md) |
