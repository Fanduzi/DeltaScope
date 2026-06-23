package mcpapi

import (
	"context"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAuditSQLToolMySQLReturningEmitsUnsupportedNotice(t *testing.T) {
	t.Parallel()
	server := NewServer(Config{Version: "test-version"})

	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "audit_sql",
		Arguments: map[string]any{"sql": "insert into users(id) values (1) returning id;"},
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got tool error: %#v", result)
	}

	body, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured map result, got %T", result.StructuredContent)
	}
	findings, ok := body["global_findings"].([]any)
	if !ok {
		t.Fatalf("expected global_findings array, got %#v", body["global_findings"])
	}
	var hasMySQLReturning, hasPostgreSQL bool
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		switch finding["rule_id"] {
		case "dialect.mysql.returning.unsupported.notice":
			hasMySQLReturning = true
		case "dialect.postgresql.syntax.detected.notice":
			hasPostgreSQL = true
		}
	}
	if !hasMySQLReturning {
		t.Fatalf("expected mysql returning unsupported notice, got %#v", findings)
	}
	if hasPostgreSQL {
		t.Fatalf("did not expect postgresql syntax notice for mysql returning, got %#v", findings)
	}
}
