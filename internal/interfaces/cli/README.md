# CLI Module

CLI adapter layer for the DeltaScope application.

## Files

| File | Responsibility |
|------|---------------|
| cli.go | Bridges process execution into the testable CLI executor |
| root.go | Builds the Cobra root command, shared CLI option state, and stable error/exit-code mapping |
| audit.go | Implements the `audit` subcommand, SQL input loading, MySQL-style connection flag parsing, password prompting, quiet/normal rendering, and fail-threshold logic |
| config_init.go | Implements `config init` and emits a deterministic default YAML template |
| version.go | Implements the `version` subcommand with ASCII logo plus build-version output |
| cli_test.go | Verifies input modes, connection/password UX, exit-code behavior, and basic command output |

## Exports

- `Run()`
- `Execute(ctx, args, stdin, stdout, stderr) int`

## Dependencies
- Upstream: `cmd/deltascope`
- Downstream: `bufio`, `internal/application/audit`, `internal/domain/policy`, `internal/domain/report`, `internal/domain/spec`, `internal/infrastructure/config/viper`, `internal/infrastructure/output/json`, `internal/infrastructure/output/markdown`, `github.com/spf13/cobra`, `golang.org/x/term`

## Notes
- `deltascope --version` prints only the semantic version for scripts.
- `deltascope version` prints the ASCII logo plus the semantic version for humans.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
