//go:build postgresql

package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerAuditPostgreSQLFunctionProcedureLifecycleRuleCoverage(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		wantRuleIDs []string
	}{
		{
			name:        "create_function_notice",
			sql:         "CREATE FUNCTION log_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$;",
			wantRuleIDs: []string{"ddl.pg.create_function.notice"},
		},
		{
			name:        "create_function_security_definer_warn",
			sql:         "CREATE FUNCTION log_change() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$ BEGIN RETURN NEW; END $$;",
			wantRuleIDs: []string{"ddl.pg.create_function.notice", "ddl.pg.create_function.security_definer.warn"},
		},
		{
			name:        "create_or_replace_function_advisory",
			sql:         "CREATE OR REPLACE FUNCTION log_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$;",
			wantRuleIDs: []string{"ddl.pg.create_function.notice", "ddl.pg.create_or_replace_function.advisory"},
		},
		{
			name:        "drop_function_advisory",
			sql:         "DROP FUNCTION log_change();",
			wantRuleIDs: []string{"ddl.pg.drop_function.advisory"},
		},
		{
			name:        "create_procedure_notice",
			sql:         "CREATE PROCEDURE reset_counter() LANGUAGE plpgsql AS $$ BEGIN NULL; END $$;",
			wantRuleIDs: []string{"ddl.pg.create_procedure.notice"},
		},
		{
			name:        "drop_procedure_advisory",
			sql:         "DROP PROCEDURE reset_counter();",
			wantRuleIDs: []string{"ddl.pg.drop_procedure.advisory"},
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
