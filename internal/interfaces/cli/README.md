# CLI Module

CLI adapter layer for the DeltaScope application.

## Files

| File | Responsibility |
|------|---------------|
| cli.go | Bridges process execution into the testable CLI executor, normalizes legacy audit prefix flag forms without exposing them to Query Access, and maps unset Cobra usage errors (including prefixed flag forms) to exit 2, except query-access usage which stays 3 |
| root.go | Builds the Cobra root command and `audit`/`query-access`/other subcommand wiring, shared CLI option state, stable error/exit-code mapping, and root flags while keeping audit-only rendering/threshold flags local to `audit` |
| audit.go | Implements the `audit` subcommand, audit-local `--format`/`--fail-on` validation and rendering, preserved partial audit results plus located parser diagnostics, SQL input loading, interactive stdin hinting, dialect-aware MySQL/TiDB catalog alias and conflict normalization before known-dialect open, PostgreSQL schema/database usage validation, password/password-env/password-file resolution, metadata-connect-timeout parsing, password prompting, and shared bounded online connection/authentication/identity/TLS error mapping (including TLS hostname/CA/server-offer/verification/handshake categories and exit 2 vs 3), quiet/normal/github-summary rendering, offline existence caveats on markdown Action Summary / quiet context / JSON context, and fail-threshold logic |
| audit_metadata.go | Bridges CLI metadata-aware options (including connect timeout, normalized port, and MySQL/TiDB database catalog selection for explicit and auto-detected opens) into the shared metadata-preparation flow and MySQL-compatible client opener, and attaches the shared offline `context.note` / `context.unproven` when existence was not checked |
| cli_impact_postgresql_tag_test.go | Verifies PostgreSQL offline primary-key equality impact in CLI JSON output |
| query_access.go | Implements the `query-access analyze` subcommand with flag-presence-aware `--sql`/`--file`/stdin loading, immediate explicit-empty validation, bounded file-input errors, dialect-aware connection flags, mode/dialect/schema flags, fixed JSON output, admission-based exit codes, identity-routed online analysis through the opaque unified SDK session, shared bounded connection/authentication/TLS translation, and the bounded PostgreSQL 17 requirement; audit-only `--format`/`--fail-on` are rejected as usage errors |
| query_access_test.go | Verifies query-access help excludes audit-only flags, explicit audit-only flags fail with no analysis output in both command positions, fixed JSON output, admission exit codes, field names, mode handling, explicit-empty SQL without stdin reads, empty stdin, bounded missing-file errors, valid stdin/file inputs, unified online entry wiring with an empty analysis dialect, bounded generic constructor/capability/authentication failure presentation, close ownership, and no-audit-field-leakage |
| query_access_unified_entry_test.go | Structurally verifies formatted `runQueryAccessOnline` source contains no product inspection or dialect-specific Query Access constructor/analysis calls and uses both unified SDK entry seams |
| query_access_postgresql_online_recording_test.go | Focused recording-driver proof that the CLI PostgreSQL unified online path delegates COUNT(1), closes its session once, maps bounded cancellation/closed-session/connection/catalog failures, and never sends user SQL, EXPLAIN, or prepare operations |
| query_access_e2e_mixed_literal_test.go | Docker-backed built-binary smoke for admitted and fail-closed MySQL 8.4 and TiDB 8.5 Query Access routes, plus CLI default/offline indeterminate result and marker no-leak evidence |
| query_access_probe_boundary_no_leak_test.go | No-leak regression for the MySQL/TiDB builtin-identity probe boundary on the CLI surface: asserts injected markers, identity facts, candidates, session/context, manifest, raw SQL, and `severity` are absent from stdout/stderr/JSON |
| query_access_postgresql_no_leak_test.go | PostgreSQL 17 integration no-leak coverage for online `COUNT(1)`, excluded shapes, and default-offline CLI paths |
| rules.go | Implements `rules list` (with dialect/level/kind/category/search/format/limit filters) and `rules explain <rule-id>` on top of the shipped rule catalog, including text and JSON output |
| config.go | Implements the `config` command group, including `lint` (semantic validation plus rule-level replacement-hazard warnings and `--strict`), `show-default`, and wiring for `status` |
| config_init.go | Implements `config init` and emits a deterministic default YAML template with empty string params encoded as `""` |
| config_init_test.go | Verifies `config init` / `show-default` / shipped example YAML lint clean, encode empty strings as quoted YAML, preserve a hand-written full-spec override, and leave default-policy audit findings unchanged |
| config_status.go | Implements `config status <rule-id>`, showing the effective ON/OFF state, level, default/current snapshots, and config effect for one rule via the config status application service, with text and JSON output |
| capabilities.go | Implements the `capabilities` summary command and shared rendering helpers for human/agent discovery of shipped dialects, modes, inputs, outputs, and public surfaces (`cli`, `http`, `mcp`, `go-api`) |
| ddl_coverage.go | Implements the `ddl-coverage` command for querying the generated (embedded) DDL coverage catalog with text and JSON output, flag validation, filter rendering, and exit-2 catalog-unavailable mapping for a missing `--catalog` override |
| capability_surface.go | Defines the pure-Go build capability surface and root CLI wording |
| capability_surface_pg.go | Defines the PostgreSQL-tagged build capability surface and root CLI wording |
| version.go | Implements the `version` subcommand with ASCII logo plus build-version and supported-dialect output |
| cli_test.go | Verifies input modes including explicit empty/whitespace `--sql` fail-closed without reading stdin, empty `--file` rejection, legacy audit prefix output/threshold flags, connection/password UX, exit-code behavior, capability/version wording surfaces, audit context output, explanation rendering in Markdown/JSON results, the user-facing Action Summary markdown contract (section presence, rule explain command, statement index, clean-result omission, JSON/quiet non-regression, and no severity field), the aggregated rule-summary markdown contract (bounded `### Skip Reasons`, no `## Skipped Rules` section, no skipped rule IDs) and JSON rule-summary list preservation, github-summary format coverage (REJECT/PASS verdict, action summary, clean-result omission, no raw SQL, no severity, help advertising, unsupported-format messaging), and audit-only help advertising of all output formats |
| cli_offline_existence_test.go | Locks offline ALTER DROP COLUMN / ALTER missing-table as pass, with existence-not-checked copy on markdown Action Summary, quiet `[context]`, and JSON `context.note` / `context.unproven`, and documents that `--quiet --format json` keeps the JSON contract |
| cli_user_input_exit_test.go | Verifies unknown flags and unparseable SQL exit 2, parser-error JSON keeps an empty verdict with `diagnostics[].classification == parser_error`, and existing format/dialect/missing-file user errors stay at exit 2 |
| cli_metadata_connection_exit_test.go | Verifies metadata-aware connection failures exit 3 with bounded dial/auth/timeout and TLS category messages, typed MySQL/pgx TLS signals, no-leak output, omitted password source after auth failure exits 2, and missing `--password-env` stays exit 2 without connecting |
| cli_unsupported_diagnostics_evidence_test.go | Verifies mixed parser-error JSON preserves valid statements/findings, and text renderers show bounded diagnostic locations without leaking SQL |
| ddl_coverage_test.go | Verifies ddl-coverage command filtering, text/JSON output, empty results, invalid flags, no-leak sanity across all 400 catalog entries, embedded-catalog lookup from an empty working directory, and exit 2 when a `--catalog` override is missing |
| rules_catalog_test.go | Verifies rules list filtering (dialect, level, kind, category, search, limit), rules explain detail output, text/JSON formats, invalid flags, empty results, and no-severity sanity |
| audit_metadata_test.go | Verifies metadata-aware CLI wiring for dialect detection, MySQL/TiDB database/schema aliases and conflicts, schema inference, PostgreSQL schema/database usage validation, create-table partial behavior, dialect-aware port defaults, and metadata-connect-timeout flag validation |
| config_status_test.go | Verifies config status text/JSON output, partial-replacement danger wording, disabled-rule wording, and error mapping (missing rule id, unknown rule, invalid format, invalid config) with no severity field |
| config_lint_test.go | Verifies config lint warnings (level-only replacement hazard), `Config OK` / `Config OK with warnings` output and exit-code matrix, `--strict`, error precedence, deterministic warning ordering, existing invalid-value errors, and YAML-null string params still failing the type check, with no severity field |

## Exports

- `Run()`
- `Execute(ctx, args, stdin, stdout, stderr) int`

## Dependencies
- Upstream: `cmd/deltascope`
- Downstream: `bufio`, `database/sql`, `encoding/json`, `internal/application/audit`, `internal/application/auditmeta`, `internal/application/configlint`, `internal/application/configstatus`, `internal/domain/policy`, `internal/domain/report`, `internal/domain/rule/catalog`, `internal/domain/spec`, `internal/infrastructure/config/viper`, `internal/infrastructure/metadata/mysql`, `internal/infrastructure/output/githubsummary`, `internal/infrastructure/output/json`, `internal/infrastructure/output/markdown`, `internal/interfaces/metadata`, `pkg/deltascope`, `github.com/spf13/cobra`, `golang.org/x/term`

## Notes
- Online `query-access analyze` keeps connection/TLS/credential lifecycle in the CLI, then passes the caller-owned pinned connection to the opaque unified SDK session without inspecting observed product or constraining the analysis request dialect. Reachable PostgreSQL identities outside PG17 return exit 3 with the fixed bounded message `online PostgreSQL Query Access requires PostgreSQL 17`; authentication, dial, timeout, and TLS mappings remain separate.
- Query Access semantic breadth and detailed probe tests live in the unified SDK suite; this module retains only CLI-owned transport, sink, lifecycle, and real-route evidence.
- `deltascope --version` prints the build version plus compiled dialect surface.
- `deltascope version` prints the ASCII logo plus the build version and compiled dialect surface.
- Audit file, inline, and stdin SQL pass through the shared one-leading-UTF-8-BOM normalization before parsing.
- A parser error in one bounded migration statement still renders audited siblings in source order, emits only the bad statement's safe diagnostic, and exits 2.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
