// Package cli exposes the command-line adapter for DeltaScope.
// input: config command invocations and YAML config files
// output: config lint results with validation errors and replacement warnings, default-config rendering, and rule config status wiring
// pos: CLI config command group for policy inspection and validation
// note: if this file changes, update this header and module README.md.
package cli

import (
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/application/configlint"
	"github.com/spf13/cobra"
)

func newConfigCmd(options *cliOptions, exitCode *int) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Generate and validate DeltaScope config",
	}
	cmd.AddCommand(newConfigInitCmd(exitCode))
	cmd.AddCommand(newConfigLintCmd(exitCode))
	cmd.AddCommand(newConfigShowDefaultCmd(exitCode))
	cmd.AddCommand(newConfigStatusCmd(options, exitCode))
	return cmd
}

func newConfigLintCmd(exitCode *int) *cobra.Command {
	var filePath string
	var strict bool

	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Validate a DeltaScope YAML config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if filePath == "" {
				*exitCode = exitUser
				return newUserError("config lint requires --file")
			}

			result, err := configlint.Inspect(cmd.Context(), configlint.Request{Path: filePath})
			if err != nil {
				// Validation errors (unknown rule, invalid level, unknown param, param type
				// mismatch, malformed YAML, missing/unreadable file) take precedence over
				// warnings and map to the same user-error path the CLI always used.
				*exitCode = exitUser
				return err
			}

			out := cmd.OutOrStdout()
			if len(result.Warnings) == 0 {
				if _, err := fmt.Fprintln(out, "Config OK"); err != nil {
					*exitCode = exitInternal
					return err
				}
				*exitCode = exitOK
				return nil
			}

			// No errors but replacement-hazard warnings. Warnings are advisory, so the
			// default exit code stays 0; --strict promotes a warnings-only result to exit 2.
			// The warning text goes to stdout so the output reads like a successful lint.
			if _, err := fmt.Fprint(out, renderConfigLintWarnings(result.Warnings, filePath)); err != nil {
				*exitCode = exitInternal
				return err
			}
			if strict {
				*exitCode = exitUser
				return nil
			}
			*exitCode = exitOK
			return nil
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "path to the YAML config file to lint")
	cmd.Flags().BoolVar(&strict, "strict", false, "fail when lint warnings are present")
	return cmd
}

// renderConfigLintWarnings renders the deterministic warning block for a config that lints
// clean except for rule-level replacement hazards. configlint returns warnings already
// ordered by rule_id then field (enabled, level, params.<key>), and each message already
// carries the rule id, the omitted field, the "replaces the whole rule policy; it does not
// merge with defaults" framing, and the effective consequence. A message may span multiple
// lines: the first line is the bullet and continuation lines indent two spaces. Each warning
// is followed by an "Inspect effective rule status:" handoff pointing at
// `deltascope config status <rule-id> --config <configPath>` so the user can confirm the
// effective result. configPath is the --file path supplied to `config lint`. The text output
// never introduces a severity field, and --strict prints this same text byte-for-byte.
func renderConfigLintWarnings(warnings []configlint.Warning, configPath string) string {
	var b strings.Builder
	b.WriteString("Config OK with warnings\n\n")
	b.WriteString("Warnings:\n")
	for _, warning := range warnings {
		lines := strings.Split(warning.Message, "\n")
		for i, line := range lines {
			if i == 0 {
				fmt.Fprintf(&b, "- %s\n", line)
			} else {
				fmt.Fprintf(&b, "  %s\n", line)
			}
		}
		b.WriteString("  Inspect effective rule status:\n")
		fmt.Fprintf(&b, "    deltascope config status %s --config %s\n", warning.RuleID, configPath)
	}
	return b.String()
}

func newConfigShowDefaultCmd(exitCode *int) *cobra.Command {
	return &cobra.Command{
		Use:   "show-default",
		Short: "Print the built-in default config",
		RunE: func(cmd *cobra.Command, args []string) error {
			output, err := renderExampleConfig()
			if err != nil {
				*exitCode = exitInternal
				return err
			}
			if _, err := cmd.OutOrStdout().Write(output); err != nil {
				*exitCode = exitInternal
				return err
			}
			*exitCode = exitOK
			return nil
		},
	}
}
