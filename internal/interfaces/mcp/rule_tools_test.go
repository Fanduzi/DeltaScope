// Package mcpapi verifies MCP rule-discovery tool behavior.
// input: MCP rule tool handlers plus shipped rule catalog and connection capability expectations
// output: focused coverage for compact list_rules rows, list_rules text surface, describe_rule, and database-aware get_capabilities
// pos: interface-layer tests for MCP rule-discovery behavior
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	rulecatalog "github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
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
	if resp.Rules[0].Level != "blocker" || resp.Rules[0].Dialect != "common" || resp.Rules[0].Kind != "dml" {
		t.Fatalf("expected compact where-rule row, got %#v", resp.Rules[0])
	}
	if resp.Rules[0].Summary != "Require DML where require" {
		t.Fatalf("unexpected summary: %q", resp.Rules[0].Summary)
	}
}

func TestListRulesCallQueryWhereReturnsOneCompactRule(t *testing.T) {
	t.Parallel()

	session := newRuleToolSession(t)
	result := callRuleTool(t, session, "list_rules", map[string]any{"query": "where"})
	body := requireStructuredMap(t, result)

	if body["query"] != "where" {
		t.Fatalf("query not reflected: %#v", body["query"])
	}
	if body["count"] != float64(1) {
		t.Fatalf("expected one rule for query=where, got %#v", body["count"])
	}
	rules, ok := body["rules"].([]any)
	if !ok || len(rules) != 1 {
		t.Fatalf("expected one compact rule, got %#v", body["rules"])
	}
	row, ok := rules[0].(map[string]any)
	if !ok {
		t.Fatalf("expected rule object, got %T", rules[0])
	}
	assertCompactListRuleMap(t, row)
	if row["rule_id"] != "dml.where.require" {
		t.Fatalf("unexpected rule_id: %#v", row["rule_id"])
	}

	text := requireToolText(t, result)
	if strings.Contains(strings.TrimSpace(text), "{") {
		t.Fatalf("filtered content[0].text looks like JSON: %q", truncateForTest(text, 120))
	}
	if !strings.Contains(text, "dml.where.require") {
		t.Fatal("filtered content[0].text missing dml.where.require")
	}
}

func TestDescribeRuleCallStillReturnsFullBody(t *testing.T) {
	t.Parallel()

	session := newRuleToolSession(t)
	result := callRuleTool(t, session, "describe_rule", map[string]any{"rule_id": "dml.where.require"})
	body := requireStructuredMap(t, result)

	if body["rule_id"] != "dml.where.require" {
		t.Fatalf("unexpected rule_id: %#v", body["rule_id"])
	}
	for _, key := range []string{"description", "why", "risk", "suggestion", "config_example", "trigger_example", "valid_example"} {
		value, ok := body[key].(string)
		if !ok || value == "" {
			t.Fatalf("describe_rule missing full-body field %q: %#v", key, body[key])
		}
	}
	if body["default_level"] != "blocker" {
		t.Fatalf("describe_rule default_level = %#v, want blocker", body["default_level"])
	}
}

func TestListRulesCallReturnsCompactCatalogRows(t *testing.T) {
	t.Parallel()

	session := newRuleToolSession(t)
	result := callRuleTool(t, session, "list_rules", map[string]any{})
	body := requireStructuredMap(t, result)

	wantCount := len(rulecatalog.All())
	count, ok := body["count"].(float64)
	if !ok || int(count) != wantCount {
		t.Fatalf("expected full shipped catalog count %d, got %#v", wantCount, body["count"])
	}
	rules, ok := body["rules"].([]any)
	if !ok {
		t.Fatalf("expected rules array, got %T", body["rules"])
	}
	if len(rules) != int(count) {
		t.Fatalf("count %d does not match rules length %d", int(count), len(rules))
	}

	foundWhere := false
	for _, raw := range rules {
		row, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("expected rule object, got %T", raw)
		}
		assertCompactListRuleMap(t, row)
		if row["rule_id"] == "dml.where.require" {
			foundWhere = true
			if row["level"] != "blocker" {
				t.Fatalf("dml.where.require level = %#v, want blocker", row["level"])
			}
			if row["dialect"] != "common" {
				t.Fatalf("dml.where.require dialect = %#v, want common", row["dialect"])
			}
			if row["kind"] != "dml" {
				t.Fatalf("dml.where.require kind = %#v, want dml", row["kind"])
			}
			if row["summary"] != "Require DML where require" {
				t.Fatalf("dml.where.require summary = %#v", row["summary"])
			}
		}
	}
	if !foundWhere {
		t.Fatal("expected compact row for dml.where.require")
	}
}

func TestListRulesCallTextIsNotStructuredJSON(t *testing.T) {
	t.Parallel()

	session := newRuleToolSession(t)
	result := callRuleTool(t, session, "list_rules", map[string]any{})

	text := requireToolText(t, result)
	structuredJSON, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if text == string(structuredJSON) {
		t.Fatal("content[0].text is a second copy of structuredContent")
	}
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		t.Fatalf("content[0].text looks like a JSON dump: %q", truncateForTest(trimmed, 120))
	}
	if len(text) > 80_000 {
		t.Fatalf("content[0].text is %d bytes; want a compact catalog, not a dump", len(text))
	}
	if len(structuredJSON) > 120_000 {
		t.Fatalf("structuredContent is %d bytes; want compact rows", len(structuredJSON))
	}
	if !strings.Contains(text, "dml.where.require") {
		t.Fatal("content[0].text missing dml.where.require")
	}
	if !strings.Contains(text, "Require DML where require") {
		t.Fatal("content[0].text missing the where-rule summary")
	}
	if strings.Contains(text, "config_example") || strings.Contains(text, "trigger_example") {
		t.Fatal("content[0].text still contains full-body describe fields")
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
		Dialects:                  []string{"mysql", "tidb", "postgresql"},
		TopLevelInputs:            []string{"sql", "dialect", "config_path", "connection_ref", "connection"},
		ConnectionInputs:          []string{"connection.host", "connection.port", "connection.socket", "connection.user", "connection.database", "connection.schema", "connection.dialect", "connection.password", "connection.password_env", "connection.password_file", "connection.connect_timeout"},
		InputRules:                []string{"connection_ref and connection are mutually exclusive", "top-level dialect overrides connection.dialect when both are set", "connection inputs support mysql, tidb, and postgresql metadata-aware audit"},
		ConnectionRefPath:         "~/.config/deltascope/connections.yaml",
		ConnectionRefOverrideFlag: "-connections-path",
		ResultFields:              []string{"verdict", "summary", "statements", "global_findings", "explanation", "context"},
		ContextFields:             []string{"mode", "dialect", "dialect_source", "schema", "schema_source", "metadata_source", "note", "unproven"},
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

func newRuleToolSession(t *testing.T) *sdkmcp.ClientSession {
	t.Helper()
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test-version"}))
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func callRuleTool(t *testing.T, session *sdkmcp.ClientSession, name string, arguments map[string]any) *sdkmcp.CallToolResult {
	t.Helper()
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: arguments,
	})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	if result.IsError {
		t.Fatalf("expected success from %s, got tool error: %#v", name, result)
	}
	return result
}

func requireToolText(t *testing.T, result *sdkmcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("expected content[0].text")
	}
	text, ok := result.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	if text.Text == "" {
		t.Fatal("expected non-empty content[0].text")
	}
	return text.Text
}

func truncateForTest(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func requireStructuredMap(t *testing.T, result *sdkmcp.CallToolResult) map[string]any {
	t.Helper()
	body, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured map, got %T", result.StructuredContent)
	}
	return body
}

func assertCompactListRuleMap(t *testing.T, row map[string]any) {
	t.Helper()
	wantKeys := map[string]bool{
		"rule_id": true,
		"level":   true,
		"dialect": true,
		"kind":    true,
		"summary": true,
	}
	for key := range row {
		if !wantKeys[key] {
			t.Fatalf("list_rules row has full-body field %q: %#v", key, row)
		}
	}
	for key := range wantKeys {
		value, ok := row[key].(string)
		if !ok || value == "" {
			t.Fatalf("list_rules row missing %q: %#v", key, row)
		}
	}
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
