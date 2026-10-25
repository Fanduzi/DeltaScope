// Package cli exposes the command-line adapter for DeltaScope.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newCapabilitiesCmd(exitCode *int) *cobra.Command {
	return &cobra.Command{
		Use:   "capabilities",
		Short: "Print a concise product capability summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := fmt.Fprint(cmd.OutOrStdout(), renderCapabilities()); err != nil {
				*exitCode = exitInternal
				return err
			}
			*exitCode = exitOK
			return nil
		},
	}
}

func renderCapabilities() string {
	buildNoteSection := ""
	if note := capabilityBuildNote(); note != "" {
		buildNoteSection = fmt.Sprintf("\nbuild notes:\n- %s\n", note)
	}

	return fmt.Sprintf(`deltascope capabilities

available in this build:
%s

product dialects:
%s

inputs:
- --sql
- --file
- stdin

outputs:
- markdown
- json
- quiet markdown lines

modes:
- offline
- metadata-aware

query access modes:
- strict
- projection_only

metadata facts:
- schema context
- instance facts
- target table snapshots

surfaces:
- cli
- http
- mcp
- go-api
%s`, renderCapabilityDialects(supportedDialects()), renderCapabilityDialects(productDialects()), buildNoteSection)
}

func renderCapabilityDialects(values []string) string {
	return strings.Join(prefixedLines("- ", values), "\n")
}

func prefixedLines(prefix string, values []string) []string {
	lines := make([]string, len(values))
	for index, value := range values {
		lines[index] = prefix + value
	}
	return lines
}
