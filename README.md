<div align="center">

# DeltaScope

[![Release](https://img.shields.io/github/v/release/Fanduzi/DeltaScope?display_name=tag)](https://github.com/Fanduzi/DeltaScope/releases)
![Platform](https://img.shields.io/badge/platform-darwin%20%7C%20linux-blue)
![Go Version](https://img.shields.io/badge/go-1.25-00ADD8?logo=go)
![License](https://img.shields.io/badge/license-not%20set-lightgrey)

English | [中文](README_ZH.md) | [Changelog](CHANGELOG.md) | [Security](SECURITY.md)
</div>

DeltaScope is a SQL audit engine for MySQL and TiDB. It starts as an offline-first library and CLI, then layers optional metadata-aware checks and a thin HTTP service on top of the same core rule engine.

## Why DeltaScope

- Reviews DDL and DML with stable `blocker`, `warning`, and `notice` findings.
- Runs fully offline by default, which keeps local development and agent workflows usable.
- Supports optional metadata-aware enrichment for existence checks, current-schema comparisons, and instance facts.
- Ships as a Go package, `deltascope` CLI, and `deltascope-server` HTTP service.

## Current Surface

- `CREATE TABLE` governance: identifiers, comments, primary-key semantics, audit columns, type-family policy, charset/collation checks, index width/prefix/redundancy, table-option rules, and metadata-backed rough sizing checks.
- `ALTER TABLE` governance: action forbids, source-aware compatibility checks, metadata-backed existence rules, alter-added index lifecycle rules, and global merge-alter guidance.
- Object lifecycle governance: `CREATE VIEW`, `DROP TABLE`, and `TRUNCATE TABLE` policy gates, plus metadata-backed existence, row-count, and adaptive-hash cautions.
- DML governance: `WHERE`, `LIMIT`, `ORDER BY`, subquery, join-`ON`, insert-row-count, `REPLACE`, `INSERT ... SELECT`, `ON DUPLICATE KEY`, and object-scope denylist rules.

## Quick Start

Run tests and inspect versions:

```bash
go test ./...
go run ./cmd/deltascope --version
go run ./cmd/deltascope version
go run ./cmd/deltascope-server -version
```

Audit inline SQL:

```bash
go run ./cmd/deltascope audit --sql "delete from users"
```

Audit a file or stdin:

```bash
go run ./cmd/deltascope audit --file ./change.sql
cat ./change.sql | go run ./cmd/deltascope audit
```

Use JSON output for agents, scripts, or CI:

```bash
go run ./cmd/deltascope audit --sql "alter table users drop column age" --format json
```

Use metadata-aware audit with MySQL-style connection flags:

```bash
go run ./cmd/deltascope audit \
  --sql "alter table users add column email varchar(255)" \
  --host 127.0.0.1 --port 3306 --user root --ask-password --schema app
```

Inspect shipped rules and product capabilities:

```bash
go run ./cmd/deltascope rules list --kind dml --level blocker
go run ./cmd/deltascope rules show dml.where.require
go run ./cmd/deltascope rules search metadata
go run ./cmd/deltascope capabilities
```

Control the non-zero threshold:

```bash
go run ./cmd/deltascope audit --sql "create table users (id bigint, primary key (id))" --fail-on warning
```

Exit codes:

- `0`: audit finished and did not reach the configured failure threshold
- `1`: audit finished, but findings reached `--fail-on`
- `2`: user input error such as bad flags, unreadable config, or malformed request data
- `3`: internal/runtime failure

## Configuration

Generate the default YAML policy:

```bash
go run ./cmd/deltascope config init > deltascope.yaml
```

Run with a config file:

```bash
go run ./cmd/deltascope audit --config ./deltascope.yaml --sql "update users set name = 'delta'"
```

Lint and inspect config defaults:

```bash
go run ./cmd/deltascope config lint --file ./deltascope.yaml
go run ./cmd/deltascope config show-default
```

The checked-in [example config](configs/deltascope.example.yaml) matches `deltascope config init`.

## Metadata-Aware Mode

DeltaScope stays usable without database access. When a metadata provider is configured, the same audit flow can attach:

- instance facts such as `version`, `character_set_database`, `innodb_large_prefix`, `innodb_default_row_format`, and `innodb_adaptive_hash_index`
- target-table snapshots with normalized column, index, and primary-key definitions

That extra context currently powers:

- create/alter/drop/truncate existence checks
- source-aware `ALTER COLUMN` compatibility checks
- adaptive-hash cautions for destructive table lifecycle operations

From the CLI, metadata-aware mode starts when any MySQL-style connection flag is supplied. DeltaScope then:

- auto-detects MySQL vs TiDB from the live instance
- uses `--schema` when given
- otherwise infers schema when the target table resolves uniquely
- fails honestly when schema inference is ambiguous or impossible for statements that need a real existing object
- keeps `--quiet` stable for shell pipelines and includes a `context` object in JSON output for agents

## HTTP Service

Run the service:

```bash
go run ./cmd/deltascope-server --listen 127.0.0.1:8083 --config ./deltascope.yaml
```

Endpoints:

- `GET /healthz`
- `GET /version`
- `POST /v1/audit`

Example:

```bash
curl -X POST http://127.0.0.1:8083/v1/audit \
  -H 'Content-Type: application/json' \
  -d '{"sql":"delete from users","dialect":"mysql"}'
```

## Library Usage

```go
result, err := deltascope.Audit(ctx, deltascope.Request{
    SQL:     "delete from users",
    Dialect: deltascope.DialectMySQL,
})
```

The public API lives in [pkg/deltascope](pkg/deltascope/README.md).

## Roadmap

Current work is focused on follow-on hardening rather than baseline capability gaps:

## Architecture

DeltaScope uses a DDD-leaning structure. Interfaces drive transport concerns, application orchestrates the audit use case, domain packages own the rule model and findings, and infrastructure packages adapt parser/config/output/metadata dependencies. The core audit path is shared by the library, CLI, and HTTP service.

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
