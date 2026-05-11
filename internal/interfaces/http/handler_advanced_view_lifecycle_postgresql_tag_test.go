//go:build postgresql

package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerAuditPostgreSQLAdvancedViewLifecycleRuleCoverage(t *testing.T) {
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
			handler, err := NewHandler("", "test-build")
			if err != nil {
				t.Fatalf("new handler: %v", err)
			}

			body := `{"sql":"` + tt.sql + `","dialect":"postgresql"}`
			req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}

			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			statements, ok := payload["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected one statement, got %#v", payload["statements"])
			}
			statement, _ := statements[0].(map[string]any)
			findings, ok := statement["findings"].([]any)
			if !ok || len(findings) < 1 {
				t.Fatalf("expected at least one finding, got %#v", statement["findings"])
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
