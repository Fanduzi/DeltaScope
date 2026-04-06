// Package cli exposes the command-line adapter for DeltaScope.
// input: capability command invocations plus shipped product-surface metadata
// output: stable human-readable summaries of supported dialects, modes, inputs, outputs, and shipped surfaces across CLI, HTTP, MCP, and Go API entrypoints
// pos: CLI capability discovery command above the current shipped product surface
// note: if this file changes, update this header and module README.md.
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
	return fmt.Sprintf(`deltascope capabilities

dialects:
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

metadata facts:
- schema context
- instance facts
- target table snapshots

surfaces:
- cli
- http
- mcp
- go-api
`, renderCapabilityDialects())
}

func renderCapabilityDialects() string {
	return strings.Join(prefixedLines("- ", supportedDialects()), "\n")
}

func prefixedLines(prefix string, values []string) []string {
	lines := make([]string, len(values))
	for index, value := range values {
		lines[index] = prefix + value
	}
	return lines
}
