//go:build postgresql

// Package mcpapi verifies MCP unsupported-statement result contracts.
// input: audit_sql calls containing structured unsupported PostgreSQL statements
// output: MCP error results that serialize the review-floored unsupported contract
// pos: MCP adapter regression coverage for the unsupported-statement verdict floor
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPUnsupportedStatementAppliesReviewFloor(t *testing.T) {
	t.Parallel()

	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "audit_sql",
		Arguments: map[string]any{"sql": "SELECT 1", "dialect": "postgresql"},
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected error result for unsupported SQL, got %#v", result)
	}
	body, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured map result, got %T", result.StructuredContent)
	}
	bodyJSON, _ := json.Marshal(body)
	if body["verdict"] != "review" {
		t.Fatalf("expected unsupported review floor, got %s", bodyJSON)
	}
	summary, _ := body["summary"].(map[string]any)
	if summary["statements"] != float64(0) {
		t.Fatalf("expected zero audited statements, got %s", bodyJSON)
	}
	unsupported, _ := body["unsupported"].([]any)
	if len(unsupported) != 1 {
		t.Fatalf("expected one unsupported detail, got %s", bodyJSON)
	}
	item, _ := unsupported[0].(map[string]any)
	if item["feature"] != "select" {
		t.Fatalf("expected unsupported feature select, got %#v", item)
	}
	diagnostics, _ := body["diagnostics"].([]any)
	if len(diagnostics) != 1 {
		t.Fatalf("expected one unsupported diagnostic, got %s", bodyJSON)
	}
	diagnostic, _ := diagnostics[0].(map[string]any)
	if diagnostic["classification"] != "unsupported_statement" || diagnostic["audited"] != false {
		t.Fatalf("expected unaudited unsupported_statement diagnostic, got %#v", diagnostic)
	}
	if strings.Contains(diagnostic["reason"].(string)+diagnostic["action_hint"].(string), "SELECT 1") {
		t.Fatalf("MCP diagnostic leaked SQL text: %s", bodyJSON)
	}
}
