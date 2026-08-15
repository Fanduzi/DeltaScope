# CLI Module

CLI adapter layer for the DeltaScope application.

## Files

| File | Responsibility |
|------|---------------|
| cli.go | Bridges process execution into the testable CLI executor |
| root.go | Builds the Cobra root command, shared CLI option state, and stable error/exit-code mapping |
| audit.go | Implements the `audit` subcommand, SQL input loading, interactive stdin hinting, MySQL-style connection flag parsing, password/password-env/password-file resolution, metadata-connect-timeout parsing, password prompting, quiet/normal/github-summary rendering, and fail-threshold logic |
| audit_metadata.go | Bridges CLI metadata-aware options (including connect timeout) into the shared metadata-preparation flow and MySQL-compatible client opener |
| query_access.go | Implements the `query-access analyze` subcommand with SQL input loading, mode/dialect/schema flags, JSON output, admission-based exit codes, identity-routed online analysis through the opaque unified SDK session, and bounded connection-failure translation for generic constructor/capability sentinels |
| query_access_test.go | Verifies query-access command JSON output, exit codes, field names, mode handling, unified online entry wiring with an empty analysis dialect, bounded generic constructor/capability failure presentation, close ownership, and no-audit-field-leakage |
| query_access_unified_entry_test.go | Structurally verifies formatted `runQueryAccessOnline` source contains no product inspection or dialect-specific Query Access constructor/analysis calls and uses both unified SDK entry seams |
| query_access_postgresql_online_recording_test.go | Focused recording-driver proof that the CLI PostgreSQL unified online path delegates COUNT(1), closes its session once, maps bounded cancellation/closed-session/connection/catalog failures, and never sends user SQL, EXPLAIN, or prepare operations |
| query_access_e2e_mixed_literal_test.go | Docker-backed built-binary smoke for admitted and fail-closed MySQL 8.4 and TiDB 8.5 Query Access routes, including CLI exit-code, result, and marker no-leak evidence |
| query_access_probe_boundary_no_leak_test.go | No-leak regression for the MySQL/TiDB builtin-identity probe boundary on the CLI surface: asserts injected markers, identity facts, candidates, session/context, manifest, raw SQL, and `severity` are absent from stdout/stderr/JSON |
| query_access_postgresql_no_leak_test.go | PostgreSQL 17 integration no-leak coverage for online `COUNT(1)`, excluded shapes, and default-offline CLI paths |
| rules.go | Implements `rules list` (with dialect/level/kind/category/search/format/limit filters) and `rules explain <rule-id>` on top of the shipped rule catalog, including text and JSON output |
| config.go | Implements the `config` command group, including `lint` (semantic validation plus rule-level replacement-hazard warnings and `--strict`), `show-default`, and wiring for `status` |
| config_init.go | Implements `config init` and emits a deterministic default YAML template |
| config_status.go | Implements `config status <rule-id>`, showing the effective ON/OFF state, level, default/current snapshots, and config effect for one rule via the config status application service, with text and JSON output |
| capabilities.go | Implements the `capabilities` summary command and shared rendering helpers for human/agent discovery of shipped dialects, modes, inputs, outputs, and public surfaces (`cli`, `http`, `mcp`, `go-api`) |
| ddl_coverage.go | Implements the `ddl-coverage` command for querying the generated DDL coverage catalog with text and JSON output, flag validation, and filter rendering |
| capability_surface.go | Defines the pure-Go build capability surface and root CLI wording |
| capability_surface_pg.go | Defines the PostgreSQL-tagged build capability surface and root CLI wording |
| version.go | Implements the `version` subcommand with ASCII logo plus build-version and supported-dialect output |
| cli_test.go | Verifies input modes, connection/password UX, exit-code behavior, capability/version wording surfaces, audit context output, explanation rendering in Markdown/JSON results, the user-facing Action Summary markdown contract (section presence, rule explain command, statement index, clean-result omission, JSON/quiet non-regression, and no severity field), github-summary format coverage (REJECT/PASS verdict, action summary, clean-result omission, no raw SQL, no severity, help advertising, unsupported-format messaging), and help advertising of all output formats |
| ddl_coverage_test.go | Verifies ddl-coverage command filtering, text/JSON output, empty results, invalid flags, and no-leak sanity across all 400 catalog entries |
| rules_catalog_test.go | Verifies rules list filtering (dialect, level, kind, category, search, limit), rules explain detail output, text/JSON formats, invalid flags, empty results, and no-severity sanity |
| audit_metadata_test.go | Verifies metadata-aware CLI wiring for dialect detection, schema inference, create-table partial behavior, and metadata-connect-timeout flag validation |
| config_status_test.go | Verifies config status text/JSON output, partial-replacement danger wording, disabled-rule wording, and error mapping (missing rule id, unknown rule, invalid format, invalid config) with no severity field |
| config_lint_test.go | Verifies config lint warnings (level-only replacement hazard), `Config OK` / `Config OK with warnings` output and exit-code matrix, `--strict`, error precedence, deterministic warning ordering, and existing invalid-value errors with no severity field |

## Exports

- `Run()`
- `Execute(ctx, args, stdin, stdout, stderr) int`

## Dependencies
- Upstream: `cmd/deltascope`
- Downstream: `bufio`, `database/sql`, `encoding/json`, `internal/application/audit`, `internal/application/auditmeta`, `internal/application/configlint`, `internal/application/configstatus`, `internal/domain/policy`, `internal/domain/report`, `internal/domain/rule/catalog`, `internal/domain/spec`, `internal/infrastructure/config/viper`, `internal/infrastructure/metadata/mysql`, `internal/infrastructure/output/githubsummary`, `internal/infrastructure/output/json`, `internal/infrastructure/output/markdown`, `internal/interfaces/metadata`, `pkg/deltascope`, `github.com/spf13/cobra`, `golang.org/x/term`

## Notes
- Online `query-access analyze` keeps connection/TLS/credential lifecycle in the CLI, then passes the caller-owned pinned connection to the opaque unified SDK session without inspecting observed product or constraining the analysis request dialect.
- Query Access semantic breadth and detailed probe tests live in the unified SDK suite; this module retains only CLI-owned transport, sink, lifecycle, and real-route evidence.
- `deltascope --version` prints the build version plus compiled dialect surface.
- `deltascope version` prints the ASCII logo plus the build version and compiled dialect surface.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
