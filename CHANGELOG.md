# Changelog

All notable changes to DeltaScope will be documented in this file.

The format follows Keep a Changelog and the project uses semantic versioning for release tags.

## [Unreleased]

## [v0.31.0] - 2026-04-14

### Changed

- PostgreSQL `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION` now returns the explicit unsupported feature `generated_column` instead of a generic AST-subtype unsupported boundary.
- PostgreSQL `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ...` now returns the explicit unsupported feature `generated_as_identity` instead of a generic AST-subtype unsupported boundary.
- PostgreSQL `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY` now returns the explicit unsupported feature `generated_as_identity` instead of a generic AST-subtype unsupported boundary.
- These mappings align the adjacent PostgreSQL generated/identity alteration forms with the same stable unsupported feature names used by `v0.26.0` (`CREATE TABLE`) and `v0.30.0` (`ADD COLUMN`).
- Corpus, service, and CLI / HTTP / MCP / `pkg/deltascope` parity tests lock these boundary outcomes with precise assertions.
- Release-facing docs now position `v0.31.0` as the **PostgreSQL ALTER TABLE GENERATED Follow-up Pack** — boundary tightening only, not generated-column support, identity-column support, or complete PostgreSQL `ALTER TABLE` support.

## [v0.30.0] - 2026-04-14

### Added

- PostgreSQL `ALTER TABLE ... ADD COLUMN` generated/identity boundary coverage is now locked as an explicit unsupported contract across corpus, service checks, and surface parity for CLI, HTTP, MCP, and `pkg/deltascope`.

### Changed

- PostgreSQL `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS (...) STORED` now returns the explicit unsupported feature `generated_column` instead of looking like an ordinary supported add-column path.
- PostgreSQL `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS IDENTITY` now returns the explicit unsupported feature `generated_as_identity` instead of looking like an ordinary supported add-column path.
- Adjacent PostgreSQL `ALTER TABLE` generated/identity alteration forms such as `DROP EXPRESSION`, `SET GENERATED`, and `DROP IDENTITY` remain generic unsupported boundaries.
- Release-facing docs now position `v0.30.0` as the **PostgreSQL ALTER TABLE GENERATED Boundary Pack** — a boundary-tightening release, not generated-column support, identity-column support, or broad PostgreSQL `ALTER TABLE` support.

## [v0.29.0] - 2026-04-14

### Added

- PostgreSQL now ships the notice-level advisory rule `ddl.pg.table.foreign_key.cross_schema.advisory` for explicit cross-schema foreign keys when the owning table schema and referenced schema are both explicit and different.
- Cross-schema advisory findings can expose `table_schema`, `referenced_schema`, `referenced_table`, and `referenced_columns` in outward finding metadata. `referenced_table` remains normalized as `"users"`, never `"auth.users"`.

### Changed

- The additive FK finding metadata surface introduced in `v0.28.0` now participates in a narrow PostgreSQL policy decision. Same-schema foreign keys and bare references such as `REFERENCES users(id)` remain unchanged.
- Bare references remain schema unknown. DeltaScope does not infer `public` and does not model PostgreSQL `search_path` semantics.
- Release-facing docs now position `v0.29.0` as the **Schema-Aware FK Policy Pack** — the first schema-aware FK policy step, not full PostgreSQL foreign key support and not a cross-schema validation engine.

## [v0.28.0] - 2026-04-13

### Added

- FK forbid finding metadata now exposes referenced-object fields (`referenced_schema`, `referenced_table`, `referenced_columns`) for PostgreSQL foreign key constraints. These fields were already present in the shared semantic contract (`spec.Constraint`) from `v0.27.0`; `v0.28.0` widens the outward finding metadata contract so CLI, HTTP, MCP, and `pkg/deltascope` users can see them directly.
- `referenced_table` is never concatenated with `referenced_schema` (e.g., never `"public.users"`). The two fields are always separate and normalized.

### Changed

- Release-facing docs now position `v0.28.0` as the **Referenced-Object Metadata Surface Pack** — an additive metadata widening of the FK forbid finding contract. It does not add new rules, new CLI flags, new public API contracts, or schema-aware FK policy support. Parser/extractor semantics are unchanged from `v0.27.0`.

## [v0.27.0] - 2026-04-13

### Added

- Additive `ReferencedSchema` field on `spec.Constraint`: PostgreSQL schema-qualified `REFERENCES public.users(id)` now preserves the referenced-object schema (`"public"`) as a parser-owned shared contract fact alongside the existing `ReferencedTable` (`"users"`). The normalized representation is always `ReferencedSchema` + `ReferencedTable` — `ReferencedTable` is never concatenated into `"public.users"`.
- PostgreSQL extractor preserves schema-qualified reference facts for both named `FOREIGN KEY ... REFERENCES schema.table` and inline `REFERENCES schema.table` forms.
- Corpus cases updated to lock schema-qualified reference semantics with precise `.expected.yaml` assertions (`ReferencedSchema = "public"`, `ReferencedTable = "users"`).
- Service-level semantic tests assert schema-qualified reference facts are preserved through the audit pipeline.

### Changed

- Release-facing docs now position `v0.27.0` as the **Schema-Qualified Reference Semantics Pack** — an additive semantic preservation of PostgreSQL schema-qualified referenced-object facts in the shared contract. It does not add new rules, new CLI flags, or new public API contracts. Current public finding metadata (CLI, HTTP, MCP, `pkg/deltascope`) remains unchanged; the shared semantic contract is richer underneath.

## [v0.26.0] - 2026-04-12

### Added

- PostgreSQL `CREATE TABLE` unsupported boundary tightening: identity columns (`GENERATED ... AS IDENTITY`), generated stored columns (`GENERATED ALWAYS AS ... STORED`), exclusion constraints (`EXCLUDE USING`), and partitioned tables (`PARTITION BY`) are now explicitly marked as unsupported at the extractor level instead of being silently accepted or partially handled.
- PostgreSQL corpus cases updated to lock the four unsupported boundaries (`generated_as_identity`, `generated_column`, `exclusion_constraint`, `partitioning`) with precise expected-outcome assertions.
- Surface parity tests across CLI, HTTP, MCP, and public Go API (`pkg/deltascope`) verify that each boundary is exposed through the correct unsupported contract on every transport.

### Changed

- Release-facing docs now position `v0.26.0` as the **PostgreSQL CREATE TABLE Unsupported Boundary Pack** — an extractor-level boundary tightening backed by corpus and surface tests. It does not add new rules, new CLI flags, or new public API contracts. It does not represent full PostgreSQL `CREATE TABLE` support.

## [v0.25.0] - 2026-04-12

### Added

- Dialect-wide SQL corpus harness (`testdata/sql-corpus/`) with MySQL, TiDB, and PostgreSQL baseline cases covering supported, unsupported, finding-producing, clean, and boundary categories.
- Two-layer corpus assertions: report-level audit checks (unsupported count, statement kind, findings) plus semantic parse/extract assertions (operation name, constraint facts) driven by a single `.expected.yaml` file per case.
- MySQL baseline corpus: DDL supported (primary key), DDL findings (foreign key forbid), DML findings (UPDATE/DELETE without WHERE), DML clean (UPDATE/DELETE with WHERE).
- TiDB baseline corpus: DDL supported (primary key), DML findings (UPDATE/DELETE without WHERE), DML clean (UPDATE with WHERE).
- PostgreSQL baseline corpus: DDL supported (named CHECK, UNIQUE, FOREIGN KEY, inline REFERENCES), DDL findings (inline REFERENCES forbid), DDL unsupported (CREATE OR REPLACE VIEW), DDL boundary (GENERATED ... AS IDENTITY, PARTITION BY).
- `GENERATED ... AS IDENTITY` is recorded as a current boundary finding in the corpus — it is not fixed in this release. Follow-up: `PostgreSQL CREATE TABLE Unsupported Boundary Pack`.

### Changed

- Release-facing docs now position `v0.25.0` as the **SQL Corpus & Boundary Confidence Pack** — a durable corpus and table-driven audit harness that answers which representative SQL statements have been run through DeltaScope and what outcomes are expected. It does not add new rules, new CLI flags, or new public API contracts.

## [v0.24.0] - 2026-04-11

### Added

- PostgreSQL `CREATE TABLE` foreign-key semantics now preserve parser-owned referenced table and referenced columns in the shared `spec.Constraint` model. Named `FOREIGN KEY` and inline `REFERENCES` forms both carry `ReferencedTable` and `ReferencedColumns` as shared contract facts. These are parser-owned structural facts, not metadata-dependent truth claims.
- Service-level and surface parity tests tightened across CLI, HTTP, MCP, and public Go API (`pkg/deltascope`) for richer PostgreSQL foreign-key shapes. Inline `REFERENCES` now asserts `ddl.table.foreign_key.forbid` fires under the default policy.
- Unsupported boundary tests added for `CREATE OR REPLACE VIEW` and `PARTITION BY` on the service layer and public Go API, confirming that adjacent unsupported forms remain explicitly outside the supported surface.

### Changed

- Release-facing docs now position `v0.24.0` as a PostgreSQL `CREATE TABLE` semantics pack — a semantic deepening of `v0.23.0` — not a new rule pack and not full PostgreSQL DDL support.

## [v0.23.0] - 2026-04-11

### Added

- PostgreSQL `CREATE TABLE` coverage expanded for common constraint shapes: table-level named `CHECK`, column-level inline `CHECK`, table-level named `UNIQUE`, column-level inline `UNIQUE`, table-level named `FOREIGN KEY`, and column-level inline `REFERENCES`.
- Shared rule reuse for the newly normalized PostgreSQL create-table structures. Named `CHECK`, `UNIQUE`, and `FOREIGN KEY` constraints can flow into existing structured naming governance where the policy makes those rule families applicable; inline `UNIQUE` contributes index facts; inline `REFERENCES` is exposed as parser-owned shared facts without adding metadata-only semantics.
- CLI, HTTP, MCP, and public Go API (`pkg/deltascope`) parity confirmed for the expanded PostgreSQL `CREATE TABLE` coverage.

### Changed

- Release-facing docs now position `v0.23.0` as a PostgreSQL `CREATE TABLE` coverage pack, not full PostgreSQL DDL support and not a new-rule release.
- Reference docs and recipes now distinguish supported, auditable, rule-mapped, and metadata-dependent behavior for the richer PostgreSQL create-table shapes.

## [v0.22.0] - 2026-04-11

### Added

- Canonical PostgreSQL confidence entrypoints for local and CI verification: `pg-unit-test-gates`, `pg-e2e-gates`, and `pg-confidence-gates`.
- Reusable release confidence gates: `release-surface-gates` for package/release contract checks and `release-version-surface-gates` for versioned docs/install surface checks.
- Bilingual release notes and release-facing docs aligned around the `v0.22.0` **E2E & Release Confidence Pack** milestone.

### Changed

- DeltaScope now documents confidence closure around the existing PostgreSQL product surfaces instead of introducing new PostgreSQL SQL semantics in this release.
- README, landing page, CLI/reference docs, CI recipes, and scripts guide now point to the `v0.22.0` release line and the canonical confidence targets used to verify transport and release-surface alignment.

## [v0.21.0] - 2026-04-11

### Added

- PostgreSQL DDL coverage expanded for common migration follow-up statements. The following `ALTER TABLE` forms are now normalized into the shared audit pipeline instead of returning capability-boundary errors:
  - `ALTER COLUMN ... SET DEFAULT` — column default assignment during phased rollout
  - `ALTER COLUMN ... DROP DEFAULT` — column default removal
  - `ALTER COLUMN ... SET NOT NULL` — nullability enforcement after backfill
  - `ALTER COLUMN ... DROP NOT NULL` — nullability relaxation
  - `VALIDATE CONSTRAINT` — constraint validation step in the recommended `NOT VALID` → `VALIDATE CONSTRAINT` pattern
  - `DROP CONSTRAINT` — constraint removal, including primary-key mapping via metadata-aware rules

- Shared rule and metadata semantics now apply to the newly normalized PostgreSQL DDL actions. `DROP CONSTRAINT` on a primary key reuses existing `ddl.alter.drop_primary_key` rules when metadata is available.

- CLI, HTTP, MCP, and public API (`pkg/deltascope`) parity confirmed for all newly supported PostgreSQL DDL forms.

### Changed

- PostgreSQL migration review workflows that previously hit capability-boundary errors for `SET DEFAULT`, `DROP DEFAULT`, `SET NOT NULL`, `DROP NOT NULL`, `VALIDATE CONSTRAINT`, or `DROP CONSTRAINT` now return normal audit results.

## [v0.20.0] - 2026-04-10

### Added

- PostgreSQL syntax heuristic notice (`dialect.postgresql.syntax.detected.notice`): when auditing on the MySQL/TiDB path, DeltaScope now detects common PostgreSQL-specific syntax tokens (`RETURNING`, `ON CONFLICT`, `::` casts, `ALTER COLUMN TYPE USING`, `GENERATED AS IDENTITY`) and emits a notice-level global finding suggesting `--dialect postgresql`. DeltaScope does not auto-switch dialect — the notice is advisory only.
- Explicit PostgreSQL capability-boundary errors: unsupported-build surfaces now return typed `PostgreSQLCapabilityBoundaryError` values instead of heuristic string matching, making it easier for tooling and CI to distinguish real parse failures from capability limits.
- CLI output trust signals: markdown output now includes a `## Audit Context` section with mode, dialect source, and an explicit trust note when a PostgreSQL syntax notice is present. JSON output includes a top-level `context` object. Quiet output appends a `[context]` line.
- Rule summary and skipped-rules visibility in CLI output formats: `rule_summary` (loaded, applicable, skipped counts) appears in JSON; `## Rule Summary` and `## Skipped Rules` sections appear in markdown; `[summary]` line appears in quiet output. GitHub Actions and SARIF output continue to emit findings only.

### Changed

- PostgreSQL migration-safety rule suggestions now provide step-by-step migration guidance instead of generic tips:
  - `ddl.pg.create_index.concurrently.require`: mentions that `CONCURRENTLY` cannot run inside a transaction.
  - `ddl.pg.alter.add_column.non_null_default.rewrite.warn`: recommends a 4-step safe path (nullable → backfill → default → not null).
  - `ddl.pg.alter.add_check.not_valid.require`: describes the 2-step `NOT VALID` → `VALIDATE CONSTRAINT` approach with lock-level detail.
  - `ddl.pg.alter.set_data_type.rewrite.warn`: recommends phased migration with shadow column strategy for large tables.

### Fixed

- PostgreSQL syntax heuristic no longer fires for tokens inside string literals, quoted identifiers, backtick identifiers, line comments, or block comments.
- Metadata request merge: mixed top-level `Schema`/`MetadataProvider` fields with legacy `Metadata` struct fields no longer drop schema or provider context.

## [v0.19.0] - 2026-04-09

### Added

- PostgreSQL migration-safety rule pack (4 rules, default level `warning`):
  - `ddl.pg.create_index.concurrently.require` — flags `CREATE INDEX` without `CONCURRENTLY` on PostgreSQL
  - `ddl.pg.alter.add_column.non_null_default.rewrite.warn` — warns when `ALTER TABLE … ADD COLUMN … NOT NULL DEFAULT …` may trigger a full table rewrite
  - `ddl.pg.alter.add_check.not_valid.require` — flags `ALTER TABLE … ADD CHECK (…)` without `NOT VALID` on large tables
  - `ddl.pg.alter.set_data_type.rewrite.warn` — warns when `ALTER TABLE … ALTER COLUMN … TYPE …` may require a full table rewrite
- `--format github-actions` output for inline CI annotations (`::error`, `::warning`, `::notice`) with proper workflow-command escaping
- `--format sarif` output producing valid SARIF 2.1.0 JSON for GitHub Code Scanning and other SARIF consumers
- `rule_summary` field in JSON output and `## Rule Summary` / `## Skipped Rules` sections in Markdown output showing loaded, applicable, and skipped rule counts

### Changed

- Documentation updated across all reference, recipe, and landing pages to reflect the v0.19.0 PostgreSQL migration-safety pack and CI output formats

## [v0.18.0] - 2026-04-09

### Added

- PostgreSQL metadata-aware audit: DeltaScope can now connect to a live PostgreSQL 12+ instance to retrieve schema metadata, run `EXPLAIN` for DML impact estimation, and evaluate rules against real database state
- Transport parity: CLI, HTTP (`POST /v1/audit`), and MCP (`audit_sql`) all support PostgreSQL metadata-aware audit with `--dialect postgresql` or `"dialect": "postgresql"`
- PostgreSQL schema resolution: qualified table names in SQL are parsed automatically; unqualified names resolve via `--schema` flag or unique-match inference across accessible schemas
- PostgreSQL DML impact estimation via `EXPLAIN` (read-only, conservative, never executes the DML)
- PostgreSQL metadata-aware rules: `ddl.alter.drop_primary_key.forbid` detects `DROP CONSTRAINT` on primary keys via `pg_constraint` mapping, `ddl.alter.rename_column.exists.require` verifies column existence, `ddl.alter.rename_index.forbid` flags index renames, `ddl.alter.drop_column.exists.require` verifies column existence, `ddl.table.exists.create.forbid` checks table existence
- PostgreSQL metadata provider (`internal/infrastructure/metadata/postgresql`) with `pgx/v5` driver for schema introspection and planner queries
- Full E2E test suites for all three transports against PostgreSQL 17: CLI (9 shell-based cases), HTTP (9 Go subtests), MCP (9 Go subtests)

### Changed

- Documentation updated across all reference, concept, recipe, and landing pages to reflect PostgreSQL metadata-aware support
- `pgx/v5` promoted from indirect to direct dependency in `go.mod`

## [v0.13.1] - 2026-04-02

### Fixed

- Landing page inline i18n script no longer embeds unescaped SQL single quotes in the DDL / CI examples, which previously caused a browser-side `Unexpected string` syntax error and prevented the page JavaScript from loading

### Changed

- Release-facing docs, examples, landing content, and source-build defaults now align with `v0.13.1`

## [v0.13.0] - 2026-04-02

### Added

- HTTP `POST /v1/audit` now supports metadata-aware execution through direct `connection` inputs, including additive `context` fields that report resolved dialect, schema, and metadata source
- Shared `internal/interfaces/metadata` helpers now normalize direct connection validation and password resolution across HTTP and MCP adapters
- Docker-backed HTTP metadata e2e coverage now exercises real `deltascope-server` binaries against MySQL and TiDB fixtures

### Changed

- Release-facing docs, examples, landing content, and source-build defaults now align with `v0.13.0`
- HTTP metadata-aware requests snapshot the policy config per request so preparation and final audit read the same policy bytes
- HTTP API docs now describe direct credential lookup failures (`password_env`, `password_file`) under the stable `connection_invalid` error contract

## [v0.12.0] - 2026-04-02

### Added

- Structured naming governance for `CREATE TABLE` table, column, index, and explicitly named constraint objects, with configurable `prefix`, `suffix`, and `contains` requirements
- Reusable naming configuration helpers and rule primitives used across table, column, index, and constraint governance checks
- Application-layer and CLI end-to-end coverage for config-driven naming governance findings

### Changed

- Release-facing docs, examples, and landing content now present naming governance as the latest shipped milestone and align versioned install snippets to `v0.12.0`
- Foreign key naming governance stays policy-aware: it only applies when foreign keys are allowed by policy and remains suppressed by the shipped default `ddl.table.foreign_key.forbid` baseline

## [v0.11.1] - 2026-03-31

### Changed

- macOS install guidance now leads with Homebrew across README and landing page surfaces, while the portable installer remains the fallback for Linux and other environments
- `install.sh` now defaults to installing only `deltascope`, prompts interactive users to choose binaries and install directory, prints an install summary, and warns before requiring `sudo`
- `deltascope audit` now prints an interactive stdin hint before waiting for pasted SQL from a terminal session
- `deltascope rules list` and `deltascope rules search` now render shipped rules as an aligned ASCII table for easier scanning in terminals and screenshots
- CLI and reference docs now match the current install and rule-list output contracts in English and Chinese

## [v0.11.0] - 2026-03-30

### Added

- GitHub Actions composite action (`action.yml`) for one-step SQL audit in CI — supports `fail-on` severity threshold, optional PR comment, and auto-downloads the correct release binary
- `docs/examples/github-actions.yml` — caller workflow example for GitHub Actions
- `docs/examples/gitlab-ci.yml` — standalone GitLab CI job example
- `/readyz` endpoint alongside existing `/healthz`; both bypass auth and rate-limit
- Structured JSON access log lines from `accessLogMiddleware` (replaces plain-text format)
- SIGTERM/SIGINT graceful shutdown with 15-second drain timeout in `deltascope-server`

### Changed

- Auth and rate-limit allow-paths defaults now include `/readyz` in addition to `/healthz` and `/metrics`

## [v0.10.0] - 2026-03-29

### Added

- Gin-based HTTP adapter with middleware guardrails (request ID, panic recovery, timeout context, structured access logs)
- Optional `X-API-Key` authentication for HTTP audit endpoints with `401 auth_required` and `403 auth_invalid`
- Optional rate limiting with `429 rate_limited` support (`api-key` and `ip` strategies)
- Prometheus `/metrics` endpoint with HTTP request count and latency metrics
- `-trusted-proxies` flag to explicitly configure trusted proxy CIDRs for client IP extraction

### Fixed

- Removed Gin global mode side effect from library-level handler construction
- Added stale-entry cleanup for in-memory rate-limit key buckets

## [v0.9.2] - 2026-03-28

### Changed

- Documentation now aligns top-level install, MCP, skill, and architecture guidance with the current multi-surface product story and release metadata
- Release notes index now includes the published `v0.9.0`, `v0.9.1`, and `v0.9.2` entries for stable navigation from the root README

## [v0.9.1] - 2026-03-28

### Fixed

- CI release pipeline: removed redundant `npm publish --dry-run` call that caused false failures when publishing a new tag (npm rejects even dry-run publishes for already-published versions)

## [v0.9.0] - 2026-03-28

### Added

- Homebrew Cask distribution via `brew tap Fanduzi/deltascope && brew install --cask deltascope`
- Claude Code Skill `deltascope-review` for inline SQL review in Claude Code, Codex, Cursor and 40+ AI agents
- `skills/` directory with public Skill file and install documentation
- Install via `npx skills add Fanduzi/DeltaScope --skill deltascope-review`

## [v0.8.1] - 2026-03-28

### Fixed

- npm launcher package metadata now declares the canonical GitHub repository URL so npm provenance validation can accept CI publishes
- source-build default version and release-facing install links now point to `v0.8.1`

## [v0.8.0] - 2026-03-28

### Added

- npm launcher package `@fanduzi/deltascope-mcp` for copy-and-use MCP onboarding through `npx`
- dedicated DeltaScope MCP onboarding guides in English and Chinese with Claude Code, Codex, generic stdio, direct connection, and `connection_ref` examples
- launcher bootstrap diagnostics on `stderr`, GitHub release checksum verification, cache metadata, and override support for release mirrors

### Changed

- release workflow now validates and publishes the MCP launcher package alongside Go release assets
- README and recipe entrypoints now present MCP quick-start guidance and explicit Node 24+ / platform requirements
- launcher cache handling now uses lock timeout and stale-lock recovery to avoid wedged first-run installs

## [v0.7.0] - 2026-03-27

### Added

- Official `deltascope-mcp` stdio server with `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities`
- Structured MCP tool errors with stable machine-readable codes
- Metadata-aware MCP support for direct `connection` inputs and named `connection_ref` configs
- Shared `internal/application/auditmeta` preparation flow reused by both CLI and MCP adapters
- Explicit MCP output schemas and client-facing capability summaries
- Docker-backed MCP metadata e2e coverage for MySQL and TiDB, including direct and `connection_ref` flows

### Changed

- Release archives and the default installer now ship `deltascope-mcp` alongside `deltascope` and `deltascope-server`
- Source-build default version now points to `v0.7.0`
- English and Chinese README, recipes, release notes, and module docs now describe the official MCP contract and release surface

## [v0.6.2] - 2026-03-25

### Added

- Aggregate `explanation` blocks on audit results and per-statement results
- Structured per-finding explanation fields including `summary`, `why`, `risk`, and `suggestion`
- Metadata-availability notes on explanation metadata for metadata-aware findings
- Public API and HTTP coverage for explainable audit result shapes
- English and Chinese release notes for `v0.6.2`

### Changed

- Markdown CLI output now renders richer explanation details for findings and aggregate audit summaries
- Rule catalog entries now carry explanation-oriented metadata, examples, and remediation hints for discovery commands
- English and Chinese README, recipe, and reference docs now align with runtime output contracts and localized links
- Default source-build version target now points to `v0.6.2`
- Release-facing install examples now target `v0.6.2`

## [v0.5.0] - 2026-03-21

### Added

- Stable `pkg/deltascope` public API
- `deltascope` CLI with `audit`, `config init`, and `version`
- ASCII-logo version command and explicit `--version` behavior
- `deltascope-server` HTTP service with `healthz`, `version`, and `v1/audit`
- Offline-first DDL and DML rule catalog for MySQL and TiDB
- Optional metadata-aware instance facts and table snapshots
