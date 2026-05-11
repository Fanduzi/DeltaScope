//go:build postgresql

package mcpapi

import (
	"context"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAuditSQLToolPostgreSQLAdvancedViewLifecycleRuleCoverage(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		wantRuleIDs []string
	}{
		{
			name:        "create_or_replace_view_advisory",
			sql:         "CREATE OR REPLACE VIEW active_users AS SELECT id FROM users WHERE active = true",
			wantRuleIDs: []string{"ddl.pg.create_or_replace_view.advisory"},
		},
		{
			name:        "create_temp_view_notice",
			sql:         "CREATE TEMP VIEW session_data AS SELECT id FROM temp_storage",
			wantRuleIDs: []string{"ddl.pg.create_temp_view.notice"},
		},
		{
			name:        "create_view_check_option_notice",
			sql:         "CREATE VIEW checked_view AS SELECT * FROM users WHERE active = true WITH CHECK OPTION",
			wantRuleIDs: []string{"ddl.pg.create_view.check_option.notice"},
		},
		{
			name:        "alter_view_rename_notice",
			sql:         "ALTER VIEW v_old RENAME TO v_new",
			wantRuleIDs: []string{"ddl.pg.alter_view.rename.notice"},
		},
		{
			name:        "alter_view_set_schema_notice",
			sql:         "ALTER VIEW v_stats SET SCHEMA archive",
			wantRuleIDs: []string{"ddl.pg.alter_view.set_schema.notice"},
		},
		{
			name:        "drop_view_cascade_warn",
			sql:         "DROP VIEW v_stats CASCADE",
			wantRuleIDs: []string{"ddl.pg.drop_view.cascade.warn"},
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
