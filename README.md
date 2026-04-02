<div align="center">

# DeltaScope

[![Release](https://img.shields.io/github/v/release/Fanduzi/DeltaScope?display_name=tag)](https://github.com/Fanduzi/DeltaScope/releases)
![Platform](https://img.shields.io/badge/platform-darwin%20%7C%20linux-blue)
![Go Version](https://img.shields.io/badge/go-1.26.1-00ADD8?logo=go)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

[![English](https://img.shields.io/badge/docs-English-blue)](README.md) [![简体中文](https://img.shields.io/badge/docs-简体中文-yellow)](README_ZH.md)

[![Changelog](https://img.shields.io/badge/Changelog-informational)](CHANGELOG.md) [![Security](https://img.shields.io/badge/Security-important)](SECURITY.md) [![License](https://img.shields.io/badge/License-blue)](LICENSE) [![Release Notes](https://img.shields.io/badge/Release_Notes-success)](docs/releases/README.md)
</div>

DeltaScope is an offline-first SQL audit engine for MySQL and TiDB. It gives DBAs, application engineers, CI pipelines, and AI agents one consistent way to review DDL and DML before they reach a database.

## Install

For macOS, prefer Homebrew. The repository installer script remains available as the generic portable installer for environments where Homebrew is not the right fit.

**macOS (recommended):**

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/main/install.sh | sh
```

Pin a specific release:

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.13.1/install.sh | \
  DELTASCOPE_VERSION=v0.13.1 sh
```

The published archive format is `deltascope_<version>_<os>_<arch>.tar.gz`. Development-oriented commands are documented under [Dev docs](docs/dev/README.md).

### Release Contract

Every tag produces archives named `deltascope_<version>_<os>_<arch>.tar.gz` containing the `deltascope`, `deltascope-server`, and `deltascope-mcp` binaries. The installer script, Homebrew Cask, and npm MCP launcher all resolve platform binaries from GitHub Release assets. See the npm package metadata for the current `@fanduzi/deltascope-mcp` package version.

## Quick Start

Audit a risky DML statement:

```bash
deltascope audit --sql "delete from users"
```

Example excerpt:

```text
Verdict: reject
Statements: 1
Blockers: 1
Warnings: 0
Notices: 0

Statement 1: DELETE
- [blocker] dml.where.require: UPDATE and DELETE statements must include a WHERE clause
```

Audit a `CREATE TABLE` statement:

```bash
deltascope audit --sql "create table tbl_users (id bigint unsigned not null auto_increment comment 'id', created_at datetime not null default current_timestamp comment 'created', updated_at datetime not null default current_timestamp on update current_timestamp comment 'updated', primary key (id)) comment='users' engine=InnoDB default charset=utf8mb4"
```

Example excerpt:

```text
Verdict: review
Statements: 1
Blockers: 0
Warnings: 1
Notices: 0

Statement 1: CREATE TABLE
- [warning] ddl.column.default.require: column "id" should define a default value
```

Audit a file:

```bash
deltascope audit --file ./migrations/20260328_add_column.sql
```

Use JSON output for CI or agents:

```bash
deltascope audit \
  --sql "create table tbl_users (id bigint unsigned not null auto_increment comment 'id', created_at datetime not null default current_timestamp comment 'created', updated_at datetime not null default current_timestamp on update current_timestamp comment 'updated', primary key (id)) comment='users' engine=InnoDB default charset=utf8mb4" \
  --format json \
  --fail-on warning
```

Example JSON shape:

```json
{
  "verdict": "review",
  "summary": {
    "statements": 1,
    "blockers": 0,
    "warnings": 1,
    "notices": 0
  },
  "statements": [ ... ],
  "context": {
    "mode": "offline",
    "dialect": "mysql",
    "dialect_source": "default"
  }
}
```

Audit with live metadata (instance-aware rules):

```bash
deltascope audit \
  --sql "alter table orders add index idx_status (status)" \
  --host 127.0.0.1 --port 3306 --user root --ask-password --schema app
```

See all shipped rules:

```bash
deltascope rules
```

## Why DeltaScope

SQL mistakes are cheap to catch before they run and expensive after. DeltaScope gives you one consistent engine across local dev, CI, HTTP service, and MCP so the same policy applies everywhere — no per-tool rule duplication, no dialect surprises.

## Key Features

- Create-table governance across identifiers, comments, primary keys, audit columns, charset/collation, indexes, and table options.
- Alter-table governance for destructive actions, compatibility checks, existence validation, and merge guidance.
- Object-lifecycle checks for `CREATE VIEW`, `DROP TABLE`, and `TRUNCATE TABLE`.
- DML protections for `WHERE`, `LIMIT`, `ORDER BY`, subqueries, join conditions, bulk insert patterns, and denylisted objects.
- Stable product surfaces: `deltascope` CLI, `deltascope-server`, `deltascope-mcp`, and `pkg/deltascope`.
- `deltascope-mcp` is the official MCP stdio server and exposes `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities`.

## MCP Quick Start

> **No install required.** The npm launcher fetches and runs the correct `deltascope-mcp` binary for your platform automatically.

Launcher requirements:

- Node.js 24 or newer
- supported native targets: `darwin` or `linux`, `amd64` or `arm64`

Recommended launcher:

```bash
claude mcp add --scope user deltascope -- npx -y @fanduzi/deltascope-mcp
codex mcp add deltascope -- npx -y @fanduzi/deltascope-mcp
```

For raw stdio TOML, native `deltascope-mcp`, direct connection, `connection_ref`, proxy setup, and common errors, see [Use DeltaScope MCP](docs/recipe/use-deltascope-mcp.md).

## AI Agent Skill

> **Works in Claude Code, Codex, Cursor, and 40+ AI coding agents.**
> Install once, get inline SQL review in every session.

DeltaScope ships a universal AI agent skill for inline SQL review during AI coding sessions. The skill detects whether DeltaScope is installed locally, calls it to audit your SQL, and surfaces findings with fix suggestions — without leaving your AI coding session.

```bash
# Install via npx skills (Claude Code, Codex, Cursor and 40+ AI agents)
npx skills add Fanduzi/DeltaScope --skill deltascope-review -a claude-code
```

Install globally (available across all projects):

```bash
npx skills add Fanduzi/DeltaScope --skill deltascope-review -a claude-code -g
```

Keep the skill up to date:

```bash
npx skills update
```

Then invoke in any supported AI session:

```
/deltascope-review
```

Paste a SQL snippet or point to a file — the agent audits it with DeltaScope and suggests fixes. See [skills/README.md](skills/README.md) for full setup and usage.

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

`POST /v1/audit` supports both offline JSON audit requests and metadata-aware requests with an optional `connection` block. The HTTP response keeps the public audit result body and adds a `context` block. See the full contract in [HTTP API reference](docs/reference/http-api.md).

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
