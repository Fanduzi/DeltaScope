// Package mcpapi exposes the MCP adapter for DeltaScope.
// input: shipped rule catalog entries and DeltaScope capability metadata for MCP rule tools
// output: full describe_rule bodies, compact list_rules rows plus text catalog, and get_capabilities summaries including query_access unavailability, the public connection.connect_timeout capability, and offline existence context_fields
// pos: MCP rule-discovery helpers above the domain rule catalog
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	rulecatalog "github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
)

// ruleDescription captures the full catalog metadata returned by describe_rule.
type ruleDescription struct {
	RuleID          string                     `json:"rule_id"`
	Summary         string                     `json:"summary"`
	Description     string                     `json:"description"`
	StatementKinds  []string                   `json:"statement_kinds"`
	DefaultEnabled  bool                       `json:"default_enabled"`
	DefaultLevel    rule.Level                 `json:"default_level"`
	DefaultParams   map[string]any             `json:"default_params"`
	MetadataAware   bool                       `json:"metadata_aware"`
	TriggerExample  string                     `json:"trigger_example,omitempty"`
	ValidExample    string                     `json:"valid_example,omitempty"`
	ConfigExample   string                     `json:"config_example,omitempty"`
	RemediationHint string                     `json:"remediation_hint,omitempty"`
	Why             string                     `json:"why,omitempty"`
	Risk            string                     `json:"risk,omitempty"`
	Suggestion      string                     `json:"suggestion,omitempty"`
	ConfigHints     []string                   `json:"config_hints,omitempty"`
	MetadataNotes   *rulecatalog.MetadataNotes `json:"metadata_notes,omitempty"`
}

// ruleListItem is the compact catalog row returned by list_rules.
type ruleListItem struct {
	RuleID  string `json:"rule_id"`
	Level   string `json:"level"`
	Dialect string `json:"dialect"`
	Kind    string `json:"kind"`
	Summary string `json:"summary"`
}

type listRulesResponse struct {
	Query string         `json:"query,omitempty"`
	Count int            `json:"count"`
	Rules []ruleListItem `json:"rules"`
}

// queryAccessCapability is the get_capabilities Query Access discovery object.
// Surfaces order is stable: cli, then http.
type queryAccessCapability struct {
	Available bool     `json:"available"`
	Surfaces  []string `json:"surfaces"`
}

type capabilitiesResponse struct {
	Transport                 string                `json:"transport"`
	Tools                     []string              `json:"tools"`
	QueryAccess               queryAccessCapability `json:"query_access"`
	AuditModes                []string              `json:"audit_modes"`
	Dialects                  []string              `json:"dialects"`
	TopLevelInputs            []string              `json:"top_level_inputs"`
	ConnectionInputs          []string              `json:"connection_inputs"`
	InputRules                []string              `json:"input_rules"`
	ConnectionRefPath         string                `json:"connection_ref_path"`
	ConnectionRefOverrideFlag string                `json:"connection_ref_override_flag"`
	ResultFields              []string              `json:"result_fields"`
	ContextFields             []string              `json:"context_fields"`
	StructuredErrors          []string              `json:"structured_errors"`
	MetadataFeatures          []string              `json:"metadata_features"`
	RuleCatalogTools          []string              `json:"rule_catalog_tools"`
	CapabilityVersion         string                `json:"capability_version"`
}

func describeRulePayload(ruleID string) (ruleDescription, error) {
	entry, ok := rulecatalog.Lookup(ruleID)
	if !ok {
		return ruleDescription{}, fmt.Errorf("rule %q not found", ruleID)
	}
	return toRuleDescription(entry), nil
}

func listRulesPayload(query string) listRulesResponse {
	entries := rulecatalog.Search(query)
	rules := make([]ruleListItem, 0, len(entries))
	for _, entry := range entries {
		rules = append(rules, toRuleListItem(entry))
	}
	return listRulesResponse{
		Query: query,
		Count: len(rules),
		Rules: rules,
	}
}

func toRuleListItem(entry rulecatalog.Entry) ruleListItem {
	return ruleListItem{
		RuleID:  entry.RuleID,
		Level:   string(entry.DefaultLevel),
		Dialect: strings.Join(entry.Dialects, ","),
		Kind:    strings.Join(entry.StatementKinds, ","),
		Summary: entry.Summary,
	}
}

func renderListRulesText(resp listRulesResponse) string {
	var b strings.Builder
	if resp.Query != "" {
		fmt.Fprintf(&b, "%d rules matching %q\n", resp.Count, resp.Query)
	} else {
		fmt.Fprintf(&b, "%d rules\n", resp.Count)
	}
	if len(resp.Rules) == 0 {
		return b.String()
	}

	headers := []string{"RULE ID", "LEVEL", "DIALECT", "KIND", "SUMMARY"}
	rows := make([][]string, len(resp.Rules))
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(header)
	}
	for i, item := range resp.Rules {
		row := []string{item.RuleID, item.Level, item.Dialect, item.Kind, item.Summary}
		rows[i] = row
		for j, value := range row {
			if len(value) > widths[j] {
				widths[j] = len(value)
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

	b.WriteByte('\n')
	writeRow(headers)
	for i, width := range widths {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(strings.Repeat("-", width))
	}
	b.WriteByte('\n')
	for _, row := range rows {
		writeRow(row)
	}
	return b.String()
}

func capabilitiesPayload(connectionsPath string) capabilitiesResponse {
	if connectionsPath == "" {
		connectionsPath = DefaultConnectionsPath
	}
	return capabilitiesResponse{
		Transport: "stdio",
		Tools: []string{
			"audit_sql",
			"describe_rule",
			"list_rules",
			"get_capabilities",
		},
		QueryAccess: queryAccessCapability{
			Available: false,
			Surfaces:  []string{"cli", "http"},
		},
		AuditModes: []string{"offline", "metadata-aware"},
		Dialects:   []string{"mysql", "tidb", "postgresql"},
		TopLevelInputs: []string{
			"sql",
			"dialect",
			"config_path",
			"connection_ref",
			"connection",
		},
		ConnectionInputs: []string{
			"connection.host",
			"connection.port",
			"connection.socket",
			"connection.user",
			"connection.database",
			"connection.schema",
			"connection.dialect",
			"connection.password",
			"connection.password_env",
			"connection.password_file",
			"connection.connect_timeout",
		},
		InputRules:                []string{"connection_ref and connection are mutually exclusive", "top-level dialect overrides connection.dialect when both are set", "connection inputs support mysql, tidb, and postgresql metadata-aware audit"},
		ConnectionRefPath:         connectionsPath,
		ConnectionRefOverrideFlag: "-connections-path",
		ResultFields:              []string{"verdict", "summary", "statements", "global_findings", "explanation", "context"},
		ContextFields:             []string{"mode", "dialect", "dialect_source", "schema", "schema_source", "metadata_source", "note", "unproven"},
		StructuredErrors:          []string{"bad_request", "connection_invalid", "connection_failed", "config_invalid"},
		MetadataFeatures:          []string{"schema context", "instance facts", "target table snapshots"},
		RuleCatalogTools:          []string{"describe_rule", "list_rules"},
		CapabilityVersion:         "mcp-v1",
	}
}

func toRuleDescription(entry rulecatalog.Entry) ruleDescription {
	return ruleDescription{
		RuleID:          entry.RuleID,
		Summary:         entry.Summary,
		Description:     entry.Description,
		StatementKinds:  append([]string(nil), entry.StatementKinds...),
		DefaultEnabled:  entry.DefaultEnabled,
		DefaultLevel:    entry.DefaultLevel,
		DefaultParams:   cloneAnyMap(entry.DefaultParams),
		MetadataAware:   entry.MetadataAware,
		TriggerExample:  entry.TriggerExample,
		ValidExample:    entry.ValidExample,
		ConfigExample:   entry.ConfigExample,
		RemediationHint: entry.RemediationHint,
		Why:             entry.Why,
		Risk:            entry.Risk,
		Suggestion:      entry.Suggestion,
		ConfigHints:     append([]string(nil), entry.ConfigHints...),
		MetadataNotes:   cloneMetadataNotes(entry.MetadataNotes),
	}
}

func cloneAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneMetadataNotes(in *rulecatalog.MetadataNotes) *rulecatalog.MetadataNotes {
	if in == nil {
		return nil
	}
	return &rulecatalog.MetadataNotes{
		Kinds:    append([]rulecatalog.MetadataKind(nil), in.Kinds...),
		Required: in.Required,
		Missing:  in.Missing,
	}
}
