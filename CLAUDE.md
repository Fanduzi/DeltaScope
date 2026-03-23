# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
make build            # produces bin/deltascope and bin/deltascope-server
make build-cli        # CLI only
make build-server     # HTTP server only
make build-linux      # Linux amd64 binaries under bin/

# Test
make test             # go test ./... (fast, no Docker)
make test-e2e-cli     # Docker-backed metadata e2e: MySQL + TiDB
make test-e2e-cli-mysql
make test-e2e-cli-tidb

# Run a single test package
go test ./internal/domain/rule/ddl/...

# Run a single test by name
go test ./internal/domain/rule/ddl/... -run TestTableCommentRule
```

E2E metadata tests require Docker, Go, and Python 3.

## Architecture

DeltaScope is an offline-first SQL audit engine for MySQL and TiDB. One audit path, three product surfaces: `deltascope` CLI, `deltascope-server` HTTP service, and `pkg/deltascope` library.

### Layer Boundaries

```
cmd/deltascope | cmd/deltascope-server   ← thin process entrypoints (flag binding only)
       ↓
internal/interfaces/cli | http           ← transport adapters (request/response translation)
       ↓
internal/application/audit | policy      ← orchestration: parse → extract → enrich → evaluate
       ↓
internal/domain/spec | rule | policy | report  ← core domain: normalized types and rule semantics
       ↓
internal/infrastructure/parser/tidb      ← TiDB parser adapter (converts AST → spec)
internal/infrastructure/config/viper     ← YAML policy loader
internal/infrastructure/metadata/mysql   ← live schema/instance facts provider
internal/infrastructure/output/json | markdown  ← renderers
       ↓
pkg/deltascope                           ← stable public API facade
```

**Key constraint**: `internal/infrastructure` adapts external dependencies. Domain packages must not import infrastructure. Rules live in `internal/domain/rule/ddl` and `internal/domain/rule/dml`.

### Audit Flow

1. **Parse** (`application/audit/parse.go`): SQL text → `ParsedSQL` via TiDB parser adapter.
2. **Extract** (`application/audit/extract.go`): `ParsedSQL` → `[]spec.Statement` (parser-neutral normalized model).
3. **Enrich** (`application/audit/metadata.go`): optional `MetadataProvider` attaches live `TableSnapshot` and `InstanceFacts` to each statement.
4. **Evaluate** (`application/audit/evaluate.go`): registered rules applied to each statement; findings aggregated into `report.Report`.

### Key Domain Types

- `spec.Statement` — normalized statement with `DDL`, `DML`, and optional `Metadata` (schema, instance facts, target table snapshot).
- `rule.Finding` — one rule result with stable ID, `Level` (blocker/warning/notice), and message.
- `report.Report` / `Verdict` — aggregated outcome; verdict maps to CLI exit codes via `--fail-on`.
- `rule.Registry` — holds statement-scoped and global rules; evaluated deterministically by registration order.
- `policy.Policy` — per-rule on/off + params; `policy.Default()` is the shipped baseline.

### Rule Organization

- **DDL rules** (`internal/domain/rule/ddl/`): CREATE TABLE governance, ALTER TABLE restrictions, object lifecycle (CREATE VIEW, DROP TABLE, TRUNCATE), merge-alter guidance, metadata-backed existence/sizing checks.
- **DML rules** (`internal/domain/rule/dml/`): WHERE/LIMIT/ORDER BY requirements, subquery/join-on guards, INSERT row-count and pattern restrictions, table denylists.
- Each package exposes `Register(registry, policy)` which registers only enabled rules.
- Rule IDs follow the pattern `ddl.<area>.<check>` or `dml.<area>.<check>`.

### Module README Contract

Every `internal/` and `pkg/` package has a `README.md` listing files, exports, and dependencies. **When adding or changing exports or dependencies, update the package README in the same commit.** This is the authoritative index for understanding a package without reading source.

### Metadata-Aware Mode

Rules that require live schema facts are metadata-gated: they are registered via the normal `Register(...)` path but no-op when no `TargetTable` snapshot or `InstanceFacts` is attached. Offline audits are always safe to run without a database connection.

## Configuration

- Default policy: `internal/domain/policy/defaults.go` — `policy.Default()`.
- User config: YAML file, loaded via `internal/infrastructure/config/viper`. Example: `configs/deltascope.example.yaml`.
- Config is re-read per request in the HTTP server (no restart needed for policy changes).
- `deltascope config init` emits a default YAML template. `deltascope config lint` validates a config file.

## Public API

The stable library surface is `pkg/deltascope`. Do not add new public types to `internal/` packages and expose them directly; new public types belong in `pkg/deltascope`.

```go
result, err := deltascope.Audit(ctx, deltascope.Request{
    SQL:     "delete from users",
    Dialect: deltascope.DialectMySQL,
})
```

## HTTP Endpoints

- `GET /healthz`
- `GET /version`
- `POST /v1/audit` — same JSON shape as library `Result`
