# CLI Module

CLI adapter layer for the DeltaScope application.

## Files

| File | Responsibility |
|------|---------------|
| cli.go | Bridges process execution into the testable CLI executor |
| root.go | Builds the Cobra root command and maps errors to stable exit codes |
| audit.go | Implements the `audit` subcommand, input loading, rendering, and fail-threshold logic |
| config_init.go | Implements `config init` and emits a default YAML template |
| version.go | Implements the `version` subcommand and build-version output |
| cli_test.go | Verifies input modes, exit-code behavior, and basic command output |

## Exports

- `Run()`
- `Execute(ctx, args, stdin, stdout, stderr) int`

## Dependencies
- Upstream: `cmd/deltascope`
- Downstream: `internal/application/audit`, `internal/domain/policy`, `internal/domain/report`, `internal/domain/spec`, `internal/infrastructure/config/viper`, `internal/infrastructure/output/json`, `internal/infrastructure/output/markdown`, `github.com/spf13/cobra`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
