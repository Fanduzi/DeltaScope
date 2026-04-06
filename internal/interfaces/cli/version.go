// Package cli exposes the command-line adapter for DeltaScope.
// input: version command invocations and the build-time version variable
// output: a stable printable version string for users and scripts
// pos: CLI metadata command implementation
// note: if this file changes, update this header and module README.md.
package cli

import (
	"fmt"
	"strings"

	publicapi "github.com/Fanduzi/DeltaScope/pkg/deltascope"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the DeltaScope logo, build version, and supported dialects",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s\n", publicapi.Logo, renderVersionLine())
			return err
		},
	}
}

func renderVersionLine() string {
	return fmt.Sprintf("deltascope %s (%s)", Version, strings.Join(supportedDialects(), ", "))
}
