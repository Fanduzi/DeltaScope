//go:build postgresql

package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAuditSQLPostgreSQLAdvancedViewLifecycleRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
		wantLevel  rule.Level
	}{
		{
			name:       "create_or_replace_view_advisory",
			sql:        "CREATE OR REPLACE VIEW active_users AS SELECT id FROM users WHERE active = true;",
			wantRuleID: "ddl.pg.create_or_replace_view.advisory",
			wantLevel:  rule.LevelNotice,
		},
		{
			name:       "create_temp_view_notice",
			sql:        "CREATE TEMP VIEW session_data AS SELECT id, payload FROM temp_storage;",
			wantRuleID: "ddl.pg.create_temp_view.notice",
			wantLevel:  rule.LevelNotice,
		},
		{
			name:       "create_view_check_option_notice",
			sql:        "CREATE VIEW checked_view AS SELECT * FROM users WHERE active = true WITH CHECK OPTION;",
			wantRuleID: "ddl.pg.create_view.check_option.notice",
			wantLevel:  rule.LevelNotice,
		},
		{
			name:       "alter_view_rename_notice",
			sql:        "ALTER VIEW v_old RENAME TO v_new;",
			wantRuleID: "ddl.pg.alter_view.rename.notice",
			wantLevel:  rule.LevelNotice,
		},
		{
			name:       "alter_view_set_schema_notice",
			sql:        "ALTER VIEW v_stats SET SCHEMA archive;",
			wantRuleID: "ddl.pg.alter_view.set_schema.notice",
			wantLevel:  rule.LevelNotice,
		},
		{
			name:       "drop_view_cascade_warn",
			sql:        "DROP VIEW v_stats CASCADE;",
			wantRuleID: "ddl.pg.drop_view.cascade.warn",
			wantLevel:  rule.LevelWarning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: spec.DialectPostgreSQL,
			})
			if err != nil {
				t.Fatalf("audit sql: %v", err)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(result.Statements))
			}

			found := false
			for _, f := range result.Statements[0].Findings {
				if f.RuleID == tt.wantRuleID {
					found = true
					if f.Level != tt.wantLevel {
						t.Errorf("expected level %s, got %s", tt.wantLevel, f.Level)
					}
					break
				}
			}
			if !found {
				t.Fatalf("expected rule %s, got %v", tt.wantRuleID, collectAuditResultRuleIDs(result))
			}
		})
	}
}

func TestAuditSQLPostgreSQLDropViewNoCascadeNoWarning(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "DROP VIEW v_stats;",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.drop_view.cascade.warn" {
			t.Fatal("cascade.warn should not fire for DROP VIEW without CASCADE")
		}
	}
}

func TestAuditSQLPostgreSQLCreateViewNoReplaceNoAdvisory(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "CREATE VIEW v_simple AS SELECT 1;",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}

	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.create_or_replace_view.advisory" {
			t.Fatal("or_replace advisory should not fire for plain CREATE VIEW")
		}
	}
}
