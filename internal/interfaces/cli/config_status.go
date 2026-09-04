// Package cli exposes the command-line adapter for DeltaScope.
// input: a rule ID, the global --config policy path, and the config status application service
// output: human and JSON status for one shipped rule under the default policy plus optional config, including Loaded and fk_forbid suppression
// pos: CLI config status command above the application config status use case
// note: if this file changes, update this header and module README.md.
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/application/configstatus"
	"github.com/spf13/cobra"
)

func newConfigStatusCmd(options *cliOptions, exitCode *int) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "status <rule-id>",
		Short: "Show the effective status of one rule under the current config",
		Long: "Show whether one shipped rule is ON or OFF, its effective level, and how your config\n" +
			"changes it. The global --config flag selects the policy file; without --config the built-in\n" +
			"default policy applies. Does not execute audits, parse SQL, or call the audit service.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "text" && format != "json" {
				*exitCode = exitUser
				return newUserError(fmt.Sprintf("invalid format %q: must be text or json", format))
			}

			result, err := configstatus.Inspect(cmd.Context(), configstatus.Request{
				RuleID:     args[0],
				ConfigPath: options.ConfigPath,
			})
			if err != nil {
				*exitCode = exitUser
				return err
			}

			var output string
			if format == "json" {
				output, err = renderConfigStatusJSON(result)
			} else {
				output = renderConfigStatusText(result)
			}
			if err != nil {
				*exitCode = exitInternal
				return err
			}

			if _, err := fmt.Fprint(cmd.OutOrStdout(), output); err != nil {
				*exitCode = exitInternal
				return err
			}
			*exitCode = exitOK
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")

	return cmd
}

// configStatusJSONOutput is the CLI JSON envelope. It wraps the core Result with a top-level
// version, matching the rules list / ddl-coverage JSON convention. Version is a CLI/build
// surface concern and intentionally does not live on the application-layer Result; the core
// stays free of build metadata. The public priority field is level, never severity.
type configStatusJSONOutput struct {
	Version            string                          `json:"version"`
	RuleID             string                          `json:"rule_id"`
	Status             configstatus.RuleStatus         `json:"status"`
	Default            configstatus.RulePolicySnapshot `json:"default"`
	Current            configstatus.RulePolicySnapshot `json:"current"`
	ConfigEffect       configstatus.ConfigEffect       `json:"config_effect"`
	Suppression        *configstatus.Suppression       `json:"suppression,omitempty"`
	RuleDetailsCommand string                          `json:"rule_details_command"`
}

// renderConfigStatusJSON marshals the core Result under the CLI version envelope. It never
// emits a severity field.
func renderConfigStatusJSON(result configstatus.Result) (string, error) {
	out := configStatusJSONOutput{
		Version:            Version,
		RuleID:             result.RuleID,
		Status:             result.Status,
		Default:            result.Default,
		Current:            result.Current,
		ConfigEffect:       result.ConfigEffect,
		Suppression:        result.Suppression,
		RuleDetailsCommand: result.RuleDetailsCommand,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal config status: %w", err)
	}
	return string(data) + "\n", nil
}

// renderConfigStatusText renders a stable, human-readable status block. Config effect messages
// are surfaced verbatim from the core so partial-replacement dangers stay faithful to what the
// audit path actually applies.
func renderConfigStatusText(result configstatus.Result) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Rule: %s\n", result.RuleID)
	b.WriteByte('\n')

	b.WriteString("Current status:\n")
	fmt.Fprintf(&b, "  %s\n", strings.ToUpper(result.Status.State))
	if result.Suppression != nil {
		fmt.Fprintf(&b, "  Not Loaded: %s (suppressed by %s).\n", result.Suppression.Reason, result.Suppression.By)
		b.WriteString("  This rule will not produce findings while that suppression applies.\n")
	} else if result.Status.State == "on" {
		fmt.Fprintf(&b, "  Findings from this rule fail as: %s.\n", string(result.Status.Level))
	} else {
		b.WriteString("  This rule will not produce findings.\n")
	}
	b.WriteByte('\n')

	b.WriteString("Config effect:\n")
	for _, message := range result.ConfigEffect.Messages {
		fmt.Fprintf(&b, "  %s\n", message)
	}
	// When the rule ends up OFF because the config mentioned it (the partial-replacement danger
	// case, or an explicit disable), restate the outcome so the effective state is unmissable.
	if result.Status.State == "off" && result.ConfigEffect.HasOverride {
		b.WriteString("  This rule is OFF.\n")
	}
	b.WriteByte('\n')

	b.WriteString("Default:\n")
	writeRulePolicySnapshot(&b, result.Default)
	b.WriteByte('\n')

	b.WriteString("Current:\n")
	writeRulePolicySnapshot(&b, result.Current)
	b.WriteByte('\n')

	b.WriteString("Rule details:\n")
	fmt.Fprintf(&b, "  %s\n", result.RuleDetailsCommand)

	return b.String()
}

// writeRulePolicySnapshot renders one policy snapshot with params keys sorted alphabetically.
func writeRulePolicySnapshot(b *strings.Builder, snapshot configstatus.RulePolicySnapshot) {
	fmt.Fprintf(b, "  enabled: %t\n", snapshot.Enabled)
	fmt.Fprintf(b, "  level: %s\n", string(snapshot.Level))
	b.WriteString("  params:\n")
	if len(snapshot.Params) == 0 {
		b.WriteString("    (none)\n")
		return
	}
	keys := make([]string, 0, len(snapshot.Params))
	for key := range snapshot.Params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(b, "    %s: %v\n", key, snapshot.Params[key])
	}
}
