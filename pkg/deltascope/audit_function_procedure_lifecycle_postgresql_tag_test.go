//go:build postgresql

package deltascope

import (
	"context"
	"testing"
)

func TestAuditPostgreSQLFunctionProcedureLifecycleRuleCoverage(t *testing.T) {
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
