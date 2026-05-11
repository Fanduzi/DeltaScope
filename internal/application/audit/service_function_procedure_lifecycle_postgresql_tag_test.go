//go:build postgresql

package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAuditSQLPostgreSQLFunctionProcedureLifecycleRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		sql        string
		wantRuleID string
		wantLevel  rule.Level
	}{
		{
			name:       "create_function_notice",
			sql:        "CREATE FUNCTION add(a int, b int) RETURNS int LANGUAGE sql AS $$ SELECT a + b $$;",
			wantRuleID: "ddl.pg.create_function.notice",
			wantLevel:  rule.LevelNotice,
		},
		{
			name:       "create_function_security_definer_warn",
			sql:        "CREATE FUNCTION admin_task() RETURNS void LANGUAGE plpgsql SECURITY DEFINER AS $$ BEGIN NULL; END $$;",
			wantRuleID: "ddl.pg.create_function.security_definer.warn",
			wantLevel:  rule.LevelWarning,
		},
		{
			name:       "create_or_replace_function_advisory",
			sql:        "CREATE OR REPLACE FUNCTION add(a int, b int) RETURNS int LANGUAGE sql AS $$ SELECT a + b $$;",
			wantRuleID: "ddl.pg.create_or_replace_function.advisory",
			wantLevel:  rule.LevelNotice,
		},
		{
			name:       "drop_function_advisory",
			sql:        "DROP FUNCTION add(int, int);",
			wantRuleID: "ddl.pg.drop_function.advisory",
			wantLevel:  rule.LevelNotice,
		},
		{
			name:       "create_procedure_notice",
			sql:        "CREATE PROCEDURE reset_counter() LANGUAGE plpgsql AS $$ BEGIN NULL; END $$;",
			wantRuleID: "ddl.pg.create_procedure.notice",
			wantLevel:  rule.LevelNotice,
		},
		{
			name:       "drop_procedure_advisory",
			sql:        "DROP PROCEDURE reset_counter();",
			wantRuleID: "ddl.pg.drop_procedure.advisory",
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

func TestAuditSQLPostgreSQLFunctionDropIfExists(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		sql            string
		wantDropRuleID string
	}{
		{
			name:           "drop_function_if_exists",
			sql:            "DROP FUNCTION IF EXISTS add(int, int);",
			wantDropRuleID: "ddl.pg.drop_function.advisory",
		},
		{
			name:           "drop_procedure_if_exists",
			sql:            "DROP PROCEDURE IF EXISTS reset_counter();",
			wantDropRuleID: "ddl.pg.drop_procedure.advisory",
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
				if f.RuleID == tt.wantDropRuleID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected rule %s, got %v", tt.wantDropRuleID, collectAuditResultRuleIDs(result))
			}
		})
	}
}

func TestAuditSQLPostgreSQLFunctionNoSecurityDefiner(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "CREATE FUNCTION safe_task() RETURNS void LANGUAGE plpgsql SECURITY INVOKER AS $$ BEGIN NULL; END $$;",
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
		if f.RuleID == "ddl.pg.create_function.security_definer.warn" {
			t.Fatal("security_definer.warn should not fire for SECURITY INVOKER")
		}
	}
}
