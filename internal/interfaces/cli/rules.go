// Package cli exposes the command-line adapter for DeltaScope.
// input: rule catalog queries, CLI filters, and stdout/stderr command surfaces
// output: shipped-rule list, detail, and search rendering for CLI discovery workflows
// pos: CLI rule catalog command group above the explanation-oriented domain catalog
// note: if this file changes, update this header and module README.md.
package cli

import (
	"fmt"
	"sort"
	"strings"

	rulecatalog "github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
	"github.com/spf13/cobra"
)

func newRulesCmd(exitCode *int) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Inspect shipped DeltaScope rules",
	}
	cmd.AddCommand(newRulesListCmd(exitCode))
	cmd.AddCommand(newRulesShowCmd(exitCode))
	cmd.AddCommand(newRulesSearchCmd(exitCode))
	return cmd
}

func newRulesListCmd(exitCode *int) *cobra.Command {
	var level string
	var kind string
	var enabledOnly bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List shipped rules",
		RunE: func(cmd *cobra.Command, args []string) error {
			items := filterCatalogEntries(rulecatalog.All(), kind, level, enabledOnly)
			if _, err := fmt.Fprint(cmd.OutOrStdout(), renderCatalogList(items)); err != nil {
				*exitCode = exitInternal
				return err
			}
			*exitCode = exitOK
			return nil
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if kind != "" && kind != "ddl" && kind != "dml" {
				*exitCode = exitUser
				return newUserError("rules list --kind must be ddl or dml")
			}
			if level != "" && level != "blocker" && level != "warning" && level != "notice" {
				*exitCode = exitUser
				return newUserError("rules list --level must be blocker, warning, or notice")
			}
			return nil
		},
		Args: cobra.NoArgs,
	}
	cmd.Flags().StringVar(&kind, "kind", "", "filter to ddl or dml rules")
	cmd.Flags().StringVar(&level, "level", "", "filter to blocker, warning, or notice")
	cmd.Flags().BoolVar(&enabledOnly, "enabled-only", false, "show only rules enabled in the shipped default policy")
	return cmd
}

func newRulesShowCmd(exitCode *int) *cobra.Command {
	return &cobra.Command{
		Use:   "show <rule-id>",
		Short: "Show one shipped rule in detail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entry, ok := rulecatalog.Lookup(args[0])
			if !ok {
				*exitCode = exitUser
				return newUserError(fmt.Sprintf("unknown rule %q", args[0]))
			}
			if _, err := fmt.Fprint(cmd.OutOrStdout(), renderCatalogEntry(entry)); err != nil {
				*exitCode = exitInternal
				return err
			}
			*exitCode = exitOK
			return nil
		},
	}
}

func newRulesSearchCmd(exitCode *int) *cobra.Command {
	return &cobra.Command{
		Use:   "search <keyword>",
		Short: "Search shipped rules by ID or summary",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			items := rulecatalog.Search(args[0])
			if _, err := fmt.Fprint(cmd.OutOrStdout(), renderCatalogList(items)); err != nil {
				*exitCode = exitInternal
				return err
			}
			*exitCode = exitOK
			return nil
		},
	}
}

func filterCatalogEntries(entries []rulecatalog.Entry, kind string, level string, enabledOnly bool) []rulecatalog.Entry {
	filtered := make([]rulecatalog.Entry, 0, len(entries))
	for _, entry := range entries {
		if kind != "" && !contains(entry.StatementKinds, kind) {
			continue
		}
		if level != "" && string(entry.DefaultLevel) != level {
			continue
		}
		if enabledOnly && !entry.DefaultEnabled {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func renderCatalogList(entries []rulecatalog.Entry) string {
	var b strings.Builder
	b.WriteString("# DeltaScope Rules\n\n")
	if len(entries) == 0 {
		b.WriteString("No rules matched.\n")
		return b.String()
	}
	for _, entry := range entries {
		fmt.Fprintf(&b, "- `%s` [%s] (%s) %s\n", entry.RuleID, entry.DefaultLevel, strings.Join(entry.StatementKinds, ","), entry.Summary)
	}
	return b.String()
}

func renderCatalogEntry(entry rulecatalog.Entry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", entry.RuleID)
	fmt.Fprintf(&b, "%s\n\n", entry.Description)
	fmt.Fprintf(&b, "- Default Enabled: `%t`\n", entry.DefaultEnabled)
	fmt.Fprintf(&b, "- Default Level: `%s`\n", entry.DefaultLevel)
	fmt.Fprintf(&b, "- Statement Kinds: `%s`\n", strings.Join(entry.StatementKinds, ", "))
	fmt.Fprintf(&b, "- Metadata Aware: `%t`\n\n", entry.MetadataAware)

	b.WriteString("## Default Params\n")
	if len(entry.DefaultParams) == 0 {
		b.WriteString("`{}`\n\n")
	} else {
		keys := make([]string, 0, len(entry.DefaultParams))
		for key := range entry.DefaultParams {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&b, "- `%s`: `%v`\n", key, entry.DefaultParams[key])
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "## Trigger Example\n```sql\n%s\n```\n\n", entry.TriggerExample)
	fmt.Fprintf(&b, "## Valid Example\n```sql\n%s\n```\n\n", entry.ValidExample)
	fmt.Fprintf(&b, "## Config Example\n```yaml\n%s\n```\n\n", entry.ConfigExample)
	fmt.Fprintf(&b, "## Remediation\n%s\n", entry.RemediationHint)
	return b.String()
}

func contains(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}
