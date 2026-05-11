//go:build postgresql

package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// ---------------------------------------------------------------------------
// v0.70.0 Task 2: Service tests — PG RLS/Policy lifecycle rules
// ---------------------------------------------------------------------------

func TestAuditSQLPostgreSQLPolicyLifecycleRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
		wantLevel  rule.Level
	}{
		{
			name:       "create_policy_notice",
			sql:        "CREATE POLICY users_select ON users AS PERMISSIVE FOR SELECT TO PUBLIC USING (true)",
			wantRuleID: "ddl.pg.create_policy.notice",
			wantLevel:  rule.LevelNotice,
		},
		{
			name:       "alter_policy_notice",
			sql:        "ALTER POLICY users_select ON users USING (user_id = current_user)",
			wantRuleID: "ddl.pg.alter_policy.notice",
			wantLevel:  rule.LevelNotice,
		},
		{
			name:       "drop_policy_warn",
			sql:        "DROP POLICY users_select ON users",
			wantRuleID: "ddl.pg.drop_policy.warn",
			wantLevel:  rule.LevelWarning,
		},
		{
			name:       "enable_rls_notice",
			sql:        "ALTER TABLE users ENABLE ROW LEVEL SECURITY",
			wantRuleID: "ddl.pg.alter.enable_rls.notice",
			wantLevel:  rule.LevelNotice,
		},
		{
			name:       "disable_rls_warn",
			sql:        "ALTER TABLE users DISABLE ROW LEVEL SECURITY",
			wantRuleID: "ddl.pg.alter.disable_rls.warn",
			wantLevel:  rule.LevelWarning,
		},
		{
			name:       "force_rls_notice",
			sql:        "ALTER TABLE users FORCE ROW LEVEL SECURITY",
			wantRuleID: "ddl.pg.alter.force_rls.notice",
			wantLevel:  rule.LevelNotice,
		},
		{
			name:       "no_force_rls_notice",
			sql:        "ALTER TABLE users NO FORCE ROW LEVEL SECURITY",
			wantRuleID: "ddl.pg.alter.no_force_rls.notice",
			wantLevel:  rule.LevelNotice,
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
			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(result.Statements))
			}

			found := false
			for _, f := range result.Statements[0].Findings {
				if f.RuleID == tt.wantRuleID {
					found = true
					if f.Level != tt.wantLevel {
						t.Errorf("expected level %q, got %q", tt.wantLevel, f.Level)
					}
					break
				}
			}
			if !found {
				t.Fatalf("expected %s finding, got %#v", tt.wantRuleID, result.Statements[0].Findings)
			}
		})
	}
}

func TestAuditSQLPostgreSQLPolicyRulesDoNotFireOnMySQL(t *testing.T) {
	t.Parallel()
	pgOnlyRuleIDs := []string{
		"ddl.pg.create_policy.notice",
		"ddl.pg.alter_policy.notice",
		"ddl.pg.drop_policy.warn",
		"ddl.pg.alter.enable_rls.notice",
		"ddl.pg.alter.disable_rls.warn",
		"ddl.pg.alter.force_rls.notice",
		"ddl.pg.alter.no_force_rls.notice",
	}

	// Use SQL that MySQL/TiDB can parse but must not trigger PG-only rules.
	tests := []struct {
		name    string
		sql     string
		dialect spec.Dialect
	}{
		{name: "alter_add_col_mysql", sql: "ALTER TABLE users ADD COLUMN bio TEXT", dialect: spec.DialectMySQL},
		{name: "alter_add_col_tidb", sql: "ALTER TABLE users ADD COLUMN bio TEXT", dialect: spec.DialectTiDB},
		{name: "drop_table_mysql", sql: "DROP TABLE users", dialect: spec.DialectMySQL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:     tt.sql,
				Dialect: tt.dialect,
			})
			if err != nil {
				t.Fatalf("audit sql: %v", err)
			}

			for _, stmt := range result.Statements {
				for _, f := range stmt.Findings {
					for _, pgID := range pgOnlyRuleIDs {
						if f.RuleID == pgID {
							t.Fatalf("PG-only rule %q fired on %s for SQL: %s", pgID, tt.dialect, tt.sql)
						}
					}
				}
			}
			for _, f := range result.GlobalFindings {
				for _, pgID := range pgOnlyRuleIDs {
					if f.RuleID == pgID {
						t.Fatalf("PG-only rule %q fired as global finding on %s for SQL: %s", pgID, tt.dialect, tt.sql)
					}
				}
			}
		})
	}
}

func TestAuditSQLPostgreSQLDropPolicyIfExists(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "DROP POLICY IF EXISTS users_select ON users",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}
	found := false
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.drop_policy.warn" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected ddl.pg.drop_policy.warn finding for DROP POLICY IF EXISTS, got %#v", result.Statements[0].Findings)
	}
}
