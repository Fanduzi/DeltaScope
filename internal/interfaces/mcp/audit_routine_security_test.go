package mcpapi

import (
	"context"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAuditSQLRoutineSecurityWarningParity(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test-version"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	for _, tc := range []struct {
		sql     string
		partial bool
	}{
		{"CREATE PROCEDURE p() SET @x=1", false},
		{"CREATE FUNCTION f() RETURNS INT RETURN 1", true},
	} {
		result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
			Name: "audit_sql", Arguments: map[string]any{"sql": tc.sql, "dialect": "mysql"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError != tc.partial {
			t.Fatalf("isError=%t, want %t", result.IsError, tc.partial)
		}
		body := requireAuditStructuredMap(t, result)
		if body["verdict"] != "review" {
			t.Fatalf("verdict=%v", body["verdict"])
		}
		stmts := body["statements"].([]any)
		found := false
		for _, value := range stmts[0].(map[string]any)["findings"].([]any) {
			f := value.(map[string]any)
			if f["rule_id"] == "ddl.routine.security.definer.warn" {
				found = true
				if f["level"] != "warning" || f["metadata"].(map[string]any)["security_source"] != "create_default" {
					t.Fatalf("finding: %+v", f)
				}
			}
		}
		if !found {
			t.Fatal("missing security warning")
		}
	}
}
