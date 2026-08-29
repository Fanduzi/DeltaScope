// Package mcpapi verifies MCP diagnostic result contracts.
// input: audit_sql calls containing parser-error and valid SQL statements
// output: bounded MCP errors that preserve partial audit results and diagnostic evidence
// pos: MCP interface parser-diagnostic and partial-result regression coverage
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestUnsupportedDiagnosticsEvidenceMCPParserError(t *testing.T) {
	t.Parallel()

	server := NewServer(Config{Version: "test-version"})

	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "audit_sql",
		Arguments: map[string]any{"sql": "CREATE FUNCTION hello() RETURNS VARCHAR(20) RETURN 'secret_body_value'", "dialect": "mysql"},
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected error result for parser-error SQL, got success: %#v", result)
	}

	body, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured map result, got %T", result.StructuredContent)
	}

	bodyJSON, _ := json.Marshal(body)
	bodyLower := strings.ToLower(string(bodyJSON))

	if strings.Contains(bodyLower, "secret_body_value") {
		t.Fatalf("MCP response leaked forbidden payload in %s", bodyJSON)
	}
	if strings.Contains(bodyLower, "near ") {
		t.Fatalf("MCP response leaked raw parser fragment in %s", bodyJSON)
	}

	if !strings.Contains(bodyLower, "parser_error") {
		t.Fatalf("expected 'parser_error' classification in MCP response, got %s", bodyJSON)
	}
	if !strings.Contains(bodyLower, "action_hint") {
		t.Fatalf("expected 'action_hint' field in MCP response, got %s", bodyJSON)
	}
	if !strings.Contains(bodyLower, "not audited") {
		t.Fatalf("expected 'not audited' reason text in MCP response, got %s", bodyJSON)
	}
	if !strings.Contains(bodyLower, "guidance_code") {
		t.Fatalf("expected guidance_code in MCP response, got %s", bodyJSON)
	}
	if !strings.Contains(bodyLower, "parser_upgrade_candidate") {
		t.Fatalf("expected parser_upgrade_candidate guidance_code value in MCP response, got %s", bodyJSON)
	}
	if !strings.Contains(bodyLower, "evidence_ref") {
		t.Fatalf("expected evidence_ref in MCP response, got %s", bodyJSON)
	}
	if !strings.Contains(bodyLower, "https://github.com/fanduzi/deltascope/") {
		t.Fatalf("expected GitHub evidence_ref URL in MCP response, got %s", bodyJSON)
	}

	for _, content := range result.Content {
		if text, ok := content.(*sdkmcp.TextContent); ok {
			if strings.Contains(text.Text, "secret_body_value") {
				t.Fatalf("MCP text content leaked forbidden payload in %q", text.Text)
			}
			if strings.Contains(strings.ToLower(text.Text), "near ") {
				t.Fatalf("MCP text content leaked raw parser fragment in %q", text.Text)
			}
		}
	}
}

func TestMCPParserErrorResponsePreservesPartialAuditResult(t *testing.T) {
	t.Parallel()

	server := NewServer(Config{Version: "test-version"})
	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     "ALTER TABLE users ADD COLUMN x INT;\nCREATE INDEX CONCURRENTLY idx_x ON users (x);\nDELETE FROM users;",
			"dialect": "mysql",
		},
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected parser-error MCP result, got %#v", result)
	}
	body, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured map result, got %T", result.StructuredContent)
	}
	bodyJSON, _ := json.Marshal(body)
	summary, _ := body["summary"].(map[string]any)
	if summary["statements"] != float64(2) {
		t.Fatalf("expected two audited statements, got %s", bodyJSON)
	}
	statements, _ := body["statements"].([]any)
	if len(statements) != 2 || !strings.Contains(string(bodyJSON), "dml.where.require") {
		t.Fatalf("expected valid statements and DELETE finding, got %s", bodyJSON)
	}
	diagnostics, _ := body["diagnostics"].([]any)
	if len(diagnostics) != 1 || diagnostics[0].(map[string]any)["line"] != float64(2) {
		t.Fatalf("expected one line-2 parser diagnostic, got %s", bodyJSON)
	}
	runContext, _ := body["context"].(map[string]any)
	if runContext["mode"] != "offline" || runContext["dialect"] != "mysql" {
		t.Fatalf("expected normal offline context on partial result, got %s", bodyJSON)
	}
	if strings.Contains(string(bodyJSON), "idx_x") {
		t.Fatalf("MCP diagnostic response leaked invalid SQL text: %s", bodyJSON)
	}
}
