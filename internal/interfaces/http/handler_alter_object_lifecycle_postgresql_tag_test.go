//go:build postgresql

package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerAuditPostgreSQLAlterObjectLifecycleRuleCoverage(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
	}{
		{
			name:       "alter_schema_rename_notice",
			sql:        "ALTER SCHEMA app RENAME TO app_new;",
			wantRuleID: "ddl.pg.alter_schema.rename.notice",
		},
		{
			name:       "alter_schema_owner_notice",
			sql:        "ALTER SCHEMA app OWNER TO app_owner;",
			wantRuleID: "ddl.pg.alter_schema.owner.notice",
		},
		{
			name:       "alter_index_rename_notice",
			sql:        "ALTER INDEX idx_users_email RENAME TO idx_users_email_v2;",
			wantRuleID: "ddl.pg.alter_index.rename.notice",
		},
		{
			name:       "alter_index_set_tablespace_notice",
			sql:        "ALTER INDEX idx_users_email SET TABLESPACE pg_default;",
			wantRuleID: "ddl.pg.alter_index.set_tablespace.notice",
		},
		{
			name:       "alter_materialized_view_rename_notice",
			sql:        "ALTER MATERIALIZED VIEW mv_stats RENAME TO mv_stats_v2;",
			wantRuleID: "ddl.pg.alter_materialized_view.rename.notice",
		},
		{
			name:       "alter_materialized_view_set_schema_notice",
			sql:        "ALTER MATERIALIZED VIEW mv_stats SET SCHEMA archive;",
			wantRuleID: "ddl.pg.alter_materialized_view.set_schema.notice",
		},
		{
			name:       "alter_type_add_attribute_notice",
			sql:        "ALTER TYPE address ADD ATTRIBUTE country text;",
			wantRuleID: "ddl.pg.alter_type.add_attribute.notice",
		},
		{
			name:       "alter_type_drop_attribute_warn",
			sql:        "ALTER TYPE address DROP ATTRIBUTE city;",
			wantRuleID: "ddl.pg.alter_type.drop_attribute.warn",
		},
		{
			name:       "alter_type_alter_attribute_type_warn",
			sql:        "ALTER TYPE address ALTER ATTRIBUTE street TYPE varchar(255);",
			wantRuleID: "ddl.pg.alter_type.alter_attribute_type.warn",
		},
		{
			name:       "alter_type_rename_attribute_notice",
			sql:        "ALTER TYPE address RENAME ATTRIBUTE street TO line1;",
			wantRuleID: "ddl.pg.alter_type.rename_attribute.notice",
		},
		{
			name:       "alter_extension_add_member_notice",
			sql:        "ALTER EXTENSION pg_trgm ADD TABLE users;",
			wantRuleID: "ddl.pg.alter_extension.add_member.notice",
		},
		{
			name:       "alter_extension_drop_member_warn",
			sql:        "ALTER EXTENSION pg_trgm DROP TABLE users;",
			wantRuleID: "ddl.pg.alter_extension.drop_member.warn",
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

			found := false
			for _, item := range findings {
				finding, ok := item.(map[string]any)
				if !ok {
					continue
				}
				ruleID, _ := finding["rule_id"].(string)
				if ruleID == tt.wantRuleID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected finding with rule_id %s, got %#v", tt.wantRuleID, findings)
			}
		})
	}
}
