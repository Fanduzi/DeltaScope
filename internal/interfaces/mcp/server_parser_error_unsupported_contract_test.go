package mcpapi

import (
	"context"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPParserErrorUnsupportedContractMySQL(t *testing.T) {
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

	// Extract error message from structured content.
	body, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured map result, got %T", result.StructuredContent)
	}
	message, _ := body["message"].(string)
	if message == "" {
		t.Fatalf("expected non-empty error message in structured content, got %#v", body)
	}

	lower := strings.ToLower(message)
	if !strings.Contains(lower, "not audited") && !strings.Contains(lower, "parse") {
		t.Fatalf("expected not-audited or parse diagnostic, got %q", message)
	}
	if !strings.Contains(lower, "audit") {
		t.Fatalf("expected audit semantics in error message, got %q", message)
	}
	if strings.Contains(message, "secret_body_value") {
		t.Fatalf("MCP response leaked forbidden payload in %q", message)
	}
	if strings.Contains(lower, "near ") {
		t.Fatalf("MCP response leaked raw parser fragment in %q", message)
	}

	// Verify text content also does not leak.
	for _, content := range result.Content {
		if text, ok := content.(*sdkmcp.TextContent); ok {
			if strings.Contains(text.Text, "secret_body_value") {
				t.Fatalf("MCP text content leaked forbidden payload in %q", text.Text)
			}
		}
	}
}

func TestMCPParserErrorUnsupportedContractTiDB(t *testing.T) {
	t.Parallel()

	server := NewServer(Config{Version: "test-version"})

	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "audit_sql",
		Arguments: map[string]any{"sql": "ALTER TABLE users LOCALITY = 'region=us-east-1'", "dialect": "tidb"},
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
	message, _ := body["message"].(string)
	if message == "" {
		t.Fatalf("expected non-empty error message in structured content, got %#v", body)
	}

	lower := strings.ToLower(message)
	if !strings.Contains(lower, "not audited") && !strings.Contains(lower, "parse") {
		t.Fatalf("expected not-audited or parse diagnostic, got %q", message)
	}
	if strings.Contains(message, "us-east-1") {
		t.Fatalf("MCP response leaked forbidden payload in %q", message)
	}
	if strings.Contains(lower, "near ") {
		t.Fatalf("MCP response leaked raw parser fragment in %q", message)
	}
}
