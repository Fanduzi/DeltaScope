// Package cli exposes the command-line adapter for DeltaScope.
// input: Cobra command construction inputs, process-like stdin/stdout/stderr dependencies, and shared CLI option state
// output: root command wiring for audit, rules, config, capabilities, ddl-coverage, and version subcommands, with audit-only rendering and threshold flags kept off the root
// pos: CLI command assembly and shared option definitions
// note: if this file changes, update this header and module README.md.
package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

type cliOptions struct {
	ConfigPath             string
	Dialect                string
	Format                 string
	FailOn                 string
	Quiet                  bool
	Host                   string
	Port                   int
	User                   string
	PasswordEnv            string
	PasswordFile           string
	AskPassword            bool
	Schema                 string
	Database               string
	Socket                 string
	MetadataConnectTimeout string
	TLSMode                string
	TLSCAFile              string
	ShowVersion            bool
}

func newRootCmd(exitCode *int, stdin io.Reader, stdout io.Writer, stderr io.Writer) *cobra.Command {
	options := &cliOptions{
		Dialect: "mysql",
		Format:  "markdown",
		FailOn:  "blocker",
		Port:    3306,
	}

	rootCmd := &cobra.Command{
		Use:           "deltascope",
		Short:         rootCommandShort(),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if options.ShowVersion {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), renderVersionLine())
				return err
			}
			*exitCode = exitUser
			return newUserError("a subcommand is required")
		},
	}

	rootCmd.SetIn(stdin)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	rootCmd.PersistentFlags().StringVar(&options.ConfigPath, "config", "", "path to YAML policy config")
	rootCmd.PersistentFlags().StringVar(&options.Dialect, "dialect", options.Dialect, dialectFlagDescription())
	rootCmd.PersistentFlags().BoolVar(&options.Quiet, "quiet", false, "suppress non-result chatter")
	rootCmd.Flags().BoolVar(&options.ShowVersion, "version", false, "print the DeltaScope build version and supported dialects")

	rootCmd.AddCommand(newAuditCmd(options, exitCode))
	rootCmd.AddCommand(newQueryAccessCmd(options, exitCode))
	rootCmd.AddCommand(newRulesCmd(exitCode))
	rootCmd.AddCommand(newConfigCmd(options, exitCode))
	rootCmd.AddCommand(newCapabilitiesCmd(exitCode))
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newDDLCoverageCmd(exitCode))

	return rootCmd
}
