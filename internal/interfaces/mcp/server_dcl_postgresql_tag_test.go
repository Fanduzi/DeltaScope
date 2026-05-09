//go:build postgresql

package mcpapi

import (
	"context"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAuditSQLToolPostgreSQLTablePrivilegeDCLRuleCoverage(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		wantRuleIDs []string
	}{
		{
			name:        "grant_select_notice",
			sql:         "GRANT SELECT ON TABLE users TO analyst;",
			wantRuleIDs: []string{"ddl.pg.grant.table_privilege.notice"},
		},
		{
			name:        "grant_all_duplicate",
			sql:         "GRANT ALL PRIVILEGES ON TABLE users TO analyst;",
			wantRuleIDs: []string{"ddl.pg.grant.table_privilege.notice", "ddl.pg.grant.table_privilege.all.warn"},
		},
		{
			name:        "revoke_all_cascade_duplicate",
			sql:         "REVOKE ALL PRIVILEGES ON TABLE users FROM analyst CASCADE;",
			wantRuleIDs: []string{"ddl.pg.revoke.table_privilege.notice", "ddl.pg.revoke.table_privilege.cascade.warn"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(Config{Version: "test-version"})
			session, err := connectClientSession(context.Background(), server)
			if err != nil {
				t.Fatalf("connect session: %v", err)
			}
			t.Cleanup(func() { _ = session.Close() })

			result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
				Name:      "audit_sql",
				Arguments: map[string]any{"sql": tt.sql, "dialect": "postgresql"},
			})
			if err != nil {
				t.Fatalf("call audit_sql: %v", err)
			}
			if result.IsError {
				t.Fatalf("expected success result, got tool error: %#v", result)
			}

			body, ok := result.StructuredContent.(map[string]any)
			if !ok {
				t.Fatalf("expected structured content, got %T", result.StructuredContent)
			}

			if unsupported, ok := body["unsupported"].([]any); ok && len(unsupported) != 0 {
				t.Fatalf("expected no unsupported details, got %#v", unsupported)
			}
			statements, ok := body["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one statement, got %#v", body["statements"])
			}
			statement, ok := statements[0].(map[string]any)
			if !ok {
				t.Fatalf("expected statement object, got %#v", statements[0])
			}
			findings, ok := statement["findings"].([]any)
			if !ok {
				t.Fatalf("expected findings array, got %#v", statement["findings"])
			}

			wantRuleIDs := map[string]bool{}
			for _, id := range tt.wantRuleIDs {
				wantRuleIDs[id] = false
			}
			for _, item := range findings {
				finding, ok := item.(map[string]any)
				if !ok {
					continue
				}
				ruleID, _ := finding["rule_id"].(string)
				if _, expected := wantRuleIDs[ruleID]; expected {
					wantRuleIDs[ruleID] = true
				}
			}
			for ruleID, found := range wantRuleIDs {
				if !found {
					t.Fatalf("expected finding with rule_id %s, got %#v", ruleID, findings)
				}
			}
		})
	}
}
