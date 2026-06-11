# CLI Module

CLI adapter layer for the DeltaScope application.

## Files

| File | Responsibility |
|------|---------------|
| cli.go | Bridges process execution into the testable CLI executor |
| root.go | Builds the Cobra root command, shared CLI option state, and stable error/exit-code mapping |
| audit.go | Implements the `audit` subcommand, SQL input loading, interactive stdin hinting, MySQL-style connection flag parsing, password/password-env/password-file resolution, metadata-connect-timeout parsing, password prompting, quiet/normal rendering, and fail-threshold logic |
| audit_metadata.go | Bridges CLI metadata-aware options (including connect timeout) into the shared metadata-preparation flow and MySQL-compatible client opener |
| rules.go | Implements `rules list` (with dialect/level/kind/category/search/format/limit filters) and `rules explain <rule-id>` on top of the shipped rule catalog, including text and JSON output |
| config.go | Implements the `config` command group, including `lint` and `show-default` validation/inspection flows |
| config_init.go | Implements `config init` and emits a deterministic default YAML template |
| capabilities.go | Implements the `capabilities` summary command and shared rendering helpers for human/agent discovery of shipped dialects, modes, inputs, outputs, and public surfaces (`cli`, `http`, `mcp`, `go-api`) |
| ddl_coverage.go | Implements the `ddl-coverage` command for querying the generated DDL coverage catalog with text and JSON output, flag validation, and filter rendering |
| capability_surface.go | Defines the pure-Go build capability surface and root CLI wording |
| capability_surface_pg.go | Defines the PostgreSQL-tagged build capability surface and root CLI wording |
| version.go | Implements the `version` subcommand with ASCII logo plus build-version and supported-dialect output |
| cli_test.go | Verifies input modes, connection/password UX, exit-code behavior, capability/version wording surfaces, audit context output, and explanation rendering in Markdown/JSON results |
| ddl_coverage_test.go | Verifies ddl-coverage command filtering, text/JSON output, empty results, invalid flags, and no-leak sanity across all 400 catalog entries |
| rules_catalog_test.go | Verifies rules list filtering (dialect, level, kind, category, search, limit), rules explain detail output, text/JSON formats, invalid flags, empty results, and no-severity sanity |
| audit_metadata_test.go | Verifies metadata-aware CLI wiring for dialect detection, schema inference, create-table partial behavior, and metadata-connect-timeout flag validation |

## Exports

- `Run()`
- `Execute(ctx, args, stdin, stdout, stderr) int`

## Dependencies
- Upstream: `cmd/deltascope`
- Downstream: `bufio`, `database/sql`, `encoding/json`, `internal/application/audit`, `internal/application/auditmeta`, `internal/domain/policy`, `internal/domain/report`, `internal/domain/rule/catalog`, `internal/domain/spec`, `internal/infrastructure/config/viper`, `internal/infrastructure/metadata/mysql`, `internal/infrastructure/output/json`, `internal/infrastructure/output/markdown`, `internal/interfaces/metadata`, `github.com/spf13/cobra`, `golang.org/x/term`

## Notes
- `deltascope --version` prints the build version plus compiled dialect surface.
- `deltascope version` prints the ASCII logo plus the build version and compiled dialect surface.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
