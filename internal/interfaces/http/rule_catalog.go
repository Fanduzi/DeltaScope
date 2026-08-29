// Package httpapi exposes the HTTP adapter for DeltaScope.
// input: HTTP rule-discovery requests plus shipped catalog and capability metadata
// output: JSON rule-list, rule-detail, and capability payloads including offline existence context_fields and stable online identity errors for the HTTP adapter
// pos: HTTP discovery helpers above the domain rule catalog and shared JSON writers
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"fmt"
	"net/http"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	rulecatalog "github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
)

type httpRuleDescription struct {
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

type httpListRulesResponse struct {
	Query string                `json:"query,omitempty"`
	Count int                   `json:"count"`
	Rules []httpRuleDescription `json:"rules"`
}

type httpCapabilitiesResponse struct {
	Transport         string   `json:"transport"`
	Endpoints         []string `json:"endpoints"`
	AuditModes        []string `json:"audit_modes"`
	Dialects          []string `json:"dialects"`
	TopLevelInputs    []string `json:"top_level_inputs"`
	InputRules        []string `json:"input_rules"`
	ResultFields      []string `json:"result_fields"`
	ContextFields     []string `json:"context_fields"`
	StructuredErrors  []string `json:"structured_errors"`
	MetadataFeatures  []string `json:"metadata_features"`
	QueryParameters   []string `json:"query_parameters"`
	RuleCatalogRoutes []string `json:"rule_catalog_routes"`
	CapabilityVersion string   `json:"capability_version"`
}

func handleListRules(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, listHTTPRulesPayload(r.URL.Query().Get("query")))
}

func handleDescribeRule(w http.ResponseWriter, ruleID string) {
	payload, err := describeHTTPRulePayload(ruleID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func handleCapabilities(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, httpCapabilitiesPayload())
}

func listHTTPRulesPayload(query string) httpListRulesResponse {
	entries := rulecatalog.Search(query)
	rules := make([]httpRuleDescription, 0, len(entries))
	for _, entry := range entries {
		rules = append(rules, toHTTPRuleDescription(entry))
	}
	return httpListRulesResponse{
		Query: query,
		Count: len(rules),
		Rules: rules,
	}
}

func describeHTTPRulePayload(ruleID string) (httpRuleDescription, error) {
	entry, ok := rulecatalog.Lookup(ruleID)
	if !ok {
		return httpRuleDescription{}, fmt.Errorf("rule %q not found", ruleID)
	}
	return toHTTPRuleDescription(entry), nil
}

func httpCapabilitiesPayload() httpCapabilitiesResponse {
	return httpCapabilitiesResponse{
		Transport: "http",
		Endpoints: []string{
			"GET /healthz",
			"GET /readyz",
			"GET /version",
			"GET /metrics",
			"POST /v1/audit",
			"POST /v1/query-access/analyze",
			"GET /v1/rules",
			"GET /v1/rules/{rule_id}",
			"GET /v1/capabilities",
		},
		AuditModes: []string{"offline", "metadata-aware"},
		Dialects:   []string{"mysql", "tidb", "postgresql"},
		TopLevelInputs: []string{
			"sql",
			"dialect",
			"schema",
			"connection_id",
		},
		InputRules:        []string{"connection_id references a named connection in the server runtime config", "top-level schema overrides the named connection schema when both are set", "top-level dialect overrides the named connection dialect when both are set", "connection_id supports mysql, tidb, and postgresql metadata-aware audit"},
		ResultFields:      []string{"verdict", "summary", "statements", "global_findings", "explanation", "context"},
		ContextFields:     []string{"mode", "dialect", "dialect_source", "schema", "schema_source", "metadata_source", "note", "unproven"},
		StructuredErrors:  []string{"invalid_json", "bad_request", "connection_invalid", "connection_failed", "authentication_failed", "identity_error", "config_invalid", "auth_required", "auth_invalid", "rate_limited", "request_timeout", "request_canceled", "internal_error", "not_found"},
		MetadataFeatures:  []string{"schema context", "instance facts", "target table snapshots"},
		QueryParameters:   []string{"GET /v1/rules?query=<text>"},
		RuleCatalogRoutes: []string{"GET /v1/rules", "GET /v1/rules/{rule_id}"},
		CapabilityVersion: "http-v1",
	}
}

func toHTTPRuleDescription(entry rulecatalog.Entry) httpRuleDescription {
	return httpRuleDescription{
		RuleID:          entry.RuleID,
		Summary:         entry.Summary,
		Description:     entry.Description,
		StatementKinds:  append([]string(nil), entry.StatementKinds...),
		DefaultEnabled:  entry.DefaultEnabled,
		DefaultLevel:    entry.DefaultLevel,
		DefaultParams:   cloneHTTPAnyMap(entry.DefaultParams),
		MetadataAware:   entry.MetadataAware,
		TriggerExample:  entry.TriggerExample,
		ValidExample:    entry.ValidExample,
		ConfigExample:   entry.ConfigExample,
		RemediationHint: entry.RemediationHint,
		Why:             entry.Why,
		Risk:            entry.Risk,
		Suggestion:      entry.Suggestion,
		ConfigHints:     append([]string(nil), entry.ConfigHints...),
		MetadataNotes:   cloneHTTPMetadataNotes(entry.MetadataNotes),
	}
}

func cloneHTTPAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneHTTPMetadataNotes(in *rulecatalog.MetadataNotes) *rulecatalog.MetadataNotes {
	if in == nil {
		return nil
	}
	return &rulecatalog.MetadataNotes{
		Kinds:    append([]rulecatalog.MetadataKind(nil), in.Kinds...),
		Required: in.Required,
		Missing:  in.Missing,
	}
}
