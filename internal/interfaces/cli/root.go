// Package cli exposes the command-line adapter for DeltaScope.
// input: Cobra command construction inputs and process-like stdin/stdout/stderr dependencies
// output: root command wiring for audit, config init, and version subcommands
// pos: CLI command assembly and shared option definitions
// note: if this file changes, update this header and module README.md.
package cli

import (
	"io"

	"github.com/spf13/cobra"
)

type cliOptions struct {
	ConfigPath string
	Dialect    string
	Format     string
	FailOn     string
	Quiet      bool
}

func newRootCmd(exitCode *int, stdin io.Reader, stdout io.Writer, stderr io.Writer) *cobra.Command {
	options := &cliOptions{
		Dialect: "mysql",
		Format:  "markdown",
		FailOn:  "blocker",
	}

	rootCmd := &cobra.Command{
		Use:           "deltascope",
		Short:         "Offline SQL review for MySQL and TiDB",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			switch options.Format {
			case "markdown", "json":
			default:
				*exitCode = exitUser
				return newUserError("format must be markdown or json")
			}

			switch options.FailOn {
			case "blocker", "warning", "notice", "none":
				return nil
			default:
				*exitCode = exitUser
				return newUserError("unsupported fail threshold: use blocker, warning, notice, or none")
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			*exitCode = exitUser
			return newUserError("a subcommand is required")
		},
	}

	rootCmd.SetIn(stdin)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	rootCmd.PersistentFlags().StringVar(&options.ConfigPath, "config", "", "path to YAML policy config")
	rootCmd.PersistentFlags().StringVar(&options.Dialect, "dialect", options.Dialect, "SQL dialect: mysql or tidb")
	rootCmd.PersistentFlags().StringVar(&options.Format, "format", options.Format, "output format: markdown or json")
	rootCmd.PersistentFlags().StringVar(&options.FailOn, "fail-on", options.FailOn, "non-zero threshold: blocker, warning, notice, or none")
	rootCmd.PersistentFlags().BoolVar(&options.Quiet, "quiet", false, "suppress non-result chatter")

	rootCmd.AddCommand(newAuditCmd(options, exitCode))
	rootCmd.AddCommand(newConfigInitCmd(exitCode))
	rootCmd.AddCommand(newVersionCmd())

	return rootCmd
}
