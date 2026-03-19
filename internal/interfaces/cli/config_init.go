// Package cli exposes the command-line adapter for DeltaScope.
// input: config init command invocations and the built-in default policy model
// output: a usable YAML policy template for local users and automation
// pos: CLI config generation command for bootstrapping DeltaScope policy files
// note: if this file changes, update this header and module README.md.
package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/spf13/cobra"
)

func newConfigInitCmd(exitCode *int) *cobra.Command {
	return &cobra.Command{
		Use:   "config init",
		Short: "Print a usable YAML policy template",
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

func renderExampleConfig() ([]byte, error) {
	cfg := policy.Default()
	ruleIDs := make([]string, 0, len(cfg.Rules))
	for ruleID := range cfg.Rules {
		ruleIDs = append(ruleIDs, ruleID)
	}
	sort.Strings(ruleIDs)

	var builder strings.Builder
	builder.WriteString("rules:\n")
	for _, ruleID := range ruleIDs {
		ruleCfg := cfg.Rules[ruleID]
		builder.WriteString(fmt.Sprintf("  %s:\n", ruleID))
		builder.WriteString(fmt.Sprintf("    enabled: %t\n", ruleCfg.Enabled))
		builder.WriteString(fmt.Sprintf("    level: %s\n", ruleCfg.Level))
		builder.WriteString("    params:\n")

		paramKeys := make([]string, 0, len(ruleCfg.Params))
		for key := range ruleCfg.Params {
			paramKeys = append(paramKeys, key)
		}
		sort.Strings(paramKeys)
		for _, key := range paramKeys {
			builder.WriteString(fmt.Sprintf("      %s: %v\n", key, ruleCfg.Params[key]))
		}
		builder.WriteString("\n")
	}

	return []byte(builder.String()), nil
}
