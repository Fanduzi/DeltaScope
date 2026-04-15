<div align="center">

# DeltaScope

[![Release](https://img.shields.io/github/v/release/Fanduzi/DeltaScope?display_name=tag)](https://github.com/Fanduzi/DeltaScope/releases)
![Platform](https://img.shields.io/badge/platform-darwin%20%7C%20linux-blue)
![Go Version](https://img.shields.io/badge/go-1.26.1-00ADD8?logo=go)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

[![English](https://img.shields.io/badge/docs-English-blue)](README.md) [![简体中文](https://img.shields.io/badge/docs-简体中文-yellow)](README_ZH.md)

[![Changelog](https://img.shields.io/badge/Changelog-informational)](CHANGELOG.md) [![Security](https://img.shields.io/badge/Security-important)](SECURITY.md) [![License](https://img.shields.io/badge/License-blue)](LICENSE) [![Release Notes](https://img.shields.io/badge/Release_Notes-success)](docs/releases/README.md)
</div>

DeltaScope is an offline-first SQL audit engine for MySQL, TiDB, and PostgreSQL. The main product surfaces are `deltascope`, `deltascope-server`, and `deltascope-mcp`; PostgreSQL offline support is converged on the main archives for the supported macOS and Linux platforms instead of living behind a separate PG-only CLI entrypoint. As of `v0.33.0`, DeltaScope ships the **PostgreSQL Generated/Identity Fact Preservation + Unsupported Metadata Surfacing Pack**: generated/identity column facts are now preserved in the shared DDL contract, and unsupported generated/identity outcomes carry structured metadata. Generated expression text remains deferred. It gives DBAs, application engineers, CI pipelines, and AI agents one consistent way to review DDL and DML before they reach a database.

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
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.33.0/install.sh | \
  DELTASCOPE_VERSION=v0.33.0 sh
```

### PostgreSQL Generated/Identity Fact Preservation + Unsupported Metadata Surfacing Pack (`v0.33.0`)

`v0.33.0` preserves narrow generated/identity column facts in the shared DDL contract and surfaces structured metadata on unsupported generated/identity outcomes. It does not add generated-column support, identity-column support, expression evaluation, or rule behavior changes.

- **Shared contract fields**: `GeneratedWhen` (string: `"a"` / `"d"`), `IsIdentity` (bool), and `IdentityOptions` (finite structured option map) added to `spec.Column` for `CREATE TABLE` and `ALTER TABLE ADD COLUMN` paths.
- **Unsupported metadata**: `UnsupportedDetail.Metadata` now carries `column`, `generated_when`, `is_identity` (identity cases), and `identity_options` (options cases) for generated/identity unsupported outcomes.
- **GeneratedExpression deferred**: no expression text is preserved — the current `pg_query_go` dependency lacks a stable expression renderer.
- **MCP surface limitation**: metadata is not directly surfaced in MCP tool error responses; CLI, HTTP, and `pkg/deltascope` expose it.
- Unsupported feature names unchanged: `generated_column`, `generated_as_identity`. No new rule IDs, CLI flags, or public API contracts.

Previous milestone: `v0.32.0` documented stable AST facts through characterization tests and recommended `v0.33.0` as a narrow fact-preservation pack. See the [v0.33.0 release notes](docs/releases/release-notes-v0.33.0.md) for details.

### PostgreSQL Boundary Support-Readiness Gate (`v0.32.0`)

`v0.32.0` is a decision milestone — not a feature release. Characterization tests document stable AST facts about PostgreSQL generated and identity columns (`GeneratedWhen` encoding, `CONSTR_IDENTITY` / `CONSTR_GENERATED` types, identity sequence option shape). A readiness report recommends `v0.33.0` as a narrow fact-preservation pack adding `GeneratedWhen` and `IsIdentity` fields to `spec.Column`. No new rules, CLI flags, or public API contracts were added.

### PostgreSQL ALTER TABLE GENERATED Follow-up Pack (`v0.31.0`)

`v0.31.0` maps additional PostgreSQL generated/identity `ALTER TABLE` forms to explicit unsupported feature tags, closing the adjacent gap left by `v0.30.0`. It does not add new rules, new CLI flags, or new public API contracts, and it is not generated-column support, identity-column support, or complete PostgreSQL `ALTER TABLE` support.

- **Drop expression** (`ALTER COLUMN ... DROP EXPRESSION`) → explicit unsupported (`generated_column`).
- **Set generated** (`ALTER COLUMN ... SET GENERATED ...`) → explicit unsupported (`generated_as_identity`).
- **Drop identity** (`ALTER COLUMN ... DROP IDENTITY`) → explicit unsupported (`generated_as_identity`).
- Corpus cases and service-level checks lock these boundary outcomes with precise assertions.
- Surface parity across CLI, HTTP, MCP, and `pkg/deltascope` verifies the same unsupported contract on every transport.
- This release is boundary tightening, not generated-column support, identity-column support, or complete PostgreSQL `ALTER TABLE` support.

Previous milestone: `v0.30.0` tightened the PostgreSQL `ALTER TABLE ... ADD COLUMN` unsupported boundary contract for generated/identity forms. See the [v0.31.0 release notes](docs/releases/release-notes-v0.31.0.md) for details.

### PostgreSQL ALTER TABLE GENERATED Boundary Pack (`v0.30.0`)

`v0.30.0` tightens the PostgreSQL `ALTER TABLE ... ADD COLUMN` unsupported boundary contract for generated/identity forms. It does not add new rules, new CLI flags, or new public API contracts, and it is not broad PostgreSQL `ALTER TABLE` support.

- **Generated stored add-column** (`GENERATED ALWAYS AS (...) STORED`) → explicit unsupported (`generated_column`).
- **Identity add-column** (`GENERATED ALWAYS AS IDENTITY`) → explicit unsupported (`generated_as_identity`).
- Corpus cases and service-level checks lock these boundary outcomes with precise assertions.
- Surface parity across CLI, HTTP, MCP, and `pkg/deltascope` verifies the same unsupported contract on every transport.
- Adjacent `DROP EXPRESSION`, `SET GENERATED`, and `DROP IDENTITY` forms now receive explicit unsupported mappings in `v0.31.0`.
- This release is boundary tightening, not generated-column support, identity-column support, or broad PostgreSQL `ALTER TABLE` support.

Previous milestone: `v0.29.0` added a narrow schema-aware FK advisory step for explicit cross-schema references. See the [v0.30.0 release notes](docs/releases/release-notes-v0.30.0.md) for details.

### Schema-Aware FK Policy Pack (`v0.29.0`)

`v0.29.0` starts using explicit PostgreSQL schema-qualified FK facts for a narrow policy decision. DeltaScope now emits the notice-level rule `ddl.pg.table.foreign_key.cross_schema.advisory` when the owning table schema and referenced schema are both explicit and different.

- **Cross-schema only**: explicit cross-schema FK gets an extra notice-level advisory. Same-schema FK does not trigger it.
- **Bare references stay unknown**: `REFERENCES users(id)` remains schema unknown. DeltaScope does not infer `public` and does not model `search_path`.
- **Existing FK forbid rule still applies**: the advisory adds context; it does not replace `ddl.table.foreign_key.forbid`.
- **Metadata stays normalized**: findings can include `table_schema`, `referenced_schema`, `referenced_table`, and `referenced_columns`, and `referenced_table` remains `"users"`, never `"auth.users"`.

Previous milestone: `v0.28.0` exposed referenced-object facts on outward FK finding metadata. See the [v0.29.0 release notes](docs/releases/release-notes-v0.29.0.md) for details.

### Referenced-Object Metadata Surface Pack (`v0.28.0`)

`v0.28.0` exposes PostgreSQL referenced-object facts (`referenced_schema`, `referenced_table`, `referenced_columns`) as additive finding metadata on the FK forbid rule, across CLI, HTTP, MCP, and `pkg/deltascope`. It does not add new rules, new CLI flags, or new public API contracts, and it is not schema-aware FK policy support.

- **Finding metadata widening**: the `ddl.table.foreign_key.forbid` finding now includes `referenced_schema` (e.g., `"public"`), `referenced_table` (e.g., `"users"`), and `referenced_columns` (e.g., `["id"]`) when the underlying constraint carries those facts. `referenced_table` is never concatenated into `"public.users"`.
- Parser/extractor semantics are unchanged from `v0.27.0`. The shared semantic contract (`spec.Constraint`) already had `ReferencedSchema`, `ReferencedTable`, and `ReferencedColumns`; `v0.28.0` widens the outward finding metadata to expose them.
- No new rule IDs, no new CLI flags, no schema-aware FK policy decisions.

Previous milestone: `v0.27.0` preserved schema-qualified PostgreSQL referenced-object facts in the shared contract. See the [v0.27.0 release notes](docs/releases/release-notes-v0.27.0.md) for details.

### Schema-Qualified Reference Semantics Pack (`v0.27.0`)

`v0.27.0` preserves schema-qualified PostgreSQL referenced-object facts in the shared contract. It does not add new rules, new CLI flags, or new public API contracts, and it is not a broad PostgreSQL FK implementation.

- **`ReferencedSchema`** is an additive field on `spec.Constraint` that preserves the schema portion of schema-qualified `REFERENCES` (e.g., `REFERENCES public.users(id)` → `ReferencedSchema = "public"`, `ReferencedTable = "users"`).
- PostgreSQL extractor preserves these facts for both named `FOREIGN KEY` and inline `REFERENCES` forms.
- Corpus and service-level tests lock the semantic contract with precise assertions.
- CLI, HTTP, MCP, and `pkg/deltascope` current public finding metadata remains unchanged — the shared semantic contract is richer underneath.

Previous milestone: `v0.26.0` tightened the PostgreSQL `CREATE TABLE` unsupported boundary contract. See the [v0.26.0 release notes](docs/releases/release-notes-v0.26.0.md) for details.

### PostgreSQL CREATE TABLE Unsupported Boundary Pack (`v0.26.0`)

`v0.26.0` tightens the PostgreSQL `CREATE TABLE` unsupported boundary contract. It does not add new rules, new CLI flags, or new public API contracts, and it is not full PostgreSQL `CREATE TABLE` support.

- **Identity columns** (`GENERATED ... AS IDENTITY`) → explicit unsupported (`generated_as_identity`).
- **Generated stored columns** (`GENERATED ALWAYS AS ... STORED`) → explicit unsupported (`generated_column`).
- **Exclusion constraints** (`EXCLUDE USING`) → explicit unsupported (`exclusion_constraint`).
- **Partitioned tables** (`PARTITION BY`) → explicit unsupported (`partitioning`).
- PostgreSQL corpus cases lock these four boundaries with precise expected-outcome assertions.
- Surface parity tests across CLI, HTTP, MCP, and `pkg/deltascope` verify each boundary is exposed through the correct unsupported contract on every transport.

Surface contract for unsupported statements:

- **CLI** and **`pkg/deltascope`**: return a partial result with an `unsupported` array carrying `feature` and `reason` fields, plus the `ErrUnsupportedStatement` sentinel error.
- **HTTP** and **MCP**: expose unsupported statements as transport-level errors (HTTP error response, MCP tool error) because the underlying audit function returns an error for unsupported boundaries.

`make release-surface-gates VERSION=v0.33.0` and `make release-version-surface-gates VERSION=v0.33.0` verify the package/release and versioned docs surfaces.

Previous milestone: `v0.33.0` preserves generated/identity facts and surfaces unsupported metadata for PostgreSQL. See the [v0.33.0 release notes](docs/releases/release-notes-v0.33.0.md) for details.

Need PostgreSQL offline audit support?

- Install the normal DeltaScope main archive on supported macOS and Linux platforms; no separate PG-only installer is required.
- `deltascope-pg_<version>_linux_amd64.tar.gz` remains available only as a legacy compatibility download for older CLI-only workflows during the transition.

The published core archive format is `deltascope_<version>_<os>_<arch>.tar.gz`. Development-oriented commands are documented under [Dev docs](docs/dev/README.md).

### Release Contract

Every tag produces core archives named `deltascope_<version>_<os>_<arch>.tar.gz` containing the `deltascope`, `deltascope-server`, and `deltascope-mcp` binaries. The supported `darwin/amd64`, `darwin/arm64`, `linux/amd64`, and `linux/arm64` main archives are PG-capable and support PostgreSQL offline across all three binaries. The installer script, Homebrew Cask, and npm MCP launcher all resolve those platform-specific main archives from GitHub Release assets. `deltascope-pg_<version>_linux_amd64.tar.gz` may still appear as a legacy compatibility download during the transition, but it is no longer part of the primary install story. See the npm package metadata for the current `@fanduzi/deltascope-mcp` package version.

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

Audit a PostgreSQL migration with CI-native output:

```bash
deltascope audit --dialect postgresql --file ./migrations/20260409_add_index.sql --format github-actions
```

Audit a PostgreSQL `CREATE TABLE` statement with named and inline constraints:

```bash
deltascope audit \
  --dialect postgresql \
  --sql "create table orders (id bigint primary key, user_id bigint references users(id), amount numeric not null check (amount >= 0), constraint uniq_orders_user unique (user_id), constraint chk_orders_amount check (amount >= 0));"
```

Audit a PostgreSQL `CREATE TABLE` with a named foreign key referencing another table:

```bash
deltascope audit \
  --dialect postgresql \
  --sql "create table orders (user_id bigint, constraint fk_orders_user foreign key (user_id) references users(id));"
```

Audit a PostgreSQL phased migration follow-up statement:

```bash
deltascope audit \
  --dialect postgresql \
  --sql "alter table users alter column status set default 'active';"
```

Audit a PostgreSQL constraint lifecycle statement:

```bash
deltascope audit \
  --dialect postgresql \
  --sql "alter table users validate constraint chk_amount;"
```

Generate SARIF output for GitHub Code Scanning:

```bash
deltascope audit --file ./migrations.sql --dialect postgresql --format sarif > deltascope.sarif
```

When SQL looks like PostgreSQL but the dialect is set to MySQL, DeltaScope emits an advisory notice without auto-switching:

```bash
deltascope audit --sql "insert into users(id) values (1) returning id;" --format markdown
```

To audit PostgreSQL SQL explicitly:

```bash
deltascope audit --dialect postgresql --sql "insert into users(id) values (1) returning id;"
```

## DML Impact Estimation

For a selective DML such as `DELETE FROM users WHERE id = 42`, DeltaScope may add an `impact` object to the statement result. The object is conservative by design and reports `estimated_rows`, `estimated_ratio`, `risk_level`, `confidence`, `source`, `reason_codes`, and optional `notes`.

```json
{
  "raw_sql": "DELETE FROM users WHERE id = 42",
  "impact": {
    "estimated_rows": 1,
    "estimated_ratio": 0.0001,
    "risk_level": "low",
    "confidence": "high",
    "source": "metadata",
    "reason_codes": ["pk_equality"],
    "notes": ["refined with table statistics"]
  }
}
```

Offline mode uses SQL shape only. Metadata-aware mode may refine the estimate with read-only table statistics. DeltaScope does not execute the DML and does not run `EXPLAIN ANALYZE`.

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
- DML protections for `WHERE`, `LIMIT`, `ORDER BY`, subqueries, join conditions, bulk insert patterns, denylisted objects, and conservative affected-row impact estimation.
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
- `make pg-unit-test-gates` runs the PostgreSQL-tagged unit gate set
- `make pg-e2e-gates` runs the Docker-backed PostgreSQL CLI, HTTP, and MCP suites
- `make pg-confidence-gates` runs the canonical PostgreSQL confidence closure
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
