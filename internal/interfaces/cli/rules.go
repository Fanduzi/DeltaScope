// Package cli exposes the command-line adapter for DeltaScope.
// input: rule catalog queries, CLI filters, and stdout/stderr command surfaces
// output: shipped-rule list, detail, and search rendering for CLI discovery workflows
// pos: CLI rule catalog command group above the explanation-oriented domain catalog
// note: if this file changes, update this header and module README.md.
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	rulecatalog "github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
	"github.com/spf13/cobra"
)

func newRulesCmd(exitCode *int) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Inspect shipped DeltaScope rules",
	}
	cmd.AddCommand(newRulesListCmd(exitCode))
	cmd.AddCommand(newRulesExplainCmd(exitCode))
	return cmd
}

// ---------- rules list ----------

type rulesListJSONOutput struct {
	Version string               `json:"version"`
	Summary rulesListJSONSummary `json:"summary"`
	Rules   []rulesListJSONRule  `json:"rules"`
}

type rulesListJSONSummary struct {
	Total    int               `json:"total"`
	Returned int               `json:"returned"`
	Filters  map[string]string `json:"filters,omitempty"`
}

type rulesListJSONRule struct {
	RuleID    string   `json:"rule_id"`
	Level     string   `json:"level"`
	Dialect   string   `json:"dialect"`
	Kind      string   `json:"kind"`
	Category  string   `json:"category"`
	Summary   string   `json:"summary"`
	Enabled   bool     `json:"enabled"`
	Tags      []string `json:"tags,omitempty"`
	ConfigKey string   `json:"config_key"`
}

func newRulesListCmd(exitCode *int) *cobra.Command {
	var dialect, level, kind, category, search, format string
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List shipped rules",
		Long:  "List rules from the shipped catalog with optional filters.\nDoes not execute audits, parse SQL, or call the audit service.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateRulesListFlags(format, limit); err != nil {
				*exitCode = exitUser
				return err
			}

			q := rulecatalog.Query{
				Dialect:  dialect,
				Level:    level,
				Kind:     kind,
				Category: category,
				Search:   search,
				Limit:    limit,
			}
			if err := q.Validate(); err != nil {
				*exitCode = exitUser
				return err
			}

			result, err := rulecatalog.QueryEntries(rulecatalog.All(), q)
			if err != nil {
				*exitCode = exitInternal
				return err
			}

			var output string
			if format == "json" {
				output = renderRulesListJSON(result, q)
			} else {
				output = renderRulesListText(result)
			}

			if _, err := fmt.Fprint(cmd.OutOrStdout(), output); err != nil {
				*exitCode = exitInternal
				return err
			}
			*exitCode = exitOK
			return nil
		},
	}

	cmd.Flags().StringVar(&dialect, "dialect", "", "filter by dialect: mysql, tidb, postgresql, common")
	cmd.Flags().StringVar(&level, "level", "", "filter by level: blocker, warning, notice")
	cmd.Flags().StringVar(&kind, "kind", "", "filter by kind: ddl, dml")
	cmd.Flags().StringVar(&category, "category", "", "filter by category (case-insensitive substring)")
	cmd.Flags().StringVar(&search, "search", "", "search across rule fields (case-insensitive substring)")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	cmd.Flags().IntVar(&limit, "limit", 0, "limit result count; 0 means no limit")

	return cmd
}

func validateRulesListFlags(format string, limit int) error {
	if format != "text" && format != "json" {
		return newUserError(fmt.Sprintf("invalid format %q: must be text or json", format))
	}
	if limit < 0 {
		return newUserError(fmt.Sprintf("invalid limit %d: must be 0 or positive", limit))
	}
	return nil
}

func renderRulesListJSON(result rulecatalog.Result, q rulecatalog.Query) string {
	filters := make(map[string]string)
	if q.Dialect != "" {
		filters["dialect"] = q.Dialect
	}
	if q.Level != "" {
		filters["level"] = q.Level
	}
	if q.Kind != "" {
		filters["kind"] = q.Kind
	}
	if q.Category != "" {
		filters["category"] = q.Category
	}
	if q.Search != "" {
		filters["search"] = q.Search
	}
	if q.Limit > 0 {
		filters["limit"] = fmt.Sprintf("%d", q.Limit)
	}

	rules := make([]rulesListJSONRule, 0, len(result.Entries))
	for _, e := range result.Entries {
		rules = append(rules, rulesListJSONRule{
			RuleID:    e.RuleID,
			Level:     string(e.DefaultLevel),
			Dialect:   strings.Join(e.Dialects, ","),
			Kind:      strings.Join(e.StatementKinds, ","),
			Category:  e.Category,
			Summary:   e.Summary,
			Enabled:   e.DefaultEnabled,
			Tags:      e.Tags,
			ConfigKey: e.ConfigKey,
		})
	}

	out := rulesListJSONOutput{
		Version: Version,
		Summary: rulesListJSONSummary{
			Total:    result.Total,
			Returned: len(rules),
			Filters:  filters,
		},
		Rules: rules,
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "marshal failed: %v"}`, err)
	}
	return string(data) + "\n"
}

func renderRulesListText(result rulecatalog.Result) string {
	var b strings.Builder

	headers := []string{"RULE ID", "LEVEL", "DIALECT", "KIND", "CATEGORY"}
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}

	rows := make([][]string, len(result.Entries))
	for i, e := range result.Entries {
		row := []string{
			e.RuleID,
			string(e.DefaultLevel),
			strings.Join(e.Dialects, ","),
			strings.Join(e.StatementKinds, ","),
			e.Category,
		}
		rows[i] = row
		for j, v := range row {
			if len(v) > widths[j] {
				widths[j] = len(v)
			}
		}
	}

	writeRow := func(cols []string) {
		for i, col := range cols {
			if i > 0 {
				b.WriteString("  ")
			}
			fmt.Fprintf(&b, "%-*s", widths[i], col)
		}
		b.WriteByte('\n')
	}

	writeSep := func() {
		for i, w := range widths {
			if i > 0 {
				b.WriteString("  ")
			}
			b.WriteString(strings.Repeat("-", w))
		}
		b.WriteByte('\n')
	}

	writeRow(headers)
	writeSep()
	for _, row := range rows {
		writeRow(row)
	}

	if len(result.Entries) == 0 {
		b.WriteString("\nNo rules matched.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "\n%d rules", len(result.Entries))
	if result.Total > len(result.Entries) {
		fmt.Fprintf(&b, " (%d total before limit)", result.Total)
	}
	b.WriteByte('\n')

	return b.String()
}

// ---------- rules explain ----------

type rulesExplainJSONOutput struct {
	Version string               `json:"version"`
	Rule    rulesExplainJSONRule `json:"rule"`
}

type rulesExplainJSONRule struct {
	RuleID         string                     `json:"rule_id"`
	Level          string                     `json:"level"`
	Enabled        bool                       `json:"enabled"`
	Dialects       []string                   `json:"dialects"`
	Kind           string                     `json:"kind"`
	Category       string                     `json:"category"`
	Summary        string                     `json:"summary"`
	Why            string                     `json:"why"`
	Risk           string                     `json:"risk"`
	Suggestion     string                     `json:"suggestion"`
	ConfigKey      string                     `json:"config_key"`
	Tags           []string                   `json:"tags,omitempty"`
	Description    string                     `json:"description"`
	StatementKinds []string                   `json:"statement_kinds"`
	DefaultParams  map[string]any             `json:"default_params"`
	MetadataAware  bool                       `json:"metadata_aware"`
	TriggerExample string                     `json:"trigger_example"`
	ValidExample   string                     `json:"valid_example"`
	ConfigExample  string                     `json:"config_example"`
	Remediation    string                     `json:"remediation"`
	ConfigHints    []string                   `json:"config_hints"`
	MetadataNotes  *rulesExplainJSONMetaNotes `json:"metadata_notes,omitempty"`
}

type rulesExplainJSONMetaNotes struct {
	Kinds    []string `json:"kinds"`
	Required string   `json:"required"`
	Missing  string   `json:"missing"`
}

func newRulesExplainCmd(exitCode *int) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "explain <rule-id>",
		Short: "Explain one shipped rule in detail",
		Long:  "Show detailed information about a single rule.\nDoes not execute audits, parse SQL, or call the audit service.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "text" && format != "json" {
				*exitCode = exitUser
				return newUserError(fmt.Sprintf("invalid format %q: must be text or json", format))
			}

			entry, ok := rulecatalog.Lookup(args[0])
			if !ok {
				*exitCode = exitUser
				return newUserError(fmt.Sprintf("rule %q not found", args[0]))
			}

			var output string
			if format == "json" {
				output = renderRulesExplainJSON(entry)
			} else {
				output = renderRulesExplainText(entry)
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

func renderRulesExplainJSON(entry rulecatalog.Entry) string {
	rule := rulesExplainJSONRule{
		RuleID:         entry.RuleID,
		Level:          string(entry.DefaultLevel),
		Enabled:        entry.DefaultEnabled,
		Dialects:       entry.Dialects,
		Kind:           strings.Join(entry.StatementKinds, ","),
		Category:       entry.Category,
		Summary:        entry.Summary,
		Why:            entry.Why,
		Risk:           entry.Risk,
		Suggestion:     entry.Suggestion,
		ConfigKey:      entry.ConfigKey,
		Tags:           entry.Tags,
		Description:    entry.Description,
		StatementKinds: entry.StatementKinds,
		DefaultParams:  entry.DefaultParams,
		MetadataAware:  entry.MetadataAware,
		TriggerExample: entry.TriggerExample,
		ValidExample:   entry.ValidExample,
		ConfigExample:  entry.ConfigExample,
		Remediation:    entry.RemediationHint,
		ConfigHints:    entry.ConfigHints,
	}
	if entry.MetadataNotes != nil {
		kinds := make([]string, 0, len(entry.MetadataNotes.Kinds))
		for _, k := range entry.MetadataNotes.Kinds {
			kinds = append(kinds, string(k))
		}
		rule.MetadataNotes = &rulesExplainJSONMetaNotes{
			Kinds:    kinds,
			Required: entry.MetadataNotes.Required,
			Missing:  entry.MetadataNotes.Missing,
		}
	}

	out := rulesExplainJSONOutput{
		Version: Version,
		Rule:    rule,
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "marshal failed: %v"}`, err)
	}
	return string(data) + "\n"
}

func renderRulesExplainText(entry rulecatalog.Entry) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Rule ID:    %s\n", entry.RuleID)
	fmt.Fprintf(&b, "Level:      %s\n", string(entry.DefaultLevel))
	fmt.Fprintf(&b, "Enabled:    %t\n", entry.DefaultEnabled)
	fmt.Fprintf(&b, "Dialects:   %s\n", strings.Join(entry.Dialects, ", "))
	fmt.Fprintf(&b, "Kind:       %s\n", strings.Join(entry.StatementKinds, ", "))
	fmt.Fprintf(&b, "Category:   %s\n", entry.Category)
	fmt.Fprintf(&b, "Config Key: %s\n", entry.ConfigKey)
	b.WriteByte('\n')

	fmt.Fprintf(&b, "Summary:\n  %s\n\n", entry.Summary)
	fmt.Fprintf(&b, "Why:\n  %s\n\n", entry.Why)
	fmt.Fprintf(&b, "Risk:\n  %s\n\n", entry.Risk)
	fmt.Fprintf(&b, "Suggestion:\n  %s\n\n", entry.Suggestion)

	if len(entry.Tags) > 0 {
		fmt.Fprintf(&b, "Tags: %s\n", strings.Join(entry.Tags, ", "))
	}

	fmt.Fprintf(&b, "Trigger Example:\n  %s\n", entry.TriggerExample)
	fmt.Fprintf(&b, "Valid Example:\n  %s\n", entry.ValidExample)

	b.WriteByte('\n')
	b.WriteString("Default Params:\n")
	if len(entry.DefaultParams) == 0 {
		b.WriteString("  (none)\n")
	} else {
		keys := make([]string, 0, len(entry.DefaultParams))
		for key := range entry.DefaultParams {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&b, "  %s: %v\n", key, entry.DefaultParams[key])
		}
	}

	// Default policy block: the authoritative baseline comes from policy.Default(),
	// not duplicated catalog values, so it tracks the engine's real defaults.
	rp := resolveDefaultRulePolicy(entry)
	b.WriteString("\nDefault policy:\n")
	b.WriteString(renderRulePolicyYAML(entry.RuleID, rp))

	// Safe override example: a complete rule policy override (enabled + level +
	// params) so omitting a field can never silently turn the rule OFF. The level
	// is downgraded to warning when the default is not already warning; otherwise
	// the default level is kept. Either way the full policy shape is shown.
	override := rp
	if override.Level != rule.LevelWarning {
		override.Level = rule.LevelWarning
	}
	b.WriteString("\nSafe override example:\n")
	b.WriteString(renderRulePolicyYAML(entry.RuleID, override))

	// Handoff to config status for the effective result under a real config file.
	fmt.Fprintf(&b, "\nInspect effective rule status:\n  deltascope config status %s --config deltascope.yaml\n", entry.RuleID)

	return b.String()
}

// resolveDefaultRulePolicy returns the shipped default policy for one rule from
// policy.Default(). Catalog-only opt-in rules are absent from Default Policy, so
// the fallback uses the catalog entry's default-disabled values.
func resolveDefaultRulePolicy(entry rulecatalog.Entry) policy.RulePolicy {
	if rp, ok := policy.Default().Rules[entry.RuleID]; ok {
		return rp
	}
	return policy.RulePolicy{
		Enabled: entry.DefaultEnabled,
		Level:   entry.DefaultLevel,
		Params:  entry.DefaultParams,
	}
}

// renderRulePolicyYAML renders a complete rule policy as an indented YAML
// snippet (rules.<id>.{enabled,level,params}). It mirrors the catalog's config
// example rendering so explain output stays consistent with config lint/status.
func renderRulePolicyYAML(ruleID string, rp policy.RulePolicy) string {
	var b strings.Builder
	b.WriteString("  rules:\n")
	fmt.Fprintf(&b, "    %s:\n", ruleID)
	fmt.Fprintf(&b, "      enabled: %t\n", rp.Enabled)
	fmt.Fprintf(&b, "      level: %s\n", string(rp.Level))
	if len(rp.Params) == 0 {
		b.WriteString("      params: {}\n")
		return b.String()
	}
	b.WriteString("      params:\n")
	keys := make([]string, 0, len(rp.Params))
	for key := range rp.Params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(&b, "        %s: %s\n", key, formatYAMLScalar(rp.Params[key]))
	}
	return b.String()
}

// formatYAMLScalar renders one policy param value as a YAML scalar. It is a
// local mirror of the catalog helper; per the v0.360.0 decision record a shared
// helper is deferred until duplication is shown.
func formatYAMLScalar(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []string:
		quoted := make([]string, 0, len(typed))
		for _, item := range typed {
			quoted = append(quoted, fmt.Sprintf("%q", item))
		}
		return fmt.Sprintf("[%s]", strings.Join(quoted, ", "))
	default:
		return fmt.Sprintf("%v", typed)
	}
}
