<div align="center">

# DeltaScope

[![Release](https://img.shields.io/github/v/release/Fanduzi/DeltaScope?display_name=tag)](https://github.com/Fanduzi/DeltaScope/releases)
![Platform](https://img.shields.io/badge/platform-darwin%20%7C%20linux-blue)
![Go Version](https://img.shields.io/badge/go-1.26.1-00ADD8?logo=go)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

English | [中文](README_ZH.md) | [Changelog](CHANGELOG.md) | [Security](SECURITY.md) | [License](LICENSE) | [Release Notes](docs/releases/release-notes-v0.6.1.md)
</div>

DeltaScope is an offline-first SQL audit engine for MySQL and TiDB. It gives DBAs, application engineers, CI pipelines, and AI agents one consistent way to review DDL and DML before they reach a database.

## Install

The primary install path is the repository installer script, which resolves the same release archive contract used by CI publishing.

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/main/install.sh | sh
```

Pin a specific release:

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.6.1/install.sh | \
  DELTASCOPE_VERSION=v0.6.1 sh
```

The published archive format is `deltascope_0.6.1_<os>_<arch>.tar.gz`. Development-oriented commands are documented under [Dev docs](docs/dev/README.md).

## Quick Start

Audit a risky DML statement:

```bash
deltascope audit --sql "delete from users"
```

Example output:

```text
Verdict: reject
Statements: 1
Blockers: 1
Warnings: 0
Notices: 0

Statement 1: DELETE
- [blocker] dml.where.require: DELETE or UPDATE must include a WHERE clause
```

Audit a `CREATE TABLE` statement:

```bash
deltascope audit --sql "create table users (id bigint unsigned not null auto_increment, primary key (id), name varchar(255) not null comment 'user name') comment='user table'"
```

Example output:

```text
Verdict: review
Statements: 1
Blockers: 0
Warnings: 1
Notices: 0

Statement 1: CREATE TABLE
- [warning] ddl.column.comment.require: column `id` must have a comment
```

Audit a file:

```bash
deltascope audit --file ./change.sql
```

Use JSON output for CI or agents:

```bash
deltascope audit \
  --sql "alter table users drop column age" \
  --format json \
  --fail-on warning
```

Example JSON shape:

```json
{
  "verdict": "review",
  "summary": {
    "blockers": 0,
    "warnings": 1,
    "notices": 0
  },
  "statements": [
    {
      "index": 1,
      "kind": "ALTER TABLE",
      "findings": [
        {
          "rule_id": "ddl.alter.drop_column.forbid",
          "level": "warning",
          "message": "dropping columns should be reviewed carefully"
        }
      ]
    }
  ]
}
```

Run metadata-aware audit against a live schema:

```bash
deltascope audit \
  --sql "alter table users add column email varchar(255)" \
  --host 127.0.0.1 --port 3306 --user root --ask-password --schema app
```

## Why DeltaScope

- Reviews DDL and DML with stable `blocker`, `warning`, and `notice` findings.
- Stays useful offline, which keeps local development, CI, and agent loops lightweight.
- Reuses the same rule engine for CLI, HTTP, and library access instead of splitting behavior across tools.
- Adds metadata-aware enrichment only when live schema or instance facts actually matter.

## Key Features

- Create-table governance across identifiers, comments, primary keys, audit columns, charset/collation, indexes, and table options.
- Alter-table governance for destructive actions, compatibility checks, existence validation, and merge guidance.
- Object-lifecycle checks for `CREATE VIEW`, `DROP TABLE`, and `TRUNCATE TABLE`.
- DML protections for `WHERE`, `LIMIT`, `ORDER BY`, subqueries, join conditions, bulk insert patterns, and denylisted objects.
- Stable product surfaces: `deltascope` CLI, `deltascope-server`, and `pkg/deltascope`.

## Recipes

- [Audit SQL offline](docs/recipe/audit-sql-offline.md)
- [Audit SQL with metadata](docs/recipe/audit-sql-with-metadata.md)
- [Review DDL before migration](docs/recipe/review-ddl-before-migration.md)
- [Guard DML in CI](docs/recipe/guard-dml-in-ci.md)
- [Use with AI agents](docs/recipe/use-with-ai-agents.md)
- [Troubleshoot metadata-aware audit](docs/recipe/troubleshoot-metadata-aware-audit.md)

## Documentation

- [Admin docs](docs/admin/README.md)
- [Concept docs](docs/concept/README.md)
- [Dev docs](docs/dev/README.md)
- [Reference docs](docs/reference/README.md)
- [Audit capability matrix](docs/reference/audit-capability-matrix.md)

## Developer Workflows

- `make test` runs `go test ./...`
- `make build` produces both local binaries under `bin/`
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
| `internal/interfaces` | Transport adapter namespace | [README](internal/interfaces/README.md) |
| `internal/interfaces/cli` | CLI adapter layer | [README](internal/interfaces/cli/README.md) |
| `internal/interfaces/http` | HTTP adapter layer | [README](internal/interfaces/http/README.md) |
| `internal/application` | Use-case orchestration layer | [README](internal/application/README.md) |
| `internal/application/audit` | Application parse/audit orchestration | [README](internal/application/audit/README.md) |
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
