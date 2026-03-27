// Package mcpapi verifies MCP rule-discovery tool behavior.
// input: MCP rule tool handlers plus shipped rule catalog expectations
// output: focused coverage for describe_rule, list_rules, and get_capabilities responses
// pos: interface-layer tests for MCP rule-discovery behavior
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestDescribeRuleToolReturnsCatalogEntry(t *testing.T) {
	t.Parallel()

	_, got, err := describeRuleTool(context.Background(), nil, describeRuleInput{RuleID: "dml.where.require"})
	if err != nil {
		t.Fatalf("describe rule: %v", err)
	}

	desc := requireRuleDescription(t, got)
	if desc.RuleID != "dml.where.require" {
		t.Fatalf("unexpected rule ID: %q", desc.RuleID)
	}
	if desc.DefaultLevel != rule.LevelBlocker {
		t.Fatalf("unexpected level: %q", desc.DefaultLevel)
	}
	if !desc.DefaultEnabled {
		t.Fatalf("expected rule to be enabled by default")
	}
	val, ok := desc.DefaultParams["required"].(bool)
	if !ok || !val {
		t.Fatalf("missing default 'required' param, got %v", desc.DefaultParams["required"])
	}
}

func TestDescribeRuleToolUnknownRule(t *testing.T) {
	t.Parallel()

	result, _, err := describeRuleTool(context.Background(), nil, describeRuleInput{RuleID: "nonexistent"})
	if err != nil {
		t.Fatalf("expected tool error result, got protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error result")
	}
	body := requireToolErrorBody(t, result)
	if body["code"] != "bad_request" {
		t.Fatalf("unexpected error code: %#v", body["code"])
	}
	if !strings.Contains(body["message"].(string), "not found") {
		t.Fatalf("unexpected error body: %#v", body)
	}
}

func TestListRulesToolFiltersByQuery(t *testing.T) {
	t.Parallel()

	_, got, err := listRulesTool(context.Background(), nil, listRulesInput{Query: "dml.where.require"})
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}

	resp := requireListRulesResponse(t, got)
	if resp.Count != 1 {
		t.Fatalf("expected single match, got %d", resp.Count)
	}
	if len(resp.Rules) != 1 {
		t.Fatalf("unexpected rules length: %d", len(resp.Rules))
	}
	if resp.Rules[0].RuleID != "dml.where.require" {
		t.Fatalf("unexpected rule ID: %q", resp.Rules[0].RuleID)
	}
	if resp.Query != "dml.where.require" {
		t.Fatalf("query not reflected: %q", resp.Query)
	}
}

func TestGetCapabilitiesToolReturnsKnownSummary(t *testing.T) {
	t.Parallel()

	_, got, err := newGetCapabilitiesTool(Config{})(context.Background(), nil, getCapabilitiesInput{})
	if err != nil {
		t.Fatalf("get capabilities: %v", err)
	}

	resp := requireCapabilitiesResponse(t, got)
	want := capabilitiesResponse{
		Transport: "stdio",
		Tools: []string{
			"audit_sql",
			"describe_rule",
			"list_rules",
			"get_capabilities",
		},
		AuditModes:                []string{"offline", "metadata-aware"},
		Dialects:                  []string{"mysql", "tidb"},
		TopLevelInputs:            []string{"sql", "dialect", "config_path", "connection_ref", "connection"},
		ConnectionInputs:          []string{"connection.host", "connection.port", "connection.socket", "connection.user", "connection.schema", "connection.dialect", "connection.password", "connection.password_env", "connection.password_file"},
		InputRules:                []string{"connection_ref and connection are mutually exclusive", "top-level dialect overrides connection.dialect when both are set"},
		ConnectionRefPath:         "~/.config/deltascope/connections.yaml",
		ConnectionRefOverrideFlag: "-connections-path",
		ResultFields:              []string{"verdict", "summary", "statements", "global_findings", "explanation", "context"},
		ContextFields:             []string{"mode", "dialect", "dialect_source", "schema", "schema_source", "metadata_source"},
		StructuredErrors:          []string{"bad_request", "connection_invalid", "connection_failed", "config_invalid"},
		MetadataFeatures:          []string{"schema context", "instance facts", "target table snapshots"},
		RuleCatalogTools:          []string{"describe_rule", "list_rules"},
		CapabilityVersion:         "mcp-v1",
	}
	if !reflect.DeepEqual(resp, want) {
		t.Fatalf("unexpected capabilities:\n got: %+v\nwant: %+v", resp, want)
	}
}

func TestGetCapabilitiesToolReflectsConfiguredConnectionsPath(t *testing.T) {
	t.Parallel()

	_, got, err := newGetCapabilitiesTool(Config{ConnectionsPath: "/tmp/custom-connections.yaml"})(context.Background(), nil, getCapabilitiesInput{})
	if err != nil {
		t.Fatalf("get capabilities: %v", err)
	}

	resp := requireCapabilitiesResponse(t, got)
	if resp.ConnectionRefPath != "/tmp/custom-connections.yaml" {
		t.Fatalf("unexpected connection_ref_path: %q", resp.ConnectionRefPath)
	}
}

func requireRuleDescription(t *testing.T, value any) ruleDescription {
	t.Helper()
	desc, ok := value.(ruleDescription)
	if !ok {
		t.Fatalf("expected ruleDescription, got %T", value)
	}
	return desc
}

func requireListRulesResponse(t *testing.T, value any) listRulesResponse {
	t.Helper()
	resp, ok := value.(listRulesResponse)
	if !ok {
		t.Fatalf("expected listRulesResponse, got %T", value)
	}
	return resp
}

func requireCapabilitiesResponse(t *testing.T, value any) capabilitiesResponse {
	t.Helper()
	resp, ok := value.(capabilitiesResponse)
	if !ok {
		t.Fatalf("expected capabilitiesResponse, got %T", value)
	}
	return resp
}

func requireToolErrorBody(t *testing.T, result any) map[string]any {
	t.Helper()
	callResult, ok := result.(*sdkmcp.CallToolResult)
	if !ok {
		t.Fatalf("expected *mcp.CallToolResult, got %T", result)
	}
	switch body := callResult.StructuredContent.(type) {
	case map[string]any:
		return body
	case toolErrorPayload:
		return map[string]any{
			"code":    body.Code,
			"message": body.Message,
		}
	default:
		t.Fatalf("expected structured error body, got %T", callResult.StructuredContent)
		return nil
	}
}
