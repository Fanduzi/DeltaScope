//go:build postgresql

package deltascope

import (
	"context"
	"testing"
)

func TestAuditPostgreSQLAdvancedViewLifecycleRuleCoverage(t *testing.T) {
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
			result, err := Audit(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: DialectPostgreSQL,
			})
			if err != nil {
				t.Fatalf("expected supported path, got error: %v", err)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(result.Statements))
			}
			if result.Statements[0].Kind != "ddl" {
				t.Fatalf("expected ddl kind, got %q", result.Statements[0].Kind)
			}

			wantRuleIDs := map[string]bool{}
			for _, id := range tt.wantRuleIDs {
				wantRuleIDs[id] = false
			}
			for _, f := range result.Statements[0].Findings {
				if _, expected := wantRuleIDs[f.RuleID]; expected {
					wantRuleIDs[f.RuleID] = true
				}
			}
			for ruleID, found := range wantRuleIDs {
				if !found {
					t.Fatalf("expected finding with rule %q, got %#v", ruleID, result.Statements[0].Findings)
				}
			}
		})
	}
}
