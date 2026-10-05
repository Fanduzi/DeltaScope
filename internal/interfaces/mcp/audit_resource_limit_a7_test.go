// Package mcpapi verifies the T05-A7 ordered-state budget at the MCP seam.
// input: a 1025-statement TiDB audit_sql call through the real in-process protocol
// output: isError=true diagnostic result carrying the retained partial audit and the exact bounded resource-limit projection
// pos: MCP transport contract test for the shared ordered-state admission budget (issue #84/T05-A7)
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"context"
	"reflect"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAuditSQLT05A7OrderedStatementBudget(t *testing.T) {
	t.Parallel()

	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test-version"}))
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     strings.Repeat("SELECT 1;\n", 1025),
			"dialect": "tidb",
		},
	})
	if err != nil {
		t.Fatalf("expected tool error result, got protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected isError=true for the over-budget audit")
	}
	body := requireAuditStructuredMap(t, result)
	if body["code"] != "bad_request" {
		t.Fatalf("expected the existing bad_request diagnostic envelope, got %#v", body["code"])
	}
	diagnosticSeen := false
	for _, raw := range body["diagnostics"].([]any) {
		if diagnostic, _ := raw.(map[string]any); diagnostic["classification"] == "unsupported_statement" {
			diagnosticSeen = true
		}
	}
	if !diagnosticSeen {
		t.Fatalf("expected the unsupported_statement diagnostic, got %#v", body["diagnostics"])
	}
	if body["verdict"] != "review" {
		t.Fatalf("verdict = %#v, want review", body["verdict"])
	}
	if c, _ := body["coverage"].(map[string]any); c["status"] != "incomplete" {
		t.Fatalf("coverage = %#v, want incomplete", body["coverage"])
	}
	statements, _ := body["statements"].([]any)
	if len(statements) != 1025 {
		t.Fatalf("statements = %d, want 1025 retained", len(statements))
	}
	for i := 0; i < 1024; i++ {
		statement, _ := statements[i].(map[string]any)
		sc, _ := statement["coverage"].(map[string]any)
		if statement["index"] != float64(i) || sc["status"] != "complete" {
			t.Fatalf("admitted statement %d = %#v, want complete", i, statement)
		}
		if statement["findings"] != nil || statement["evidence_gaps"] != nil || statement["impact"] != nil {
			t.Fatalf("admitted statement %d must not carry findings, gaps, or impact, got %#v", i, statement)
		}
	}
	if body["global_findings"] != nil {
		t.Fatalf("global findings = %#v, want none", body["global_findings"])
	}
	if summary, _ := body["summary"].(map[string]any); summary["blockers"] != float64(0) || summary["warnings"] != float64(0) || summary["notices"] != float64(0) {
		t.Fatalf("finding counters = %#v, want zero", body["summary"])
	}
	last, _ := statements[1024].(map[string]any)
	lc, _ := last["coverage"].(map[string]any)
	normalized, _ := last["normalized_sql"].(string)
	if last["index"] != float64(1024) || last["kind"] != "unknown" || strings.TrimSpace(last["raw_sql"].(string)) != "SELECT 1;" || normalized == "" {
		t.Fatalf("blocked statement identity = %#v, want index 1024 kind unknown raw SELECT 1;", last)
	}
	if lc["status"] != "incomplete" || last["findings"] != nil || last["evidence_gaps"] != nil || last["impact"] != nil {
		t.Fatalf("blocked statement = %#v, want incomplete with no findings, gaps, or impact", last)
	}
	unsupported, _ := body["unsupported"].([]any)
	if len(unsupported) != 1 {
		t.Fatalf("unsupported entries = %d, want 1: %#v", len(unsupported), body["unsupported"])
	}
	item, _ := unsupported[0].(map[string]any)
	if item["feature"] != "audit.resource_limit" || item["reason"] != "ordered-state statement budget exhausted" {
		t.Fatalf("unsupported entry = %#v, want the audit.resource_limit budget projection", item)
	}
	if item["index"] != float64(1024) || item["sql"] != last["raw_sql"] {
		t.Fatalf("unsupported identity = %#v, want index 1024 bound to the blocked statement", item)
	}
	wantMeta := map[string]any{
		"phase":    "ordered_state",
		"resource": "statements",
		"limit":    float64(1024),
		"consumed": float64(1024),
		"line":     float64(1025),
		"column":   float64(1),
	}
	if metadata, _ := item["metadata"].(map[string]any); !reflect.DeepEqual(metadata, wantMeta) {
		t.Fatalf("resource metadata = %#v, want %#v", item["metadata"], wantMeta)
	}
}
