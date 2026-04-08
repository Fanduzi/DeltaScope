// Package mcpapi exposes the MCP adapter for DeltaScope.
// input: shipped rule catalog entries and DeltaScope capability metadata for MCP rule tools
// output: structured payload builders for describe_rule, list_rules, and get_capabilities
// pos: MCP rule-discovery helpers above the domain rule catalog
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	rulecatalog "github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
)

// ruleDescription captures the catalog metadata returned by describe/list rules.
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

type listRulesResponse struct {
	Query string            `json:"query,omitempty"`
	Count int               `json:"count"`
	Rules []ruleDescription `json:"rules"`
}

type capabilitiesResponse struct {
	Transport                 string   `json:"transport"`
	Tools                     []string `json:"tools"`
	AuditModes                []string `json:"audit_modes"`
	Dialects                  []string `json:"dialects"`
	TopLevelInputs            []string `json:"top_level_inputs"`
	ConnectionInputs          []string `json:"connection_inputs"`
	InputRules                []string `json:"input_rules"`
	ConnectionRefPath         string   `json:"connection_ref_path"`
	ConnectionRefOverrideFlag string   `json:"connection_ref_override_flag"`
	ResultFields              []string `json:"result_fields"`
	ContextFields             []string `json:"context_fields"`
	StructuredErrors          []string `json:"structured_errors"`
	MetadataFeatures          []string `json:"metadata_features"`
	RuleCatalogTools          []string `json:"rule_catalog_tools"`
	CapabilityVersion         string   `json:"capability_version"`
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
	rules := make([]ruleDescription, 0, len(entries))
	for _, entry := range entries {
		rules = append(rules, toRuleDescription(entry))
	}
	return listRulesResponse{
		Query: query,
		Count: len(rules),
		Rules: rules,
	}
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
			"connection.schema",
			"connection.dialect",
			"connection.password",
			"connection.password_env",
			"connection.password_file",
		},
		InputRules:                []string{"connection_ref and connection are mutually exclusive", "top-level dialect overrides connection.dialect when both are set", "connection inputs support mysql, tidb, and postgresql metadata-aware audit"},
		ConnectionRefPath:         connectionsPath,
		ConnectionRefOverrideFlag: "-connections-path",
		ResultFields:              []string{"verdict", "summary", "statements", "global_findings", "explanation", "context"},
		ContextFields:             []string{"mode", "dialect", "dialect_source", "schema", "schema_source", "metadata_source"},
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
