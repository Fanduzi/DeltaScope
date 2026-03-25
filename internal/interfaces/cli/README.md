# CLI Module

CLI adapter layer for the DeltaScope application.

## Files

| File | Responsibility |
|------|---------------|
| cli.go | Bridges process execution into the testable CLI executor |
| root.go | Builds the Cobra root command, shared CLI option state, and stable error/exit-code mapping |
| audit.go | Implements the `audit` subcommand, SQL input loading, MySQL-style connection flag parsing, password prompting, quiet/normal rendering, and fail-threshold logic |
| audit_metadata.go | Opens metadata-aware clients, auto-detects dialects, infers schemas, and attaches provider wiring for online audit runs |
| rules.go | Implements `rules list`, `rules show`, and `rules search` on top of the shipped rule catalog |
| config.go | Implements the `config` command group, including `lint` and `show-default` validation/inspection flows |
| config_init.go | Implements `config init` and emits a deterministic default YAML template |
| capabilities.go | Implements the `capabilities` summary command for human/agent CLI discovery |
| version.go | Implements the `version` subcommand with ASCII logo plus build-version output |
| cli_test.go | Verifies input modes, connection/password UX, exit-code behavior, audit context output, and explanation rendering in Markdown/JSON results |
| audit_metadata_test.go | Verifies metadata-aware CLI wiring for dialect detection, schema inference, and create-table partial behavior |

## Exports

- `Run()`
- `Execute(ctx, args, stdin, stdout, stderr) int`

## Dependencies
- Upstream: `cmd/deltascope`
- Downstream: `bufio`, `database/sql`, `internal/application/audit`, `internal/domain/policy`, `internal/domain/report`, `internal/domain/spec`, `internal/infrastructure/config/viper`, `internal/infrastructure/metadata/mysql`, `internal/infrastructure/output/json`, `internal/infrastructure/output/markdown`, `github.com/spf13/cobra`, `golang.org/x/term`

## Notes
- `deltascope --version` prints only the semantic version for scripts.
- `deltascope version` prints the ASCII logo plus the semantic version for humans.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
