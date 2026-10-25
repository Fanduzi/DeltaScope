// Package cli exposes the command-line adapter for DeltaScope.
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
	return fmt.Sprintf("deltascope %s (%s)", resolvedVersion(), strings.Join(supportedDialects(), ", "))
}

func resolvedVersion() string {
	if Version != "" {
		return Version
	}
	return publicapi.ReportedVersion()
}
