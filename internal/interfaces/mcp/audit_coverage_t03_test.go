// Package mcpapi verifies the T03 incomplete-coverage MCP contract.
// input: audit_sql tool calls containing recognized-but-unsupported DDL
// output: isError=true tool results carrying the partial audit payload
// pos: MCP transport contract tests for incomplete audit results (issue #82)
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"context"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Recognized-but-unsupported statements surface as a tool error whose
// structured payload still carries the partial audit result and coverage.
func TestAuditSQLT03IncompleteCoverageIsError(t *testing.T) {
	t.Parallel()

	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test-version"}))
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     "CREATE SEQUENCE golden_seq START WITH 1; ALTER TABLE t ADD COLUMN c INT;",
			"dialect": "mysql",
		},
	})
	if err != nil {
		t.Fatalf("expected tool error result, got protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected isError=true for incomplete coverage")
	}
	body := requireAuditStructuredMap(t, result)
	if body["verdict"] != "review" {
		t.Fatalf("structured verdict = %#v, want review", body["verdict"])
	}
	if c, _ := body["coverage"].(map[string]any); c["status"] != "incomplete" {
		t.Fatalf("expected aggregate coverage incomplete, got %#v", body["coverage"])
	}
	stmts, _ := body["statements"].([]any)
	if len(stmts) != 2 {
		t.Fatalf("expected 2 retained statements, got %#v", body["statements"])
	}
	unsupported, _ := body["unsupported"].([]any)
	if len(unsupported) != 1 {
		t.Fatalf("expected 1 unsupported detail, got %#v", body["unsupported"])
	}
}
